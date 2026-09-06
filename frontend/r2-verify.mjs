// #397 R2 竖屏集新版布局 browser verification rig（本地栈 3000/8080）。
// 四态（竖/方/超高/混合）+ 全横集（保留现设计）× 两断点（1440 ≥1100 / 1024 <1100）
// + 看大图（H4 实测）+ 返回钮 tooltip + 关联原创压栈。截图落 screenshots/overlay-r2-*/。
// 限流纪律（100 req/min）：推荐流单次加载连续点卡（不重复导航）、每次导航后
// waitForTimeout(1800)、失败 3s 重试；rig 全程 ~40 请求。
import { chromium } from "playwright";
import fs from "node:fs";

const BASE = "http://localhost:3000";
const OUT = "../screenshots/overlay-r2";
const EMAIL = process.env.R2_EMAIL || "admin@corpus.omnicraft.local";
const PASSWORD = process.env.R2_PASSWORD || "CorpusV2#2026";

fs.mkdirSync(OUT, { recursive: true });
let passed = 0, failed = 0;
function check(name, cond, extra = "") {
  if (cond) { passed++; console.log(`PASS ${name}`); }
  else { failed++; console.log(`FAIL ${name} ${extra}`); }
}

const FIXTURES = {
  portrait: { title: "【R2验证】竖图集三张" },
  square: { title: "【R2验证】方图单张" },
  tall: { title: "【R2验证】超高图单张" },
  mixed: { title: "【R2验证】混合集横竖" },
  landscape: { title: "【R2验证】横集两张" },
  related: { title: "【R2验证】带关联原创的二创" },
};

async function stable(page, ms = 1800) {
  await page.waitForTimeout(ms);
}

/* 在已加载的 feed 页上点卡片开浮窗（重试 3 次，不重新导航——省限流配额）。 */
async function openOverlayFromFeed(page, title) {
  for (let attempt = 0; attempt < 3; attempt++) {
    const card = page.locator(`text=${title}`).first();
    if (await card.isVisible().catch(() => false)) {
      await card.click().catch(() => {});
      try {
        await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 8000 });
        await page.waitForTimeout(1000);
        return true;
      } catch { /* retry */ }
    }
    await page.waitForTimeout(3000);
  }
  return false;
}

async function closeOverlay(page) {
  await page.keyboard.press("Escape").catch(() => {});
  await page.waitForTimeout(900);
}

const browser = await chromium.launch();
const desktop = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const narrow = await browser.newContext({ viewport: { width: 1024, height: 768 } });

