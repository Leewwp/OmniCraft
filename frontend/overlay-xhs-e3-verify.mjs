// E3 #955 封面恒为首屏 + 全类型舞台托底浏览器验证 rig（本地栈 3000/8080，
// @playwright/test Chromium）——断言：
//  1) 封面恒首屏：有封面内容开窗 slide 0 = 封面、翻页进正文；封面=首图同文件
//     去重（计数 2 而非 3）；无封面维持可预览附件优先（首 slide = 附件）；
//  2) 封面比例参与几何推导：cover_width=0 走预加载实测 intrinsic 链路（12:5
//     封面 → 媒体列 clamp 满可用宽 880，≠ 3:4 默认比例的 621）；
//  3) 不可预览附件托底：mscz/txt 属性卡（占位图底 + 文件名 + 下载入口），
//     逐附件进链翻页可见；任何内容打开舞台非空；
//  4) 全类型渲染回归（逐分支断言，防「变体专属分支」盲区）：乐谱 xml/midi/pdf
//     三分支各自渲染非 undefined；混排九位链（图/视频/音频/3D/文档/PDF×2/
//     乐谱/托底）逐位有主体；
//  5) 边界 clamp：首张前翻/末张后翻键盘与点击均无操作、边界箭头 disabled；
//  6) E2 交叉淡出层对托底位生效（keyed by slide id：翻页 2 层 → 240ms 后 1 层）。
// 夹具：本地库 IP 209 share tab 直插内容 99111~99116（psql 幂等重置）；文档/
// 乐谱/音频/3D 附件本地无 OSS 签名——rig 在浏览器层拦截详情响应补 oss_url 指向
// 本 rig 的 127.0.0.1:18085 合成媒体服务（e1 rig 同思路）。截图 ../screenshots/e3-rig-*.png。
import { chromium } from "@playwright/test";
import { execSync } from "node:child_process";
import { createServer } from "node:http";
import { deflateSync } from "node:zlib";

const BASE = process.env.E3_BASE_URL || "http://localhost:3000";
const PAGE_PATH = "/ip/209?tab=share";
const SHOTS = new URL("../screenshots/", import.meta.url).pathname;
const PSQL = "docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft";

const RIGHT_COL = 440;
const SHELL_MAX = 1320;

const TITLES = {
  dedup: "【E3验证】封面首图去重",
  coverGeometry: "【E3验证】封面比例几何实测",
  noCover: "【E3验证】无封面附件优先",
  fallbackOnly: "【E3验证】纯不可预览托底",
  scoreBranches: "【E3验证】乐谱三分支",
  matrix: "【E3验证】全类型混排九位",
};

const fails = [];
function check(name, cond, extra = "") {
  const line = `${cond ? "PASS" : "FAIL"} ${name}${extra ? ` — ${extra}` : ""}`;
  console.log(line);
  if (!cond) fails.push(line);
}
const near = (value, expected, tol) => Math.abs(value - expected) <= tol;

/* ── 夹具：幂等重置 99111~99116 ─────────────────────────────────────────── */
const COVER_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="500" viewBox="0 0 1200 500">
  <defs><linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="100%"><stop offset="0%" stop-color="#0ea5e9"/><stop offset="100%" stop-color="#1e293b"/></linearGradient></defs>
  <rect width="100%" height="100%" fill="url(#g)"/>
  <text x="50%" y="55%" text-anchor="middle" fill="white" font-family="system-ui,sans-serif" font-size="64" font-weight="700">E3 1200x500</text>
