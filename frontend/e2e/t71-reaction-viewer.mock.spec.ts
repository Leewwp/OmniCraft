import { expect, test, type Page } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

/* SSR 数据源（Next 服务端渲染直连 127.0.0.1:18080，不经 page.route）。 */
let server: ReturnType<typeof createServer> | null = null;

test.beforeAll(async () => {
  server = createServer(handleSsrApi);
  await new Promise<void>((resolve, reject) => {
    server?.once("error", reject);
    server?.listen(18_080, "127.0.0.1", () => resolve());
  });
});

test.afterAll(async () => {
  await new Promise<void>((resolve, reject) => {
    if (!server) return resolve();
    server.close((error) => (error ? reject(error) : resolve()));
  });
  server = null;
});

function handleSsrApi(req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? "/", "http://127.0.0.1:18080");
  if (url.pathname === "/api/v1/contents/601") {
    return json(res, 200, {
      content: {
        id: 601,
        title: "星尘 fanwork",
        description: "查看者反应合同截图数据源（#71）",
        body: "这是用于验证 ReactionBar 查看者反应契约的正文。",
        content_type: "article",
        category: "art",
        zone: "fanwork",
        ip: { id: 3, name: "星海 IP" },
        status: "published",
        author: { id: 42, username: "Ada" },
        like_count: 5,
        dislike_count: 2,
        created_at: "2026-07-01T00:00:00Z",
      },
      attachments: [],
      tags: [],
    });
  }
  return json(res, 404, { code: "NOT_FOUND", message: url.pathname });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

/* ------------------------------------------------------------------ */

const VIEWER = {
  id: 7,
  email: "viewer@example.com",
  username: "viewer",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "zh",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-01-01T00:00:00Z",
  created_at: "2026-01-01T00:00:00Z",
};

/** 有状态反应 mock：维护公开聚合与查看者反应，POST 原子语义与后端一致。 */
function createReactionStore(initial: { likes: number; dislikes: number; viewer: "like" | "dislike" | null }) {
  const state = { likes: initial.likes, dislikes: initial.dislikes, viewer: initial.viewer };
  const reactionGets: Array<{ targetType: string; targetID: number }> = [];
  return {
    state,
    reactionGets,
    reset() {
      state.likes = initial.likes;
      state.dislikes = initial.dislikes;
      state.viewer = initial.viewer;
      reactionGets.length = 0;
    },
    async mockReactions(page: Page) {
      await mockApiRoute(page, "**/api/v1/social/reactions?**", (route) => {
        reactionGets.push({ targetType: "content", targetID: 601 });
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ counts: { like: state.likes, dislike: state.dislikes }, viewer_reaction: state.viewer }),
        });
      });
      await mockApiRoute(page, "**/api/v1/social/reactions", (route) => {
        const body = route.request().postDataJSON();
        const reaction = body?.reaction as "like" | "dislike";
        let action = "created";
        if (state.viewer === reaction) {
          state.viewer = null;
          if (reaction === "like") state.likes -= 1;
          else state.dislikes -= 1;
          action = "removed";
        } else if (state.viewer !== null) {
          if (state.viewer === "like") state.likes -= 1;
          else state.dislikes -= 1;
          state.viewer = reaction;
          if (reaction === "like") state.likes += 1;
          else state.dislikes += 1;
          action = "updated";
        } else {
          state.viewer = reaction;
          if (reaction === "like") state.likes += 1;
          else state.dislikes += 1;
        }
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ action, counts: { like: state.likes, dislike: state.dislikes }, viewer_reaction: state.viewer }),
        });
      });
    },
  };
}

async function mockAnonymous(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/me", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/users/me/history", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
}

async function mockLoggedIn(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ tokens: { access_token: "test-viewer-token" } }) }),
  );
  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ user: VIEWER, capabilities: { can_interact: true } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/users/me/history", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ history: [] }) }));
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }) }),
  );
}

