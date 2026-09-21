import assert from "node:assert/strict";
import test from "node:test";

import { streamUsageGuide } from "@/lib/usage-guide-stream";

function sseResponse(chunks: string[]): Response {
  const encoder = new TextEncoder();
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    },
  });
  return new Response(stream, { status: 200, headers: { "content-type": "text/event-stream" } });
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "content-type": "application/json" } });
}

async function run(res: Response) {
  const deltas: string[] = [];
  const events: string[] = [];
  await streamUsageGuide(
    () => Promise.resolve(res),
    "http://backend.test/api/v1/agent/usage-guide/1?stream=true",
    {
      onDelta: (d) => { deltas.push(d); events.push("delta"); },
      onDone: () => { events.push("done"); },
      onError: () => { events.push("error"); },
      onClose: () => { events.push("close"); },
    },
  );
  return { deltas, events };
}

/* SP-25 FR-04（中-9）：跨 chunk 截断行不得产生半行 JSON 拼接。
 * 夹具用 gin SSEvent 真实格式 `event:delta\ndata:{...}`（冒号后无空格）；
 * 最后一个用例保留带空格变体，两种形态都必须能解析。 */

test("delta lines split across network chunks are reassembled", async () => {
  // 一条完整 data 行被切在 JSON 中间与事件边界上。
  const { deltas, events } = await run(sseResponse([
    'event:delta\nda',
    'ta:{"type":"delta","del',
    'ta":"雨夜"}\n\nevent:do',
    'ne\ndata:{"type":"done"}\n\n',
  ]));
  assert.deepEqual(deltas, ["雨夜"]);
  assert.deepEqual(events, ["delta", "done", "close"]);
});

test("multiple complete lines in one chunk all parse", async () => {
  const { deltas, events } = await run(sseResponse([
    'event:delta\ndata:{"type":"delta","delta":"第一段"}\n\nevent:delta\ndata:{"type":"delta","delta":"第二段"}\n\nevent:done\ndata:{"type":"done"}\n\n',
  ]));
  assert.deepEqual(deltas, ["第一段", "第二段"]);
  assert.deepEqual(events, ["delta", "delta", "done", "close"]);
});

test("stream error event surfaces onError without onClose", async () => {
  const { events } = await run(sseResponse([
    'event:error\ndata:{"type":"error","error_code":"provider_error","error_message":"provider unavailable"}\n\n',
  ]));
  assert.deepEqual(events, ["error"]);
});

test("spaced data-prefix variant (data: {...}) also parses", async () => {
  const { deltas, events } = await run(sseResponse([
    'event: delta\ndata: {"type":"delta","delta":"空格形态"}\n\nevent: done\ndata: {"type":"done"}\n\n',
  ]));
  assert.deepEqual(deltas, ["空格形态"]);
  assert.deepEqual(events, ["delta", "done", "close"]);
});

test("structured JSON response delivers the guide in one delta", async () => {
  const { deltas, events } = await run(jsonResponse({ guide: "结构化指南", structured: true }));
  assert.deepEqual(deltas, ["结构化指南"]);
  assert.deepEqual(events, ["delta", "done", "close"]);
});

test("non-2xx response surfaces onError only", async () => {
  const { events } = await run(new Response("nope", { status: 404 }));
  assert.deepEqual(events, ["error"]);
});
