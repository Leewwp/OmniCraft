// T44 浏览器验证 rig（道 B 隔离栈 3202/8086）：
// 登录 → /studio/contents 全状态徽标 + 编辑弹层 + 删除 + 去申诉预填。
import { chromium } from "playwright";

const BASE = "http://localhost:3202";
const shot = (page, name) => page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();
const fails = [];
const expect = (cond, msg) => { if (!cond) fails.push(msg); console.log((cond ? "PASS" : "FAIL") + " — " + msg); };

// 1. 登录
await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
await page.fill('input[type="email"]', "t44-author@example.test");
await page.fill('input[type="password"]', "t44Pass123");
await page.click('button[type="submit"]');
await page.waitForURL(/\/(home|studio|feed)/, { timeout: 15000 }).catch(() => {});
console.log("after login url:", page.url());
if (!page.url().includes("login")) {
  // 2. studio 全状态列表
  await page.goto(`${BASE}/studio/contents`, { waitUntil: "networkidle" });
  await page.waitForSelector("text=T44", { timeout: 15000 });
  const rows = await page.locator("a[href^='/content/']").allTextContents();
  expect(rows.length === 5, `五状态内容全部可见（实际 ${rows.length} 行）`);
  expect(await page.locator("text=草稿").count() >= 1, "draft 徽标可见");
  expect(await page.locator("text=审核中").count() >= 1, "pending 徽标可见");
  expect(await page.locator("text=复核中").count() >= 1, "under_review 徽标可见");
  expect(await page.locator("text=已封禁").count() >= 1, "banned 徽标可见");
  expect(await page.locator("text=含违规素材").count() >= 1, "ban_reason 透出");
  await shot(page, "t44-studio-all-statuses");

  // 3. banned 行：去申诉按钮 + 编辑禁用
  const bannedRow = page.locator("div.rounded-lg", { hasText: "T44 已封禁的内容" }).first();
  expect(await bannedRow.locator("text=去申诉").count() === 1, "banned 行有去申诉按钮");
  const bannedEdit = bannedRow.locator("button[title*='编辑'], button[disabled]").first();
  expect(await bannedRow.locator("button[disabled]").count() >= 1, "banned 行编辑按钮禁用");

  // 4. 编辑弹层（published 行）
  const pubRow = page.locator("div.rounded-lg", { hasText: "T44 已发布的内容" }).first();
  await pubRow.locator("button").nth(0).click(); // 第一个操作按钮 = 编辑（去申诉不存在于非 banned 行）
  await page.waitForSelector('[role="dialog"]', { timeout: 5000 });
  expect(await page.locator('[role="dialog"] input#edit-title').inputValue() === "T44 已发布的内容", "编辑弹层预填原标题");
  await shot(page, "t44-edit-dialog");
  await page.fill("#edit-title", "T44 已发布的内容（编辑后）");
  await page.click('button:has-text("保存")');
  await page.waitForSelector("text=内容已更新", { timeout: 10000 }).catch(() => {});
  await page.waitForTimeout(1200);
  expect(await page.locator("text=编辑后").count() >= 1, "编辑后标题出现在列表");

  // 5. 删除（draft 行）
  const draftRow = page.locator("div.rounded-lg", { hasText: "T44 草稿内容" }).first();
  await draftRow.locator("button").last().click(); // 最后一个按钮 = 删除
  await page.waitForSelector('[role="dialog"]', { timeout: 5000 });
  await shot(page, "t44-delete-confirm");
  await page.click('button:has-text("删除")');
  await page.waitForTimeout(1500);
  expect(await page.locator("text=T44 草稿内容").count() === 0, "删除后草稿行消失");

  // 6. 去申诉预填
  const bannedRow2 = page.locator("div.rounded-lg", { hasText: "已封禁" }).first();
  await bannedRow2.locator("a[href*='/appeals']").click();
  await page.waitForURL(/\/appeals\?target_type=content&target_id=\d+/, { timeout: 10000 });
  await page.waitForSelector("form, textarea, input[type='number']", { timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(800);
  const prefillId = await page.locator("input[type='number']").inputValue().catch(() => "");
  expect(page.url().includes("target_type=content"), "申诉页带 target_type 预填");
  expect(prefillId !== "" && prefillId !== null, `target_id 已预填（值=${prefillId}）`);
  await shot(page, "t44-appeals-prefilled");
} else {
  fails.push("登录未跳转（停留在 " + page.url() + "）");
}

await browser.close();
console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n` + fails.join("\n"));
process.exit(fails.length === 0 ? 0 : 1);
