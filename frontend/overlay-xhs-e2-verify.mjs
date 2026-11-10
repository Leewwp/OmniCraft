// E2 #954 开合动画浏览器验证 rig（本地栈 3000/8080，@playwright/test Chromium）：
// 整容器封面 morph 开窗 + 关窗溶解回源编排 + 就绪门退场 + 切换交叉淡出——断言：
//  1) 开窗：起跑帧 scale = 卡片封面宽 ÷ 媒体列宽（等比）、中点在收缩区间、
//     400ms 后壳 transform 复位无残留；
//  2) 关窗：morphBack 存在（非复用开窗 FLIP 的二次闪现形态）、200ms 中点
//     scale ≈ 1−ease(0.5)·(1−s_end)、≈320ms 壳 opacity<0.08（落地前整壳透明、
//     无末闪）、全程 scale 单调收缩（无先缩又放大）、origin 锚舞台左上角；
//  3) 关窗开始即解锁滚动锁（morphBack 起跑帧 body/html overflow 已解除）；
//  4) 快速连按翻页：任意时刻舞台层数 ≤2（1 新 + 1 淡出中），动画结束层数 =1；
//  5) 翻离首屏（翻到文档页 = reading 档）后关窗：舞台瞬时回源比例
//     （pane 宽 = 壳高 × 0.75 封面比，无 240ms 档位过渡中间值）再收缩。
// 夹具：沿用 e1 rig 造数（99101 混排三比例 / 99104 图+csv 混合，幂等重置）；
// 文档附件 oss_url 经详情响应补丁指向本 rig 的 127.0.0.1:18084 CSV 服务。
// 截图 ../screenshots/e2-rig-*.png。
import { chromium } from "@playwright/test";
import { execSync } from "node:child_process";
import { createServer } from "node:http";

const BASE = process.env.E2_BASE_URL || "http://localhost:3000";
const PAGE_PATH = "/ip/209?tab=share";
const SHOTS = new URL("../screenshots/", import.meta.url).pathname;
const PSQL = "docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft";

const TITLES = {
  mixedRatios: "【E1验证】混排三比例图集",
  imagePlusDoc: "【E1验证】影像加文档混合",
};

const fails = [];
function check(name, cond, extra = "") {
  const line = `${cond ? "PASS" : "FAIL"} ${name}${extra ? ` — ${extra}` : ""}`;
  console.log(line);
  if (!cond) fails.push(line);
}
const near = (value, expected, tol) => Math.abs(value - expected) <= tol;

/* ── 夹具：幂等重置 99101 / 99104（e1 rig 同款） ─────────────────────────── */
execSync(`${PSQL} -c "DELETE FROM content_items WHERE id IN (99101, 99104)"`);
execSync(`${PSQL} -c "
INSERT INTO content_items (id, title, author_id, zone, ip_id, content_type, status, description, created_at, updated_at, is_public, allow_copy)
VALUES
 (99101,'${TITLES.mixedRatios}',313,'fanwork',209,'image','published','E2 动效夹具：3:4 + 1:1 + 9:16 混排',NOW(),NOW(),true,true),
 (99104,'${TITLES.imagePlusDoc}',313,'fanwork',209,'image','published','E2 动效夹具：3:4 图 + csv 文档（关窗回源）',NOW() - interval '3 seconds',NOW(),true,true)
"`);
execSync(`${PSQL} -c "
INSERT INTO content_attachments (content_item_id, file_type, oss_key, mime_type, width, height, sort_order, is_primary, scan_status, scan_required, original_file_name, file_size)
VALUES
 (99101,'image','/seed-media/real/gallery/f1-portrait-1.svg','image/svg+xml',900,1200,0,true,'not_required',false,'e2-portrait.svg',1000),
 (99101,'image','/seed-media/real/gallery/f1-square.svg','image/svg+xml',800,800,1,false,'not_required',false,'e2-square.svg',1000),
 (99101,'image','/seed-media/real/gallery/f1-portrait-2.svg','image/svg+xml',720,1280,2,false,'not_required',false,'e2-tall.svg',1000),
 (99104,'image','/seed-media/real/gallery/f1-portrait-1.svg','image/svg+xml',900,1200,0,true,'not_required',false,'e2-portrait.svg',1000),
 (99104,'document','uploads/313/document/e2-mixed-sheet.csv','text/csv',NULL,NULL,1,false,'clean',false,'e2-mixed-sheet.csv',120)
"`);

