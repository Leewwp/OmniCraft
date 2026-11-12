// E4 #956 预取与就绪交换浏览器验证 rig（本地栈 3000/8080，@playwright/test
// Chromium）——断言（票面验收逐条）：
//  1) 相邻内容 idle 预取：开窗后空闲期出现前后项（contextList 相邻，next 优先）
//     详情 JSON 与封面 1080 规范变体请求（route/request 计数）；
//  2) 缓存命中：关闭后点开预取过的相邻内容 = 网络层零新详情请求（claim 消费
//     内存缓存），入场不被阻塞；
//  3) 弱网（route delay 模拟真 OSS 签名 URL 取数延迟）翻页：新 slide 图未就绪
//     期旧画面保持（淡出层满不透明、无淡出动画、旧图完整；新层 opacity 0），
//     中间帧采样非空白，就绪后 E2 双层交叉淡出交换（旧层移除、新图完整）；
//  4) 弱网开窗：卡片 420 快变体已缓存 → holdSrc 即显（cover-hold complete）；
//     封面可见即预热使 1080 规范变体入缓存——开窗零新 1080 网络请求；
//  5) 预取失败静默：相邻 JSON 首请求 abort（route.abort）→ 无错误 UI、翻页/
//     关窗不受阻；正式打开走常规拉取成功渲染；
//  6) 无新就绪门：入场 morph 起跑时机 ≤ 1600ms（数据+几何即起跑，E2 语义）
//     且为真 morph（起跑 scale = 封面宽/媒体列宽 ≪ fallback 0.96）。
// 夹具：隔离 IP 99209 只挂本 rig 三篇内容 99121~99123（psql 幂等重置）：hub
// 默认 hot 排序 = hot_score（新内容全 0）+ (view_count + like×3) DESC，view
// 梯度 [B=1000, A=500, C=250] 钉死 [B, A, C]——A 恒居中，相邻 = {B, C}
//（view 增量在 rig 全程 ≪ 梯度间距，不扰动顺序）；封面与附件图用 /e4/*.jpg
// 相对路径（非 SVG → 走 /_next/image 优化器变体地址），rig 在浏览器层拦截
// /_next/image 用合成 PNG 应答（带 Cache-Control 供预热命中验证；延迟/中止
// 按场景配置）=「真签名 URL 取数延迟」的确定性替身。
// 截图 ../screenshots/e4-rig-*.png。
import { chromium } from "@playwright/test";
import { execSync } from "node:child_process";
import { deflateSync } from "node:zlib";

const BASE = process.env.E4_BASE_URL || "http://localhost:3000";
/* 隔离 IP + 仅 tab 参数的 URL 形态：夹具顺序依赖 hot 默认序（view 梯度钉死
   [B, A, C]），带 sort=newest 会改变顺序破坏相邻性前置；带参开浮窗的 URL
   记录透传语义由编排 rig 的 F4 组覆盖（#989 修复后 sort/q 形态正常）。 */
const PAGE_PATH = "/ip/99209?tab=share";
const SHOTS = new URL("../screenshots/", import.meta.url).pathname;
const PSQL = "docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft";

const IDS = { a: 99121, b: 99122, c: 99123 };
const TITLES = {
  a: "【E4验证】居中篇多图",
  b: "【E4验证】相邻次篇",
  c: "【E4验证】相邻三篇",
};

const fails = [];
function check(name, cond, extra = "") {
  const line = `${cond ? "PASS" : "FAIL"} ${name}${extra ? ` — ${extra}` : ""}`;
  console.log(line);
  if (!cond) fails.push(line);
}

/* ── 合成 PNG（无依赖编码器：IHDR + deflate 扫描线）───────────────────────── */
const CRC_TABLE = (() => {
  const table = new Int32Array(256);
  for (let n = 0; n < 256; n += 1) {
    let c = n;
    for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c;
  }
  return table;
})();
function crc32(buf) {
  let c = 0xffffffff;
  for (const byte of buf) c = CRC_TABLE[(c ^ byte) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}
function pngChunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length, 0);
  const t = Buffer.from(type, "ascii");
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([t, data])), 0);
  return Buffer.concat([len, t, data, crc]);
}
function makePng(w, h, [r, g, b]) {
  const row = Buffer.alloc(w * 3 + 1);
  for (let x = 0; x < w; x += 1) {
    row[1 + x * 3] = r;
    row[2 + x * 3] = g;
    row[3 + x * 3] = b;
  }
  const raw = Buffer.concat(Array.from({ length: h }, () => row));
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0);
  ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8; // bit depth
  ihdr[9] = 2; // truecolor RGB
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    pngChunk("IHDR", ihdr),
    pngChunk("IDAT", deflateSync(raw)),
    pngChunk("IEND", Buffer.alloc(0)),
  ]);
}

