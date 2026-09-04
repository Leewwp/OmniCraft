/* A-07 浏览器全流程验证（无头 Chromium，隔离栈 3100/8083）。
 * 证据截图输出到 ../screenshots/a07-*.png；每阶段打印 A07-STEP 结果行。
 * rig 本身不进 PR（与 a06-verify*.mjs 同惯例）。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const BASE = process.env.A07_BASE_URL || "http://localhost:3100";
const SHOTS = path.join(process.cwd(), "..", "screenshots");
const AGENT_ENTRY_CTA = zh.agent.searchAgentEntryCta;
/* 同屏存在 Header 搜索框与搜索页输入框两个 combobox，按 aria-label 精确定位后者。 */
const PAGE_SEARCH_NAME = zh.agent.searchKeywordPlaceholder;

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("A07-STEP " + line);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 1) 匿名搜索页：keyword-only、无 Agent 入口、无模式切换 */
  await page.goto(`${BASE}/search`);
  const searchbox = page.getByRole("combobox", { name: PAGE_SEARCH_NAME });
  await searchbox.waitFor({ state: "visible", timeout: 20000 });
  const entryCount = await page.getByRole("link", { name: AGENT_ENTRY_CTA }).count();
  step("anonymous-no-agent-entry", entryCount === 0, `entry count=${entryCount}`);
  const modeToggle = await page.getByText(zh.agent.aiSearch ?? "AI 搜索").count();
  step("anonymous-no-mode-toggle", modeToggle === 0, `legacy toggle count=${modeToggle}`);
  await page.screenshot({ path: path.join(SHOTS, "a07-search-anonymous.png"), fullPage: false });

  /* 2) 建议下拉（T21 语义保持：中缀命中） */
  await searchbox.fill("镜头");
  const listbox = page.getByRole("listbox");
  await listbox.waitFor({ state: "visible", timeout: 8000 });
  const options = await listbox.getByRole("option").allInnerTexts();
  step("suggestions-infix", options.length >= 2, options.join(" | "));
  await page.screenshot({ path: path.join(SHOTS, "a07-search-suggestions.png") });

  /* 3) 关键词提交 → 内容结果（顶部搜索栏关键词职责不变）。
   * 断言以页面正文是否渲染出 3 张已知卡片为准（卡片标题未必都含检索词）。 */
  await searchbox.press("Enter");
  const expectedTitles = ["镜头感构图指南", "原创实验｜新手也能完成的配色练习 05", "配色练习镜头笔记"];
  let rendered = [];
  for (let i = 0; i < 20; i += 1) {
    await page.waitForTimeout(500);
    const bodyText = await page.locator("body").innerText();
    rendered = expectedTitles.filter((title) => bodyText.includes(title));
    if (rendered.length >= 3) break;
  }
  step("keyword-results", rendered.length >= 3, rendered.join(" / "));
  const stillOnSearch = page.url().includes("/search");
  step("stays-on-search", stillOnSearch, page.url());
  await page.screenshot({ path: path.join(SHOTS, "a07-search-results.png"), fullPage: false });

  /* 4) 登录（verified 用户） */
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "a07-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "A07Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL(/recommend|\/$|search/, { timeout: 20000 }).catch(() => {});
  step("login", !page.url().includes("login"), page.url());

  /* 5) 登录态搜索页：Agent 入口存在 + href 随 query 变化 */
  await page.goto(`${BASE}/search`);
  const entry = page.getByRole("link", { name: AGENT_ENTRY_CTA });
  await entry.waitFor({ state: "visible", timeout: 15000 });
  let href = await entry.getAttribute("href");
  step("entry-default-href", href === "/agent", `href=${href}`);
  const box = page.getByRole("combobox", { name: PAGE_SEARCH_NAME });
  await box.fill("新手 家具");
  await page.waitForTimeout(300);
  href = await entry.getAttribute("href");
  const expected = `/agent?q=${encodeURIComponent("新手 家具")}`;
  step("entry-carries-query", href === expected, `href=${href} expected=${expected}`);
  await page.screenshot({ path: path.join(SHOTS, "a07-search-agent-entry.png") });

  /* 6) 点击入口 → /agent?q= 预填 composer（与新工作台一致体验） */
  await entry.click();
  await page.waitForURL(/\/agent/, { timeout: 15000 });
  const composer = page.locator(`textarea[aria-label="${zh.agent.workspace.composerLabel}"]`);
  await composer.waitFor({ state: "visible", timeout: 20000 });
  const prefilled = await composer.inputValue();
  step("workspace-prefilled", prefilled === "新手 家具", `composer="${prefilled}"`);
  await page.screenshot({ path: path.join(SHOTS, "a07-agent-prefilled.png"), fullPage: false });

  /* 7) no_evidence 一致展示：站外无依据问题 → 工作台 no-evidence 块 + 关键词 CTA 回 /search?q= */
  await composer.fill("珠穆朗玛峰最新测量的海拔是多少米");
  await composer.press("Enter");
  const noEvidenceTitle = zh.agent.noEvidence.title;
  let sawNoEvidence = false;
  let sawDegraded = false;
  try {
    for (let i = 0; i < 90; i += 1) {
      await page.waitForTimeout(1000);
      sawNoEvidence = await page.getByText(noEvidenceTitle).first().isVisible().catch(() => false);
      sawDegraded = await page.getByText(zh.agent.degraded.title).first().isVisible().catch(() => false);
      if (sawNoEvidence || sawDegraded) break;
    }
  } catch {}
  step("no-evidence-or-degraded-shown", sawNoEvidence || sawDegraded, sawNoEvidence ? "no_evidence" : sawDegraded ? "degraded" : "timeout");
  if (sawNoEvidence || sawDegraded) {
    const cta = page.getByRole("link", { name: zh.agent.noEvidence.searchCta }).first();
    const ctaHref = await cta.getAttribute("href").catch(() => null);
    step("fallback-cta-back-to-search", !!ctaHref && ctaHref.startsWith("/search?q="), `cta=${ctaHref}`);
    await page.screenshot({ path: path.join(SHOTS, "a07-agent-no-evidence.png"), fullPage: false });
  }

  /* 8) 移动视口：搜索页入口可达 */
  const mobile = await context.newPage();
  await mobile.setViewportSize({ width: 375, height: 812 });
  await mobile.goto(`${BASE}/search`);
  await mobile.getByRole("link", { name: AGENT_ENTRY_CTA }).waitFor({ state: "visible", timeout: 15000 });
  step("mobile-entry-visible", true);
  await mobile.screenshot({ path: path.join(SHOTS, "a07-search-mobile.png"), fullPage: false });
  await mobile.close();
} finally {
  await browser.close();
  const failed = results.filter((line) => line.startsWith("FAIL"));
  console.log(`A07-SUMMARY pass=${results.length - failed.length} fail=${failed.length}`);
  if (failed.length) process.exitCode = 1;
}
