import assert from "node:assert/strict";
import test from "node:test";

import { startAgentStream } from "@/lib/agent-stream";
import { setAccessToken } from "@/lib/api";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function emptyStreamResponse(): Response {
  return new Response(new ReadableStream({ start(controller) { controller.close(); } }), { status: 200 });
}

test("startAgentStream does not POST when the CSRF precondition fails", async () => {
  const originalFetch = globalThis.fetch;
  let streamRequests = 0;
  globalThis.fetch = (async () => ({ ok: false, status: 503 })) as unknown as typeof fetch;
  const streamFetch = (async () => {
    streamRequests += 1;
    return { ok: true, status: 200, body: null } as Response;
  }) as typeof fetch;
  try {
    let error: Error | null = null;
    await startAgentStream(
      streamFetch,
      "http://api.test/api/v1/agent/chat/stream",
      {},
      { onEvent: () => undefined, onError: (err) => (error = err) },
    );
    assert.ok(error, "missing CSRF token must surface a client error");
    assert.equal(streamRequests, 0, "stream POST must not run without a CSRF token");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("startAgentStream refreshes an invalid CSRF token and retries once", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input) => {
    assert.match(String(input), /\/api\/v1\/auth\/csrf$/);
    return jsonResponse(200, { csrf_token: "fresh-csrf" });
  }) as typeof fetch;
  const headers: string[] = [];
  let attempts = 0;
  const streamFetch = (async (_input, init) => {
    attempts += 1;
    headers.push((init?.headers as Record<string, string>)["X-CSRF-Token"]);
    return attempts === 1
      ? jsonResponse(403, { code: "CSRF_TOKEN_INVALID", message: "invalid" })
      : emptyStreamResponse();
  }) as typeof fetch;
  try {
    await startAgentStream(streamFetch, "http://api.test/api/v1/agent/chat/stream", {}, { onEvent: () => undefined });
    assert.equal(attempts, 2);
    assert.equal(headers[1], "fresh-csrf");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("startAgentStream refreshes an expired access token and retries once", async () => {
  const originalFetch = globalThis.fetch;
  setAccessToken("expired-access");
  globalThis.fetch = (async (input) => {
    const url = String(input);
    if (url.endsWith("/api/v1/auth/refresh")) {
      return jsonResponse(200, { tokens: { access_token: "fresh-access" } });
    }
    if (url.endsWith("/api/v1/auth/csrf")) {
      return jsonResponse(200, { csrf_token: "csrf-for-refresh" });
    }
    throw new Error(`unexpected request ${url}`);
  }) as typeof fetch;
  const authHeaders: string[] = [];
  let attempts = 0;
  const streamFetch = (async (_input, init) => {
    attempts += 1;
    authHeaders.push((init?.headers as Record<string, string>).Authorization);
    return attempts === 1
      ? jsonResponse(401, { code: "TOKEN_EXPIRED", message: "expired" })
      : emptyStreamResponse();
  }) as typeof fetch;
  try {
    await startAgentStream(streamFetch, "http://api.test/api/v1/agent/chat/stream", {}, { onEvent: () => undefined });
    assert.deepEqual(authHeaders, ["Bearer expired-access", "Bearer fresh-access"]);
  } finally {
    setAccessToken(null);
    globalThis.fetch = originalFetch;
  }
});
