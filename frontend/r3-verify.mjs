// 浮窗动效契约 browser verification rig（本地栈，Playwright Chromium）。
// #398 R3（C1 图源统一 / C2 超高图退化 / C3 快照纯净 / C4 真就绪）+ #409 F1
// 动效契约重建（单一时间轴 / 三处同源 / 动效期间零换图 / 首帧保持 / 时长
// 300/240 逐向单一时钟）。VT 主路径（默认 Chromium）与 FLIP 兜底（init script
// 删 startViewTransition）各过一遍；快速开关 10 次稳定性；reduced-motion。
// 截图 ../screenshots/overlay-r3/。
//
// 夹具：本地库 IP 209（seed-ui-rich-01）share tab 的【F1验证】* 内容
// （scripts 外直插，热榜/最新排序置顶）；2026-09-07 整库恢复后 R2 夹具
// 已不存在，本 rig 2026-09-08 起改用 F1 夹具面。
// 运行：BASE=http://localhost:3002 PAGE=/ip/209?tab=share node r3-verify.mjs
import { chromium } from "playwright";
import fs from "node:fs";

const BASE = process.env.BASE || "http://localhost:3000";
const PAGE = process.env.PAGE || "/recommend";
const OUT = "../screenshots/overlay-r3";
fs.mkdirSync(OUT, { recursive: true });

let passed = 0, failed = 0;
function check(name, cond, extra = "") {
  if (cond) { passed++; console.log(`PASS ${name}`); }
  else { failed++; console.log(`FAIL ${name} ${extra}`); }
}

const TITLES = {
  portrait: "【F1验证】竖图集三张",
  square: "【F1验证】方图单张",
  tall: "【F1验证】超高图单张",
  mixed: "【F1验证】混合集横竖",
  landscape: "【F1验证】横集两张",
  jpg: "【F1验证】JPG竖封面文章",
};

/* rAF 采样器（#409 F1）：每帧记录壳层 opacity / 封面 transform / 封面 src /
   骨架存在 / VT 命名残留；起跑时抓一次 transitionDuration（时长契约）。 */
