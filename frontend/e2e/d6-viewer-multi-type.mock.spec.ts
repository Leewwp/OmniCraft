import { expect, test, type Page } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { deflateSync } from "node:zlib";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

/* ────────────────────────────────────────────────────────────────────────────
 * D6 #863 放大态多类型化与跨可放大项导航——mocked 契约（真 Chromium）：
 * 混排内容（image + model3d + text-pdf + audio + 乐谱 pdf）的放大态
 * 仅覆盖可放大三类（image/model3d/text-pdf），入口规则（image 双入口、
 * 3D/pdf 仅右上放大钮、乐谱 pdf 无入口）、独立计数、边界 clamp、
 * 打开定位正确项（稳定 id 映射）、关闭回最后所看项、图片缩放零回归。
 * 本地无真实 OSS：签名 oss_url 指向本套件 18080 mock 服务的合成媒体字节
 * （纯色 PNG / ASCII STL / 最小 PDF）；pdf embed 与 3D 画布按 DOM 级形态
 * 断言（headless 下插件渲染不可截图验证）。
 * ──────────────────────────────────────────────────────────────────────────── */

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");
const BASE = "http://127.0.0.1:18080";

/* ── 合成媒体字节 ─────────────────────────────────────────────────────────── */

const CRC_TABLE = new Uint32Array(256);
for (let n = 0; n < 256; n += 1) {
  let c = n;
  for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  CRC_TABLE[n] = c >>> 0;
}
function crc32(buf: Buffer): number {
  let c = 0xffffffff;
  for (const byte of buf) c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8);
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
    raw[y * stride] = 0;
    for (let x = 0; x < width; x += 1) {
      const off = y * stride + 1 + x * 3;
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

/** ASCII STL（单三角面）——ModelViewer STLLoader 可解析（全屏复用同链路）。 */
function tinyStl(): Buffer {
  return Buffer.from(
    `solid d6
facet normal 0 0 1
  outer loop
    vertex 0 0 0
    vertex 10 0 0
    vertex 0 10 0
  endloop
endfacet
endsolid d6
`,
    "ascii",
  );
}

/** 最小 PDF（DOM 级 embed 断言用；headless 不验证插件渲染内容）。 */
const TINY_PDF = Buffer.from(
  "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n",
  "ascii",
);

/** 最小可播放 WAV（RIFF/PCM，0.1s 静音段）——audio 位占位形态用。 */
function tinyWav(): Buffer {
  const sampleRate = 8000;
  const dataBytes = (sampleRate / 10) * 2;
  const buf = Buffer.alloc(44 + dataBytes);
  buf.write("RIFF", 0, "ascii");
  buf.writeUInt32LE(36 + dataBytes, 4);
  buf.write("WAVE", 8, "ascii");
  buf.write("fmt ", 12, "ascii");
  buf.writeUInt32LE(16, 16);
  buf.writeUInt16LE(1, 20);
  buf.writeUInt16LE(1, 22);
  buf.writeUInt32LE(sampleRate, 24);
  buf.writeUInt32LE(sampleRate * 2, 28);
  buf.writeUInt16LE(2, 32);
  buf.writeUInt16LE(16, 34);
  buf.write("data", 36, "ascii");
  buf.writeUInt32LE(dataBytes, 40);
  return buf;
}

const FILES = new Map<string, { body: Buffer; type: string }>([
  ["/files/d6-cover.png", { body: solidPng(300, 400, [99, 102, 241]), type: "image/png" }],
  ["/files/d6-gallery.png", { body: solidPng(240, 320, [16, 185, 129]), type: "image/png" }],
  ["/files/d6-bracket.stl", { body: tinyStl(), type: "model/stl" }],
  ["/files/d6-readme.pdf", { body: TINY_PDF, type: "application/pdf" }],
  ["/files/d6-score.pdf", { body: TINY_PDF, type: "application/pdf" }],
  ["/files/d6-narration.wav", { body: tinyWav(), type: "audio/wav" }],
]);

/* ── 内容夹具 ─────────────────────────────────────────────────────────────── */

function att(id: number, fileType: string, url: string, name: string, size: number, extra: Record<string, unknown> = {}) {
  return {
    id,
    content_item_id: 0,
    file_type: fileType,
    oss_key: `uploads/d6/${name}`,
    oss_url: url,
    original_file_name: name,
    file_size: size,
    scan_status: "clean",
    sort_order: 0,
    ...extra,
  };
}

function detail(contentType: string, title: string, attachments: unknown[]) {
  return {
    content: {
      id: 0,
      title,
      zone: "original",
      content_type: contentType,
      category: "material",
      status: "published",
      author: { id: 42, username: "Ada" },
      description: "D6 放大态多类型化契约",
      created_at: "2026-10-11T00:00:00Z",
      allow_copy: true,
    },
    attachments,
    tags: [],
  };
}

/* 9501 混排五位链：image(0) / model3d(1) / text_pdf(2) / audio(3) / sheet_music_pdf(4)。
 * 可放大序列 = [image, model3d, text_pdf]。 */
const D6_CONTENTS: Array<{ id: number; fixture: Record<string, unknown> }> = [
  {
    id: 9501,
    fixture: detail("template", "D6 混排放大态验证", [
      att(9511, "image", `${BASE}/files/d6-gallery.png`, "d6-gallery.png", 2048, { width: 240, height: 320 }),
      att(9512, "model3d", `${BASE}/files/d6-bracket.stl`, "d6-bracket.stl", 1024, { scan_status: "not_required", sort_order: 1 }),
      att(9513, "text", `${BASE}/files/d6-readme.pdf`, "d6-readme.pdf", 1024, { mime_type: "application/pdf", sort_order: 2 }),
      att(9514, "audio", `${BASE}/files/d6-narration.wav`, "d6-narration.wav", 2048, { sort_order: 3 }),
      att(9515, "sheet_music_pdf", `${BASE}/files/d6-score.pdf`, "d6-score.pdf", 2048, { sort_order: 4 }),
    ]),
  },
  {
    id: 9502,
    fixture: detail("sheet_music", "D6 乐谱 pdf 无放大入口验证", [
      att(9521, "sheet_music_pdf", `${BASE}/files/d6-score.pdf`, "d6-score.pdf", 2048),
    ]),
  },
  {
    id: 9503,
    fixture: detail("image", "D6 图片缩放零回归验证", [
      att(9531, "image", `${BASE}/files/d6-gallery.png`, "d6-gallery.png", 2048, { width: 240, height: 320 }),
      att(9532, "image", `${BASE}/files/d6-cover.png`, "d6-cover.png", 2048, { width: 300, height: 400, sort_order: 1 }),
    ]),
  },
];

const FEED_CARDS = D6_CONTENTS.map(({ id, fixture }) => {
  const content = (fixture as { content: { id: number; title: string; zone: string; content_type: string; category: string; status: string; author: { id: number; username: string } } }).content;
  return {
    id,
    title: content.title,
    zone: "original",
    content_type: content.content_type,
    category: content.category,
    status: "published",
    author: content.author,
    like_count: 1,
  };
});

/* ── mock 服务（SSR 与客户端同源 18080） ─────────────────────────────────── */

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

function fixtureFor(id: number): Record<string, unknown> | null {
  const found = D6_CONTENTS.find(({ id: candidate }) => candidate === id);
  if (!found) return null;
  const fixture = JSON.parse(JSON.stringify(found.fixture)) as Record<string, unknown>;
  (fixture.content as Record<string, unknown>).id = found.id;
  for (const attachment of fixture.attachments as Array<Record<string, unknown>>) {
    attachment.content_item_id = found.id;
  }
  return fixture;
}

function handleApi(req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? "/", BASE);
  const contentMatch = url.pathname.match(/^\/api\/v1\/contents\/(\d+)$/);
  if (contentMatch) {
    const fixture = fixtureFor(Number(contentMatch[1]));
    if (fixture) return json(res, 200, fixture);
  }
  const file = FILES.get(url.pathname);
  if (file && req.method === "GET") {
    res.writeHead(200, {
      "content-type": file.type,
      "content-length": String(file.body.length),
      "access-control-allow-origin": "*",
    });
    res.end(file.body);
    return;
  }
  if (url.pathname === "/api/v1/contents" && url.searchParams.get("zone") === "original") {
    return json(res, 200, { contents: FEED_CARDS, total: FEED_CARDS.length, page: 1, page_size: 24 });
  }
  return json(res, 404, { code: "NOT_FOUND", message: url.pathname });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

async function mockClientApis(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/auth/me", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ unread_counts: { total: 0 } }) }));
  await mockApiRoute(page, "**/api/v1/users/me/history", (route) => route.fulfill({ status: 401, contentType: "application/json", body: JSON.stringify({ code: "UNAUTHORIZED" }) }));
  await mockApiRoute(page, "**/api/v1/social/comments?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ comments: [] }) }));
  await mockApiRoute(page, "**/api/v1/social/reactions?**", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ counts: { like: 0, dislike: 0 }, user_reaction: null }) }));
  for (const { id } of D6_CONTENTS) {
    await mockApiRoute(page, `**/api/v1/contents/${id}/related-fanworks?**`, (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));
  }
  /* 浮层层内详情拉取（客户端）：与 SSR 同源同夹具（guard 只放行已注册模式）。 */
  await mockApiRoute(page, "**/api/v1/contents/95*", (route) => {
    const url = new URL(route.request().url());
    const match = url.pathname.match(/^\/api\/v1\/contents\/(\d+)$/);
    const fixture = match ? fixtureFor(Number(match[1])) : null;
    if (!fixture) return route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ code: "NOT_FOUND" }) });
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(fixture) });
  });
  /* 客户端 feed 再验证返回同一列表（卡片不因空响应消失）；相似内容行（sort=hot）为空。 */
  await mockApiRoute(page, "**/api/v1/contents?**", (route) => {
    const url = new URL(route.request().url());
    if (url.searchParams.get("sort") === "hot") {
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) });
    }
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: FEED_CARDS, total: FEED_CARDS.length, page: 1, page_size: 24 }) });
  });
  /* 图片优化器：png 字节回源。 */
  await page.route("**/_next/image?*", (route) => {
    const reqUrl = new URL(route.request().url());
    const target = decodeURIComponent(reqUrl.searchParams.get("url") ?? "");
    const file = FILES.get(new URL(target).pathname);
    if (!file) return route.fulfill({ status: 404 });
    return route.fulfill({ status: 200, contentType: file.type, body: file.body });
  });
}

