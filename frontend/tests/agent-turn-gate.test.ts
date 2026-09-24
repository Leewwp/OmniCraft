import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

/* #663 守门（source-contract 风格，先例：后端 routes_test / judge_wiring、
 * 前端 #416 源码断言）：回合装配逻辑只许住在 TurnModel（lib/agent-turn.ts）——
 * 工作台组件内禁止流事件 case 分派与轮内展示状态（12 态家族/模块级 ID 计数器）
 * 回潮。副作用分派（done/error 的会话写入与 abort）以 if 形态合法。 */

const STREAM_EVENT_CASE_MARKERS = [
  'case "start"',
  'case "think_delta"',
  'case "tool_status"',
  'case "citation"',
  'case "usage"',
  'case "delta"',
  'case "done"',
  'case "error"',
];

const TURN_STATE_MARKERS = [
  "setTurnThinking",
  "setTurnTools",
  "setTurnCitations",
  "setTurnDegraded",
  "setLastAnswerKind",
  "setTurnEmptyNoEvidence",
  "setTurnErrorCode",
  "setTurnTraceId",
  "setTurnUsage",
  "setTurnFollowUps",
  "setTurnError",
  "setStoppedNotice",
  "nextMessageId",
  "turnAnswerIdRef",
  "turnCitationsRef",
];

function gateViolations(source: string): string[] {
  return [...STREAM_EVENT_CASE_MARKERS, ...TURN_STATE_MARKERS].filter((marker) =>
    source.includes(marker),
  );
}

test("守门：AgentWorkspace 组件内无流事件 case 分派与轮内状态回潮", async () => {
  const source = await readFile(
    new URL("../components/agent/AgentWorkspace.tsx", import.meta.url),
    "utf8",
  );
  assert.deepEqual(gateViolations(source), [], "回合装配只许住在 TurnModel（lib/agent-turn.ts）");
});

test("守门反例：事件 case 分派/轮内状态回潮必须被守门拦下（谓词有效性证明）", () => {
  const fixture = `
    function handleEvent(event) {
      switch (event.type) {
        case "delta":
          setTurnCitations((previous) => [...previous, event.citation]);
          break;
        case "done":
          setTurnTraceId(event.trace_id);
          break;
      }
    }
    let nextMessageId = 1;
  `;
  const violations = gateViolations(fixture);
  assert.ok(violations.includes('case "delta"'), "拦事件 case 分派");
  assert.ok(violations.includes('case "done"'), "拦 done 终裁分派");
  assert.ok(violations.includes("setTurnCitations"), "拦轮内状态回潮");
  assert.ok(violations.includes("setTurnTraceId"), "拦轮内状态回潮");
  assert.ok(violations.includes("nextMessageId"), "拦模块级 ID 计数器");
});

test("守门：TurnModel 双入口与终态并同居 lib/agent-turn.ts（装配单一住所）", async () => {
  const source = await readFile(new URL("../lib/agent-turn.ts", import.meta.url), "utf8");
  for (const marker of [
    "export function reduceAgentTurn",
    "export function mapAgentHistoryToTurns",
    "export function mapAgentHistoryMessages",
    "export function mergeTerminalIntoLastTurn",
  ]) {
    assert.ok(source.includes(marker), `${marker} 必须在 TurnModel 模块内`);
  }
});
