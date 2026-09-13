import assert from "node:assert/strict";
import test from "node:test";

// SP-17/T2 (#491)：lib 层 auth-required 事件桥——api.ts（非 React 环境）在
// 401 刷新失败时通过它通知 AuthGateProvider 开登录浮窗；无 provider（门未
// 挂载）时兜底带 redirect 跳 /login。

test("auth gate event bridge", async (t) => {
  const dom = (await import("./runtime-test-helpers")).installDom();
  const {
    AUTH_REQUIRED_EVENT,
    emitAuthRequired,
    setAuthGateActive,
  } = await import("@/lib/auth-gate");

  await t.test("active gate receives the event with pendingAction detail", () => {
    setAuthGateActive(true);
    let fired = 0;
    let detail: unknown;
    const listener = (event: Event) => {
      fired += 1;
      detail = (event as CustomEvent).detail;
    };
    dom.window.addEventListener(AUTH_REQUIRED_EVENT, listener);
    try {
      const pending = () => undefined;
      emitAuthRequired({ pendingAction: pending });
      assert.equal(fired, 1);
      assert.equal((detail as { pendingAction?: unknown })?.pendingAction, pending);
    } finally {
      dom.window.removeEventListener(AUTH_REQUIRED_EVENT, listener);
      setAuthGateActive(false);
    }
  });

  await t.test("inactive gate does not dispatch the event (fallback handles it)", () => {
    setAuthGateActive(false);
    let fired = 0;
    const listener = () => {
      fired += 1;
    };
    dom.window.addEventListener(AUTH_REQUIRED_EVENT, listener);
    try {
      emitAuthRequired();
      assert.equal(fired, 0, "no listener should observe the event without an active gate");
    } finally {
      dom.window.removeEventListener(AUTH_REQUIRED_EVENT, listener);
    }
  });

  await t.test("event name is stable", () => {
    assert.equal(AUTH_REQUIRED_EVENT, "omnicraft:auth-required");
  });
});
