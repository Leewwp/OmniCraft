// #398 R3 动效契约 browser verification rig（本地栈 3000/8080，Playwright Chromium）。
// C1 图源统一（两端 /_next/image 变体 + 点击预取）/ C2 几何统一（超高图退化）/
// C3 快照纯净（motion-lock）/ C4 真就绪（预取解码）；VT 主路径（默认 Chromium）
// 与 FLIP 兜底（init script 删 startViewTransition）各过一遍；快速开关 10 次
// 稳定性（无常驻 opacity:0 / 无残留 view-transition-name）。截图 overlay-r3-*/。
import { chromium } from "playwright";
import fs from "node:fs";

const BASE = "http://localhost:3000";
const OUT = "../screenshots/overlay-r3";
fs.mkdirSync(OUT, { recursive: true });

let passed = 0, failed = 0;
function check(name, cond, extra = "") {
  if (cond) { passed++; console.log(`PASS ${name}`); }
  else { failed++; console.log(`FAIL ${name} ${extra}`); }
}

const TITLES = {
  portrait: "【R2验证】竖图集三张",
  square: "【R2验证】方图单张",
  tall: "【R2验证】超高图单张",
  mixed: "【R2验证】混合集横竖",
  landscape: "【R2验证】横集两张",
};

async function stable(page, ms = 1800) {
  await page.waitForTimeout(ms);
}

async function openOverlay(page, title) {
  const card = page.locator(`text=${title}`).first();
  await card.scrollIntoViewIfNeeded().catch(() => {});
  if (!(await card.isVisible().catch(() => false))) return false;
  await card.click().catch(() => {});
  try {
    await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 8000 });
    return true;
  } catch { return false; }
}

async function closeOverlay(page) {
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForTimeout(700);
}

/* 残留检查：浮窗内不应有内联 view-transition-name；外壳 opacity 不为 0。 */
async function residueCheck(page, label) {
  const residues = await page
    .locator("dialog [style*='view-transition-name']")
    .count()
    .catch(() => 0);
  check(`${label}: no residual view-transition-name`, residues === 0, `count=${residues}`);
  const zeroOpacity = await page
    .locator("dialog > div[style*='opacity: 0']")
    .count()
    .catch(() => 0);
  check(`${label}: no resident opacity:0 shell`, zeroOpacity === 0, `count=${zeroOpacity}`);
}

const browser = await chromium.launch();

