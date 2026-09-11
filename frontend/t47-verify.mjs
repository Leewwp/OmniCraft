/* T47 浏览器验证（隔离栈 3200/8084，阈值 0.30 经 /config/public 下发）：
 * 1) 高踩比评论（8100：1 赞 5 踩）默认折叠——正文不可见，仅「已折叠 · 点击显示」
 * 2) 点击展开后正文可见，且出现「收起」可再折叠
 * 3) 对照评论（8101：5 赞 1 踩）不折叠
 * rig 不进 PR；截图 ../screenshots/t47-*.png */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T47_BASE_URL || "http://localhost:3200";
const SHOTS = path.join(HERE, "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T47-STEP " + line);
}

async function waitForText(page, text, maxMs = 30000) {
  for (let i = 0; i < Math.ceil(maxMs / 500); i += 1) {
    await page.waitForTimeout(500);
    if ((await page.locator("body").innerText()).includes(text)) return true;
  }
  return (await page.locator("body").innerText()).includes(text);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  await page.goto(`${BASE}/original/7301`);
  await waitForText(page, "压测评论 #8100", 40000);
  await page.waitForTimeout(1200); /* 等折叠阈值下发 */

  const body = await page.locator("body").innerText();
  /* 1) 8100 折叠：正文（压测评论 #8100）不可见，折叠开关可见 */
  const foldedHidden = !body.includes("压测评论 #8100");
  const foldToggleVisible = body.includes(zh.social.commentFolded);
  step("high-ratio-folded-by-default", foldedHidden && foldToggleVisible, `hidden=${foldedHidden} toggle=${foldToggleVisible}`);
  await page.screenshot({ path: path.join(SHOTS, "t47-folded-default.png"), fullPage: false });

  /* 2) 点击展开 → 正文可见 + 出现收起 */
  await page.getByRole("button", { name: new RegExp(zh.social.commentFolded) }).first().click();
  const revealed = await waitForText(page, "压测评论 #8100", 10000);
  step("click-to-reveal", revealed);
  const collapseVisible = (await page.locator("body").innerText()).includes(zh.social.commentCollapse);
  step("collapse-affordance-after-reveal", collapseVisible);
  await page.screenshot({ path: path.join(SHOTS, "t47-revealed.png"), fullPage: false });

  /* 3) 对照 8101（5 赞 1 踩）不折叠 */
  const controlVisible = (await page.locator("body").innerText()).includes("压测评论 #8101");
  step("low-ratio-not-folded", controlVisible);
} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log(failed.length === 0 ? "T47 ALL PASS" : `T47 FAILED: ${failed.length}`);
process.exit(failed.length === 0 ? 0 : 1);
