import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { test, expect, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

/* #412 F4+F5 侧边栏垂直节奏机器验收（mocked，真浏览器真几何）：
   - 两态切换前后，所有两态共存的锚点元素（toggle / 分区头 / 导航行）
     getBoundingClientRect().top 逐一相等（用户给定的自检标准）。
   - 分区头两态等高（36px）、分隔线语义 token 明暗两主题 computed color。
   - 收起态即时 tooltip（hover opacity→1 且 transition-delay=0s）。
   - 「热门」收起整段移除；宽度 48↔228；状态 localStorage 持久化。 */

let backendServer: ReturnType<typeof createServer> | null = null;

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
  if (url.pathname === "/api/v1/contents") {
    return json(res, 200, {
      contents: [],
      total: 0,
      page: 1,
      page_size: 12,
    });
  }
  if (url.pathname === "/api/v1/stats/summary") {
    return json(res, 200, { summary: { users: 9, ips: 2, contents: 0 } });
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

async function setup(page: Page) {
  await page.setViewportSize({ width: 1440, height: 900 });
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/contents?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ contents: [], total: 0, page: 1, page_size: 12 }),
    }),
  );
  await page.route("**/_next/image?*", (route) => route.fulfill({ status: 404 }));
  await page.route(/\/seed-media\/.*/, (route) => route.fulfill({ status: 404 }));
}

interface AnchorSnapshot {
  key: string;
  top: number;
  height: number;
}

/** 采集两态共存的锚点几何：toggle 按钮、分区头、每个导航行。 */
async function snapshotAnchors(page: Page): Promise<AnchorSnapshot[]> {
  return page.evaluate(() => {
    const out: { key: string; top: number; height: number }[] = [];
    const aside = document.querySelector("aside");
    if (!aside) throw new Error("sidebar aside not found");

    const toggle = aside.querySelector("button[aria-label]");
    if (toggle) {
      const r = toggle.getBoundingClientRect();
      out.push({ key: "toggle", top: r.top, height: r.height });
    }
    aside.querySelectorAll('[data-sidebar-anchor="section-header"]').forEach((el, i) => {
      const r = el.getBoundingClientRect();
      out.push({ key: `section-header-${i}`, top: r.top, height: r.height });
    });
    aside.querySelectorAll('[data-sidebar-anchor="item"]').forEach((el, i) => {
      const r = el.getBoundingClientRect();
      out.push({ key: `item-${i}`, top: r.top, height: r.height });
    });
    return out;
  });
}

async function settle(page: Page) {
  /* 宽度过渡 200ms + 文字淡入 150ms，450ms 覆盖全部动画路径。 */
  await page.waitForTimeout(450);
}

test("expand/collapse keeps every shared anchor's rect.top identical (machine acceptance)", async ({ page }) => {
  await setup(page);
  await page.goto("/");
  await expect(page.locator("aside")).toBeVisible({ timeout: 15_000 });

  const expanded = await snapshotAnchors(page);
  expect(expanded.length).toBeGreaterThanOrEqual(10);

  /* 收起 */
  await page.locator("aside button[aria-label]").first().click();
  await settle(page);

  const aside = page.locator("aside");
  await expect(aside).toHaveClass(/w-12/);

  const collapsed = await snapshotAnchors(page);
  expect(collapsed.length).toBe(expanded.length);
  for (let i = 0; i < expanded.length; i++) {
    expect(collapsed[i].key).toBe(expanded[i].key);
    expect(
      collapsed[i].top,
      `${collapsed[i].key} top must not move between states`,
    ).toBe(expanded[i].top);
  }

  /* 展开回去（方向对称） */
  await page.locator("aside button[aria-label]").first().click();
  await settle(page);
  await expect(aside).toHaveClass(/w-\[228px\]/);

  const reExpanded = await snapshotAnchors(page);
  for (let i = 0; i < expanded.length; i++) {
    expect(
      reExpanded[i].top,
      `${reExpanded[i].key} top must return after re-expand`,
    ).toBe(expanded[i].top);
  }

  /* 状态持久化 */
  const stored = await page.evaluate(() => window.localStorage.getItem("sidebarCollapsed"));
  expect(stored).toBe("false");
});

