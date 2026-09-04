/* A-06 浏览器全流程验证（无头 Chromium，真实后端 + 真实 LLM 流式）。
 * 证据截图输出到 ../screenshots/a06-*.png；每阶段打印 A06-STEP 结果行。 */
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const require = createRequire(import.meta.url);
const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const W = zh.agent.workspace;
const T = zh.agent.tools;
const C = zh.agent.citations;
const SHOTS = path.join(process.cwd(), "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("A06-STEP " + line);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 登录 */
  await page.goto("http://localhost:3000/login");
  await page.fill('input[type="email"]', "a06-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "A06Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL(/agent|recommend|\/$/, { timeout: 15000 }).catch(() => {});
  step("login", !page.url().includes("login"), page.url());

  /* 工作台空态 */
  await page.goto("http://localhost:3000/agent");
  const composer = page.locator(`textarea[aria-label="${W.composerLabel}"]`);
  await composer.waitFor({ state: "visible", timeout: 15000 });
  const emptyTitle = await page.getByText(W.emptyTitle).isVisible().catch(() => false);
  step("empty-state", emptyTitle, "welcome + suggestions");

  /* 发起真实对话（触发检索 + 真流式） */
  const question = "请查看 id 为 6 的内容《原创实验｜新手也能完成的配色练习 05》的详情并介绍它";
  await composer.fill(question);
  await composer.press("Enter");

  /* 流式观察：正文区域出现 assistant 气泡并增长 */
  const transcript = page.locator('[data-slot="agent-transcript"]');
  await transcript.waitFor({ state: "visible", timeout: 10000 });
  let streamed = false;
  try {
    for (let i = 0; i < 60; i += 1) {
      await page.waitForTimeout(1000);
      const text = (await transcript.innerText()).trim();
      if (text.length > (question.length + 80)) { streamed = true; break; }
    }
  } catch {}
  step("streaming-answer", streamed, "assistant content grew beyond question");

  /* 等待本轮结束（发送按钮回归）再做终态断言，消除与流式的竞态。 */
  const sendBtn = page.locator(`button[aria-label="${W.sendMessage}"]`);
  let turnEnded = false;
  try {
    await sendBtn.waitFor({ state: "visible", timeout: 60000 });
    turnEnded = true;
  } catch {}
  await page.waitForTimeout(800);
  step("turn-ended", turnEnded);

  /* 工具步骤区（可折叠；等待完成）。折叠态摘要 = 「N 个工具步骤」，展开后见步骤文案。 */
  const toolToggle = page.locator(`button[aria-label="${T.title}"]`);
  let toolsOk = false;
  let toolsDetail = "";
  try {
    await toolToggle.first().waitFor({ state: "visible", timeout: 25000 });
    const blockCount = await toolToggle.count();
    if (blockCount > 1) {
      const dup = await page.evaluate((label) => {
        const blocks = [...document.querySelectorAll(`button[aria-label="${label}"]`)];
        return blocks.map((b) => {
          const card = b.closest("div.rounded-md");
          const prev = card?.previousElementSibling?.className ?? "";
          const nextSiblingText = (card?.nextElementSibling?.textContent ?? "").slice(0, 30);
          return { prevClass: prev.slice(0, 40), next: nextSiblingText };
        });
      }, T.title);
      toolsDetail = `DUP count=${blockCount} ${JSON.stringify(dup)}`;
    }
    const first = toolToggle.first();
    const expanded = await first.getAttribute("aria-expanded");
    if (expanded === "false") await first.click();
    await page.waitForTimeout(400);
    const stepRows = await first.locator("xpath=following-sibling::ul/li").count();
    toolsOk = stepRows >= 1;
    toolsDetail += ` expanded=${expanded} steps=${stepRows}`;
  } catch (error) {
    toolsDetail = String(error).slice(0, 100);
    await page.screenshot({ path: path.join(SHOTS, "a06-debug-tools.png") }).catch(() => {});
  }
  step("tool-steps-collapsible", toolsOk, toolsDetail);

  /* 引用列表（真跑 grounded 需引用候选非空；当前本地检索域 no_evidence 属既有状态） */
  const citationsHeading = page.getByRole("heading", { name: C.title });
  let citationsOk = false;
  try {
    await citationsHeading.waitFor({ state: "visible", timeout: 15000 });
    citationsOk = true;
  } catch {}
  step("citations-list", citationsOk, citationsOk ? "grounded turn" : "no_evidence turn（检索域既有状态，展示层由 mock e2e 覆盖）");

  /* no_evidence 拒答卡（行为正确性证据）：grounded 轮无拒答卡同样是正确行为。 */
  const noEvidenceCard = page.getByText(zh.agent.noEvidence.title);
  if (await noEvidenceCard.first().isVisible().catch(() => false)) {
    await page.screenshot({ path: path.join(SHOTS, "a06-no-evidence.png") });
    step("no-evidence-refusal", true, "refusal card rendered, no fabricated answer");
  } else {
    step("no-evidence-refusal", citationsOk, citationsOk ? "grounded turn（无拒答卡为正确）" : "refusal card not found");
  }

  await page.screenshot({ path: path.join(SHOTS, "a06-grounded-desktop.png"), fullPage: false });

  /* 行内 [1] 锚定 → 引用卡高亮聚焦 */
  const anchor = page.locator('button[aria-label^="' + zh.markdown.citationJump.split(" ")[0] + '"]').first();
  let anchorOk = false;
  if (await anchor.isVisible().catch(() => false)) {
    await anchor.click();
    await page.waitForTimeout(400);
    const focused = await page.evaluate(() => document.activeElement?.id || "");
    const card = page.locator("#agent-citation-0");
    const ring = await card.evaluate((el) => el.className.includes("ring-2")).catch(() => false);
    anchorOk = focused === "agent-citation-0" && ring;
    step("inline-anchor", anchorOk, `focus=${focused} ring=${ring}`);
    await page.screenshot({ path: path.join(SHOTS, "a06-citation-anchor.png") });
  } else {
    step("inline-anchor", false, "model did not emit [1] in this turn (display layer covered by mock e2e)");
  }

  /* 引用卡 → 内容详情浮层 */
  const card0 = page.locator("#agent-citation-0");
  let overlayOk = false;
  let overlayDetail = "";
  if (await card0.isVisible().catch(() => false)) {
    await card0.click();
    try {
      await page.getByRole("dialog").waitFor({ state: "visible", timeout: 12000 });
      overlayOk = true;
      await page.screenshot({ path: path.join(SHOTS, "a06-citation-overlay.png") });
      await page.keyboard.press("Escape");
      await page.getByRole("dialog").waitFor({ state: "hidden", timeout: 5000 }).catch(() => {});
    } catch (error) {
      overlayDetail = String(error).slice(0, 100);
      await page.screenshot({ path: path.join(SHOTS, "a06-debug-overlay.png") }).catch(() => {});
    }
  } else {
    overlayDetail = "card not visible";
  }
  step("citation-overlay", overlayOk, overlayDetail);

  /* 消息操作：复制（no_evidence 轮无 answer 行则跳过为合理） */
  const copyBtn = page.locator(`button[aria-label="${W.copyMessage}"]`).first();
  let copyOk = false;
  let copyDetail = "";
  if (await copyBtn.isVisible().catch(() => false)) {
    await copyBtn.hover();
    await copyBtn.click();
    try {
      await page.getByText(W.copySuccess).first().waitFor({ state: "visible", timeout: 5000 });
      copyOk = true;
    } catch {}
  } else {
    copyDetail = "no answer row (no_evidence turn) — copy covered by mock e2e";
  }
  step("copy-message", copyOk, copyDetail);

  /* 侧边栏 ⋯ 菜单：截图 + 重命名 + 置顶 */
  const menuBtn = page.locator(`button[aria-label="${W.menuLabel}"]`).first();
  let menuOk = false;
  try {
    await menuBtn.waitFor({ state: "visible", timeout: 5000 });
    await menuBtn.hover();
    await menuBtn.click();
    await page.getByRole("menuitem", { name: W.menuRename }).waitFor({ state: "visible", timeout: 5000 });
    menuOk = true;
    await page.screenshot({ path: path.join(SHOTS, "a06-sidebar-menu.png") });
    /* 重命名 */
    await page.getByRole("menuitem", { name: W.menuRename }).click();
    const renameInput = page.locator('input[aria-label*="重命名输入框"]').first();
    await renameInput.waitFor({ state: "visible", timeout: 5000 });
    await renameInput.fill("A06 验证会话");
    await renameInput.press("Enter");
    let renamed = false;
    try {
      await page.getByText("A06 验证会话").first().waitFor({ state: "visible", timeout: 5000 });
      renamed = true;
    } catch {}
    step("sidebar-rename", renamed);

    /* 置顶 */
    await menuBtn.click();
    await page.getByRole("menuitem", { name: W.menuPin }).waitFor({ state: "visible", timeout: 5000 });
    await page.getByRole("menuitem", { name: W.menuPin }).click();
    await page.waitForTimeout(600);
    const pinned = await page.getByText(W.groupPinned).isVisible().catch(() => false);
    step("sidebar-pin", pinned);
    await page.screenshot({ path: path.join(SHOTS, "a06-pinned-group.png") });

    /* 取消置顶（复原，便于重复验证） */
    await menuBtn.click();
    await page.getByRole("menuitem", { name: W.menuUnpin }).waitFor({ state: "visible", timeout: 5000 });
    await page.getByRole("menuitem", { name: W.menuUnpin }).click();
    await page.waitForTimeout(400);
  } catch (error) {
    step("sidebar-menu", false, String(error).slice(0, 120));
  }
  if (menuOk) step("sidebar-menu-open", true);

  /* 输入自动增高：真实 fill 走 React 受控路径后量高度。 */
  let grows = null;
  try {
    const before = await composer.evaluate((el) => el.offsetHeight);
    await composer.fill("第一行\n第二行\n第三行\n第四行\n第五行\n第六行\n第七行\n第八行\n第九行\n第十行");
    await page.waitForTimeout(400);
    grows = await composer.evaluate((el, beforeHeight) => ({ before: beforeHeight, after: el.offsetHeight, capped: el.offsetHeight <= 208 }), before);
  } catch (error) {
    console.log("A06-DEBUG autogrow error:", String(error).slice(0, 160));
  }
  const growOk = grows ? grows.after > grows.before + 60 && grows.capped : false;
  step("composer-autogrow", growOk, JSON.stringify(grows));
  await composer.fill("");

  /* 侧边栏 ⋯ 菜单：删除会话（ConfirmModal 确认） */
  let deleteOk = false;
  try {
    await menuBtn.first().click();
    await page.getByRole("menuitem", { name: W.menuDelete }).waitFor({ state: "visible", timeout: 5000 });
    await page.getByRole("menuitem", { name: W.menuDelete }).click();
    await page.getByText(W.deleteConfirmTitle).waitFor({ state: "visible", timeout: 5000 });
    /* 先验证取消不发请求 */
    await page.getByRole("button", { name: zh.common.cancel ?? "取消" }).click();
    const stillThere = await page.getByText("A06 验证会话").first().isVisible().catch(() => false);
    await menuBtn.first().click();
    await page.getByRole("menuitem", { name: W.menuDelete }).click();
    const confirmBtn = page.locator(`button:has-text("${W.deleteConfirmAction}")`).last();
    await confirmBtn.waitFor({ state: "visible", timeout: 5000 });
    await confirmBtn.click();
    await page.getByText(W.deleteSuccess).first().waitFor({ state: "visible", timeout: 5000 });
    let gone = false;
    try {
      await page.locator(`aside[aria-label="${W.sidebarLabel}"]`).getByText("A06 验证会话").first().waitFor({ state: "hidden", timeout: 8000 });
      gone = true;
    } catch {}
    deleteOk = gone && stillThere;
  } catch (error) {
    step("sidebar-delete-detail", false, String(error).slice(0, 160));
  }
  step("sidebar-delete", deleteOk, "cancel keeps, confirm deletes");

  /* 暗色主题截图（复用主页面：设 localStorage 后 reload，避免新页鉴权变量） */
  await page.evaluate(() => localStorage.setItem("theme", '"dark"'));
  await page.reload();
  await composer.waitFor({ state: "visible", timeout: 15000 });
  await page.waitForTimeout(800);
  const isDark = await page.evaluate(() => document.documentElement.classList.contains("dark"));
  await page.screenshot({ path: path.join(SHOTS, "a06-dark-desktop.png") });
  step("dark-theme", isDark);

  /* 窄屏：抽屉 + 布局不破（同页改视口） */
  await page.setViewportSize({ width: 375, height: 844 });
  await page.waitForTimeout(500);
  await page.locator(`button[aria-label="${W.openConversations}"]`).click();
  const drawerVisible = await page.locator(`[role="dialog"][aria-label="${W.sidebarLabel}"]`).isVisible().catch(() => false);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  await page.screenshot({ path: path.join(SHOTS, "a06-narrow-mobile.png") });
  step("narrow-drawer", drawerVisible, `overflowX=${overflow}px`);
  await page.evaluate(() => localStorage.setItem("theme", '"light"'));
} finally {
  await browser.close();
}

const failed = results.filter((line) => line.startsWith("FAIL"));
console.log(`A06-SUMMARY ${results.length - failed.length}/${results.length} passed`);
if (failed.length) {
  console.log("A06-FAILED-LINES:\n" + failed.join("\n"));
  process.exitCode = 1;
}
