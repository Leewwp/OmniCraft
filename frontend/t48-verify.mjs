/* T48 浏览器验证（无头 Chromium，道 P 隔离栈 3206/8093）：
 * 种子：内容 id=1（fanwork allow_copy published，作者 lanep-ok）+ content_version v1。
 * 1) lanep-creator 登录 → /content/1 → 点「提交 PR」→ 跳 /studio/pr-requests?content_id=1&create=1
 * 2) 创建面板渲染（v1 预选）→ 填 message/new_text → 提交成功
 * 3) lanep-ok（作者）登录 /studio/pr-requests → PR 列表出现 → 选中 → diff 可见
 * rig 不进 PR；截图输出 ../screenshots/t48-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T48_BASE_URL || "http://localhost:3206";
const SHOTS = path.join(HERE, "..", "screenshots");
const CREATE = zh.studio.pr.create;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T48-STEP " + line);
}

async function login(page, email) {
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', "LaneP#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
  return !page.url().includes("login");
}

// auth 竞态自愈的受保护页导航（lane3 已登记的平台问题，非本票范围）
async function gotoProtected(page, url, email, readyText) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    await page.goto(url, { waitUntil: "domcontentloaded" });
    for (let i = 0; i < 16; i += 1) {
      if (page.url().includes("/login")) break;
      if ((await page.getByText(readyText, { exact: false }).first().count()) > 0) return true;
      await page.waitForTimeout(500);
    }
    await login(page, email);
  }
  return false;
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 1) 内容详情页 → PR 入口 */
  step("login-contributor", await login(page, "lanep-creator@seed.omnicraft.local"));
  await page.goto(`${BASE}/content/1`);
  const entry = page.getByRole("link", { name: zh.pr.submit }).first();
  await entry.waitFor({ state: "visible", timeout: 20000 });
  step("content-page-pr-entry-visible", true);
  await page.screenshot({ path: path.join(SHOTS, "t48-content-entry.png"), fullPage: false });
  await entry.click();
  await page.waitForURL((url) => url.pathname.includes("/studio/pr-requests") && url.search.includes("content_id=1") && url.search.includes("create=1"), { timeout: 15000 });
  step("entry-deep-link-params", true, page.url());

  /* 2) 创建面板：版本预选 + 填写 + 提交 */
  await page.getByRole("heading", { name: CREATE.title }).waitFor({ state: "visible", timeout: 20000 });
  const baseSelect = page.locator("#pr-base-version");
  await baseSelect.waitFor({ state: "visible", timeout: 15000 });
  const preselected = await baseSelect.inputValue();
  step("base-version-preselected", preselected === "1", `base_version_id=${preselected}`);
  await page.locator("#pr-message").fill("修正第二段的错别字并补充结尾");
  await page.locator("#pr-new-text").fill("这是 T48 旅程修改后的完整新文本：故事有了新结尾。");
  await page.screenshot({ path: path.join(SHOTS, "t48-create-panel.png"), fullPage: false });
  await page.getByRole("button", { name: CREATE.submit }).click();
  let submitted = false;
  for (let i = 0; i < 25; i += 1) {
    if ((await page.locator("body").innerText()).includes(CREATE.success)) { submitted = true; break; }
    await page.waitForTimeout(400);
  }
  step("pr-submit-success", submitted, CREATE.success);
  await page.screenshot({ path: path.join(SHOTS, "t48-submit-success.png"), fullPage: false });

  /* 3) 作者侧：列表出现 + diff 可见 */
  const page2 = await context.newPage();
  step("login-author", await login(page2, "lanep-ok@seed.omnicraft.local"));
  await gotoProtected(page2, `${BASE}/studio/pr-requests`, "lanep-ok@seed.omnicraft.local", zh.dashboard.pr.title);
  // PRCard 展示 submitter_id（接口不展开用户名），用提交的 message 定位
  let listed = false;
  for (let i = 0; i < 20; i += 1) {
    const b = await page2.locator("body").innerText();
    if (b.includes("修正第二段的错别字并补充结尾")) { listed = true; break; }
    await page2.waitForTimeout(500);
  }
  step("author-lists-new-pr", listed);
  await page2.screenshot({ path: path.join(SHOTS, "t48-author-list.png"), fullPage: false });

  await page2.getByRole("button", { name: zh.pr.viewDiff }).first().click();
  await page2.waitForTimeout(2000);
  const diffSeen = (await page2.locator("body").innerText()).includes(zh.pr.originalVersion);
  step("author-diff-visible", diffSeen, zh.pr.originalVersion);
  await page2.screenshot({ path: path.join(SHOTS, "t48-author-diff.png"), fullPage: false });

  await browser.close();
  const failed = results.filter((r) => r.startsWith("FAIL"));
  console.log(`T48-SUMMARY ${results.length - failed.length}/${results.length}`);
  process.exit(failed.length ? 1 : 0);
} catch (err) {
  console.error("T48-RIG-ERROR", err);
  await browser.close();
  process.exit(1);
}
