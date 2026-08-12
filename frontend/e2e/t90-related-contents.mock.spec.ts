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
        description: "相似内容块截图合同（fanwork/article/art/IP 3）",
        body: "这是用于验证相关内容块的正文。",
        content_type: "article",
        category: "art",
        zone: "fanwork",
        ip: { id: 3, name: "星海 IP" },
        status: "published",
        author: { id: 42, username: "Ada" },
        created_at: "2026-07-01T00:00:00Z",
      },
      attachments: [],
      tags: [],
    });
  }
  if (url.pathname === "/api/v1/contents/502") {
    return json(res, 200, {
      content: {
        id: 502,
        title: "空分支原创",
        description: "相似内容空分支截图合同",
        body: "没有关联行也没有相似内容的原创。",
        content_type: "article",
        category: "literature",
        zone: "original",
        status: "published",
        author: { id: 43, username: "Bob" },
        created_at: "2026-07-02T00:00:00Z",
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

const DERIVATIVES = [
  { id: 611, title: "衍生 611", zone: "fanwork", content_type: "article", category: "art", author: { id: 44, username: "Cid" }, like_count: 5 },
];

/* 相似内容列表：含当前内容 601 与关联行 611，验证客户端去重（10 条 → 8 条）。 */
const SIMILAR_LIST = [
  { id: 601, title: "星尘 fanwork", zone: "fanwork", content_type: "article", category: "art", author: { id: 42, username: "Ada" }, like_count: 1 },
  { id: 611, title: "衍生 611", zone: "fanwork", content_type: "article", category: "art", author: { id: 44, username: "Cid" }, like_count: 5 },
  ...Array.from({ length: 8 }, (_, i) => ({
    id: 612 + i,
    title: `相似 ${612 + i}`,
    zone: "fanwork" as const,
    content_type: "article",
    category: "art",
    author: { id: 45, username: "Dee" },
    like_count: 3,
  })),
];

function relatedDetail(id: number, title: string) {
  return {
    content: {
      id,
      title,
      description: `${title} 详情`,
      body: `${title} 正文`,
      content_type: "article",
      category: "art",
      zone: "fanwork",
      ip: { id: 3, name: "星海 IP" },
      status: "published",
      author: { id: 45, username: "Dee" },
      created_at: "2026-07-03T00:00:00Z",
    },
    attachments: [],
    tags: [],
  };
}

async function mockClientApis(page: Page, similarHandler: (requestUrl: string) => void) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/me", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ unread_counts: { total: 0 } }) }));
  await mockApiRoute(page, "**/api/v1/users/me/history", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/social/comments?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ comments: [] }) }));
  await mockApiRoute(page, "**/api/v1/social/reactions?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ counts: { like: 0, dislike: 0 }, viewer_reaction: null }) }));
  await mockApiRoute(page, "**/api/v1/contents/601/versions", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ versions: [] }) }));

  /* 关联行（RF 组件客户端拉取）。 */
  await mockApiRoute(page, "**/api/v1/contents/601/related-fanworks?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: DERIVATIVES, total: DERIVATIVES.length, page: 1, page_size: 8 }) }));
  await mockApiRoute(page, "**/api/v1/contents/612/related-fanworks?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));
  await mockApiRoute(page, "**/api/v1/contents/502/related-fanworks?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));

  /* 相似内容：固定 list 合同（AC2）。仅对 601 的固定合同（zone=fanwork、
     content_type=article、category=art、ip_id=3、sort=hot）返回列表，
     其余筛选请求返回空，保证空分支测试语义正确。 */
  await mockApiRoute(page, "**/api/v1/contents?**", (route) => {
    similarHandler(route.request().url());
    const url = new URL(route.request().url());
    if (url.pathname !== "/api/v1/contents") return route.fallback();
    const isSimilarContract =
      url.searchParams.get("zone") === "fanwork" &&
      url.searchParams.get("content_type") === "article" &&
      url.searchParams.get("category") === "art" &&
      url.searchParams.get("ip_id") === "3" &&
      url.searchParams.get("sort") === "hot";
    if (isSimilarContract) {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: SIMILAR_LIST, total: SIMILAR_LIST.length, page: 1, page_size: 12 }) });
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) });
  });

  /* 浮层内点击的相关卡片详情（栈内打开验证）。 */
  await mockApiRoute(page, "**/api/v1/contents/601", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(relatedDetail(601, "星尘 fanwork")) }));
  await mockApiRoute(page, "**/api/v1/contents/612", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(relatedDetail(612, "相似 612")) }));
  await mockApiRoute(page, "**/api/v1/contents/613", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(relatedDetail(613, "相似 613")) }));
  await mockApiRoute(page, "**/api/v1/contents/613/related-fanworks?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));
}

