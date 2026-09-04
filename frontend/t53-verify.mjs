// T53 浏览器验证 rig（道 B 隔离栈 3202/8086 + worker 6384）：
// 举报者「我的举报」可见处理状态/处置说明 → admin 维持举报 → worker 投递通知 →
// 举报者消息中心收到通知 → 我的举报状态翻转为已处理。
import { chromium } from "playwright";

const BASE = "http://localhost:3202";
const shot = (page, name) => page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });

const browser = await chromium.launch();
const fails = [];
const expect = (cond, msg) => { if (!cond) fails.push(msg); console.log((cond ? "PASS" : "FAIL") + " — " + msg); };

async function login(page, email, password) {
  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 15000 });
}

// 1. 举报者视角：我的举报 tab（pending + resolved 各一条，resolved 带处置说明）
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await login(page, "t53-reporter@example.test", "t53Pass123");
  await page.goto(`${BASE}/appeals`, { waitUntil: "networkidle" });
  await page.click('[role="tab"]:has-text("我的举报")');
  await page.waitForSelector("text=content #2", { timeout: 10000 });
  const rows = await page.locator("div.rounded-md", { hasText: "content #" }).all();
  expect(rows.length === 2, `两条举报可见（实际 ${rows.length} 行）`);
  const pendingRow = page.locator("div.rounded-md", { hasText: "content #2" }).first();
  expect(await pendingRow.locator("text=待处理").count() === 1, "pending 举报显示「待处理」");
  const resolvedRow = page.locator("div.rounded-md", { hasText: "content #3" }).first();
  expect(await resolvedRow.locator("text=已处理").count() === 1, "resolved 举报显示「已处理」");
  expect(await resolvedRow.locator("text=已删除该违规内容").count() === 1, "处置说明透出");
  await shot(page, "t53-my-reports");
  await ctx.close();
}

// 2. admin 视角：维持举报（带处理说明）——触发后端通知 + worker 投递
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await login(page, "t53-admin@example.test", "t53Admin123");
  await page.goto(`${BASE}/admin/reports?status=pending`, { waitUntil: "networkidle" });
  await page.click("text=spam");
  await page.waitForSelector('button:has-text("维持举报")', { timeout: 10000 });
  await shot(page, "t53-admin-detail");
  await page.click('button:has-text("维持举报")');
  await page.waitForSelector('[role="dialog"] textarea', { timeout: 5000 });
  await page.fill('[role="dialog"] textarea', "已下架该内容");
  await shot(page, "t53-admin-confirm");
  await page.click('[role="dialog"] button:has-text("确认维持")');
  // 等副作用生效：详情视图退回列表（PATCH 完成）
  await page.waitForSelector('h1:has-text("举报处理")', { timeout: 10000 });
  await ctx.close();
}

// 3. 举报者收到处理通知（worker 消费 notification.create 落库）
{
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await login(page, "t53-reporter@example.test", "t53Pass123");
  await page.goto(`${BASE}/messages`, { waitUntil: "networkidle" });
  let notified = false;
  for (let i = 0; i < 10 && !notified; i++) {
    await page.waitForTimeout(2000);
    await page.reload({ waitUntil: "networkidle" });
    notified = (await page.locator("text=举报已处理").count()) >= 1;
  }
  expect(notified, "举报者消息中心收到「举报已处理」通知");
  expect(await page.locator("text=已下架该内容").count() >= 1, "通知 body 带处置说明");
  await shot(page, "t53-reporter-notification");

  // 4. 我的举报状态翻转
  await page.goto(`${BASE}/appeals`, { waitUntil: "networkidle" });
  await page.click('[role="tab"]:has-text("我的举报")');
  await page.waitForSelector("text=content #2", { timeout: 10000 });
  const row = page.locator("div.rounded-md", { hasText: "content #2" }).first();
  expect(await row.locator("text=已处理").count() === 1, "处置后我的举报状态翻转为「已处理」");
  expect(await row.locator("text=已下架该内容").count() === 1, "处置后我的举报显示新处置说明");
  await shot(page, "t53-my-reports-after");
  await ctx.close();
}

await browser.close();
console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n` + fails.join("\n"));
process.exit(fails.length === 0 ? 0 : 1);
