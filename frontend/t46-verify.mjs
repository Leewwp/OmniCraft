/* T46 浏览器验证（隔离栈 3200/8084）：
 * 1) 匿名：加载更多 ×2 → 第 51+ 条评论可达（旧版 50 截断修复）
 * 2) 匿名：展开回复 → 子回复两级展示
 * 3) 登录：回复（parent_id 挂顶层）、编辑本人评论（PATCH 重审）、删除（确认弹窗）
 * 4) 讨论：IP Hub → 讨论详情 → 子回复嵌套展示（后端随顶层带回子回复）
 * rig 不进 PR；截图 ../screenshots/t46-*.png */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T46_BASE_URL || "http://localhost:3200";
const SHOTS = path.join(HERE, "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T46-STEP " + line);
}

async function waitForText(page, text, maxMs = 30000) {
  for (let i = 0; i < Math.ceil(maxMs / 500); i += 1) {
    await page.waitForTimeout(500);
    if ((await page.locator("body").innerText()).includes(text)) return true;
  }
  return (await page.locator("body").innerText()).includes(text);
}

async function loadAllCommentPages(page) {
  for (let round = 0; round < 3; round += 1) {
    const btn = page.getByRole("button", { name: zh.social.loadMoreComments });
    try {
      await btn.waitFor({ state: "visible", timeout: 10000 });
    } catch {
      break; /* 全部加载完或页面仍未渲染完且无更多按钮 */
    }
    const before = (await page.locator("body").innerText()).match(/压测评论 #8\d{3}/g)?.length ?? 0;
    await btn.click();
    for (let i = 0; i < 20; i += 1) {
      await page.waitForTimeout(400);
      const now = (await page.locator("body").innerText()).match(/压测评论 #8\d{3}/g)?.length ?? 0;
      if (now > before) break;
    }
  }
}

const browser = await chromium.launch();
try {
  /* 可重跑复位：恢复 8002、清上轮验证回复（平台坑：服务可被误杀，rig 必须可重跑） */
  const { execSync } = await import("node:child_process");
  execSync(`docker exec lane3-pg psql -U omnicraft -d omnicraft -c "
    UPDATE comments SET status='published', body='第二条：lane3_verify 的评论' WHERE id=8002;
    DELETE FROM comments WHERE body LIKE 'T46 回复验证%';"`);

  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* ---------- 1) 匿名：加载更多到第 51+ 条 ---------- */
  await page.goto(`${BASE}/original/7301`);
  await waitForText(page, "压测评论 #8100", 40000);
  let bodyText = await page.locator("body").innerText();
  const firstPageCount = (bodyText.match(/压测评论 #8\d{3}/g) || []).length;
  step("first-page-20", firstPageCount === 20, `count=${firstPageCount}`);

  await loadAllCommentPages(page);
  bodyText = await page.locator("body").innerText();
  const totalShown = (bodyText.match(/压测评论 #8\d{3}/g) || []).length;
  step("load-more-reaches-53", totalShown === 53, `total=${totalShown}`);
  step("comment-51-visible", bodyText.includes("压测评论 #8150"));
  await page.screenshot({ path: path.join(SHOTS, "t46-loadmore.png"), fullPage: false });

  /* ---------- 2) 匿名：展开回复（懒加载，宿主 8001） ---------- */
  const cardOf = (bodyText_) =>
    page.locator("p", { hasText: bodyText_ }).first().locator("xpath=ancestor::div[contains(@class,'rounded-md')][1]");
  const host8001 = cardOf("第一条：验证昵称与头像加载");
  await host8001.getByRole("button", { name: new RegExp(`^${zh.social.showReplies}$`) }).click();
  let childVisible = false;
  for (let i = 0; i < 15; i += 1) {
    await page.waitForTimeout(400);
    bodyText = await page.locator("body").innerText();
    if (bodyText.includes("子回复一：嵌套展示验证")) { childVisible = true; break; }
  }
  step("lazy-replies-expanded", childVisible);
  await page.screenshot({ path: path.join(SHOTS, "t46-replies-expanded.png"), fullPage: false });

  /* ---------- 3) 登录：回复 / 编辑 / 删除 ---------- */
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "lane3-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "Lane3Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });

  await page.goto(`${BASE}/original/7301`);
  await waitForText(page, "压测评论 #8100", 40000);
  await loadAllCommentPages(page);
  await waitForText(page, "第一条：验证昵称与头像加载", 15000);

  /* 3a) 回复：宿主 8001 卡片内点「回复」（cardOf 已在匿名段定义） */
  const hostCard = cardOf("第一条：验证昵称与头像加载");
  await hostCard.getByRole("button", { name: new RegExp(`^${zh.social.reply}$`) }).click();
  const replyBox = page.getByPlaceholder(`回复 @t45_author`);
  await replyBox.fill("T46 回复验证：两级扁平");
  await page.screenshot({ path: path.join(SHOTS, "t46-reply-box.png"), fullPage: false });
  await hostCard.getByRole("button", { name: new RegExp(`^${zh.social.reply}$`) }).last().click();
  let replySent = false;
  for (let i = 0; i < 15; i += 1) {
    await page.waitForTimeout(400);
    if ((await page.locator("body").innerText()).includes("T46 回复验证：两级扁平")) { replySent = true; break; }
  }
  step("reply-posted-with-parent", replySent);

  /* 3b) 编辑本人评论（8002 = lane3_verify 的「第二条：lane3_verify 的评论」）
   * 编辑态会把正文 <p> 替换成 textarea——按正文找卡片的定位器随之失效，
   * 编辑框用 getByDisplayValue（预填旧正文）精确定位 */
  const ownCard = cardOf("第二条：lane3_verify 的评论");
  await ownCard.getByRole("button", { name: new RegExp(`^${zh.social.edit}$`) }).click();
  /* 编辑框无 placeholder（顶部撰写框有），以此区分 */
  const editor = page.locator("textarea:not([placeholder])");
  await editor.fill("第二条：编辑后的内容（重审通过）");
  await page.getByRole("button", { name: new RegExp(`^${zh.social.saveEdit}$`) }).click();
  let edited = false;
  for (let i = 0; i < 15; i += 1) {
    await page.waitForTimeout(400);
    const text = await page.locator("body").innerText();
    if (text.includes("编辑后的内容（重审通过）") && text.includes("已编辑")) { edited = true; break; }
  }
  step("edit-patch-roundtrip", edited);

  /* 3c) 删除本人评论（确认弹窗） */
  const ownCard2 = cardOf("编辑后的内容（重审通过）");
  await ownCard2.getByRole("button", { name: new RegExp(`^${zh.social.delete}$`) }).click();
  const dialog = page.getByRole("dialog");
  await dialog.waitFor({ state: "visible", timeout: 8000 });
  await page.screenshot({ path: path.join(SHOTS, "t46-delete-confirm.png"), fullPage: false });
  await dialog.getByRole("button", { name: zh.common.confirm }).click();
  let deleted = false;
  for (let i = 0; i < 15; i += 1) {
    await page.waitForTimeout(400);
    if (!(await page.locator("body").innerText()).includes("编辑后的内容（重审通过）")) { deleted = true; break; }
  }
  step("delete-with-confirm", deleted);
  await page.screenshot({ path: path.join(SHOTS, "t46-after-delete.png"), fullPage: false });

  /* ---------- 4) 讨论路径嵌套 ---------- */
  await page.goto(`${BASE}/ip/8501?tab=discussions`);
  let discussionOpened = false;
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(500);
    if ((await page.locator("body").innerText()).includes("T46 讨论嵌套验证")) { discussionOpened = true; break; }
  }
  step("discussion-visible-in-hub", discussionOpened);
  if (discussionOpened) {
    await page.getByText("T46 讨论嵌套验证").first().click();
    let nested = false;
    for (let i = 0; i < 20; i += 1) {
      await page.waitForTimeout(500);
      if ((await page.locator("body").innerText()).includes("讨论子回复（嵌套）")) { nested = true; break; }
    }
    step("discussion-nested-reply-visible", nested);
    await page.screenshot({ path: path.join(SHOTS, "t46-discussion-nested.png"), fullPage: false });
  }
} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log(failed.length === 0 ? "T46 ALL PASS" : `T46 FAILED: ${failed.length}`);
process.exit(failed.length === 0 ? 0 : 1);
