import { expect, test, type Page } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { deflateSync } from "node:zlib";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

const COVERS: Array<{ file: string; w: number; h: number; rgb: [number, number, number] }> = [
  { file: "ratio-2-3", w: 240, h: 360, rgb: [99, 102, 241] },
  { file: "ratio-16-9", w: 320, h: 180, rgb: [6, 182, 212] },
  { file: "ratio-1-4", w: 120, h: 480, rgb: [245, 158, 11] },
  { file: "ratio-1-1", w: 240, h: 240, rgb: [16, 185, 129] },
  { file: "ratio-5-1", w: 500, h: 100, rgb: [236, 72, 153] },
  { file: "ratio-3-4", w: 225, h: 300, rgb: [139, 92, 246] },
  { file: "ratio-9-16", w: 180, h: 320, rgb: [59, 130, 246] },
  { file: "ratio-1-2", w: 160, h: 320, rgb: [249, 115, 22] },
  { file: "ratio-16-10", w: 320, h: 200, rgb: [14, 165, 233] },
];

const ORIGINAL_CONTENTS = [
  { id: 1, title: "Tall portrait 2:3", zone: "original", content_type: "image", cover_image_url: "/img/ratio-2-3.png", cover_width: 600, cover_height: 900, like_count: 12 },
  { id: 2, title: "Landscape 16:9", zone: "original", content_type: "image", cover_image_url: "/img/ratio-16-9.png", cover_width: 1600, cover_height: 900, like_count: 8 },
  { id: 3, title: "Extreme tall 1:4", zone: "original", content_type: "image", cover_image_url: "/img/ratio-1-4.png", cover_width: 400, cover_height: 1600, like_count: 20 },
  { id: 4, title: "Square 1:1", zone: "original", content_type: "image", cover_image_url: "/img/ratio-1-1.png", cover_width: 1000, cover_height: 1000, like_count: 6 },
  { id: 5, title: "Extreme wide 5:1", zone: "original", content_type: "image", cover_image_url: "/img/ratio-5-1.png", cover_width: 2500, cover_height: 500, like_count: 3 },
  { id: 6, title: "Legacy no size", zone: "original", content_type: "image", cover_image_url: "/img/ratio-16-10.png", like_count: 9 },
  { id: 7, title: "Video poster 9:16", zone: "original", content_type: "video", cover_image_url: "/img/ratio-9-16.png", cover_width: 720, cover_height: 1280, like_count: 5 },
  { id: 8, title: "Portrait 1:2", zone: "original", content_type: "image", cover_image_url: "/img/ratio-1-2.png", cover_width: 500, cover_height: 1000, like_count: 2 },
];

const SEARCH_ITEMS = [
  { id: 101, title: "Fanwork remix 4:3", zone: "fanwork", content_type: "image", cover_image_url: "/img/ratio-3-4.png", cover_width: 1200, cover_height: 900, author: { id: 7, username: "Bea" }, ip: { id: 3, name: "Indigo IP" }, tags: ["art"], like_count: 3, comment_count: 1 },
  { id: 102, title: "Fanwork extreme 1:5", zone: "fanwork", content_type: "image", cover_image_url: "/img/ratio-1-4.png", cover_width: 300, cover_height: 1500, author: { id: 7, username: "Bea" }, ip: { id: 3, name: "Indigo IP" }, tags: ["study"], like_count: 9, comment_count: 2 },
  { id: 103, title: "Fanwork square", zone: "fanwork", content_type: "image", cover_image_url: "/img/ratio-1-1.png", cover_width: 900, cover_height: 900, author: { id: 8, username: "Cid" }, ip: { id: 3, name: "Indigo IP" }, like_count: 4, comment_count: 0 },
  { id: 104, title: "Original landscape 3:2", zone: "original", content_type: "image", cover_image_url: "/img/ratio-16-9.png", cover_width: 1500, cover_height: 1000, like_count: 7 },
  { id: 105, title: "Fanwork legacy video", zone: "fanwork", content_type: "video", cover_image_url: "/img/ratio-16-10.png", author: { id: 8, username: "Cid" }, ip: { id: 3, name: "Indigo IP" }, like_count: 1, comment_count: 0 },
];

let server: ReturnType<typeof createServer> | null = null;

