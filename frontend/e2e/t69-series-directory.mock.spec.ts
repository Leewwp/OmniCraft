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
  /* 原创区瀑布流 SSR 合同：第3章作为可点击卡片进入浮层。 */
  if (url.pathname === "/api/v1/contents" && url.searchParams.get("zone") === "original") {
    return json(res, 200, { contents: [contentCard(603)], total: 1 });
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

async function openOverlayFromFeed(page: Page, chapterTitle: string) {
  await page.goto("/original");
  await expect(page.getByRole("article").filter({ hasText: chapterTitle })).toBeVisible();
  await page.getByRole("article").filter({ hasText: chapterTitle }).locator("button").first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.locator("header h2")).toHaveText(chapterTitle);
  await expect(overlayNav(dialog)).toBeVisible();
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

  test("desktop: directory opens in the overlay stack, chapter selection pushes without page navigation, back retraces", async ({ page }) => {
    await mockClientApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    const dialog = await openOverlayFromFeed(page, "第三章：迷雾");
    const nav = overlayNav(dialog);

    /* 浮层内上一章/目录/下一章都是按钮，不整页跳转。 */
    await expect(nav.getByRole("button", { name: /上一章：第二章：山雨/ })).toBeVisible();
    await expect(nav.getByRole("button", { name: /下一章：第四章：星轨/ })).toBeVisible();
    const catalogTrigger = nav.getByRole("button", { name: "查看 山海纪行 系列目录" });
    await expect(catalogTrigger).toHaveAttribute("aria-haspopup", "listbox");
    await expect(nav.getByRole("link")).toHaveCount(0, { timeout: 3_000 });

    /* 下一章压栈：标题切换且 URL 不变。 */
    await nav.getByRole("button", { name: /下一章：第四章：星轨/ }).click();
    await expect(dialog.locator("header h2")).toHaveText("第四章：星轨");
    expect(page.url()).toMatch(/\/original$/);

    /* 返回逐层恢复上一章。 */
    await dialog.getByRole("button", { name: /返回 第三章：迷雾/ }).click();
    await expect(dialog.locator("header h2")).toHaveText("第三章：迷雾");
    await expect(nav.getByRole("button", { name: /上一章：第二章：山雨/ })).toBeVisible();

    /* 目录：有界高度、内部滚动、listbox 语义、当前章节 aria-selected。 */
    await catalogTrigger.click();
    const listbox = dialog.getByRole("listbox");
    await expect(listbox).toBeVisible();
    await expect(listbox).toHaveClass(/max-h-72/);
    await expect(listbox).toHaveClass(/overflow-y-auto/);
    const options = listbox.getByRole("option");
    await expect(options).toHaveCount(10);
    await expect(options.nth(2)).toHaveAttribute("aria-selected", "true");
    await expect(options.nth(2)).toContainText("第三章：迷雾");
    await expect(options.nth(1)).toHaveAttribute("aria-selected", "false");
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t69-series-dir-desktop.png") });

    /* 目录内选择第一章：压栈、首章上一章禁用、无整页跳转。 */
    await options.filter({ hasText: "第一章：启程" }).click();
    await expect(dialog.locator("header h2")).toHaveText("第一章：启程");
    expect(page.url()).toMatch(/\/original$/);
    const firstPrev = nav.getByRole("button", { name: "上一章不可用，已是第一章" });
    await expect(firstPrev).toBeDisabled();
    await expect(firstPrev).toHaveAttribute("aria-disabled", "true");
    await expect(nav.getByText("已是第一章")).toBeVisible();
    await expect(nav.getByRole("button", { name: /下一章：第二章：山雨/ })).toBeVisible();

    /* 目录内选择末章：下一章禁用。 */
    await nav.getByRole("button", { name: "查看 山海纪行 系列目录" }).click();
    await dialog.getByRole("option", { name: /第十章：归途/ }).click();
    await expect(dialog.locator("header h2")).toHaveText("第十章：归途");
    const lastNext = nav.getByRole("button", { name: "下一章不可用，已是最后一章" });
    await expect(lastNext).toBeDisabled();
    await expect(nav.getByText("已是最后一章")).toBeVisible();

    /* 浏览器后退逐层恢复先前章节。 */
    await page.goBack();
    await expect(dialog.locator("header h2")).toHaveText("第一章：启程");
    await page.goBack();
    await expect(dialog.locator("header h2")).toHaveText("第三章：迷雾");
    await page.goBack();
    await expect(dialog).not.toBeVisible();
    expect(page.url()).toMatch(/\/original$/);
    await page.screenshot({ path: path.join(SCREENSHOTS, "web-t69-series-dir-return.png") });
  });

  test("keyboard: directory opens with focus inside, arrows move, Enter pushes, Escape closes and returns focus", async ({ page }) => {
    await mockClientApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    const dialog = await openOverlayFromFeed(page, "第三章：迷雾");
    const nav = overlayNav(dialog);

    await nav.getByRole("button", { name: "查看 山海纪行 系列目录" }).focus();
    await page.keyboard.press("ArrowDown");
    const listbox = dialog.getByRole("listbox");
    await expect(listbox).toBeVisible();
    /* 打开后焦点进入选择器并落在当前章节。 */
    await expect(dialog.getByRole("option", { name: /当前章节：第 3 章：第三章：迷雾/ })).toBeFocused();

    await page.keyboard.press("ArrowUp");
    await expect(dialog.getByRole("option", { name: /第 2 章：第二章：山雨/ })).toBeFocused();
    await page.keyboard.press("ArrowUp");
    await expect(dialog.getByRole("option", { name: /第 1 章：第一章：启程/ })).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(dialog.locator("header h2")).toHaveText("第一章：启程");
    expect(page.url()).toMatch(/\/original$/);

    /* 重开目录，Escape 关闭并把焦点还给「目录」trigger。 */
    await nav.getByRole("button", { name: "查看 山海纪行 系列目录" }).focus();
    await page.keyboard.press("ArrowDown");
    await expect(listbox).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog.getByRole("listbox")).not.toBeVisible();
    await expect(nav.getByRole("button", { name: "查看 山海纪行 系列目录" })).toBeFocused();
  });

  test("mobile and tablet: series actions and directory stay inside their containers", async ({ page }) => {
    await mockClientApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    const dialog = await openOverlayFromFeed(page, "第三章：迷雾");
    const nav = overlayNav(dialog);

    /* 浮层保持打开时缩到移动宽度：系列操作行与目录不得越出容器。 */
    await page.setViewportSize({ width: 390, height: 844 });
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