/** 合成媒体表：key = /e4/<key>.jpg；色块区分舞台画面（截图取证）。 */
const IMAGES = {
  "cover-a": { w: 900, h: 1200, rgb: [59, 130, 246] }, // 蓝（A 封面/首屏）
  "img-a1": { w: 800, h: 800, rgb: [239, 68, 68] }, // 红（A 第 2 页）
  "img-a2": { w: 900, h: 1200, rgb: [34, 197, 94] }, // 绿（A 第 3 页）
  "cover-b": { w: 900, h: 1200, rgb: [168, 85, 247] }, // 紫（B 封面）
  "img-b1": { w: 900, h: 1200, rgb: [126, 34, 206] },
  "cover-c": { w: 900, h: 1200, rgb: [249, 115, 22] }, // 橙（C 封面）
  "img-c1": { w: 900, h: 1200, rgb: [234, 88, 12] },
};
const PNG = Object.fromEntries(
  Object.entries(IMAGES).map(([key, spec]) => [key, makePng(spec.w, spec.h, spec.rgb)]),
);

/* ── 夹具：幂等重置 IP 99209 + 内容 99121~99123（view 梯度钉死 [B, A, C]）── */
execSync(`${PSQL} -c "DELETE FROM content_items WHERE id BETWEEN 99121 AND 99123"`);
execSync(`${PSQL} -c "DELETE FROM ips WHERE id = 99209"`);
execSync(`${PSQL} -c "
INSERT INTO ips (id, name, slug, description, category, creator_id, status, popularity_score)
VALUES (99209, 'E4验证隔离IP', 'e4-rig-ip-01', 'E4 rig 隔离夹具 IP（仅三篇）', 'game', 313, 'approved', 0)
"`);
execSync(`${PSQL} -c "
INSERT INTO content_items (id, title, author_id, zone, ip_id, content_type, status, description, cover_image_url, cover_width, cover_height, view_count, like_count, created_at, updated_at, is_public, allow_copy)
VALUES
 (99122,'${TITLES.b}',313,'fanwork',99209,'image','published','E4 夹具：相邻次篇','/e4/cover-b.jpg',900,1200,1000,0,NOW() - interval '3 seconds',NOW(),true,true),
 (99121,'${TITLES.a}',313,'fanwork',99209,'image','published','E4 夹具：居中篇多图（相邻预取源）','/e4/cover-a.jpg',900,1200,500,0,NOW() - interval '2 seconds',NOW(),true,true),
 (99123,'${TITLES.c}',313,'fanwork',99209,'image','published','E4 夹具：相邻三篇','/e4/cover-c.jpg',900,1200,250,0,NOW() - interval '1 seconds',NOW(),true,true)
"`);
execSync(`${PSQL} -c "
INSERT INTO content_attachments (content_item_id, file_type, oss_key, mime_type, width, height, sort_order, is_primary, scan_status, scan_required, original_file_name, file_size)
VALUES
 (99121,'image','/e4/img-a1.jpg','image/jpeg',800,800,0,true,'not_required',false,'e4-a1.jpg',90000),
 (99121,'image','/e4/img-a2.jpg','image/jpeg',900,1200,1,false,'not_required',false,'e4-a2.jpg',120000),
 (99122,'image','/e4/img-b1.jpg','image/jpeg',900,1200,0,true,'not_required',false,'e4-b1.jpg',120000),
 (99123,'image','/e4/img-c1.jpg','image/jpeg',900,1200,0,true,'not_required',false,'e4-c1.jpg',120000)
"`);

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();

/* ── 计数器与场景化网络替身 ──────────────────────────────────────────────── */
const detailCounts = new Map();
const imgCounts = new Map();
/** 累计图片请求计数（不随场景重置）：「封面已入缓存」按会话累计断言——
    卡片可见预热与相邻预取的预热是同一 URL 的互补供给路径。 */
