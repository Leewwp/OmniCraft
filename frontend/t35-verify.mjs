/* T35 浏览器验证（无头 Chromium，隔离栈 3100/8083）。
 * 验收：任意用户主页发起对话成功（首条 201）+ guard 连续第二条被拦（不得放宽）。
 *
 * 会话坑位（dev 模式既有缺陷，非本票范围）：React StrictMode 并发 refresh 触发
 * 后端重放防护 → full-load 跨页会话 401 死锁。因此各验证段用 login?redirect=
 * 落地（router.push 为 SPA 导航，内存 token 存活），段间重新登录。
 *
 * 截图输出 ../screenshots/t35-*.png；rig 不进 PR（a06/a07 同惯例）。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const BASE = process.env.T35_BASE_URL || "http://localhost:3100";
const SHOTS = path.join(process.cwd(), "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T35-STEP " + line);
}

async function login(page, redirect) {
  await page.goto(`${BASE}/login?redirect=${encodeURIComponent(redirect)}`);
  await page.fill('input[type="email"]', "a07-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "A07Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("/login"), { timeout: 20000 });
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });

  /* ── 段 0：匿名入口 → /login ── */
  const anon = await context.newPage();
  await anon.goto(`${BASE}/user/2`);
  const entrySel = { name: zh.user.sendMessage };
  await anon.getByRole("button", entrySel).waitFor({ state: "visible", timeout: 15000 });
  await anon.getByRole("button", entrySel).click();
  await anon.waitForURL(/\/login/, { timeout: 10000 });
  step("anonymous-redirects-login", anon.url().includes("/login"), anon.url());
  await anon.close();

  /* ── 段 1：自己主页不显示入口（会话恢复后断言） ── */
  const page = await context.newPage();
  await login(page, "/user/1");
  await page
    .getByRole("button", { name: zh.user.editProfile })
    .waitFor({ state: "visible", timeout: 15000 });
  const ownCount = await page.getByRole("button", entrySel).count();
  step("own-profile-hidden", ownCount === 0, `edit=shown dm=${ownCount}`);

  /* ── 段 2：他人主页入口 + 弹窗 + 首条 201 + 消息中心会话 ── */
  await login(page, "/user/2");
  const entry = page.getByRole("button", entrySel);
  await entry.waitFor({ state: "visible", timeout: 15000 });
  const followVisible = await page
    .getByRole("button", { name: new RegExp(`^(${zh.user.follow}|${zh.user.following})$`) })
    .isVisible()
    .catch(() => false);
  step("entry-visible-next-to-follow", followVisible, `follow=${followVisible}`);
  await page.screenshot({ path: path.join(SHOTS, "t35-profile-entry.png"), fullPage: false });

  await entry.click();
  const dialog = page.getByRole("dialog");
  await dialog.waitFor({ state: "visible", timeout: 8000 });
  const textarea = dialog.locator("textarea");
  await textarea.waitFor({ state: "visible" });
  const focused = await page.evaluate(() => document.activeElement?.tagName);
  step("dialog-open-focused", focused === "TEXTAREA", `activeElement=${focused}`);
  await page.screenshot({ path: path.join(SHOTS, "t35-compose-dialog.png") });

  const firstResponse = page.waitForResponse(
    (r) => r.url().includes("/api/v1/messages") && r.request().method() === "POST",
    { timeout: 20000 },
  );
  await textarea.fill("你好！很喜欢你的空岛物语系列，想请教配色思路。");
  await dialog.getByRole("button", { name: zh.messages.chat.send }).click();
  const res = await firstResponse;
  step("first-dm-201", res.status() === 201, `HTTP ${res.status()}`);
  await page.waitForURL(/\/messages$/, { timeout: 15000 });
  step("routed-to-messages", page.url().endsWith("/messages"), page.url());

  await page.getByRole("tab", { name: zh.messages.tabs.conversations }).click();
  const convVisible = await page.getByText("creator").first().isVisible().catch(() => false);
  step("conversation-created", convVisible, "creator conversation listed");
  await page.screenshot({ path: path.join(SHOTS, "t35-messages-conversation.png"), fullPage: false });

  /* ── 段 3：guard 连续第二条被拦（重新登录新会话，方向内已有首条） ── */
  await login(page, "/user/2");
  await entry.waitFor({ state: "visible", timeout: 15000 });
  await entry.click();
  const dialog2 = page.getByRole("dialog");
  await dialog2.waitFor({ state: "visible", timeout: 8000 });
  const guardResponse = page.waitForResponse(
    (r) => r.url().includes("/api/v1/messages") && r.request().method() === "POST",
    { timeout: 20000 },
  );
  await dialog2.locator("textarea").fill("再问一次！");
  await dialog2.getByRole("button", { name: zh.messages.chat.send }).click();
  const guardRes = await guardResponse;
  step("second-dm-blocked-403", guardRes.status() === 403, `HTTP ${guardRes.status()}`);
  const notice = dialog2.getByRole("alert");
  await notice.waitFor({ state: "visible", timeout: 8000 });
  const noticeText = await notice.innerText();
  step("guard-notice-localized", noticeText.includes("尚未回复"), noticeText);
  step("dialog-kept-open-on-guard", await dialog2.isVisible());
  await page.screenshot({ path: path.join(SHOTS, "t35-guard-second-dm.png"), fullPage: false });
} finally {
  await browser.close();
  const failed = results.filter((line) => line.startsWith("FAIL"));
  console.log(`T35-SUMMARY pass=${results.length - failed.length} fail=${failed.length}`);
  if (failed.length) process.exitCode = 1;
}
