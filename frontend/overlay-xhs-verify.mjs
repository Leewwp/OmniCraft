// 详情浮窗对齐小红书批（E #952~#957）· 断言矩阵单一入口（编排 rig）。
//
// 跑法（前置服务，均需已在跑）：
//   1) PostgreSQL（docker compose up -d postgres —— 容器名 omnicraft-postgres，
//      rig 经 `docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft`
//      幂等重置夹具内容 99101~99123 / 隔离 IP 99209）；
//   2) 后端 API http://localhost:8080（`cd backend && REDIS_PASSWORD=localdev
//      go run cmd/server/main.go`）；
//   3) 前端 dev http://localhost:3000（`cd frontend && npm run dev`）。
//   环境变量：OVERLAY_XHS_BASE_URL（缺省 http://localhost:3000）仅作用于
//   本文件的补遗组；四个子 rig 各自读 E{1,2,3,4}_BASE_URL（缺省同 3000）。
//   执行：`cd frontend && node overlay-xhs-verify.mjs`（可重复执行，幂等）。
//
// 编排模式：顺序 spawn 四个 per-ticket rig（e1 几何 / e2 开合动画 / e3 媒体源
// / e4 预取就绪——各持独立夹具段与合成附件服务端口 18083~18085，互不冲突），
// 之后在本进程跑「补遗组」：原型断言矩阵中未被 e1~e4 覆盖的条目 + 终态补强
// （无 console/page 错误、中间张双侧箭头可用、跨比例 letterbox contain 形态、
// 关窗退场完成）与「F4 组」（#989：带 sort/q 参数开浮窗的 URL 记录透传/浮窗
// 存活/背景不重拉不重挂/关窗回源恢复 query）。子 rig 保留为历史 seam（各票
// 验证证据的可独立复跑单元），本文件是常规验证门的唯一入口：全绿 = 断言矩阵全过。
// 截图 ../screenshots/e{1,2,3,4}-rig-*.png 与 e5-matrix-*.png。
import { spawn } from "node:child_process";
import { chromium } from "@playwright/test";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const BASE = process.env.OVERLAY_XHS_BASE_URL || "http://localhost:3000";
const PAGE_PATH = "/ip/209?tab=share";
const SHOTS = resolve(HERE, "../screenshots");
const MIXED_TITLE = "【E1验证】混排三比例图集"; // e1 rig 幂等重置的夹具（补遗组复用）

const SUB_RIGS = [
  { name: "e1-geometry", script: "overlay-xhs-e1-verify.mjs", group: "几何冻结/跨内容宽度/宽舞台档/档位切换" },
  { name: "e2-motion", script: "overlay-xhs-e2-verify.mjs", group: "morph 收敛/关窗中点 scale/落地前透明度/即解锁/连按层数/关窗回源" },
  { name: "e3-media", script: "overlay-xhs-e3-verify.mjs", group: "封面首屏/托底/全类型渲染非空/边界 forbidden" },
  { name: "e4-prefetch", script: "overlay-xhs-e4-verify.mjs", group: "相邻 idle 预取/缓存命中/弱网 hold-crossfade/预取失败静默" },
];

