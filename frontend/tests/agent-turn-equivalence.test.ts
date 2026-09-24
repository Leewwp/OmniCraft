import assert from "node:assert/strict";
import test from "node:test";

import {
  createAgentTurn,
  mapAgentHistoryMessages,
  mapAgentHistoryToTurns,
  mergeTerminalIntoLastTurn,
  reduceAgentTurn,
  type AgentHistoryMessageDTO,
  type AgentTurn,
} from "@/lib/agent-turn";
import type { AgentStreamCitation, AgentStreamEvent, AgentStreamTool } from "@/lib/agent-stream";

/* #663 TurnModel 双入口等价断言：live 事件与历史行对等价语义输入产出同形
 *  回合内容（提问行/think-tools 相块/正文/随答案引用/脱敏占位）。终态字段
 *  （追问/用量/trace/answer_kind）不落库，历史入口保持缺省——这是落库契约
 *  差异而非装配漂移，另行断言。两入口还分别与迁移前固定样本（旧实现的行级
 *  mapper 输出）对照，防两入口共同映射错误。 */

const citation: AgentStreamCitation = { content_id: 3, title: "Cited content", zone: "original" };
const tool: AgentStreamTool = { name: "search_content", status: "success", hits: 3, duration_ms: 42 };

function liveTurn(events: AgentStreamEvent[]): AgentTurn {
  return events.reduce(reduceAgentTurn, createAgentTurn("q", { id: "live-x", firstRound: true }));
}

/** 回合内容投影（等价比较面）：终态字段与 id/firstRound/streaming 不比。 */
function contentOf(turn: AgentTurn) {
  return {
    query: turn.query,
    segments: turn.segments,
    answer: turn.answer,
    answerCitations: turn.answerCitations ?? null,
    moderationBlocked: turn.moderationBlocked ?? null,
  };
}

test("双入口同形：完整轮（思考+工具+引用+正文）live 事件 ↔ 历史行产出同形内容", () => {
  const live = liveTurn([
    { type: "start", trace_id: "t1" },
    { type: "think_delta", delta: "先把口语化需求" },
    { type: "think_delta", delta: "扩展为检索词" },
    { type: "tool_status", tool },
    { type: "delta", delta: "hello " },
    { type: "delta", delta: "world" },
    {
      type: "done",
      conversation_id: 7,
      answer_kind: "grounded_content",
      answer: "hello world",
      citations: [citation],
      tools: [tool],
      degraded: false,
    },
  ]);
  const history = mapAgentHistoryToTurns(
    [
      { id: 1, role: "user", content: "q" },
      { id: 2, role: "assistant", phase: "think", content: "先把口语化需求扩展为检索词" },
      { id: 3, role: "assistant", phase: "tools", tools: [tool] },
      { id: 4, role: "assistant", content: "hello world", citations: [citation] },
    ] satisfies AgentHistoryMessageDTO[],
    "(hidden)",
  );

  assert.equal(history.length, 1);
  assert.deepEqual(contentOf(history[0]), contentOf(live));

  /* 终态差异面：live 携带 done 终裁与 trace；历史不落库、保持缺省。 */
  assert.equal(live.terminal.answerKind, "grounded_content");
  assert.equal(live.terminal.traceId, "t1");
  assert.equal(history[0].terminal.answerKind, null);
  assert.deepEqual(history[0].terminal.followUps, []);
  assert.equal(history[0].streaming, false);
  assert.equal(history[0].settled, true);
});

test("双入口同形：撤答轮（no_evidence）与降级轮历史侧不留正文", () => {
  const liveNoEvidence = liveTurn([
    { type: "tool_status", tool },
    { type: "delta", delta: "我找不到足够材料。" },
    { type: "done", conversation_id: 7, answer_kind: "no_evidence", answer: "", citations: [], tools: [tool] },
  ]);
  const historyNoEvidence = mapAgentHistoryToTurns(
    [
      { id: 1, role: "user", content: "q" },
      { id: 2, role: "assistant", phase: "tools", tools: [tool] },
      { id: 3, role: "assistant", content: "   " },
    ] satisfies AgentHistoryMessageDTO[],
    "(hidden)",
  );
  assert.equal(historyNoEvidence.length, 1);
  assert.equal(historyNoEvidence[0].answer, "", "空答案行剔除后历史轮无正文");
  assert.equal(liveNoEvidence.answer, "", "no_evidence 撤答");
  assert.deepEqual(contentOf(historyNoEvidence[0]), contentOf(liveNoEvidence));
});

