import { expect, test, type Page } from "@playwright/test";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

const USER = {
  id: 1,
  email: "alice@example.test",
  username: "alice",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "zh",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-06-30T00:00:00Z",
  created_at: "2026-06-30T00:00:00Z",
  accept_collab_invites: true,
};

const INVITE_MESSAGE = {
  id: 20,
  sender_id: 2,
  msg_type: "collab_invite",
  text: "联合创作邀请",
  body: "联合创作邀请",
  metadata: {
    invite_id: 7,
    content_id: 601,
    content_title: "星尘 fanwork",
    inviter_id: 2,
    inviter_username: "bob",
  },
  created_at: "2026-06-30T12:10:00Z",
};

const TEXT_MESSAGE = {
  id: 10,
  sender_id: 2,
  text: "你好，很高兴认识你",
  body: "你好，很高兴认识你",
  created_at: "2026-06-30T12:01:00Z",
};

const CONVERSATION = {
  id: 42,
  participants: [
    { id: 1, username: "alice", avatar_url: "" },
    { id: 2, username: "bob", avatar_url: "" },
  ],
  last_message: {
    id: 20,
    text: "联合创作邀请",
    sender_id: 2,
    msg_type: "collab_invite",
    created_at: "2026-06-30T12:10:00Z",
  },
  unread_count: 1,
  updated_at: "2026-06-30T12:10:00Z",
};

async function mockCollabSession(page: Page, options: { accept?: { status?: number; body?: unknown }; decline?: { status?: number; body?: unknown }; patchFails?: boolean } = {}) {
  await mockPublicApis(page);
  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", path: "/", domain: "127.0.0.1" }]);

  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-user-token" } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ user: USER }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/notifications", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ notifications: [], page: 1, page_size: 20 }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/messages", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ conversations: [CONVERSATION], page: 1, page_size: 20 }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/messages/42", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ messages: [INVITE_MESSAGE, TEXT_MESSAGE], total: 2 }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/collab-invites/7/accept", (route) => {
    const response = options.accept ?? { status: 200, body: { invite: { id: 7, status: "accepted" } } };
    route.fulfill({ status: response.status ?? 200, contentType: "application/json", body: JSON.stringify(response.body) });
  });

  await mockApiRoute(page, "**/api/v1/collab-invites/7/decline", (route) => {
    const response = options.decline ?? { status: 200, body: { invite: { id: 7, status: "declined" } } };
    route.fulfill({ status: response.status ?? 200, contentType: "application/json", body: JSON.stringify(response.body) });
  });

  await mockApiRoute(page, "**/api/v1/users/1", async (route) => {
    if (options.patchFails) {
      route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ code: "DB_ERROR", message: "db error" }) });
      return;
    }
    // Defer the save so the optimistic switch-off is observable before
    // refreshUser() restores the mocked preference; an instant fulfill lets
    // the whole toggle handler finish inside one render pass.
    await new Promise((resolve) => setTimeout(resolve, 300));
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({}) });
  });
}

async function openChatWithInvite(page: Page) {
  await page.goto("/messages");
  await page.getByRole("tab", { name: "私信" }).click();
  await page.getByRole("button", { name: /bob/ }).first().click();
}

async function gotoDesktop(page: Page) {
  await page.setViewportSize({ width: 1280, height: 800 });
}

test("pending invite card renders with accept and decline actions", async ({ page }) => {
  await mockCollabSession(page);
  await gotoDesktop(page);
  await openChatWithInvite(page);

  const card = page.getByRole("group");
  await expect(card).toBeVisible();
  await expect(card.getByText("bob 邀请你联合创作《星尘 fanwork》")).toBeVisible();
  await expect(page.getByRole("link", { name: "星尘 fanwork" })).toBeVisible();
  await expect(card.getByText("待接受")).toBeVisible();
  await expect(page.getByRole("button", { name: "接受《星尘 fanwork》的联合创作邀请" })).toBeVisible();
  await expect(page.getByRole("button", { name: "拒绝《星尘 fanwork》的联合创作邀请" })).toBeVisible();
  await expect(page.getByRole("log").getByText("你好，很高兴认识你")).toBeVisible();

  await page.screenshot({ path: path.join(SCREENSHOTS, "community-collab-invite-pending.png"), fullPage: false });
});

test("accepting an invite switches the card to the accepted read-only state", async ({ page }) => {
  await mockCollabSession(page, { accept: { body: { invite: { id: 7, status: "accepted" } } } });
  await gotoDesktop(page);
  await openChatWithInvite(page);

  await page.getByRole("button", { name: "接受《星尘 fanwork》的联合创作邀请" }).click();
  await expect(page.getByText("已接受")).toBeVisible();
  await expect(page.getByRole("button", { name: /接受《星尘 fanwork》的联合创作邀请/ })).toHaveCount(0);

  await page.screenshot({ path: path.join(SCREENSHOTS, "community-collab-invite-states-accepted.png") });
});