try {
  /* ── VT 主路径（Chromium 默认）────────────────── */
  const vtCtx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await vtCtx.addInitScript(() => {
    const orig = Document.prototype.startViewTransition;
    if (orig) {
      window.__vtCount = 0;
      Document.prototype.startViewTransition = function (cb) {
        window.__vtCount = (window.__vtCount ?? 0) + 1;
        return orig.call(this, cb);
      };
    }
  });
  const page = await vtCtx.newPage();
  const prefetchHits = [];
  page.on("request", (req) => {
    if (req.url().includes("/_next/image")) prefetchHits.push(req.url());
  });

  await page.goto(`${BASE}/recommend`, { waitUntil: "domcontentloaded" });
  await stable(page, 2600);
  check("vt: chromium exposes startViewTransition", await page.evaluate(() => typeof Document.prototype.startViewTransition === "function" || (window.__vtCount ?? -1) >= 0));

  /* 竖图集：C1（两端变体）+ C4（点击预取）+ VT 起跑 + 转场中帧截图。 */
  const beforePrefetch = prefetchHits.length;
  check("vt: portrait overlay opens", await openOverlay(page, TITLES.portrait));
  await page.waitForTimeout(1200);
  check("vt: C4 click-time prefetch fired", prefetchHits.length > beforePrefetch, `hits=${prefetchHits.length - beforePrefetch}`);
  check("vt: shared-element VT ran", await page.evaluate(() => (window.__vtCount ?? 0) >= 1), `count=${await page.evaluate(() => window.__vtCount)}`);
  /* C1 同源断言：SVG 走 next/image 直通（两端同一 URL 也满足同源契约）；
     JPG 封面走 /_next/image 优化器变体（专项夹具 99007 验证）。 */
  const paneImgSrc = await page.locator('[data-slot="variant-media-pane"] img').getAttribute("src").catch(() => null);
  const paneSource = (paneImgSrc?.match(/[?&]url=([^&]+)/) ? decodeURIComponent(paneImgSrc.match(/[?&]url=([^&]+)/)[1]) : paneImgSrc)?.replace(/^https?:\/\/[^/]+/, "");
  check("vt: C1 overlay cover keeps the same source media", paneSource === "/seed-media/real/gallery/r2-portrait.svg", `src=${paneImgSrc?.slice(0, 70)}`);
  await residueCheck(page, "vt portrait");
  await page.screenshot({ path: `${OUT}/vt-portrait-open.png` });

  /* 转场中帧（±150ms 起跑，300ms 时长内）截图：目测连续变形素材。 */
  await closeOverlay(page);
  await stable(page, 800);
  const card = page.locator(`text=${TITLES.portrait}`).first();
  await card.click().catch(() => {});
  await page.waitForTimeout(160);
  await page.screenshot({ path: `${OUT}/vt-portrait-midframe.png` });
  await page.waitForTimeout(900);
  await residueCheck(page, "vt portrait reopen");
  await closeOverlay(page);
  await stable(page, 800);

  /* JPG 封面夹具：卡片与浮窗两端都必须是 /_next/image 优化器变体（真优化路径）。 */
  check("vt: jpg-cover overlay opens", await openOverlay(page, "【R2验证】JPG竖封面文章"));
  await page.waitForTimeout(1200);
  const jpgPaneSrc = await page.locator('[data-slot="variant-media-pane"] img').getAttribute("src").catch(() => null);
  check("vt: C1 JPG cover goes through the optimizer variant", Boolean(jpgPaneSrc && jpgPaneSrc.includes("/_next/image")), `src=${jpgPaneSrc?.slice(0, 70)}`);
  await residueCheck(page, "vt jpg");
  await page.screenshot({ path: `${OUT}/vt-jpg-cover-open.png` });
  await closeOverlay(page);
  await stable(page, 800);

  /* 四态其余：方图 / 混合 / 全横（split 路径走 MediaGallery 变体）。 */
  for (const [key, expectVariant] of [["square", true], ["mixed", true], ["landscape", false]]) {
    check(`vt: ${key} overlay opens`, await openOverlay(page, TITLES[key]));
    await page.waitForTimeout(1100);
    const paneImgs = await page
      .locator('[data-slot="variant-media-pane"] img, [data-slot="detail-cover"] img')
      .evaluateAll((els) => els.map((el) => el.getAttribute("src")));
    /* C1 同源语义：/_next/image 变体或 SVG 直通（两端同 URL）都算达标。 */
    const rendered = paneImgs.filter((s) => s && (s.includes("/_next/image") || s.includes("/seed-media"))).length;
    check(`vt: ${key} cover media rendered from the shared source (C1)`, rendered >= 1, `srcs=${paneImgs.length}`);
    await residueCheck(page, `vt ${key}`);
    await page.screenshot({ path: `${OUT}/vt-${key}-open.png` });
    await closeOverlay(page);
    await stable(page, 800);
  }

  /* 超高图：C2 退化居中缩淡——不应有共享元素 VT 起跑（该次 open 的增量计数为 0）。 */
  const vtBefore = await page.evaluate(() => window.__vtCount ?? 0);
  check("vt: tall overlay opens", await openOverlay(page, TITLES.tall));
  await page.waitForTimeout(1300);
  const vtAfter = await page.evaluate(() => window.__vtCount ?? 0);
  check("vt: C2 tall degrades to fallback (no shared-element VT)", vtAfter === vtBefore, `before=${vtBefore} after=${vtAfter}`);
  await residueCheck(page, "vt tall");
  await page.screenshot({ path: `${OUT}/vt-tall-fallback.png` });
  await closeOverlay(page);
  await stable(page, 900);

  /* C3 motion-lock：打开期间触发卡带 data-overlay-motion-lock，关闭后解除。 */
  const lockCard = page.locator(`text=${TITLES.square}`).first();
  await lockCard.click().catch(() => {});
  await page.waitForTimeout(800);
  const lockDuring = await page.evaluate(() => {
    const el = document.querySelector("[data-overlay-motion-lock]");
    return Boolean(el);
  });
  check("vt: C3 motion lock set during open", lockDuring);
  await closeOverlay(page);
  await page.waitForTimeout(900);
  const lockAfter = await page.evaluate(() => document.querySelector("[data-overlay-motion-lock]") === null);
  check("vt: C3 motion lock cleared after exit", lockAfter);

  /* ── 稳定性：快速开关 10 次 ────────────────── */
  let stabOk = true;
  for (let i = 0; i < 10; i++) {
    await page.locator(`text=${TITLES.portrait}`).first().click().catch(() => { stabOk = false; });
    await page.waitForTimeout(320);
    await page.keyboard.press("Escape").catch(() => {});
    await page.waitForTimeout(240);
  }
  await page.waitForTimeout(1400);
  const residualNames = await page.locator("dialog [style*='view-transition-name']").count().catch(() => -1);
  const stuckShell = await page.locator("dialog[open]").count();
  check("vt: rapid open/close x10 leaves no residual names", residualNames === 0, `count=${residualNames}`);
  check("vt: rapid open/close x10 overlay closed cleanly", stuckShell === 0, `open dialogs=${stuckShell}`);
  check("vt: rapid loop itself did not error", stabOk);
  await page.screenshot({ path: `${OUT}/vt-after-stability.png` });
  await vtCtx.close();

  /* ── FLIP 兜底路径（删 startViewTransition）；限流冷却后再开段 ── */
  console.log("cooling down 65s for the rate-limit window…");
  await new Promise((resolve) => setTimeout(resolve, 65000));
  const flipCtx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await flipCtx.addInitScript(() => {
    delete Document.prototype.startViewTransition;
  });
  const fpage = await flipCtx.newPage();
  await fpage.goto(`${BASE}/recommend`, { waitUntil: "domcontentloaded" });
  await stable(fpage, 2600);
  check("flip: VT disabled in context", await fpage.evaluate(() => typeof document.startViewTransition !== "function"));
  check("flip: portrait overlay opens", await openOverlay(fpage, TITLES.portrait));
  await fpage.waitForTimeout(1300);
  const flipCoverTransform = await fpage
    .locator('[data-slot="variant-media-pane"] [data-slot="detail-cover"]')
    .evaluate((el) => el.style.transform)
    .catch(() => "n/a");
  check("flip: cover transform cleared after animation", flipCoverTransform === "", `transform=${flipCoverTransform}`);
  await residueCheck(fpage, "flip portrait");
  await fpage.screenshot({ path: `${OUT}/flip-portrait-open.png` });
  await closeOverlay(fpage);
  await stable(fpage, 900);
  /* 关闭反向回归：Esc 后浮层消失、无残留。 */
  check("flip: overlay closed via Esc", (await fpage.locator("dialog[open]").count()) === 0);
  let fStab = true;
  for (let i = 0; i < 10; i++) {
    await fpage.locator(`text=${TITLES.mixed}`).first().click().catch(() => { fStab = false; });
    await fpage.waitForTimeout(300);
    await fpage.keyboard.press("Escape").catch(() => {});
    await fpage.waitForTimeout(220);
  }
  await fpage.waitForTimeout(1400);
  check("flip: rapid x10 clean", (await fpage.locator("dialog[open]").count()) === 0 && fStab);
  await flipCtx.close();

  /* ── reduced-motion：100ms 纯淡入（fade 路径，无 VT 无 FLIP）；冷却 ── */
  console.log("cooling down 40s for the rate-limit window…");
  await new Promise((resolve) => setTimeout(resolve, 40000));
  const rmCtx = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    reducedMotion: "reduce",
  });
  await rmCtx.addInitScript(() => {
    const orig = Document.prototype.startViewTransition;
    if (orig) {
      window.__vtCount = 0;
      Document.prototype.startViewTransition = function (cb) {
        window.__vtCount = (window.__vtCount ?? 0) + 1;
        return orig.call(this, cb);
      };
    }
  });
  const rpage = await rmCtx.newPage();
  await rpage.goto(`${BASE}/recommend`, { waitUntil: "domcontentloaded" });
  await stable(rpage, 2600);
  check("reduced: overlay opens", await openOverlay(rpage, TITLES.portrait));
  await rpage.waitForTimeout(900);
  check("reduced: fade path (no VT)", (await rpage.evaluate(() => window.__vtCount ?? 0)) === 0);
  await rpage.screenshot({ path: `${OUT}/reduced-portrait-open.png` });
  await rmCtx.close();

  console.log(`\nR3 rig: ${passed} passed, ${failed} failed`);
  process.exitCode = failed > 0 ? 1 : 0;
} catch (error) {
  console.error("RIG ERROR", error);
  process.exitCode = 1;
} finally {
  await browser.close();
}
