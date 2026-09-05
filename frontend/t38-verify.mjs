/* T38 浏览器验证（隔离栈 3203/8087）：verdict 契约 + 理由点赞
 * 1) voter 3853 登录 → 队列投 case 3850（附理由）→ VerdictDetail 出现
 * 2) 理由区显示其他判官昵称（judge_name 契约，非 user#id 兜底）、点赞数为数字（非 NaN）
 * 3) 点赞他人理由 → 计数 2→3；自赞提示路径由 curl 覆盖（409 REASON_SELF_VOTE）
 * rig 不进 PR；截图 ../screenshots/t38-*.png */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T38_BASE_URL || "http://localhost:3203";
const SHOTS = path.join(HERE, "..", "screenshots");

const J = zh.judge;
const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T38-STEP " + line);
}
async function waitForText(page, text, maxMs = 30000) {
  for (let i = 0; i < Math.ceil(maxMs / 400); i += 1) {
    await page.waitForTimeout(400);
    if ((await page.locator("body").innerText()).includes(text)) return true;
  }
  return (await page.locator("body").innerText()).includes(text);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "judge3853@lanej.local");
  await page.fill('input[type="password"]', "Lanej-Judge-Pass9101");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });

  await page.goto(`${BASE}/judge/queue`);
  const approveBtn = page.getByRole("button", { name: J.reviewCard.approve, exact: true });
  await approveBtn.waitFor({ state: "visible", timeout: 40000 });
  const cardText = await page.locator("body").innerText();
  step("queue-shows-case-3850", cardText.includes("#7500"));

  await approveBtn.click();
  await page.getByRole("button", { name: J.reviewCard.confirm, exact: true }).waitFor({ state: "visible", timeout: 8000 });
  await page.locator("textarea").fill("T38 验证：不违反社区规范");
  await page.getByRole("button", { name: J.reviewCard.confirm, exact: true }).click();
  const voted = await waitForText(page, J.voteSuccess, 15000);
  step("vote-success", voted);

  /* VerdictDetail 出现（投票成功内嵌） */
  const verdictReady = await waitForText(page, J.verdict.title, 20000);
  step("verdict-visible", verdictReady);

  const bodyText = await page.locator("body").innerText();
  step("judge-name-shown", bodyText.includes("lanej_t38_judge_a") && bodyText.includes("lanej_t38_judge_b"), "judge_name 契约渲染昵称");
  const reasonA = bodyText.includes("内容确实违规，建议下架");
  const reasonB = bodyText.includes("艺术表达不应下架");
  step("reasons-shown", reasonA && reasonB);
  step("no-nan-counters", !bodyText.includes("NaN"), "点赞计数无 NaN");

  /* judge_a 的理由（vote 26）：赞 2 → 点击 → 3（理由 <p> 的父级即理由卡片块） */
  const reasonBlock = page.locator("p", { hasText: "内容确实违规，建议下架" }).locator("..");
  const likeBtn = reasonBlock.getByRole("button", { name: zh.social.like }).first();
  const before = await likeBtn.innerText();
  await likeBtn.click();
  await page.waitForTimeout(1200);
  const after = await likeBtn.innerText();
  step("like-count-increments", before.trim().startsWith("2") && after.trim().startsWith("3"), `before=${before.trim()} after=${after.trim()}`);
  await page.screenshot({ path: path.join(SHOTS, "t38-verdict.png"), fullPage: true });
} finally {
  await browser.close();
}

const failed = results.filter((r) => r.startsWith("FAIL"));
console.log(`T38-SUMMARY pass=${results.length - failed.length} fail=${failed.length}`);
if (failed.length) {
  console.log("T38-FAILED:\n" + failed.join("\n"));
  process.exit(1);
}
