import assert from "node:assert/strict";
import test from "node:test";

// SP-17/T2 (#491)：api.ts 401 刷新失败不再 window.location.href 硬跳 /login，
// 改走 auth-required 事件桥；SSE 直连的 refreshAccessTokenHeader 同口径。

test("api 401 refresh failure routes through the auth-required bridge", async (t) => {
  const { installDom, waitFor } = await import("./runtime-test-helpers");
  installDom();

  const { api, setAccessToken } = await import("@/lib/api");
  const { AUTH_REQUIRED_EVENT, setAuthGateActive } = await import("@/lib/auth-gate");

  await t.test("request() emits the bridge event instead of hard navigation", async () => {
    setAccessToken("expired-token");
    setAuthGateActive(true);
    let fired = 0;
    const listener = () => {
      fired += 1;
    };
    window.addEventListener(AUTH_REQUIRED_EVENT, listener);

    const originalFetch = globalThis.fetch;
    globalThis.fetch = (async (input: string | URL | Request) => {
      const url = String(input);
      if (url.endsWith("/auth/refresh")) {
        return new Response(JSON.stringify({ code: "INVALID_TOKEN", message: "no" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        });
      }
      return new Response(
        JSON.stringify({ code: "TOKEN_EXPIRED", message: "Session expired" }),
        { status: 401, headers: { "Content-Type": "application/json" } },
      );
    }) as typeof fetch;

    try {
      await assert.rejects(api.get("/api/v1/social/reactions?target_type=content&target_id=1"));
      await waitFor(() => assert.equal(fired, 1, "auth-required event must fire on refresh failure"));
      assert.ok(
        !window.location.href.startsWith("/login"),
        `must not hard-navigate, saw ${window.location.href}`,
      );
    } finally {
      window.removeEventListener(AUTH_REQUIRED_EVENT, listener);
      globalThis.fetch = originalFetch;
      setAccessToken(null);
      setAuthGateActive(false);
    }
  });

  await t.test("refreshAccessTokenHeader emits the bridge event instead of hard navigation", async () => {
    setAccessToken("expired-token");
    setAuthGateActive(true);
    let fired = 0;
    const listener = () => {
      fired += 1;
    };
    window.addEventListener(AUTH_REQUIRED_EVENT, listener);

    const originalFetch = globalThis.fetch;
    globalThis.fetch = (async (input: string | URL | Request) => {
      const url = String(input);
      if (url.endsWith("/auth/refresh")) {
        return new Response(JSON.stringify({ code: "INVALID_TOKEN", message: "no" }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        });
      }
      return new Response("unused", { status: 404 });
    }) as typeof fetch;

    try {
      const { refreshAccessTokenHeader } = await import("@/lib/api");
      const ok = await refreshAccessTokenHeader({});
      assert.equal(ok, false);
      await waitFor(() => assert.equal(fired, 1, "auth-required event must fire on SSE refresh failure"));
      assert.ok(!window.location.href.startsWith("/login"));
    } finally {
      window.removeEventListener(AUTH_REQUIRED_EVENT, listener);
      globalThis.fetch = originalFetch;
      setAccessToken(null);
      setAuthGateActive(false);
    }
  });
});
