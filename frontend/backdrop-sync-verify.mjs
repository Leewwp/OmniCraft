// 遮罩同钟契约 browser verification rig（本地栈，Playwright Chromium）。
// 2026-09-09 #408 验收反馈轮：遮罩视觉移出 ::backdrop（VT 快照期不可见 +
// Safari 不执行 ::backdrop 动画 → 瞬现瞬消），改 dialog 外兄弟层
// .content-detail-backdrop 与壳层/封面同一时钟 JS 驱动。断言：
//   1) 点击后早期（≤180ms）遮罩 ≤ 反馈层 0.35 上浮余量（不得瞬到全黑）；
//   2) 落定后遮罩=1 且壳层=1；
//   3) 关闭期遮罩与壳层同帧下降（存在中间帧 <1 且 >0）；
//   4) 完全退出后遮罩层随之卸载；
//   5) VT 主路径与 FLIP 兜底（init script 删 startViewTransition）各过一遍。
// 附带：浮窗内评论 Composer 发送钮两态截图（空=灰 / 有字=主题色）。
// 运行：BASE=http://localhost:3000 node backdrop-sync-verify.mjs
import { chromium } from "playwright";
import fs from "node:fs";

const BASE = process.env.BASE || "http://localhost:3000";
const PAGE = process.env.PAGE || "/ip/209?tab=share";
const OUT = "../screenshots/backdrop-sync";
fs.mkdirSync(OUT, { recursive: true });

let passed = 0, failed = 0;
function check(name, cond, extra = "") {
  if (cond) { passed++; console.log(`PASS ${name}`); }
  else { failed++; console.log(`FAIL ${name} ${extra}`); }
}

/* rAF 采样器：记录遮罩/壳层 computed opacity。 */
const SAMPLER = () => {
  window.__bs = { samples: [], running: false };
  window.__bsStart = () => {
    window.__bs.samples = [];
    if (window.__bs.running) return;
    window.__bs.running = true;
    const tick = () => {
      if (!window.__bs.running) return;
      const backdrop = document.querySelector(".content-detail-backdrop");
      const shell = document.querySelector("dialog[open] > div");
      window.__bs.samples.push({
        t: performance.now(),
        mask: backdrop ? getComputedStyle(backdrop).opacity : null,
        shell: shell ? getComputedStyle(shell).opacity : null,
        /* VT 关闭档标记（globals.css html[data-vt-close] 把 root 交叉淡化压到
           240ms）：VT 路径的遮罩渐变由快照层承载，活元素瞬时 0 属预期。 */
        vtClose: document.documentElement.hasAttribute("data-vt-close"),
      });
      window.requestAnimationFrame(tick);
    };
    window.requestAnimationFrame(tick);
  };
  window.__bsStop = () => { window.__bs.running = false; };
};

async function sampleAround(page, action) {
  await page.evaluate(() => window.__bsStart());
  await action();
  await page.waitForTimeout(1500);
  await page.evaluate(() => window.__bsStop());
  return page.evaluate(() => window.__bs.samples);
}

async function openFirstCard(page) {
  const card = page
    .locator("article button[aria-label]", { hasText: "" })
    .filter({ has: page.locator('[data-slot="card-cover"]') })
    .first();
  await card.waitFor({ state: "visible", timeout: 10000 });
  await card.click();
}