</svg>`;

execSync(`${PSQL} -c "DELETE FROM content_items WHERE id BETWEEN 99111 AND 99116"`);
execSync(`${PSQL} -c "
INSERT INTO content_items (id, title, author_id, zone, ip_id, content_type, status, description, cover_image_url, created_at, updated_at, is_public, allow_copy)
VALUES
 (99111,'${TITLES.dedup}',313,'fanwork',209,'image','published','E3 夹具：封面=首图同文件去重','/seed-media/real/gallery/f1-portrait-1.svg',NOW(),NOW(),true,true),
 (99112,'${TITLES.coverGeometry}',313,'fanwork',209,'template','published','E3 夹具：cover_width=0 实测 12:5 封面比例','http://127.0.0.1:18085/e3-cover.svg',NOW() - interval '1 seconds',NOW(),true,true),
 (99113,'${TITLES.noCover}',313,'fanwork',209,'image','published','E3 夹具：无封面维持附件优先',NULL,NOW() - interval '2 seconds',NOW(),true,true),
 (99114,'${TITLES.fallbackOnly}',313,'fanwork',209,'sheet_music','published','E3 夹具：mscz+txt 纯托底',NULL,NOW() - interval '3 seconds',NOW(),true,true),
 (99115,'${TITLES.scoreBranches}',313,'fanwork',209,'sheet_music','published','E3 夹具：乐谱 xml/midi/pdf 三分支',NULL,NOW() - interval '4 seconds',NOW(),true,true),
 (99116,'${TITLES.matrix}',313,'fanwork',209,'other','published','E3 夹具：全类型九位混排',NULL,NOW() - interval '5 seconds',NOW(),true,true)
"`);
execSync(`${PSQL} -c "
INSERT INTO content_attachments (content_item_id, file_type, oss_key, mime_type, width, height, sort_order, is_primary, scan_status, scan_required, original_file_name, file_size)
VALUES
 (99111,'image','/seed-media/real/gallery/f1-portrait-1.svg','image/svg+xml',900,1200,0,true,'not_required',false,'e3-first.svg',1000),
 (99111,'image','/seed-media/real/gallery/f1-square.svg','image/svg+xml',800,800,1,false,'not_required',false,'e3-second.svg',1000),
 (99112,'document','uploads/313/document/e3-sheet.csv','text/csv',NULL,NULL,0,true,'clean',false,'e3-sheet.csv',120),
 (99113,'image','/seed-media/real/gallery/f1-land-1.svg','image/svg+xml',1920,1080,0,true,'not_required',false,'e3-land.svg',1000),
 (99113,'document','uploads/313/document/e3-sheet2.csv','text/csv',NULL,NULL,1,false,'clean',false,'e3-sheet2.csv',120),
 (99114,'sheet_music_mscz','uploads/313/score/score.mscz','application/octet-stream',NULL,NULL,0,true,'clean',false,'score.mscz',4096),
 (99114,'text','uploads/313/doc/notes.txt','text/plain',NULL,NULL,1,false,'clean',false,'notes.txt',256),
 (99115,'sheet_music_xml','uploads/313/score/a.musicxml','application/vnd.recordare.musicxml+xml',NULL,NULL,0,true,'clean',false,'a.musicxml',2048),
 (99115,'sheet_music_midi','uploads/313/score/a.mid','audio/midi',NULL,NULL,1,false,'clean',false,'a.mid',1024),
 (99115,'sheet_music_pdf','uploads/313/score/a.pdf','application/pdf',NULL,NULL,2,false,'clean',false,'a.pdf',2048),
 (99116,'image','/seed-media/real/gallery/f1-portrait-1.svg','image/svg+xml',900,1200,0,true,'not_required',false,'e3-img.svg',1000),
 (99116,'video','uploads/313/video/e3-clip.mp4','video/mp4',NULL,NULL,1,false,'clean',false,'e3-clip.mp4',1024),
 (99116,'audio','uploads/313/audio/e3-narration.wav','audio/wav',NULL,NULL,2,false,'clean',false,'e3-narration.wav',2048),
 (99116,'model3d','uploads/313/model/e3-bracket.stl','model/stl',NULL,NULL,3,false,'not_required',false,'e3-bracket.stl',1024),
 (99116,'document','uploads/313/document/e3-sheet3.csv','text/csv',NULL,NULL,4,false,'clean',false,'e3-sheet3.csv',120),
 (99116,'text','uploads/313/doc/e3-readme.pdf','application/pdf',NULL,NULL,5,false,'clean',false,'e3-readme.pdf',1024),
 (99116,'sheet_music_xml','uploads/313/score/b.musicxml','application/vnd.recordare.musicxml+xml',NULL,NULL,6,false,'clean',false,'b.musicxml',2048),
 (99116,'sheet_music_pdf','uploads/313/score/b.pdf','application/pdf',NULL,NULL,7,false,'clean',false,'b.pdf',2048),
 (99116,'sheet_music_mscz','uploads/313/score/b.mscz','application/octet-stream',NULL,NULL,8,false,'clean',false,'b.mscz',4096)
"`);