test("双入口同形：moderation 脱敏行 = 占位正文 + moderationBlocked", () => {
  const history = mapAgentHistoryToTurns(
    [
      { id: 1, role: "user", content: "q" },
      { id: 2, role: "assistant", moderation: "blocked" },
    ] satisfies AgentHistoryMessageDTO[],
    "该回复已隐藏",
  );
  assert.equal(history.length, 1);
  assert.equal(history[0].moderationBlocked, true);
  assert.equal(history[0].answer, "该回复已隐藏");
  assert.deepEqual(history[0].segments, []);
});

test("历史入口 vs 迁移前固定样本（旧行级 mapper 输出）：行序与内容逐一对应", () => {
  const rows: AgentHistoryMessageDTO[] = [
    { id: 1, role: "user", content: "看看 88 号" },
    { id: 2, role: "assistant", content: "思考内容", phase: "think" },
    {
      id: 3,
      role: "assistant",
      phase: "tools",
      tools: [
        { name: "get_content_detail", status: "success", args_summary: "content_id=88", hits: 1, duration_ms: 12 },
        { name: "search_ips", status: "error", args_summary: "钢琴", duration_ms: 45 },
      ],
    },
    { id: 4, role: "assistant", content: "回答正文", citations: [{ content_id: 88, title: "ok", zone: "fanwork" }] },
    { id: 5, role: "user", content: "第二轮提问" },
    { id: 6, role: "assistant", content: "第二轮回答" },
  ];

  /* 迁移前固定样本：旧 mapper（逐字迁入）对同一输入的行级输出。 */
  const legacyRows = mapAgentHistoryMessages(rows, "(hidden)");
  assert.deepEqual(
    legacyRows.map((row) => row.phase ?? "body"),
    ["body", "think", "tools", "body", "body", "body"],
  );

  const turns = mapAgentHistoryToTurns(rows, "(hidden)");
  assert.equal(turns.length, 2, "两轮：按 user 行分轮");
  /* 轮内容 = 旧行序列按轮分组的同序展开（提问行/思考/工具/正文）。 */
  assert.deepEqual(
    turns.map((turn) => turn.query),
    ["看看 88 号", "第二轮提问"],
  );
  assert.deepEqual(
    turns[0].segments.map((segment) => (segment.kind === "think" ? segment.content : segment.tools.map((item) => item.name))),
    ["思考内容", ["get_content_detail", "search_ips"]],
  );
  assert.equal(turns[0].answer, "回答正文");
  assert.deepEqual(turns[1].segments, []);
  assert.equal(turns[1].answer, "第二轮回答");
  assert.deepEqual(turns[0].answerCitations, [{ content_id: 88, title: "ok", zone: "fanwork" }]);
});

test("live 入口 vs 迁移前固定样本：标准事件序列的回合内容 = 旧组件装配形状", () => {
  /* 旧组件对同一事件序列的装配（迁移前行为，agent-turn-reducer 测试族已逐
     语义锚定）：think 合并、tools 列表、正文终稿替换、引用随答案。 */
  const turn = liveTurn([
    { type: "think_delta", delta: "先思考 " },
    { type: "think_delta", delta: "再思考" },
    { type: "tool_status", tool },
    { type: "citation", citation },
    { type: "delta", delta: "流式正文" },
    {
      type: "done",
      conversation_id: 7,
      answer_kind: "grounded_content",
      answer: "服务端终稿",
      citations: [citation],
      tools: [tool],
      degraded: false,
      usage: { prompt_tokens: 5, completion_tokens: 7 },
    },
  ]);
  assert.deepEqual(turn.segments, [
    { kind: "think", content: "先思考 再思考" },
    { kind: "tools", tools: [tool] },
  ]);
  assert.equal(turn.answer, "服务端终稿");
  assert.deepEqual(turn.answerCitations, [citation]);
  assert.deepEqual(turn.terminal.citations, [], "done 引用随答案后轮级临时引用清空");
  assert.deepEqual(turn.terminal.usage, { prompt_tokens: 5, completion_tokens: 7 });
});

test("mergeTerminalIntoLastTurn：live 终态并入树尾轮、空树不并入", () => {
  const base = mapAgentHistoryToTurns(
    [
      { id: 1, role: "user", content: "q" },
      { id: 2, role: "assistant", content: "a" },
    ] satisfies AgentHistoryMessageDTO[],
    "(hidden)",
  );
  const merged = mergeTerminalIntoLastTurn(base, {
    ...base[0].terminal,
    followUps: ["追问一"],
    traceId: "t9",
  });
  assert.equal(merged.length, 1);
  assert.deepEqual(merged[0].terminal.followUps, ["追问一"]);
  assert.equal(merged[0].terminal.traceId, "t9");
  assert.equal(merged[0].answer, "a", "并入只覆终态，不动回合内容");
  assert.deepEqual(mergeTerminalIntoLastTurn([], base[0].terminal), []);
});
