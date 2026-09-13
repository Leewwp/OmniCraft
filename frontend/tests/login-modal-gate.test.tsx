import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { AuthProvider } from "@/contexts/AuthContext";
import { ToastProvider } from "@/components/ui/Toast";
import {
  createFakeCaptcha,
  cleanup,
  fireEvent,
  installDom,
  render,
  typeInto,
  waitFor,
} from "./runtime-test-helpers";

const { within } = require("@testing-library/react") as typeof import("@testing-library/react");


// JSDOM 手动装配未含 localStorage 全局（comment-section-interactions 同款补法）：
// AuthContext 的 clearTokens 在裸 localStorage 上操作。
{
  const globalStore = globalThis as unknown as { localStorage?: Storage; window: typeof window };
  if (!globalStore.localStorage) {
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: globalStore.window.localStorage,
    });
  }
}


// SP-17/T2 (#491)：AuthGateProvider + LoginModal——auth-required 事件开浮窗、
// 登录成功关浮窗并自动续做 pendingAction、CAPTCHA_REQUIRED 动态插验证码、
// USER_BANNED 申诉指引、Esc 关闭归还焦点、浮窗内触发渲染进容器元素。

const { FakeCaptcha, getMountCount } = createFakeCaptcha();

let continueRuns: string[] = [];

function GateHarness() {
  const { requireAuth, setPortalContainer } = require("@/components/auth/AuthGateProvider").useAuthGate() as {
    requireAuth: (pending?: () => void) => void;
    setPortalContainer: (el: HTMLElement | null) => void;
  };
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  React.useEffect(() => {
    setPortalContainer(containerRef.current);
    return () => setPortalContainer(null);
  }, [setPortalContainer]);
  return (
    <div>
      <button
        type="button"
        data-testid="protected-action"
        onClick={() => requireAuth(() => { continueRuns.push("ran"); })}
      >
        do protected action
      </button>
      <button type="button" data-testid="plain-trigger" onClick={() => requireAuth()}>
        open gate
      </button>
      <div ref={containerRef} data-testid="overlay-container" />
    </div>
  );
}

function renderGate() {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <ToastProvider>
        <AuthProvider>
          <AuthGateProviderWithCaptcha>
            <GateHarness />
          </AuthGateProviderWithCaptcha>
        </AuthProvider>
      </ToastProvider>
    </IntlProvider>,
  );
}

// 注入 fake 验证码组件（RegisterPageContent 的 CaptchaComponent 注入先例）。
let captchaComponent: React.ComponentType<{ onToken: (t: string) => void }> | undefined;
function AuthGateProviderWithCaptcha({ children }: { children: React.ReactNode }) {
  const mod = require("@/components/auth/AuthGateProvider") as {
    AuthGateProvider: React.ComponentType<{ children: React.ReactNode; captchaComponent?: React.ComponentType<{ onToken: (t: string) => void }> }>;
  };
  return <mod.AuthGateProvider captchaComponent={captchaComponent}>{children}</mod.AuthGateProvider>;
}

const loginResponse = {
  user: {
    id: 7,
    email: "person@example.com",
    username: "person",
    avatar_url: "",
    bio: "",
    reputation: 10,
    preferred_locale: "en",
    role: "user",
    is_banned: false,
    email_verified_at: "2026-01-01T00:00:00Z",
    accept_collab_invites: true,
    created_at: "2026-01-01T00:00:00Z",
  },
  tokens: { access_token: "fresh-access" },
  capabilities: { can_interact: true },
};

type FetchLog = Array<{ url: string; body: unknown }>;

