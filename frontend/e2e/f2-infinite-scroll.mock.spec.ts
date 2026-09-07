import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import { test, expect, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

/* #410 F2 三瀑布流无限滚动合同（mocked）：首屏 12、滚动逐页追加、无重复、
   判尽终态、首屏内不级联追加、筛选切换重置回第 1 页且携带 page_size。 */

interface FeedItem {
  id: number;
  title: string;
  zone: "original" | "fanwork";
  content_type?: string;
  cover_width?: number;
  cover_height?: number;
  like_count?: number;
}

function makeItems(zone: "original" | "fanwork", count: number, prefix: string, offset = 0): FeedItem[] {
  return Array.from({ length: count }, (_, i) => ({
    id: offset + i + 1,
    title: `${prefix} ${offset + i + 1}`,
    zone,
    content_type: "image",
    cover_width: 600,
    cover_height: 800,
    like_count: 3,
  }));
}

/** 30 条数据集：page_size=12 → 12/12/6 三页。 */
const RECOMMEND_ITEMS = makeItems("fanwork", 30, "Rec item");
const FILM_TV_ITEMS = makeItems("original", 20, "Film item");

function respondPage(items: FeedItem[], page: number, pageSize: number) {
  const start = (page - 1) * pageSize;
  return {
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      contents: items.slice(start, start + pageSize),
      total: items.length,
      page,
      page_size: pageSize,
    }),
  };
}

let backendServer: ReturnType<typeof createServer> | null = null;

/** 真 18080 mock API（t87 同款架构）：SSR 首屏 fetch 无法被 page.route 拦截，
    须由真实 HTTP 服务供给；客户端追加请求经 guard 放行后同样落到这里。 */
function startBackend(): Promise<void> {
  return new Promise((resolve, reject) => {
    backendServer = createServer((req, res) => handleApi(req, res));
    backendServer.once("error", reject);
    backendServer.listen(18080, "127.0.0.1", () => resolve());
  });
}

function stopBackend(): Promise<void> {
  return new Promise((resolve, reject) => {
    if (!backendServer) return resolve();
    backendServer.close((error) => (error ? reject(error) : resolve()));
    backendServer = null;
  });
}

function handleApi(req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? "/", "http://127.0.0.1:18080");
  if (url.pathname === "/api/v1/contents" && req.method === "GET") {
    const page = Math.max(1, Number(url.searchParams.get("page") ?? "1"));
    const pageSize = Math.max(1, Number(url.searchParams.get("page_size") ?? "20"));
    const zone = url.searchParams.get("zone") ?? "";
    const category = url.searchParams.get("category") ?? "";
    const items = zone === "original" && category === "film_tv" ? FILM_TV_ITEMS : RECOMMEND_ITEMS;
    const start = (page - 1) * pageSize;
    return json(res, 200, {
      contents: items.slice(start, start + pageSize),
      total: items.length,
      page,
      page_size: pageSize,
    });
  }
  if (url.pathname === "/api/v1/stats/summary") {
    return json(res, 200, { summary: { users: 42, ips: 7, contents: RECOMMEND_ITEMS.length } });
  }
  return json(res, 404, { code: "NOT_FOUND", message: url.pathname });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

test.beforeAll(async () => {
  await startBackend();
});

test.afterAll(async () => {
  await stopBackend();
});

async function mockContentsApi(page: Page) {
  await mockPublicApis(page);
  /* 客户端 feed 请求由 page.route 兑付（浏览器直连 18080 会跨域被拦，
    CORS 不适用于 route 兑付；SSR 首屏则走 beforeAll 起的真 18080 服务）。 */
  await mockApiRoute(page, "**/api/v1/contents?**", (route) => {
    const url = new URL(route.request().url());
    const pageNo = Math.max(1, Number(url.searchParams.get("page") ?? "1"));
    const pageSize = Math.max(1, Number(url.searchParams.get("page_size") ?? "20"));
    const zone = url.searchParams.get("zone") ?? "";
    const category = url.searchParams.get("category") ?? "";
    const items = zone === "original" && category === "film_tv" ? FILM_TV_ITEMS : RECOMMEND_ITEMS;
    const start = (pageNo - 1) * pageSize;
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        contents: items.slice(start, start + pageSize),
        total: items.length,
        page: pageNo,
        page_size: pageSize,
      }),
    });
  });
  /* 封面图请求统一占位，避免真实网络。 */
  await page.route("**/_next/image?*", (route) => route.fulfill({ status: 404 }));
  await page.route(/\/seed-media\/.*/, (route) => route.fulfill({ status: 404 }));
}

