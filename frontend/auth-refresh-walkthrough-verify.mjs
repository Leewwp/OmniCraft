// #381 auth refresh race —— 长会话顺序导航走查 rig（验收 2）
// 用法：本地栈（backend :8080 + frontend dev :3000，需包含本票修复）运行：
//   node frontend/auth-refresh-walkthrough-verify.mjs
// 验收：登录后连续硬加载 8 个受保护页面，无一弹回 /login，无 401 泄漏到 console。
// 账号为 .local 种子域测试账号（scripts/corpus 同源）。
import { chromium } from "playwright";

const BASE = process.env.WALKTHROUGH_BASE || "http://localhost:3000";
const EMAIL = process.env.WALKTHROUGH_EMAIL || "smoke-judge@seed.omnicraft.local";
const PASSWORD = process.env.WALKTHROUGH_PASSWORD || "CorpusV2#2026";
const SHOT_DIR = "screenshots";

// 顺序导航的受保护页面（对普通用户可访问），登录后硬加载逐个访问
const PAGES = [
  { path: "/dashboard", marker: /仪表盘|Dashboard|工作台|创作/ },
  { path: "/messages", marker: /消息|Message/ },
  { path: "/history", marker: /历史|History|足迹/ },
  { path: "/ip", marker: /IP|原创|作品|库/ },
  { path: "/studio", marker: /创作|Studio|工作|发布/ },
  { path: "/agent", marker: /助手|Agent|AI|提问/ },
  { path: "/settings", marker: /设置|Settings|账号|资料/ },
  { path: "/dashboard", marker: /仪表盘|Dashboard|工作台|创作/ }, // 第 8 页回访，制造同秒二次轮换窗口
];

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

const unauthorizedResponses = [];
let trackAuth = false;
page.on("response", (res) => {
  // 只统计登录成功之后的 401：登录前匿名 fetchMe 的 /auth/refresh 401（无
  // cookie）是预期行为。
  if (trackAuth && res.status() === 401 && res.url().includes("/api/v1/")) {
    unauthorizedResponses.push(`${res.request().method()} ${new URL(res.url()).pathname}`);
  }
});

try {
  // 预热：dev 模式首次编译很慢，会稀释「同秒轮换」竞态窗口；先匿名刷一遍让
  // Next 编译缓存就位，模拟审计走查的真实快节奏导航。
  for (const p of PAGES.map((x) => x.path)) {
    await page.goto(`${BASE}${p}`, { waitUntil: "domcontentloaded", timeout: 60000 }).catch(() => {});
  }

  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 30000 });
  check("登录成功", true);
  trackAuth = true;

  // 登录后立即连续硬加载 8 页（不人为等待，最大化同秒轮换窗口压力）
  for (let i = 0; i < PAGES.length; i++) {
    const p = PAGES[i];
    await page.goto(`${BASE}${p.path}`, { waitUntil: "domcontentloaded", timeout: 60000 });
    const landed = !page.url().startsWith(`${BASE}/login`);
    check(
      `第 ${i + 1} 页 ${p.path} 不弹登录`,
      landed,
      `landed=${page.url()}`,
    );
    if (i === 0 || i === PAGES.length - 1) {
      await page
        .getByText(p.marker)
        .first()
        .waitFor({ timeout: 20000 })
        .then(() => check(`第 ${i + 1} 页内容渲染`, true))
        .catch(() => check(`第 ${i + 1} 页内容渲染`, false, "marker not found"));
    }
  }

  await page.screenshot({ path: `${SHOT_DIR}/a381-walkthrough-final.png`, fullPage: true }).catch(() => {});
  check(
    "全程无 401 泄漏到 /api/v1",
    unauthorizedResponses.length === 0,
    unauthorizedResponses.slice(0, 5).join(", "),
  );
} catch (e) {
  failed++;
  console.log(`FAIL exception: ${e.message}`);
  await page.screenshot({ path: `${SHOT_DIR}/a381-walkthrough-error.png`, fullPage: true }).catch(() => {});
} finally {
  await browser.close();
}

console.log(`\nTOTAL pass=${passed} fail=${failed}`);
process.exit(failed > 0 ? 1 : 0);
