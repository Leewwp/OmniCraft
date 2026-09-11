/* SP-15 #434 D 票行为验收 rig（无头 Chromium，真实后端 + 真实 LLM）。
 * 断言工具调用 args 的实际内容（不因「指令已写」视为完成）：
 *  ① 多轮消解：第二轮 search_content query 含第一轮引用标题实体、非裸指代
 *  ② 复合拆分：一条复合消息触发 ≥2 个不同 query 的检索
 *  ③ 4 例光标题查询（#433 裁决 B 转入）：ke-0051 / vi-0003×2 / vi-0013
 *     必须触发检索且 query 自包含（含具体实体、无裸指代）
 * 另经 UI 驱动一次多轮对话留演示截图 ../screenshots/sp15-d-*.png。 */
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";

const zh = JSON.parse(readFileSync(path.join(process.cwd(), "messages/zh.json"), "utf8"));
const W = zh.agent.workspace;
const SHOTS = path.join(process.cwd(), "..", "screenshots");
const API = "http://localhost:8080";

const results = [];
function step(name, ok, detail = "") {
  const line = `${ok ? "PASS" : "FAIL"} ${name}${detail ? " — " + detail : ""}`;
  results.push(line);
  console.log("SP15D-STEP " + line);
}

/** 归一化：小写 + 去空白与中西标点，便于实体包含/公共子串判定。 */
function norm(s) {
  return (s || "").toLowerCase().replace(/[\s《》「」『』（）()【】\[\]·:：,，.。!！?？;；'’"“”\-—_~]/g, "");
}
/** 最长公共连续子串长度（实体命中阈值用）。 */
function longestRun(a, b) {
  if (!a || !b) return 0;
  let best = 0;
  for (let i = 0; i < a.length; i++) {
    for (let j = 0; j < b.length; j++) {
      let k = 0;
      while (i + k < a.length && j + k < b.length && a[i + k] === b[j + k]) k++;
      if (k > best) best = k;
    }
  }
  return best;
}
function entityInQuery(query, fragment, minRun = 4) {
  const q = norm(query);
  const f = norm(fragment);
  if (!q || !f) return false;
  return q.includes(f) || longestRun(q, f) >= Math.min(minRun, f.length);
}
const searchQueries = (turn) =>
  (turn.tools || []).filter((t) => t.name === "search_content").map((t) => t.args_summary || "").filter(Boolean);

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

    /* 页内 SSE 直连：精确捕获 tool_status（args_summary）与 done 事件。
     * 鉴权走 Bearer access token（app 的 token 只存内存模块变量，evaluate
     * 拿不到，故 rig 自行 API 登录一次换取）。 */
    const apiLogin = async () =>
      page.evaluate(async ({ API }) => {
        const csrfRes = await fetch(`${API}/api/v1/auth/csrf`, { credentials: "include" });
        const { csrf_token } = await csrfRes.json();
        const res = await fetch(`${API}/api/v1/auth/login`, {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf_token },
          body: JSON.stringify({ email: "a06-verify@seed.omnicraft.local", password: "A06Verify#2026" }),
        });
        if (!res.ok) return { httpError: res.status };
        const data = await res.json();
        return { token: data?.tokens?.access_token || "" };
      }, { API });
    const loginRes = await apiLogin();
    step("api-login", !!loginRes.token, `http=${loginRes.httpError ?? 200}`);
    if (!loginRes.token) throw new Error("api login failed");
    const bearer = loginRes.token;

    const agentTurn = async (message, conversationId) =>
      page.evaluate(
        async ({ API, message, conversationId, bearer }) => {
          const csrfRes = await fetch(`${API}/api/v1/auth/csrf`, { credentials: "include" });
          const { csrf_token } = await csrfRes.json();
          const res = await fetch(`${API}/api/v1/agent/chat/stream`, {
            method: "POST",
            credentials: "include",
            headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf_token, Authorization: `Bearer ${bearer}` },
            body: JSON.stringify(conversationId ? { conversation_id: conversationId, message } : { message }),
          });
          if (!res.ok) return { httpError: res.status };
          const reader = res.body.getReader();
          const dec = new TextDecoder();
          let buf = "";
          const tools = [];
          let done = null;
          let streamError = null;
          let conversation = conversationId || 0;
          const handle = (line) => {
            if (!line.startsWith("data:")) return;
            const raw = line.slice(5).trim();
            if (!raw || raw === "[DONE]") return;
            try {
              const ev = JSON.parse(raw);
              if (ev.type === "tool_status" && ev.tool) tools.push(ev.tool);
              if (ev.type === "done") done = ev;
              if (ev.type === "error") streamError = ev;
              if (ev.type === "start" && ev.conversation_id) conversation = ev.conversation_id;
            } catch {}
          };
          for (;;) {
            const { done: finished, value } = await reader.read();
            if (finished) break;
            buf += dec.decode(value, { stream: true });
            let nl = buf.indexOf("\n");
            while (nl >= 0) {
              handle(buf.slice(0, nl));
              buf = buf.slice(nl + 1);
              nl = buf.indexOf("\n");
            }
          }
          if (buf) handle(buf);
          return { tools, done, streamError, conversation };
        },
        { API, message, conversationId, bearer },
      );

    /* ① 多轮消解（SSE 精确断言）。 */
    const T1 = "站内有哪些关于配色练习的原创内容？请介绍一下";
    const T2 = "第二个的作者还有什么作品";
    const turn1 = await agentTurn(T1);
    step("mt-turn1-grounded", turn1.done?.answer_kind === "grounded_content",
      `kind=${turn1.done?.answer_kind} err=${turn1.streamError?.error_code ?? "none"} http=${turn1.httpError ?? 200}`);
    const titles = (turn1.done?.citations || []).map((c) => c.title).filter(Boolean);
    step("mt-turn1-citations", titles.length >= 2, `titles=${JSON.stringify(titles.slice(0, 4))}`);
    if (turn1.done?.answer_kind !== "grounded_content" || titles.length < 2) {
      throw new Error(`turn-1 precondition failed (kind=${turn1.done?.answer_kind} titles=${titles.length} http=${turn1.httpError ?? 200})`);
    }

    const turn2 = await agentTurn(T2, turn1.conversation);
    const q2 = searchQueries(turn2);
    const resolved = q2.some((q) => titles.some((t) => entityInQuery(q, t, 4)));
    step("mt-turn2-search-triggered", q2.length >= 1, `queries=${JSON.stringify(q2)} kind=${turn2.done?.answer_kind}`);
    step("mt-turn2-resolved-entity", resolved,
      `need one query containing a turn-1 citation title; got ${JSON.stringify(q2)}`);

    /* ② 复合拆分（SSE）。 */
    const composite = await agentTurn("帮我分别找找站内的水彩入门教程和摄影后期调色技巧");
    const qc = searchQueries(composite).map(norm);
    const distinct = new Set(qc);
    step("composite-split", distinct.size >= 2, `distinct=${distinct.size} queries=${JSON.stringify([...distinct])}`);

    /* ③ 4 例光标题查询（#433 裁决 B 配套，各自新会话）。 */
    const bareTitles = [
      { id: "ke-0051", msg: "The Room of Requirement Keeps Its Secrets", frag: "room of requirement", min: 8 },
      { id: "vi-0003#a", msg: "鼬的乌鸦停在碑前", frag: "乌鸦", min: 2 },
      { id: "vi-0003#b", msg: "鼬的乌鸦停在碑前", frag: "乌鸦", min: 2 },
      { id: "vi-0013", msg: "九月没有来信", frag: "来信", min: 2 },
    ];
    for (const c of bareTitles) {
      const turn = await agentTurn(c.msg);
      const q = searchQueries(turn);
      const hit = q.some((query) => entityInQuery(query, c.frag, c.min));
      step(`bare-${c.id}-search-triggered`, q.length >= 1, `kind=${turn.done?.answer_kind} queries=${JSON.stringify(q)}`);
      step(`bare-${c.id}-self-contained-query`, hit, `need query containing ${JSON.stringify(c.frag)}`);
    }

    /* ④ UI 驱动多轮对话演示记录（截图，不做额外断言）。 */
    await page.goto("http://localhost:3000/agent");
    const composer = page.locator(`textarea[aria-label="${W.composerLabel}"]`);
    await composer.waitFor({ state: "visible", timeout: 15000 });
    const sendBtn = page.locator(`button[aria-label="${W.sendMessage}"]`);
    const turnEnd = async (timeout = 120000) => {
      await sendBtn.waitFor({ state: "visible", timeout });
      await page.waitForTimeout(800);
    };
    await composer.fill(T1);
    await composer.press("Enter");
    await turnEnd();
    await composer.fill(T2);
    await composer.press("Enter");
    await turnEnd();
    await page.screenshot({ path: path.join(SHOTS, "sp15-d-1-multiturn.png"), fullPage: false });
    step("ui-demo-multiturn-screenshot", true, "sp15-d-1-multiturn.png");
  } catch (err) {
    step("rig-error", false, String(err));
  } finally {
    await browser.close();
  }
  const failed = results.filter((l) => l.startsWith("FAIL"));
  console.log(failed.length ? `SP15D-RESULT FAIL (${failed.length}/${results.length})` : `SP15D-RESULT PASS (${results.length})`);
  process.exit(failed.length ? 1 : 0);
}

main();
