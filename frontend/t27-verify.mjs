// T27 (FIX-34) browser verification rig (authored against the lane-M isolated
// stack: frontend :3205; screenshots land in repo-root screenshots/).
// Flow: login admin → /admin/contents → trash tab → restore content 7301 →
// confirm modal → row leaves trash → content visible on the public page.
import { chromium } from "playwright";

const BASE = "http://localhost:3205";
const EMAIL = "lanem-admin@seed.omnicraft.local";
const PASSWORD = "LanemAdmin#2026";
const SHOT = (n) => `screenshots/t27-${n}.png`;

let passed = 0, failed = 0;
function check(name, cond, extra = "") {
  if (cond) { passed++; console.log(`PASS ${name}`); }
  else { failed++; console.log(`FAIL ${name} ${extra}`); }
}

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 960 } });
const page = await ctx.newPage();

try {
  // 1. login
  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.locator('input[type="email"]').waitFor({ state: "hidden", timeout: 30000 }).catch(() => {});
  check("login", !(await page.locator('input[type="email"]').isVisible().catch(() => false)));

  // 2. admin contents list view
  await page.goto(`${BASE}/admin/contents`, { waitUntil: "domcontentloaded", timeout: 60000 });
  const trashTab = page.getByRole("tab", { name: /回收站|Trash/ });
  await trashTab.waitFor({ state: "visible", timeout: 30000 }).catch(() => {});
  check("trash tab visible", await trashTab.isVisible());
  await page.screenshot({ path: SHOT("1-list-view"), fullPage: true });

  // 3. switch to trash tab — trashed content 7301 must appear
  await trashTab.click();
  await page.waitForTimeout(1500);
  const row = page.locator("tr", { hasText: "回收站测试内容 T27" });
  await row.waitFor({ state: "visible", timeout: 8000 });
  check("trashed content visible in trash view", true);
  const restoreBtn = row.getByRole("button", { name: /恢复|Restore/ });
  check("restore button present", await restoreBtn.isVisible());
  await page.screenshot({ path: SHOT("2-trash-view"), fullPage: true });

  // 4. restore with confirm modal
  await restoreBtn.click();
  await page.waitForTimeout(600);
  const confirmBtn = page.getByRole("button", { name: /确认恢复|Restore/ }).last();
  check("restore confirm modal", await confirmBtn.isVisible());
  await page.screenshot({ path: SHOT("3-restore-modal"), fullPage: true });
  await confirmBtn.click();
  await page.waitForTimeout(2000);
  const rowGone = (await page.locator("tr", { hasText: "回收站测试内容 T27" }).count()) === 0;
  check("row leaves trash after restore", rowGone);
  await page.screenshot({ path: SHOT("4-after-restore"), fullPage: true });

  // 5. content is publicly visible again (status=published + deleted_at NULL)
  await page.goto(`${BASE}/original/7301`, { waitUntil: "domcontentloaded", timeout: 60000 });
  await page.waitForTimeout(2500);
  const bodyText = await page.locator("body").innerText();
  check("restored content publicly visible", bodyText.includes("回收站测试内容 T27"), bodyText.slice(0, 120));
  await page.screenshot({ path: SHOT("5-public-visible"), fullPage: true });
} catch (e) {
  failed++;
  console.log("RIG_ERROR", e.message);
  await page.screenshot({ path: SHOT("0-error"), fullPage: true }).catch(() => {});
} finally {
  await browser.close();
  console.log(`RESULT: ${passed} passed, ${failed} failed`);
  process.exit(failed > 0 ? 1 : 0);
}
