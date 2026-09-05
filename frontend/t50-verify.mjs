/* T50 浏览器验证（无头 Chromium，道 P 隔离栈 3206/8093）：
 * 种子：content_contributors (内容1, user 5, pr_count=2)。
 * 1) lanep-ok 登录 /studio/contributors → lanep-creator 可见（真实用户名、PR 计数 2、状态正常）
 * 2) 点击屏蔽 → ConfirmModal 确认 → 状态变已屏蔽
 * 3) 刷新页面 → blocked 状态保持（真实服务端数据，非前端本地翻转）
 * rig 不进 PR；截图输出 ../screenshots/t50-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T50_BASE_URL || "http://localhost:3206";
const SHOTS = path.join(HERE, "..", "screenshots");
const CT = zh.dashboard.contributors;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T50-STEP " + line);
}

async function login(page, email) {
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', "LaneP#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
  return !page.url().includes("login");
}

async function gotoContributors(page, email) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    await page.goto(`${BASE}/studio/contributors`, { waitUntil: "domcontentloaded" });
    for (let i = 0; i < 16; i += 1) {
      if (page.url().includes("/login")) break;
      if ((await page.getByRole("heading", { name: CT.title }).count()) > 0) return true;
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

  /* 1) 贡献者列表真实数据 */
  step("login-author", await login(page, "lanep-ok@seed.omnicraft.local"));
  await gotoContributors(page, "lanep-ok@seed.omnicraft.local");
  await page.getByRole("cell", { name: "lanep-creator" }).waitFor({ state: "visible", timeout: 20000 });
  const row = page.locator("tbody tr", { hasText: "lanep-creator" });
  const rowText = await row.innerText();
  step("contributor-real-username", rowText.includes("lanep-creator"));
  step("contributor-pr-count", rowText.includes("2"), "pr_count=2");
  step("contributor-status-normal", rowText.includes(CT.normal));
  await page.screenshot({ path: path.join(SHOTS, "t50-contributors-list.png"), fullPage: false });

  /* 2) 屏蔽操作（ConfirmModal） */
  await row.getByRole("button", { name: CT.block }).click();
  await page.getByRole("button", { name: CT.block, exact: true }).last().click();
  let blockedSeen = false;
  for (let i = 0; i < 15; i += 1) {
    if ((await page.locator("tbody tr", { hasText: "lanep-creator" }).innerText()).includes(CT.blocked)) {
      blockedSeen = true; break;
    }
    await page.waitForTimeout(500);
  }
  step("block-action-flips-status", blockedSeen);
  await page.screenshot({ path: path.join(SHOTS, "t50-contributor-blocked.png"), fullPage: false });

  /* 3) 刷新后 blocked 状态保持（真实服务端状态） */
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.getByRole("cell", { name: "lanep-creator" }).waitFor({ state: "visible", timeout: 20000 });
  const rowAfter = await page.locator("tbody tr", { hasText: "lanep-creator" }).innerText();
  step("blocked-state-persists-reload", rowAfter.includes(CT.blocked));

  await browser.close();
  const failed = results.filter((r) => r.startsWith("FAIL"));
  console.log(`T50-SUMMARY ${results.length - failed.length}/${results.length}`);
  process.exit(failed.length ? 1 : 0);
} catch (err) {
  console.error("T50-RIG-ERROR", err);
  await browser.close();
  process.exit(1);
}
