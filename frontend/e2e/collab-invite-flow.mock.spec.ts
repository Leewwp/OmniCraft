import { expect, test, type Page } from "@playwright/test";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

const CREATOR = {
  id: 5,
  email: "creator@example.com",
  username: "creator",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "zh",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-01-01T00:00:00Z",
  created_at: "2026-01-01T00:00:00Z",
};

const USER_RESULTS = [
  { id: 1, username: "Luminary", avatar_url: "https://cdn.example.test/1.png", reputation: 10, role: "creator" },
  { id: 2, username: "Lumina", avatar_url: "", reputation: 8, role: "user" },
  { id: 3, username: "Lumos", avatar_url: "", reputation: 6, role: "user" },
];

function removeButton(page: Page, username: string) {
  return page.getByRole("button", { name: `移除 ${username}`, exact: true });
}

/** Auth + studio session with the collaboration cap exposed. The config
 *  handler is registered after mockPublicApis so it wins (page.route is
 *  LIFO) and adds collaboration.max_invitees_per_publish. */
async function mockCollabPublishSession(page: Page) {
  await mockPublicApis(page);

  await mockApiRoute(page, "**/api/v1/config/public", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        features: {
          web_agent_enabled: false,
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
        collaboration: { max_invitees_per_publish: 5 },
      }),
    }),
  );

  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", path: "/", domain: "127.0.0.1" }]);

  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-creator-token" } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ user: CREATOR }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/ips?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ips: [{ id: 42, name: "星穹铁道" }] }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/users/search?**", async (route) => {
    const url = new URL(route.request().url());
    const q = url.searchParams.get("q") ?? "";
    const users = q.trim() === "" ? [] : USER_RESULTS;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ users, total: users.length }),
    });
  });
}

async function mockCreateContent(page: Page, contentCreates: unknown[]) {
  await mockApiRoute(page, "**/api/v1/contents", async (route) => {
    if (route.request().method() !== "POST") {
      await route.fallback();
      return;
    }
    contentCreates.push(route.request().postDataJSON());
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({ content: { id: 123 } }),
    });
  });
}

async function mockCollabInvites(page: Page, inviteBodies: Array<{ invitee_id: number }>, failFor?: Set<number>) {
  await mockApiRoute(page, "**/api/v1/contents/*/collab-invites", async (route) => {
    if (route.request().method() !== "POST") {
      await route.fallback();
      return;
    }
    const body = route.request().postDataJSON() as { invitee_id: number };
    inviteBodies.push(body);
    if (failFor?.has(body.invitee_id)) {
      await route.fulfill({
        status: 403,
        contentType: "application/json",
        body: JSON.stringify({ code: "INVITE_BLOCKED", message: "blocked" }),
      });
      return;
    }
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({ invite: { id: body.invitee_id, status: "pending" } }),
    });
  });
}

async function pickCollaborator(page: Page, username: string) {
  const input = page.getByRole("combobox", { name: "联合创作者" });
  await input.fill("lum");
  await page.getByRole("option", { name: username, exact: true }).click();
}

async function openOriginalForm(page: Page) {
  await page.goto("/studio/publish/original");
  await page.getByRole("button", { name: /文章/ }).click();
}

async function openFanworkForm(page: Page) {
  await page.goto("/studio/publish/fanwork");
  await page.getByRole("button", { name: /文章/ }).click();
}

async function pickIp(page: Page) {
  await page.getByPlaceholder("搜索并选择 IP...").fill("星穹");
  await page.getByRole("button", { name: "星穹铁道", exact: true }).click();
}

test("t7 collab publish: original with collaborators sends one invite per selected user", async ({ page }) => {
  await mockCollabPublishSession(page);
  const contentCreates: unknown[] = [];
  const inviteBodies: Array<{ invitee_id: number }> = [];
  await mockCreateContent(page, contentCreates);
  await mockCollabInvites(page, inviteBodies);

  await openOriginalForm(page);
  await page.getByPlaceholder("输入作品标题").fill("星尘原创");
  await page.getByRole("button", { name: "影视", exact: true }).click();
  await pickCollaborator(page, "Luminary");
  await pickCollaborator(page, "Lumina");
  await page.screenshot({ path: path.join(SCREENSHOTS, "t7-collab-publish-original.png"), fullPage: true });

  await page.getByRole("button", { name: /^发布创作$/i }).click();

  await expect.poll(() => contentCreates.length).toBe(1);
  expect(contentCreates[0]).toMatchObject({ zone: "original", content_type: "article" });
  expect(contentCreates[0]).not.toHaveProperty("collaborators");
  expect(contentCreates[0]).not.toHaveProperty("invitee_ids");

  await expect.poll(() => inviteBodies.length).toBe(2);
  expect(inviteBodies).toEqual([{ invitee_id: 1 }, { invitee_id: 2 }]);

  await expect(page.getByText("发布成功！", { exact: true })).toBeVisible();
  await expect(page).toHaveURL(/\/studio\/contents$/);
});

