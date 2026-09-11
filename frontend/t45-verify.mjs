/* T45 浏览器验证（匿名视角）：/content/7301 评论区显示真实作者昵称
 * （t45_author / lane3_verify），无空白昵称；响应不含 email 值。
 * rig 不进 PR；截图 ../screenshots/t45-*.png。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const BASE = process.env.T45_BASE_URL || "http://localhost:3200";
const SHOTS = path.join(HERE, "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T45-STEP " + line);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 1) 匿名打开内容详情：评论区昵称可见 */
  await page.goto(`${BASE}/original/7301`);
  await page.waitForLoadState("networkidle", { timeout: 30000 }).catch(() => {});
  let body = "";
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(500);
    body = await page.locator("body").innerText();
    if (body.includes("t45_author") && body.includes("lane3_verify")) break;
  }
  step("anonymous-comment-author-names", body.includes("t45_author") && body.includes("lane3_verify"));
  step("anonymous-comment-body", body.includes("第一条：验证昵称与头像加载"));
  await page.screenshot({ path: path.join(SHOTS, "t45-content-comments-anonymous.png"), fullPage: false });

  /* 2) 匿名视角评论 API 响应不含 email 值（T03 回归） */
  const resp = await page.request.get(`${process.env.T45_API_URL || "http://localhost:8084"}/api/v1/social/comments?content_id=7301&page=1&page_size=20`);
  const raw = await resp.text();
  step("api-no-email-value", !raw.includes("@seed.omnicraft.local"), `status=${resp.status()}`);
  step("api-author-present", raw.includes('"username":"t45_author"') && raw.includes('"avatar_url":"https://oss.example/t45-avatar.png"'));
} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log(failed.length === 0 ? "T45 ALL PASS" : `T45 FAILED: ${failed.length}`);
process.exit(failed.length === 0 ? 0 : 1);
