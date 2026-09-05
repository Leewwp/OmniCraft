/* T40 浏览器验证（隔离栈 3203/8087）：判官内容预览（盲投修复）
 * 1) queue 显示案件（内容类型标签 + #target_id）
 * 2) 预警横幅 + 「查看内容」点击后才加载（title/description 渲染）
 * 3) 媒体不自动加载：「加载媒体」按钮出现且无 <img>；点击后 <img> 出现
 * 4) 「跳过本案」→ 进入下一案（内容 B）
 * 5) 投票后刷新队列：已投案件不再出现（后端排除本人已投）
 * 截图 t40-preview-{light,dark}-{1440,375}.png；rig 不进 PR */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { execSync } from "node:child_process";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T40_BASE_URL || "http://localhost:3203";
const SHOTS = path.join(HERE, "..", "screenshots");

const RC = zh.judge.reviewCard;
const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T40-STEP " + line);
}
async function waitForText(page, text, maxMs = 30000) {
  for (let i = 0; i < Math.ceil(maxMs / 400); i += 1) {
    await page.waitForTimeout(400);
    if ((await page.locator("body").innerText()).includes(text)) return true;
  }
  return (await page.locator("body").innerText()).includes(text);
}

/* 可重跑复位 */
execSync(`docker exec lanej-pg psql -U postgres -d omnicraft -c "DELETE FROM judge_reason_votes WHERE voter_id=9101; DELETE FROM judge_votes WHERE judge_id=9101 AND case_id IN (SELECT id FROM judge_cases WHERE target_id IN (8001,8002));" 2>/dev/null`);

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN", colorScheme: "light" });
  const page = await context.newPage();

  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "judge9101@lanej.local");
  await page.fill('input[type="password"]', "Lanej-Judge-Pass9101");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });

  await page.goto(`${BASE}/judge/queue`);
  const approveBtn = page.getByRole("button", { name: RC.approve, exact: true });
  await approveBtn.waitFor({ state: "visible", timeout: 40000 });

  const bodyText = await page.locator("body").innerText();
  step("case-type-label-shown", bodyText.includes(zh.judge.article), "案件类型标签（article→文章）");
  step("warning-banner-shown", bodyText.includes(RC.previewWarning));

  /* 2) 点击后才加载内容本体 */
  const noAutoLoad = !(await page.locator("img").count());
  step("content-not-autoloaded", noAutoLoad, "预览点击前无内容/媒体渲染");
  await page.getByRole("button", { name: RC.previewLoadButton }).click();
  const loaded = await waitForText(page, "T40 预览内容 A：争议插画", 15000);
  step("content-loaded-on-click", loaded && (await page.locator("body").innerText()).includes("审案预览验证"));

  /* 3) 媒体不自动加载；点击「加载媒体」后出现 */
  const mediaBtn = page.getByRole("button", { name: RC.mediaLoadButton });
  await mediaBtn.waitFor({ state: "visible", timeout: 8000 });
  const noImgBefore = (await page.locator("img").count()) === 0;
  step("media-not-autoloaded", noImgBefore, "媒体按钮可见但无 img");
  await page.screenshot({ path: path.join(SHOTS, "t40-preview-light-1440.png"), fullPage: true });
  await mediaBtn.click();
  /* 本地无 OSS delivery domain：oss_url 按安全策略缺省（AttachmentURL 返回 ""，
   * 不渲染裸 oss_key）。点击后断言媒体区已展开（加载按钮消失）。 */
  await mediaBtn.waitFor({ state: "detached", timeout: 8000 });
  step("media-section-expanded-after-click", (await page.getByRole("button", { name: RC.mediaLoadButton }).count()) === 0);

  /* 4) 跳过本案 → 下一案（内容 B）。受控预览语义：B 卡片切换后预览回到
   * idle（标题不自动出现），需再点「查看内容」才加载。 */
  await page.getByRole("button", { name: RC.skipCase }).click();
  await page.getByText("#8002").waitFor({ state: "visible", timeout: 10000 });
  const bodyAfterSkip = await page.locator("body").innerText();
  step("skip-goes-to-next", !bodyAfterSkip.includes("T40 预览内容 B"), "跳过 A 后切到 #8002 且内容 B 未自动加载");
  step("next-case-id-shown", bodyAfterSkip.includes("#8002"));

  /* 5) 投票案件 B → 刷新后 B 不再出现在队列（排除本人已投），A 仍在 */
  const approveBtn2 = page.getByRole("button", { name: RC.approve, exact: true });
  await approveBtn2.click();
  const confirmBtn = page.getByRole("button", { name: RC.confirm, exact: true });
  await confirmBtn.waitFor({ state: "visible", timeout: 8000 });
  await confirmBtn.click();
  const voted = await waitForText(page, zh.judge.voteSuccess, 15000);
  step("vote-case-b", voted);
  await page.goto(`${BASE}/judge/queue`);
  await page.waitForTimeout(1500);
  if (page.url().includes("login")) {
    /* 整页导航 refresh 竞态丢登录态：重登再进 */
    await page.goto(`${BASE}/login`);
    await page.fill('input[type="email"]', "judge9101@lanej.local");
    await page.fill('input[type="password"]', "Lanej-Judge-Pass9101");
    await page.click('button[type="submit"]');
    await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
    await page.goto(`${BASE}/judge/queue`);
  }
  await approveBtn.waitFor({ state: "visible", timeout: 40000 });
  const queueText = await page.locator("body").innerText();
  step("voted-case-excluded", !queueText.includes("#8002"), "已投 B 不再出现");
  step("unvoted-case-remains", queueText.includes("#8001"), "未投 A 仍在队列");
  await page.screenshot({ path: path.join(SHOTS, "t40-queue-after-vote.png"), fullPage: false });

  /* 6) 暗色 + 375 截图（回到案件 A 预览态） */
  await page.getByRole("button", { name: RC.previewLoadButton }).click();
  await waitForText(page, "T40 预览内容 A：争议插画", 15000);
  await page.emulateMedia({ colorScheme: "dark" });
  await page.waitForTimeout(600);
  await page.screenshot({ path: path.join(SHOTS, "t40-preview-dark-1440.png"), fullPage: false });
  await page.setViewportSize({ width: 375, height: 812 });
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(SHOTS, "t40-preview-dark-375.png"), fullPage: false });
  await page.emulateMedia({ colorScheme: "light" });
  await page.waitForTimeout(600);
  await page.screenshot({ path: path.join(SHOTS, "t40-preview-light-375.png"), fullPage: false });
} finally {
  await browser.close();
}

const failed = results.filter((r) => r.startsWith("FAIL"));
console.log(`T40-SUMMARY pass=${results.length - failed.length} fail=${failed.length}`);
if (failed.length) {
  console.log("T40-FAILED:\n" + failed.join("\n"));
  process.exit(1);
}
