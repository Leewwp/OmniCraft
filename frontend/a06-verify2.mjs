/* A-06 补充验证：复制（带剪贴板权限）+ 暗色主题 + 窄屏抽屉。复用既有 grounded 会话。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const W = zh.agent.workspace;
const SHOTS = path.join(process.cwd(), "..", "screenshots");
const results = [];
const step = (name, ok, detail = "") => {
  results.push(ok);
  console.log(`A06-STEP ${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
};

const browser = await chromium.launch();
try {
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    locale: "zh-CN",
    permissions: ["clipboard-read", "clipboard-write"],
  });
  const page = await context.newPage();

  await page.goto("http://localhost:3000/login");
  await page.fill('input[type="email"]', "a06-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "A06Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL(/agent|recommend|\/$/, { timeout: 15000 }).catch(() => {});

  await page.goto("http://localhost:3000/agent");
  const composer = page.locator(`textarea[aria-label="${W.composerLabel}"]`);
  await composer.waitFor({ state: "visible", timeout: 15000 });

  /* 现场发一条 detail 引导 query（确定性 grounded，~一次工具调用） */
  await composer.fill("请查看 id 为 6 的内容《原创实验｜新手也能完成的配色练习 05》的详情并介绍它");
  await composer.press("Enter");
  const sendBtn = page.locator(`button[aria-label="${W.sendMessage}"]`);
  await sendBtn.waitFor({ state: "visible", timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(800);

  /* 复制最近一条回答 */
  const copyBtn = page.locator(`button[aria-label="${W.copyMessage}"]`).first();
  let copyOk = false;
  let clipText = "";
  if (await copyBtn.isVisible().catch(() => false)) {
    await copyBtn.hover();
    await copyBtn.click();
    try {
      await page.getByText(W.copySuccess).first().waitFor({ state: "visible", timeout: 5000 });
      copyOk = true;
      clipText = await page.evaluate(() => navigator.clipboard.readText().then((t) => t.slice(0, 24)));
    } catch {}
  }
  step("copy-message", copyOk, `clipboard="${clipText}"`);

  /* 暗色主题：走真实 UI（header 主题下拉 → 深色） */
  await page.getByRole("button", { name: zh.nav.themeSwitch }).click();
  await page.getByRole("menuitem", { name: zh.nav.themeDark }).click();
  await page.waitForTimeout(800);
  const isDark = await page.evaluate(() => document.documentElement.classList.contains("dark"));
  await page.screenshot({ path: path.join(SHOTS, "a06-dark-desktop.png") });
  step("dark-theme", isDark);

  /* 窄屏抽屉 + 布局不破 */
  await page.setViewportSize({ width: 375, height: 844 });
  await page.waitForTimeout(600);
  await page.locator(`button[aria-label="${W.openConversations}"]`).click();
  const drawer = page.getByRole("dialog", { name: W.sidebarLabel });
  let drawerOk = false;
  try {
    await drawer.waitFor({ state: "visible", timeout: 5000 });
    drawerOk = true;
  } catch {}
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  await page.screenshot({ path: path.join(SHOTS, "a06-narrow-mobile.png") });
  step("narrow-drawer", drawerOk && overflow <= 0, `overflowX=${overflow}px`);
} finally {
  await browser.close();
}
const failed = results.filter((ok) => !ok).length;
console.log(`A06-SUMMARY ${results.length - failed}/${results.length} passed`);
if (failed) process.exitCode = 1;