async function runPass(browser, { label, killVt }) {
  console.log(`\n===== pass: ${label} =====`);
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  if (killVt) {
    await context.addInitScript(() => { delete Document.prototype.startViewTransition; });
  } else {
    /* VT 路径的遮罩渐变在快照层：活元素在回调内瞬时换值，活读数不作早期
       断言。改为捕获 startViewTransition 调用瞬间（= 旧快照烘入值）的活
       遮罩不透明度——开应处于反馈层、关应为全量。 */
    await context.addInitScript(() => {
      const orig = Document.prototype.startViewTransition;
      Document.prototype.startViewTransition = function (cb) {
        const b = document.querySelector(".content-detail-backdrop");
        (window.__vtCaptures ??= []).push(b ? getComputedStyle(b).opacity : null);
        return orig.call(this, cb);
      };
    });
  }
  const page = await context.newPage();
  /* networkidle 在多 pass 后偶发不落定（服务器侧悬挂请求，非断言对象）——
     改 domcontentloaded + 各段自己的元素等待。 */
  await page.goto(`${BASE}${PAGE}`, { waitUntil: "domcontentloaded" });
  await page.evaluate(SAMPLER);

  /* ── 开：早期反馈层 + 落定 ─────────────────────────────── */
  const openSamples = await sampleAround(page, () => openFirstCard(page));
  const clickTime = openSamples[0]?.t ?? 0;
  const early = openSamples.filter((s) => s.t - clickTime < 180 && s.mask !== null);
  const settled = openSamples.filter((s) => s.t - clickTime > 1100);
  const maskElPresent = openSamples.some((s) => s.mask !== null);
  check(`[${label}] backdrop layer exists`, maskElPresent);
  const earlyMax = early.length ? Math.max(...early.map((s) => parseFloat(s.mask))) : null;
  if (killVt) {
    check(
      `[${label}] early mask stays at feedback plateau (<0.55, no instant black)`,
      earlyMax !== null && earlyMax < 0.55,
      `earlyMax=${earlyMax}`,
    );
  } else {
    const openCapture = await page.evaluate(() => (window.__vtCaptures ?? [])[0] ?? null);
    check(
      `[${label}] VT open snapshot bakes the feedback plateau (≤0.55)`,
      openCapture !== null && parseFloat(openCapture) <= 0.55,
      `openCapture=${openCapture}`,
    );
  }
  const settledMask = settled.length ? parseFloat(settled[settled.length - 1].mask) : null;
  const settledShell = settled.length ? parseFloat(settled[settled.length - 1].shell) : null;
  check(`[${label}] settled mask = 1`, settledMask !== null && settledMask > 0.99, `mask=${settledMask}`);
  check(`[${label}] settled shell = 1`, settledShell !== null && settledShell > 0.99, `shell=${settledShell}`);

  /* ── 关：遮罩与壳层同帧下降 + 卸载 ─────────────────────── */
  const closeSamples = await sampleAround(page, () =>
    page.keyboard.press("Escape"),
  );
  const closeStart = closeSamples[0]?.t ?? 0;
  const closeOnes = closeSamples.filter((s) => s.mask !== null && parseFloat(s.mask) > 0.99);
  const closeMid = closeSamples.filter(
    (s) => s.mask !== null && parseFloat(s.mask) > 0.05 && parseFloat(s.mask) < 0.99,
  );
  const closeOnesAfterMid = closeOnes.filter((s) => closeMid.length && s.t > closeMid[0].t);
  const vtCloseFrames = closeSamples.filter((s) => s.vtClose).length;
  const reachedZero = closeSamples.some((s) => s.mask !== null && parseFloat(s.mask) <= 0.05);
  check(`[${label}] close starts from full mask`, closeOnes.length > 0, `ones=${closeOnes.length}`);
  if (killVt) {
    check(
      `[${label}] close has intermediate mask frames (live transition fade)`,
      closeMid.length > 0,
      `mid=${closeMid.length}`,
    );
  } else {
    /* VT 路径：渐变在快照层（root 交叉淡化 240ms 档），活元素瞬时归零；
       断言 240 档标记确实生效 + 活元素确已置 0（回调执行）。 */
    check(`[${label}] close activates the 240ms VT root档 (data-vt-close)`, vtCloseFrames > 0, `frames=${vtCloseFrames}`);
    check(`[${label}] live mask reaches 0 inside the VT callback window`, reachedZero);
    const closeCapture = await page.evaluate(() => {
      const caps = window.__vtCaptures ?? [];
      return caps[caps.length - 1] ?? null;
    });
    check(
      `[${label}] VT close snapshot bakes the full mask (>0.99)`,
      closeCapture !== null && parseFloat(closeCapture) > 0.99,
      `closeCapture=${closeCapture}`,
    );
  }
  check(
    `[${label}] full-opacity frames do not appear after fade begins`,
    closeOnesAfterMid.length === 0,
    `lateOnes=${closeOnesAfterMid.length}`,
  );
  const maskGone = await page
    .locator(".content-detail-backdrop")
    .count()
    .then((n) => n === 0);
  check(`[${label}] backdrop layer unmounts after exit`, maskGone);

  await context.close();
  return { ok: failed === 0 };
}

async function composerStates(browser) {
  console.log("\n===== pass: composer two-state screenshots =====");
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();
  /* 登录种子测试账号（评论区匿名不渲染输入框；Agent 工作台是 Composer
     最直接的可视面，评论/私信/讨论回复共享同一组件与按钮契约）。 */
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "a06-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "A06Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL(/agent|recommend|\/$/, { timeout: 15000 }).catch(() => {});
  await page.goto(`${BASE}/agent`, { waitUntil: "domcontentloaded" });
  const textarea = page.locator("textarea").first();
  await textarea.waitFor({ state: "visible", timeout: 10000 });
  const composerBtn = page.locator("div:has(> textarea) > button").last();
  const emptyClass = await composerBtn.getAttribute("class");
  check("[composer] empty state is gray filled", /bg-canvas-subtle/.test(emptyClass ?? ""), emptyClass ?? "");
  const btnBox = await composerBtn.boundingBox();
  await page.screenshot({
    path: `${OUT}/composer-empty-gray.png`,
    clip: { x: Math.max(0, btnBox.x - 320), y: Math.max(0, btnBox.y - 24), width: Math.min(700, 320 + btnBox.width + 48), height: btnBox.height + 48 },
  });
  await textarea.fill("验证发送按钮两态");
  await page.waitForTimeout(120);
  const filledClass = await composerBtn.getAttribute("class");
  check("[composer] filled state is theme colored", /bg-primary/.test(filledClass ?? ""), filledClass ?? "");
  await page.screenshot({
    path: `${OUT}/composer-filled-primary.png`,
    clip: { x: Math.max(0, btnBox.x - 320), y: Math.max(0, btnBox.y - 24), width: Math.min(700, 320 + btnBox.width + 48), height: btnBox.height + 48 },
  });
  await context.close();
}

