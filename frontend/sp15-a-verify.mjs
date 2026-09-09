/* SP-15 #433 A 票浏览器三场景验证（无头 Chromium，真实后端 + 真实 LLM）。
 * 场景 ①「你好」→ 规则层模板回应（conversational，零 LLM）
 * 场景 ② 乱码 → 澄清反问保留（conversational，不走 no_evidence 卡）
 * 场景 ③ 内容题 → 引用回答不变（grounded_content）
 * 证据截图输出到 ../screenshots/sp15-a-*.png。 */
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const require = createRequire(import.meta.url);
const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const W = zh.agent.workspace;
const SHOTS = path.join(process.cwd(), "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("SP15A-STEP " + line);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  await page.goto("http://localhost:3000/login");
  await page.fill('input[type="email"]', "a06-verify@seed.omnicraft.local");
  await page.fill('input[type="password"]', "A06Verify#2026");
  await page.click('button[type="submit"]');
  await page.waitForURL(/agent|recommend|\/$/, { timeout: 15000 }).catch(() => {});
  step("login", !page.url().includes("login"), page.url());

  await page.goto("http://localhost:3000/agent");
  const composer = page.locator(`textarea[aria-label="${W.composerLabel}"]`);
  await composer.waitFor({ state: "visible", timeout: 15000 });

  const sendBtn = page.locator(`button[aria-label="${W.sendMessage}"]`);
  const noEvidenceTitle = zh.agent.noEvidence.title;
  async function turnEnded(timeout = 90000) {
    await sendBtn.waitFor({ state: "visible", timeout });
    await page.waitForTimeout(600);
  }
  async function cardShown() {
    return page.getByText(noEvidenceTitle, { exact: false }).isVisible().catch(() => false);
  }

  /* 场景 ①：规则层闲聊短路 —— 「你好」应即时得到模板回应 */
  const chitchatStarted = Date.now();
  await composer.fill("你好");
  await composer.press("Enter");
  const templateLocator = page.getByText("OmniCraft Agent", { exact: false });
  await templateLocator.first().waitFor({ state: "visible", timeout: 15000 });
  await turnEnded(30000);
  const chitchatMs = Date.now() - chitchatStarted;
  step("s1-chitchat-template", true, `template visible in ${chitchatMs}ms`);
  step("s1-no-rejection-card", !(await cardShown()), "conversational turn must not show the no-evidence card");
  await page.screenshot({ path: path.join(SHOTS, "sp15-a-1-chitchat.png"), fullPage: false });

  /* 场景 ②：乱码 → 模型层澄清反问（conversational 保留，不清空） */
  await composer.fill("￥%……&*kjsadf？？");
  await composer.press("Enter");
  await turnEnded();
  const s2card = await cardShown();
  const s2text = await page.locator('[data-slot="agent-transcript"]').innerText().catch(() => "");
  const s2tail = s2text.split("￥%……&*kjsadf？？").pop() || "";
  const clarified = !s2card && s2tail.trim().length >= 6;
  step("s2-garbled-clarification", clarified,
    `card=${s2card} reply=${JSON.stringify(s2tail.trim().slice(0, 60))}`);
  await page.screenshot({ path: path.join(SHOTS, "sp15-a-2-garbled.png"), fullPage: false });

  /* 场景 ③：内容题 → 引用回答（grounded 契约不变，走 search_content 检索面） */
  const question = "站内有哪些关于配色练习的原创内容？请介绍一下";
  await composer.fill(question);
  await composer.press("Enter");
  await turnEnded();
  const s3card = await cardShown();
  const s3text = await page.locator('[data-slot="agent-transcript"]').innerText().catch(() => "");
  const s3tail = s3text.split(question).pop() || "";
  const cited = await page.getByText(zh.agent.citations.title, { exact: false }).isVisible().catch(() => false);
  const s3ok = !s3card && s3tail.trim().length > 40 && cited;
  step("s3-grounded-cited", s3ok, `card=${s3card} answerLen=${s3tail.trim().length} citationsBlock=${cited}`);
  await page.screenshot({ path: path.join(SHOTS, "sp15-a-3-grounded.png"), fullPage: false });
} catch (err) {
  step("rig-error", false, String(err));
} finally {
  await browser.close();
}
const failed = results.filter((l) => l.startsWith("FAIL"));
console.log(failed.length ? `SP15A-RESULT FAIL (${failed.length}/${results.length})` : "SP15A-RESULT PASS");
process.exit(failed.length ? 1 : 0);
