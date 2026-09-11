// T31（FIX-27）浏览器 rig —— worktree 本地 only，不入 PR。
// 验收面：用户侧可见 admin_response（票面 AC）+ account 申诉链回归 +
// admin 状态筛选 API + content 查看链接的数据面。
import { chromium } from "playwright";
import { execSync } from "child_process";

const BASE = "http://localhost:3204";
const API = "http://localhost:8088";
const SHOT_DIR = "screenshots";
const FLOW_EMAIL = "lanez-flow@seed.omnicraft.local";
const FLOW_PASSWORD = "LanezFlow#2026";
const ADMIN_EMAIL = "lanez-admin@seed.omnicraft.local";
const ADMIN_PASSWORD = "LanezAdmin#2026";

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

async function csrf() {
  const res = await fetch(`${API}/api/v1/auth/csrf`);
  const body = await res.json();
  return body.csrf_token;
}

async function apiLogin(email, password) {
  const token = await csrf();
  const res = await fetch(`${API}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": token, Cookie: `csrf-token=${token}` },
    body: JSON.stringify({ email, password }),
  });
  return { status: res.status, body: await res.json() };
}

async function apiCall(method, path, token, bodyObj) {
  const token0 = await csrf();
  const res = await fetch(`${API}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      "X-CSRF-Token": token0,
      Cookie: `csrf-token=${token0}`,
    },
    body: bodyObj ? JSON.stringify(bodyObj) : undefined,
  });
  let body = null;
  try { body = await res.json(); } catch { body = null; }
  return { status: res.status, body };
}

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();