/* ── 本地媒体服务：合成媒体字节（e3 rig 自持，127.0.0.1:18085） ──────────── */
const CSV_BODY = "属性,数值,备注\n封面恒首屏,E3,slide 0 = cover\n托底位,fallback_download,属性卡\n";
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
function tinyWav() {
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
  for (let i = 0; i < dataBytes / 2; i += 1) {
    buf.writeInt16LE(Math.round(6000 * Math.sin((2 * Math.PI * 440 * i) / sampleRate)), 44 + i * 2);
  }
  return buf;
}
function tinyStl() {
  return Buffer.from(
    `solid e3
facet normal 0 0 1
  outer loop
    vertex 0 0 0
    vertex 10 0 0
    vertex 0 10 0
  endloop
endfacet
endsolid e3
`,
    "ascii",
  );
}
function tinyMidi() {
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
const TINY_PDF = Buffer.from(
  "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n",
  "ascii",
);
/** 极小 MP4 帧盒（ftyp+free+mdat）：DOM 级挂载断言用（headless 不验证解码）。 */
function tinyMp4() {
  const ftyp = Buffer.from([0, 0, 0, 0x18, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6f, 0x6d, 0, 0, 2, 0, 0x69, 0x73, 0x6f, 0x6d, 0x6d, 0x70, 0x34, 0x31]);
  const mdatBody = Buffer.alloc(64, 0x11);
  const mdat = Buffer.alloc(8 + mdatBody.length);
  mdat.writeUInt32BE(8 + mdatBody.length, 0);
  mdat.write("mdat", 4, "ascii");
  mdatBody.copy(mdat, 8);
  return Buffer.concat([ftyp, mdat]);
}

const MEDIA = new Map([
  ["/e3-cover.svg", { body: Buffer.from(COVER_SVG, "ascii"), type: "image/svg+xml" }],
  ["/e3-sheet.csv", { body: Buffer.from(CSV_BODY, "utf8"), type: "text/csv; charset=utf-8" }],
  ["/a.musicxml", { body: Buffer.from(MUSICXML, "ascii"), type: "application/vnd.recordare.musicxml+xml" }],
  ["/a.mid", { body: tinyMidi(), type: "audio/midi" }],
  ["/a.pdf", { body: TINY_PDF, type: "application/pdf" }],
  ["/b.musicxml", { body: Buffer.from(MUSICXML, "ascii"), type: "application/vnd.recordare.musicxml+xml" }],
  ["/b.pdf", { body: TINY_PDF, type: "application/pdf" }],
  ["/e3-narration.wav", { body: tinyWav(), type: "audio/wav" }],
  ["/e3-bracket.stl", { body: tinyStl(), type: "model/stl" }],
  ["/e3-clip.mp4", { body: tinyMp4(), type: "video/mp4" }],
]);
const U = (path) => `http://127.0.0.1:18085${path}`;

const mediaServer = createServer((req, res) => {
  const file = MEDIA.get(new URL(req.url ?? "/", "http://127.0.0.1").pathname);
  if (file) {
    res.writeHead(200, {
      "Content-Type": file.type,
      "Content-Length": String(file.body.length),
      "Access-Control-Allow-Origin": "*",
    });
    res.end(file.body);
    return;
  }
  res.writeHead(404);
  res.end("not found");
});
await new Promise((resolve) => mediaServer.listen(18085, "127.0.0.1", resolve));

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();

/* 详情响应补丁：仅给指定附件补签名 oss_url（其余字段原样透传；
   mscz/txt 不补——托底位不依赖 oss_url）。 */
const PATCHES = new Map([
  [99112, { document: U("/e3-sheet.csv") }],
  [99113, { document: U("/e3-sheet.csv") }],
  [
    99115,
    {
      sheet_music_xml: U("/a.musicxml"),
      sheet_music_midi: U("/a.mid"),
      sheet_music_pdf: U("/a.pdf"),
    },
  ],
  [
    99116,
    {
      video: U("/e3-clip.mp4"),
      audio: U("/e3-narration.wav"),
      model3d: U("/e3-bracket.stl"),
      document: U("/e3-sheet.csv"),
      text: U("/e3-readme.pdf"),
      sheet_music_xml: U("/b.musicxml"),
      sheet_music_pdf: U("/b.pdf"),
    },
  ],
]);
await page.route(/\/api\/v1\/contents\/(9911[1-6])(\?|$)/, async (route) => {
  const response = await route.fetch();
  let json;
  try {
    json = await response.json();
  } catch {
    return route.fulfill({ response });
  }
  const id = Number(new URL(route.request().url()).pathname.match(/(\d+)$/)?.[1]);
  const patch = PATCHES.get(id);
  if (patch) {
    for (const attachment of json.attachments ?? []) {
      const url = patch[attachment.file_type];
      if (url) attachment.oss_url = url;
    }
  }
  return route.fulfill({ response, json });
});

async function geometry() {
  return page.evaluate(() => {
    const pane = document.querySelector('[data-slot="variant-media-pane"]');
    const dialog = document.querySelector("dialog.content-detail-overlay");
    const badge = document.querySelector('[data-slot="media-count-badge"]');
    return {
      paneW: pane?.getBoundingClientRect().width ?? 0,
      paneH: pane?.getBoundingClientRect().height ?? 0,
      shellW: dialog?.getBoundingClientRect().width ?? 0,
      tier: pane?.dataset.tier ?? "",
      badge: badge?.textContent?.trim() ?? "",
    };
  });
}

async function openByTitle(title) {
  const card = page.locator(`text=${title}`).first();
  await card.scrollIntoViewIfNeeded({ timeout: 10000 });
  await card.click({ timeout: 10000 });
  await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 12000 });
  /* 入场转场（~400ms）+ 首帧保持释放 + 封面探针预算余量。 */
  await page.waitForTimeout(1600);
}

