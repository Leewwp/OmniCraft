/* T54 浏览器验证（隔离栈 3200/8084）：评论区举报入口
 * 1) 任意评论（非本人）可发起举报（ConfirmModal 填原因）→ 已举报态
 * 2) 刷新后再举报同一条 → 后端 409 → 提示文案 + 已举报态
 * rig 不进 PR；截图 ../screenshots/t54-*.png */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { execSync } from "node:child_process";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T54_BASE_URL || "http://localhost:3200";
const SHOTS = path.join(HERE, "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T54-STEP " + line);
}

async function waitForText(page, text, maxMs = 30000) {
  for (let i = 0; i < Math.ceil(maxMs / 500); i += 1) {
    await page.waitForTimeout(500);
    if ((await page.locator("body").innerText()).includes(text)) return true;
  }
  return (await page.locator("body").innerText()).includes(text);
}

/* 可重跑复位（含此前误触的内容举报记录，避免 ReactionBar 按钮禁用干扰定位） */
execSync(`docker exec lane3-pg psql -U omnicraft -d omnicraft -c "DELETE FROM reports WHERE reporter_id=9101 AND ((target_type='comment' AND target_id=8100) OR (target_type='content' AND target_id=7301));"`);

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 评论区的举报按钮（排除页面上方 ReactionBar 的同名内容举报按钮） */
  const commentReportBtn = () =>
    page.locator("div.space-y-3").getByRole("button", { name: new RegExp(`^${zh.social.report}$`) }).first();

  /* 登录 */
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "lane3-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "Lane3Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });

  /* 1) 首次举报 8100（他人评论；8100 是列表第一条，其举报按钮即评论区第一个） */
  await page.goto(`${BASE}/original/7301`);
  await waitForText(page, "压测评论 #8100", 40000);
  const firstReportBtn = commentReportBtn();
  await firstReportBtn.waitFor({ state: "visible", timeout: 20000 });
  await firstReportBtn.click();
  const dialog = page.getByRole("dialog");
  await dialog.waitFor({ state: "visible", timeout: 8000 });
  await page.screenshot({ path: path.join(SHOTS, "t54-report-dialog.png"), fullPage: false });
  await dialog.getByLabel(zh.social.reportReason).fill("T54 举报验证：垃圾内容");
  await dialog.getByRole("button", { name: new RegExp(`^${zh.social.report}$`) }).click();
  const firstReported = await waitForText(page, zh.social.reported, 15000);
  step("first-report-reported-state", firstReported);

  /* 2) 刷新后再举报同一条 → 409 提示 */
  await page.goto(`${BASE}/original/7301`);
  await waitForText(page, "压测评论 #8100", 40000);
  const againReportBtn = commentReportBtn();
  await againReportBtn.waitFor({ state: "visible", timeout: 20000 });
  await againReportBtn.click();
  const dialog2 = page.getByRole("dialog");
  await dialog2.waitFor({ state: "visible", timeout: 8000 });
  await dialog2.getByLabel(zh.social.reportReason).fill("再次举报同一条");
  await dialog2.getByRole("button", { name: new RegExp(`^${zh.social.report}$`) }).click();
  let conflictNotice = false;
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(400);
    const text = await page.locator("body").innerText();
    if (text.includes("你已举报过该内容")) { conflictNotice = true; break; }
    if (text.includes(zh.social.reported)) break;
  }
  step("repeat-report-409-notice", conflictNotice);
  const reportedAfterConflict = await waitForText(page, zh.social.reported, 8000);
  step("repeat-report-ends-in-reported-state", reportedAfterConflict);
  await page.screenshot({ path: path.join(SHOTS, "t54-after-409.png"), fullPage: false });
} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log(failed.length === 0 ? "T54 ALL PASS" : `T54 FAILED: ${failed.length}`);
process.exit(failed.length === 0 ? 0 : 1);