test.beforeAll(async () => {
  server = createServer(handleApi);
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

function handleApi(req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? "/", "http://127.0.0.1:18080");
  if (url.pathname === "/api/v1/contents" && req.method === "GET") {
    return json(res, 200, { contents: ORIGINAL_CONTENTS, total: ORIGINAL_CONTENTS.length, page: 1, page_size: 24 });
  }
  if (url.pathname === "/api/v1/stats/summary") {
    return json(res, 200, { summary: { users: 42, ips: 7, contents: ORIGINAL_CONTENTS.length } });
  }
  return json(res, 404, { code: "NOT_FOUND", message: url.pathname });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

/* 微型 PNG 编码器：为每种比例生成纯色封面图（无需 canvas 依赖）。 */
const CRC_TABLE = new Uint32Array(256);
for (let n = 0; n < 256; n += 1) {
  let c = n;
  for (let k = 0; k < 8; k += 1) {
    c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  }
  CRC_TABLE[n] = c >>> 0;
}

function crc32(buf: Buffer): number {
  let c = 0xffffffff;
  for (const byte of buf) {
    c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8);
  }
  return (c ^ 0xffffffff) >>> 0;
}

function pngChunk(type: string, data: Buffer): Buffer {
  const out = Buffer.alloc(12 + data.length);
  out.writeUInt32BE(data.length, 0);
  out.write(type, 4, "ascii");
  data.copy(out, 8);
  out.writeUInt32BE(crc32(out.subarray(4, 8 + data.length)), 8 + data.length);
  return out;
}

function solidPng(width: number, height: number, rgb: [number, number, number]): Buffer {
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8;
  ihdr[9] = 2;
  const stride = 1 + width * 3;
  const raw = Buffer.alloc(stride * height);
  for (let y = 0; y < height; y += 1) {
    const row = y * stride;
    raw[row] = 0;
    for (let x = 0; x < width; x += 1) {
      const off = row + 1 + x * 3;
      raw[off] = rgb[0];
      raw[off + 1] = rgb[1];
      raw[off + 2] = rgb[2];
    }
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    pngChunk("IHDR", ihdr),
    pngChunk("IDAT", deflateSync(raw)),
    pngChunk("IEND", Buffer.alloc(0)),
  ]);
}

const PNG_BY_FILE = new Map(COVERS.map((c) => [`/img/${c.file}.png`, solidPng(c.w, c.h, c.rgb)]));

async function mockFeedApis(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/contents?**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: ORIGINAL_CONTENTS, total: ORIGINAL_CONTENTS.length, page: 1, page_size: 24 }) }),
  );
  await mockApiRoute(page, "**/api/v1/contents/search?**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: SEARCH_ITEMS, total: SEARCH_ITEMS.length, page: 1, page_size: 20 }) }),
  );
  for (const [imgPath, body] of PNG_BY_FILE) {
    await page.route(imgPath, (route) => route.fulfill({ status: 200, contentType: "image/png", body }));
  }
  await page.route("**/_next/image?*", (route) => {
    const reqUrl = new URL(route.request().url());
    const imgPath = decodeURIComponent(reqUrl.searchParams.get("url") ?? "");
    const body = PNG_BY_FILE.get(imgPath);
    if (!body) return route.fulfill({ status: 404 });
    return route.fulfill({ status: 200, contentType: "image/png", body });
  });
}

function aspectFrame(page: Page, title: string) {
  return page.locator(`[aria-label="${title}"] [data-slot="card-cover-aspect"]`);
}

/**
 * 卡片瀑布流无水平溢出：仅断言卡片区域（data-slot="card-cover"）不超出视口。
 * 页面级 390px 视口存在既有 26px 溢出（原创区页头标题/分类 Tab 行，非本票
 * 卡片改动引入，见 t87 实现时在 main 上的复现记录），不归本票范围。
 */
async function assertCardsFitViewport(page: Page) {
  const overflow = await page.locator('[data-slot="card-cover"]').evaluateAll((els) => {
    const vw = document.documentElement.clientWidth;
    let worst = 0;
    for (const el of els) {
      const r = el.getBoundingClientRect();
      if (r.left < -1 || r.right > vw + 1) worst = Math.max(worst, Math.max(-r.left, r.right - vw));
    }
    return worst;
  });
  expect(overflow).toBeLessThanOrEqual(0);
}

/** 等待瀑布流完成测量（fallback 网格消失，绝对定位布局生效），避免断言落在水合过渡态。 */
async function waitForMasonryLayout(page: Page) {
  await page.waitForFunction(() => !document.querySelector(".w-full.grid"));
}

async function distinctColumnLefts(page: Page): Promise<number[]> {
  return page
    .locator('[data-slot="card-cover"]')
    .evaluateAll((els) => [...new Set(els.map((el) => Math.round(el.getBoundingClientRect().left)))]);
}

