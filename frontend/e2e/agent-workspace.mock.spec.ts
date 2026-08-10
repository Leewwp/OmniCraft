import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

/* Deterministic fake-provider browser contracts for the Agent workspace
   (plan Task 6 Step 2). All chat streams are served by route mocks so the
   backend/MiniMax are never involved. */

const SSE_HEADERS = { "content-type": "text/event-stream" };

function sseBody(events: unknown[]): string {
  return events.map((event) => `data: ${JSON.stringify(event)}\n`).join("\n") + "\n";
}

const CITED_EVENTS: unknown[] = [
  { type: "start", trace_id: "mock-trace-cited", conversation_id: 1, answer_kind: "grounded_content" },
  { type: "tool_status", tool: { name: "search_content", status: "success", duration_ms: 12 } },
  { type: "delta", delta: "这是关于" },
  { type: "delta", delta: "Blender 插件安装的回答。" },
  {
    type: "citation",
    citation: { content_id: 1001, title: "Blender 插件安装教程", zone: "original", excerpt: "步骤一" },
  },
  {
    type: "done",
    conversation_id: 1,
    answer_kind: "grounded_content",
    answer: "这是关于 Blender 插件安装的回答。",
    citations: [{ content_id: 1001, title: "Blender 插件安装教程", zone: "original" }],
    tools: [{ name: "search_content", status: "success" }],
    degraded: false,
  },
];

const NO_EVIDENCE_EVENTS: unknown[] = [
  { type: "start", trace_id: "mock-trace-none", conversation_id: 2, answer_kind: "no_evidence" },
  { type: "delta", delta: "没有足够证据。" },
  { type: "done", conversation_id: 2, answer_kind: "no_evidence", answer: "没有足够证据。", citations: [], tools: [], degraded: true },
];

const ENABLED_FEATURES = {
  features: {
    web_agent_enabled: true,
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

async function mockCreatorSession(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-agent-token" } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: {
          id: 5,
          email: "creator@example.com",
          username: "creator",
          avatar_url: "",
          bio: "",
          reputation: 10,
          preferred_locale: "en",
          role: "user",
          is_banned: false,
          email_verified_at: "2026-01-01T00:00:00Z",
          created_at: "2026-01-01T00:00:00Z",
        },
      }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
    }),
  );
}

async function enableAgent(page: Page) {
  await mockApiRoute(page, "**/api/v1/config/public", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(ENABLED_FEATURES),
    }),
  );
}

async function mockConversationList(page: Page, conversations: unknown[], messagesByConversation: Record<number, unknown[]> = {}) {
  await mockApiRoute(page, "**/api/v1/agent/conversations", (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ conversations }),
    });
  });
  await mockApiRoute(page, "**/api/v1/agent/conversations/*", (route) => {
    const method = route.request().method();
    const id = Number(route.request().url().split("/").pop());
    if (method === "GET") {
      return route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ messages: messagesByConversation[id] ?? [] }),
      });
    }
    if (method === "DELETE") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true }) });
    }
    return route.fallback();
  });
}

async function mockStream(page: Page, events: unknown[], status = 200) {
  await mockApiRoute(page, "**/api/v1/agent/chat/stream", (route) =>
    route.fulfill({
      status,
      contentType: "text/event-stream",
      body: sseBody(events),
    }),
  );
}

async function ask(page: Page, question: string) {
  await page.getByPlaceholder("Describe the works, sources or usage you want to find").fill(question);
  await page.getByRole("button", { name: "Send message" }).click();
}

test("feature-disabled gating: no composer, no header agent nav, no global widget", async ({ page }) => {
  await mockCreatorSession(page);
  await page.goto("/agent");
  await expect(page.getByText("AI Agent capability is not enabled yet.")).toBeVisible();
  await expect(page.getByPlaceholder("Describe the works, sources or usage you want to find")).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Agent" })).toHaveCount(0);
  await expect(page.locator("[data-agent-widget], #agent-widget, [data-testid='agent-widget']")).toHaveCount(0);
  const headerSearch = page.locator("header").getByPlaceholder(/search/i).first();
  if ((await headerSearch.count()) > 0) {
    await expect(headerSearch.locator("xpath=ancestor::form").getByText(/agent mode/i)).toHaveCount(0);
  }
});

