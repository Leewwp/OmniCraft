import assert from "node:assert/strict";
import test from "node:test";

import {
  applyKeywordFallbackCitations,
  closeAgentStream,
  createAgentTurn,
  reduceAgentTurn,
  stopAgentTurn,
} from "@/lib/agent-turn";
import type { AgentStreamCitation, AgentStreamEvent, AgentStreamTool } from "@/lib/agent-stream";

/* #663 TurnModel live 入口（reduceAgentTurn）纯函数测试族：无渲染、无 waitFor。
 *  判定语义逐字迁移自旧组件内实现，本文件同时是「迁移前后语义等价」的锚点：
 *  每个用例的期望值 = 旧实现的对应行为。 */

const citation: AgentStreamCitation = { content_id: 3, title: "Cited content", zone: "original" };
const tool: AgentStreamTool = { name: "search_content", status: "success", hits: 3, duration_ms: 42 };

function newTurn(query = "q") {
  return createAgentTurn(query, { id: "live-1", firstRound: true });
}

function reduceAll(turn: ReturnType<typeof newTurn>, events: AgentStreamEvent[]) {
  return events.reduce(reduceAgentTurn, turn);
}

test("流式增量拼接：think/tools/delta/citation/usage 依序累积", () => {
  const turn = reduceAll(newTurn("q"), [
    { type: "start", trace_id: "t1" },
    { type: "think_delta", delta: "先思考 " },
    { type: "think_delta", delta: "再思考" },
    { type: "tool_status", tool },
    { type: "delta", delta: "hello " },
    { type: "delta", delta: "world" },
    { type: "citation", citation },
    { type: "usage", usage: { prompt_tokens: 10, completion_tokens: 20 } },
  ]);

  assert.equal(turn.streaming, true);
  assert.equal(turn.settled, false);
  assert.equal(turn.terminal.traceId, "t1");
  assert.deepEqual(
    turn.segments.map((segment) => segment.kind),
    ["think", "tools"],
  );
  assert.equal(turn.segments[0].kind === "think" && turn.segments[0].content, "先思考 再思考");
  assert.deepEqual(
    turn.segments[1].kind === "tools" && turn.segments[1].tools,
    [tool],
  );
  assert.equal(turn.answer, "hello world");
  assert.deepEqual(turn.terminal.citations, [citation]);
  assert.deepEqual(turn.terminal.usage, { prompt_tokens: 10, completion_tokens: 20 });
});

test("空 delta/空 tool/无 trace 的 start 均为无操作（旧实现的防御语义）", () => {
  const base = newTurn();
  assert.equal(reduceAgentTurn(base, { type: "think_delta" }), base);
  assert.equal(reduceAgentTurn(base, { type: "delta" }), base);
  assert.equal(reduceAgentTurn(base, { type: "tool_status" }), base);
  assert.equal(reduceAgentTurn(base, { type: "citation" }), base);
  assert.equal(reduceAgentTurn(base, { type: "start" }), base);
  assert.equal(
    reduceAgentTurn(base, { type: "usage", usage: { prompt_tokens: "x", completion_tokens: 1 } }),
    base,
    "非数字 usage 不落",
  );
});

test("连续同类事件合并、跨类分段：think→tools→think 产生三段", () => {
  const turn = reduceAll(newTurn(), [
    { type: "think_delta", delta: "a" },
    { type: "think_delta", delta: "b" },
    { type: "tool_status", tool },
    { type: "think_delta", delta: "c" },
  ]);
  assert.deepEqual(
    turn.segments.map((segment) => segment.kind),
    ["think", "tools", "think"],
  );
  assert.deepEqual(
    turn.segments.map((segment) => (segment.kind === "think" ? segment.content : segment.tools.length)),
    ["ab", 1, "c"],
  );
});

test("done 终裁分支一：no_evidence 撤下已流出正文", () => {
  const turn = reduceAll(newTurn(), [
    { type: "delta", delta: "我找不到足够材料。" },
    {
      type: "done",
      conversation_id: 13,
      answer_kind: "no_evidence",
      answer: "",
      citations: [],
      tools: [{ name: "search_content", status: "success", hits: 0 }],
    },
  ]);
  assert.equal(turn.streaming, false);
  assert.equal(turn.settled, true);
  assert.equal(turn.answer, "", "no_evidence 撤答");
  assert.equal(turn.terminal.answerKind, "no_evidence");
  assert.equal(turn.terminal.emptyNoEvidence, false, "有成功工具 = 检索文案而非空轮");
  assert.deepEqual(turn.terminal.followUps, []);
});

test("done 终裁分支二：degraded 撤答并置降级", () => {
  const turn = reduceAll(newTurn(), [
    { type: "delta", delta: "模型总结必须隐藏" },
    {
      type: "done",
      conversation_id: 7,
      answer_kind: "grounded_content",
      answer: "模型总结必须隐藏",
      citations: [citation],
      degraded: true,
    },
  ]);
  assert.equal(turn.answer, "", "degraded 撤答");
  assert.equal(turn.terminal.degraded, true);
  assert.deepEqual(turn.terminal.citations, [citation], "降级轮引用走轮级列表");
  assert.equal(turn.answerCitations, undefined, "不随答案持久化");
});

