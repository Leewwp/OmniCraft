// T28 (FIX-35) browser verification rig (authored against the lane-M isolated
// stack: frontend :3205; screenshots land in repo-root screenshots/).
// Flow: login admin → /admin/users search across pages → /admin/queue DLQ card
// real topic/attempts → replay with ConfirmModal → status feedback (entry kept)
// → /admin/dashboard no partial-failure banner on the happy path.
import { chromium } from "playwright";

const BASE = "http://localhost:3205";
const EMAIL = "lanem-admin@seed.omnicraft.local";
const PASSWORD = "LanemAdmin#2026";
const SHOT = (n) => `screenshots/t28-${n}.png`;

let passed = 0, failed = 0;
function check(name, cond, extra = "") {
  if (cond) { passed++; console.log(`PASS ${name}`); }
  else { failed++; console.log(`FAIL ${name} ${extra}`); }
}

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 960 } });
const page = await ctx.newPage();

// 全页导航会触发前端 app 初始化的并发 refresh token rotation race（现有系统
// 行为，非 T28 引入）：偶发掉回 /login。掉线则重新登录后重试目标页。
async function gotoAdmin(path) {
  await page.goto(`${BASE}${path}`, { waitUntil: "domcontentloaded", timeout: 60000 });
  if (page.url().includes("/login")) {
    await page.fill('input[type="email"]', EMAIL);
    await page.fill('input[type="password"]', PASSWORD);
    await page.click('button[type="submit"]');
    await page.locator('input[type="email"]').waitFor({ state: "hidden", timeout: 30000 }).catch(() => {});
    await page.goto(`${BASE}${path}`, { waitUntil: "domcontentloaded", timeout: 60000 });
  }
}

try {
  // 1. login
  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.locator('input[type="email"]').waitFor({ state: "hidden", timeout: 30000 }).catch(() => {});
  check("login", !(await page.locator('input[type="email"]').isVisible().catch(() => false)));

  // 2. users: default first page is id DESC (top 20 of 25) — id 3 user must be absent
  await page.goto(`${BASE}/admin/users`, { waitUntil: "domcontentloaded", timeout: 60000 });
  await page.locator("tbody tr").first().waitFor({ state: "visible", timeout: 30000 });
  const firstPageText = await page.locator("tbody").innerText();
  check("default first page excludes deep user (id DESC pagination)", !firstPageText.includes("t28rig-user-01"), firstPageText.slice(0, 80));
  await page.screenshot({ path: SHOT("1-users-default"), fullPage: true });

  // 3. server-side search hits the deep user across pages
  await page.fill('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="text"]', "t28rig-user-01");
  await page.waitForTimeout(1800); // debounce 400ms + fetch
  const rows = page.locator("tbody tr");
  const rowCount = await rows.count();
  const rowText = rowCount > 0 ? await rows.first().innerText() : "";
  check("search across pages hits deep user", rowCount === 1 && rowText.includes("t28rig-user-01"), `rows=${rowCount}`);
  await page.screenshot({ path: SHOT("2-users-search-hit"), fullPage: true });

  // 4. queue: DLQ card shows real topic + retries from backend contract (F-113)
  await gotoAdmin("/admin/queue");
  const dlqCard = page.locator("div", { has: page.getByText("notification.create") }).last();
  await page.getByRole("button", { name: /重放|Replay/ }).first().waitFor({ state: "visible", timeout: 30000 });
  const dlqText = await page.locator("body").innerText();
  check("DLQ card shows real original_topic", dlqText.includes("notification.create"));
  check("DLQ card shows real retry count", /重试\s*3\s*次|3\s*retries/i.test(dlqText), dlqText.slice(dlqText.indexOf("死信") , dlqText.indexOf("死信") + 200));
  await page.screenshot({ path: SHOT("3-queue-dlq-card"), fullPage: true });

  // 5. replay with ConfirmModal; entry stays (re-deliver, not delete)
  await page.getByRole("button", { name: /重放|Replay/ }).first().click();
  await page.waitForTimeout(600);
  const modalText = await page.locator("body").innerText();
  check("replay confirm modal visible", modalText.includes("重放死信条目") || modalText.includes("Replay dead-letter entry"));
  await page.screenshot({ path: SHOT("4-replay-modal"), fullPage: true });
  await page.getByRole("button", { name: /^重放$|^Replay$/ }).last().click();
  await page.locator('[role="status"]').waitFor({ state: "visible", timeout: 15000 });
  const statusText = await page.locator('[role="status"]').innerText();
  check("replay success status shown", statusText.includes("已重投") || statusText.includes("re-delivered"), statusText);
  await page.waitForTimeout(1200); // list refresh
  const afterReplay = await page.locator("body").innerText();
  check("DLQ entry kept after replay (re-deliver semantics)", afterReplay.includes("notification.create") && afterReplay.includes("simulated failure"));
  await page.screenshot({ path: SHOT("5-after-replay"), fullPage: true });

  // 6. dashboard happy path: no partial-failure banner
  await gotoAdmin("/admin/dashboard");
  await page.waitForTimeout(2500);
  const dashText = await page.locator("body").innerText();
  check("dashboard has no partial-failure banner on happy path", !dashText.includes("部分仪表盘数据加载失败") && !dashText.includes("failed to load"), dashText.slice(0, 120));
  await page.screenshot({ path: SHOT("6-dashboard"), fullPage: true });
} catch (e) {
  failed++;
  console.log("RIG_ERROR", e.message);
  await page.screenshot({ path: SHOT("0-error"), fullPage: true }).catch(() => {});
} finally {
  await browser.close();
  console.log(`RESULT: ${passed} passed, ${failed} failed`);
  process.exit(failed > 0 ? 1 : 0);
}
