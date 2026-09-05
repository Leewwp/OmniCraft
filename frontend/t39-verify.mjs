/* T39 浏览器验证（隔离栈 3203/8087）：verdict 结案横幅（closed_ 前缀词表）
 * 1) voter 3853 登录 → 队列显示 case 3960（预两票 approve，min_votes=3）
 * 2) 投「不违规」→ 第三票触发闭案 closed_approve
 * 3) VerdictDetail 渲染结案横幅「判决结果：不违规」（旧词表 === "closed" 恒假永不显示）
 * rig 不进 PR；截图 ../screenshots/t39-*.png；奖励落库断言由外部 DB 查询完成 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T39_BASE_URL || "http://localhost:3203";
const SHOTS = path.join(HERE, "..", "screenshots");

const J = zh.judge;
const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T39-STEP " + line);
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
  step("queue-shows-case-3960", (await page.locator("body").innerText()).includes("#7600"));

  await approveBtn.click();
  await page.getByRole("button", { name: J.reviewCard.confirm, exact: true }).waitFor({ state: "visible", timeout: 8000 });
  await page.getByRole("button", { name: J.reviewCard.confirm, exact: true }).click();
  const voted = await waitForText(page, J.voteSuccess, 15000);
  step("third-vote-accepted", voted);

  const verdictReady = await waitForText(page, J.verdict.title, 20000);
  step("verdict-visible", verdictReady);

  /* 结案横幅：closed_approve + 3/3 不违规 → 「判决结果：不违规」（verdict.result 文案） */
  const bannerText = J.verdict.result.replace("{result}", J.reviewCard.approve);
  const banner = await waitForText(page, bannerText, 10000);
  step("closed-banner-rendered", banner, `期望横幅「${bannerText}」`);
  await page.screenshot({ path: path.join(SHOTS, "t39-verdict-closed.png"), fullPage: true });
} finally {
  await browser.close();
}

const failed = results.filter((r) => r.startsWith("FAIL"));
console.log(`T39-SUMMARY pass=${results.length - failed.length} fail=${failed.length}`);
if (failed.length) {
  console.log("T39-FAILED:\n" + failed.join("\n"));
  process.exit(1);
}