test.describe("Ticket 10: 相关内容块与到底提示（桌面/web）(#90)", () => {
  test.use({ locale: "zh-CN" });

  test("desktop: related block renders both rows with fixed-contract dedupe, cards open in the overlay stack, end hint shows", async ({ page }) => {
    const similarRequests: string[] = [];
    await mockClientApis(page, (requestUrl) => similarRequests.push(requestUrl));
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/content/601");
    const block = page.locator('[data-slot="related-contents"]');
    await block.scrollIntoViewIfNeeded();
    await expect(block).toBeVisible();

    /* AC2：相似内容请求严格使用固定 list 合同。 */
    await expect.poll(() => similarRequests.length).toBeGreaterThan(0);
    const similarUrl = new URL(similarRequests[0]);
    expect(similarUrl.pathname).toBe("/api/v1/contents");
    expect(similarUrl.searchParams.get("zone")).toBe("fanwork");
    expect(similarUrl.searchParams.get("content_type")).toBe("article");
    expect(similarUrl.searchParams.get("category")).toBe("art");
    expect(similarUrl.searchParams.get("ip_id")).toBe("3");
    expect(similarUrl.searchParams.get("sort")).toBe("hot");
    expect(similarUrl.searchParams.get("page_size")).toBe("12");
    expect(similarUrl.searchParams.get("sort")).not.toBe("recommended");

    /* AC1：关联行（复用 RelatedFanworks 合同）+ 相似行（去重后 ≤8）。 */
    await expect(block.getByText("相关创作")).toBeVisible();
    await expect(block.getByText("衍生 611")).toBeVisible();
    await expect(block.getByText("你可能也喜欢")).toBeVisible();
    const similarCards = block.locator('[data-slot="related-contents-similar"] [data-slot="card-cover"]');
    await expect(similarCards).toHaveCount(8);
    /* 去重：当前内容 601 与关联行 611 不得出现在相似行。 */
    await expect(block.locator('[data-slot="related-contents-similar"]').getByText("星尘 fanwork")).toHaveCount(0);
    await expect(block.locator('[data-slot="related-contents-similar"]').getByText("衍生 611")).toHaveCount(0);

    /* AC4：相关块之后显示「已经到底了」。 */
    await expect(block.getByText("已经到底了")).toBeVisible();

    await page.screenshot({ path: path.join(SCREENSHOTS, "t90-related-contents-desktop.png"), fullPage: false });

    /* AC3：相关卡片在当前浮层导航栈内打开。 */
    await block.locator('[data-slot="related-contents-similar"] [data-slot="card-cover"]').nth(0).click();
    const dialog = page.getByRole("dialog");
    const overlayTitle = dialog.locator("header h2");
    await expect(dialog).toBeVisible();
    await expect(overlayTitle).toHaveText("相似 612");

    /* 浮层内继续下钻：612 的相似行卡片压栈（栈深 ≤5 沿用既有机制）。
       612 的相似行去重后首卡是 601，显式点击「相似 613」卡片。 */
    const innerBlock = dialog.locator('[data-slot="related-contents"]');
    await innerBlock.scrollIntoViewIfNeeded();
    await expect(innerBlock).toBeVisible();
    await expect(innerBlock.locator('[data-slot="related-contents-similar"] [data-slot="card-cover"]')).toHaveCount(8);
    await innerBlock
      .locator('[data-slot="related-contents-similar"] article')
      .filter({ hasText: "相似 613" })
      .locator("button")
      .click();
    await expect(overlayTitle).toHaveText("相似 613");
    await expect(dialog.getByRole("button", { name: /返回 相似 612/ })).toBeVisible();

    /* 逐层返回。 */
    await dialog.getByRole("button", { name: /返回 相似 612/ }).click();
    await expect(overlayTitle).toHaveText("相似 612");
    await page.screenshot({ path: path.join(SCREENSHOTS, "t90-related-contents-overlay-stack.png") });
  });

  test("empty branch: no related and no similar degrades to the end hint only", async ({ page }) => {
    const similarRequests: string[] = [];
    await mockClientApis(page, (requestUrl) => similarRequests.push(requestUrl));
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/original/502");
    const endHint = page.locator('[data-slot="related-contents-end"]');
    await expect(endHint).toBeVisible();
    await endHint.scrollIntoViewIfNeeded();
    await expect(endHint).toHaveText("已经到底了");
    await expect(page.locator('[data-slot="related-contents"]')).toHaveCount(0);
    /* 空分支不渲染空块标题。 */
    await expect(page.getByText("相关创作")).toHaveCount(0);
    await expect(page.getByText("你可能也喜欢")).toHaveCount(0);

    await page.screenshot({ path: path.join(SCREENSHOTS, "t90-related-contents-empty.png") });
  });

  test("mobile: the related block is not rendered and issues no similar request", async ({ page }) => {
    const similarRequests: string[] = [];
    await mockClientApis(page, (requestUrl) => similarRequests.push(requestUrl));
    await page.setViewportSize({ width: 375, height: 844 });

    await page.goto("/content/601");
    await expect(page.locator('[data-slot="related-contents"]')).toHaveCount(0);
    await expect(page.locator('[data-slot="related-contents-end"]')).toHaveCount(0);
    await page.waitForTimeout(300);
    expect(similarRequests.length).toBe(0);
    await page.screenshot({ path: path.join(SCREENSHOTS, "t90-related-contents-mobile.png") });
  });
});
