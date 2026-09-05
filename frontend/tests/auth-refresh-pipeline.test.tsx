import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import { refreshSession, setAccessToken } from "@/lib/api";
import { cleanup, installDom, render, waitFor } from "./runtime-test-helpers";

/**
 * #381 auth refresh 竞态（前端面）：
 * AuthContext.refresh 与 api.ts refreshSession 必须共享同一条应用级单飞管线。
 * 现状两套单飞互不感知（authRefreshInFlight vs refreshPromise），同一标签页内
 * 并发调用会对 /auth/refresh 发出两次真实请求，打进服务端轮换竞态窗口。
 */

interface Call {
  input: string;
  init?: RequestInit;
}

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("AuthContext.refresh and refreshSession share one refresh pipeline", async () => {
  installDom();
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: window.localStorage,
  });
  const originalFetch = globalThis.fetch;
  const calls: Call[] = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const call: Call = { input: String(input), init };
    calls.push(call);
    const path = call.input;
    if (path.endsWith("/auth/csrf")) return jsonResponse(200, { csrf_token: "csrf-1" });
    if (path.endsWith("/auth/refresh")) {
      return jsonResponse(200, { tokens: { access_token: "at-1" } });
    }
    if (path.endsWith("/auth/me")) {
      return jsonResponse(200, {
        user: { id: 7, username: "race-user", email: "race@example.test" },
      });
    }
    return jsonResponse(404, { code: "NOT_FOUND", message: "unexpected" });
  }) as typeof fetch;

  let ctxRef: ReturnType<typeof useAuth> | null = null;
  function Probe() {
    const ctx = useAuth();
    if (ctx.user && !ctxRef) ctxRef = ctx;
    return React.createElement("div", null, ctx.user ? "ready" : "loading");
  }

  try {
    render(React.createElement(AuthProvider, null, React.createElement(Probe)));
    await waitFor(() => assert.ok(ctxRef, "AuthProvider should restore the user"));
    // 等挂载期 fetchMe 自己的 refresh 结算完，只统计其后的调用
    const settleIndex = calls.length;

    const [viaContext, viaSession] = await Promise.all([
      ctxRef!.refresh(),
      refreshSession(),
    ]);
    assert.equal(viaContext, true, "AuthContext.refresh must succeed");
    assert.equal(viaSession, true, "refreshSession must succeed");

    const refreshCalls = calls
      .slice(settleIndex)
      .filter((c) => c.input.endsWith("/auth/refresh")).length;
    assert.equal(
      refreshCalls,
      1,
      `concurrent context/session refresh must share ONE pipeline, got ${refreshCalls} POSTs`,
    );
  } finally {
    cleanup();
    globalThis.fetch = originalFetch;
    setAccessToken(null);
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      writable: true,
      value: undefined,
    });
  }
});