test("original feed renders data-driven natural ratios with the extreme height cap", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await mockFeedApis(page);
  await page.goto("/original");
  const frames = page.locator('[data-slot="card-cover-aspect"]');
  await expect(frames.first()).toBeVisible({ timeout: 15_000 });
  await waitForMasonryLayout(page);
  expect(await frames.count()).toBeGreaterThanOrEqual(8);

  await expect(aspectFrame(page, "Tall portrait 2:3")).toHaveAttribute("style", /aspect-ratio:\s*600 \/ 900/);
  await expect(aspectFrame(page, "Landscape 16:9")).toHaveAttribute("style", /aspect-ratio:\s*1600 \/ 900/);
  await expect(aspectFrame(page, "Square 1:1")).toHaveAttribute("style", /aspect-ratio:\s*1000 \/ 1000/);
  await expect(aspectFrame(page, "Video poster 9:16")).toHaveAttribute("style", /aspect-ratio:\s*720 \/ 1280/);
  await expect(aspectFrame(page, "Legacy no size")).toHaveAttribute("style", /aspect-ratio:\s*3 \/ 4/);

  await expect(aspectFrame(page, "Extreme tall 1:4")).toHaveAttribute("style", /max-height:\s*400px/);
  await expect(aspectFrame(page, "Extreme wide 5:1")).toHaveAttribute("style", /max-height:\s*400px/);
  await expect(aspectFrame(page, "Tall portrait 2:3")).not.toHaveAttribute("style", /max-height/);

  const extremeHeight = await aspectFrame(page, "Extreme tall 1:4").evaluate((el) => el.getBoundingClientRect().height);
  expect(extremeHeight).toBeLessThanOrEqual(400);

  const lefts = await distinctColumnLefts(page);
  expect(lefts.length).toBe(4);

  await assertCardsFitViewport(page);
  await page.screenshot({ path: path.join(SCREENSHOTS, "t87-original-mixed-ratio-desktop.png"), fullPage: true });
});

test("mobile original feed keeps two stable columns with the extreme cap", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockFeedApis(page);
  await page.goto("/original");
  const frames = page.locator('[data-slot="card-cover-aspect"]');
  await expect(frames.first()).toBeVisible({ timeout: 15_000 });
  await waitForMasonryLayout(page);

  const extremeHeight = await aspectFrame(page, "Extreme tall 1:4").evaluate((el) => el.getBoundingClientRect().height);
  expect(extremeHeight).toBeLessThanOrEqual(400);

  const lefts = await distinctColumnLefts(page);
  expect(lefts.length).toBe(2);

  await assertCardsFitViewport(page);
  await page.screenshot({ path: path.join(SCREENSHOTS, "t87-original-mixed-ratio-mobile.png"), fullPage: true });
});

test("cards are keyboard-focusable via Tab traversal", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await mockFeedApis(page);
  await page.goto("/original");
  const frames = page.locator('[data-slot="card-cover-aspect"]');
  await expect(frames.first()).toBeVisible({ timeout: 15_000 });
  await waitForMasonryLayout(page);

  const cardTitles = ORIGINAL_CONTENTS.map((c) => c.title);
  let focusedLabel = "";
  for (let i = 0; i < 60 && !focusedLabel; i += 1) {
    await page.keyboard.press("Tab");
    focusedLabel = await page.evaluate(() => document.activeElement?.getAttribute("aria-label") ?? "");
    if (!cardTitles.includes(focusedLabel)) focusedLabel = "";
  }
  expect(focusedLabel).toBe("Tall portrait 2:3");
  await page.screenshot({ path: path.join(SCREENSHOTS, "t87-original-keyboard-focus.png") });
});

test("search grid mixes fanwork and original cards from the shared fact source", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await mockFeedApis(page);
  await page.goto("/search");
  await page.getByPlaceholder("Search by keyword...").fill("ratio");
  await page.keyboard.press("Enter");

  const frames = page.locator('[data-slot="card-cover-aspect"]');
  await expect(frames.first()).toBeVisible({ timeout: 15_000 });
  await waitForMasonryLayout(page);
  expect(await frames.count()).toBe(5);

  const fanwork = page.locator('[aria-label="Fanwork extreme 1:5"]');
  await expect(fanwork).toHaveClass(/border border-border/);
  await expect(aspectFrame(page, "Fanwork extreme 1:5")).toHaveAttribute("style", /max-height:\s*400px/);
  await expect(aspectFrame(page, "Fanwork legacy video")).toHaveAttribute("style", /aspect-ratio:\s*3 \/ 4/);

  await assertCardsFitViewport(page);
  await page.screenshot({ path: path.join(SCREENSHOTS, "t87-search-mixed-zones-shared-card.png"), fullPage: true });
});
