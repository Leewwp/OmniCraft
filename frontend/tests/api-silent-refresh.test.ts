import assert from "node:assert/strict";
import test from "node:test";

import { ApiRequestError, api, setAccessToken } from "@/lib/api";

/**
 * T32（FIX-32 + #322）：
 * ① 401 UNAUTHORIZED（会话过期真码）触发一次静默刷新重试——现状只认
 *    TOKEN_EXPIRED 死码，过期即抛错踢登录；
 * ② 刷新后重试仍 401 → 抛错，不二次刷新（每请求至多一次防循环）；
 * ③ USER_BANNED 401 不触发刷新（封禁≠会话过期）；
 * ④ 并发请求共享同一 in-flight refresh（单飞，#322 轮换竞态）。
 */

interface Call {
  input: string;
  init?: RequestInit;
}

function installFetch(handler: (call: Call, index: number) => Response | Promise<Response>) {
  const originalFetch = globalThis.fetch;
  const calls: Call[] = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const call: Call = { input: String(input), init };
    const index = calls.length;
    calls.push(call);
    return handler(call, index);
  }) as typeof fetch;
  return {
    calls,
    restore() {
      globalThis.fetch = originalFetch;
    },
  };
}

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("401 UNAUTHORIZED triggers one silent refresh and retries with the new token", async () => {
  const mock = installFetch((call) => {
    if (call.input.endsWith("/auth/csrf")) {
      return jsonResponse(200, { csrf_token: "csrf-1" });
    }
    if (call.input.endsWith("/auth/refresh")) {
      return jsonResponse(200, { tokens: { access_token: "new-token" } });
    }
    if (call.input.includes("/users/me")) {
      const auth = (call.init?.headers as Record<string, string>)?.Authorization ?? "";
      if (auth === "Bearer new-token") {
        return jsonResponse(200, { user: { id: 7 } });
      }
      return jsonResponse(401, { code: "UNAUTHORIZED", message: "invalid or expired token" });
    }
    return jsonResponse(404, { code: "NOT_FOUND", message: "unexpected" });
  });
  setAccessToken("stale-token");

  try {
    const data = await api.get<{ user: { id: number } }>("/api/v1/users/me");
    assert.equal(data.user.id, 7);
    const refreshCalls = mock.calls.filter((c) => c.input.endsWith("/auth/refresh"));
    assert.equal(refreshCalls.length, 1, "exactly one refresh call");
    const retry = mock.calls.at(-1);
    assert.equal(
      (retry?.init?.headers as Record<string, string>)?.Authorization,
      "Bearer new-token",
      "retry must carry the refreshed token",
    );
  } finally {
    setAccessToken(null);
    mock.restore();
  }
});

test("401 after a successful refresh is not refreshed again (no loop)", async () => {
  const mock = installFetch((call) => {
    if (call.input.endsWith("/auth/csrf")) {
      return jsonResponse(200, { csrf_token: "csrf-1" });
    }
    if (call.input.endsWith("/auth/refresh")) {
      return jsonResponse(200, { tokens: { access_token: "new-token" } });
    }
    return jsonResponse(401, { code: "UNAUTHORIZED", message: "invalid or expired token" });
  });
  setAccessToken("stale-token");

  try {
    await assert.rejects(
      api.get("/api/v1/users/me"),
      (err: unknown) => err instanceof ApiRequestError && err.status === 401,
    );
    const refreshCalls = mock.calls.filter((c) => c.input.endsWith("/auth/refresh"));
    assert.equal(refreshCalls.length, 1, "refresh must run at most once per request");
  } finally {
    setAccessToken(null);
    mock.restore();
  }
});

test("401 USER_BANNED does not trigger a refresh", async () => {
  const mock = installFetch(() =>
    jsonResponse(401, { code: "USER_BANNED", message: "account has been banned" }),
  );
  setAccessToken("banned-token");

  try {
    await assert.rejects(
      api.get("/api/v1/users/me"),
      (err: unknown) => err instanceof ApiRequestError && err.code === "USER_BANNED",
    );
    assert.equal(mock.calls.length, 1, "no refresh attempt for ban denial");
  } finally {
    setAccessToken(null);
    mock.restore();
  }
});

test("TOKEN_EXPIRED keeps its existing refresh-retry path", async () => {
  const mock = installFetch((call) => {
    if (call.input.endsWith("/auth/csrf")) {
      return jsonResponse(200, { csrf_token: "csrf-1" });
    }
    if (call.input.endsWith("/auth/refresh")) {
      return jsonResponse(200, { tokens: { access_token: "new-token" } });
    }
    return jsonResponse(401, { code: "TOKEN_EXPIRED", message: "expired" });
  });
  setAccessToken("stale-token");

  try {
    await assert.rejects(
      api.get("/api/v1/users/me"),
      (err: unknown) => err instanceof ApiRequestError && err.code === "TOKEN_EXPIRED",
    );
    const refreshCalls = mock.calls.filter((c) => c.input.endsWith("/auth/refresh"));
    assert.equal(refreshCalls.length, 1);
  } finally {
    setAccessToken(null);
    mock.restore();
  }
});

test("concurrent 401s share one in-flight refresh (#322 single-flight)", async () => {
  const mock = installFetch((call) => {
    if (call.input.endsWith("/auth/csrf")) {
      return jsonResponse(200, { csrf_token: "csrf-1" });
    }
    if (call.input.endsWith("/auth/refresh")) {
      return jsonResponse(200, { tokens: { access_token: "new-token" } });
    }
    const auth = (call.init?.headers as Record<string, string>)?.Authorization ?? "";
    if (auth === "Bearer new-token") {
      return jsonResponse(200, { ok: true });
    }
    return jsonResponse(401, { code: "UNAUTHORIZED", message: "invalid or expired token" });
  });
  setAccessToken("stale-token");

  try {
    const [a, b] = await Promise.all([
      api.get("/api/v1/users/me"),
      api.get("/api/v1/notifications/unread-count"),
    ]);
    assert.ok(a);
    assert.ok(b);
    const refreshCalls = mock.calls.filter((c) => c.input.endsWith("/auth/refresh"));
    assert.equal(refreshCalls.length, 1, "concurrent 401s must share one refresh");
  } finally {
    setAccessToken(null);
    mock.restore();
  }
});