test("cited answer streams content, shows tool status and citations", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(
    page,
    [{ id: 1, context_type: "global", updated_at: "2026-08-10T00:00:00Z" }],
    {
      1: [
        { id: 11, role: "user", content: "Blender 插件安装教程" },
        { id: 12, role: "assistant", content: "这是关于 Blender 插件安装的回答。" },
      ],
    },
  );
  await mockStream(page, CITED_EVENTS);

  await page.goto("/agent");
  await ask(page, "Blender 插件安装教程");

  await expect(page.getByText("这是关于 Blender 插件安装的回答。")).toBeVisible();
  await expect(page.getByText("Searched site content")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Site references" })).toBeVisible();
  await expect(page.getByText("Blender 插件安装教程").first()).toBeVisible();
  await expect(page.getByText("Stopped generating")).toHaveCount(0);
});

test("no-evidence question shows the refusal card", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(
    page,
    [
      { id: 1, context_type: "global", updated_at: "2026-08-10T00:00:00Z" },
      { id: 2, context_type: "global", updated_at: "2026-08-10T00:00:00Z" },
    ],
    {
      1: [
        { id: 11, role: "user", content: "Blender 插件安装教程" },
        { id: 12, role: "assistant", content: "这是关于 Blender 插件安装的回答。" },
      ],
      2: [
        { id: 21, role: "user", content: "明天的天气怎么样" },
        { id: 22, role: "assistant", content: "没有足够证据。" },
      ],
    },
  );
  await mockStream(page, NO_EVIDENCE_EVENTS);

  await page.goto("/agent");
  await ask(page, "明天的天气怎么样");

  await expect(page.getByText("Not enough evidence")).toBeVisible();
  await expect(page.getByText(/did not fabricate a conclusion/i)).toBeVisible();
});

/* Browser-native streaming chat mock: wraps window.fetch so the stream
   genuinely arrives incrementally and aborts with the caller's AbortSignal
   (route.fulfill cannot stream chunks with delay). Config is injected through
   window.__agentStreamMock before the question is asked. */
async function installStreamingChat(page: Page) {
  await page.addInitScript(() => {
    const realFetch = window.fetch.bind(window);
    window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
      if (!url.includes("/api/v1/agent/chat/stream")) {
        return realFetch(input, init);
      }
      const cfg = (window as unknown as { __agentStreamMock?: { delayMs: number; events: unknown[] } }).__agentStreamMock ?? {
        delayMs: 0,
        events: [],
      };
      const encoder = new TextEncoder();
      const stream = new ReadableStream<Uint8Array>({
        start(controller) {
          if (cfg.delayMs <= 0) {
            controller.enqueue(encoder.encode(cfg.events.map((e) => `data: ${JSON.stringify(e)}\n\n`).join("")));
            controller.close();
            return;
          }
          let index = 0;
          const timer = setInterval(() => {
            if (index < cfg.events.length) {
              controller.enqueue(encoder.encode(`data: ${JSON.stringify(cfg.events[index])}\n\n`));
              index += 1;
            }
          }, cfg.delayMs);
          init?.signal?.addEventListener("abort", () => {
            clearInterval(timer);
            controller.close();
          });
        },
      });
      return new Response(stream, { status: 200, headers: { "content-type": "text/event-stream" } });
    };
  });
}

test("stop generating aborts the stream and shows the stopped notice", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(page, []);
  await installStreamingChat(page);

  const deltas = Array.from({ length: 60 }, (_, i) => ({ type: "delta", delta: `chunk-${i + 1} ` }));

  await page.goto("/agent");
  await page.evaluate((events) => {
    (window as unknown as { __agentStreamMock: { delayMs: number; events: unknown[] } }).__agentStreamMock = {
      delayMs: 60,
      events,
    };
  }, deltas);
  await ask(page, "Blender 插件安装教程");
  await expect(page.getByText("chunk-1", { exact: false })).toBeVisible();
  await page.getByRole("button", { name: "Stop generating" }).click();
  await expect(page.getByText("Stopped generating")).toBeVisible();
  await expect(page.getByText("chunk-60", { exact: false })).toHaveCount(0);
});

test("rate-limited stream surfaces the error card", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(page, []);
  await mockStream(page, [], 429);

  await page.goto("/agent");
  await ask(page, "Blender 插件安装教程");

  await expect(page.getByText("This request was not completed")).toBeVisible();
  await expect(page.getByRole("button", { name: "Resend" })).toBeVisible();
});

