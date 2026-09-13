import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { AuthProvider } from "@/contexts/AuthContext";
import { ToastProvider } from "@/components/ui/Toast";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const { within } = require("@testing-library/react") as typeof import("@testing-library/react");

// next/navigation 桩（message-compose-entry.test.tsx 先例）：组件树里的
// useRouter 需要应用路由上下文，单测环境下用桩替代并收集 push 目标。
const Module = require("node:module") as { _load: (request: string, parent: unknown, isMain: boolean) => unknown };
const originalModuleLoad = Module._load;
const routerPushes: string[] = [];
Module._load = function patchedLoad(request: string, parent: unknown, isMain: boolean) {
  if (request === "next/navigation") {
    return {
      useRouter: () => ({
        push: (url: string) => {
          routerPushes.push(url);
        },
        replace: (url: string) => {
          routerPushes.push(url);
        },
      }),
      useSearchParams: () => new URLSearchParams(),
      usePathname: () => "/",
    };
  }
  return originalModuleLoad.call(this, request, parent, isMain);
};

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


// SP-17/T2 (#491)：未登录入口统一改走 auth gate 浮窗——
// 关注/私信/点赞/收藏/评论/讨论区；不再 router.push("/login")。
// 能力拒绝保持禁用+原因的既有行为由 social-entry-states 覆盖，此处不重复。

function GateHost({ children }: { children: React.ReactNode }) {
  const mod = require("@/components/auth/AuthGateProvider") as {
    AuthGateProvider: React.ComponentType<{ children: React.ReactNode }>;
  };
  return <mod.AuthGateProvider>{children}</mod.AuthGateProvider>;
}

function renderWithGate(ui: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <ToastProvider>
        <AuthProvider>
          <GateHost>{ui}</GateHost>
        </AuthProvider>
      </ToastProvider>
    </IntlProvider>,
  );
}

async function assertOpensLoginModal(view: ReturnType<typeof render>) {
  const dialog = await waitFor(() => view.getByRole("dialog"));
  assert.ok(dialog, "login modal must open");
  await waitFor(() => {
    assert.ok(!window.location.href.includes("/login"), "must not navigate to /login");
  });
}

const publicConfig = {
  features: { web_agent_enabled: false, payment_enabled: false, creator_support_enabled: false, desktop_deploy_enabled: false },
  captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
  client: { download_enabled: false, download_url: "", latest_version: "" },
  legal: { current_terms_version: "" },
};

/** 钉公共配置与评论列表；其余请求一律 404。 */
function stubAnonymousFetch() {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url.includes("/api/v1/config/public")) {
      return new Response(JSON.stringify(publicConfig), { status: 200, headers: { "Content-Type": "application/json" } });
    }
    if (url.includes("/api/v1/social/comments")) {
      return new Response(JSON.stringify({ comments: [], total: 0 }), { status: 200, headers: { "Content-Type": "application/json" } });
    }
    if (url.includes("/api/v1/ips/") && url.includes("/discussions")) {
      return new Response(
        JSON.stringify({ discussions: [{ id: 1, title: "seeded", reply_count: 0, created_at: "2026-01-01T00:00:00Z" }] }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }
    return new Response(JSON.stringify({ code: "NOT_FOUND", message: "unstubbed" }), { status: 404, headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;
  return {
    restore() {
      globalThis.fetch = originalFetch;
    },
  };
}

/* 审查修复：Module._load 补丁必须在收尾恢复——单进程串行跑全部测试文件时，
   泄漏的 next/navigation 桩会污染后续文件（后续文件把补丁态当“原始态”保存）。 */
test.after(() => {
  Module._load = originalModuleLoad;
});

test.afterEach(async () => {
  cleanup();
  const { setAccessToken } = await import("@/lib/api");
  setAccessToken(null);
});

test("anonymous entry points open the login modal instead of navigating", async (t) => {
  await t.test("FollowButton opens the gate", async () => {
    installDom();
    const stub = stubAnonymousFetch();
    const { FollowButton } = await import("@/components/social/FollowButton");
    const view = renderWithGate(<FollowButton targetType="user" targetId={1} />);

    fireEvent.click(within(view.container).getByRole("button", { name: "Follow" }));
    await assertOpensLoginModal(view);
    stub.restore();
  });

  await t.test("MessageComposeButton opens the gate", async () => {
    installDom();
    const stub = stubAnonymousFetch();
    const { MessageComposeButton } = await import("@/components/social/MessageComposeButton");
    const view = renderWithGate(<MessageComposeButton userId={2} displayName="peer" />);

    fireEvent.click(within(view.container).getByRole("button", { name: "Message" }));
    await assertOpensLoginModal(view);
    stub.restore();
  });

  await t.test("ReactionBar like button is clickable when anonymous and opens the gate", async () => {
    installDom();
    const stub = stubAnonymousFetch();
    const { ReactionBar } = await import("@/components/social/ReactionBar");
    const view = renderWithGate(<ReactionBar contentId={42} initialLikes={3} />);

    const likeButton = within(view.container).getByRole("button", { name: /3/ });
    assert.equal((likeButton as HTMLButtonElement).disabled, false, "anonymous like must be clickable (gate opens)");
    fireEvent.click(likeButton);
    await assertOpensLoginModal(view);
    stub.restore();
  });

  await t.test("CommentSection login prompt has a button that opens the gate", async () => {
    installDom();
    const stub = stubAnonymousFetch();
    const { CommentSection } = await import("@/components/social/CommentSection");
    const view = renderWithGate(<CommentSection contentId={42} />);

    const loginButton = await waitFor(() =>
      within(view.container).getByRole("button", { name: "Log in" }),
    );
    fireEvent.click(loginButton);
    await assertOpensLoginModal(view);
    stub.restore();
  });

  await t.test("DiscussionBoard start button opens the gate", async () => {
    installDom();
    const stub = stubAnonymousFetch();
    const { DiscussionBoard } = await import("@/components/social/DiscussionBoard");
    // startEntry 登录分支仅 compact + 有讨论时渲染（与生产渲染条件一致）。
    const view = renderWithGate(<DiscussionBoard ipId={9} compact />);

    const startButton = await waitFor(() =>
      within(view.container).getByRole("button", { name: "New Post" }),
    );
    fireEvent.click(startButton);
    await assertOpensLoginModal(view);
    stub.restore();
  });
});