test("done 终裁分支三：正常轮服务端终稿替换正文、引用随答案持久化、轮级引用清空", () => {
  const turn = reduceAll(newTurn(), [
    { type: "citation", citation },
    { type: "delta", delta: "流式正文" },
    {
      type: "done",
      conversation_id: 7,
      message_id: 4,
      trace_id: "t9",
      answer_kind: "grounded_content",
      answer: "服务端终稿",
      citations: [citation],
      tools: [tool],
      usage: { prompt_tokens: 1, completion_tokens: 2 },
      follow_ups: ["追问一", "追问二"],
    },
  ]);
  assert.equal(turn.answer, "服务端终稿");
  assert.deepEqual(turn.answerCitations, [citation]);
  assert.deepEqual(turn.terminal.citations, [], "轮级临时引用清空（防同轮双渲染）");
  assert.deepEqual(turn.terminal.followUps, ["追问一", "追问二"]);
  assert.equal(turn.terminal.emptyNoEvidence, false);
  assert.deepEqual(turn.terminal.usage, { prompt_tokens: 1, completion_tokens: 2 });
  assert.equal(turn.terminal.traceId, "t9", "done.trace_id 覆盖 start 轮内值");
});

test("done 终稿 citations 为空数组时保持无 answerCitations（空泡/无引用形态）", () => {
  const turn = reduceAll(newTurn(), [
    { type: "delta", delta: "正文" },
    { type: "done", conversation_id: 7, answer_kind: "grounded_content", answer: "正文", citations: [], tools: [] },
  ]);
  assert.equal(turn.answer, "正文");
  assert.equal(turn.answerCitations, undefined);
});

test("追问挂载条件：仅 grounded_content 且未降级且携带 follow_ups", () => {
  const doneWith = (overrides: Record<string, unknown>) =>
    reduceAll(newTurn(), [
      { type: "done", conversation_id: 7, answer: "a", answer_kind: "grounded_content", ...overrides },
    ]).terminal.followUps;

  assert.deepEqual(doneWith({ follow_ups: ["x"] }), ["x"]);
  assert.deepEqual(doneWith({ degraded: true, follow_ups: ["x"] }), [], "降级不挂追问");
  assert.deepEqual(doneWith({ answer_kind: "chitchat", follow_ups: ["x"] }), [], "非 grounded 不挂");
  assert.deepEqual(doneWith({}), [], "缺失 follow_ups = 无");
});

test("#610 空轮判定：no_evidence 且零成功工具（零调用或全部失败）", () => {
  const emptyBy = (tools: AgentStreamTool[]) =>
    reduceAll(newTurn(), [
      {
        type: "done",
        conversation_id: 7,
        answer_kind: "no_evidence",
        answer: "",
        citations: [],
        tools,
      },
    ]).terminal.emptyNoEvidence;

  assert.equal(emptyBy([]), true, "零调用 = 空轮");
  assert.equal(emptyBy([{ name: "generate_image", status: "error" }]), true, "全部失败 = 空轮");
  assert.equal(emptyBy([{ name: "search_content", status: "success", hits: 0 }]), false, "有成功检索 = 检索文案");
});

test("provider_error 降级：撤答、置降级、不置错误态", () => {
  const turn = reduceAll(newTurn(), [
    { type: "delta", delta: "partial" },
    {
      type: "error",
      error_code: "AGENT_PROVIDER_ERROR",
      degraded: true,
      degraded_reason: "provider_error",
    },
  ]);
  assert.equal(turn.streaming, false);
  assert.equal(turn.answer, "", "provider 降级撤答");
  assert.equal(turn.terminal.degraded, true);
  assert.equal(turn.terminal.error, false);

  const withFallback = applyKeywordFallbackCitations(turn, [citation]);
  assert.deepEqual(withFallback.terminal.citations, [citation], "关键词回退写轮级引用");
});

test("普通错误：置错误态与安全码", () => {
  const turn = reduceAll(newTurn(), [
    { type: "error", error_code: "AGENT_RATE_LIMIT_EXCEEDED" },
  ]);
  assert.equal(turn.streaming, false);
  assert.equal(turn.terminal.error, true);
  assert.equal(turn.terminal.errorCode, "AGENT_RATE_LIMIT_EXCEEDED");

  const noCode = reduceAll(newTurn(), [{ type: "error" }]);
  assert.equal(noCode.terminal.errorCode, null, "缺失 error_code 归一 null");
});

test("停止：保留半答与思考/工具，仅收敛 streaming 并标记停止", () => {
  const streaming = reduceAll(newTurn(), [
    { type: "think_delta", delta: "思考" },
    { type: "delta", delta: "partial answer" },
  ]);
  const stopped = stopAgentTurn(streaming);
  assert.equal(stopped.streaming, false);
  assert.equal(stopped.answer, "partial answer", "半答保留");
  assert.equal(stopped.segments[0].kind === "think" && stopped.segments[0].content, "思考");
  assert.equal(stopped.terminal.stopped, true);
  assert.equal(stopped.settled, true);
  assert.equal(stopped.terminal.error, false);
});

test("无终局关流：仅收敛 streaming（半答与轮内态保留）", () => {
  const closed = closeAgentStream(reduceAll(newTurn(), [{ type: "delta", delta: "half" }]));
  assert.equal(closed.streaming, false);
  assert.equal(closed.answer, "half");
  assert.equal(closed.terminal.stopped, false);
});

test("createAgentTurn 初始形状：仅提问行，streaming、firstRound 就绪", () => {
  const turn = createAgentTurn("问题", { id: "live-x", firstRound: false });
  assert.equal(turn.query, "问题");
  assert.equal(turn.settled, false);
  assert.equal(turn.answer, "");
  assert.deepEqual(turn.segments, []);
  assert.equal(turn.streaming, true);
  assert.equal(turn.firstRound, false);
  assert.deepEqual(turn.terminal, {
    answerKind: null,
    degraded: false,
    citations: [],
    followUps: [],
    usage: null,
    traceId: null,
    emptyNoEvidence: false,
    stopped: false,
    error: false,
    errorCode: null,
  });
});
