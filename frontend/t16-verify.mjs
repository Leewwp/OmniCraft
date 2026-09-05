/* T16 浏览器验证（无头 Chromium，道 P 隔离栈 3206/8093）：
 * 1) lanep-creator 登录 → /studio/publish/ip 提交新 IP（pending）
 * 2) lanep-ok（admin）登录 → /admin/ips → 对该 IP 点「驳回」→ ConfirmModal 填原因 → 确认
 * 3) 断言：表格中该 IP 消失（驳回成功）；DB 断言 ip_review_logs + 创建者 ip_status 通知由后端 rig 内 execSync psql 完成
 * rig 不进 PR；截图输出 ../screenshots/t16-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import { execSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T16_BASE_URL || "http://localhost:3206";
const SHOTS = path.join(HERE, "..", "screenshots");
const FORM = zh.studio.publishIP;
const REJECT_REASON = "RIG 驳回原因验证：介绍与题材不符";

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T16-STEP " + line);
}

async function login(page, email) {
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', "LaneP#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
  return !page.url().includes("login");
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();
  const stamp = Date.now().toString(36);
  const ipName = `T16 界面驳回 ${stamp}`;

  /* 1) 创建者提交新 IP（pending） */
  step("login-creator", await login(page, "lanep-creator@seed.omnicraft.local"));
  await page.goto(`${BASE}/studio/publish/ip`);
  await page.getByLabel(FORM.nameLabel).fill(ipName);
  await page.locator('button[aria-pressed]').first().click();
  await page.getByRole("button", { name: FORM.submit }).click();
  let created = false;
  for (let i = 0; i < 25; i += 1) {
    await page.waitForTimeout(400);
    if ((await page.locator("body").innerText()).includes(FORM.successTitle)) { created = true; break; }
  }
  step("creator-ip-pending", created, FORM.successTitle);

  /* 2) admin 驳回：ConfirmModal 填原因 */
  const page2 = await context.newPage();
  step("login-admin", await login(page2, "lanep-ok@seed.omnicraft.local"));
  await page2.goto(`${BASE}/admin/ips`);
  const rejectBtn = page2.getByRole("button", { name: new RegExp(`${zh.admin.ips.reject}: ${ipName}`) });
  await rejectBtn.waitFor({ state: "visible", timeout: 20000 });
  await rejectBtn.click();
  await page2.screenshot({ path: path.join(SHOTS, "t16-admin-reject-modal.png"), fullPage: false });

  const reasonBox = page2.getByLabel(zh.admin.ips.rejectReason);
  await reasonBox.waitFor({ state: "visible", timeout: 10000 });
  await reasonBox.fill(REJECT_REASON);
  await page2.getByRole("button", { name: zh.admin.ips.confirmReject }).click();

  /* 3) 驳回成功 → 行从待审表格消失 */
  let gone = false;
  for (let i = 0; i < 25; i += 1) {
    await page2.waitForTimeout(400);
    if (!(await page2.locator("body").innerText()).includes(ipName)) { gone = true; break; }
  }
  step("admin-reject-row-gone", gone);
  await page2.screenshot({ path: path.join(SHOTS, "t16-admin-after-reject.png"), fullPage: false });

  /* 4) DB 断言：ip_review_logs 一行 + 创建者通知带原因（worker 消费后） */
  const logs = execSync(
    `docker exec lanep-pg psql -U postgres -d omnicraft -tAc "SELECT count(*) FROM ip_review_logs WHERE reason = '${REJECT_REASON}'"`,
  ).toString().trim();
  step("db-ip-review-logs-row", logs === "1", `count=${logs}`);

  let notifCount = "0";
  for (let i = 0; i < 15; i += 1) {
    notifCount = execSync(
      `docker exec lanep-pg psql -U postgres -d omnicraft -tAc "SELECT count(*) FROM notifications WHERE type='ip_status' AND body LIKE '%${REJECT_REASON}%'"`,
    ).toString().trim();
    if (notifCount === "1") break;
    await new Promise((r) => setTimeout(r, 600));
  }
  step("db-creator-notified-with-reason", notifCount === "1", `count=${notifCount}`);

  await browser.close();
  const failed = results.filter((r) => r.startsWith("FAIL"));
  console.log(`T16-SUMMARY ${results.length - failed.length}/${results.length}`);
  process.exit(failed.length ? 1 : 0);
} catch (err) {
  console.error("T16-RIG-ERROR", err);
  await browser.close();
  process.exit(1);
}
