// T26 (FIX-33) browser verification rig (authored against the lane-M isolated
// stack: frontend :3205, API :8089; screenshots land in repo-root screenshots/).
// Flow: login admin → /admin/config shows real snake_case values →
// save does not drift → extra_penalty accepts negative (F-115).
import { chromium } from "playwright";

const BASE = "http://localhost:3205";
const API = "http://localhost:8089";
const EMAIL = "lanem-admin@seed.omnicraft.local";
const PASSWORD = "LanemAdmin#2026";
const SHOT = (n) => `screenshots/t26-${n}.png`;

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
  await page.waitForURL(/\/(dashboard|profile|admin|$)/, { timeout: 15000 }).catch(() => {});
  await page.waitForTimeout(1500);
  check("login", !(await page.locator('input[type="email"]').isVisible().catch(() => false)));

  // 2. open admin config
  await page.goto(`${BASE}/admin/config`, { waitUntil: "networkidle" });
  await page.waitForTimeout(2000);
  const saveBtn = page.locator("header button, div").filter({ hasText: /保存|Save/ }).last();
  const minVotes = page.locator("#config-min-votes");
  await minVotes.waitFor({ state: "visible", timeout: 10000 });
  const mv = await minVotes.inputValue();
  check("real value min_votes_required=22 (not default 20/fallback)", mv === "22", `got ${mv}`);
  const pt = await page.locator("#config-verdict-pass-threshold").inputValue();
  check("real value pass_threshold=0.65", pt === "0.65", `got ${pt}`);
  const pen = await page.locator("#config-repeat-violation-penalty").inputValue();
  check("real value extra_penalty=-2 (negative visible)", pen === "-2", `got ${pen}`);
  await page.screenshot({ path: SHOT("1-real-values"), fullPage: true });

  // 3. F-115: min attribute relaxed to allow negatives
  const minAttr = await page.locator("#config-repeat-violation-penalty").getAttribute("min");
  check("F-115 min attr = -10", minAttr === "-10", `got ${minAttr}`);
  await page.fill("#config-repeat-violation-penalty", "-3");
  const invalidCount = await page.locator("[data-config-field]:invalid").count();
  check("negative penalty does not trip :invalid", invalidCount === 0, `invalid fields: ${invalidCount}`);

  // 4. edit min_votes and save; assert no drift after round-trip
  await page.fill("#config-min-votes", "23");
  await page.getByRole("button", { name: /保存|Save/ }).first().click();
  await page.waitForTimeout(800);
  const confirmBtn = page.getByRole("button", { name: /确认|Confirm/ }).last();
  await confirmBtn.click({ timeout: 5000 }).catch(() => console.log("no confirm modal"));
  await page.waitForTimeout(2000);
  const mv2 = await page.locator("#config-min-votes").inputValue();
  const pen2 = await page.locator("#config-repeat-violation-penalty").inputValue();
  check("save no drift (min_votes=23 kept)", mv2 === "23", `got ${mv2}`);
  check("save no drift (penalty=-3 kept)", pen2 === "-3", `got ${pen2}`);
  await page.screenshot({ path: SHOT("2-after-save"), fullPage: true });

  // 5. server truth: fresh GET confirms persisted values
  const resp = await page.evaluate(async (api) => {
    const csrfRes = await fetch(`${api}/api/v1/auth/csrf`, { credentials: "include" });
    const { csrf_token } = await csrfRes.json();
    const res = await fetch(`${api}/api/v1/admin/config`, { credentials: "include" });
    return res.json();
  }, API);
  const j = resp?.config?.judge ?? {};
  const rep = resp?.config?.reputation ?? {};
  check("server truth min_votes_required=23", j.min_votes_required === 23, JSON.stringify(j));
  check("server truth extra_penalty=-3", rep.repeat_violation_extra_penalty === -3, JSON.stringify(rep));
} catch (e) {
  failed++;
  console.log("RIG_ERROR", e.message);
  await page.screenshot({ path: SHOT("0-error"), fullPage: true }).catch(() => {});
} finally {
  await browser.close();
  console.log(`RESULT: ${passed} passed, ${failed} failed`);
  process.exit(failed > 0 ? 1 : 0);
}
