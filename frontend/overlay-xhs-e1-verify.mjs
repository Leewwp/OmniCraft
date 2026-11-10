// E1 #953 几何引擎浏览器验证 rig（本地栈 3000/8080，@playwright/test Chromium）：
// 内容推导壳宽 + 按内容冻结 + 阅读宽舞台档——四组断言：
//  1) 混排比例图集（3:4/1:1/9:16）内容内翻页：媒体列/壳宽读数零变化（|Δ|<1px）；
//  2) 跨比例内容开窗宽度不同（3:4 ≈ 壳高×0.75 vs 16:9 clamp 满可用宽）；
//  3) 文档族内容开窗即宽舞台档（pane data-tier=reading，媒体列 = 壳宽 − 固定右列 440）；
//  4) 影像+文档混合：翻到文档页一次 240ms 档位切换（连续采样有中间值、档位只翻
//     一次、读数单调收敛），翻回影像档同理，无逐张漂移。
// 夹具：本地库 IP 209 share tab 直插内容 99101~99104（psql 幂等重置）。文档族
// 附件本地无 OSS 签名（oss_url 为空）——rig 在浏览器层拦截详情响应补 oss_url
// 指向本 rig 的 127.0.0.1:18083 CSV 服务（与 d5 mock spec 同思路，最小补丁面）。
// 截图 ../screenshots/e1-rig-*.png。
import { chromium } from "@playwright/test";
import { execSync } from "node:child_process";
import { createServer } from "node:http";

const BASE = process.env.E1_BASE_URL || "http://localhost:3000";
const PAGE_PATH = "/ip/209?tab=share";
const SHOTS = new URL("../screenshots/", import.meta.url).pathname;
const PSQL = "docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft";

/* 固定右列/上限常量（lib/overlay-geometry 同源；rig 侧独立复述用于交叉校验）。 */
const RIGHT_COL = 440;
const SHELL_MAX = 1320;

const TITLES = {
  mixedRatios: "【E1验证】混排三比例图集",
  landscape: "【E1验证】横图单张16比9",
  docOnly: "【E1验证】文档宽舞台",
  imagePlusDoc: "【E1验证】影像加文档混合",
};

const fails = [];
function check(name, cond, extra = "") {
  const line = `${cond ? "PASS" : "FAIL"} ${name}${extra ? ` — ${extra}` : ""}`;
  console.log(line);
  if (!cond) fails.push(line);
}
const near = (value, expected, tol) => Math.abs(value - expected) <= tol;

/* ── 夹具：幂等重置 99101~99104 ─────────────────────────────────────────── */
execSync(`${PSQL} -c "DELETE FROM content_items WHERE id BETWEEN 99101 AND 99104"`);
execSync(`${PSQL} -c "
INSERT INTO content_items (id, title, author_id, zone, ip_id, content_type, status, description, created_at, updated_at, is_public, allow_copy)
VALUES
 (99101,'${TITLES.mixedRatios}',313,'fanwork',209,'image','published','E1 几何夹具：3:4 + 1:1 + 9:16 混排',NOW(),NOW(),true,true),
 (99102,'${TITLES.landscape}',313,'fanwork',209,'image','published','E1 几何夹具：16:9 横图单张',NOW() - interval '1 seconds',NOW(),true,true),
 (99103,'${TITLES.docOnly}',313,'fanwork',209,'template','published','E1 几何夹具：csv 文档单附件（宽舞台）',NOW() - interval '2 seconds',NOW(),true,true),
 (99104,'${TITLES.imagePlusDoc}',313,'fanwork',209,'image','published','E1 几何夹具：3:4 图 + csv 文档（档位切换）',NOW() - interval '3 seconds',NOW(),true,true)
"`);
execSync(`${PSQL} -c "
INSERT INTO content_attachments (content_item_id, file_type, oss_key, mime_type, width, height, sort_order, is_primary, scan_status, scan_required, original_file_name, file_size)
VALUES
 (99101,'image','/seed-media/real/gallery/f1-portrait-1.svg','image/svg+xml',900,1200,0,true,'not_required',false,'e1-portrait.svg',1000),
 (99101,'image','/seed-media/real/gallery/f1-square.svg','image/svg+xml',800,800,1,false,'not_required',false,'e1-square.svg',1000),
 (99101,'image','/seed-media/real/gallery/f1-portrait-2.svg','image/svg+xml',720,1280,2,false,'not_required',false,'e1-tall.svg',1000),
 (99102,'image','/seed-media/real/gallery/f1-land-1.svg','image/svg+xml',1920,1080,0,true,'not_required',false,'e1-land.svg',1000),
 (99103,'document','uploads/313/document/e1-sheet.csv','text/csv',NULL,NULL,0,true,'clean',false,'e1-sheet.csv',120),
 (99104,'image','/seed-media/real/gallery/f1-portrait-1.svg','image/svg+xml',900,1200,0,true,'not_required',false,'e1-portrait.svg',1000),
 (99104,'document','uploads/313/document/e1-mixed-sheet.csv','text/csv',NULL,NULL,1,false,'clean',false,'e1-mixed-sheet.csv',120)
"`);