test("declining an invite switches the card to the declined read-only state", async ({ page }) => {
  await mockCollabSession(page, { decline: { body: { invite: { id: 7, status: "declined" } } } });
  await gotoDesktop(page);
  await openChatWithInvite(page);

  await page.getByRole("button", { name: "拒绝《星尘 fanwork》的联合创作邀请" }).click();
  await expect(page.getByText("已拒绝")).toBeVisible();
  await expect(page.getByRole("button", { name: /接受《星尘 fanwork》的联合创作邀请/ })).toHaveCount(0);

  await page.screenshot({ path: path.join(SCREENSHOTS, "community-collab-invite-states-declined.png") });
});

test("expired invite renders as muted read-only state", async ({ page }) => {
  await mockCollabSession(page, { accept: { body: { invite: { id: 7, status: "expired" } } } });
  await gotoDesktop(page);
  await openChatWithInvite(page);

  await page.getByRole("button", { name: "接受《星尘 fanwork》的联合创作邀请" }).click();
  await expect(page.getByText("已过期")).toBeVisible();
  await expect(page.getByRole("button", { name: /接受《星尘 fanwork》的联合创作邀请/ })).toHaveCount(0);

  await page.screenshot({ path: path.join(SCREENSHOTS, "community-collab-invite-states-expired.png") });
});

test("failed accept keeps the card pending with inline error and toast", async ({ page }) => {
  await mockCollabSession(page, {
    accept: { status: 409, body: { code: "INVITE_ALREADY_RESPONDED", message: "already responded" } },
  });
  await gotoDesktop(page);
  await openChatWithInvite(page);

  await page.getByRole("button", { name: "接受《星尘 fanwork》的联合创作邀请" }).click();
  await expect(page.locator('p[role="alert"]')).toContainText("操作失败，请重试");
  await expect(page.locator('div[role="alert"]').filter({ hasText: "操作失败，请重试" })).toBeVisible();
  await expect(page.getByRole("button", { name: "接受《星尘 fanwork》的联合创作邀请" })).toBeVisible();
  await expect(page.getByRole("group").getByText("待接受")).toBeVisible();
});

test("mobile invite card keeps 44px action buttons", async ({ page }) => {
  await mockCollabSession(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await openChatWithInvite(page);

  const acceptButton = page.getByRole("button", { name: "接受《星尘 fanwork》的联合创作邀请" });
  await expect(acceptButton).toBeVisible();
  const box = await acceptButton.boundingBox();
  expect(box?.height).toBeGreaterThanOrEqual(44);

  await page.screenshot({ path: path.join(SCREENSHOTS, "community-collab-invite-mobile.png") });
});

test("settings collaboration switch reflects the user preference and saves", async ({ page }) => {
  await mockCollabSession(page);
  await gotoDesktop(page);
  await page.goto("/settings");

  const toggle = page.getByRole("switch");
  await expect(toggle).toBeVisible();
  await expect(toggle).toHaveAttribute("aria-checked", "true");
  await expect(page.getByText("接收联合创作邀请")).toBeVisible();
  await expect(page.getByText("关闭后其他用户无法向你发送联合创作邀请。")).toBeVisible();

  await page.screenshot({ path: path.join(SCREENSHOTS, "community-collab-settings-desktop.png") });

  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-checked", "false");
  await expect(page.getByText("设置已保存")).toBeVisible();
  await expect(toggle).toHaveAttribute("aria-checked", "true");
});

test("failed settings save rolls the switch back with localized feedback", async ({ page }) => {
  await mockCollabSession(page, { patchFails: true });
  await gotoDesktop(page);
  await page.goto("/settings");

  const toggle = page.getByRole("switch");
  await expect(toggle).toHaveAttribute("aria-checked", "true");
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-checked", "true");
  await expect(page.locator('p[role="alert"]')).toContainText("保存失败，请重试");
  await expect(page.locator('div[role="alert"]').filter({ hasText: "保存失败，请稍后重试" })).toBeVisible();
});

test("mobile settings shows the collaboration switch", async ({ page }) => {
  await mockCollabSession(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/settings");

  const toggle = page.getByRole("switch");
  await expect(toggle).toBeVisible();
  await expect(toggle).toHaveAttribute("aria-checked", "true");

  await page.screenshot({ path: path.join(SCREENSHOTS, "community-collab-settings-mobile.png") });
});
