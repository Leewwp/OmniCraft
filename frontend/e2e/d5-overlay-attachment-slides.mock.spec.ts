import { expect, test, type Page } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { deflateSync } from "node:zlib";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

/* ────────────────────────────────────────────────────────────────────────────
 * D5 #862 可预览媒体链与逐类型轮播位——mocked 契约（真 Chromium）：
 * 附件族内容（audio/model3d/document/text-pdf/sheet_music/混排/>9 图集）
 * 在详情浮层左列呈现对应 slide 形态，右栏只留下载/扫描语义（无重复查看器）；
 * 三件套切换、>9 收敛、切位停播（audio）、混排角标。
 * 本地无真实 OSS：签名 oss_url 指向本套件 18080 mock 服务的合成媒体字节
 * （tiny wav/stl/csv/musicxml/midi/pdf/png）；pdf embed 与 3D 画布按 DOM 级
 * 形态断言（headless 下插件渲染不可截图验证）。
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

/** 最小可播放 WAV（RIFF/PCM，0.1s 静音段）。 */
function tinyWav(): Buffer {
  const sampleRate = 8000;
  const dataBytes = sampleRate / 10 * 2;
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
  for (let i = 0; i < dataBytes / 2; i += 1) {
    buf.writeInt16LE(Math.round(6000 * Math.sin((2 * Math.PI * 440 * i) / sampleRate)), 44 + i * 2);
  }
  return buf;
}

/** ASCII STL（单三角面）——ModelViewer STLLoader 可解析。 */
function tinyStl(): Buffer {
  return Buffer.from(
    `solid d5
facet normal 0 0 1
  outer loop
    vertex 0 0 0
    vertex 10 0 0
    vertex 0 10 0
  endloop
endfacet
endsolid d5
`,
    "ascii",
  );
}

/** 最小 MusicXML（单小节全音符）——OSMD 可渲染出谱面 SVG。 */
const MUSICXML = `<?xml version="1.0" encoding="UTF-8"?>
<score-partwise version="3.1">
  <part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>
  <part id="P1">
    <measure number="1">
      <attributes><divisions>1</divisions><time><beats>4</beats><beat-type>4</beat-type></time><clef><sign>G</sign><line>2</line></clef></attributes>
      <note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><type>whole</type></note>
    </measure>
  </part>
</score-partwise>
`;

/** 最小 MIDI（format 0 单轨：tempo + 一个音符）——midi-player-js 可解析。 */
function tinyMidi(): Buffer {
  const header = Buffer.alloc(14);
  header.write("MThd", 0, "ascii");
  header.writeUInt32BE(6, 4);
  header.writeUInt16BE(0, 8);
  header.writeUInt16BE(1, 10);
  header.writeUInt16BE(480, 12);
  const trackBody = Buffer.from([
    0x00, 0xff, 0x51, 0x03, 0x07, 0xa1, 0x20,
    0x00, 0x90, 0x3c, 0x64,
    0x83, 0x60, 0x80, 0x3c, 0x40,
    0x00, 0xff, 0x2f, 0x00,
  ]);
  const trackHeader = Buffer.alloc(8);
  trackHeader.write("MTrk", 0, "ascii");
  trackHeader.writeUInt32BE(trackBody.length, 4);
  return Buffer.concat([header, trackHeader, trackBody]);
}

/** 最小 PDF（DOM 级 embed 断言用；headless 不验证插件渲染内容）。 */
const TINY_PDF = Buffer.from(
  "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n",
  "ascii",
);

const FILES = new Map<string, { body: Buffer; type: string }>([
  ["/files/narration-a.wav", { body: tinyWav(), type: "audio/wav" }],
  ["/files/narration-b.wav", { body: tinyWav(), type: "audio/wav" }],
  ["/files/bracket.stl", { body: tinyStl(), type: "model/stl" }],
  ["/files/sheet.csv", { body: Buffer.from("part,qty,note\nbracket,2,left side\n", "ascii"), type: "text/csv" }],
  ["/files/readme.pdf", { body: TINY_PDF, type: "application/pdf" }],
  ["/files/score.musicxml", { body: Buffer.from(MUSICXML, "ascii"), type: "application/vnd.recordare.musicxml+xml" }],
  ["/files/score.mscz", { body: Buffer.from("mscz-bytes", "ascii"), type: "application/octet-stream" }],
  ["/files/mixed.png", { body: solidPng(300, 400, [99, 102, 241]), type: "image/png" }],
]);
for (let i = 0; i < 10; i += 1) {
  FILES.set(`/files/gallery-${i}.png`, { body: solidPng(240, 320, [16 + i * 20, 185, 129]), type: "image/png" });
}

/* ── 内容夹具 ─────────────────────────────────────────────────────────────── */

