import { expect, test, type Page } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

/* SSR 数据源（Next 服务端渲染直连 127.0.0.1:18080，不经 page.route）。 */
let server: ReturnType<typeof createServer> | null = null;

const NUMBER_WORDS = ["一", "二", "三", "四", "五", "六", "七", "八", "九", "十"];

const CHAPTERS = Array.from({ length: 10 }, (_, index) => {
  const names = ["启程", "山雨", "迷雾", "星轨", "星夜", "回声", "断桥", "长夜", "破晓", "归途"];
  return { id: 601 + index, title: `第${NUMBER_WORDS[index]}章：${names[index]}`, index: index + 1 };
});

function contentDetail(id: number) {
  const chapter = CHAPTERS.find((item) => item.id === id);
  const currentIndex = chapter?.index ?? 1;
  return {
    content: {
      id,
      title: chapter?.title ?? `章节 ${id}`,
      description: "浮层内系列目录截图合同",
      body: "这是用于验证浮层内系列目录的正文。",
      content_type: "article",
      category: "literature",
      zone: "original",
      status: "published",
      author: { id: 42, username: "Ada" },
      created_at: "2026-07-01T00:00:00Z",
    },
    attachments: [],
    tags: [],
    series_memberships: [
      {
        series_id: 7,
        series_title: "山海纪行",
        series_zone: "original",
        current_index: currentIndex,
        total: CHAPTERS.length,
        ...(currentIndex > 1 ? { previous: { id: 600 + currentIndex - 1, title: CHAPTERS[currentIndex - 2]?.title } } : {}),
        ...(currentIndex < CHAPTERS.length ? { next: { id: 600 + currentIndex + 1, title: CHAPTERS[currentIndex]?.title } } : {}),
      },
    ],
  };
}

const SERIES_DETAIL = {
  series: {
    id: 7,
    title: "山海纪行",
    description: "十章公开旅程",
    zone: "original",
    owner: { id: 42, username: "Ada" },
    cover: null,
    item_count: CHAPTERS.length,
  },
  items: CHAPTERS.map((chapter, index) => ({
    id: 700 + index,
    sort_order: index,
    content_item_id: chapter.id,
    content: {
      id: chapter.id,
      title: chapter.title,
      zone: "original",
      content_type: "article",
      status: "published",
    },
  })),
};

function contentCard(id: number) {
  const chapter = CHAPTERS.find((item) => item.id === id);
  return {
    id,
    title: chapter?.title ?? `章节 ${id}`,
    zone: "original",
    content_type: "article",
    category: "literature",
    status: "published",
    author: { id: 42, username: "Ada" },
    like_count: 1,
  };
}

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
  const contentMatch = url.pathname.match(/^\/api\/v1\/contents\/(\d+)$/);
  if (contentMatch && Number(contentMatch[1]) >= 601 && Number(contentMatch[1]) <= 610) {
    return json(res, 200, contentDetail(Number(contentMatch[1])));
  }
  if (url.pathname === "/api/v1/series/7") {
    return json(res, 200, SERIES_DETAIL);
  }
  /* 原创区瀑布流 SSR 合同：第3章（常规用例）与第10章（末章边界）作为
     可点击卡片进入浮层。 */
  if (url.pathname === "/api/v1/contents" && url.searchParams.get("zone") === "original") {
    return json(res, 200, { contents: [contentCard(603), contentCard(610)], total: 2 });
  }
  return json(res, 404, { code: "NOT_FOUND", message: url.pathname });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

/* ------------------------------------------------------------------ */

async function mockClientApis(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/me", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ unread_counts: { total: 0 } }) }));
  await mockApiRoute(page, "**/api/v1/users/me/history", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/social/comments?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ comments: [] }) }));
  await mockApiRoute(page, "**/api/v1/social/reactions?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ counts: { like: 0, dislike: 0 }, user_reaction: null }) }));

  /* 浮层栈内打开的内容详情（客户端拉取）。 */
  for (const chapter of CHAPTERS) {
    await mockApiRoute(page, `**/api/v1/contents/${chapter.id}`, (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(contentDetail(chapter.id)) }));
    await mockApiRoute(page, `**/api/v1/contents/${chapter.id}/related-fanworks?**`, (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));
  }

  /* 浮层内目录的章节列表（客户端拉取公开系列详情合同）。 */
  await mockApiRoute(page, "**/api/v1/series/7", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(SERIES_DETAIL) }));

  /* 相似内容固定 list 合同（浮层内桌面相关内容块；zone=original&content_type=article&category=literature&sort=hot）。 */
  await mockApiRoute(page, "**/api/v1/contents?**", (route) => {
    const url = new URL(route.request().url());
    if (url.pathname !== "/api/v1/contents") return route.fallback();
    const isSimilarContract =
      url.searchParams.get("zone") === "original" &&
      url.searchParams.get("content_type") === "article" &&
      url.searchParams.get("category") === "literature" &&
      url.searchParams.get("sort") === "hot";
    if (!isSimilarContract) {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) });
    }
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        contents: [contentCard(604), contentCard(609)],
        total: 2,
        page: 1,
        page_size: 12,
      }),
    });
  });
}