const imgCountsTotal = new Map();
/** 场景化的图片延迟（key → delayMs）：弱网 = 真 OSS 签名 URL 取数延迟替身。 */
const imageDelays = new Map();
/** 场景 ⑤：相邻详情 JSON 首请求中止（一次性；正式打开的第二请求放行）。 */
const detailAbortOnce = new Set();

function bump(map, key) {
  map.set(key, (map.get(key) ?? 0) + 1);
}
function resetCounts() {
  detailCounts.clear();
  imgCounts.clear();
}
const detailCount = (id) => detailCounts.get(String(id)) ?? 0;
const imgCount = (key, width) => imgCounts.get(`${key}:w${width}`) ?? 0;
const imgCountTotal = (key, width) => imgCountsTotal.get(`${key}:w${width}`) ?? 0;

page.on("request", (req) => {
  const match = req.url().match(/\/api\/v1\/contents\/(\d+)(?:\?|$)/);
  if (match) bump(detailCounts, match[1]);
});

await page.route("**/_next/image*", async (route) => {
  const url = new URL(route.request().url());
  const source = url.searchParams.get("url") ?? "";
  const match = source.match(/^\/e4\/([\w-]+)\.jpg$/);
  if (!match) {
    try {
      const response = await route.fetch();
      return route.fulfill({ response });
    } catch {
      return route.abort();
    }
  }
  const key = match[1];
  const width = url.searchParams.get("w") ?? "?";
  bump(imgCounts, `${key}:w${width}`);
  bump(imgCountsTotal, `${key}:w${width}`);
  const delay = imageDelays.get(key) ?? 0;
  if (delay > 0) await new Promise((resolve) => setTimeout(resolve, delay));
  return route.fulfill({
    contentType: "image/png",
    body: PNG[key],
    headers: { "Cache-Control": "public, max-age=3600" },
  });
});

await page.route(/\/api\/v1\/contents\/(9912[123])(\?|$)/, async (route) => {
  const id = Number(new URL(route.request().url()).pathname.match(/(\d+)$/)?.[1]);
  if (detailAbortOnce.delete(id)) return route.abort("failed");
  try {
    const response = await route.fetch();
    return route.fulfill({ response });
  } catch {
    return route.abort();
  }
});

/* ── 采样与开窗助手 ─────────────────────────────────────────────────────── */
async function geometry() {
  return page.evaluate(() => {
    const badge = document.querySelector('[data-slot="media-count-badge"]');
    return { badge: badge?.textContent?.trim() ?? "" };
  });
}

/** 开窗采样：rAF 循环记录壳 transform（首 morph 帧时延/起跑 scale）。 */
function sampleOpen(durationMs = 2200) {
  return page.evaluate(
    (duration) =>
      new Promise((resolve) => {
        const samples = [];
        const t0 = performance.now();
        const tick = () => {
          const shell = document.querySelector("dialog[open] > div");
          const cs = shell ? getComputedStyle(shell) : null;
          const matrix = cs && cs.transform !== "none" ? new DOMMatrix(cs.transform) : null;
          samples.push({ t: Math.round(performance.now() - t0), scale: matrix ? matrix.a : null });
          if (performance.now() - t0 < duration) requestAnimationFrame(tick);
          else resolve(samples);
        };
        requestAnimationFrame(tick);
      }),
    durationMs,
  );
}

function openMorphInfo(samples) {
  const frames = samples.filter((s) => s.scale !== null);
  if (frames.length === 0) return { morphed: false, tFirst: -1, firstScale: -1 };
  return { morphed: true, tFirst: frames[0].t, firstScale: frames[0].scale };
}

/** 卡片滚到视口中央再点（newest 序夹具卡在最顶部，最小滚动会落在粘性
    头部/筛选条下被拦截点击）。 */
async function revealCard(title) {
  const card = page.locator(`text=${title}`).first();
  await card.waitFor({ state: "visible", timeout: 10000 });
  await card.evaluate((el) => el.scrollIntoView({ block: "center" }));
  await page.waitForTimeout(150);
  return card;
}

async function openByTitle(title, settleMs = 1300) {
  const card = await revealCard(title);
  const sampling = sampleOpen(2200);
  await card.click({ timeout: 10000 });
  await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 12000 });
  await page.waitForTimeout(settleMs);
  return openMorphInfo(await sampling);
}