function att(id: number, fileType: string, url: string, name: string, size: number, extra: Record<string, unknown> = {}) {
  return {
    id,
    content_item_id: 0,
    file_type: fileType,
    oss_key: `uploads/d5/${name}`,
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
      description: "D5 附件族轮播位契约",
      created_at: "2026-10-11T00:00:00Z",
      allow_copy: true,
    },
    attachments,
    tags: [],
  };
}

const D5_CONTENTS: Array<{ id: number; fixture: Record<string, unknown> }> = [
  {
    id: 9301,
    fixture: detail("audio", "D5 音频位验证", [
      att(9311, "audio", `${BASE}/files/narration-a.wav`, "narration-a.wav", 2048),
    ]),
  },
  {
    id: 9302,
    fixture: detail("3d_print", "D5 模型位验证", [
      att(9321, "model3d", `${BASE}/files/bracket.stl`, "bracket.stl", 1024, { scan_status: "not_required" }),
    ]),
  },
  {
    id: 9303,
    fixture: detail("template", "D5 文档位验证", [
      att(9331, "document", `${BASE}/files/sheet.csv`, "sheet.csv", 512),
    ]),
  },
  {
    id: 9304,
    fixture: detail("other", "D5 文本 PDF 位验证", [
      att(9341, "text", `${BASE}/files/readme.pdf`, "readme.pdf", 1024, { mime_type: "application/pdf" }),
      att(9342, "text", `${BASE}/files/notes.txt`, "notes.txt", 256, { mime_type: "text/plain" }),
    ]),
  },
  {
    id: 9305,
    fixture: detail("sheet_music", "D5 乐谱位验证", [
      att(9351, "sheet_music_xml", `${BASE}/files/score.musicxml`, "score.musicxml", 2048),
      att(9352, "sheet_music_mscz", `${BASE}/files/score.mscz`, "score.mscz", 4096),
    ]),
  },
  {
    id: 9306,
    fixture: detail("template", "D5 混排链验证", [
      att(9361, "image", `${BASE}/files/mixed.png`, "mixed.png", 2048, { width: 300, height: 400 }),
      att(9362, "document", `${BASE}/files/sheet.csv`, "sheet.csv", 512),
    ]),
  },
  {
    id: 9307,
    fixture: detail(
      "image",
      "D5 溢出图集验证",
      Array.from({ length: 10 }, (_, i) =>
        att(9370 + i, "image", `${BASE}/files/gallery-${i}.png`, `gallery-${i}.png`, 1024, { width: 240, height: 320, sort_order: i }),
      ),
    ),
  },
  {
    id: 9308,
    fixture: detail("audio", "D5 音频切位停播验证", [
      att(9381, "audio", `${BASE}/files/narration-a.wav`, "narration-a.wav", 2048),
      att(9382, "audio", `${BASE}/files/narration-b.wav`, "narration-b.wav", 2048, { sort_order: 1 }),
    ]),
  },
];