/* ── 本地媒体服务：文档族 slide 的合成 CSV（本地无 OSS 签名的最小替身） ──── */
const CSV_BODY = "属性,数值,备注\n关窗回源,E2,瞬时回封面比例\nmorphBack,400ms,ease\n";
const mediaServer = createServer((req, res) => {
  res.writeHead(200, { "Content-Type": "text/csv; charset=utf-8", "Access-Control-Allow-Origin": "*" });
  res.end(CSV_BODY);
});
await new Promise((resolve) => mediaServer.listen(18084, "127.0.0.1", resolve));
const DOC_URL = "http://127.0.0.1:18084/e2-sheet.csv";

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();

/* 详情响应补丁：仅给文档族附件补 oss_url（其余字段原样透传）。 */
await page.route(/\/api\/v1\/contents\/(99104)(\?|$)/, async (route) => {
  const response = await route.fetch();
  let json;
  try {
    json = await response.json();
  } catch {
    return route.fulfill({ response });
  }
  for (const attachment of json.attachments ?? []) {
    if (attachment.file_type === "document") attachment.oss_url = DOC_URL;
  }
  return route.fulfill({ response, json });
});

/* ── 采样器：rAF 循环记录壳 transform/opacity、舞台几何、层数、滚动锁 ────── */
async function sample(durationMs) {
  return page.evaluate(async (duration) => {
    const samples = [];
    const t0 = performance.now();
    while (performance.now() - t0 < duration) {
      const shell = document.querySelector("dialog[open] > div");
      const pane = document.querySelector('[data-slot="variant-media-pane"]');
      const cs = shell ? getComputedStyle(shell) : null;
      const matrix = cs && cs.transform !== "none" ? new DOMMatrix(cs.transform) : null;
      /* 布局宽高走 computed（getBoundingClientRect 含壳 morph transform，会把
         收缩运动混进几何读数）。 */
      const paneCs = pane ? getComputedStyle(pane) : null;
      samples.push({
        t: Math.round(performance.now() - t0),
        scale: matrix ? matrix.a : 1,
        transformNone: !matrix,
        opacity: cs ? parseFloat(cs.opacity) : null,
        origin: cs ? cs.transformOrigin : "",
        paneW: paneCs ? parseFloat(paneCs.width) : null,
        paneH: paneCs ? parseFloat(paneCs.height) : null,
        tier: pane?.dataset.tier ?? "",
        layers: document.querySelectorAll(
          '[data-slot="variant-media-pane"] > [data-slot="detail-cover"], [data-slot="variant-media-pane"] > [data-slot="slide-fade-out"]',
        ).length,
        bodyLock: document.body.style.overflow,
        htmlLock: document.documentElement.style.overflow,
      });
      await new Promise((resolve) => requestAnimationFrame(resolve));
    }
    return samples;
  }, durationMs);
}

function nearest(samples, t) {
  return samples.reduce((best, s) => (Math.abs(s.t - t) < Math.abs(best.t - t) ? s : best), samples[0]);
}

/** origin 锚舞台左上角：壳有 1px 边框（桌面 min-[960px]:border），舞台相对
    壳的左上偏移 = 边框宽（0~2px 亚像素），非硬编码 0 0。 */
function originAtStageCorner(origin) {
  const m = /^([\d.]+)px ([\d.]+)px$/.exec(origin ?? "");
  if (!m) return false;
  return parseFloat(m[1]) <= 2 && parseFloat(m[2]) <= 2;
}

/** 序列单调（开=不降 / 关=不增），容差内允许亚像素抖动。 */
function monotonic(values, direction, tol) {
  return values.every((v, i) => {
    if (i === 0) return true;
    const delta = v - values[i - 1];
    return direction === "up" ? delta >= -tol : delta <= tol;
  });
}

async function openByTitle(title) {
  const card = page.locator(`text=${title}`).first();
  await card.scrollIntoViewIfNeeded({ timeout: 10000 });
  const coverBox = await card
    .locator("xpath=ancestor-or-self::article[1]")
    .locator('[data-slot="card-cover"]')
    .boundingBox();
  await card.click({ timeout: 10000 });
  await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 12000 });
  /* 入场 morph（400ms）+ 首帧保持释放余量。 */
  await page.waitForTimeout(1100);
  return coverBox;
}