const SAMPLER = () => {
  window.__f1 = { samples: [], running: false, shellDur: null, coverDur: null };
  window.__f1Start = () => {
    if (window.__f1.running) return;
    window.__f1.running = true;
    const tick = () => {
      if (!window.__f1.running) return;
      const shell = document.querySelector("dialog[open] > div");
      const covers = Array.from(document.querySelectorAll('[data-slot="detail-cover"]'));
      const cover = covers.filter((c) => c.offsetParent !== null).pop() ?? null;
      if (shell && cover) {
        const so = getComputedStyle(shell).opacity;
        const ct = getComputedStyle(cover).transform;
        /* 时长契约捕获：仅在动画进行中的帧（opacity 渐变中 / transform 帧间变化）
           抓 computed transitionDuration——Invert 静态位姿帧上 transition 为 none，
           会误捕 0s。 */
        const soV = parseFloat(so);
        const prev = window.__f1.samples[window.__f1.samples.length - 1];
        const ctChanging = prev && prev.ct !== ct;
        if (soV > 0.01 && soV < 0.99) {
          const d = getComputedStyle(shell).transitionDuration;
          if (window.__f1.shellDur === null && d !== "0s") window.__f1.shellDur = d;
        }
        if (ct && ct !== "none" && ctChanging) {
          const d = getComputedStyle(cover).transitionDuration;
          if (window.__f1.coverDur === null && d !== "0s") window.__f1.coverDur = d;
        }
        window.__f1.samples.push({
          t: performance.now(),
          so,
          ct,
          /* #430 双层：可见层 = 保持层（动效期，卡片同串）在顶，否则规范层。 */
          src: (cover.querySelector('img[data-slot="cover-hold"]') ?? cover.querySelector("img"))?.currentSrc ?? null,
          sk: Boolean(cover.querySelector(".animate-pulse")),
        });
      }
      window.requestAnimationFrame(tick);
    };
    window.requestAnimationFrame(tick);
  };
  window.__f1Stop = () => { window.__f1.running = false; };
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

/** 分析一次采样流：active 窗口 = opacity 渐变中（so∈(0.01,0.99)）或 transform
    帧间变化（矩阵在动；静态 invert 位姿只算起跑前一帧的噪声，±2 帧容差）。 */
function analyze(samples) {
  const ramping = (s) => {
    const v = parseFloat(s.so);
    return v > 0.01 && v < 0.99;
  };
  const geoChanging = (i) => {
    const s = samples[i];
    if (!s.ct || s.ct === "none") return false;
    const prev = samples[i - 1]?.ct;
    const next = samples[i + 1]?.ct;
    return (prev !== undefined && s.ct !== prev) || (next !== undefined && s.ct !== next);
  };
  const rampIdx = samples.map((_, i) => i).filter((i) => ramping(samples[i]));
  const geoIdx = samples.map((_, i) => i).filter((i) => geoChanging(i));
  const first = Math.min(
    ...(rampIdx.length ? rampIdx : [Infinity]),
    ...(geoIdx.length ? geoIdx : [Infinity]),
  );
  const last = Math.max(
    ...(rampIdx.length ? rampIdx : [-Infinity]),
    ...(geoIdx.length ? geoIdx : [-Infinity]),
  );
  const hasWindow = Number.isFinite(first) && Number.isFinite(last) && last >= first;
  const active = hasWindow ? samples.slice(first, last + 1) : [];
  return {
    ok: hasWindow,
    elapsed: hasWindow && active.length > 1 ? samples[last].t - samples[first].t : 0,
    rampFirst: rampIdx.length ? rampIdx[0] : -1,
    rampLast: rampIdx.length ? rampIdx[rampIdx.length - 1] : -1,
    geoFirst: geoIdx.length ? geoIdx[0] : -1,
    geoLast: geoIdx.length ? geoIdx[geoIdx.length - 1] : -1,
    srcsDuringActive: [...new Set(active.map((s) => s.src).filter(Boolean))],
    skeletonsDuringActive: active.some((s) => s.sk),
    preActiveMaxOpacity: hasWindow && first > 0 ? Math.max(...samples.slice(0, first).map((s) => parseFloat(s.so))) : 0,
    srcFlipAfterActive: (() => {
      if (!hasWindow || last + 1 >= samples.length) return null;
      const during = new Set(active.map((s) => s.src).filter(Boolean));
      const flipIdx = samples.findIndex((s, i) => i > last && s.src && !during.has(s.src));
      if (flipIdx < 0) return null;
      return { at: samples[flipIdx].t - samples[first].t };
    })(),
  };
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

/* VT 伪元素动画时长/缓动（document.getAnimations 的 pseudoElement 目标）。 */
async function vtAnimationTimings(page) {
  return page.evaluate(() => {
    const out = {};
    for (const anim of document.getAnimations()) {
      const pseudo = anim.effect?.pseudoElement ?? "";
      if (!pseudo.includes("view-transition")) continue;
      const timing = anim.effect?.getTiming?.() ?? {};
      const key = pseudo.replace("::view-transition-", "").replace(/([)"])/g, "");
      out[key] = { duration: timing.duration, easing: timing.easing ?? null };
    }
    return out;
  });
}

const browser = await chromium.launch();

try {
  /* ── FLIP 兜底路径（删 startViewTransition）：单一时间轴机器断言 ── */
  const flipCtx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await flipCtx.addInitScript((arg) => {
    delete Document.prototype.startViewTransition;
    window.eval(`(${arg.SAMPLER_SRC})()`);
  }, { SAMPLER_SRC: SAMPLER.toString() });
  const fpage = await flipCtx.newPage();
  await fpage.goto(`${BASE}${PAGE}`, { waitUntil: "domcontentloaded" });
  await stable(fpage, 2600);
  check("flip: VT disabled in context", await fpage.evaluate(() => typeof document.startViewTransition !== "function"));

  /* 开：点击前挂采样器 → 壳层与封面同帧起跑/同窗结束、300ms 单一时钟、
     零换图、零骨架、无起跑前壳层淡入（160ms 先行反馈已移除）。 */
  await fpage.evaluate(() => window.__f1Start());
  const flipCard = fpage.locator(`text=${TITLES.portrait}`).first();
  await flipCard.scrollIntoViewIfNeeded().catch(() => {});
  await flipCard.click().catch(() => {});
  await fpage.locator("dialog[open]").waitFor({ state: "visible", timeout: 9000 }).catch(() => {});
  await stable(fpage, 1400);
  const openSamples = await fpage.evaluate(() => { window.__f1Stop(); return window.__f1.samples; });
  const openAna = analyze(openSamples);
  const openDurs = await fpage.evaluate(() => ({ shell: window.__f1.shellDur, cover: window.__f1.coverDur }));
  check("flip: open sampled enough frames", openSamples.length > 10 && openAna.ok, `n=${openSamples.length} window=${openAna.ok}`);
  check("flip: open shell+cover start same frame (±2)", openAna.rampFirst >= 0 && openAna.geoFirst >= 0 && Math.abs(openAna.rampFirst - openAna.geoFirst) <= 2, `ramp=${openAna.rampFirst} geo=${openAna.geoFirst}`);
  check("flip: open shell+cover end same frame (±3)", openAna.rampLast >= 0 && openAna.geoLast >= 0 && Math.abs(openAna.rampLast - openAna.geoLast) <= 3, `ramp=${openAna.rampLast} geo=${openAna.geoLast}`);
  check("flip: open elapsed ≈ 300ms", openAna.elapsed >= 220 && openAna.elapsed <= 540, `elapsed=${Math.round(openAna.elapsed)}ms`);
  check("flip: open duration contract 300ms on both shell and cover", openDurs.shell === "0.3s" && openDurs.cover === "0.3s", `shell=${openDurs.shell} cover=${openDurs.cover}`);
  check("flip: zero src swap during open", openAna.srcsDuringActive.length <= 1, `srcs=${openAna.srcsDuringActive.length}`);
  check("flip: no skeleton during open motion", !openAna.skeletonsDuringActive);
  check("flip: no pre-motion shell fade (160ms early feedback removed)", openAna.preActiveMaxOpacity <= 0.011, `max=${openAna.preActiveMaxOpacity}`);
  await residueCheck(fpage, "flip portrait open");
  await fpage.screenshot({ path: `${OUT}/f1-flip-portrait-open.png` });

  /* 关：240ms 单一时钟 + 零换图。 */
  await fpage.evaluate(() => { window.__f1.samples = []; window.__f1.shellDur = null; window.__f1.coverDur = null; });
  await fpage.evaluate(() => window.__f1Start());
  await fpage.keyboard.press("Escape").catch(() => {});
  await stable(fpage, 1200);
  const closeSamples = await fpage.evaluate(() => { window.__f1Stop(); return window.__f1.samples; });
  const closeAna = analyze(closeSamples);
  const closeDurs = await fpage.evaluate(() => ({ shell: window.__f1.shellDur, cover: window.__f1.coverDur }));
  check("flip: close elapsed ≈ 240ms", closeAna.ok && closeAna.elapsed >= 170 && closeAna.elapsed <= 430, `ok=${closeAna.ok} elapsed=${Math.round(closeAna.elapsed)}ms`);
  check("flip: close shell+cover start same frame (±2)", closeAna.rampFirst >= 0 && closeAna.geoFirst >= 0 && Math.abs(closeAna.rampFirst - closeAna.geoFirst) <= 2, `ramp=${closeAna.rampFirst} geo=${closeAna.geoFirst}`);
  check("flip: close duration contract 240ms on both shell and cover", closeDurs.shell === "0.24s" && closeDurs.cover === "0.24s", `shell=${closeDurs.shell} cover=${closeDurs.cover}`);
  check("flip: close zero src swap", closeAna.srcsDuringActive.length <= 1, `srcs=${closeAna.srcsDuringActive.length}`);
  check("flip: overlay closed via Esc", (await fpage.locator("dialog[open]").count()) === 0);
  await stable(fpage, 700);

  /* 首帧保持（混合集 = 媒体链首项与卡片封面不同文件）：动效全程渲染卡片封面，
     落定后才切换到链内媒体（post-settle swap）。 */
  const mixedCard = fpage.locator(`text=${TITLES.mixed}`).first();
  await mixedCard.scrollIntoViewIfNeeded().catch(() => {});
  const cardSrcMixed = await fpage.evaluate((t) => {
    const el = document.evaluate(`//article[.//*[contains(text(),'${t.slice(1, -1)}')]]`, document, null, 9, null).singleNodeValue;
    return el?.querySelector('[data-slot="card-cover"] img')?.currentSrc ?? null;
  }, TITLES.mixed);
  await fpage.evaluate(() => window.__f1.samples = []);
  await fpage.evaluate(() => window.__f1Start());
  await mixedCard.click().catch(() => {});
  await fpage.locator("dialog[open]").waitFor({ state: "visible", timeout: 9000 }).catch(() => {});
  await stable(fpage, 1500);
  const mixedSamples = await fpage.evaluate(() => { window.__f1Stop(); return window.__f1.samples; });
  const mixedAna = analyze(mixedSamples);
  check("flip: mixed entrance renders card cover (first-frame hold)", mixedAna.ok && mixedAna.srcsDuringActive.length === 1 && mixedAna.srcsDuringActive[0] === cardSrcMixed, `ok=${mixedAna.ok} active srcs=${JSON.stringify(mixedAna.srcsDuringActive).slice(0, 90)} card=${cardSrcMixed?.slice(0, 60)}`);
  check("flip: mixed swaps to chain media only after settle", Boolean(mixedAna.srcFlipAfterActive && mixedAna.srcFlipAfterActive.at >= mixedAna.elapsed - 60), `flipAt=${mixedAna.srcFlipAfterActive?.at} elapsed=${Math.round(mixedAna.elapsed)}`);
  await closeOverlay(fpage);
  await stable(fpage, 700);
  await flipCtx.close();

  /* ── VT 主路径（Chromium 默认）────────────────── */
  console.log("cooling down 60s for the rate-limit window…");
  await new Promise((resolve) => setTimeout(resolve, 60000));
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

  await page.goto(`${BASE}${PAGE}`, { waitUntil: "domcontentloaded" });
  await stable(page, 2600);

  /* 开：VT 起跑 + 伪元素时长断言（root 交叉淡化与命名组同长 = 单一时间轴）。 */
  const beforePrefetch = prefetchHits.length;
  const vtBefore = await page.evaluate(() => window.__vtCount ?? 0);
  check("vt: portrait overlay opens", await openOverlay(page, TITLES.portrait));
  await page.waitForTimeout(110);
  const openTimings = await vtAnimationTimings(page);
  const groupOpen = openTimings["group(content-detail-cover"] ?? openTimings["group(content-detail-cover)"];
  const rootOldOpen = openTimings["old(root"] ?? openTimings["old(root)"];
  check("vt: shared-element VT ran", (await page.evaluate(() => window.__vtCount ?? 0)) > vtBefore);
  check("vt: open group animation 300ms", groupOpen?.duration === 300, `dur=${groupOpen?.duration}`);
  check("vt: open root crossfade 300ms (single timeline)", rootOldOpen?.duration === 300, `dur=${rootOldOpen?.duration}`);
  await stable(page, 900);
  /* #409 F1：SVG 封面为直通源（优化器 400），点击预取按契约跳过——本段
     预取命中断言移至 JPG 夹具段（位图才有规范变体请求）。 */

  /* 同源断言：卡片封面与浮窗首帧渲染同一地址（SVG 直通 / 位图规范变体）。 */
  /* #430 双层：canonical 层（首 img）为规范源；hold 层为卡片同串（fade 后卸载）。 */
  const paneImgSrc = await page.locator('[data-slot="variant-media-pane"] img').first().getAttribute("src").catch(() => null);
  check("vt: C1 overlay cover keeps the same source media", paneImgSrc === "/seed-media/real/gallery/f1-portrait-1.svg", `src=${paneImgSrc?.slice(0, 70)}`);
  await residueCheck(page, "vt portrait");
  await page.screenshot({ path: `${OUT}/f1-vt-portrait-open.png` });
  await closeOverlay(page);
  await stable(page, 800);

  /* 关：data-vt-close 方向档（root/group 同压 240ms），结束移除。 */
  await page.locator(`text=${TITLES.portrait}`).first().click().catch(() => {});
  try { await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 8000 }); } catch { /* 继续（限流降级） */ }
  await page.waitForTimeout(1100);
  const closeVTCount = await page.evaluate(() => window.__vtCount ?? 0);
  /* Esc 后在页内轮询抓第一帧伪元素动画快照（标记先于 VT 起跑置上，
     动画对象在 ready 后才存在，须等它们出现再读时长）。 */
  const closeTimingsP = page.evaluate(() => new Promise((resolve) => {
    const seen = {};
    const deadline = performance.now() + 900;
    const poll = () => {
      let found = 0;
      for (const anim of document.getAnimations()) {
        const pseudo = anim.effect?.pseudoElement ?? "";
        if (!pseudo.includes("view-transition")) continue;
        const timing = anim.effect?.getTiming?.() ?? {};
        if (!(pseudo in seen)) { seen[pseudo] = { duration: timing.duration }; found++; }
      }
      if (found >= 4 || performance.now() > deadline) resolve(seen);
      else requestAnimationFrame(poll);
    };
    poll();
  }));
  const markerP = page.waitForFunction(() => document.documentElement.hasAttribute("data-vt-close"), { timeout: 2500 }).then(() => true).catch(() => false);
  await page.keyboard.press("Escape").catch(() => {});
  const closeAttr = await markerP;
  const closeTimings = await closeTimingsP;
  const groupClose = closeTimings["::view-transition-group(content-detail-cover)"];
  const rootOldClose = closeTimings["::view-transition-old(root)"];
  const groupCloseDur = groupClose?.duration ?? null;
  const rootOldCloseDur = rootOldClose?.duration ?? null;
  check("vt: close direction marker observed during close VT", Boolean(closeAttr));
  check("vt: close group animation 240ms", groupCloseDur === 240, `dur=${groupCloseDur}`);
  check("vt: close root crossfade 240ms (single close clock)", rootOldCloseDur === 240, `dur=${rootOldCloseDur}`);
  await stable(page, 900);
  check("vt: close direction marker removed after settle", await page.evaluate(() => !document.documentElement.hasAttribute("data-vt-close")));
  check("vt: close fired a view transition", (await page.evaluate(() => window.__vtCount ?? 0)) > closeVTCount);
  await closeOverlay(page);
  await stable(page, 800);

  /* JPG 封面夹具：卡片与浮窗两端都必须是 w=1080 规范变体（真优化路径、三处同源）。 */
  check("vt: jpg-cover overlay opens", await openOverlay(page, TITLES.jpg));
  await page.waitForTimeout(1200);
  const jpgCardSrc = await page.evaluate(() => {
    const el = document.evaluate("//article[.//*[contains(text(),'JPG竖封面文章')]]", document, null, 9, null).singleNodeValue;
    return el?.querySelector('[data-slot="card-cover"] img')?.getAttribute("src") ?? null;
  });
  const jpgPaneSrc = await page.locator('[data-slot="variant-media-pane"] img').first().getAttribute("src").catch(() => null);
  const jpgHoldSrc = await page
    .locator('[data-slot="variant-media-pane"] img[data-slot="cover-hold"]')
    .getAttribute("src")
    .catch(() => null);
  /* #430 两变体渐进（修订 #409 F1 三处同源）：卡片 420 快变体；浮窗规范层
     w=1080；若保持层仍在（位图且未过 settle 窗），其串 = 卡片 420 变体。 */
  check(
    "vt: jpg card quick variant 420, overlay canonical 1080 (two-variant progressive)",
    Boolean(
      jpgCardSrc &&
      jpgCardSrc.includes("/_next/image") &&
      jpgCardSrc.includes("w=420") &&
      jpgPaneSrc?.includes("w=1080") &&
      (!jpgHoldSrc || jpgHoldSrc.includes("w=420")),
    ),
    `card=${jpgCardSrc?.slice(0, 70)} pane=${jpgPaneSrc?.slice(0, 70)} hold=${jpgHoldSrc?.slice(0, 70)}`,
  );
  check("vt: C4 click-time prefetch fired for the bitmap cover", prefetchHits.some((u) => u.includes("w=1080")), `w1080 hits=${prefetchHits.filter((u) => u.includes("w=1080")).length}/${prefetchHits.length}`);
  await residueCheck(page, "vt jpg");
  await page.screenshot({ path: `${OUT}/f1-vt-jpg-cover-open.png` });
  await closeOverlay(page);
  await stable(page, 800);

  /* 其余形态：方图 / 混合 / 全横（split 路径走 MediaGallery 变体）。 */
  for (const key of ["square", "mixed", "landscape"]) {
    check(`vt: ${key} overlay opens`, await openOverlay(page, TITLES[key]));
    await page
      .locator('[data-slot="variant-media-pane"] img, [data-slot="detail-cover"] img')
      .first()
      .waitFor({ state: "visible", timeout: 6000 })
      .catch(() => {});
    await page.waitForTimeout(1100);
    const paneImgs = await page
      .locator('[data-slot="variant-media-pane"] img, [data-slot="detail-cover"] img')
      .evaluateAll((els) => els.map((el) => el.getAttribute("src")));
    const rendered = paneImgs.filter((s) => s && (s.includes("/_next/image") || s.includes("/seed-media"))).length;
    check(`vt: ${key} cover media rendered from the shared source (C1)`, rendered >= 1, `srcs=${paneImgs.length}`);
    await residueCheck(page, `vt ${key}`);
    await page.screenshot({ path: `${OUT}/f1-vt-${key}-open.png` });
    await closeOverlay(page);
    await stable(page, 800);
  }

  /* 超高图：C2 退化居中缩淡——不应有共享元素 VT 起跑（该次 open 的增量计数为 0）。 */
  const vtBeforeTall = await page.evaluate(() => window.__vtCount ?? 0);
  check("vt: tall overlay opens", await openOverlay(page, TITLES.tall));
  await page.waitForTimeout(1300);
  const vtAfterTall = await page.evaluate(() => window.__vtCount ?? 0);
  check("vt: C2 tall degrades to fallback (no shared-element VT)", vtAfterTall === vtBeforeTall, `before=${vtBeforeTall} after=${vtAfterTall}`);
  await residueCheck(page, "vt tall");
  await page.screenshot({ path: `${OUT}/f1-vt-tall-fallback.png` });
  await closeOverlay(page);
  await stable(page, 900);

  /* C3 motion-lock：打开期间触发卡带 data-overlay-motion-lock，关闭后解除。 */
  const lockCard = page.locator(`text=${TITLES.square}`).first();
  await lockCard.click().catch(() => {});
  await page.waitForTimeout(800);
  const lockDuring = await page.evaluate(() => Boolean(document.querySelector("[data-overlay-motion-lock]")));
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
  await page.screenshot({ path: `${OUT}/f1-vt-after-stability.png` });
  await vtCtx.close();

  /* ── 暗色模式（横/竖两张开态截图，观感留档给用户亲验）── */
  console.log("cooling down 45s for the rate-limit window…");
  await new Promise((resolve) => setTimeout(resolve, 45000));
  const darkCtx = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: "dark" });
  const dpage = await darkCtx.newPage();
  await dpage.goto(`${BASE}${PAGE}`, { waitUntil: "domcontentloaded" });
  await stable(dpage, 2600);
  check("dark: portrait overlay opens", await openOverlay(dpage, TITLES.portrait));
  await stable(dpage, 1000);
  await dpage.screenshot({ path: `${OUT}/f1-dark-portrait-open.png` });
  await closeOverlay(dpage);
  await stable(dpage, 700);
  check("dark: jpg overlay opens", await openOverlay(dpage, TITLES.jpg));
  await stable(dpage, 1000);
  await dpage.screenshot({ path: `${OUT}/f1-dark-jpg-open.png` });
  await darkCtx.close();

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
  await rpage.goto(`${BASE}${PAGE}`, { waitUntil: "domcontentloaded" });
  await stable(rpage, 2600);
  check("reduced: overlay opens", await openOverlay(rpage, TITLES.portrait));
  await rpage.waitForTimeout(400);
  const reducedInfo = await rpage.evaluate(() => {
    const shell = document.querySelector("dialog[open] > div");
    if (!shell) return null;
    return {
      inline: shell.style.transition,
      opacity: getComputedStyle(shell).opacity,
    };
  });
  check(
    "reduced: fade path 100ms pure opacity (inline contract)",
    Boolean(reducedInfo && /opacity\s+100ms/.test(reducedInfo.inline) && !/transform|scale/.test(reducedInfo.inline) && parseFloat(reducedInfo.opacity) >= 0.99),
    `inline=${reducedInfo?.inline} opacity=${reducedInfo?.opacity}`,
  );
  check("reduced: fade path (no VT)", (await rpage.evaluate(() => window.__vtCount ?? 0)) === 0);
  await rpage.screenshot({ path: `${OUT}/f1-reduced-portrait-open.png` });
  await rmCtx.close();

  console.log(`\nR3+F1 rig: ${passed} passed, ${failed} failed`);
  process.exitCode = failed > 0 ? 1 : 0;
} catch (error) {
  console.error("RIG ERROR", error);
  process.exitCode = 1;
} finally {
  await browser.close();
}
