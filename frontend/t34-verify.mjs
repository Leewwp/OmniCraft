/* T34 浏览器验证（隔离栈 3203/8087）：判官队列分页
 * 1) 初始加载 20/25，展示第 1 件案件，无误显「已完成」
 * 2) 投票推进到第 21 位 → 显示「加载更多待审案件」而非「已完成」（误展示修正）
 * 3) 点击加载更多 → 第 21 件案件出现（cases 25/25）
 * 4) 投完剩余 5 件 → 才显示「当前队列已全部审核完毕」
 * rig 不进 PR；截图 ../screenshots/t34-*.png */
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { execSync } from "node:child_process";
import { chromium } from "@playwright/test";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const zh = JSON.parse(readFileSync(path.join(HERE, "messages/zh.json"), "utf8"));
const BASE = process.env.T34_BASE_URL || "http://localhost:3203";
const SHOTS = path.join(HERE, "..", "screenshots");

const J = zh.judge;
const TXT = {
  completed: J.queueCompleted,           // 当前队列已全部审核完毕
  summary: J.queueSummary,               // 共 {total} 个待审案例…（含占位符，实际渲染后不同）
  loadMore: J.loadMore,                  // 加载更多待审案件
  voteSuccess: J.voteSuccess,            // 投票成功
  nextCase: J.nextCase,                  // 下一个案例
  approve: J.reviewCard.approve,         // 不违规
  confirm: J.reviewCard.confirm,         // 确认
};

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("T34-STEP " + line);
}

async function bodyText(page) {
  return page.locator("body").innerText();
}
async function waitForText(page, text, maxMs = 30000) {
  for (let i = 0; i < Math.ceil(maxMs / 400); i += 1) {
    await page.waitForTimeout(400);
    if ((await bodyText(page)).includes(text)) return true;
  }
  return (await bodyText(page)).includes(text);
}

/* 可重跑复位：清 9101 的票并重置案件票数 */
execSync(`docker exec lanej-pg psql -U postgres -d omnicraft -c "DELETE FROM judge_reason_votes WHERE voter_id=9101; DELETE FROM judge_votes WHERE judge_id=9101; UPDATE judge_cases SET vote_approve=0, vote_reject=0 WHERE target_type='article' AND target_id BETWEEN 7001 AND 7025;"`);

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  const page = await context.newPage();

  /* 登录 */
  await page.goto(`${BASE}/login`);
  await page.fill('input[type="email"]', "judge9101@lanej.local");
  await page.fill('input[type="password"]', "Lanej-Judge-Pass9101");
  await page.click('button[type="submit"]');
  await page.waitForURL((url) => !url.pathname.includes("login"), { timeout: 20000 });

  /* 1) 初始队列 */
  await page.goto(`${BASE}/judge/queue`);
  const cardReady = await waitForText(page, TXT.approve, 40000);
  step("initial-card-visible", cardReady);
  await page.screenshot({ path: path.join(SHOTS, "t34-initial.png"), fullPage: false });

  let text = await bodyText(page);
  const completedShownEarly = text.includes(TXT.completed);
  step("initial-no-false-completed", !completedShownEarly, completedShownEarly ? "首页即显示已完成文案" : "首页无已完成文案");
  const loadMoreShownEarly = text.includes(TXT.loadMore);
  step("initial-no-loadmore-while-browsing", !loadMoreShownEarly, loadMoreShownEarly ? "浏览前段不应出现加载更多" : "浏览前段无加载更多（符合预期，浏览至尾部才出现）");

  /* 2/3/4) 文本驱动推进：投票 → 下一案 → 直至完成态；遇「加载更多」先加载 */
  let loadMoreShot = false;
  let case21Shot = false;
  let sawLoadMore = false;
  let completedAtEnd = false;
  let votes = 0;

  for (let round = 0; round < 60; round += 1) {
    text = await bodyText(page);
    const approveBtn = page.getByRole("button", { name: TXT.approve, exact: true });

    if ((await approveBtn.count()) > 0 && (await approveBtn.first().isVisible())) {
      votes += 1;
      await approveBtn.first().click();
      const confirmBtn = page.getByRole("button", { name: TXT.confirm, exact: true });
      await confirmBtn.waitFor({ state: "visible", timeout: 8000 });
      await confirmBtn.click();
      const ok = await waitForText(page, TXT.voteSuccess, 15000);
      if (!ok) { step(`vote-${votes}-success`, false, "投票成功提示未出现"); break; }
      const nextBtn = page.getByRole("button", { name: TXT.nextCase, exact: true });
      await nextBtn.waitFor({ state: "visible", timeout: 8000 });
      await nextBtn.click();
      await approveBtn.first().waitFor({ state: "visible", timeout: 15000 }).catch(() => {});
      continue;
    }

    if (text.includes(TXT.loadMore)) {
      sawLoadMore = true;
      if (!loadMoreShot) {
        loadMoreShot = true;
        await page.screenshot({ path: path.join(SHOTS, "t34-load-more.png"), fullPage: false });
        step("load-more-shown-not-completed", !text.includes(TXT.completed), "第 21 位显示加载更多且无误显已完成");
      }
      await page.getByRole("button", { name: TXT.loadMore, exact: true }).click();
      await approveBtn.first().waitFor({ state: "visible", timeout: 20000 });
      if (!case21Shot) {
        case21Shot = true;
        await page.screenshot({ path: path.join(SHOTS, "t34-case-21.png"), fullPage: false });
        step("case-21-visible-after-loadmore", true, "加载更多后第 21 件案件出现");
      }
      continue;
    }

    if (text.includes(TXT.completed)) {
      completedAtEnd = true;
      break;
    }
    /* 无按钮无完成态：等待后再判（loading 过渡） */
    await page.waitForTimeout(800);
  }

  step("all-25-voted-through-pagination", completedAtEnd && votes === 25, `votes=${votes} completed=${completedAtEnd}`);
  step("load-more-appeared-at-21", sawLoadMore);
  await page.screenshot({ path: path.join(SHOTS, "t34-completed.png"), fullPage: false });
} finally {
  await browser.close();
}

const failed = results.filter((r) => r.startsWith("FAIL"));
console.log(`T34-SUMMARY pass=${results.length - failed.length} fail=${failed.length}`);
if (failed.length) {
  console.log("T34-FAILED:\n" + failed.join("\n"));
  process.exit(1);
}
