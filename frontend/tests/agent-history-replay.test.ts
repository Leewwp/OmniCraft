import assert from "node:assert/strict";
import test from "node:test";

import { mapAgentHistoryMessages, type AgentHistoryMessageDTO } from "@/lib/agent-history";

/* #538：历史回放映射——think/tools phase 行按序保留（与流式轮内渲染同构）、
   blocked 行占位、空答案行剔除、引用畸形项由 normalizer 剔除。 */

test("tools phase rows replay with their persisted step summaries in stream order", () => {
  const rows: AgentHistoryMessageDTO[] = [
    { id: 1, role: "user", content: "看看 88 号" },
    { id: 2, role: "assistant", phase: "think", content: "思考内容" },
    {
      id: 3,
      role: "assistant",
      phase: "tools",
      tools: [
        { name: "get_content_detail", status: "success", args_summary: "content_id=88", hits: 1, duration_ms: 12 },
        { name: "search_ips", status: "error", args_summary: "钢琴", duration_ms: 45 },
      ],
    },
    { id: 4, role: "assistant", content: "回答正文" },
  ];

  const mapped = mapAgentHistoryMessages(rows, "(hidden)");

  assert.equal(mapped.length, 4);
  assert.deepEqual(
    mapped.map((m) => m.phase ?? "body"),
    ["body", "think", "tools", "body"],
  );
  assert.deepEqual(mapped[2].tools?.map((t) => t.name), ["get_content_detail", "search_ips"]);
  assert.equal(mapped[2].tools?.[1].status, "error");
  assert.equal(mapped[2].role, "assistant");
});

test("tools rows survive without content and malformed steps degrade to an empty strip", () => {
  const rows: AgentHistoryMessageDTO[] = [
    { id: 1, role: "user", content: "q" },
    { id: 2, role: "assistant", phase: "tools" },
  ];

  const mapped = mapAgentHistoryMessages(rows, "(hidden)");

  assert.equal(mapped.length, 2);
  assert.equal(mapped[1].phase, "tools");
  assert.deepEqual(mapped[1].tools ?? [], [], "missing steps array degrades to an empty strip");
});

test("blocked answers redact to the placeholder and empty answers drop", () => {
  const rows: AgentHistoryMessageDTO[] = [
    { id: 1, role: "user", content: "q" },
    { id: 2, role: "assistant", moderation: "blocked" },
    { id: 3, role: "assistant", content: "   " },
    { id: 4, role: "assistant", content: "正常回答" },
  ];

  const mapped = mapAgentHistoryMessages(rows, "该回复已隐藏");

  assert.equal(mapped.length, 3);
  assert.equal(mapped[1].moderationBlocked, true);
  assert.equal(mapped[1].content, "该回复已隐藏");
  assert.equal(mapped[2].content, "正常回答");
});

test("answer citations pass through the normalizer and malformed ones drop", () => {
  const rows: AgentHistoryMessageDTO[] = [
    {
      id: 1,
      role: "assistant",
      content: "回答",
      citations: [
        { content_id: 88, title: "ok", zone: "fanwork" },
        { content_id: 0, title: "malformed" },
      ],
    },
  ];

  const mapped = mapAgentHistoryMessages(rows, "(hidden)");

  assert.equal(mapped.length, 1);
  assert.equal(mapped[0].citations?.length, 1);
});
