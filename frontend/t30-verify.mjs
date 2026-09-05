// T30（FIX-20）注销软删除浏览器全旅程 rig —— worktree 本地 only，不入 PR。
// 旅程：t30 用户登录 → settings 删除账号（密码+确认勾选）→ 登出回首页 →
// 原邮箱重新登录 → 凭证错误（匿名化后不存在该账号）→ 截图。
import { chromium } from "playwright";
import { execSync } from "child_process";

const BASE = "http://localhost:3204";
const SHOT_DIR = "screenshots";
const EMAIL = "t30-flow@seed.omnicraft.local";
// hash 复用 flow 用户的（LanezFlow#2026），登录与删除确认同密码。
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

const psql = (sql) =>
  execSync(`docker exec lanez-pg psql -U postgres -d omnicraft -tAc "${sql}"`).toString().trim();

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();

try {
  // 0) 独立测试账号（不影响后续票的 flow 用户）；hash 直接复制 flow 用户，免 shell 转义
  execSync(
    `docker exec lanez-pg psql -U postgres -d omnicraft -c "INSERT INTO users (email, password_hash, username, role, is_banned, reputation, email_verified_at) SELECT '${EMAIL}', password_hash, 't30-flow', 'user', false, 10, NOW() FROM users WHERE email='lanez-flow@seed.omnicraft.local' ON CONFLICT (email) DO NOTHING"`,
  );
  const userId = psql(`SELECT id FROM users WHERE email='${EMAIL}'`);
  check("A0 测试账号就位", !!userId, userId);

  // 1) 登录
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 20000 });
  check("B1 登录成功", true);

  // 2) settings → 展开删除账号 → 输密码 → 勾确认 → 确认删除
  await page.goto(`${BASE}/settings`);
  await page.getByRole("button", { name: /注销账号|Delete Account|删除账号/ }).first().waitFor({ timeout: 15000 });
  await page.screenshot({ path: `${SHOT_DIR}/a30-01-settings-before.png`, fullPage: true });
  await page.getByRole("button", { name: /注销账号|Delete Account|删除账号/ }).first().click();
  await page.locator('input[type="password"]').last().fill(PASSWORD);
  // 勾选确认 checkbox（最后一个 checkbox 在删除卡片内）
  const checkboxes = page.locator('input[type="checkbox"], [role="checkbox"]');
  await checkboxes.last().check();
  await page.getByRole("button", { name: /确认注销|确认删除|Confirm Delete/ }).first().click();
  // 注销成功后 logout + push("/") 可能与 protected layout 的 replace(login) 竞态，
  // 两处任一到达即视为登出成功。
  await page.waitForURL((u) => u.pathname === "/" || u.pathname === "" || u.pathname.startsWith("/login"), { timeout: 20000 });
  check("B2 注销成功并登出", true);
  await page.screenshot({ path: `${SHOT_DIR}/a30-02-after-delete.png`, fullPage: true });

  // 3) DB 断言：软删除 + 不再伪装封禁
  const row = psql(`SELECT is_banned || '|' || COALESCE(ban_reason,'') || '|' || (deleted_at IS NOT NULL) FROM users WHERE id=${userId}`);
  check("B3 DB 软删除 + 不伪装封禁", row === "false||true", row);

  // 4) 原邮箱重新登录 → 凭证错误（匿名化后账号不存在）
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.getByText(/邮箱或密码不正确|Invalid email or password/).first().waitFor({ timeout: 15000 });
  check("B4 注销后原邮箱登录走凭证错误（非封禁文案）", true);
  await page.screenshot({ path: `${SHOT_DIR}/a30-03-religin-after-delete.png`, fullPage: true });

  // 5) /auth/me deleted 语义（旧 token 已随注销失效清理，重新验证走 API 语义）
  // 由单测 TestMeAfterDeleteReturnsDeletedSemantics 覆盖 401 deleted 语义。
} catch (e) {
  failed++;
  console.log(`FAIL exception: ${e.message}`);
  await page.screenshot({ path: `${SHOT_DIR}/a30-99-error.png`, fullPage: true }).catch(() => {});
} finally {
  await browser.close();
}

console.log(`\nTOTAL pass=${passed} fail=${failed}`);
process.exit(failed > 0 ? 1 : 0);