function overlayNav(dialog: ReturnType<Page["getByRole"]>) {
  return dialog.locator('nav[aria-label="所属内容系列"]');
}

/* #397 R2：无附件文章 → 自动文字封面 3:4 = 竖版 → variant 路径整个移除
   header，标题迁移为 sr-only h2（dialog aria-labelledby 三职之一）。按
   level+name 锚定对 variant/split-media 两种壳层同构适用。 */
function overlayTitle(dialog: ReturnType<Page["getByRole"]>, title: string) {
  return dialog.getByRole("heading", { level: 2, name: title });
}

/* variant 右栏关联内容块（#397 布局钉死：系列跳转行在此，浮窗内无 SeriesNav）。 */
function overlayRelatedBlock(dialog: ReturnType<Page["getByRole"]>) {
  return dialog.locator('[data-slot="overlay-related-block"]');
}

async function openOverlayFromFeed(page: Page, chapterTitle: string) {
  await page.goto("/original");
  await expect(page.getByRole("article").filter({ hasText: chapterTitle })).toBeVisible();
  await page.getByRole("article").filter({ hasText: chapterTitle }).locator("button").first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(overlayTitle(dialog, chapterTitle)).toBeVisible();
  await expect(overlayRelatedBlock(dialog).getByText("同系列 · 山海纪行")).toBeVisible();
  return dialog;
}

