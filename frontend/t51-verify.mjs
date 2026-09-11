/* T51 浏览器验证（无头 Chromium，道 P 隔离栈 3206/8093）：
 * 1) creator 登录 → /contents/4（fanwork，真实发布链路建的 v1）：版本历史模块渲染
 *    v1 条目 + 最新徽标；点预览 → 弹层正文 = description 全文
 * 2) /contents/5（存量模拟：发布于接线前，无版本行）：空态文案说明版本来源与不回填
 * 种子：content 4 = API 真实发布（source_original_id=2）+ DB 置 published；
 *       content 5 = DB 直插 published 无版本。
 * rig 不进 PR；截图输出 ../screenshots/t51-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T51_BASE_URL || "http://localhost:3206";
const SHOTS = path.join(HERE, "..", "screenshots");
const EMPTY_COPY = zh.content.noVersionHistory;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T51-STEP " + line);
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

  step("login-creator", await login(page, "lanep-creator@seed.omnicraft.local"));

  /* 1) v1 场景：新发布 fanwork 详情页版本历史 */
  await page.goto(`${BASE}/content/4`, { waitUntil: "domcontentloaded" });
  await page.getByText(zh.content.versionHistory).first().waitFor({ state: "visible", timeout: 20000 });
  const v1Visible = (await page.getByText("v1", { exact: true }).count()) > 0;
  const latestBadge = (await page.getByText(zh.content.latest).count()) > 0;
  step("v1-entry-rendered", v1Visible && latestBadge, `v1=${v1Visible} latest=${latestBadge}`);
  await page.screenshot({ path: path.join(SHOTS, "t51-v1-entry.png"), fullPage: true });

  /* 2) v1 预览弹层：正文 = description 全文 */
  await page.getByRole("button", { name: zh.content.preview }).first().click();
  const previewText = await page.locator("pre").first().innerText();
  step(
    "v1-preview-full-description",
    previewText.includes("T51 fanwork body: initial version v1 visible on detail page."),
    previewText.slice(0, 60),
  );
  await page.screenshot({ path: path.join(SHOTS, "t51-v1-preview.png") });
  await page.keyboard.press("Escape");

  /* 3) 存量空态：无版本 fanwork 显示来源说明文案 */
  await page.goto(`${BASE}/content/5`, { waitUntil: "domcontentloaded" });
  await page.getByText(zh.content.versionHistory).first().waitFor({ state: "visible", timeout: 20000 });
  const emptyEl = page.getByText(EMPTY_COPY.slice(0, 12), { exact: false }).first();
  await emptyEl.waitFor({ state: "visible", timeout: 10000 });
  const emptyText = await emptyEl.innerText();
  const explainsSource = emptyText.includes("发布") && emptyText.includes("PR 合并");
  const explainsLegacy = emptyText.includes("版本功能上线前");
  step("legacy-empty-copy", explainsSource && explainsLegacy, emptyText.slice(0, 48));
  await page.screenshot({ path: path.join(SHOTS, "t51-legacy-empty.png"), fullPage: true });
} finally {
  await browser.close();
}

const failed = results.filter((r) => r.startsWith("FAIL"));
console.log(`T51-RIG ${results.length - failed.length}/${results.length} passed`);
process.exit(failed.length > 0 ? 1 : 0);