/* ── #430 首帧渐进断言：卡片 420 快变体 → 浮窗首帧保持层秒出 → settle 后
   交叉淡入 1080 规范变体（保持层卸载）。机制级（DOM/加载状态），不依赖
   录屏帧率，dev 与生产构建均可跑。 */
async function firstFrameProgressive(browser) {
  console.log("\n===== pass: #430 first-frame progressive =====");
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  /* load + 尽力 networkidle + 稳定窗：domcontentloaded 即点卡片会与水合竞态
     （点击无响应→浮窗迟开，首帧断言失真）；networkidle 偶发不落定不做硬门。 */
  await page.goto(`${BASE}${PAGE}`, { waitUntil: "load" });
  await page.waitForLoadState("networkidle", { timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(300);

  /* 卡片封面 = 420 快变体（选位图封面卡：SVG 直通无变体链路，不属本断言面）。 */
  const card = page
    .locator("article button[aria-label]", { hasText: "" })
    .filter({
      has: page.locator('[data-slot="card-cover"] img[src*="/_next/image"]'),
    })
    .first();
  await card.waitFor({ state: "visible", timeout: 10000 });
  const cardImg = card.locator('[data-slot="card-cover"] img[src*="/_next/image"]').first();
  /* currentSrc（绝对地址）与浮窗捕获的 hold src 同形——用解析后的地址比对。 */
  const cardSrc = await cardImg.evaluate((el) => el.src);
  check("[first-frame] card cover uses the 420 quick variant", /w=420&q=75/.test(cardSrc ?? ""), cardSrc ?? "");

  /* hover 65ms+ 预热 1080（预取时机前移的目标变体），随后点击。
     位图卡可能在折叠线下：先滚入视口并等瀑布流稳定，避免点击与无限滚动
     竞态（Playwright actionability 稳定门会重试到超时）。 */
  await card.scrollIntoViewIfNeeded();
  await page.waitForTimeout(800);
  await card.hover();
  await page.waitForTimeout(200);
  const clickStarted = Date.now();
  await card.click();

  /* 早期窗（≤1.5s，含详情拉取+挂载）：保持层已出图（complete+naturalWidth>0）
     ——无 spinner 空窗。waitFor visible 只保证进 DOM，不保证解码完成。 */
  const holdLocator = page.locator('dialog img[data-slot="cover-hold"]');
  let earlyHoldPainted = false;
  try {
    await page.waitForFunction(
      () => {
        const el = document.querySelector("dialog img[data-slot='cover-hold']");
        return Boolean(el && el.complete && el.naturalWidth > 0);
      },
      null,
      { timeout: 1500 },
    );
    earlyHoldPainted = true;
  } catch { /* fallthrough to the check */ }
  check(
    "[first-frame] overlay first frame paints the held card variant (no spinner gap)",
    earlyHoldPainted,
    `elapsed=${Date.now() - clickStarted}ms`,
  );
  const holdSrc = earlyHoldPainted ? await holdLocator.first().getAttribute("src") : null;
  check("[first-frame] hold layer src matches the card quick variant", holdSrc === cardSrc, `${holdSrc}`);
  await page.screenshot({ path: `${OUT}/sp430-first-frame-hold.png` });

  /* settle 后交叉淡入：保持层淡出并卸载，规范层 1080 承接可视。 */
  let holdGone = false;
  try {
    await holdLocator.first().waitFor({ state: "detached", timeout: 8000 });
    holdGone = true;
  } catch { /* fallthrough */ }
  check("[first-frame] hold layer fades out and unmounts after settle", holdGone);

  const coverImg = page.locator('dialog [data-slot="detail-cover"] img').first();
  const settledSrc = await coverImg.getAttribute("src").catch(() => null);
  check(
    "[first-frame] settled cover is the 1080 canonical variant",
    /w=1080&q=75/.test(settledSrc ?? ""),
    settledSrc ?? "",
  );
  const settledPainted = await coverImg.evaluate((el) => el.naturalWidth > 0).catch(() => false);
  check("[first-frame] settled canonical variant is decoded and visible", settledPainted);
  await page.screenshot({ path: `${OUT}/sp430-settled-1080.png` });

  await context.close();
}

/* ONLY=first-frame 可单独跑某段（服务器多轮全页加载后偶发降速时分段复验）。 */
const ONLY = process.env.ONLY ?? "";
const browser = await chromium.launch();
try {
  if (!ONLY || ONLY === "vt") await runPass(browser, { label: "vt", killVt: false });
  if (!ONLY || ONLY === "flip") await runPass(browser, { label: "flip-fallback", killVt: true });
  if (!ONLY || ONLY === "first-frame") await firstFrameProgressive(browser);
  if (!ONLY || ONLY === "composer") await composerStates(browser);
} finally {
  await browser.close();
}
console.log(`\nRESULT pass=${passed} fail=${failed}`);
process.exit(failed === 0 ? 0 : 1);
