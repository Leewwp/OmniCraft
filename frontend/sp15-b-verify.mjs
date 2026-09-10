/* SP-15 #435 B 票浏览器验证（无头 Chromium，真实后端 + 真实 LLM）。
 * ① grounded 轮答案下方渲染推荐追问药丸（group 名来自真实 zh catalog）
 * ② 点击 chip = 填入 composer 且不自动发送（无新用户气泡、无第二次流请求）
 * 证据截图输出到 ../screenshots/sp15-b-1-chips.png。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const W = zh.agent.workspace;
const SHOTS = path.join(process.cwd(), "..", "screenshots");

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("SP15B-STEP " + line);
}

async function main() {
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
    if (page.url().includes("login")) throw new Error("ui login failed");

    await page.goto("http://localhost:3000/agent");
    const composer = page.locator(`textarea[aria-label="${W.composerLabel}"]`);
    await composer.waitFor({ state: "visible", timeout: 15000 });
    const sendBtn = page.locator(`button[aria-label="${W.sendMessage}"]`);

    /* grounded 轮 ×≤3：M3 非流式追问调用受 4s 预算约束（思考延迟 4-15s，
     * 命中是概率性）——规格为渐进增强（错过即放弃），rig 允许多轮重试，
     * ≥1 轮渲染 chips 即继续点击断言。 */
    const chipsGroup = page.getByRole("group", { name: W.followUpsLabel });
    let chipCount = 0;
    let attempts = 0;
    for (const question of [
      "站内有哪些关于配色练习的原创内容？请介绍一下",
      "站内有哪些关于水彩练习的原创内容？",
      "找一些适合新手的家具 Mod 并介绍一下",
    ]) {
      attempts++;
      await composer.fill(question);
      await composer.press("Enter");
      await sendBtn.waitFor({ state: "visible", timeout: 120000 });
      await page.waitForTimeout(1200); /* done 后 chips 随终稿渲染 */
      chipCount = await chipsGroup.getByRole("button").count();
      if (chipCount >= 1) break;
      console.log(`SP15B-NOTE attempt ${attempts}: chips=0 (join miss), retrying…`);
    }
    step("grounded-chips-rendered", chipCount >= 1, `chips=${chipCount} attempts=${attempts}`);
    if (chipCount < 1) throw new Error("no follow-up chips rendered in 3 attempts (provider latency)");

    const firstChip = chipsGroup.getByRole("button").first();
    const chipText = (await firstChip.innerText()).trim();
    const bubbleBefore = await page.locator('[data-slot="agent-transcript"]').innerText();

    /* 点击 = 填入 composer、不自动发送。 */
    await firstChip.click();
    await page.waitForTimeout(300);
    const composerValue = await composer.inputValue();
    const bubbleAfter = await page.locator('[data-slot="agent-transcript"]').innerText();
    const filled = composerValue === chipText;
    const notSent = bubbleAfter === bubbleBefore;
    step("chip-click-fills-composer", filled, `chip=${JSON.stringify(chipText)} composer=${JSON.stringify(composerValue)}`);
    step("chip-click-does-not-send", notSent, "transcript unchanged after chip click");

    await page.screenshot({ path: path.join(SHOTS, "sp15-b-1-chips.png"), fullPage: false });
    step("screenshot", true, "sp15-b-1-chips.png");
  } catch (err) {
    step("rig-error", false, String(err));
  } finally {
    await browser.close();
  }
  const failed = results.filter((l) => l.startsWith("FAIL"));
  console.log(
    failed.length ? `SP15B-RESULT FAIL (${failed.length}/${results.length})` : `SP15B-RESULT PASS (${results.length})`,
  );
  process.exit(failed.length ? 1 : 0);
}

main();
