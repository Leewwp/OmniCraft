import assert from "node:assert/strict";
import test from "node:test";

import { mapAgentHistoryToTurns, type AgentTurn, type AgentTurnTerminal } from "@/lib/agent-turn";
import {
  handoverFirstRoundTerminal,
  invalidateFirstRoundHandover,
  resetFirstRoundHandoverStore,
  takeOverFirstRoundHandover,
} from "@/lib/agent-first-round-handover";

/* #795 首轮交接 module 行为测试：保存先于导航、即时卸载不丢记录、重挂载接管
 * 只消费一次、空历史/无记录不误消费、按会话 id 隔离（迟到回载不串会话）、
 * 明确失效清理。同实例合并不经过本 module（工作台直接读活动轮终态），其接线
 * 证明在 agent-workspace.test.tsx 的整页卸载/重挂载回归与 mocked e2e。 */

function terminalWith(overrides: Partial<AgentTurnTerminal> = {}): AgentTurnTerminal {
  return {
    answerKind: "grounded_content",
    degraded: false,
    citations: [],
    followUps: ["追问一", "追问二"],
    usage: { prompt_tokens: 120, completion_tokens: 45 },
    traceId: "trace-795",
    emptyNoEvidence: false,
    needsKeywordFallback: false,
    stopped: false,
    error: false,
    errorCode: null,
    ...overrides,
  };
}

/** 服务端历史行（事实源）：一轮问答 + tools 相块行。 */
function historyTurns() {
  return mapAgentHistoryToTurns(
    [
      { id: 1, role: "user", content: "首轮问题" },
      {
        id: 2,
        role: "assistant",
        phase: "tools",
        tools: [{ name: "search_content", status: "success", duration_ms: 9 }],
      },
      { id: 3, role: "assistant", content: "首轮回答正文。" },
    ],
    "hidden",
  );
}

test.beforeEach(() => {
  resetFirstRoundHandoverStore();
});

test("保存先于导航：navigate 回调内记录已可接管，且导航后只消费一次", () => {
  /* 闭包内赋值 → holder 对象（let 变量的流分析不跨闭包）。 */
  const seen: { duringNavigate?: AgentTurn[] } = {};
  handoverFirstRoundTerminal(41, terminalWith(), () => {
    /* 模拟导航 adapter 同步触发重挂载接管：navigate 执行时记录必须已写入。 */
    seen.duringNavigate = takeOverFirstRoundHandover(41, historyTurns());
  });
  const duringNavigate = seen.duringNavigate;
  assert.ok(duringNavigate, "navigate ran");
  assert.deepEqual(duringNavigate[duringNavigate.length - 1].terminal.followUps, ["追问一", "追问二"]);
  assert.equal(
    duringNavigate[duringNavigate.length - 1].terminal.usage?.completion_tokens,
    45,
    "terminal merged into the last history turn during the navigate callback",
  );
  /* 消费一次即失效：导航后的接管回归历史轮终态缺省（落库契约）。 */
  const again = takeOverFirstRoundHandover(41, historyTurns());
  assert.deepEqual(again[again.length - 1].terminal.followUps, []);
  assert.equal(again[again.length - 1].terminal.usage, null);
});

test("即时卸载不丢记录：navigate 抛出/直接返回后记录仍可被后续重挂载接管", () => {
  /* 形态一：navigate 正常返回（旧实例随即卸载，无人再消费）。 */
  handoverFirstRoundTerminal(41, terminalWith(), () => undefined);
  /* 形态二：导航中途抛错（卸载竞态的最坏形态），保存必须已经完成。 */
  assert.throws(() => {
    handoverFirstRoundTerminal(42, terminalWith({ traceId: "trace-42" }), () => {
      throw new Error("navigation teardown");
    });
  }, /navigation teardown/);
  const revived41 = takeOverFirstRoundHandover(41, historyTurns());
  const revived42 = takeOverFirstRoundHandover(42, historyTurns());
  assert.equal(revived41[revived41.length - 1].terminal.traceId, "trace-795");
  assert.equal(revived42[revived42.length - 1].terminal.traceId, "trace-42");
});

test("重挂载接管并入树尾轮一次；再次接管回归历史轮终态缺省", () => {
  handoverFirstRoundTerminal(41, terminalWith(), () => undefined);
  const first = takeOverFirstRoundHandover(41, historyTurns());
  assert.equal(first.length, historyTurns().length, "takeover keeps server history rows");
  const last = first[first.length - 1];
  assert.deepEqual(last.terminal.followUps, ["追问一", "追问二"]);
  assert.deepEqual(last.terminal.usage, { prompt_tokens: 120, completion_tokens: 45 });
  assert.equal(last.terminal.traceId, "trace-795");
  const second = takeOverFirstRoundHandover(41, historyTurns());
  assert.deepEqual(second[second.length - 1].terminal.followUps, []);
  assert.equal(second[second.length - 1].terminal.usage, null);
  assert.equal(second[second.length - 1].terminal.traceId, null);
});

test("空历史不消费记录：待后续非空历史仍可接管（不提前弹出静默丢弃终态）", () => {
  handoverFirstRoundTerminal(41, terminalWith(), () => undefined);
  const empty = takeOverFirstRoundHandover(41, []);
  assert.deepEqual(empty, [], "empty history maps to an empty tree");
  const late = takeOverFirstRoundHandover(41, historyTurns());
  assert.deepEqual(late[late.length - 1].terminal.followUps, ["追问一", "追问二"], "record survives the empty-history load");
});

test("无记录时原样返回（服务端历史为事实源）且不产生副本", () => {
  const history = historyTurns();
  const untouched = takeOverFirstRoundHandover(41, history);
  assert.equal(untouched, history, "no record → same reference, server history stays authoritative");
});

test("迟到历史返回隔离：记录按会话 id 键控，别会话的接管碰不到它", () => {
  handoverFirstRoundTerminal(41, terminalWith(), () => undefined);
  const other = takeOverFirstRoundHandover(42, historyTurns());
  assert.deepEqual(other[other.length - 1].terminal.followUps, [], "another conversation takes over without the record");
  const mine = takeOverFirstRoundHandover(41, historyTurns());
  assert.deepEqual(mine[mine.length - 1].terminal.followUps, ["追问一", "追问二"], "record for 41 untouched by the 42 takeover");
});

test("明确失效清理：invalidate 后历史回载不再并入终态（删除/离开/404 路径）", () => {
  handoverFirstRoundTerminal(41, terminalWith(), () => undefined);
  invalidateFirstRoundHandover(41);
  const after = takeOverFirstRoundHandover(41, historyTurns());
  assert.deepEqual(after[after.length - 1].terminal.followUps, []);
  assert.equal(after[after.length - 1].terminal.traceId, null);
  /* 失效后重复 invalidate 幂等。 */
  invalidateFirstRoundHandover(41);
});

test("最新终态覆盖：同一会话重复交接以最后一次为准", () => {
  handoverFirstRoundTerminal(41, terminalWith({ followUps: ["旧追问"] }), () => undefined);
  handoverFirstRoundTerminal(41, terminalWith({ followUps: ["新追问"] }), () => undefined);
  const revived = takeOverFirstRoundHandover(41, historyTurns());
  assert.deepEqual(revived[revived.length - 1].terminal.followUps, ["新追问"]);
});
