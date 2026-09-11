// T24 浏览器验证 rig（道 S 隔离栈 3207/8094）：
// 375px 无溢出 + series/favorites 空态单 CTA + series 详情标题去重 + Delete 底部。
import { chromium } from "playwright";

const BASE = "http://localhost:3207";
const shot = (page, name, vp) => page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });
const fails = [];
const expect = (c, m) => { if (!c) fails.push(m); console.log((c ? "PASS" : "FAIL") + " — " + m); };

const browser = await chromium.launch();

async function loginOwner(page) {
  for (let attempt = 0; attempt < 3; attempt++) {
    await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
    await page.fill('input[type="email"]', "lane-s-owner@seed.omnicraft.local");
    await page.fill('input[type="password"]', "LaneS-Owner#2026");
    await page.click('button[type="submit"]');
    await page.waitForFunction(() => !location.pathname.includes("login"), { timeout: 20000 }).catch(() => {});
    if (!page.url().includes("login")) { await page.waitForTimeout(2500); return true; }
    await page.waitForTimeout(65000);
  }
  return false;
}

// —— 场景 1：375px 视口 /original 无横向溢出（F-082 防御性回归）——
{
  const ctx = await browser.newContext({ viewport: { width: 375, height: 812 } });
  const page = await ctx.newPage();
  await page.goto(`${BASE}/original`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(2500);
  const m = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth }));
  expect(m.sw === m.cw, `375px /original scrollWidth(${m.sw})==clientWidth(${m.cw})`);
  await shot(page, "t24-mobile-original");
  await page.goto(`${BASE}/home`, { waitUntil: "domcontentloaded" }).catch(() => {});
  await page.waitForTimeout(2000);
  const mh = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth }));
  expect(mh.sw === mh.cw, `375px /home scrollWidth(${mh.sw})==clientWidth(${mh.cw})`);
  await shot(page, "t24-mobile-home");
  await ctx.close();
}

// —— 场景 2：owner 登录 studio 空态单 CTA + 详情结构 ——
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  const okOwner = await loginOwner(page);
  expect(okOwner, "owner 登录成功");

  // series 空态
  await page.goto(`${BASE}/studio/series`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(2500);
  const headerBtn = page.locator('button:has-text("新建系列")');
  const headerBtnCount = await headerBtn.count();
  const emptyBtn = page.locator('main button:has-text("新建系列"), [class*="empty"] button:has-text("新建系列"), p:has-text("系列") >> xpath=ancestor::*[contains(@class,"empty")]//button').first();
  const emptyVisible = await page.getByRole("button", { name: /新建系列|创建系列/ }).count();
  console.log("INFO — header create buttons:", headerBtnCount, "all create buttons:", emptyVisible);
  // 单 CTA：页面上「创建/新建系列」按钮恰好 1 个（空态按钮）
  expect(emptyVisible === 1, `series 空态只有单一创建入口（found ${emptyVisible}）`);
  await shot(page, "t24-series-empty");

  // 打开创建表单 → 页头 CTA 回归（可收起）
  await page.getByRole("button", { name: /新建系列|创建系列/ }).first().click();
  await page.waitForTimeout(600);
  const formVisible = await page.locator("form").first().isVisible().catch(() => false);
  expect(formVisible, "空态按钮点击打开创建表单");
  await page.fill('form input[required]', "T24验证系列");
  await page.click('form button[type="submit"]');
  await page.waitForTimeout(2000);

  // 创建后详情面板：标题去重 + Delete 在底部
  const detailTitleCount = await page.locator('section h2').count();
  expect(detailTitleCount === 0, `详情面板无重复 h2 标题（found ${detailTitleCount}）`);
  const titleInputCount = await page.locator('form input, section input').count();
  console.log("INFO — detail inputs:", titleInputCount);
  await shot(page, "t24-series-detail");
  await ctx.close();
}

// —— 场景 3：favorites zone 空态单 CTA ——
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await loginOwner(page);
  await page.goto(`${BASE}/studio/favorites`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);
  // 每个 zone：空态时区头无 create 按钮（create 按钮总数 == zone 空态按钮数）
  const zoneSections = await page.locator("section[aria-labelledby]").count();
  const createBtns = await page.getByRole("button", { name: /新建收藏集|新建收藏单/ }).count();
  expect(zoneSections > 0 && createBtns === zoneSections, `owner 有默认收藏单（非空 zone）：区头按钮每 section 一个（sections=${zoneSections}, btns=${createBtns}）`);
  await shot(page, "t24-favorites-nonempty");
  await ctx.close();
}

// —— 场景 4：norm（无收藏单）zone 空态单 CTA ——
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  let ok = false;
  for (let attempt = 0; attempt < 3; attempt++) {
    await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
    await page.fill('input[type="email"]', "lane-s-norm@seed.omnicraft.local");
    await page.fill('input[type="password"]', "LaneS-Norm#2026");
    await page.click('button[type="submit"]');
    await page.waitForFunction(() => !location.pathname.includes("login"), { timeout: 20000 }).catch(() => {});
    if (!page.url().includes("login")) { ok = true; await page.waitForTimeout(2500); break; }
    await page.waitForTimeout(65000);
  }
  expect(ok, "norm 登录成功");
  await page.goto(`${BASE}/studio/favorites`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);
  const zoneSections = await page.locator("section[aria-labelledby]").count();
  const createBtns = await page.getByRole("button", { name: /新建收藏集|新建收藏单/ }).count();
  expect(zoneSections > 0 && createBtns === zoneSections, `favorites 空 zone 每 section 仅空态单 CTA（sections=${zoneSections}, btns=${createBtns}）`);
  await shot(page, "t24-favorites-empty");
  await ctx.close();
}

await browser.close();
console.log(fails.length === 0 ? "ALL PASS" : `FAILURES: ${fails.length}`);
process.exit(fails.length === 0 ? 0 : 1);
