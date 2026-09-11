/* SP-15 #433 回退门收紧后定点复验：两条曾逃逸的标题式查询必须先检索（出现工具步骤），
 * 乱码仍应即时澄清（零工具）。结果行 SP15T-*；截图 ../screenshots/sp15-t-*.png。 */
import { createRequire } from "node:module";
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const require = createRequire(import.meta.url);
const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const W = zh.agent.workspace;
const T = zh.agent.tools;
const SHOTS = path.join(process.cwd(), "..", "screenshots");

function step(name, ok, detail = "") {
  console.log(`SP15T-STEP ${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`);
  if (!ok) process.exitCode = 1;
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

  await page.goto("http://localhost:3000/agent");
  const composer = page.locator(`textarea[aria-label="${W.composerLabel}"]`);
  await composer.waitFor({ state: "visible", timeout: 15000 });
  const sendBtn = page.locator(`button[aria-label="${W.sendMessage}"]`);

  /* 场景 A/B：标题式查询必须先检索（工具步骤出现），不得即时澄清 */
  const titleQueries = ["鼬的乌鸦停在碑前", "九月没有来信"];
  for (let i = 0; i < titleQueries.length; i += 1) {
    const q = titleQueries[i];
    await composer.fill(q);
    await composer.press("Enter");
    await sendBtn.waitFor({ state: "visible", timeout: 90000 });
    await page.waitForTimeout(600);
    const toolBlock = page.locator(`button[aria-label="${T.title}"], [aria-label="${T.title}"]`).first();
    const hasTools = await toolBlock.isVisible().catch(() => false);
    step(`title-query-${i + 1}-searches-first`, hasTools, `q=${q} toolSteps=${hasTools}`);
    await page.screenshot({ path: path.join(SHOTS, `sp15-t-${i + 1}-title-search.png`), fullPage: false });
  }

  /* 场景 C：乱码仍即时澄清（零工具步骤） */
  await composer.fill("￥%……&*zzxhw");
  await composer.press("Enter");
  await sendBtn.waitFor({ state: "visible", timeout: 90000 });
  await page.waitForTimeout(600);
  const garbledToolBlock = page.locator(`button[aria-label="${T.title}"], [aria-label="${T.title}"]`).first();
  const garbledNewTools = await garbledToolBlock.count();
  const transcript = await page.locator('[data-slot="agent-transcript"]').innerText().catch(() => "");
  const tail = transcript.split("￥%……&*zzxhw").pop() || "";
  step("garbled-still-clarifies", garbledNewTools === 0 && tail.trim().length >= 6, `reply=${JSON.stringify(tail.trim().slice(0, 50))}`);
  await page.screenshot({ path: path.join(SHOTS, "sp15-t-3-garbled.png"), fullPage: false });
} catch (err) {
  step("rig-error", false, String(err));
} finally {
  await browser.close();
}