try {
  // ===== API 数据面 =====
  const flowLogin = await apiLogin(FLOW_EMAIL, FLOW_PASSWORD);
  check("A0 flow 登录", flowLogin.status === 200);
  const flowToken = flowLogin.body.tokens.access_token;
  const adminLogin = await apiLogin(ADMIN_EMAIL, ADMIN_PASSWORD);
  const adminToken = adminLogin.body.tokens.access_token;

  // A1 假目标 404
  const fake = await apiCall("POST", "/api/v1/appeals", flowToken, { target_type: "content", target_id: 999999, reason: "r" });
  check("A1 假目标 404 TARGET_NOT_FOUND", fake.status === 404 && fake.body.code === "TARGET_NOT_FOUND", JSON.stringify(fake.body));

  // A2 造被 ban 的 content → flow 申诉（浏览器表单）→ admin 批准
  const ipId = parseInt(psql(`INSERT INTO ips (name, slug, description, status, created_at, updated_at) VALUES ('t31-ip','t31-ip','d','approved',NOW(),NOW()) RETURNING id`), 10);
  execSync(`docker exec lanez-pg psql -U postgres -d omnicraft -c "UPDATE ips SET creator_id=3 WHERE id=${ipId}"`);
  const contentId = parseInt(psql(`INSERT INTO content_items (title, description, author_id, zone, ip_id, content_type, status, is_public, allow_copy, agent_enabled, is_paid, view_count, like_count, dislike_count, download_count, created_at, updated_at) VALUES ('T31 验证内容','d',3,'fanwork',${ipId},'article','published',true,true,false,false,0,0,0,0,NOW(),NOW()) RETURNING id`), 10);
  const ban = await apiCall("POST", `/api/v1/admin/contents/${contentId}/ban`, adminToken, { reason: "违规" });
  check("A2a admin ban content", ban.status === 200, JSON.stringify(ban.body).slice(0, 120));

  // A3 admin 状态筛选数据面
  const pendingList = await apiCall("GET", "/api/v1/admin/appeals?status=pending", adminToken);
  const approvedList0 = await apiCall("GET", "/api/v1/admin/appeals?status=approved", adminToken);
  check("A3a status 筛选默认/枚举可用", pendingList.status === 200 && approvedList0.status === 200);
  const beforeApproved = approvedList0.body.total;

  // ===== 浏览器用户面 =====
  // 说明：refresh 双发竞态（#322 / FIX-32，T32 范围）会让全页导航概率性登出，
  // 本 rig 登录后全部走 client-side 导航（用户菜单 goTo=router.push）。
  // B1 flow 登录 → 用户菜单进 appeals → 表单提交 content 申诉
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', FLOW_EMAIL);
  await page.fill('input[type="password"]', FLOW_PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 20000 });
  await page.getByRole("button", { name: FLOW_EMAIL.replace(/.*@/, "") }).first().waitFor({ timeout: 5000 }).catch(() => {});
  async function gotoAppealsViaMenu() {
    await page.locator("header button", { hasText: /lanez-flow/ }).first().click();
    await page.getByRole("menuitem", { name: /我的申诉|My Appeals/ }).first().click();
    await page.waitForURL(/\/appeals/, { timeout: 15000 });
  }
  await gotoAppealsViaMenu();
  await page.getByRole("button", { name: /新建申诉|New Appeal/ }).first().waitFor({ timeout: 15000 });
  await page.getByRole("button", { name: /新建申诉|New Appeal/ }).first().click();
  await page.locator("select").first().selectOption("content");
  await page.fill('input[type="number"]', String(contentId));
  await page.fill("textarea", "这是误判申诉，请复核内容状态。");
  await page.getByRole("button", { name: /提交$/ }).first().click();
  await page.getByText(/待处理|Pending/).first().waitFor({ timeout: 15000 });
  check("B1 浏览器提交 content 申诉", true);
  await page.screenshot({ path: `${SHOT_DIR}/a31-01-appeal-submitted.png`, fullPage: true });

  // A4 重复 resolve 防护 + 批准（API，带 admin_response）
  const appealId = parseInt(psql(`SELECT id FROM appeals WHERE user_id=3 AND target_type='content' AND status='pending' ORDER BY id DESC LIMIT 1`), 10);
  const resolve1 = await apiCall("POST", `/api/v1/admin/appeals/${appealId}`, adminToken, { status: "approved", admin_response: "复核通过，内容已恢复上架。" });
  check("A4a admin 批准", resolve1.status === 200, JSON.stringify(resolve1.body).slice(0, 120));
  const resolve2 = await apiCall("POST", `/api/v1/admin/appeals/${appealId}`, adminToken, { status: "rejected", admin_response: "x" });
  check("A4b 重复 resolve 409", resolve2.status === 409 && resolve2.body.code === "APPEAL_ALREADY_RESOLVED", JSON.stringify(resolve2.body));
  const approvedList = await apiCall("GET", "/api/v1/admin/appeals?status=approved", adminToken);
  check("A4c approved 筛选含新申诉", approvedList.body.total === beforeApproved + 1, `before=${beforeApproved} after=${approvedList.body.total}`);

  // B2 用户侧 client-side 重新进入 appeals（remount 重拉数据）可见 admin_response（票面 AC）
  // /appeals（protected 组）无全局 Header：先 history back 回 home（SPA 内）再进菜单。
  await page.goBack();
  await page.waitForURL((u) => !u.pathname.startsWith("/appeals"), { timeout: 15000 });
  await gotoAppealsViaMenu();
  await page.getByText(/复核通过，内容已恢复上架。/).first().waitFor({ timeout: 15000 });
  const adminResponseLabel = await page.getByText(/处理意见|Admin response/).first().isVisible().catch(() => false);
  check("B2 用户侧可见 admin_response + label", adminResponseLabel);
  await page.screenshot({ path: `${SHOT_DIR}/a31-02-admin-response-visible.png`, fullPage: true });

  // B3 content 恢复 published（comment approved 恢复语义同型，已由单测覆盖）
  const contentStatus = psql(`SELECT status FROM content_items WHERE id=${contentId}`);
  check("B3 申诉批准后 content 恢复 published", contentStatus === "published", contentStatus);
} catch (e) {
  failed++;
  console.log(`FAIL exception: ${e.message}`);
  await page.screenshot({ path: `${SHOT_DIR}/a31-99-error.png`, fullPage: true }).catch(() => {});
} finally {
  await browser.close();
}

console.log(`\nTOTAL pass=${passed} fail=${failed}`);
process.exit(failed > 0 ? 1 : 0);
