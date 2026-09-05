// T32（FIX-32 + #322）浏览器全旅程 rig —— worktree 本地 only，不入 PR。
// 场景（backend override：access_token TTL=5s）：
// B1 登录 → 等 token 过期 → client-side 导航受保护页：fetchMe 静默刷新恢复，不踢登录；
// B2 两次全页导航（refresh 双发竞态场景 #322）：会话保持不弹回登录页；
// B3 未登录带 query 访问受保护页 → 登录 → 回跳 URL 保参。
import { chromium } from "playwright";

const BASE = "http://localhost:3204";
const SHOT_DIR = "screenshots";
const EMAIL = "lanez-flow@seed.omnicraft.local";
const PASSWORD = "LanezFlow#2026";

let passed = 0;
let failed = 0;
function check(name, cond, detail = "") {
  if (cond) {
    passed++;
    console.log(`PASS ${name}`);
  } else {
    failed++;
    console.log(`FAIL ${name} ${detail}`);
  }
}

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();

try {
  // ===== B1：过期 token 自动恢复 =====
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 20000 });
  check("B1a 登录成功", true);

  // access_token TTL=5s：等过期后 client-side 导航（内存 token 已过期，
  // fetchMe 走 isTokenExpired → refresh → 恢复）。
  await page.waitForTimeout(6500);
  await page.getByRole("button", { name: /lanez-flow/ }).first().click();
  await page.getByRole("menuitem", { name: /我的申诉|My Appeals/ }).first().click();
  await page.waitForURL(/\/appeals/, { timeout: 20000 });
  await page.getByRole("button", { name: /新建申诉|New Appeal/ }).first().waitFor({ timeout: 15000 });
  check("B1b 过期 token 静默刷新恢复（未踢登录）", true);
  await page.screenshot({ path: `${SHOT_DIR}/a32-01-expired-token-recovered.png`, fullPage: true });

  // ===== B2：#322 双发竞态——连续全页导航会话保持 =====
  await page.waitForTimeout(5500); // 再次过期
  await page.goto(`${BASE}/settings`, { waitUntil: "load" });
  await page.getByText(/账户设置|账号设置|Settings|设置/).first().waitFor({ timeout: 20000 }).catch(() => {});
  const url1 = page.url();
  check("B2a 第一次全页导航恢复会话", !url1.startsWith(`${BASE}/login`), url1);

  await page.waitForTimeout(5500);
  await page.goto(`${BASE}/history`, { waitUntil: "load" });
  await page.waitForTimeout(1500);
  const url2 = page.url();
  check("B2b 第二次全页导航仍保持会话（#322 不再弹回）", !url2.startsWith(`${BASE}/login`), url2);
  await page.screenshot({ path: `${SHOT_DIR}/a32-02-double-nav-kept.png`, fullPage: true });

  // ===== B3：带 query 受保护 URL 回跳保参 =====
  // 登出（清会话）
  await page.evaluate(() => {
    window.localStorage.clear();
  });
  const fresh = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const p2 = await fresh.newPage();
  // 未登录直接访问带 query 的受保护页
  await p2.goto(`${BASE}/appeals?target_type=account&utm_source=e2e`, { waitUntil: "load" });
  await p2.waitForURL(/\/login\?redirect=/, { timeout: 20000 });
  const loginUrl = p2.url();
  const redirectParam = decodeURIComponent(new URL(loginUrl).searchParams.get("redirect") || "");
  check(
    "B3a redirect 保留完整 query",
    redirectParam === "/appeals?target_type=account&utm_source=e2e",
    redirectParam,
  );
  await p2.fill('input[type="email"]', EMAIL);
  await p2.fill('input[type="password"]', PASSWORD);
  await p2.click('button[type="submit"]');
  await p2.waitForURL(/\/appeals\?/, { timeout: 20000 });
  const backUrl = p2.url();
  check(
    "B3b 登录后回跳 URL 保参",
    backUrl.includes("/appeals?target_type=account&utm_source=e2e"),
    backUrl,
  );
  await p2.screenshot({ path: `${SHOT_DIR}/a32-03-redirect-with-query.png`, fullPage: true });
  await fresh.close();
} catch (e) {
  failed++;
  console.log(`FAIL exception: ${e.message}`);
  await page.screenshot({ path: `${SHOT_DIR}/a32-99-error.png`, fullPage: true }).catch(() => {});
} finally {
  await browser.close();
}

console.log(`\nTOTAL pass=${passed} fail=${failed}`);
process.exit(failed > 0 ? 1 : 0);