const results = [];
function check(name, ok, detail = "") {
  results.push({ name, ok });
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? "  — " + detail : ""}`);
}

/** 顺序跑子 rig：stdout/stderr 透传，exit code 归一为一条断言。 */
function runSubRig(rig) {
  return new Promise((resolveRun) => {
    console.log(`\n════ 子 rig ${rig.name}（${rig.group}）════`);
    const child = spawn(process.execPath, [resolve(HERE, rig.script)], {
      stdio: ["ignore", "inherit", "inherit"],
      env: { ...process.env },
    });
    child.on("error", (error) => {
      check(`子 rig ${rig.name} 可执行`, false, String(error));
      resolveRun(false);
    });
    child.on("close", (code) => {
      check(`子 rig ${rig.name} 全绿（exit=${code}）`, code === 0, `script=${rig.script}`);
      resolveRun(code === 0);
    });
  });
}

/** 补遗组：原型断言矩阵遗留条目 + 终态补强（复用 e1 夹具 99101，纯图附件无 oss_url 补丁需求）。 */
async function residualGroup() {
  console.log("\n════ 补遗组（原型矩阵遗留 + 终态补强）════");
  const browser = await chromium.launch();
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  const pageErrors = [];
  page.on("pageerror", (e) => pageErrors.push(String(e)));
  page.on("console", (m) => {
    if (m.type() === "error") pageErrors.push(m.text());
  });
  try {
    await page.goto(`${BASE}${PAGE_PATH}`, { waitUntil: "domcontentloaded", timeout: 30000 });
    await page.waitForTimeout(800);

    const card = page.locator(`text=${MIXED_TITLE}`).first();
    await card.waitFor({ state: "visible", timeout: 10000 });
    await card.evaluate((el) => el.scrollIntoView({ block: "center" }));
    await page.waitForTimeout(150);
    await card.click({ timeout: 10000 });
    await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 12000 });
    await page.waitForTimeout(1100);
    await page.screenshot({ path: `${SHOTS}/e5-matrix-open.png` });

    async function badge() {
      return page.evaluate(() => document.querySelector('[data-slot="media-count-badge"]')?.textContent?.trim() ?? "");
    }
    async function arrowDisabled(index) {
      return page.evaluate((i) => {
        const arrow = document.querySelectorAll('[data-slot="media-arrow"]')[i];
        return Boolean(arrow && arrow.disabled);
      }, index);
    }

    /* 中间张双侧箭头可用（原型 A·中间张断言）。 */
    await page.keyboard.press("ArrowRight");
    await page.waitForTimeout(450);
    check("中间张（2/3）双侧箭头均可用", (await badge()) === "2 / 3" && !(await arrowDisabled(0)) && !(await arrowDisabled(1)),
      `badge=${await badge()} L禁=${await arrowDisabled(0)} R禁=${await arrowDisabled(1)}`);

    /* 跨比例 9:16 张在冻结媒体列内 letterbox（contain 不裁切）。 */
    await page.keyboard.press("ArrowRight");
    await page.waitForTimeout(500);
    check("末张（3/3）在冻结媒体列 letterbox（object-contain）", (await badge()) === "3 / 3", `badge=${await badge()}`);
    const fitMode = await page.evaluate(() => {
      const pane = document.querySelector('[data-slot="variant-media-pane"]');
      const img = pane?.querySelector('[data-slot="detail-cover"] img');
      return img ? getComputedStyle(img).objectFit : "";
    });
    check("媒体主体 contain 形态（黑边 letterbox，不裁切）", fitMode === "contain", `object-fit=${fitMode}`);
    await page.screenshot({ path: `${SHOTS}/e5-matrix-letterbox.png` });

    /* 关窗退场完成（隐藏帧后 dialog 移除）。 */
    await page.keyboard.press("Escape");
    await page.waitForTimeout(700);
    check("关窗退场完成（dialog 移除）", (await page.locator("dialog[open]").count()) === 0);
    await page.screenshot({ path: `${SHOTS}/e5-matrix-after-close.png` });

    /* 全程无 JS 异常/非资源类 console 错误（原型终态断言的本地栈口径：资源加载
       失败——游客 auth refresh 401、本地缺失的 config 端点 404、已停机 rig 媒体
       服务的夹具残留——均为环境噪声不计入；JS 异常与非资源 console.error 计入）。 */
    const realErrors = pageErrors.filter(
      (text) => !text.startsWith("Failed to load resource:") && !text.includes("favicon") && !text.includes("Download the React DevTools"),
    );
    check("全程无 JS 异常/非资源类 console 错误", realErrors.length === 0, realErrors.slice(0, 3).join(" | "));
  } finally {
    await context.close().catch(() => {});
    await browser.close().catch(() => {});
  }
}

/** F4 #989：IP hub 带 sort/q 参数开浮窗——会话记录 URL 透传来源 search、
 *  浮窗存活、背景列表不重拉不重挂、关窗回源恢复原 query（复用 e4 隔离 IP
 *  99209 夹具：恰三篇，newest 序下三张均在首屏）。 */
async function f4UrlSearchGroup() {
  console.log("\n════ F4 组（sort/q 参数浮窗 URL 回写——#989）════");
  const browser = await chromium.launch();
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  const ORIGIN_SEARCH = "?tab=share&sort=newest";
  let listRequests = 0;
  page.on("request", (req) => {
    if (/\/api\/v1\/ips\/99209\/contents/.test(req.url())) listRequests += 1;
  });
  try {
    await page.goto(`${BASE}/ip/99209${ORIGIN_SEARCH}`, { waitUntil: "domcontentloaded", timeout: 30000 });
    await page.waitForTimeout(600);
    const card = page.locator("text=【E4验证】相邻次篇").first();
    await card.waitFor({ state: "visible", timeout: 10000 });
    await card.evaluate((el) => {
      el.setAttribute("data-f4-probe", "alive");
      window.__f4DocMarker = 1;
    });
    await page.evaluate((el) => el.scrollIntoView({ block: "center" }), await card.elementHandle());
    await page.waitForTimeout(150);
    const requestsBeforeClick = listRequests;

    await card.click({ timeout: 10000 });
    await page.locator("dialog[open]").waitFor({ state: "visible", timeout: 12000 });
    await page.waitForTimeout(1500);

    check(
      "F4 浮窗 1.5s 后仍存活（带 sort 参数开浮窗不自杀）",
      (await page.locator("dialog[open]").count()) === 1,
    );
    const url = new URL(page.url());
    check(
      "F4 记录 URL = 规范路径 + 来源 search 原样透传",
      url.pathname === "/content/99122" && url.search === ORIGIN_SEARCH,
      `url=${url.pathname}${url.search}`,
    );
    check(
      "F4 背景列表不重拉（点击后零新 /ips/99209/contents 请求）",
      listRequests === requestsBeforeClick,
      `before=${requestsBeforeClick} after=${listRequests}`,
    );
    const identity = await page.evaluate(() => ({
      doc: window.__f4DocMarker === 1,
      grid: Boolean(document.querySelector("[data-f4-probe='alive']")),
    }));
    check(
      "F4 无文档卸载且背景网格不重挂（window/卡片 DOM 探针存活）",
      identity.doc && identity.grid,
      `doc=${identity.doc} grid=${identity.grid}`,
    );
    await page.screenshot({ path: `${SHOTS}/e5-matrix-f4-url-search.png` });

    await page.keyboard.press("Escape").catch(() => {});
    await page.waitForTimeout(900);
    const afterClose = new URL(page.url());
    check(
      "F4 关窗回源恢复原 query（来源记录 search 原样）",
      afterClose.pathname === "/ip/99209" && afterClose.search === ORIGIN_SEARCH,
      `url=${afterClose.pathname}${afterClose.search}`,
    );
  } finally {
    await context.close().catch(() => {});
    await browser.close().catch(() => {});
  }
}

for (const rig of SUB_RIGS) {
  await runSubRig(rig);
}
await residualGroup();
await f4UrlSearchGroup();

const failed = results.filter((r) => !r.ok).length;
console.log(`\n${failed === 0 ? "ALL PASS" : failed + " FAILED"} (${results.length} checks)`);
process.exit(failed === 0 ? 0 : 1);
