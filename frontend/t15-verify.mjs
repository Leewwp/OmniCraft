/* T15 浏览器验证（无头 Chromium，道 P 隔离栈 3206/8093）：
 * 1) 登录 lanep-lowrep（DB 直插 verified、reputation 1）→ /studio/publish/ip 填表提交
 *    → 断言 toast 命中 INSUFFICIENT_REPUTATION 映射文案「信誉分不足」而非通用失败/裸 code
 * 2) 对照组：登录 lanep-ok（reputation 10）同表单提交 → pending 成功面板（201）
 * rig 不进 PR；截图输出 ../screenshots/t15-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T15_BASE_URL || "http://localhost:3206";
const SHOTS = path.join(HERE, "..", "screenshots");
const LOW_REP_COPY = zh.common.insufficientReputation;
const GENERIC_FAILED = zh.studio.publishIP.failed;
const FORM = zh.studio.publishIP;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T15-STEP " + line);
}

async function login(page, email) {
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', "LaneP#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
  return !page.url().includes("login");
}

async function fillAndSubmitIPForm(page, name) {
  await page.goto(`${BASE}/studio/publish/ip`);
  await page.getByLabel(FORM.nameLabel).fill(name);
  // 分类必选：分类 chips 是 aria-pressed 按钮（无 group role）
  await page.locator('button[aria-pressed]').first().click();
  await page.getByRole("button", { name: FORM.submit }).click();
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 1) 低信誉账号提交被 guard 403 拦截，用户看到信誉分专属文案 */
  step("login-lowrep", await login(page, "lanep-lowrep@seed.omnicraft.local"));
  await fillAndSubmitIPForm(page, "T15 低信誉拦截验证");
  let body = "";
  let hitLowRep = false;
  for (let i = 0; i < 25; i += 1) {
    await page.waitForTimeout(400);
    body = await page.locator("body").innerText();
    if (body.includes(LOW_REP_COPY.slice(0, 6))) { hitLowRep = true; break; }
  }
  step("toast-low-reputation-copy", hitLowRep, LOW_REP_COPY);
  step("toast-not-generic-failed", !body.includes(GENERIC_FAILED));
  step("toast-not-raw-code", !/INSUFFICIENT_REPUTATION/.test(body));
  await page.screenshot({ path: path.join(SHOTS, "t15-lowrep-403-toast.png"), fullPage: false });

  /* 2) 对照组：正常账号同表单提交成功进入 pending 成功面板 */
  const page2 = await context.newPage();
  step("login-ok", await login(page2, "lanep-ok@seed.omnicraft.local"));
  await fillAndSubmitIPForm(page2, "T15 正常账号对照");
  let body2 = "";
  let hitPending = false;
  for (let i = 0; i < 25; i += 1) {
    await page2.waitForTimeout(400);
    body2 = await page2.locator("body").innerText();
    if (body2.includes(FORM.successTitle)) { hitPending = true; break; }
  }
  step("ok-account-pending-panel", hitPending, FORM.successTitle);
  await page2.screenshot({ path: path.join(SHOTS, "t15-ok-account-pending.png"), fullPage: false });

  await browser.close();
  const failed = results.filter((r) => r.startsWith("FAIL"));
  console.log(`T15-SUMMARY ${results.length - failed.length}/${results.length}`);
  process.exit(failed.length ? 1 : 0);
} catch (err) {
  console.error("T15-RIG-ERROR", err);
  await browser.close();
  process.exit(1);
}