test("t7 collab publish: fanwork with IP and collaborators renders the picker after the source fields", async ({ page }) => {
  await mockCollabPublishSession(page);
  const contentCreates: unknown[] = [];
  const inviteBodies: Array<{ invitee_id: number }> = [];
  await mockCreateContent(page, contentCreates);
  await mockCollabInvites(page, inviteBodies);

  await openFanworkForm(page);
  await page.getByPlaceholder("输入作品标题").fill("星尘二创");
  await pickIp(page);
  await pickCollaborator(page, "Luminary");

  // Both boxes are measured at the same scroll position (the fanwork page
  // scrolls the form while interacting), so the y comparison stays valid.
  const sourceFieldset = page.locator("fieldset");
  await expect(sourceFieldset).toBeVisible();
  const sourceBox = await sourceFieldset.boundingBox();
  expect(sourceBox, "fanwork source fieldset must have a bounding box").not.toBeNull();

  const pickerBox = await page.getByRole("combobox", { name: "联合创作者" }).boundingBox();
  expect(pickerBox, "collaborator picker must have a bounding box").not.toBeNull();
  expect(
    pickerBox!.y >= sourceBox!.y + sourceBox!.height - 1,
    "picker must render below the fanwork source fieldset",
  ).toBe(true);
  await page.screenshot({ path: path.join(SCREENSHOTS, "t7-collab-publish-fanwork.png"), fullPage: true });

  await page.getByRole("button", { name: /^发布创作$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(1);
  expect(contentCreates[0]).toMatchObject({ zone: "fanwork", ip_id: 42 });
  await expect.poll(() => inviteBodies.length).toBe(1);
  expect(inviteBodies).toEqual([{ invitee_id: 1 }]);
  await expect(page.getByText("发布成功！", { exact: true })).toBeVisible();
  await expect(page).toHaveURL(/\/studio\/contents$/);
});

test("t7 collab publish: a failed invite shows a warning toast without failing the publish", async ({ page }) => {
  await mockCollabPublishSession(page);
  const contentCreates: unknown[] = [];
  const inviteBodies: Array<{ invitee_id: number }> = [];
  await mockCreateContent(page, contentCreates);
  await mockCollabInvites(page, inviteBodies, new Set([1]));

  await openFanworkForm(page);
  await page.getByPlaceholder("输入作品标题").fill("邀请失败二创");
  await pickIp(page);
  await pickCollaborator(page, "Luminary");
  await pickCollaborator(page, "Lumina");

  await page.getByRole("button", { name: /^发布创作$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(1);
  await expect.poll(() => inviteBodies.length).toBe(2);

  const warning = page.locator('div[role="alert"]').filter({ hasText: "Luminary" });
  await expect(warning).toBeVisible();
  await expect(warning).toContainText("联合创作邀请发送失败");
  await expect(warning).toContainText("/content/123");
  await expect(page.getByText("发布成功！", { exact: true })).toBeVisible();
  await expect(page).toHaveURL(/\/studio\/contents$/);
  await page.screenshot({ path: path.join(SCREENSHOTS, "t7-collab-invite-failed-toast.png"), fullPage: false });
});

test.describe("mobile", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });

  test("t7 collab publish mobile: chips wrap and removal targets stay at least 44px tall without horizontal overflow", async ({ page }) => {
    await mockCollabPublishSession(page);
    await openFanworkForm(page);
    await page.getByPlaceholder("输入作品标题").fill("移动端二创");
    await pickIp(page);
    for (const username of ["Luminary", "Lumina", "Lumos"]) {
      await pickCollaborator(page, username);
    }

    for (const username of ["Luminary", "Lumina", "Lumos"]) {
      const remove = removeButton(page, username);
      await expect(remove).toBeVisible();
      const box = await remove.boundingBox();
      expect(box, `remove button for ${username} should have a bounding box`).not.toBeNull();
      expect(box!.height, `remove button for ${username} should be at least 44px tall on coarse pointers`).toBeGreaterThanOrEqual(44);
    }

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);

    await page.screenshot({ path: path.join(SCREENSHOTS, "t7-collab-publish-mobile.png"), fullPage: true });
  });
});