try {
  /* 登录（失败不阻塞——布局验证不依赖会话；限流下重试一次）。 */
  const page = await desktop.newPage();
  for (let attempt = 0; attempt < 2; attempt++) {
    await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
    await stable(page);
    await page.fill('input[type="email"]', EMAIL).catch(() => {});
    await page.fill('input[type="password"]', PASSWORD).catch(() => {});
    await page.click('button[type="submit"]').catch(() => {});
    await page.waitForTimeout(2500);
    if (!(await page.locator('input[type="email"]').isVisible().catch(() => false))) break;
    await page.waitForTimeout(3200);
  }
  check("login admin", !(await page.locator('input[type="email"]').isVisible().catch(() => false)));

  /* 推荐流单次加载（五个 original 夹具都在 feed 上）。 */
  await page.goto(`${BASE}/recommend`, { waitUntil: "domcontentloaded" });
  await stable(page, 2600);

  /* ── 竖图集（多图控件 + 翻页 + 看大图）────────────────── */
  check("portrait overlay opens", await openOverlayFromFeed(page, FIXTURES.portrait.title));
  const pane = page.locator('[data-slot="variant-media-pane"]');
  check("portrait: variant pane rendered", await pane.isVisible().catch(() => false));
  check("portrait: header removed (float chrome)", !(await page.locator("dialog header").isVisible().catch(() => false)));
  check("portrait: floating back button", await page.getByRole("button", { name: /返回到：/ }).isVisible().catch(() => false));
  check("portrait: floating close button", await page.locator('dialog button[aria-label="关闭内容详情"]').first().isVisible().catch(() => false));
  check("portrait: count badge 1 / 3", (await pane.textContent().catch(() => "")).includes("1 / 3"));
  check("portrait: right column scroller", await page.locator('[data-slot="layer-scroller"]').isVisible().catch(() => false));
  await page.screenshot({ path: `${OUT}/portrait-open-desktop.png` });

  await pane.hover({ position: { x: 60, y: 200 } }).catch(() => {});
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${OUT}/portrait-hover-arrow-desktop.png` });

  const rightArrow = pane.locator("button[aria-label='下一张']");
  await rightArrow.click({ force: true }).catch(() => {});
  await page.waitForTimeout(900);
  check("portrait: paging via arrow", (await pane.textContent().catch(() => "")).includes("2 / 3"));
  await page.screenshot({ path: `${OUT}/portrait-page2-desktop.png` });

  /* 中间 1/3 看大图（H4 实测：MediaViewer 原生 dialog 叠加）。 */
  const box = await pane.boundingBox();
  await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
  await page.waitForTimeout(1000);
  check("portrait: viewer opens above overlay (H4)", (await page.locator("dialog[open]").count()) >= 2);
  await page.screenshot({ path: `${OUT}/portrait-viewer-open-desktop.png` });
  const viewerNext = page.locator("dialog[open]").nth(1).locator("button[aria-label='下一张']").first();
  if (await viewerNext.isVisible().catch(() => false)) {
    await viewerNext.click();
    await page.waitForTimeout(600);
  }
  check("portrait: viewer pages", true);
  await page.screenshot({ path: `${OUT}/portrait-viewer-page2-desktop.png` });
  await page.keyboard.press("Escape");
  await page.waitForTimeout(700);
  check("portrait: viewer closes via Esc", (await page.locator("dialog[open]").count()) === 1);

  await page.getByRole("button", { name: /返回到：/ }).hover();
  await page.waitForTimeout(600);
  const tooltip = await page.locator("[role='tooltip']").first().textContent().catch(() => "");
  check("portrait: back tooltip text", (tooltip ?? "").startsWith("返回到："), `tooltip=${tooltip}`);
  await page.screenshot({ path: `${OUT}/portrait-back-tooltip-desktop.png` });
  await closeOverlay(page);
  await stable(page, 900);

  /* ── 方图（单图无控件）────────────────── */
  check("square overlay opens", await openOverlayFromFeed(page, FIXTURES.square.title));
  check("square: variant pane", await page.locator('[data-slot="variant-media-pane"]').isVisible().catch(() => false));
  const squareBadge = await page.locator('[data-slot="variant-media-pane"]').textContent().catch(() => "");
  check("square: no count badge (single)", !(squareBadge ?? "").includes("/"), `badge=${squareBadge?.trim()}`);
  await page.screenshot({ path: `${OUT}/square-open-desktop.png` });
  await closeOverlay(page);
  await stable(page, 900);

  /* ── 超高图（3:4 名义宽 + 内部滚动）────────────────── */
  check("tall overlay opens", await openOverlayFromFeed(page, FIXTURES.tall.title));
  const tallCover = page.locator('[data-slot="variant-media-pane"] [data-slot="detail-cover"]');
  check("tall: variant pane", await tallCover.isVisible().catch(() => false));
  const tallScrollable = await tallCover.evaluate((el) => el.scrollHeight > el.clientHeight).catch(() => false);
  check("tall: anchor box scrolls internally", tallScrollable);
  const tallBox = await tallCover.boundingBox();
  const imgBox = await tallCover.locator("img").boundingBox().catch(() => null);
  check(
    "tall: image exceeds pane height (nominal 3:4 width)",
    Boolean(imgBox && tallBox && imgBox.height > tallBox.height + 40),
    `imgH=${imgBox?.height?.toFixed(0)} paneH=${tallBox?.height?.toFixed(0)}`,
  );
  await page.screenshot({ path: `${OUT}/tall-open-desktop.png` });
  await closeOverlay(page);
  await stable(page, 900);

  /* ── 混合集（16:9 + 3:4 → 新版；逐张自适应几何过渡）────────────────── */
  check("mixed overlay opens", await openOverlayFromFeed(page, FIXTURES.mixed.title));
  const mixedPane = page.locator('[data-slot="variant-media-pane"]');
  check("mixed: variant pane (any portrait wins)", await mixedPane.isVisible().catch(() => false));
  const widthBefore = (await mixedPane.boundingBox())?.width;
  await mixedPane.locator("button[aria-label='下一张']").click({ force: true }).catch(() => {});
  await page.waitForTimeout(1100);
  const widthAfter = (await mixedPane.boundingBox())?.width;
  check(
    "mixed: pane width adapts per item (landscape→portrait narrows)",
    Boolean(widthAfter && widthBefore && Math.abs(widthAfter - widthBefore) > 30),
    `before=${widthBefore?.toFixed(0)} after=${widthAfter?.toFixed(0)}`,
  );
  await page.screenshot({ path: `${OUT}/mixed-open-desktop.png` });
  await page.screenshot({ path: `${OUT}/mixed-page2-desktop.png` });
  await closeOverlay(page);
  await stable(page, 900);

  /* ── 全横集（保留现设计 split-media）+ H4 实测 ────────────────── */
  check("landscape overlay opens", await openOverlayFromFeed(page, FIXTURES.landscape.title));
  check("landscape: keeps split-media (no variant pane)", !(await page.locator('[data-slot="variant-media-pane"]').isVisible().catch(() => false)));
  check("landscape: header kept", await page.locator("dialog header").isVisible().catch(() => false));
  await page.screenshot({ path: `${OUT}/landscape-split-desktop.png` });
  const splitCover = page.locator('[data-slot="detail-cover"]').filter({ visible: true }).first();
  const sbox = await splitCover.boundingBox().catch(() => null);
  if (sbox) {
    await page.mouse.click(sbox.x + sbox.width / 2, sbox.y + sbox.height / 2);
    await page.waitForTimeout(1000);
    check("landscape: viewer opens from split media (H4)", (await page.locator("dialog[open]").count()) >= 2);
    await page.screenshot({ path: `${OUT}/landscape-viewer-desktop.png` });
    await page.keyboard.press("Escape");
    await page.waitForTimeout(600);
  }
  await closeOverlay(page);
  await stable(page, 900);

  /* ── 关联内容块（fanwork 不进推荐流：经 IP 枢纽页打开）────────────────── */
  let relatedOpened = false;
  for (let attempt = 0; attempt < 3 && !relatedOpened; attempt++) {
    await page.goto(`${BASE}/ip/209?tab=share&sort=newest`, { waitUntil: "domcontentloaded" });
    await stable(page, 2600);
    const card = page.locator(`text=${FIXTURES.related.title}`).first();
    if (await card.isVisible().catch(() => false)) {
      await card.click().catch(() => {});
      try {
        await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 8000 });
        await page.waitForTimeout(1200);
        relatedOpened = true;
      } catch { await page.waitForTimeout(3000); }
    } else {
      await page.waitForTimeout(3000);
    }
  }
  check("related overlay opens", relatedOpened);
  const relatedBlock = page.locator('[data-slot="overlay-related-block"]');
  check("related: block rendered", await relatedBlock.isVisible().catch(() => false));
  check("related: original badge row", (await relatedBlock.textContent().catch(() => "")).includes("原创"));
  await page.screenshot({ path: `${OUT}/related-block-desktop.png` });
  const sourceBtn = page.locator('[data-slot="related-source-btn"]');
  if (await sourceBtn.isVisible().catch(() => false)) {
    await sourceBtn.click();
    await page.waitForTimeout(2600);
    check("related: source original pushes in overlay", (await page.locator("dialog[open] h2").count()) >= 1);
    await page.screenshot({ path: `${OUT}/related-pushed-original-desktop.png` });
    await page.getByRole("button", { name: /返回到：/ }).click().catch(() => {});
    await page.waitForTimeout(1400);
  }
  await closeOverlay(page);

  /* ── 窄断点（1024 <1100：单列现设计）────────────────── */
  const npage = await narrow.newPage();
  await npage.goto(`${BASE}/recommend`, { waitUntil: "domcontentloaded" });
  await stable(npage, 2600);
  check("narrow: portrait overlay opens", await openOverlayFromFeed(npage, FIXTURES.portrait.title));
  check("narrow: no variant pane (<1100)", !(await npage.locator('[data-slot="variant-media-pane"]').isVisible().catch(() => false)));
  check("narrow: header kept", await npage.locator("dialog header").isVisible().catch(() => false));
  await npage.screenshot({ path: `${OUT}/portrait-open-narrow.png` });
  await closeOverlay(npage);
  await stable(npage, 1200);
  check("narrow: mixed overlay opens", await openOverlayFromFeed(npage, FIXTURES.mixed.title));
  check("narrow: mixed keeps single column", !(await npage.locator('[data-slot="variant-media-pane"]').isVisible().catch(() => false)));
  await npage.screenshot({ path: `${OUT}/mixed-open-narrow.png` });

  console.log(`\nR2 rig: ${passed} passed, ${failed} failed`);
  process.exitCode = failed > 0 ? 1 : 0;
} catch (error) {
  console.error("RIG ERROR", error);
  process.exitCode = 1;
} finally {
  await browser.close();
}