async function currentGeometry() {
  return page.evaluate(() => {
    const pane = document.querySelector('[data-slot="variant-media-pane"]');
    const badge = document.querySelector('[data-slot="media-count-badge"]');
    const cs = pane ? getComputedStyle(pane) : null;
    return { paneW: cs ? parseFloat(cs.width) : 0, paneH: cs ? parseFloat(cs.height) : 0, tier: pane?.dataset.tier ?? "", badge: badge?.textContent?.trim() ?? "" };
  });
}

try {
  await page.goto(`${BASE}${PAGE_PATH}`, { waitUntil: "networkidle", timeout: 30000 });
  await page.waitForTimeout(800);

  /* ── 1. 开窗 morph：等比起跑、中点收缩区间、400ms 复位 ─────────────────── */
  const coverBox = await openByTitle(TITLES.mixedRatios);
  const openGeo = await currentGeometry();
  const s0Expected = coverBox.width / openGeo.paneW;
  check("开窗后滚动锁在位（unlock 断言的前置）", openGeo.paneW > 0 && (await page.evaluate(() => document.body.style.overflow)) === "hidden");
  await page.screenshot({ path: `${SHOTS}/e2-rig-open-settled.png` });
  /* 关掉重开：采样窗只含第二次开窗的 morph（不含上一次关窗的 morphBack）。 */
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForTimeout(900);
  const card = page.locator(`text=${TITLES.mixedRatios}`).first();
  await card.scrollIntoViewIfNeeded();
  const openSampling = sample(1500);
  await card.click();
  await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 12000 });
  /* 开窗中点截图：等 morph invert 帧出现后再拍（固定延时抓不准动画窗口）。 */
  await page
    .waitForFunction(() => {
      const shell = document.querySelector("dialog[open] > div");
      return Boolean(shell && getComputedStyle(shell).transform !== "none");
    }, null, { timeout: 4000 })
    .catch(() => {});
  await page.waitForTimeout(140);
  await page.screenshot({ path: `${SHOTS}/e2-rig-open-morph-mid.png` });
  const openSamples = await openSampling;

  const morphFrames = openSamples.filter((s) => !s.transformNone);
  check("开窗存在整容器 morph 帧（transform 非 identity）", morphFrames.length >= 3, `frames=${morphFrames.length}`);
  if (morphFrames.length) {
    const startScale = morphFrames[0].scale;
    check("开窗起跑帧 scale = 封面宽 ÷ 媒体列宽（等比无压扁）", near(startScale, s0Expected, 0.08), `s0=${startScale.toFixed(3)} 期望=${s0Expected.toFixed(3)}`);
    const t0 = morphFrames[0].t;
    const mid = nearest(openSamples, t0 + 200);
    check("开窗 200ms 中点 scale 在收缩区间（介于起跑与 1 之间）", mid.scale > startScale + 0.02 && mid.scale < 0.99, `mid=${mid.scale.toFixed(3)}`);
    check("开窗全程 scale 单调放大（无回缩抖动）", monotonic(morphFrames.map((s) => s.scale), "up", 0.03));
    const tail = openSamples.slice(-8);
    check("开窗 400ms 后壳 transform 复位（无残留）", tail.every((s) => s.transformNone));
  }

  /* ── 2. 快速连按翻页：双层交叉淡出 + 单属主可中断 ──────────────────────── */
  await page.waitForTimeout(300);
  const switchSampling = sample(1600);
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(70);
  await page.screenshot({ path: `${SHOTS}/e2-rig-crossfade-mid.png` });
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(70);
  await page.keyboard.press("ArrowLeft");
  const switchSamples = await switchSampling;
  const maxLayers = Math.max(...switchSamples.map((s) => s.layers));
  check("快速连按任意时刻舞台层数 ≤ 2（1 新 + 1 淡出中）", maxLayers <= 2, `max=${maxLayers}`);
  check("连按期间确有双层交叉淡出帧", switchSamples.some((s) => s.layers === 2));
  check("动画结束舞台层数 = 1（无叠层残留/重影）", switchSamples[switchSamples.length - 1].layers === 1);
  check("连按后计数落位 2 / 3", (await currentGeometry()).badge === "2 / 3");

  /* ── 3. 关窗 morphBack：中点 scale、落地前透明度、单调收缩、origin、即解锁 ── */
  const closeGeo = await currentGeometry();
  const sEnd = coverBox.width / closeGeo.paneW;
  const closeSampling = sample(1000);
  await page.keyboard.press("Escape");
  await page.waitForTimeout(180);
  await page.screenshot({ path: `${SHOTS}/e2-rig-close-morph-mid.png` });
  const closeSamples = await closeSampling;
  const back = closeSamples.filter((s) => !s.transformNone);
  check("关窗为独立 morphBack（存在反向收缩帧）", back.length >= 3, `frames=${back.length}`);
  if (back.length) {
    const t0 = back[0].t;
    check("morphBack origin 锚媒体舞台左上角（贴壳左缘，仅边框偏移）", originAtStageCorner(back[0].origin), `origin=${back[0].origin}`);
    check("关窗开始即解锁滚动锁（morphBack 起跑帧 body/html 已复位）", back[0].bodyLock === "" && back[0].htmlLock === "", `body=${back[0].bodyLock} html=${back[0].htmlLock}`);
    const mid = nearest(closeSamples, t0 + 200);
    const expectedMid = 1 - 0.8 * (1 - sEnd); /* CSS ease 在 0.5 进度 ≈ 0.80 */
    check("关窗 200ms 中点 scale ≈ 1 − ease(0.5)·(1 − s_end)", near(mid.scale, expectedMid, 0.14), `mid=${mid.scale.toFixed(3)} 期望≈${expectedMid.toFixed(3)} s_end=${sEnd.toFixed(3)}`);
    check("关窗中点已显著收缩（过半程且未落地）", mid.scale < 1 - 0.5 * (1 - sEnd) && mid.scale > sEnd - 0.02, `mid=${mid.scale.toFixed(3)}`);
    const pre = nearest(closeSamples, t0 + 320);
    check("关窗 ≈320ms 落地前整壳透明度 < 0.08（无末闪）", pre.opacity !== null && pre.opacity < 0.08, `opacity=${pre.opacity}`);
    check("关窗全程 scale 单调收缩（无先缩又放大的二次闪现）", monotonic(back.map((s) => s.scale), "down", 0.03));
    const faded = closeSamples.filter((s) => s.opacity !== null && s.opacity < 0.1);
    check("整壳溶解后不再回升（隐藏帧零可见变化）", faded.every((s) => s.opacity < 0.12), `fadedFrames=${faded.length}`);
  }
  await page.waitForTimeout(600);
  check("关窗后浮窗完全移除", (await page.locator("dialog[open]").count()) === 0);

  /* ── 4. 翻离首屏（文档 reading 档）后关窗：舞台瞬时回源比例再收缩 ──────── */
  await openByTitle(TITLES.imagePlusDoc);
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(600);
  const docGeo = await currentGeometry();
  check("前置：翻到文档页 = reading 档宽舞台", docGeo.tier === "reading" && docGeo.paneW > 800, `tier=${docGeo.tier} pane=${docGeo.paneW.toFixed(0)}`);
  await page.screenshot({ path: `${SHOTS}/e2-rig-reading-stage.png` });

  const resetSampling = sample(1000);
  await page.keyboard.press("Escape");
  await page.waitForTimeout(150);
  await page.screenshot({ path: `${SHOTS}/e2-rig-reading-close-source.png` });
  const resetSamples = await resetSampling;
  const resetBack = resetSamples.filter((s) => !s.transformNone);
  check("回源关窗同样走 morphBack", resetBack.length >= 3, `frames=${resetBack.length}`);
  if (resetBack.length) {
    const start = resetBack[0];
    const sourcePaneW = Math.round(start.paneH * 0.75);
    check("关窗起跑舞台已瞬时回源比例（pane 宽 = 壳高 × 0.75 封面比）", near(start.paneW, sourcePaneW, 4), `pane=${start.paneW.toFixed(1)} 期望≈${sourcePaneW} tier=${start.tier}`);
    check("回源为瞬时跳变（收缩期内无 240ms 档位过渡中间值）", resetBack.every((s) => s.paneW < 700 || s.paneW > 860), `pane序列=${resetBack.map((s) => s.paneW.toFixed(0)).slice(0, 8).join(",")}`);
    check("回源关窗 origin 仍锚舞台左上角", originAtStageCorner(start.origin), `origin=${start.origin}`);
  }
  await page.waitForTimeout(600);
  check("回源关窗后浮窗完全移除", (await page.locator("dialog[open]").count()) === 0);
} finally {
  await context.close().catch(() => {});
  await browser.close().catch(() => {});
  mediaServer.close();
}

console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n${fails.join("\n")}`);
process.exit(fails.length === 0 ? 0 : 1);