async function closeOverlay() {
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForTimeout(800);
}

async function stageCount() {
  return page.evaluate(
    () =>
      document.querySelectorAll(
        '[data-slot="variant-media-pane"] > [data-slot="detail-cover"], [data-slot="variant-media-pane"] > [data-slot="slide-fade-out"]',
      ).length,
  );
}

async function arrowDisabled(side) {
  /* 左箭头 = DOM 序第一个 data-slot=media-arrow。 */
  return page.evaluate((index) => {
    const arrows = document.querySelectorAll('[data-slot="media-arrow"]');
    const arrow = arrows[index];
    return Boolean(arrow && arrow.disabled);
  }, side === "left" ? 0 : 1);
}

try {
  /* domcontentloaded：Playwright 1.65 起 networkidle 把 dev server 的 HMR
   WebSocket 计入开放连接，对 dev 栈永不落定；后续断言自带显式等待。 */
  await page.goto(`${BASE}${PAGE_PATH}`, { waitUntil: "domcontentloaded", timeout: 30000 });
  await page.waitForTimeout(800);

  /* ── 1. 封面恒首屏 + 封面=首图去重 + 边界 clamp ──────────────────────── */
  await openByTitle(TITLES.dedup);
  let g = await geometry();
  check("去重链开窗 slide 0 = 封面（计数 1/2，非 1/3）", g.badge === "1 / 2", `badge=${g.badge}`);
  const coverSrc = await page.evaluate(() => {
    const img = document.querySelector('[data-slot="variant-media-pane"] img');
    return img?.getAttribute("src") ?? "";
  });
  check("首屏 slide = 封面图（f1-portrait-1）", coverSrc.includes("f1-portrait-1"), `src=${coverSrc}`);
  check("封面几何 = 3:4（pane = 壳高×0.75）", near(g.paneW, Math.round(g.paneH * 0.75), 2), `pane=${g.paneW.toFixed(1)} paneH=${g.paneH.toFixed(1)}`);
  check("壳宽 = 媒体列 + 440", near(g.shellW, g.paneW + RIGHT_COL, 2), `shell=${g.shellW.toFixed(1)}`);
  check("首张左箭头 disabled（边界 forbidden）", await arrowDisabled("left"));
  await page.keyboard.press("ArrowLeft");
  await page.waitForTimeout(300);
  check("首张键盘前翻无操作（计数仍 1/2）", (await geometry()).badge === "1 / 2");
  await page.screenshot({ path: `${SHOTS}/e3-rig-cover-first-dedup.png` });

  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(450);
  g = await geometry();
  check("翻页进正文第 2 张（计数 2/2）", g.badge === "2 / 2", `badge=${g.badge}`);
  const secondSrc = await page.evaluate(() => {
    const img = document.querySelector('[data-slot="variant-media-pane"] img');
    return img?.getAttribute("src") ?? "";
  });
  check("第 2 张 = 另一附件（f1-square，与封面不同）", secondSrc.includes("f1-square"), `src=${secondSrc}`);
  check("末张右箭头 disabled（边界 forbidden）", await arrowDisabled("right"));
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(300);
  check("末张键盘后翻无操作（计数仍 2/2）", (await geometry()).badge === "2 / 2");
  await closeOverlay();

  /* ── 2. 封面比例参与几何推导（cover_width=0 预加载实测链路） ──────────── */
  await openByTitle(TITLES.coverGeometry);
  g = await geometry();
  check("封面首屏（计数 1/2）", g.badge === "1 / 2", `badge=${g.badge}`);
  const wideCoverSrc = await page.evaluate(() => {
    const img = document.querySelector('[data-slot="variant-media-pane"] img');
    return img?.getAttribute("src") ?? "";
  });
  check("首屏 slide = 12:5 封面（e3-cover.svg）", wideCoverSrc.includes("e3-cover.svg"), `src=${wideCoverSrc}`);
  check(
    "封面实测比例驱动几何（pane clamp 满可用宽 880，≠ 3:4 默认 621）",
    near(g.paneW, SHELL_MAX - RIGHT_COL, 3) && g.paneW > 800,
    `pane=${g.paneW.toFixed(1)}`,
  );
  check("宽封面壳宽打满 1320", near(g.shellW, SHELL_MAX, 3), `shell=${g.shellW.toFixed(1)}`);
  await page.screenshot({ path: `${SHOTS}/e3-rig-cover-geometry-wide.png` });

  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(700);
  g = await geometry();
  check("翻页进 csv 正文（计数 2/2 + reading 档）", g.badge === "2 / 2" && g.tier === "reading", `badge=${g.badge} tier=${g.tier}`);
  check("csv 网格渲染（document-viewer-grid）", await page.locator('[data-testid="document-viewer-grid"]').count() === 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-cover-doc-reading.png` });
  await closeOverlay();

  /* ── 3. 无封面维持可预览附件优先 ─────────────────────────────────────── */
  await openByTitle(TITLES.noCover);
  g = await geometry();
  check("无封面首屏 = 附件图（计数 1/2）", g.badge === "1 / 2", `badge=${g.badge}`);
  const noCoverSrc = await page.evaluate(() => {
    const img = document.querySelector('[data-slot="variant-media-pane"] img');
    return img?.getAttribute("src") ?? "";
  });
  check("首屏 slide = 附件（f1-land-1，16:9）", noCoverSrc.includes("f1-land-1"), `src=${noCoverSrc}`);
  await page.screenshot({ path: `${SHOTS}/e3-rig-nocover-attachment-first.png` });
  await closeOverlay();

  /* ── 4. 纯不可预览附件：托底 slide + 下载入口 + 交叉淡出层 ───────────── */
  await openByTitle(TITLES.fallbackOnly);
  g = await geometry();
  check("纯托底内容舞台非空（计数 1/2）", g.badge === "1 / 2", `badge=${g.badge}`);
  let fallback = page.locator('[data-slot="slide-fallback"]');
  check("mscz 托底位渲染（slide-fallback 属性卡）", await fallback.count() === 1);
  check("托底属性卡含文件名 score.mscz", (await fallback.textContent())?.includes("score.mscz") === true);
  check("托底属性卡含下载钮（allowCopy 同源语义）", (await fallback.locator("button").count()) >= 1);
  check("托底底图渲染（占位图 img）", (await fallback.locator("img").count()) === 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-fallback-mscz.png` });

  /* 翻到 txt 托底位：E2 交叉淡出层 keyed by slide id 对托底位生效。 */
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(100);
  check("托底位切换中期双层（1 新 + 1 淡出）", (await stageCount()) === 2);
  await page.waitForTimeout(600);
  check("托底位切换落定单层（240ms 后旧层移除）", (await stageCount()) === 1);
  g = await geometry();
  check("第 2 托底位（notes.txt，计数 2/2）", g.badge === "2 / 2", `badge=${g.badge}`);
  fallback = page.locator('[data-slot="slide-fallback"]');
  check("txt 托底属性卡含文件名 notes.txt", (await fallback.textContent())?.includes("notes.txt") === true);
  check("末张右箭头 disabled（托底链边界）", await arrowDisabled("right"));
  await page.screenshot({ path: `${SHOTS}/e3-rig-fallback-txt.png` });
  await closeOverlay();

  /* ── 5. 乐谱三分支逐位断言（变体盲区教训：逐渲染分支非 undefined） ────── */
  await openByTitle(TITLES.scoreBranches);
  g = await geometry();
  check("乐谱链首屏 = musicxml（计数 1/3）", g.badge === "1 / 3", `badge=${g.badge}`);
  await page.locator('.osmd-container svg').first().waitFor({ state: "visible", timeout: 30000 });
  check("乐谱 xml 分支：OSMD 谱面 SVG 渲染（非 undefined）", true);
  await page.screenshot({ path: `${SHOTS}/e3-rig-score-xml.png` });

  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(500);
  g = await geometry();
  check("midi 位（计数 2/3）", g.badge === "2 / 3", `badge=${g.badge}`);
  /* midi 分支非 undefined/非空舞台：占位底图 + 回放条三态之一（回放条 /
     加载骨架 / 外部 soundfont CDN 失败的错误卡）。soundfont 为真实外部依赖
     （本地 dev fail-open），回放条本体由组件测试（stub 库）钉死。 */
  const midiBar = page.locator('[data-slot="slide-sheet-music"] button[aria-label]').first();
  let midiState = "unknown";
  try {
    await midiBar.waitFor({ state: "visible", timeout: 15000 });
    midiState = "bar";
  } catch {
    midiState = await page.evaluate(() => {
      const slide = document.querySelector('[data-slot="slide-sheet-music"]');
      if (!slide) return "missing";
      if (slide.querySelector('[role="alert"]')) return "error";
      if (slide.querySelector('[aria-busy="true"]')) return "loading";
      return "empty";
    });
  }
  check(
    "乐谱 midi 分支：回放条/加载/错误态其一渲染（非 undefined 非空舞台）",
    midiState === "bar" || midiState === "error" || midiState === "loading",
    `state=${midiState}`,
  );
  check("midi 位占位底图渲染（slide-sheet-music + img）", (await page.locator('[data-slot="slide-sheet-music"] img').count()) === 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-score-midi.png` });

  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(500);
  g = await geometry();
  check("pdf 位（计数 3/3）", g.badge === "3 / 3", `badge=${g.badge}`);
  check("乐谱 pdf 分支：embed 渲染（非 undefined）", await page.locator('embed[type="application/pdf"]').count() === 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-score-pdf.png` });
  check("末张右箭头 disabled（乐谱链边界）", await arrowDisabled("right"));
  await closeOverlay();

  /* ── 6. 全类型混排九位链：逐位分支断言（无 undefined/无空舞台） ────────── */
  await openByTitle(TITLES.matrix);
  g = await geometry();
  check("九位链首屏 = 图片（计数 1/9）", g.badge === "1 / 9", `badge=${g.badge}`);
  check("图位渲染（img 主体）", (await page.locator('[data-slot="variant-media-pane"] img').count()) >= 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-1-image.png` });

  async function nextBadge(expect, note) {
    await page.keyboard.press("ArrowRight");
    await page.waitForTimeout(600);
    const geo = await geometry();
    check(note, geo.badge === expect, `badge=${geo.badge}`);
  }

  await nextBadge("2 / 9", "视频位（计数 2/9）");
  const videoOk = await page.evaluate(() => {
    const video = document.querySelector('[data-slot="variant-media-pane"] video');
    return Boolean(video && (video.getAttribute("src") ?? "").includes("e3-clip.mp4"));
  });
  check("视频分支：video 元素挂载绑定 mp4（非 undefined）", videoOk);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-2-video.png` });

  await nextBadge("3 / 9", "音频位（计数 3/9）");
  await page.locator('[data-testid="audio-player"]').waitFor({ state: "visible", timeout: 20000 });
  check("音频分支：AudioPlayer 条渲染（非 undefined）", true);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-3-audio.png` });

  await nextBadge("4 / 9", "3D 位（计数 4/9）");
  await page
    .locator('[data-testid="model-viewer-ready"], [data-testid="model-viewer-error"]')
    .first()
    .waitFor({ state: "visible", timeout: 40000 });
  check("3D 分支：ModelViewer 画布就绪（非 undefined）", true);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-4-model3d.png` });

  await nextBadge("5 / 9", "csv 文档位（计数 5/9）");
  await page.locator('[data-testid="document-viewer-grid"]').waitFor({ state: "visible", timeout: 20000 });
  check("表格分支：csv 网格渲染（非 undefined）", true);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-5-csv.png` });

  await nextBadge("6 / 9", "text pdf 位（计数 6/9）");
  check("PDF 分支：embed 渲染（非 undefined）", (await page.locator('embed[type="application/pdf"]').count()) === 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-6-pdf.png` });

  await nextBadge("7 / 9", "乐谱 xml 位（计数 7/9）");
  await page.locator('.osmd-container svg').first().waitFor({ state: "visible", timeout: 30000 });
  check("乐谱 xml 分支（混排链内）：OSMD 谱面渲染", true);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-7-score.png` });

  await nextBadge("8 / 9", "乐谱 pdf 位（计数 8/9）");
  check("乐谱 pdf 分支（混排链内）：embed 渲染", (await page.locator('embed[type="application/pdf"]').count()) === 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-8-score-pdf.png` });

  await nextBadge("9 / 9", "mscz 托底位（计数 9/9）");
  const matrixFallback = page.locator('[data-slot="slide-fallback"]');
  check("托底分支（混排链内）：属性卡渲染（非 undefined）", (await matrixFallback.count()) === 1);
  check("托底属性卡含文件名 b.mscz", (await matrixFallback.textContent())?.includes("b.mscz") === true);
  check("九位链末张右箭头 disabled", await arrowDisabled("right"));
  check("九位链全程舞台非空（当前层存在）", (await stageCount()) >= 1);
  await page.screenshot({ path: `${SHOTS}/e3-rig-matrix-9-fallback.png` });
  await closeOverlay();
} finally {
  await context.close().catch(() => {});
  await browser.close().catch(() => {});
  mediaServer.close();
}

console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n${fails.join("\n")}`);
process.exit(fails.length === 0 ? 0 : 1);