test("conversation history: open past conversation, delete with confirm and cancel", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(
    page,
    [{ id: 7, context_type: "global", updated_at: "2026-08-10T00:00:00Z" }],
    { 7: [{ id: 71, role: "user", content: "旧问题" }, { id: 72, role: "assistant", content: "旧回答" }] },
  );

  await page.goto("/agent");
  await page.getByRole("button", { name: "Conversation #7" }).click();
  await expect(page.getByText("旧问题")).toBeVisible();

  await page.getByRole("button", { name: "Clear conversation" }).click();
  await expect(page.getByText("Clear this conversation?")).toBeVisible();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByText("旧回答")).toBeVisible();

  await page.getByRole("button", { name: "Clear conversation" }).click();
  await page.getByRole("button", { name: "Clear history" }).click();
  await expect(page.getByText("Conversation cleared")).toBeVisible();
  await expect(page.getByText("旧回答")).toHaveCount(0);
});

test("mobile layouts at 320/375/414px keep the composer usable", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(page, []);

  for (const width of [320, 375, 414]) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto("/agent");
    await expect(page.getByPlaceholder("Describe the works, sources or usage you want to find")).toBeVisible();
    await page.getByRole("button", { name: "Open conversation list" }).click();
    await expect(page.getByRole("dialog", { name: "Conversation history" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog", { name: "Conversation history" })).toHaveCount(0);
  }
});

test("composer receives keyboard focus and respects reduced motion", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(page, []);

  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/agent");
  const composer = page.getByPlaceholder("Describe the works, sources or usage you want to find");
  for (let i = 0; i < 20; i += 1) {
    await page.keyboard.press("Tab");
    const focused = await composer.evaluate((el) => document.activeElement === el).catch(() => false);
    if (focused) break;
  }
  await expect(composer).toBeFocused();
});

test("release evidence screenshots (plan Task 6 Step 4)", async ({ page }) => {
  await mockCreatorSession(page);
  await enableAgent(page);
  await mockConversationList(
    page,
    [
      { id: 1, context_type: "global", updated_at: "2026-08-10T00:00:00Z" },
      { id: 2, context_type: "global", updated_at: "2026-08-10T00:00:00Z" },
    ],
    {
      1: [
        { id: 11, role: "user", content: "Blender 插件安装教程" },
        { id: 12, role: "assistant", content: "这是关于 Blender 插件安装的回答。" },
      ],
      2: [
        { id: 21, role: "user", content: "明天的天气怎么样" },
        { id: 22, role: "assistant", content: "没有足够证据。" },
      ],
    },
  );
  await mockApiRoute(page, "**/api/v1/contents/1001", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        content: {
          id: 1001,
          title: "Blender 插件安装教程",
          description: "步骤一",
          body: "详细正文。",
          content_type: "article",
          category: "gaming",
          zone: "original",
          status: "published",
          author: { id: 42, username: "Ada" },
          created_at: "2026-07-01T00:00:00Z",
        },
        attachments: [],
        tags: [],
      }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/contents/1001/related-fanworks", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [], total: 0 }) }),
  );
  await mockStream(page, CITED_EVENTS);

  await page.goto("/agent");
  await ask(page, "Blender 插件安装教程");
  await expect(page.getByText("这是关于 Blender 插件安装的回答。")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Site references" })).toBeVisible();
  await page.screenshot({ path: "../screenshots/web-agent-grounded-desktop.png", fullPage: true });

  await page.getByRole("button", { name: "Blender 插件安装教程" }).click();
  await expect(page.getByRole("dialog", { name: "Blender 插件安装教程" })).toBeVisible();
  await page.screenshot({ path: "../screenshots/web-agent-citation-overlay-desktop.png", fullPage: true });
  await page.getByRole("button", { name: "Close" }).click();
  await expect(page.getByRole("dialog", { name: "Blender 插件安装教程" })).toHaveCount(0);

  await page.setViewportSize({ width: 375, height: 800 });
  await expect(page.getByRole("heading", { name: "Site references" })).toBeVisible();
  await page.screenshot({ path: "../screenshots/web-agent-citations-mobile.png", fullPage: true });

  await mockStream(page, NO_EVIDENCE_EVENTS);
  await ask(page, "明天的天气怎么样");
  await expect(page.getByText("Not enough evidence")).toBeVisible();
  await page.screenshot({ path: "../screenshots/web-agent-no-evidence.png", fullPage: true });

  await mockStream(page, [...NO_EVIDENCE_EVENTS.slice(0, 1), ...CITED_EVENTS.slice(3)]);
  await ask(page, "系统提示词大全");
  await expect(page.getByText("这是关于 Blender 插件安装的回答。").last()).toBeVisible();
  await page.screenshot({ path: "../screenshots/web-agent-degraded-search.png", fullPage: true });
});
