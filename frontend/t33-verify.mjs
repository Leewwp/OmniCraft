// T33（FIX-37）浏览器 rig —— worktree 本地 only，不入 PR。
// 验收：settings 信誉明细区渲染 + 分页（14 条 / 10 每页 → 加载更多到 14）。
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
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 20000 });
  check("B1 登录", true);

  await page.goto(`${BASE}/settings`);
  await page.getByText(/信誉明细|Reputation details/).first().waitFor({ timeout: 15000 });
  // total 由接口异步填充：等「共 14 条」出现（避免取到初始 0）。
  await page.getByText(/共 14 条|14 entries/).first().waitFor({ timeout: 15000 });
  check("B2 信誉明细区渲染 + 总数 14", true);

  // 前 10 条 + i18n reason（如「优质内容」「标签建议被认可」「AI 审核违规」）
  await page.getByText(/优质内容|AI 审核违规|标签建议被认可|有效举报|PR 已合并/).first().waitFor({ timeout: 10000 });
  const loadMore = page.getByRole("button", { name: /加载更多|Load more/ });
  check("B3 reason i18n 渲染 + 加载更多可见", await loadMore.isVisible());

  await loadMore.click();
  await page.waitForFunction(() => document.querySelectorAll("ul li").length >= 14, { timeout: 15000 });
  const count = await page.locator("ul li").count();
  check("B4 分页加载更多到 14 条", count >= 14, `count=${count}`);
  await page.screenshot({ path: `${SHOT_DIR}/a33-01-reputation-detail.png`, fullPage: true });
} catch (e) {
  failed++;
  console.log(`FAIL exception: ${e.message}`);
  await page.screenshot({ path: `${SHOT_DIR}/a33-99-error.png`, fullPage: true }).catch(() => {});
} finally {
  await browser.close();
}

console.log(`\nTOTAL pass=${passed} fail=${failed}`);
process.exit(failed > 0 ? 1 : 0);