const FEED_CARDS = D5_CONTENTS.map(({ id, fixture }) => {
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
  const found = D5_CONTENTS.find(({ id: candidate }) => candidate === id);
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
  for (const { id } of D5_CONTENTS) {
    await mockApiRoute(page, `**/api/v1/contents/${id}/related-fanworks?**`, (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }));
  }
  /* 浮层层内详情拉取（客户端）：与 SSR 同源同夹具（guard 只放行已注册模式）。 */
  await mockApiRoute(page, "**/api/v1/contents/93*", (route) => {
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

function rightRail(page: Page) {
  return page.locator('[data-slot="layer-scroller"]');
}

/* ── 用例 ─────────────────────────────────────────────────────────────────── */

test.use({ locale: "zh-CN" });

test.describe("D5 #862 附件族轮播位（mocked 契约）", () => {
  test("audio 位：占位图 + 悬浮播放条；右栏只留下载语义", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    const dialog = await openOverlay(page, "D5 音频位验证");

    await expect(pane(page).locator('[data-testid="audio-player"]')).toBeVisible({ timeout: 10_000 });
    await expect(pane(page).locator("img").first()).toBeVisible();
    await expect(pane(page).locator('[data-slot="media-refresh-preview"]')).toBeVisible();

    const rail = rightRail(page);
    await expect(rail.locator('[data-testid="attachment-preview-entry"]')).toHaveCount(0);
    await expect(rail.locator('[data-testid="audio-player"]')).toHaveCount(0);
    await expect(rail.getByText("narration-a.wav")).toBeVisible();
    await expect(dialog.locator('[data-slot="media-paging-dots"]')).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-audio-slide.png") });
  });

  test("model3d 位：STL 画布挂载；右栏无入口卡", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D5 模型位验证");

    await expect(pane(page).locator('[data-testid="model-viewer-ready"], [data-testid="model-viewer-error"]')).toBeVisible({ timeout: 20_000 });
    await expect(rightRail(page).locator('[data-testid="attachment-preview-entry"]')).toHaveCount(0);
    await expect(rightRail(page).getByText("bracket.stl")).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-model3d-slide.png") });
  });

  test("document 位：csv 网格渲染；右栏无入口卡", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D5 文档位验证");

    await expect(pane(page).locator('[data-testid="document-viewer-grid"]')).toBeVisible({ timeout: 15_000 });
    await expect(pane(page).getByText("bracket")).toBeVisible();
    await expect(rightRail(page).locator('[data-testid="attachment-preview-entry"]')).toHaveCount(0);
    await expect(rightRail(page).getByText("sheet.csv")).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-document-slide.png") });
  });

  test("text-pdf 位：原生 embed 进轮播；txt 留右栏下载行", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D5 文本 PDF 位验证");

    const embed = pane(page).locator('embed[type="application/pdf"]');
    await expect(embed).toBeAttached({ timeout: 10_000 });
    expect(await embed.getAttribute("src")).toContain("readme.pdf");
    /* txt（text/plain）不进链：右栏通用下载行保留两个 text 附件的下载语义。 */
    const rail = rightRail(page);
    await expect(rail.getByText("notes.txt")).toBeVisible();
    await expect(rail.getByText("readme.pdf")).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-text-pdf-slide.png") });
  });

  test("sheet_music 位：musicxml → OSMD 谱面；mscz 留右栏下载行", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D5 乐谱位验证");

    await expect(pane(page).locator(".osmd-container svg")).toBeVisible({ timeout: 20_000 });
    const rail = rightRail(page);
    await expect(rail.locator(".osmd-container")).toHaveCount(0);
    await expect(rail.getByText("score.mscz")).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-sheet-music-slide.png") });
  });

  test("混排链：图+文档逐 slide 类型角标与三件套切换；查看器只收图片", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D5 混排链验证");

    await expect(pane(page).locator('[data-slot="media-type-badge"]')).toBeVisible({ timeout: 10_000 });
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 2");

    /* 第二位 = 文档：角标仍在、网格挂载。 */
    await pane(page).hover();
    await page.getByRole("button", { name: "下一张" }).click();
    await expect(pane(page).locator('[data-testid="document-viewer-grid"]')).toBeVisible({ timeout: 15_000 });
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("2 / 2");

    /* 查看器位主体点击不开查看器（embed/网格原生输入不抢）；detail-cover 在
       右栏行内封面（隐藏于桌面但仍在 DOM）也有一份——锚定到浮层媒体列。 */
    /* 点击落点避开悬浮返回钮（媒体列左上）与翻页箭头（两侧垂直中位）。 */
    await pane(page).locator('[data-slot="detail-cover"]').click({ position: { x: 220, y: 180 } });
    await expect(page.locator("dialog[open] canvas, dialog[open] img")).toHaveCount(0);

    /* 回到图片位：点击进查看器（MediaViewer 只收 image slides）。 */
    await page.getByRole("button", { name: "上一张" }).click();
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 2");
    await pane(page).locator('[data-slot="detail-cover"]').click({ position: { x: 220, y: 180 } });
    await expect(page.locator("dialog[open] img").first()).toBeVisible({ timeout: 10_000 });
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-mixed-chain.png") });
  });

  test(">9 图集：隐藏圆点、保留 N/M 徽标与箭头", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D5 溢出图集验证");

    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("1 / 10", { timeout: 10_000 });
    await expect(page.locator('[data-slot="media-paging-dots"]')).toHaveCount(0);
    await pane(page).hover();
    await expect(page.getByRole("button", { name: "下一张" })).toBeVisible();
    await page.getByRole("button", { name: "下一张" }).click();
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("2 / 10");
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-overflow-10.png") });
  });

  test("audio 切位停播：切到第二位后旧播放器卸载（DOM 级停播断言）", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await mockClientApis(page);
    await openOverlay(page, "D5 音频切位停播验证");

    const first = pane(page).locator('[data-testid="audio-player"]');
    await expect(first).toBeVisible({ timeout: 10_000 });
    /* 用户起播（真实手势，绕过自动播放策略）。 */
    await pane(page).locator('[data-testid="audio-play-button"]').click();
    await expect
      .poll(async () => pane(page).locator("audio").evaluate((el) => (el as HTMLAudioElement).paused))
      .toBe(false);

    /* 切位：旧播放器卸载（当前位单挂载）→ 至多一段声音。 */
    await pane(page).hover();
    await page.getByRole("button", { name: "下一张" }).click();
    const second = pane(page).locator('[data-testid="audio-player"]');
    await expect(second).toBeVisible();
    await expect(pane(page).locator("audio")).toHaveCount(1);
    await expect
      .poll(async () => pane(page).locator("audio").evaluate((el) => (el as HTMLAudioElement).paused))
      .toBe(true);
    await expect(pane(page).locator('[data-slot="media-count-badge"]')).toContainText("2 / 2");
    await page.screenshot({ path: path.join(SCREENSHOTS, "d5-overlay-audio-switch.png") });
  });
});
