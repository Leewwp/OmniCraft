// T12 浏览器验证 rig（道 S 隔离栈 3207/8094）：
// 1) norm 单 context：发帖（统一审核门 201 回归）→ ?d= 直开浮层 → 回复（content 字段映射）；
// 2) low：受保护 new 页提交 → 后端 403 → 用户文案提示 + 无落库。
// 登录限流 5/min/IP：全 rig 只登两次，每次登录后断言成功。
import { chromium } from "playwright";

const BASE = "http://localhost:3207";
const API = "http://localhost:8094";
const shot = (page, name) => page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });
const fails = [];
const expect = (cond, msg) => { if (!cond) fails.push(msg); console.log((cond ? "PASS" : "FAIL") + " — " + msg); };

const browser = await chromium.launch();

async function login(page, email, password) {
  await page.goto(`${BASE}/login`, { waitUntil: "domcontentloaded" });
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);
  await page.click('button[type="submit"]');
  await page.waitForFunction(() => !location.pathname.includes("login"), { timeout: 20000 }).catch(() => {});
  await page.waitForTimeout(2500); // AuthContext refresh+me
  return !page.url().includes("login");
}

// —— 场景 3：low 提交 → 403 用户文案 + 无落库 ——
{
  console.log("waiting 90s for login rate-limit window...");
  await new Promise(r => setTimeout(r, 90000));
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  const loggedIn = await login(page, "lane-s-low@seed.omnicraft.local", "LaneS-Low#2026");
  expect(loggedIn, "low 登录成功（me 200）");

  const beforeTotal = await page.evaluate(async () => {
    const r = await fetch("http://localhost:8094/api/v1/ips/4200/discussions?page=1&page_size=1");
    const d = await r.json();
    return d.total;
  });
  await page.goto(`${BASE}/ip/4200/discussions/new`, { waitUntil: "domcontentloaded" });
  await page.fill("#discussion-title", "低信誉前端直发帖");
  await page.fill("#discussion-body", "rep=1 经前端提交应被后端 403 拦截。");
  await page.click('button[type="submit"]');
  const alertEl = page.locator('p[role="alert"]').first();
  await alertEl.waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
  const alertText = await alertEl.textContent().catch(() => "");
  expect(!!alertText && alertText.trim().length > 0, "提交后出现用户可读错误提示: " + JSON.stringify(alertText));
  // 落库复核：讨论总数不变（后端真的拦了）
  const total = await page.evaluate(async () => {
    const r = await fetch("http://localhost:8094/api/v1/ips/4200/discussions?page=1&page_size=1");
    const d = await r.json();
    return d.total;
  });
  expect(total === beforeTotal, `讨论总数不变（后端拦截落库）before=${beforeTotal} after=${total}`);
  await shot(page, "t12-low-denied");
  await ctx.close();
  globalThis.__t12TotalAfter = total;
}

await browser.close();
console.log(fails.length === 0 ? "ALL PASS" : `FAILURES: ${fails.length}`);
process.exit(fails.length === 0 ? 0 : 1);
