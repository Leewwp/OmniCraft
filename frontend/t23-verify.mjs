/* T23 浏览器验证（无头 Chromium，道 3 隔离栈 3200/8084）：
 * 1) 登录 lane3-verify 用户（DB 直插 verified、reputation 10，redis 预置 publish:freeze:9101）
 * 2) /studio/publish/original 选「文章」类型 → 填标题/分类 → 提交
 * 3) 断言 toast 命中 PUBLISH_FROZEN 专属文案（含 7 天自动解冻 + 素质课程指引），
 *    且不再是裸 code / 通用「发布失败」。
 * rig 不进 PR；截图输出 ../screenshots/t23-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T23_BASE_URL || "http://localhost:3200";
const SHOTS = path.join(HERE, "..", "screenshots");
const FROZEN = zh.publish.frozen;
const GENERIC_FAILED = zh.studio.publish.failed;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T23-STEP " + line);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 1) 登录 */
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "lane3-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "Lane3Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
  step("login", !page.url().includes("login"), page.url());

  /* 2) 原创发布 → 选文章类型 */
  await page.goto(`${BASE}/studio/publish/original`);
  const articleTile = page.getByRole("button", { name: new RegExp(zh.studio.publish.typeLabel.article) }).first();
  await articleTile.waitFor({ state: "visible", timeout: 20000 });
  await articleTile.click();
  await page.screenshot({ path: path.join(SHOTS, "t23-publish-article-form.png"), fullPage: false });

  /* 3) 填表并提交（标题 + 分类即可过前端校验，POST 被 publishGuard 403 拦截） */
  await page.fill('input[placeholder="输入作品标题"]', "T23 冻结文案验证");
  await page.getByRole("button", { name: zh.home.categoryFilmTv, exact: true }).click();
  await page.getByRole("button", { name: zh.studio.publish.submit }).click();

  /* 4) 断言专属冻结文案（含指引），而非通用失败/裸 code */
  let toastText = "";
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(400);
    const body = await page.locator("body").innerText();
    if (body.includes(FROZEN.slice(0, 10))) { toastText = body; break; }
  }
  const hitFrozen = toastText.includes(FROZEN.slice(0, 10));
  const hasGuidance = toastText.includes("7 天") && toastText.includes("素质课程");
  const notGeneric = !toastText.includes(GENERIC_FAILED);
  const notRawCode = !/PUBLISH_FROZEN/.test(toastText);
  step("toast-frozen-copy", hitFrozen, FROZEN.slice(0, 24) + "…");
  step("toast-guidance-7d-course", hasGuidance);
  step("toast-not-generic-failed", notGeneric);
  step("toast-not-raw-code", notRawCode);
  await page.screenshot({ path: path.join(SHOTS, "t23-publish-frozen-toast.png"), fullPage: false });

  /* 5) 对照组：删除冻结键后同表单可正常进入提交流程（无冻结文案） */
  const { execSync } = await import("node:child_process");
  execSync("docker exec lane3-redis redis-cli DEL publish:freeze:9101");
  await page.fill('input[placeholder="输入作品标题"]', "T23 解冻对照");
  await page.getByRole("button", { name: zh.studio.publish.submit }).click();
  let unfrozenBody = "";
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(400);
    unfrozenBody = await page.locator("body").innerText();
    if (!unfrozenBody.includes(FROZEN.slice(0, 10)) && unfrozenBody.includes(zh.studio.publish.submitting || "发布中")) break;
  }
  step("unfrozen-no-frozen-copy", !unfrozenBody.includes(FROZEN.slice(0, 10)));
  await page.screenshot({ path: path.join(SHOTS, "t23-publish-unfrozen.png"), fullPage: false });
} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log(failed.length === 0 ? "T23 ALL PASS" : `T23 FAILED: ${failed.length}`);
process.exit(failed.length === 0 ? 0 : 1);
