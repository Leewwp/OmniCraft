/* T52 浏览器验证（无头 Chromium，道 P 隔离栈 3206/8093）：
 * 1) lanep-creator 登录 → /studio/ips：既有 rejected 行可见（未通过徽标 + 原因 + 重新提交按钮），深链跳 /studio/publish/ip
 * 2) creator 新提交 IP → /studio/ips 出现「审核中」行
 * 3) admin（curl）approve → 刷新 /studio/ips 状态变「已通过」+ 查看链接；DB 断言创建者收到 ip_status 通知
 * rig 不进 PR；截图输出 ../screenshots/t52-*.png（worktree 内）。 */
import { readFileSync } from "node:fs";
import { execSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T52_BASE_URL || "http://localhost:3206";
const SHOTS = path.join(HERE, "..", "screenshots");
const MY = zh.studio.myIPs;
const FORM = zh.studio.publishIP;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T52-STEP " + line);
}

async function login(page, email) {
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', "LaneP#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });
  return !page.url().includes("login");
}

// 整页跳转可能命中 lane3 登记过的 auth refresh 竞态（2×refresh 401 → 弹回
// /login），页面停在骨架屏或登录页。带自愈：发现弹回就重新登录再进一次。
async function gotoMyIPs(page, email) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    await page.goto(`${BASE}/studio/ips`, { waitUntil: "domcontentloaded" });
    let onPage = false;
    for (let i = 0; i < 16; i += 1) {
      if (page.url().includes("/login")) break;
      if ((await page.getByRole("heading", { name: MY.title }).count()) > 0) {
        // 标题渲染 = auth 就绪；再等骨架屏消失（表格或空态出现）
        for (let j = 0; j < 20; j += 1) {
          const b = await page.locator("body").innerText();
          if (!b.includes("animate-pulse")) return true;
          await page.waitForTimeout(400);
        }
        onPage = true;
        break;
      }
      await page.waitForTimeout(500);
    }
    if (onPage) return true;
    await login(page, email);
  }
  return false;
}

async function csrfPost(pathname, email, body) {
  const out = execSync(
    `cd /tmp && rm -f t52.cookies && curl -s -c t52.cookies http://localhost:8093/api/v1/auth/csrf -o /dev/null && ` +
    `CSRF=$(grep csrf t52.cookies | awk '{print $NF}') && ` +
    `TOKEN=$(curl -s -b t52.cookies -c t52.cookies -X POST http://localhost:8093/api/v1/auth/login -H "Content-Type: application/json" -H "X-CSRF-Token: $CSRF" -d '{"email":"${email}","password":"LaneP#2026"}' | python3 -c "import json,sys; print(json.load(sys.stdin)['tokens']['access_token'])") && ` +
    `curl -s -c t52.cookies http://localhost:8093/api/v1/auth/csrf -o /dev/null && CSRF=$(grep csrf t52.cookies | awk '{print $NF}') && ` +
    `curl -s -b t52.cookies -X POST "http://localhost:8093${pathname}" -H "Content-Type: application/json" -H "X-CSRF-Token: $CSRF" -H "Authorization: Bearer $TOKEN" -d '${body}'`,
  ).toString();
  return out;
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 1) 既有 rejected 行：徽标 + 原因 + 重新提交深链 */
  step("login-creator", await login(page, "lanep-creator@seed.omnicraft.local"));
  await gotoMyIPs(page, "lanep-creator@seed.omnicraft.local");
  await page.getByRole("heading", { name: MY.title }).waitFor({ state: "visible", timeout: 20000 });
  const body = await page.locator("body").innerText();
  step("rejected-row-badge", body.includes(MY.statusRejected));
  step("rejected-row-reason", body.includes("介绍信息不足，请补充世界观设定"), "T16 落库的原因");
  await page.screenshot({ path: path.join(SHOTS, "t52-my-ips-rejected.png"), fullPage: false });

  const resubmit = page.getByRole("link", { name: MY.resubmit }).first();
  await resubmit.click();
  await page.waitForURL((url) => url.pathname.includes("/studio/publish/ip"), { timeout: 10000 });
  step("resubmit-deep-link", page.url().includes("/studio/publish/ip"));

  /* 2) 新提交 → pending 行出现 */
  const stamp = Date.now().toString(36);
  const ipName = `T52 状态旅程 ${stamp}`;
  await page.getByLabel(FORM.nameLabel).fill(ipName);
  await page.locator('button[aria-pressed]').first().click();
  await page.getByRole("button", { name: FORM.submit }).click();
  let submitted = false;
  for (let i = 0; i < 25; i += 1) {
    await page.waitForTimeout(400);
    if ((await page.locator("body").innerText()).includes(FORM.successTitle)) { submitted = true; break; }
  }
  step("creator-submit-pending-panel", submitted);

  await gotoMyIPs(page, "lanep-creator@seed.omnicraft.local");
  let pendingSeen = false;
  for (let i = 0; i < 20; i += 1) {
    const b = await page.locator("body").innerText();
    if (b.includes(ipName) && b.includes(MY.statusPending)) { pendingSeen = true; break; }
    await page.waitForTimeout(500);
  }
  step("my-ips-pending-row", pendingSeen, ipName);
  await page.screenshot({ path: path.join(SHOTS, "t52-my-ips-pending.png"), fullPage: false });

  /* 3) admin approve（curl）→ 前端状态变更 + 通知落库 */
  const ipId = execSync(
    `docker exec lanep-pg psql -U postgres -d omnicraft -tAc "SELECT id FROM ips WHERE name = '${ipName}'"`,
  ).toString().trim();
  csrfPost(`/api/v1/admin/ips/${ipId}/approve`, "lanep-ok@seed.omnicraft.local", "{}");
  step("admin-approve-called", true, `ip id=${ipId}`);

  // approve 后重新拉取列表（gotoMyIPs 内含 auth 竞态自愈；该 auth 问题已
  // 另行登记，不属本票范围）。
  await gotoMyIPs(page, "lanep-creator@seed.omnicraft.local");
  let approvedSeen = false;
  for (let i = 0; i < 25; i += 1) {
    const row = page.locator("tbody tr", { hasText: ipName });
    if ((await row.count()) > 0 && (await row.first().innerText()).includes(MY.statusApproved)) { approvedSeen = true; break; }
    await page.waitForTimeout(600);
  }
  step("my-ips-status-flips-to-approved", approvedSeen);
  await page.screenshot({ path: path.join(SHOTS, "t52-my-ips-approved.png"), fullPage: false });

  let notifCount = "0";
  for (let i = 0; i < 15; i += 1) {
    notifCount = execSync(
      `docker exec lanep-pg psql -U postgres -d omnicraft -tAc "SELECT count(*) FROM notifications WHERE user_id = (SELECT id FROM users WHERE email='lanep-creator@seed.omnicraft.local') AND type='ip_status' AND body LIKE '%${ipName}%'"`,
    ).toString().trim();
    if (notifCount !== "0") break;
    await new Promise((r) => setTimeout(r, 600));
  }
  step("creator-ip-status-notification", notifCount !== "0", `count=${notifCount}`);

  await browser.close();
  const failed = results.filter((r) => r.startsWith("FAIL"));
  console.log(`T52-SUMMARY ${results.length - failed.length}/${results.length}`);
  process.exit(failed.length ? 1 : 0);
} catch (err) {
  console.error("T52-RIG-ERROR", err);
  await browser.close();
  process.exit(1);
}
