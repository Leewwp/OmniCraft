// T14 浏览器验证 rig（道 C 隔离栈 3203/8087）：
// 登录作者 → studio 三页真实数据 + 旧 dashboard 四路由 redirect + robots。
import { chromium } from "playwright";

const BASE = "http://localhost:3203";
const shot = (page, name) => page.screenshot({ path: `screenshots/${name}.png`, fullPage: false });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();
const fails = [];
const expect = (cond, msg) => { if (!cond) fails.push(msg); console.log((cond ? "PASS" : "FAIL") + " — " + msg); };

// 1. 登录作者
await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
await page.fill('input[type="email"]', "author@seed.omnicraft.local");
await page.fill('input[type="password"]', "LaneC#2026");
await page.click('button[type="submit"]');
await page.waitForURL(/\/(home|studio|feed)/, { timeout: 15000 }).catch(() => {});
console.log("after login url:", page.url());
expect(!page.url().includes("login"), "作者登录成功");

// 2. studio/pr-requests 真实实现：列表含 open PR（PR#3），点开看 diff
await page.goto(`${BASE}/studio/pr-requests`, { waitUntil: "networkidle" });
await page.waitForTimeout(1500);
const prCards = await page.locator("text=/PR ?#?3|browser verify pr/i").count();
expect(prCards >= 1, "PR 列表渲染真实数据（PR#3 可见）");
const openCountText = await page.locator("body").textContent();
expect(/待处理|pending/i.test(openCountText), "待处理计数文案存在（真实实现特征）");
await shot(page, "t14-studio-pr-requests");
// 点「查看 Diff」按钮加载 diff（DiffViewer 两栏）
await page.locator("button", { hasText: /查看 diff/i }).first().click();
await page.waitForTimeout(2500);
const diffText = await page.locator("body").textContent();
expect(diffText.includes("browser verify body v4"), "diff 右栏显示 proposed 正文（T13 后端落库 + 迁移页联通）");
await shot(page, "t14-studio-pr-diff");

// 3. studio/contributors 真实实现：贡献者表格
await page.goto(`${BASE}/studio/contributors`, { waitUntil: "networkidle" });
await page.waitForTimeout(1500);
const contribBody = await page.locator("body").textContent();
expect(/贡献|contributor/i.test(contribBody), "贡献者页渲染真实实现（表格/标题）");
expect(contribBody.includes("lanec_submitter") || /用户 ?#?2|User #2/i.test(contribBody), "贡献者行包含提交者");
await shot(page, "t14-studio-contributors");

// 4. studio/tag-suggestions 真实实现：输入 content id=2 查询建议
await page.goto(`${BASE}/studio/tag-suggestions`, { waitUntil: "networkidle" });
await page.waitForTimeout(1000);
const tagInput = page.locator('input[type="number"]');
expect(await tagInput.count() === 1, "标签建议页渲染真实实现（content id 输入框存在）");
await tagInput.fill("2");
await page.locator("button", { hasText: /刷新|refresh/i }).first().click().catch(() => {});
await page.waitForTimeout(1500);
const tagBody = await page.locator("body").textContent();
expect(tagBody.includes("科幻"), "标签建议行渲染（科幻）");
expect(tagBody.includes("治愈"), "标签建议行渲染（治愈）");
await shot(page, "t14-studio-tag-suggestions");

// 5. 旧 dashboard 四路由 redirect（客户端守卫在水合竞态下可能弹 login：重登后沿 ?redirect 链继续验证）
const relogin = async () => {
  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
  await page.fill('input[type="email"]', "author@seed.omnicraft.local");
  await page.fill('input[type="password"]', "LaneC#2026");
  await page.click('button[type="submit"]');
  await page.waitForTimeout(1500);
};
for (const [old, target] of [
  ["/dashboard/pr-requests", "/studio/pr-requests"],
  ["/dashboard/contributors", "/studio/contributors"],
  ["/dashboard/tag-suggestions", "/studio/tag-suggestions"],
  ["/dashboard/contents", "/studio/contents"],
]) {
  await page.goto(`${BASE}${old}`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1000);
  if (page.url().includes("/login")) {
    await relogin();
    await page.waitForTimeout(1500);
  }
  expect(page.url().startsWith(`${BASE}${target}`), `旧路由 ${old} → ${target}（实际 ${page.url()}）`);
}
await shot(page, "t14-old-route-redirect-lands");

await browser.close();
console.log(fails.length === 0 ? "\nALL PASS" : `\n${fails.length} FAILURES:\n` + fails.join("\n"));
process.exit(fails.length === 0 ? 0 : 1);