async function cardCount(page: import("@playwright/test").Page): Promise<number> {
  return page.locator('[data-slot="card-cover"]').count();
}

async function scrollToBottom(page: import("@playwright/test").Page) {
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
}

test("recommend feed: first screen 12, scroll appends one page at a time, no cascade, ends with terminal state", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await mockContentsApi(page);
  await page.goto("/recommend");

  await expect(page.locator('[data-slot="card-cover"]')).toHaveCount(12, { timeout: 15_000 });

  /* 首屏内不级联追加：停留 1s 条数不变（哨兵几何修复的直接断言——
     旧实现哨兵停在固定高度容器顶部，会持续翻页直至耗尽）。 */
  await page.waitForTimeout(1000);
  expect(await cardCount(page)).toBe(12);

  /* 滚到底 → 追加一页（24）；再滚 → 追加最后一页（30）+ 判尽终态。 */
  await scrollToBottom(page);
  await expect(page.locator('[data-slot="card-cover"]')).toHaveCount(24, { timeout: 10_000 });
  await scrollToBottom(page);
  await expect(page.locator('[data-slot="card-cover"]')).toHaveCount(30, { timeout: 10_000 });
  await expect(page.getByText("You've reached the end")).toBeVisible({ timeout: 10_000 });

  /* 无重复条目（按卡片封面所属 article 的 aria-label 计）。 */
  const labels = await page.locator('[data-slot="card-cover"]').evaluateAll((els) =>
    els.map((el) => el.closest("article")?.querySelector("button[aria-label]")?.getAttribute("aria-label") ?? ""),
  );
  expect(labels).toHaveLength(30);
  expect(new Set(labels).size).toBe(30);

  /* 判尽后不再请求（条数稳定）。 */
  await scrollToBottom(page);
  await page.waitForTimeout(600);
  expect(await cardCount(page)).toBe(30);
});

test("original feed: filter switch resets to page 1 with page_size and keeps scroll-append under the filter", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const requests: string[] = [];
  await mockContentsApi(page);
  page.on("request", (r) => {
    if (r.url().includes("/api/v1/contents")) requests.push(new URL(r.url()).searchParams.toString());
  });
  await page.goto("/original");

  await expect(page.locator('[data-slot="card-cover"]')).toHaveCount(12, { timeout: 15_000 });

  /* 切换类目 → 重置回第 1 页，请求携带 page=1 + page_size=12 + category。 */
  await page.getByRole("button", { name: "Film & TV" }).click();
  await expect(page.getByRole("button", { name: "Film item 1", exact: true })).toBeVisible({ timeout: 10_000 });
  await expect(page.locator('[data-slot="card-cover"]')).toHaveCount(12);
  const resetRequest = requests.find((q) => q.includes("category=film_tv"));
  expect(resetRequest).toBeTruthy();
  expect(resetRequest).toContain("page=1");
  expect(resetRequest).toContain("page_size=12");
  expect(resetRequest).toContain("zone=original");

  /* 筛选态下滚动继续追加（同筛选参数的 page=2）。 */
  await scrollToBottom(page);
  await expect(page.locator('[data-slot="card-cover"]')).toHaveCount(20, { timeout: 10_000 });
  const appendRequest = requests.filter((q) => q.includes("category=film_tv") && q.includes("page=2"));
  expect(appendRequest.length).toBeGreaterThanOrEqual(1);
  expect(appendRequest[0]).toContain("page_size=12");
  await expect(page.getByText("You've reached the end")).toBeVisible({ timeout: 10_000 });
});