/** 按 script 决定 /auth/login 的响应；其余 auth 端点钉死。 */
function stubLoginFetch(script: Array<{ status: number; body: unknown }>) {
  const log: FetchLog = [];
  let call = 0;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url.endsWith("/auth/login")) {
      let body: unknown = undefined;
      try {
        body = JSON.parse(String(init?.body ?? "{}"));
      } catch { /* ignore */ }
      log.push({ url, body });
      const step = script[Math.min(call, script.length - 1)];
      call += 1;
      return new Response(JSON.stringify(step.body), {
        status: step.status,
        headers: { "Content-Type": "application/json" },
      });
    }
    if (url.endsWith("/auth/csrf")) {
      return new Response(JSON.stringify({ csrf_token: "test-csrf" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    if (url.endsWith("/auth/refresh")) {
      return new Response(JSON.stringify({ code: "INVALID_TOKEN", message: "anonymous" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });
    }
    // 封闭 stub：未匹配端点（pollUnread / merge-ips 等后台请求）一律 404，
    // 绝不透传 real fetch——本机 8080 常驻 dev 后端时泄漏会带回真实 401
    // TOKEN_EXPIRED，经事件桥清空 pendingAction（实测踩坑），且机器相关。
    return new Response(JSON.stringify({ code: "NOT_FOUND", message: "hermetic stub" }), {
      status: 404,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  return {
    log,
    restore() {
      globalThis.fetch = originalFetch;
    },
  };
}

async function submitCredentials(view: ReturnType<typeof render>, email: string, password: string) {
  const dialog = view.getByRole("dialog");
  await typeInto(await waitFor(() => within(dialog).getByLabelText("Email") as HTMLInputElement), email);
  await typeInto(await waitFor(() => within(dialog).getByLabelText("Password") as HTMLInputElement), password);
  fireEvent.submit(within(dialog).getByRole("button", { name: "Log in" }).closest("form")!);
}

test.afterEach(async () => {
  cleanup();
  continueRuns = [];
  captchaComponent = undefined;
  const { setAccessToken } = await import("@/lib/api");
  setAccessToken(null);
});

test("login modal gate lifecycle", async (t) => {
  const { AUTH_REQUIRED_EVENT } = await import("@/lib/auth-gate");

  await t.test("auth-required event opens the modal; login success closes it and runs pendingAction", async () => {
    installDom();
    const stub = stubLoginFetch([{ status: 200, body: loginResponse }]);
    const view = renderGate();

    fireEvent.click(view.getByTestId("protected-action"));
    const dialog = await waitFor(() => view.getByRole("dialog"), { timeout: 5000 });
    assert.ok(within(dialog).getAllByLabelText("Email").length > 0);

    await submitCredentials(view, "person@example.com", "correct-horse");
    await waitFor(() => {
      assert.equal(continueRuns.length, 1, "pendingAction must run after login");
    }, { timeout: 5000 });
    await waitFor(() => {
      let modalGone = false;
      try {
        view.getByRole("dialog");
      } catch {
        modalGone = true;
      }
      assert.ok(modalGone, "modal must close after successful login");
    }, { timeout: 5000 });
    assert.equal(stub.log.length, 1);
    const sentBody = stub.log[0].body as Record<string, unknown>;
    assert.equal(sentBody.email, "person@example.com");
    stub.restore();
  });

  await t.test("CAPTCHA_REQUIRED dynamically inserts the captcha and completed captcha logs in", async () => {
    installDom();
    captchaComponent = FakeCaptcha;
    const stub = stubLoginFetch([
      { status: 400, body: { code: "CAPTCHA_REQUIRED", message: "captcha required" } },
      { status: 200, body: loginResponse },
    ]);
    const view = renderGate();

    fireEvent.click(view.getByTestId("protected-action"));
    await waitFor(() => view.getByRole("dialog"));

    await submitCredentials(view, "person@example.com", "correct-horse");
    const dialog = view.getByRole("dialog");
    const captcha = await waitFor(() => within(dialog).getByTestId("fake-captcha"), { timeout: 5000 });
    assert.ok(captcha, "captcha widget must appear after CAPTCHA_REQUIRED");

    // 完成验证码 → 重试携带 captcha_token。
    fireEvent.click(within(dialog).getByRole("button", { name: "solve captcha" }));
    await submitCredentials(view, "person@example.com", "correct-horse");
    await waitFor(() => {
      assert.equal(continueRuns.length, 1);
    }, { timeout: 5000 });
    const retryBody = stub.log[stub.log.length - 1].body as Record<string, unknown>;
    assert.match(String(retryBody.captcha_token ?? ""), /^ticket-/, "retry must carry the captcha token");
    assert.equal(getMountCount() >= 1, true);
    stub.restore();
  });

  await t.test("USER_BANNED shows the feedback appeal entry", async () => {
    installDom();
    const stub = stubLoginFetch([
      { status: 403, body: { code: "USER_BANNED", message: "banned" } },
    ]);
    const view = renderGate();

    fireEvent.click(view.getByTestId("protected-action"));
    await waitFor(() => view.getByRole("dialog"));
    await submitCredentials(view, "banned@example.com", "whatever-ok");

    const dialog = view.getByRole("dialog");
    await waitFor(() => {
      assert.ok(within(dialog).getByText("To appeal or ask questions, reach us via the feedback form:"), "banned guidance must render");
      assert.ok(within(dialog).getByText("Submit feedback"), "appeal link must render");
    }, { timeout: 5000 });
    stub.restore();
  });

  await t.test("Esc closes the modal and returns focus to the trigger", async () => {
    installDom();
    const stub = stubLoginFetch([]);
    const view = renderGate();

    const trigger = view.getByTestId("plain-trigger");
    // jsdom 的 click 不移动焦点（真实浏览器会）：先 focus 再 click，模拟
    // 真实触发序列，聚焦归还断言才有意义。
    trigger.focus();
    fireEvent.click(trigger);
    await waitFor(() => view.getByRole("dialog"));

    fireEvent.keyDown(document.body, { key: "Escape" });
    await waitFor(() => {
      let modalGone = false;
      try {
        view.getByRole("dialog");
      } catch {
        modalGone = true;
      }
      assert.ok(modalGone, "Esc must close the modal");
    });
    assert.ok(document.activeElement === trigger, "focus must return to the trigger element");
    stub.restore();
  });

  await t.test("modal renders inside the overlay container element when set", async () => {
    installDom();
    const stub = stubLoginFetch([]);
    const view = renderGate();

    fireEvent.click(view.getByTestId("plain-trigger"));
    await waitFor(() => view.getByRole("dialog"));

    const container = view.getByTestId("overlay-container");
    assert.ok(container.querySelector('[role="dialog"]'), "modal must be a DOM child of the portal container (dialog top layer)");

    fireEvent.keyDown(document.body, { key: "Escape" });
    await waitFor(() => {
      let modalGone = false;
      try {
        view.getByRole("dialog");
      } catch {
        modalGone = true;
      }
      assert.ok(modalGone, "Esc must close the modal");
    });
    stub.restore();
  });
});