test("section header keeps equal height in both states; divider token color validates per theme", async ({ page }) => {
  await setup(page);
  await page.goto("/");
  await expect(page.locator("aside")).toBeVisible({ timeout: 15_000 });

  const heights = "[data-sidebar-anchor='section-header']";
  await expect(page.locator(heights)).toHaveCount(2);
  for (const el of await page.locator(heights).all()) {
    await expect(el).toHaveCSS("height", "36px");
  }

  const dividerColor = async () =>
    page.evaluate(() => {
      const header = document.querySelector("[data-sidebar-anchor='section-header'] span[aria-hidden='true']");
      return getComputedStyle(header as HTMLElement).backgroundColor;
    });

  expect(await dividerColor()).toBe("rgb(232, 232, 232)");

  await page.evaluate(() => document.documentElement.classList.add("dark"));
  await page.waitForTimeout(60);
  expect(await dividerColor()).toBe("rgb(48, 54, 61)");

  await page.evaluate(() => document.documentElement.classList.remove("dark"));
});

test("collapsed rail shows instant tooltips on hover and hides trending entirely", async ({ page }) => {
  await setup(page);
  await page.addInitScript(() => window.localStorage.setItem("sidebarCollapsed", "true"));
  await page.goto("/");
  const aside = page.locator("aside");
  await expect(aside).toBeVisible({ timeout: 15_000 });
  await expect(aside).toHaveClass(/w-12/);

  /* 热门区块收起整段移除 */
  await expect(page.getByText("Trending IPs", { exact: false })).toHaveCount(0);

  /* 即时 tooltip：hover 第一行 → opacity=1 且无延迟 */
  const firstRow = page.locator("[data-sidebar-anchor='item']").first();
  await firstRow.hover();
  await page.waitForTimeout(220);
  const tooltipState = await page.evaluate(() => {
    const row = document.querySelector("[data-sidebar-anchor='item']") as HTMLElement;
    const tip = row.querySelector("[class*='group-hover:opacity-100']") as HTMLElement;
    const cs = getComputedStyle(tip);
    return { opacity: cs.opacity, delay: cs.transitionDelay };
  });
  expect(tooltipState.opacity).toBe("1");
  expect(tooltipState.delay).toBe("0s");

  /* tooltip 逃逸窄轨：右缘在侧栏外 */
  const escape = await page.evaluate(() => {
    const rail = document.querySelector("aside")!.getBoundingClientRect();
    const row = document.querySelector("[data-sidebar-anchor='item']") as HTMLElement;
    const tip = row.querySelector("[class*='group-hover:opacity-100']") as HTMLElement;
    const r = tip.getBoundingClientRect();
    return { railRight: rail.right, tipLeft: r.left };
  });
  expect(escape.tipLeft).toBeGreaterThanOrEqual(escape.railRight);
});

test("expanded list scrolls independently while the toggle stays fixed", async ({ page }) => {
  await setup(page);
  await page.setViewportSize({ width: 1440, height: 500 });
  await page.goto("/");
  const aside = page.locator("aside");
  await expect(aside).toBeVisible({ timeout: 15_000 });

  const toggleTopBefore = await page.evaluate(() => {
    const rail = document.querySelector("aside")!;
    return rail.querySelector("button[aria-label]")!.getBoundingClientRect().top;
  });

  /* 展开态列表滚动 */
  await page.evaluate(() => {
    const rail = document.querySelector("aside")!;
    const scroller = rail.querySelector(".overflow-y-auto") as HTMLElement | null;
    if (scroller) scroller.scrollTop = 120;
  });
  await page.waitForTimeout(120);

  const toggleTopAfter = await page.evaluate(() => {
    const rail = document.querySelector("aside")!;
    return rail.querySelector("button[aria-label]")!.getBoundingClientRect().top;
  });
  expect(toggleTopAfter).toBe(toggleTopBefore);
});
