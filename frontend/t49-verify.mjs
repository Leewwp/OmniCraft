/* T49 浏览器验证（无头 Chromium，道 P 隔离栈 3206/8093）：
 * 种子：内容 id=1（作者 lanep-ok，published）+ 已有 open PR（T48 旅程遗留）。
 * 1) lanep-ok 登录 /studio/overview → 待办卡出现真实条目（open PR 的 message）
 * 2) lanep-creator 在 /content/1 对标签提建议（+）
 * 3) lanep-ok 刷新 /studio/overview → 待办卡出现 tag 条目；DB 断言 tag_suggestion 通知落库
 * rig 不进 PR；截图输出 ../screenshots/t49-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import { execSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T49_BASE_URL || "http://localhost:3206";
const SHOTS = path.join(HERE, "..", "screenshots");
const OVERVIEW_TITLE = zh.studio.overview.pendingTasks;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T49-STEP " + line);
}

async function login(page, email) {
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', "LaneP#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
  return !page.url().includes("login");
}

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

  /* 1) 作者 overview：待办卡出现 open PR 真实条目 */
  step("login-author", await login(page, "lanep-ok@seed.omnicraft.local"));
  await gotoProtected(page, `${BASE}/studio/overview`, "lanep-ok@seed.omnicraft.local", OVERVIEW_TITLE);
  let body = "";
  let prTaskSeen = false;
  for (let i = 0; i < 20; i += 1) {
    body = await page.locator("body").innerText();
    if (body.includes("修正第二段的错别字并补充结尾")) { prTaskSeen = true; break; }
    await page.waitForTimeout(600);
  }
  step("pending-card-shows-open-pr", prTaskSeen, "open PR message");
  await page.screenshot({ path: path.join(SHOTS, "t49-overview-pr-task.png"), fullPage: false });

  /* 2) 贡献者对内容标签提交建议（hover tag → + 按钮） */
  const page2 = await context.newPage();
  step("login-contributor", await login(page2, "lanep-creator@seed.omnicraft.local"));
  await page2.goto(`${BASE}/content/1`);
  // +/- 按钮随标签 hover 出现（group-hover:flex），先 hover 再点击
  const tagBadge = page2.locator("span.group", { hasText: "奇幻" }).first();
  await tagBadge.waitFor({ state: "visible", timeout: 20000 });
  await tagBadge.hover();
  const addLabel = zh.content.suggestAddTag.replace("{tag}", "奇幻");
  const addBtn = page2.getByRole("button", { name: addLabel }).first();
  await addBtn.waitFor({ state: "visible", timeout: 10000 });
  await addBtn.click();
  let suggested = false;
  for (let i = 0; i < 20; i += 1) {
    if ((await page2.locator("body").innerText()).includes(zh.content.tagSuggestionSubmitted)) { suggested = true; break; }
    await page2.waitForTimeout(400);
  }
  step("tag-suggestion-submitted", suggested, zh.content.tagSuggestionSubmitted);

  /* 3) 作者 overview 待办卡出现 tag 条目 + DB 通知断言
   * （client-side 导航离开再回来触发重挂载拉新数据，避免二次整页跳转的
   * auth 竞态与登录限流；该 auth 问题已另行登记，不属本票） */
  await page.getByRole("link", { name: zh.studio.sidebar.myContent }).first().click();
  await page.waitForURL((url) => url.pathname.includes("/studio/contents"), { timeout: 15000 });
  await page.getByRole("link", { name: zh.studio.sidebar.overview }).first().click();
  let tagTaskSeen = false;
  for (let i = 0; i < 25; i += 1) {
    body = await page.locator("body").innerText();
    if (body.includes("标签添加：奇幻") || body.includes("添加：奇幻")) { tagTaskSeen = true; break; }
    await page.waitForTimeout(600);
  }
  step("pending-card-shows-tag-suggestion", tagTaskSeen, "标签添加：奇幻");
  await page.screenshot({ path: path.join(SHOTS, "t49-overview-tag-task.png"), fullPage: false });

  let notifCount = "0";
  for (let i = 0; i < 15; i += 1) {
    notifCount = execSync(
      `docker exec lanep-pg psql -U postgres -d omnicraft -tAc "SELECT count(*) FROM notifications WHERE type='tag_suggestion' AND body LIKE '%奇幻%'"`,
    ).toString().trim();
    if (notifCount !== "0") break;
    await new Promise((r) => setTimeout(r, 600));
  }
  step("author-notified-tag-suggestion", notifCount !== "0", `count=${notifCount}`);

  await browser.close();
  const failed = results.filter((r) => r.startsWith("FAIL"));
  console.log(`T49-SUMMARY ${results.length - failed.length}/${results.length}`);
  process.exit(failed.length ? 1 : 0);
} catch (err) {
  console.error("T49-RIG-ERROR", err);
  await browser.close();
  process.exit(1);
}