test.describe("Ticket 69: 浮窗内系列目录与章节导航 (#69)", () => {
  test.use({ locale: "zh-CN" });

  test("standalone direct content URL keeps full-page series navigation", async ({ page }) => {
    await mockClientApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/original/603");
    const standaloneNav = page.getByRole("navigation", { name: "所属内容系列" });
    await expect(standaloneNav).toBeVisible();
    await expect(page.getByRole("link", { name: /上一章：第二章：山雨/ })).toHaveAttribute("href", "/original/602");
    await expect(page.getByRole("link", { name: /系列目录|catalog/i })).toHaveAttribute("href", "/series/7");
    await expect(page.getByRole("link", { name: /下一章：第四章：星轨/ })).toHaveAttribute("href", "/original/604");

    /* 首章与末章直接 URL：disabled 状态可读且不渲染无效链接。 */
    await page.goto("/original/601");
    await expect(page.getByRole("button", { name: /上一章不可用，已是第一章/ })).toBeDisabled();
    await page.goto("/original/610");
    await expect(page.getByRole("button", { name: /下一章不可用，已是最后一章/ })).toBeDisabled();
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t69-series-dir-standalone.png") });
  });

  test("desktop: series jump lives in the variant related block, chapter push stays in the overlay stack, back retraces", async ({ page }) => {
    await mockClientApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    const dialog = await openOverlayFromFeed(page, "第三章：迷雾");
    /* R2 布局钉死：同系列跳转 = 关联内容块内一行（系列名 + 第 X/Y 篇 + 上一章/
       下一章，边界禁用）；目录 listbox 仅存于全页 SeriesNav（浮窗内随 #397 移除）。 */
    const seriesRow = overlayRelatedBlock(dialog);
    await expect(seriesRow.getByText("第 3/10 篇")).toBeVisible();

    /* 上一章/下一章都是按钮，不整页跳转。 */
    const prevButton = seriesRow.getByRole("button", { name: "上一章：第二章：山雨" });
    await expect(prevButton).toBeVisible();
    await expect(seriesRow.getByRole("button", { name: "下一章：第四章：星轨" })).toBeVisible();
    await expect(seriesRow.getByRole("link")).toHaveCount(0, { timeout: 3_000 });
    await expect(seriesRow.getByRole("listbox")).toHaveCount(0);

    /* 下一章压栈：标题切换且 URL 不变。 */
    await seriesRow.getByRole("button", { name: "下一章：第四章：星轨" }).click();
    await expect(overlayTitle(dialog, "第四章：星轨")).toBeVisible();
    expect(page.url()).toMatch(/\/original$/);

    /* 返回逐层恢复上一章（variant 悬浮返回钮 aria-label = 「返回到：XXX」）。 */
    await dialog.getByRole("button", { name: /返回到：第三章：迷雾/ }).click();
    await expect(overlayTitle(dialog, "第三章：迷雾")).toBeVisible();
    await expect(seriesRow.getByRole("button", { name: "上一章：第二章：山雨" })).toBeVisible();

    /* 首章边界：连续上一章到第一章，上一章禁用（aria-label 退化为无章节名）。 */
    await seriesRow.getByRole("button", { name: "上一章：第二章：山雨" }).click();
    await expect(overlayTitle(dialog, "第二章：山雨")).toBeVisible();
    await overlayRelatedBlock(dialog).getByRole("button", { name: "上一章：第一章：启程" }).click();
    await expect(overlayTitle(dialog, "第一章：启程")).toBeVisible();
    await expect(overlayRelatedBlock(dialog).getByRole("button", { name: "上一章", exact: true })).toBeDisabled();
    await expect(overlayRelatedBlock(dialog).getByRole("button", { name: "下一章：第二章：山雨" })).toBeVisible();

    /* 浏览器后退逐层恢复先前章节并最终关闭浮窗。 */
    await page.goBack();
    await expect(overlayTitle(dialog, "第二章：山雨")).toBeVisible();
    await page.goBack();
    await expect(overlayTitle(dialog, "第三章：迷雾")).toBeVisible();
    await page.goBack();
    await expect(dialog).not.toBeVisible();
    expect(page.url()).toMatch(/\/original$/);
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t69-series-dir-return.png") });

    /* 末章边界：从信息流直接打开第十章，下一章禁用。 */
    await openOverlayFromFeed(page, "第十章：归途");
    await expect(overlayRelatedBlock(dialog).getByText("第 10/10 篇")).toBeVisible();
    await expect(overlayRelatedBlock(dialog).getByRole("button", { name: "下一章", exact: true })).toBeDisabled();
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t69-series-dir-desktop.png") });
  });

  test("keyboard: series buttons are focusable, Enter pushes, Escape retraces and closes", async ({ page }) => {
    await mockClientApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    const dialog = await openOverlayFromFeed(page, "第三章：迷雾");

    /* 键盘推进：聚焦关联块下一章按钮后 Enter 压栈，标题切换且无整页跳转。 */
    const nextButton = overlayRelatedBlock(dialog).getByRole("button", { name: "下一章：第四章：星轨" });
    await nextButton.focus();
    await page.keyboard.press("Enter");
    await expect(overlayTitle(dialog, "第四章：星轨")).toBeVisible();
    expect(page.url()).toMatch(/\/original$/);

    /* Escape 逐层返回：弹层后焦点还给压栈触发钮（AC4 焦点恢复契约）。 */
    await page.keyboard.press("Escape");
    await expect(overlayTitle(dialog, "第三章：迷雾")).toBeVisible();
    await expect(nextButton).toBeFocused();

    /* 栈底再按 Escape 关闭浮窗。 */
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
  });

  test("mobile and tablet: series actions and directory stay inside their containers", async ({ page }) => {
    await mockClientApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    const dialog = await openOverlayFromFeed(page, "第三章：迷雾");

    /* 浮层保持打开时缩到移动宽度：<1100px 退出 variant，回到单列布局，
       SeriesNav（含目录）随默认尾部渲染。 */
    await page.setViewportSize({ width: 390, height: 844 });
    const nav = overlayNav(dialog);
    await expect(nav).toBeVisible();
    const mobileBox = await nav.boundingBox();
    assertBox(mobileBox);
    for (const name of [/上一章：第二章：山雨/, /系列目录/, /下一章：第四章：星轨/]) {
      const button = nav.getByRole("button", { name });
      await expect(button).toBeVisible();
      const box = await button.boundingBox();
      assertBox(box);
      expect(box.x + box.width).toBeLessThanOrEqual(mobileBox.x + mobileBox.width + 1);
      expect(box.x).toBeGreaterThanOrEqual(mobileBox.x - 1);
    }

    await nav.getByRole("button", { name: "查看 山海纪行 系列目录" }).click();
    const listbox = dialog.getByRole("listbox");
    await expect(listbox).toBeVisible();
    const listboxBox = await listbox.boundingBox();
    assertBox(listboxBox);
    expect(listboxBox.x).toBeGreaterThanOrEqual(0);
    expect(listboxBox.x + listboxBox.width).toBeLessThanOrEqual(390);
    expect(listboxBox.height).toBeLessThanOrEqual(288 + 1);
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t69-series-dir-mobile.png") });
    await page.keyboard.press("Escape");
    await expect(listbox).not.toBeVisible();

    /* 平板宽度：三个动作仍在容器内。 */
    await page.setViewportSize({ width: 768, height: 1024 });
    const tabletBox = await nav.boundingBox();
    assertBox(tabletBox);
    for (const name of [/上一章：第二章：山雨/, /系列目录/, /下一章：第四章：星轨/]) {
      const box = await nav.getByRole("button", { name }).boundingBox();
      assertBox(box);
      expect(box.x + box.width).toBeLessThanOrEqual(tabletBox.x + tabletBox.width + 1);
      expect(box.x).toBeGreaterThanOrEqual(tabletBox.x - 1);
    }
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t69-series-dir-tablet.png") });
  });
});

function assertBox(box: { x: number; y: number; width: number; height: number } | null): asserts box is { x: number; y: number; width: number; height: number } {
  expect(box).not.toBeNull();
}
