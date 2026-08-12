import { expect, test, type Page } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

/* SSR 数据源（Next 服务端渲染直连 127.0.0.1:18080，不经 page.route）。 */
let server: ReturnType<typeof createServer> | null = null;
let detailPayload: Record<string, unknown>;

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
    return json(res, 200, detailPayload);
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

function contentDetail(overrides: Record<string, unknown> = {}) {
  return {
    content: {
      id: 601,
      title: "星尘 fanwork",
      description: "#74 收藏成员关系状态截图数据源",
      body: "用于验证收藏状态以收藏成员关系为唯一事实源的正文。",
      content_type: "article",
      category: "art",
      zone: "fanwork",
      status: "published",
      author: { id: 42, username: "Ada" },
      like_count: 3,
      dislike_count: 1,
      created_at: "2026-07-01T00:00:00Z",
    },
    attachments: [],
    tags: [],
    series_memberships: [],
    ...overrides,
  };
}

/** 有状态收藏集 mock：与后端一致的成员关系语义 —— 内容属于任一活动收藏集即已收藏。 */
function createCollectionStore(initial: Array<{ id: number; title: string; contains: boolean }>) {
  const collections = initial.map((entry) => ({ ...entry }));
  return {
    async mock(page: Page) {
      await mockApiRoute(page, "**/api/v1/collections?**", (route) =>
        route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            items: collections.map((entry) => ({
              id: entry.id,
              user_id: 7,
              title: entry.title,
              description: "",
              zone: "fanwork",
              is_public: false,
              is_default: false,
              item_count: entry.contains ? 1 : 0,
              contains_item: entry.contains,
              ...(entry.contains ? { item_id: entry.id * 100 + 1 } : {}),
            })),
            total: collections.length,
          }),
        }),
      );
      await mockApiRoute(page, "**/api/v1/collections/*/items", (route) => {
        const match = /\/collections\/(\d+)\/items$/.exec(route.request().url());
        const collectionId = match ? Number(match[1]) : 0;
        const entry = collections.find((item) => item.id === collectionId);
        if (!entry) {
          return route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ code: "COLLECTION_NOT_FOUND" }) });
        }
        entry.contains = true;
        return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ item: { id: entry.id * 100 + 1, collection_id: entry.id, content_item_id: 601 } }) });
      });
      await mockApiRoute(page, "**/api/v1/collections/*/items/*", (route) => {
        const match = /\/collections\/(\d+)\/items\/(\d+)$/.exec(route.request().url());
        const collectionId = match ? Number(match[1]) : 0;
        const entry = collections.find((item) => item.id === collectionId);
        if (!entry) {
          return route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ code: "COLLECTION_NOT_FOUND" }) });
        }
        entry.contains = false;
        return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ message: "removed" }) });
      });
    },
    isMember() {
      return collections.some((entry) => entry.contains);
    },
  };
}

async function mockAnonymous(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/me", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
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
  await mockApiRoute(page, "**/api/v1/social/reactions?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ counts: { like: 3, dislike: 1 }, viewer_reaction: null }) }));
  await mockApiRoute(page, "**/api/v1/contents/601/versions", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ versions: [] }) }));
  await mockApiRoute(page, "**/api/v1/contents/601/related-fanworks?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));
}

test.describe("Ticket 74: 收藏状态以收藏成员关系为唯一事实源 (#74)", () => {
  test.use({ locale: "zh-CN" });

  test("anonymous: no favorited state, entry offers adding to a collection", async ({ page }) => {
    detailPayload = contentDetail();
    await mockAnonymous(page);
    await mockDetailRemainder(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/content/601");

    const action = page.getByRole("button", { name: "添加到收藏集" });
    await expect(action).toBeVisible();
    await expect(action).toBeDisabled();
    await expect(page.getByRole("button", { name: "已收藏" })).toHaveCount(0);

    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t74-anonymous-not-favorited.png") });
  });

  test("member of two collections: removing one keeps favorited, removing the last cancels it", async ({ page }) => {
    detailPayload = contentDetail({ is_favorited: true });
    const store = createCollectionStore([
      { id: 11, title: "星尘研究", contains: true },
      { id: 12, title: "灵感收藏", contains: true },
    ]);
    await mockLoggedIn(page);
    await store.mock(page);
    await mockDetailRemainder(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/content/601");

    const favoritedButton = page.getByRole("button", { name: "已收藏" });
    await expect(favoritedButton).toBeVisible();
    await expect(page.getByRole("button", { name: "添加到收藏集" })).toHaveCount(0);

    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t74-multi-favorited.png") });

    // 打开选择器：两行均已添加，与详情「已收藏」同源一致。
    await favoritedButton.click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("已添加")).toHaveCount(2);
    await expect(dialog.getByRole("button", { name: /从 星尘研究 移除/ })).toBeVisible();
    await expect(dialog.getByRole("button", { name: /从 灵感收藏 移除/ })).toBeVisible();

    // 从多个收藏集之一移除：仍是「已收藏」。
    await dialog.getByRole("button", { name: /从 星尘研究 移除/ }).click();
    await expect(dialog.getByText("已添加")).toHaveCount(1);
    await expect(page.getByRole("button", { name: "已收藏" })).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t74-multi-after-one-removal.png") });

    // 从最后一个有效收藏集移除：详情状态取消，按钮回到「添加到收藏集」。
    await dialog.getByRole("button", { name: /从 灵感收藏 移除/ }).click();
    await expect(dialog.getByText("已添加")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "添加到收藏集" })).toBeVisible();
    await expect(page.getByRole("button", { name: "已收藏" })).toHaveCount(0);

    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t74-last-removal-cancels.png") });
  });
});
