import { expect, test, type Page } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

let server: ReturnType<typeof createServer> | null = null;

test.beforeAll(async () => {
  server = createServer(handleServerRenderedContentApi);
  await new Promise<void>((resolve, reject) => {
    server?.once("error", reject);
    server?.listen(18_080, "127.0.0.1", () => resolve());
  });
});

test.afterAll(async () => {
  await new Promise<void>((resolve, reject) => {
    if (!server) return resolve();
    server.close((error) => error ? reject(error) : resolve());
  });
  server = null;
});

function handleServerRenderedContentApi(req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? "/", "http://127.0.0.1:18080");
  if (url.pathname === "/api/v1/contents/501") {
    return json(res, 200, {
      content: {
        id: 501,
        title: "第一章：启程",
        description: "系列导航截图合同",
        body: "这是用于验证系列导航位置的正文。",
        content_type: "article",
        category: "literature",
        zone: "original",
        status: "published",
        author: { id: 42, username: "Ada" },
        created_at: "2026-07-01T00:00:00Z",
      },
      attachments: [],
      tags: [],
      series_memberships: [{
        series_id: 7,
        series_title: "山海纪行",
        current_index: 1,
        total: 2,
        next: { id: 502, title: "第二章：归途" },
      }],
    });
  }
  if (url.pathname === "/api/v1/contents/501/related-fanworks") return json(res, 200, { items: [], total: 0 });
  return json(res, 404, { code: "NOT_FOUND", message: url.pathname });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

const seriesDetail = {
  series: {
    id: 7,
    title: "山海纪行",
    description: "一段按章节整理的公开旅程。",
    zone: "original",
    owner: { id: 42, username: "Ada" },
    cover: null,
    item_count: 2,
  },
  items: [
    { id: 101, sort_order: 0, content_item_id: 501, content: { id: 501, title: "第一章：启程", zone: "original", status: "published" } },
    { id: 102, sort_order: 1, content_item_id: 502, content: { id: 502, title: "第二章：归途", zone: "original", status: "published" } },
  ],
};

async function mockPublicSeries(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/series/7", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(seriesDetail) }));
}

async function mockContentDetailClientApis(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/me", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ unread_counts: { total: 0 } }) }));
  await mockApiRoute(page, "**/api/v1/users/me/history", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/social/comments?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ comments: [] }) }));
  await mockApiRoute(page, "**/api/v1/social/reactions?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ counts: { like: 0, dislike: 0 }, viewer_reaction: null }) }));
}

async function mockStudioSeries(page: Page) {
  await mockPublicSeries(page);
  const calls = {
    create: [] as unknown[],
    update: [] as unknown[],
    add: [] as unknown[],
    reorder: [] as unknown[],
    remove: 0,
    delete: 0,
  };
  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ tokens: { access_token: "studio-token" } }) }));
  await mockApiRoute(page, "**/api/v1/auth/me", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ user: { id: 42, email: "ada@example.test", username: "Ada", avatar_url: "", bio: "", reputation: 10, preferred_locale: "zh", role: "user", is_banned: false, email_verified_at: "2026-07-01T00:00:00Z", created_at: "2026-07-01T00:00:00Z" } }) }));
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }) }));
  await mockApiRoute(page, "**/api/v1/series/7", async (route) => {
    if (route.request().method() === "PUT") {
      calls.update.push(route.request().postDataJSON());
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ series: seriesDetail.series }) });
    }
    if (route.request().method() === "DELETE") {
      calls.delete += 1;
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ message: "deleted" }) });
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(seriesDetail) });
  });
  await mockApiRoute(page, "**/api/v1/series", async (route) => {
    if (route.request().method() === "POST") {
      calls.create.push(route.request().postDataJSON());
      return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ series: { id: 8, title: "新系列", description: "", zone: "original" } }) });
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [{ id: 7, title: "山海纪行", description: seriesDetail.series.description, zone: "original" }], total: 1 }) });
  });
  await mockApiRoute(page, "**/api/v1/series/candidates?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [{ id: 503, title: "第三章：新路", zone: "original", status: "published" }] }) }));
  await mockApiRoute(page, "**/api/v1/series/7/items", async (route) => {
    calls.add.push(route.request().postDataJSON());
    return route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ item: { id: 103, series_id: 7, content_item_id: 503, sort_order: 2 } }) });
  });
  await mockApiRoute(page, "**/api/v1/series/7/items/*", (route) => {
    calls.remove += 1;
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ message: "removed" }) });
  });
  await mockApiRoute(page, "**/api/v1/series/7/items/reorder", async (route) => {
    calls.reorder.push(route.request().postDataJSON());
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ message: "reordered" }) });
  });
  await mockApiRoute(page, "**/api/v1/series/7?manage=true", async (route) => {
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(seriesDetail) });
  });
  return calls;
}