/* ── 本地媒体服务：文档族 slide 的合成 CSV（本地无 OSS 签名的最小替身） ──── */
const CSV_BODY = "属性,数值,备注\n几何引擎,E1,内容推导壳宽\n固定右列,440px,小红书实测\n宽舞台,取满可用宽,文档族档位\n";
const mediaServer = createServer((req, res) => {
  res.writeHead(200, { "Content-Type": "text/csv; charset=utf-8", "Access-Control-Allow-Origin": "*" });
  res.end(CSV_BODY);
});
await new Promise((resolve) => mediaServer.listen(18083, "127.0.0.1", resolve));
const DOC_URL = "http://127.0.0.1:18083/e1-sheet.csv";

const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();

/* 详情响应补丁：仅给文档族附件补 oss_url（其余字段原样透传）。 */
await page.route(/\/api\/v1\/contents\/(99103|99104)(\?|$)/, async (route) => {
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

async function geometry() {
  return page.evaluate(() => {
    const pane = document.querySelector('[data-slot="variant-media-pane"]');
    const dialog = document.querySelector("dialog.content-detail-overlay");
    const badge = document.querySelector('[data-slot="media-count-badge"]');
    const paneRect = pane?.getBoundingClientRect();
    const dialogRect = dialog?.getBoundingClientRect();
    return {
      paneW: paneRect?.width ?? 0,
      paneH: paneRect?.height ?? 0,
      shellW: dialogRect?.width ?? 0,
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
  /* 入场转场（~400ms）+ 首帧保持释放余量。 */
  await page.waitForTimeout(1100);
}

async function closeOverlay() {
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForTimeout(800);
}

/** 档位切换期连续采样（rAF-30ms 混合节流）：宽度序列 + 档位翻转点。 */
async function sampleGeometry(durationMs) {
  return page.evaluate(async (duration) => {
    const pane = document.querySelector('[data-slot="variant-media-pane"]');
    const dialog = document.querySelector("dialog.content-detail-overlay");
    const samples = [];
    const t0 = performance.now();
    while (performance.now() - t0 < duration) {
      samples.push({
        t: Math.round(performance.now() - t0),
        w: pane.getBoundingClientRect().width,
        shell: dialog.getBoundingClientRect().width,
        tier: pane.dataset.tier ?? "",
      });
      await new Promise((resolve) => setTimeout(resolve, 30));
    }
    return samples;
  }, durationMs);
}

try {
  await page.goto(`${BASE}${PAGE_PATH}`, { waitUntil: "networkidle", timeout: 30000 });
  await page.waitForTimeout(800);

  /* ── 1. 混排比例内容：内容内翻页零几何变化 ─────────────────────────── */
  await openByTitle(TITLES.mixedRatios);
  const page1 = await geometry();
  check("混排开窗 = visual 档（首图 3:4）", page1.tier === "visual", `tier=${page1.tier}`);
  check("混排首帧媒体列 = 壳高 × 0.75", near(page1.paneW, Math.round(page1.paneH * 0.75), 2), `pane=${page1.paneW.toFixed(1)} paneH=${page1.paneH.toFixed(1)} 期望=${Math.round(page1.paneH * 0.75)}`);
  check("混排壳宽 = 媒体列 + 固定右列 440", near(page1.shellW, page1.paneW + RIGHT_COL, 2), `shell=${page1.shellW.toFixed(1)} pane+440=${(page1.paneW + RIGHT_COL).toFixed(1)}`);
  await page.screenshot({ path: `${SHOTS}/e1-rig-mixed-p1.png` });

  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(450);
  const page2 = await geometry();
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(450);
  const page3 = await geometry();
  check("混排翻页 3:4→1:1 媒体列零变化", Math.abs(page2.paneW - page1.paneW) < 1, `Δ=${(page2.paneW - page1.paneW).toFixed(2)}px badge=${page2.badge}`);
  check("混排翻页 1:1→9:16 媒体列零变化", Math.abs(page3.paneW - page2.paneW) < 1, `Δ=${(page3.paneW - page2.paneW).toFixed(2)}px badge=${page3.badge}`);
  check("混排翻页全程壳宽零变化", Math.abs(page3.shellW - page1.shellW) < 1, `Δ=${(page3.shellW - page1.shellW).toFixed(2)}px`);
  check("混排翻页计数 1/3→3/3（翻页真实发生）", page1.badge === "1 / 3" && page3.badge === "3 / 3", `${page1.badge} → ${page3.badge}`);
  await page.screenshot({ path: `${SHOTS}/e1-rig-mixed-p3.png` });
  await closeOverlay();

  /* ── 2. 跨比例内容开窗宽度不同 ─────────────────────────────────────── */
  await openByTitle(TITLES.landscape);
  const land = await geometry();
  check("16:9 内容媒体列 clamp 满可用宽（1320−440=880）", near(land.paneW, SHELL_MAX - RIGHT_COL, 2), `pane=${land.paneW.toFixed(1)}`);
  check("16:9 内容壳宽打满 1320", near(land.shellW, SHELL_MAX, 2), `shell=${land.shellW.toFixed(1)}`);
  check("跨比例内容开窗宽度不同（3:4 vs 16:9）", land.paneW - page1.paneW > 150, `3:4=${page1.paneW.toFixed(1)} 16:9=${land.paneW.toFixed(1)}`);
  check("16:9 壳宽 = 媒体列 + 固定右列 440", near(land.shellW, land.paneW + RIGHT_COL, 2), `shell=${land.shellW.toFixed(1)}`);
  await page.screenshot({ path: `${SHOTS}/e1-rig-landscape.png` });
  await closeOverlay();

  /* ── 3. 文档族内容开窗即宽舞台档 ───────────────────────────────────── */
  await openByTitle(TITLES.docOnly);
  const doc = await geometry();
  check("文档族开窗即 reading 档", doc.tier === "reading", `tier=${doc.tier}`);
  check("文档族开窗即宽媒体列（= 可用宽 880）", near(doc.paneW, SHELL_MAX - RIGHT_COL, 2), `pane=${doc.paneW.toFixed(1)}`);
  check("宽舞台媒体列 > 影像档典型宽（880 > 3:4 ≈ 620）", doc.paneW - page1.paneW > 150, `reading=${doc.paneW.toFixed(1)} visual3:4=${page1.paneW.toFixed(1)}`);
  check("文档族壳宽 = 媒体列 + 固定右列 440", near(doc.shellW, doc.paneW + RIGHT_COL, 2), `shell=${doc.shellW.toFixed(1)}`);
  await page.screenshot({ path: `${SHOTS}/e1-rig-reading-stage.png` });
  await closeOverlay();

  /* ── 4. 影像+文档混合：一次 240ms 档位切换，翻回同理 ────────────────── */
  await openByTitle(TITLES.imagePlusDoc);
  const mixed1 = await geometry();
  check("混合内容开窗 = visual 档（首 slide 3:4 图）", mixed1.tier === "visual", `tier=${mixed1.tier}`);
  check("混合内容开窗媒体列 = 壳高 × 0.75", near(mixed1.paneW, Math.round(mixed1.paneH * 0.75), 2), `pane=${mixed1.paneW.toFixed(1)}`);
  await page.screenshot({ path: `${SHOTS}/e1-rig-mixed-doc-visual.png` });

  /* 翻到文档页：立刻起采样，覆盖整段 240ms 档位切换。 */
  const sampling = sampleGeometry(750);
  await page.keyboard.press("ArrowRight");
  const samples = await sampling;
  const mixed2 = await geometry();
  check("翻到文档页切 reading 档", mixed2.tier === "reading", `tier=${mixed2.tier} badge=${mixed2.badge}`);
  check("文档页媒体列收敛到可用宽 880", near(mixed2.paneW, SHELL_MAX - RIGHT_COL, 3), `pane=${mixed2.paneW.toFixed(1)}`);

  const widthSamples = samples.map((s) => s.w);
  const tierFlips = samples.reduce((count, sample, index) => {
    return index > 0 && sample.tier !== samples[index - 1].tier ? count + 1 : count;
  }, 0);
  const intermediates = widthSamples.filter((w) => w > mixed1.paneW + 5 && w < mixed2.paneW - 5);
  check("档位切换只发生一次（无逐张漂移）", tierFlips === 1, `tierFlips=${tierFlips}`);
  check("档位切换经 240ms 过渡（采样捕获中间宽度值）", intermediates.length >= 2, `intermediateSamples=${intermediates.length}`);
  check("切换后宽度稳定不再回漂（末段 200ms 全部 ≈ 880）", samples.slice(-6).every((s) => near(s.w, SHELL_MAX - RIGHT_COL, 3)), `tail=${samples.slice(-6).map((s) => s.w.toFixed(0)).join(",")}`);
  check("文档页壳宽同步 = 媒体列 + 440", near(mixed2.shellW, mixed2.paneW + RIGHT_COL, 3), `shell=${mixed2.shellW.toFixed(1)}`);
  await page.screenshot({ path: `${SHOTS}/e1-rig-mixed-doc-reading.png` });

  /* 翻回影像页：镜像一次切换。 */
  await page.keyboard.press("ArrowLeft");
  await page.waitForTimeout(500);
  const mixed3 = await geometry();
  check("翻回影像页回 visual 档", mixed3.tier === "visual", `tier=${mixed3.tier}`);
  check("翻回影像页媒体列回到冻结比例宽", near(mixed3.paneW, mixed1.paneW, 2), `pane=${mixed3.paneW.toFixed(1)} 开窗=${mixed1.paneW.toFixed(1)}`);
  check("翻回影像页壳宽回 = 媒体列 + 440", near(mixed3.shellW, mixed3.paneW + RIGHT_COL, 2), `shell=${mixed3.shellW.toFixed(1)}`);
  await page.screenshot({ path: `${SHOTS}/e1-rig-mixed-doc-back.png` });
  await closeOverlay();
} finally {
  await context.close().catch(() => {});
  await browser.close().catch(() => {});
  mediaServer.close();
}

console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n${fails.join("\n")}`);
process.exit(fails.length === 0 ? 0 : 1);