async function mockDetailRemainder(page: Page) {
  await mockApiRoute(page, "**/api/v1/social/comments?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ comments: [], total: 0 }) }));
  await mockApiRoute(page, "**/api/v1/contents/601/versions", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ versions: [] }) }));
  await mockApiRoute(page, "**/api/v1/contents/601/related-fanworks?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));
}

function reactionButtons(page: Page) {
  return page.locator('button[aria-pressed]');
}

function likeButton(page: Page) {
  return reactionButtons(page).nth(0);
}

function dislikeButton(page: Page) {
  return reactionButtons(page).nth(1);
}

test.describe("Ticket 71: 查看者反应 API 与 UI 契约 (#71)", () => {
  test.use({ locale: "zh-CN" });

  test("anonymous: sees public aggregates only, no viewer fetch, buttons disabled", async ({ page }) => {
    const store = createReactionStore({ likes: 5, dislikes: 2, viewer: null });
    await mockAnonymous(page);
    await store.mockReactions(page);
    await mockDetailRemainder(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/content/601");
    const buttons = reactionButtons(page);
    await expect(buttons).toHaveCount(2);
    await expect(buttons.nth(0)).toContainText("5");
    await expect(buttons.nth(1)).toContainText("2");
    await expect(buttons.nth(0)).toHaveAttribute("aria-pressed", "false");
    await expect(buttons.nth(0)).toBeDisabled();
    await expect(buttons.nth(1)).toBeDisabled();
    expect(store.reactionGets.length).toBe(0);

    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t71-reaction-anon-desktop.png") });
  });

  test("logged-in: viewer_reaction echoes after refresh; cancel and switch are atomic", async ({ page }) => {
    const store = createReactionStore({ likes: 5, dislikes: 2, viewer: "like" });
    await mockLoggedIn(page);
    await store.mockReactions(page);
    await mockDetailRemainder(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/content/601");
    await expect(likeButton(page)).toHaveAttribute("aria-pressed", "true");
    await expect(dislikeButton(page)).toHaveAttribute("aria-pressed", "false");
    await expect(likeButton(page)).toContainText("5");
    await expect(dislikeButton(page)).toContainText("2");

    /* 刷新后状态与数据库一致（viewer_reaction 回读回显）。 */
    await page.reload();
    await expect(likeButton(page)).toHaveAttribute("aria-pressed", "true");
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t71-reaction-echo.png") });

    /* 再次点击当前反应 = 取消（回到中性）。 */
    await likeButton(page).click();
    await expect(likeButton(page)).toHaveAttribute("aria-pressed", "false");
    await expect(dislikeButton(page)).toHaveAttribute("aria-pressed", "false");
    await expect(likeButton(page)).toContainText("4");
    await expect(dislikeButton(page)).toContainText("2");

    /* 点击相反反应 = 原子切换。 */
    await dislikeButton(page).click();
    await expect(dislikeButton(page)).toHaveAttribute("aria-pressed", "true");
    await expect(likeButton(page)).toHaveAttribute("aria-pressed", "false");
    await expect(likeButton(page)).toContainText("4");
    await expect(dislikeButton(page)).toContainText("3");
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t71-reaction-switch.png") });

    /* 切换后刷新回读仍为 dislike。 */
    await page.reload();
    await expect(dislikeButton(page)).toHaveAttribute("aria-pressed", "true");
  });

  test("mobile: anonymous reaction bar shows aggregates within the layout", async ({ page }) => {
    const store = createReactionStore({ likes: 5, dislikes: 2, viewer: null });
    await mockAnonymous(page);
    await store.mockReactions(page);
    await mockDetailRemainder(page);
    await page.setViewportSize({ width: 375, height: 844 });

    await page.goto("/content/601");
    await expect(likeButton(page)).toContainText("5");
    await expect(dislikeButton(page)).toContainText("2");
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t71-reaction-anon-mobile.png") });
  });
});