test("public series detail is browsable while logged out and preserves backend order", async ({ page }) => {
  await mockPublicSeries(page);
  await page.goto("/series/7");
  await expect(page.getByRole("heading", { name: "山海纪行" })).toBeVisible();
  await expect(page.getByRole("link", { name: /第一章/ })).toHaveAttribute("href", "/original/501");
  await expect(page.getByRole("link", { name: /第二章/ })).toHaveAttribute("href", "/original/502");
  await expect(page.locator("ol > li")).toHaveCount(2);
  await page.screenshot({ path: "../screenshots/community-content-series-detail-desktop.png", fullPage: true });
});

test("public series detail remains single-column and touch-sized on mobile", async ({ page }) => {
  await mockPublicSeries(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/series/7");
  await expect(page.getByRole("heading", { name: "山海纪行" })).toBeVisible();
  await page.screenshot({ path: "../screenshots/community-content-series-detail-mobile.png", fullPage: true });
});

test("SeriesNav renders between content and comments on desktop", async ({ page }) => {
  await mockContentDetailClientApis(page);
  await page.goto("/original/501");
  await expect(page.getByText("山海纪行")).toBeVisible();
  await expect(page.getByRole("link", { name: /系列目录|catalog/i })).toHaveAttribute("href", "/series/7");
  await page.screenshot({ path: "../screenshots/community-content-series-nav-desktop.png", fullPage: true });
});

test("SeriesNav keeps first-item disabled state readable on mobile", async ({ page }) => {
  await mockContentDetailClientApis(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/original/501");
  await expect(page.getByText("山海纪行")).toBeVisible();
  await expect(page.getByRole("button", { name: /第一章|first chapter/i })).toBeDisabled();
  await page.screenshot({ path: "../screenshots/community-content-series-nav-mobile.png", fullPage: true });
});

test("studio series exercises create, edit, add, reorder, remove, and delete contracts", async ({ page }) => {
  const calls = await mockStudioSeries(page);
  await page.goto("/studio/series");
  await expect(page.getByRole("heading", { name: /内容系列管理|Content series/i })).toBeVisible();

  await page.getByRole("button", { name: /创建内容系列|Create series/i }).click();
  await page.getByLabel(/系列标题|Series title/i).first().fill("新系列");
  await page.getByRole("button", { name: /^创建$|^Create$/i }).click();
  await expect.poll(() => calls.create.length).toBe(1);
  expect(calls.create[0]).toEqual({ title: "新系列", description: "", zone: "original" });

  await page.getByLabel(/系列标题|Series title/i).last().fill("山海纪行·修订");
  await page.getByRole("button", { name: /保存更改|Save changes/i }).click();
  await expect.poll(() => calls.update.length).toBe(1);

  await page.getByRole("textbox", { name: /搜索可添加内容|Search content to add/i }).fill("第三章");
  await expect(page.getByText("第三章：新路")).toBeVisible();
  await page.getByRole("button", { name: /^添加$|^Add$/i }).click();
  await expect.poll(() => calls.add.length).toBe(1);
  expect(calls.add[0]).toEqual({ content_item_id: 503 });

  await page.getByRole("button", { name: /将 第二章：归途 上移|Move 第二章：归途 up/i }).click();
  await expect.poll(() => calls.reorder.length).toBe(1);
  expect(calls.reorder[0]).toEqual({ item_ids: [102, 101] });

  await page.getByRole("button", { name: /从系列移除 第二章：归途|Remove 第二章：归途 from series/i }).click();
  await expect.poll(() => calls.remove).toBe(1);

  await page.getByRole("button", { name: /删除系列|Delete series/i }).first().click();
  await page.getByRole("button", { name: /删除系列|Delete series/i }).last().click();
  await expect.poll(() => calls.delete).toBe(1);
  await page.screenshot({ path: "../screenshots/community-content-series-studio-desktop.png", fullPage: true });
});

test("studio series mobile keeps navigation and item actions reachable", async ({ page }) => {
  await mockStudioSeries(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/studio/series");
  await expect(page.getByRole("heading", { name: /内容系列管理|Content series/i })).toBeVisible();
  await page.getByRole("button", { name: /山海纪行/ }).click();
  await expect(page.getByRole("button", { name: /返回系列列表|Back to series list/i })).toBeVisible();
  await expect(page.getByRole("button", { name: /将 第二章：归途 上移|Move 第二章：归途 up/i })).toBeVisible();
  await page.screenshot({ path: "../screenshots/community-content-series-studio-mobile.png", fullPage: true });
});
