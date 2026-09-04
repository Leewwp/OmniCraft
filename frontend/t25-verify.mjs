/* T25 浏览器验证（隔离栈 3200/8084，override image_max_mb=5）：
 * 1) 原创发布类型清单顺序跟随 /config/public（config 序：article 第 2、
 *    prompt 不在 original 配置中不展示）
 * 2) IP 发布表单封面上传上限提示显示「限制：5MB」（运行时覆盖值，
 *    非硬编码 20）
 * rig 不进 PR；截图 ../screenshots/t25-*.png */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T25_BASE_URL || "http://localhost:3200";
const SHOTS = path.join(HERE, "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T25-STEP " + line);
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

  /* 登录 */
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "lane3-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "Lane3Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });

  /* 2) IP 发布封面：上限提示为运行时 5MB（content.limitMb = 限制：{maxMB}MB）
   *    注意：本栈登录后的第二次整页跳转会触发 refresh 轮换竞态弹回登录页
   *    （疑似独立 auth 缺陷，非本票范围），故封面检查放在首个跳转完成。 */
  await page.goto(`${BASE}/studio/publish/ip`);
  const hintVisible = await waitForText(page, "限制：5MB", 40000);
  step("cover-limit-hint-5mb", hintVisible);
  const staleHint = (await page.locator("body").innerText()).includes("限制：20MB");
  step("cover-limit-not-stale-20mb", !staleHint);
  await page.screenshot({ path: path.join(SHOTS, "t25-cover-limit-5mb.png"), fullPage: false });

  /* 1) 原创类型清单：顺序跟配置（image, article, video, audio, template,
   *    sheet_music, other——article 第二、无 prompt）。按正文出现序判定。 */
  await page.goto(`${BASE}/studio/publish/original`);
  await waitForText(page, zh.studio.publish.typeLabel.article, 40000);
  await page.waitForTimeout(1500); /* 等配置下发重排 */
  const bodyText = await page.locator("body").innerText();
  const labelOrder = ["image", "article", "video", "audio", "template", "sheet_music", "prompt", "other"]
    .map((v) => ({ v, at: bodyText.indexOf(zh.studio.publish.typeLabel[v]) }))
    .filter((x) => x.at >= 0)
    .sort((a, b) => a.at - b.at)
    .map((x) => x.v);
  const articleIdx = labelOrder.indexOf("article");
  const videoIdx = labelOrder.indexOf("video");
  const promptIdx = labelOrder.indexOf("prompt");
  step("type-grid-follows-config-order", articleIdx === 1 && videoIdx === 2, `order=${labelOrder.join("/")}`);
  step("type-grid-drops-unlisted-prompt", promptIdx === -1, `promptIdx=${promptIdx}`);
  await page.screenshot({ path: path.join(SHOTS, "t25-type-grid-config-order.png"), fullPage: false });

} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log(failed.length === 0 ? "T25 ALL PASS" : `T25 FAILED: ${failed.length}`);
process.exit(failed.length === 0 ? 0 : 1);
