// T29（FIX-15）封禁申诉全旅程 rig —— worktree 本地 only，不入 PR。
// 语义边界：refresh 对 banned 拒绝（FIX-15 未改），全页加载后 banned 会话
// 必然登出；封禁屏仅在活跃 SPA 会话的 /auth/me 200 窗口可达（由
// tests/banned-protected-layout.test.tsx 组件级覆盖渲染矩阵）。
// 本 rig 覆盖浏览器可见面：登录页 403 指引 → /feedback 工单 → 解封后登录 →
// appeals 页 account 表单 UI；API 链路：ban → /auth/me 200+capabilities →
// account 申诉 → admin 批准 → 解封即时生效（缓存失效）。
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
const banFlow = () => psql(`UPDATE users SET is_banned=true, ban_reason='spam' WHERE email='${FLOW_EMAIL}'`);
const unbanFlow = () => psql(`UPDATE users SET is_banned=false, ban_reason='' WHERE email='${FLOW_EMAIL}'`);

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

async function apiGetMe(token) {
  const res = await fetch(`${API}/api/v1/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  return { status: res.status, body: await res.json() };
}

async function apiSubmitAccountAppeal(token) {
  const token0 = await csrf();
  const res = await fetch(`${API}/api/v1/appeals`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}`, "X-CSRF-Token": token0, Cookie: `csrf-token=${token0}` },
    body: JSON.stringify({ target_type: "account", reason: "误封申诉：我未发送垃圾信息，请复核。" }),
  });
  return { status: res.status, body: await res.json() };
}

async function apiResolveAppeal(appealId) {
  const admin = await apiLogin(ADMIN_EMAIL, ADMIN_PASSWORD);
  if (admin.status !== 200) throw new Error("admin login failed");
  const token0 = await csrf();
  const res = await fetch(`${API}/api/v1/admin/appeals/${appealId}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${admin.body.tokens.access_token}`,
      "X-CSRF-Token": token0,
      Cookie: `csrf-token=${token0}`,
    },
    body: JSON.stringify({ status: "approved", admin_response: "复核通过，予以解封" }),
  });
  return { status: res.status, body: await res.json() };
}

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();

try {
  // 0) 复位：解封 flow 用户
  unbanFlow();

  // ===== 浏览器可见面 =====
  // 1) 正常登录
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', FLOW_EMAIL);
  await page.fill('input[type="password"]', FLOW_PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 20000 });
  check("B1 正常登录成功", true);
  await page.screenshot({ path: `${SHOT_DIR}/a29-01-logged-in.png`, fullPage: true });

  // 2) DB 封禁 → 全页刷新被踢回登录页（refresh 对 banned 拒绝）
  banFlow();
  await page.goto(`${BASE}/settings`);
  await page.waitForURL(/\/login/, { timeout: 20000 });

  // 3) 登录 → 403 USER_BANNED 文案 + 工单入口
  await page.fill('input[type="email"]', FLOW_EMAIL);
  await page.fill('input[type="password"]', FLOW_PASSWORD);
  await page.click('button[type="submit"]');
  await page.getByText(/账号已被封禁|Account has been banned/).first().waitFor({ timeout: 15000 });
  const feedbackLink = page.getByRole("link", { name: /提交工单|Submit feedback/ }).first();
  await feedbackLink.waitFor({ timeout: 10000 });
  check("B2 登录页 USER_BANNED 文案 + 工单入口", true);
  await page.screenshot({ path: `${SHOT_DIR}/a29-02-login-banned-guidance.png`, fullPage: true });

  // 4) 工单入口 → /feedback 公开路由可达
  await feedbackLink.click();
  await page.waitForURL(/\/feedback/, { timeout: 15000 });
  check("B3 工单入口跳转 /feedback", true);
  await page.screenshot({ path: `${SHOT_DIR}/a29-03-feedback.png`, fullPage: true });

  // ===== API 链路（ban 窗口内的申诉出路闭环）=====
  // 5) flow 用户曾登录的 token 已随登出作废：直接 API 复现「持旧 token」窗口。
  unbanFlow();
  const login = await apiLogin(FLOW_EMAIL, FLOW_PASSWORD);
  check("A1 API 登录拿 token", login.status === 200, JSON.stringify(login.body));
  const accessToken = login.body.tokens.access_token;

  banFlow();
  const meBanned = await apiGetMe(accessToken);
  check(
    "A2 ban 后 /auth/me 200 + capabilities.USER_BANNED",
    meBanned.status === 200 &&
      meBanned.body.capabilities?.can_interact === false &&
      meBanned.body.capabilities?.interaction_denial_reason === "USER_BANNED" &&
      meBanned.body.user?.is_banned === true,
    JSON.stringify(meBanned.body).slice(0, 200),
  );

  const appeal = await apiSubmitAccountAppeal(accessToken);
  check(
    "A3 banned token 提交 account 申诉 201（target_id 强制本人）",
    appeal.status === 201 && appeal.body.appeal?.target_type === "account" && appeal.body.appeal?.target_id === 3,
    JSON.stringify(appeal.body).slice(0, 200),
  );

  const resolved = await apiResolveAppeal(appeal.body.appeal.id);
  check("A4 admin 批准 account 申诉", resolved.status === 200, JSON.stringify(resolved.body).slice(0, 200));

  const meUnbanned = await apiGetMe(accessToken);
  check(
    "A5 解封即时生效（capabilities 恢复）",
    meUnbanned.status === 200 && meUnbanned.body.capabilities?.can_interact === true && meUnbanned.body.user?.is_banned === false,
    JSON.stringify(meUnbanned.body).slice(0, 200),
  );

  // ===== 解封后浏览器登录 + appeals 表单 UI =====
  // 6) 解封后登录成功
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', FLOW_EMAIL);
  await page.fill('input[type="password"]', FLOW_PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.startsWith("/login"), { timeout: 20000 });
  check("B4 解封后浏览器登录成功", true);
  await page.screenshot({ path: `${SHOT_DIR}/a29-04-unbanned-login.png`, fullPage: true });

  // 7) appeals 页 account 选项 + hint + 免 target_id + 列表已通过
  await page.goto(`${BASE}/appeals`);
  await page.getByRole("button", { name: /新建申诉|New Appeal/ }).first().waitFor({ timeout: 15000 });
  await page.getByRole("button", { name: /新建申诉|New Appeal/ }).first().click();
  await page.locator("select").first().selectOption("account");
  await page.getByText(/账号申诉无需填写目标 ID|No target ID is needed/).first().waitFor({ timeout: 10000 });
  const targetIdVisible = await page.locator('input[type="number"]').isVisible().catch(() => false);
  check("B5 appeals 表单 account 选项 + hint + 免 target_id", !targetIdVisible);
  await page.screenshot({ path: `${SHOT_DIR}/a29-05-appeal-form-account.png`, fullPage: true });

  const approvedBadge = await page.getByText(/已通过|Approved/).first().isVisible().catch(() => false);
  check("B6 申诉列表显示已通过", approvedBadge);
  await page.screenshot({ path: `${SHOT_DIR}/a29-06-appeal-approved.png`, fullPage: true });
} catch (e) {
  failed++;
  console.log(`FAIL exception: ${e.message}`);
  await page.screenshot({ path: `${SHOT_DIR}/a29-99-error.png`, fullPage: true }).catch(() => {});
} finally {
  await browser.close();
}

console.log(`\nTOTAL pass=${passed} fail=${failed}`);
process.exit(failed > 0 ? 1 : 0);
