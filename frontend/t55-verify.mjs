// T55 浏览器验证 rig（道 B 隔离栈 3202/8086）：
// 通知深链各归位（讨论二跳/report→我的举报/feedback→反馈列表/message→会话 Tab）+
// 下拉点击即标记已读 + 广播角标显示。
// 种子经 docker exec 幂等重置（notifications 每次全删重插）。
import { chromium } from "playwright";
import { execSync } from "node:child_process";

const BASE = "http://localhost:3202";
const shot = (page, name) => page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });
const PSQL = 'docker exec -i laneb-pg psql -U omnicraft -d omnicraft';

const browser = await chromium.launch();
const fails = [];
const expect = (cond, msg) => { if (!cond) fails.push(msg); console.log((cond ? "PASS" : "FAIL") + " — " + msg); };

// 通知种子（reporter = user 1；讨论 1 挂 ip 2）：新→旧
execSync(`${PSQL} -c "DELETE FROM notifications WHERE user_id = 1"`);
execSync(`${PSQL} -c "
INSERT INTO notifications (user_id, channel, type, title, body, target_type, target_id, is_read, created_at)
VALUES
 (1,'system','message','新私信','新私信：你好','message',1,false,NOW() - interval '1 minutes'),
 (1,'system','report_result','举报已处理','已下架该内容','report',3,false,NOW() - interval '2 minutes'),
 (1,'system','system','反馈已处理','Feedback ticket resolved','feedback_ticket',1,false,NOW() - interval '3 minutes'),
 (1,'reply','comment','讨论新回复','T55 讨论新回复','discussion',1,false,NOW() - interval '4 minutes'),
 (1,'broadcast','broadcast','系统广播','T55 广播内容',NULL,NULL,false,NOW() - interval '5 minutes')
"`);

const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();
await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
await page.fill('input[type="email"]', "t53-reporter@example.test");
await page.fill('input[type="password"]', "t53Pass123");
await page.click('button[type="submit"]');
await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 15000 });
await page.goto(`${BASE}/`, { waitUntil: "networkidle" });
await page.waitForTimeout(1000);

async function openDropdown() {
  await page.waitForTimeout(800);
  const bell = page.locator('button[aria-label="通知"]');
  await bell.click({ timeout: 10000 }).catch(async (e) => {
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);
    await bell.click({ timeout: 10000 });
  });
  await page.waitForSelector('[role="dialog"]', { timeout: 8000 });
  await page.waitForTimeout(400);
}

// 1. 下拉：五条 + 广播角标 chip
await openDropdown();
const rows = page.locator('[role="dialog"] button:has(p)');
expect(await rows.count() === 5, `下拉显示 5 条通知（实际 ${await rows.count()}）`);
expect(await page.locator('[role="dialog"]').getByText("系统广播").count() >= 1, "广播行/角标标签可见");
const unreadHighlightedBefore = await page.locator('[role="dialog"] button.bg-accent-subtle').count();
expect(unreadHighlightedBefore === 5, `五条全部未读高亮（实际 ${unreadHighlightedBefore}）`);
await shot(page, "t55-dropdown-with-broadcast");

// 2. 讨论通知二跳：/ip/2?tab=discussions&d=1
await page.locator('[role="dialog"] button', { hasText: "讨论新回复" }).first().click();
await page.waitForURL(/\/ip\/2\?tab=discussions&d=1/, { timeout: 12000 });
expect(page.url().includes("/ip/2?tab=discussions&d=1"), `讨论通知落到 IP 讨论浮层（${page.url()}）`);
await page.waitForTimeout(1200);
await shot(page, "t55-discussion-deeplink");

// 3. report 通知 → /appeals?tab=reports 且「我的举报」tab 激活
await page.goto(`${BASE}/`, { waitUntil: "networkidle" });
await openDropdown();
await page.locator('[role="dialog"] button', { hasText: "已下架该内容" }).first().click();
await page.waitForURL(/\/appeals\?tab=reports/, { timeout: 12000 });
await page.waitForTimeout(1000);
const reportsTabActive = await page.locator('[data-slot="tabs-trigger"]', { hasText: "我的举报" }).getAttribute("data-active");
expect(reportsTabActive !== null, "举报结果通知落到「我的举报」tab");
await shot(page, "t55-report-deeplink");

// 4. feedback 通知 → /feedback/mine
await page.goto(`${BASE}/`, { waitUntil: "networkidle" });
await openDropdown();
await page.locator('[role="dialog"] button', { hasText: "Feedback ticket resolved" }).first().click();
await page.waitForURL(/\/feedback\/mine/, { timeout: 12000 });
expect(page.url().includes("/feedback/mine"), `反馈通知落到我的反馈（${page.url()}）`);
await shot(page, "t55-feedback-deeplink");

// 5. message 通知 → /messages?tab=messages 且会话 tab 激活
await page.goto(`${BASE}/`, { waitUntil: "networkidle" });
await openDropdown();
await page.locator('[role="dialog"] button', { hasText: "新私信：你好" }).first().click();
await page.waitForURL(/\/messages/, { timeout: 12000 });
await page.waitForTimeout(1000);
const msgTabSelected = await page.locator('#messages-tab-messages').getAttribute("aria-selected");
expect(msgTabSelected === "true", "私信通知落到会话 Tab");
expect(await page.locator('text=个对话').count() >= 1, "会话 Tab 计数为真实聚合文案");
await shot(page, "t55-message-deeplink");

// 6. 已读生效：四条点击过的通知不再高亮（只剩广播 1 条未读）
await page.goto(`${BASE}/`, { waitUntil: "networkidle" });
await page.waitForTimeout(600);
await openDropdown();
const unreadHighlightedAfter = await page.locator('[role="dialog"] button.bg-accent-subtle').count();
expect(unreadHighlightedAfter === 1, `点击过的通知已标记已读（剩余未读高亮 ${unreadHighlightedAfter} 条，应只剩广播 1 条）`);
await shot(page, "t55-markread-effect");

await ctx.close();
await browser.close();
console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n` + fails.join("\n"));
process.exit(fails.length === 0 ? 0 : 1);