async function openOverlay(page: Page, title: string) {
  await page.goto("/original");
  await expect(page.getByRole("article").filter({ hasText: title })).toBeVisible({ timeout: 15_000 });
  await page.getByRole("article").filter({ hasText: title }).locator("button").first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("heading", { level: 1, name: title })).toBeVisible({ timeout: 10_000 });
  return dialog;
}

function pane(page: Page) {
  return page.locator('[data-slot="variant-media-pane"]');
}

function viewerDialog(page: Page) {
  /* 查看器 dialog 经 createPortal 挂 body、含 viewer-stage；浮层本体 dialog 无此槽。 */
  return page.locator('dialog:has([data-slot="viewer-stage"])');
}

/** 查看器序号（media.viewer.position = 「第 X / 共 N」）。 */
async function viewerPosition(page: Page): Promise<string> {
  const text = await viewerDialog(page).locator('[data-slot="viewer-position"]').textContent();
  return (text ?? "").trim();
}

async function paneNext(page: Page) {
  await pane(page).hover();
  await page.getByRole("button", { name: "下一张" }).click();
}

/* ── 用例 ─────────────────────────────────────────────────────────────────── */

test.use({ locale: "zh-CN" });

test.describe("D6 #863 放大态多类型化（mocked 契约）", () => {
  test("混排链：3D 位放大钮打开即定位该 3D，放大态内独立计数 + 边界 clamp", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D6 混排放大态验证");

    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 5", { timeout: 10_000 });
    /* 切到 model3d 位（2/5）。 */
    await paneNext(page);
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("2 / 5");
    await expect(pane(page).locator('[data-testid="model-viewer-ready"], [data-testid="model-viewer-error"]')).toBeVisible({ timeout: 20_000 });

    /* 右上放大钮（唯一入口）：打开后当前项 = 该 3D（稳定 id 映射，不是首图）。 */
    await pane(page).hover();
    await pane(page).locator('[data-slot="media-enlarge-button"]').click();
    await expect(viewerDialog(page)).toBeVisible({ timeout: 10_000 });
    await expect(viewerDialog(page).locator('[data-testid="model-viewer-ready"], [data-testid="model-viewer-error"]')).toBeVisible({ timeout: 20_000 });
    expect(await viewerPosition(page)).toBe("第 2 / 共 3");
    await page.screenshot({ path: path.join(SCREENSHOTS, "d6-viewer-model3d-fullscreen.png") });

    /* 放大态内 next → pdf（3/3），边界 clamp。 */
    await viewerDialog(page).getByRole("button", { name: "下一张" }).click();
    const embed = viewerDialog(page).locator('embed[type="application/pdf"]');
    await expect(embed).toBeAttached({ timeout: 10_000 });
    expect(await embed.getAttribute("src")).toContain("d6-readme.pdf");
    expect(await viewerPosition(page)).toBe("第 3 / 共 3");
    await expect(viewerDialog(page).getByRole("button", { name: "下一张" })).toBeDisabled();
    await page.screenshot({ path: path.join(SCREENSHOTS, "d6-viewer-pdf-fullscreen.png") });

    /* prev → prev → 首图（1/3）。 */
    await viewerDialog(page).getByRole("button", { name: "上一张" }).click();
    await viewerDialog(page).getByRole("button", { name: "上一张" }).click();
    await expect(viewerDialog(page).locator("img").first()).toBeVisible({ timeout: 10_000 });
    expect(await viewerPosition(page)).toBe("第 1 / 共 3");

    /* Esc 关闭：左列回到最后所看项（首图位 1/5），焦点恢复到放大钮触发点。 */
    await page.keyboard.press("Escape");
    await expect(viewerDialog(page)).toHaveCount(0);
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 5", { timeout: 10_000 });
    await expect
      .poll(async () => page.evaluate(() => document.activeElement?.getAttribute("data-slot") ?? ""))
      .toBe("media-enlarge-button");
  });

  test("关闭回最后所看项：Esc 关闭查看器后左列定位 pdf 位 + 焦点恢复", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D6 混排放大态验证");

    /* image 位主体点击（双入口之一）开查看器。 */
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 5", { timeout: 10_000 });
    await pane(page).locator('[data-slot="detail-cover"]').click({ position: { x: 200, y: 300 } });
    await expect(viewerDialog(page)).toBeVisible({ timeout: 10_000 });
    expect(await viewerPosition(page)).toBe("第 1 / 共 3");

    /* 翻到 pdf（3/3）后 Esc：只关查看器，左列定位 pdf 位（text_pdf = 3/5）。 */
    await viewerDialog(page).getByRole("button", { name: "下一张" }).click();
    await viewerDialog(page).getByRole("button", { name: "下一张" }).click();
    await expect(viewerDialog(page).locator('embed[type="application/pdf"]')).toBeAttached({ timeout: 10_000 });
    await page.keyboard.press("Escape");
    await expect(viewerDialog(page)).toHaveCount(0);
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("3 / 5", { timeout: 10_000 });
    /* 焦点恢复到触发媒体区（浮层仍开、Esc 未退层）。 */
    await expect(page.getByRole("dialog").getByRole("heading", { level: 1, name: "D6 混排放大态验证" })).toBeVisible();
  });

  test("无放大入口位：乐谱 pdf 位无放大钮、主体点击不开查看器", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D6 乐谱 pdf 无放大入口验证");

    const scoreEmbed = pane(page).locator('embed[type="application/pdf"]');
    await expect(scoreEmbed).toBeAttached({ timeout: 10_000 });
    expect(await pane(page).locator('[data-slot="media-enlarge-button"]').count()).toBe(0);
    await pane(page).locator('[data-slot="detail-cover"]').click({ position: { x: 220, y: 180 } });
    await expect(viewerDialog(page)).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOTS, "d6-sheet-music-pdf-no-entry.png") });
  });

  test("audio 位无放大入口（混排链第 4 位）", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D6 混排放大态验证");

    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 5", { timeout: 10_000 });
    for (let i = 0; i < 3; i += 1) {
      await paneNext(page);
    }
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("4 / 5");
    await expect(pane(page).locator('[data-testid="audio-player"]')).toBeVisible({ timeout: 10_000 });
    expect(await pane(page).locator('[data-slot="media-enlarge-button"]').count()).toBe(0);
    await expect(viewerDialog(page)).toHaveCount(0);
  });

  test("图片缩放平移零回归：查看器内 1–5x 按钮缩放 + 双击复位", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D6 图片缩放零回归验证");

    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 2", { timeout: 10_000 });
    await pane(page).locator('[data-slot="detail-cover"]').click({ position: { x: 200, y: 300 } });
    const viewer = viewerDialog(page);
    await expect(viewer).toBeVisible({ timeout: 10_000 });
    const img = viewer.locator("img").first();
    await expect(img).toBeVisible({ timeout: 10_000 });

    const transform = async () => (await img.getAttribute("style")) ?? "";
    await viewer.getByRole("button", { name: "放大", exact: true }).click();
    expect(await transform()).toContain("scale(1.5)");
    await viewer.getByRole("button", { name: "放大", exact: true }).click();
    expect(await transform()).toContain("scale(2.25)");
    await viewer.getByRole("button", { name: "重置缩放" }).click();
    expect(await transform()).toContain("scale(1)");
    await page.screenshot({ path: path.join(SCREENSHOTS, "d6-viewer-image-zoom.png") });

    /* 关闭回最后所看项（仍在首图位）。 */
    await page.keyboard.press("Escape");
    await expect(viewerDialog(page)).toHaveCount(0);
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 2");
  });
});