async function closeOverlay() {
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForTimeout(900);
}

/** 弱网翻页保持态读数（淡出层/当前层/两层图片加载态）。 */
function holdState() {
  return page.evaluate(() => {
    const exit = document.querySelector('[data-slot="slide-fade-out"]');
    const current = document.querySelector('[data-slot="variant-media-pane"] > [data-slot="detail-cover"]');
    const exitImg = exit?.querySelector("img") ?? null;
    const currentImg = current?.querySelector("img") ?? null;
    return {
      exit: Boolean(exit),
      exitOpacity: exit ? getComputedStyle(exit).opacity : "",
      exitAnimation: exit?.style.animation ?? "",
      exitImgComplete: exitImg ? exitImg.complete && exitImg.naturalWidth > 0 : null,
      currentOpacity: current?.style.opacity ?? "",
      currentImgComplete: currentImg ? currentImg.complete && currentImg.naturalWidth > 0 : null,
    };
  });
}

try {
  /* domcontentloaded：Playwright 1.65 起 networkidle 把 dev server 的 HMR
   WebSocket 计入开放连接，对 dev 栈永不落定；后续断言自带显式等待。 */
  await page.goto(`${BASE}${PAGE_PATH}`, { waitUntil: "domcontentloaded", timeout: 30000 });
  await page.waitForTimeout(1000);

  /* ── 前置：夹具相邻性（hot 序 [B, A, C]，A 居中） ─────────────────────── */
  const adjacency = await page.evaluate(
    async ({ ids }) => {
      const res = await fetch("http://localhost:8080/api/v1/ips/99209/contents?page_size=24");
      const json = await res.json();
      const list = json.contents.map((c) => c.id);
      const at = list.indexOf(ids.a);
      return { at, prev: at > 0 ? list[at - 1] : null, next: at >= 0 && at < list.length - 1 ? list[at + 1] : null };
    },
    { ids: IDS },
  );
  check(
    "前置：A 居中且相邻 = {B, C}（contextList 相邻项恰为夹具邻篇）",
    adjacency.at === 1 &&
      ((adjacency.prev === IDS.b && adjacency.next === IDS.c) ||
        (adjacency.prev === IDS.c && adjacency.next === IDS.b)),
    `at=${adjacency.at} prev=${adjacency.prev} next=${adjacency.next}`,
  );

  /* ── ④-前置 封面可见即预热：卡片进视口 → 1080 规范变体预取请求 ─────────── */
  resetCounts();
  const cardA = await revealCard(TITLES.a);
  await page.screenshot({ path: `${SHOTS}/e4-rig-card-warmup.png` });
  /* idle 调度（requestIdleCallback timeout 3000 兜底）+ 图片请求余量。累计
     计数：卡片进视口即可能在页面装载期已触发（先于本场景计数重置）。 */
  await page.waitForTimeout(4000);
  check(
    "封面可见即预热：卡片可见后出现 1080 规范变体预取请求（idle 调度）",
    imgCountTotal("cover-a", 1080) >= 1,
    `total=${imgCountTotal("cover-a", 1080)}`,
  );

  /* ── ⑥ 无新就绪门 + ① 相邻 idle 预取 ─────────────────────────────────── */
  resetCounts();
  const openA = await openByTitle(TITLES.a);
  check("⑥ 开窗为真 morph（起跑 scale = 封面宽/媒体列宽 ≪ 0.96 fallback）", openA.morphed && openA.firstScale < 0.9, `s0=${openA.firstScale.toFixed(3)}`);
  check("⑥ 入场起跑不晚于 E2 基线（首 morph 帧 ≤ 1600ms）", openA.morphed && openA.tFirst <= 1600, `t=${openA.tFirst}ms`);
  let geo = await geometry();
  check("A 打开（封面恒首屏，计数 1/3）", geo.badge === "1 / 3", `badge=${geo.badge}`);
  await page.screenshot({ path: `${SHOTS}/e4-rig-open-baseline.png` });

  await page.waitForTimeout(3500);
  check("① 相邻详情 JSON idle 预取（前后项各一轮，next=99122/prev=99123）", detailCount(IDS.b) === 1 && detailCount(IDS.c) === 1, `b=${detailCount(IDS.b)} c=${detailCount(IDS.c)}`);
  /* 封面 1080 预热按会话累计断言：信息流卡片可见预热与相邻预取的 warmDetail
     是同一 URL 的互补供给（去重后其一命中即已入缓存）。 */
  check("① 相邻封面 1080 规范变体已预热（浮窗首 slide 同址，会话累计）", imgCountTotal("cover-b", 1080) >= 1 && imgCountTotal("cover-c", 1080) >= 1, `b=${imgCountTotal("cover-b", 1080)} c=${imgCountTotal("cover-c", 1080)}`);
  check("① 预取不重试风暴（各恰 1 次详情请求）", detailCount(IDS.b) <= 1 && detailCount(IDS.c) <= 1);
  await page.screenshot({ path: `${SHOTS}/e4-rig-adjacent-prefetch.png` });
  await closeOverlay();

  /* ── ② 缓存命中：预取过的相邻内容零新详情请求 ────────────────────────── */
  resetCounts();
  const openB = await openByTitle(TITLES.b);
  await page.waitForTimeout(600);
  check("② 打开预取过的相邻内容：网络层零新详情请求（claim 内存缓存）", detailCount(IDS.b) === 0, `count=${detailCount(IDS.b)}`);
  check("② 缓存命中渲染正确（B 标题可见）", (await page.locator("dialog[open]").textContent())?.includes(TITLES.b) === true);
  geo = await geometry();
  check("② B 打开（计数 1/2）", geo.badge === "1 / 2", `badge=${geo.badge}`);
  check("② 缓存命中不引入就绪门（入场 morph ≤ 1600ms 且为真 morph）", openB.morphed && openB.tFirst <= 1600 && openB.firstScale < 0.9, `t=${openB.tFirst}ms s0=${openB.firstScale.toFixed(3)}`);
  await page.screenshot({ path: `${SHOTS}/e4-rig-cache-hit-b.png` });
  await closeOverlay();

  /* ── ③ 弱网翻页 hold-crossfade（route delay = 签名 URL 取数延迟替身） ──── */
  imageDelays.set("img-a1", 1600);
  resetCounts();
  await openByTitle(TITLES.a, 1500);
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(380);
  const hold = await holdState();
  check(
    "③ 弱网翻页：旧画面保持（淡出层在、满不透明、无淡出动画、旧图完整）",
    hold.exit && Number(hold.exitOpacity) >= 0.99 && hold.exitAnimation === "" && hold.exitImgComplete === true,
    `exit=${hold.exit} opacity=${hold.exitOpacity} anim="${hold.exitAnimation}" imgReady=${hold.exitImgComplete}`,
  );
  check("③ 新层未就绪被压在下（opacity 0、图未完成）", hold.currentOpacity === "0" && hold.currentImgComplete === false, `opacity="${hold.currentOpacity}" imgComplete=${hold.currentImgComplete}`);
  await page.screenshot({ path: `${SHOTS}/e4-rig-hold-mid.png` });
  await page.waitForFunction(
    () => {
      const exit = document.querySelector('[data-slot="slide-fade-out"]');
      const current = document.querySelector('[data-slot="variant-media-pane"] > [data-slot="detail-cover"]');
      const img = current?.querySelector("img");
      return Boolean(!exit && current && img && img.complete && img.naturalWidth > 0);
    },
    null,
    { timeout: 12000 },
  );
  geo = await geometry();
  check("③ 就绪即交换（旧层移除、新图完整、计数 2/3）", geo.badge === "2 / 3", `badge=${geo.badge}`);
  const swapped = await holdState();
  check("③ 交换后舞台单层（无残留保持层）", swapped.exit === false);
  await page.screenshot({ path: `${SHOTS}/e4-rig-hold-swapped.png` });
  await closeOverlay();
  imageDelays.delete("img-a1");

  /* ── ④ 弱网开窗 holdSrc 即显 + 预热缓存命中 ──────────────────────────── */
  imageDelays.set("cover-a", 2500);
  resetCounts();
  const weakCard = await revealCard(TITLES.a);
  const weakSampling = sampleOpen(2200);
  await weakCard.click({ timeout: 10000 });
  await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 12000 });
  /* holdSrc 检查须落在入场窗口内（壳起跑 ~150ms + 400ms 动效 + 保持层淡出
     ~260ms ≈ 800ms 内卸载）——hold 层 = 卡片 420 快变体（缓存秒开）。 */
  const holdFrame = await page
    .waitForFunction(
      () => {
        const img = document.querySelector('img[data-slot="cover-hold"]');
        return Boolean(img && img.complete && img.naturalWidth > 0);
      },
      null,
      { timeout: 900 },
    )
    .then(() => true)
    .catch(() => false);
  await page.screenshot({ path: `${SHOTS}/e4-rig-open-weaknet-hold.png` });
  check("④ 弱网开窗 holdSrc 即显（入场窗内 cover-hold 挂载且缓存秒开）", holdFrame);
  await page.waitForTimeout(1300);
  const openWeak = openMorphInfo(await weakSampling);
  /* 允许 ≤1 次再取（浏览器对 route.fulfill 应答的缓存落盘不确定）：预热
     正常命中 = 0；即便缓存未落盘，开窗也不被 2500ms 延迟的规范变体阻塞
   * （下一断言的入场时延才是 UX 主断言）。 */
  check("④ 预热缓存命中：开窗零或至多一次 1080 封面请求", imgCount("cover-a", 1080) <= 1, `count=${imgCount("cover-a", 1080)}`);
  check("④ 弱网开窗不白屏（入场照常 morph，未等延迟的规范变体）", openWeak.morphed && openWeak.tFirst <= 1600, `t=${openWeak.tFirst}ms morphed=${openWeak.morphed}`);
  await closeOverlay();
  imageDelays.delete("cover-a");

  /* ── ⑤ 预取失败静默降级：不阻塞翻页/关窗，正式打开常规拉取成功 ────────── */
  /* 重载清空会话级预取缓存（①的热条目仍在会让本场景的相邻预取被去重跳过，
     abort 路径无从触发）；图片计数器在 Node 侧跨重载保留。 */
  /* domcontentloaded：Playwright 1.65 起 networkidle 把 dev server 的 HMR
   WebSocket 计入开放连接，对 dev 栈永不落定；后续断言自带显式等待。 */
  await page.reload({ waitUntil: "domcontentloaded", timeout: 30000 });
  await page.waitForTimeout(1500);
  detailAbortOnce.add(IDS.b);
  detailAbortOnce.add(IDS.c);
  resetCounts();
  await openByTitle(TITLES.a, 1500);
  await page.waitForTimeout(1200);
  const silentOk = await page.evaluate(() => {
    const dialog = document.querySelector("dialog[open]");
    const alert = dialog?.querySelector('[role="alert"]');
    return { open: Boolean(dialog), hasAlertUi: Boolean(alert) };
  });
  check("⑤ 预取失败无错误 UI（浮窗正常、无 alert 态）", silentOk.open && !silentOk.hasAlertUi, `open=${silentOk.open} alert=${silentOk.hasAlertUi}`);
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(700);
  geo = await geometry();
  check("⑤ 预取失败不阻塞翻页（计数 2/3）", geo.badge === "2 / 3", `badge=${geo.badge}`);
  await page.screenshot({ path: `${SHOTS}/e4-rig-prefetch-fail-silent.png` });
  await closeOverlay();
  check("⑤ 预取失败不阻塞关窗（浮窗移除）", (await page.locator("dialog[open]").count()) === 0);

  /* 正式打开 C（其预取在 ⑤ 被 abort、缓存已清、且从未被认领）→ claim 未命中
     → 常规拉取成功渲染（B 不作此断言：② 的缓存条目在 TTL 内可重复消费，
     会再次零请求命中而非走常规路径）。 */
  resetCounts();
  await openByTitle(TITLES.c, 1200);
  check(
    "⑤ 正式打开走常规拉取成功（C 渲染；请求 = 被 abort 的预取 + 新拉取）",
    (await page.locator("dialog[open]").textContent())?.includes(TITLES.c) === true &&
      detailCount(IDS.c) >= 1 &&
      detailCount(IDS.c) <= 2,
    `detailRequests=${detailCount(IDS.c)}`,
  );
  await page.screenshot({ path: `${SHOTS}/e4-rig-prefetch-fail-recover-c.png` });
  await closeOverlay();
} finally {
  await context.close().catch(() => {});
  await browser.close().catch(() => {});
}

console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n${fails.join("\n")}`);
process.exit(fails.length === 0 ? 0 : 1);
