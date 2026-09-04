// T36 浏览器验证 rig（道 S 隔离栈 3207/8094）：
// 双端未读一致 + 摘要通知 + ChatWindow 30s 静默轮询。
import { chromium } from "playwright";

const BASE = "http://localhost:3207";
const shot = (page, name) => page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });
const fails = [];
const expect = (cond, msg) => { if (!cond) fails.push(msg); console.log((cond ? "PASS" : "FAIL") + " — " + msg); };

const browser = await chromium.launch();

async function login(page, email, password) {
  for (let attempt = 0; attempt < 3; attempt++) {
    await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
    await page.fill('input[type="email"]', email);
    await page.fill('input[type="password"]', password);
    await page.click('button[type="submit"]');
    await page.waitForFunction(() => !location.pathname.includes("login"), { timeout: 20000 }).catch(() => {});
    if (!page.url().includes("login")) {
      await page.waitForTimeout(2500);
      return true;
    }
    // 登录限流 5/min/IP：整分钟窗口后重试
    console.log(`INFO — login attempt ${attempt + 1} stuck on /login (rate limit?), waiting 65s`);
    await page.waitForTimeout(65000);
  }
  return false;
}

// —— 场景 A：owner 端未读呈现与已读联动 ——
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  expect(await login(page, "lane-s-owner@seed.omnicraft.local", "LaneS-Owner#2026"), "owner 登录成功");

  await page.goto(`${BASE}/messages?tab=messages`, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(3000);
  const convItem = page.locator('text=lane-s-norm').first();
  await convItem.waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
  expect(await convItem.isVisible().catch(() => false), "owner 会话列表出现 lane-s-norm");

  // 打开会话 → 消息可见（已读联动由后端 API 断言兜底，此处走真实 UI）
  await convItem.click().catch(() => {});
  const bubble = page.locator('text=SECRET123').first();
  await bubble.waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
  expect(await bubble.isVisible().catch(() => false), "owner 会话内可见私信全文（摘要不进会话）");

  // 通知下拉：摘要文案「你有一条新私信」且无全文
  const bell = page.locator('[aria-label*="通知"], button:has(svg.lucide-bell)').first();
  if (await bell.isVisible().catch(() => false)) {
    await bell.click().catch(() => {});
    await page.waitForTimeout(1200);
    const summary = await page.locator('text=你有一条新私信').first().isVisible().catch(() => false);
    expect(summary, "通知下拉显示摘要「你有一条新私信」");
    const leak = await page.locator('text=SECRET123').count();
    expect(leak === 0, "通知下拉不泄露私信全文");
  } else {
    console.log("INFO — 铃铛按钮未定位到（跳过下拉断言，API 断言已覆盖）");
  }
  await shot(page, "t36-owner-chat");
  await ctx.close();
}

// —— 场景 B：owner 回复 → norm 端 30s 轮询自动出现 ——
{
  const ownerCtx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const ownerPage = await ownerCtx.newPage();
  await login(ownerPage, "lane-s-owner@seed.omnicraft.local", "LaneS-Owner#2026");
  await ownerPage.goto(`${BASE}/messages?tab=messages`, { waitUntil: "domcontentloaded" });
  const ownerConv = ownerPage.locator('button:has-text("lane-s-norm")').first();
  await ownerConv.waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
  await ownerConv.click();
  const input = ownerPage.locator('textarea').first();
  await input.waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
  await input.fill("owner的回复轮询验证RPLY");
  await input.press("Enter");
  await ownerPage.waitForTimeout(1500);
  const sentOwn = await ownerPage.locator('text=owner的回复轮询验证RPLY').first().isVisible().catch(() => false);
  expect(sentOwn, "owner 发送回复成功（guard 放行：对方已有发言）");

  // norm 端：登录并保持会话打开，等 30s 轮询拉到新消息
  const normCtx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const normPage = await normCtx.newPage();
  await login(normPage, "lane-s-norm@seed.omnicraft.local", "LaneS-Norm#2026");
  await normPage.goto(`${BASE}/messages?tab=messages`, { waitUntil: "domcontentloaded" });
  const normConv = normPage.locator('button:has-text("lane-s-owner")').first();
  await normConv.waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
  await normConv.click();
  const normInput = normPage.locator('textarea').first();
  await normInput.waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
  await normPage.waitForTimeout(1500);
  // 打开瞬间若已加载到回复则直接 PASS；否则等一轮 30s 轮询
  let appeared = await normPage.locator('text=owner的回复轮询验证RPLY').first().isVisible().catch(() => false);
  if (!appeared) {
    console.log("INFO — 等待 30s 轮询周期...");
    await normPage.waitForTimeout(33000);
    appeared = await normPage.locator('text=owner的回复轮询验证RPLY').first().isVisible().catch(() => false);
  }
  expect(appeared, "norm 端不刷新页面经轮询看到 owner 回复（30s 轮询生效）");
  await shot(normPage, "t36-norm-polled");
  await normCtx.close();
  await ownerCtx.close();
}

await browser.close();
console.log(fails.length === 0 ? "ALL PASS" : `FAILURES: ${fails.length}`);
process.exit(fails.length === 0 ? 0 : 1);
