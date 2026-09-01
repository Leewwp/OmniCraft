import assert from "node:assert/strict";
import test from "node:test";

import { startAgentStream, type AgentStreamEvent, type AgentStreamHandlers } from "@/lib/agent-stream";

function csrfFetchOk(): typeof fetch {
  return (async () =>
    new Response(JSON.stringify({ csrf_token: "csrf-test" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    })) as typeof fetch;
}

function chunkedStreamResponse(chunks: string[], status = 200): Response {
  const encoder = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    },
  });
  return new Response(body, { status });
}

function streamFetch(res: Response): typeof fetch {
  return (async () => res) as typeof fetch;
}

test("events split across network chunks are parsed completely", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = csrfFetchOk();
  try {
    const doneEvent = JSON.stringify({
      type: "done",
      conversation_id: 42,
      answer_kind: "cited",
      answer: "这是一个跨越多个网络分块的长答案，包含中文标点与 markdown 表格。\n| a | b |\n|---|---|\n| 1 | 2 |",
      citations: [
        { content_id: 7, title: "引用一", zone: "original", excerpt: "摘录文本，同样很长，用于撑大单行事件体积。" },
      ],
    });
    const all = `data: {"type":"delta","delta":"你好"}\n\ndata: ${doneEvent}\n\n`;
    const cut1 = Math.floor(all.length * 0.3);
    const cut2 = Math.floor(all.length * 0.75);
    const res = chunkedStreamResponse([all.slice(0, cut1), all.slice(cut1, cut2), all.slice(cut2)]);

    const events: AgentStreamEvent[] = [];
    const errors: Error[] = [];
    await startAgentStream(streamFetch(res), "http://api.test/api/v1/agent/chat/stream", {}, {
      onEvent: (event) => events.push(event),
      onError: (error) => errors.push(error),
    });

    assert.deepEqual(errors, []);
    assert.equal(events.length, 2, `expected 2 events, got ${JSON.stringify(events)}`);
    assert.equal(events[0].type, "delta");
    if (events[0].type === "delta") assert.equal(events[0].delta, "你好");
    const done = events[1];
    assert.equal(done.type, "done");
    if (done.type === "done") {
      assert.equal(done.conversation_id, 42);
      assert.equal(done.citations?.length, 1);
      assert.equal(done.citations?.[0]?.content_id, 7);
      assert.ok(done.answer?.includes("跨"));
      assert.ok(done.answer?.endsWith("| 1 | 2 |"));
    }
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("multi-byte UTF-8 split mid-character across chunks survives", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = csrfFetchOk();
  try {
    const line = 'data: {"type":"delta","delta":"你"}\n\n';
    const bytes = new TextEncoder().encode(line);
    const res = new Response(
      new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(bytes.subarray(0, bytes.length - 4));
          controller.enqueue(bytes.subarray(bytes.length - 4));
          controller.close();
        },
      }),
      { status: 200 },
    );

    const events: AgentStreamEvent[] = [];
    const errors: Error[] = [];
    await startAgentStream(streamFetch(res), "http://api.test/api/v1/agent/chat/stream", {}, {
      onEvent: (event) => events.push(event),
      onError: (error) => errors.push(error),
    });
    assert.deepEqual(errors, []);
    assert.equal(events.length, 1);
    if (events[0].type === "delta") assert.equal(events[0].delta, "你");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("final line without trailing newline is still delivered", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = csrfFetchOk();
  try {
    const res = chunkedStreamResponse(['data: {"type":"delta","delta":"收尾"}']);
    const events: AgentStreamEvent[] = [];
    const errors: Error[] = [];
    await startAgentStream(streamFetch(res), "http://api.test/api/v1/agent/chat/stream", {}, {
      onEvent: (event) => events.push(event),
      onError: (error) => errors.push(error),
    });
    assert.deepEqual(errors, []);
    assert.equal(events.length, 1);
    if (events[0].type === "delta") assert.equal(events[0].delta, "收尾");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("an over-long unterminated line aborts the stream with an error", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = csrfFetchOk();
  try {
    const huge = `data: {"type":"delta","delta":"${"x".repeat(3 * 1024 * 1024)}"}`;
    const res = chunkedStreamResponse([huge]);
    const events: AgentStreamEvent[] = [];
    const errors: Error[] = [];
    await startAgentStream(streamFetch(res), "http://api.test/api/v1/agent/chat/stream", {}, {
      onEvent: (event) => events.push(event),
      onError: (error) => errors.push(error),
    });
    assert.equal(events.length, 0, "no events should leak from a rejected over-long line");
    assert.equal(errors.length, 1, "buffer limit must abort with onError");
    assert.match(errors[0].message, /buffer|line|limit/i);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
