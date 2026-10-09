import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

/* #854 游客 Agent 面 mocked 契约：双态壳按真实身份选表面；游客工作台只打
   /agent/guest/* 端点（绝不携带 Authorization 或落到受保护 /agent 路径）；
   服务端权威余量驱动 caption；用尽后转化卡换下输入区，CTA 打开 SP-17
   登录浮窗。全部交互由路由 mock 承载，不触真实后端。 */

const GUEST_FEATURES = {
  features: {
    web_agent_enabled: true,
    guest_agent_enabled: true,
    desktop_deploy_enabled: false,
    creator_support_enabled: false,
    payment_enabled: false,
  },
  captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
  client: { download_enabled: false, download_url: "", latest_version: "" },
  legal: { current_terms_version: "test", current_privacy_version: "test" },
  upload: {
    image_gallery_min_items: 2,
    image_gallery_max_items: 9,
    video_gallery_min_items: 1,
    video_gallery_max_items: 3,
  },
};

const GATE_OFF_FEATURES = {
  ...GUEST_FEATURES,
  features: { ...GUEST_FEATURES.features, guest_agent_enabled: false },
};

function sseBody(events: unknown[]): string {
  return events.map((event) => `data: ${JSON.stringify(event)}\n`).join("\n") + "\n";
}

async function mockGuestSurface(
  page: Page,
  options: { quota: { remaining: number; max_turns: number; exhausted: boolean } },
) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/config/public", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(GUEST_FEATURES),
    }),
  );
  await mockApiRoute(page, "**/api/v1/agent/guest/quota", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        remaining: options.quota.remaining,
        max_turns: options.quota.max_turns,
        exhausted: options.quota.exhausted,
        conversation_ttl_days: 7,
      }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/agent/guest/conversations", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ conversations: [] }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/agent/guest/conversations/*", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ conversation: {}, messages: [] }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/agent/guest/models", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ models: [] }) }),
  );
  // Any hit on the protected agent surface means the shell routed a guest to
  // the authenticated endpoints — a contract failure, not a harmless extra.
  const protectedHits: string[] = [];
  page.on("request", (request) => {
    const url = request.url();
    if (/\/api\/v1\/agent\/(conversations|models|quota|chat)/.test(url)) protectedHits.push(url);
  });
  return { protectedHits };
}

test("guest workspace serves the anonymous surface with a server-driven quota caption", async ({ page }) => {
  const { protectedHits } = await mockGuestSurface(page, {
    quota: { remaining: 3, max_turns: 3, exhausted: false },
  });

  await page.goto("/agent");
  const caption = page.getByTestId("guest-quota-caption");
  await expect(caption).toHaveText(/3 turns left/);
  await expect(page.locator("textarea")).toBeEnabled();
  // The exhausted conversion card must not be present while budget remains.
  await expect(page.getByTestId("guest-exhausted-card")).toHaveCount(0);
  // Only guest endpoints were touched.
  expect(protectedHits, "guest surface must not call protected agent endpoints").toEqual([]);
});

test("a guest turn streams over the guest endpoint and decrements the caption", async ({ page }) => {
  let quotaState = { remaining: 3, max_turns: 3, exhausted: false };
  await mockGuestSurface(page, { quota: quotaState });
  await mockApiRoute(page, "**/api/v1/agent/guest/quota", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ...quotaState, conversation_ttl_days: 7 }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/agent/guest/chat/stream", (route) => {
    quotaState = { remaining: 2, max_turns: 3, exhausted: false };
    return route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: sseBody([
        { type: "start", trace_id: "guest-trace-1", conversation_id: 11, answer_kind: "grounded_content" },
        { type: "delta", delta: "Grounded answer." },
        {
          type: "done",
          conversation_id: 11,
          message_id: 21,
          answer_kind: "grounded_content",
          answer: "Grounded answer.",
          citations: [],
          tools: [],
          degraded: false,
        },
      ]),
    });
  });

  await page.goto("/agent");
  await page.locator("textarea").fill("What is this platform about?");
  await page.keyboard.press("Enter");
  // The done event triggers the server-authoritative quota refresh; the
  // mocked quota endpoint now answers remaining=2.
  await expect(page.getByTestId("guest-quota-caption")).toHaveText(/2 turns left/, { timeout: 15000 });
});

test("an exhausted budget shows the conversion card and opens the SP-17 login modal", async ({ page }) => {
  await mockGuestSurface(page, { quota: { remaining: 0, max_turns: 3, exhausted: true } });

  await page.goto("/agent");
  const card = page.getByTestId("guest-exhausted-card");
  await expect(card).toBeVisible();
  await expect(page.getByTestId("guest-quota-caption")).toHaveCount(0);
  await expect(page.locator("textarea")).toBeDisabled();

  await card.getByRole("button").click();
  await expect(page.getByTestId("login-modal")).toBeVisible();
});

test("gate off degrades /agent to the login guide with no guest surface", async ({ page }) => {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/config/public", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(GATE_OFF_FEATURES),
    }),
  );

  await page.goto("/agent");
  await expect(page.getByTestId("guest-quota-caption")).toHaveCount(0);
  await expect(page.getByTestId("guest-exhausted-card")).toHaveCount(0);
  // Login-guide fallback: the guest CTA hands the visitor to the login wall.
  await expect(page.getByText("Sign in to use the AI assistant")).toBeVisible();
  // The entry disappears for guests while the gate is off.
  await expect(page.locator("header a[href='/agent']")).toHaveCount(0);
});
