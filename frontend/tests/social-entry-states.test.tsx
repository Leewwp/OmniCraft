import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { api } from "@/lib/api";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const pushes: string[] = [];
/* SP-17/T2：未登录改走 auth gate 事件桥（不再 router.push("/login")）。 */
const authGateEmissions: unknown[] = [];
const authStub = {
  user: null as null | { id: number },
  capabilities: { can_interact: false, interaction_denial_reason: "AUTH_STATUS_UNAVAILABLE" },
};

Module._load = function loadWithSocialStubs(request, parent, isMain) {
  if (request === "@/lib/auth-gate") {
    return {
      emitAuthRequired: (detail?: unknown) => {
        authGateEmissions.push(detail);
      },
      setAuthGateActive: () => {},
    };
  }
  if (request === "next/navigation") {
    return { useRouter: () => ({ push: (path: string) => pushes.push(path) }) };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
      interactionDenialKey: (reason?: string) =>
        reason === "INSUFFICIENT_REPUTATION"
          ? "capabilities.deniedInsufficientReputation"
          : "capabilities.deniedUnavailable",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type IPDiscussionsTabComponent = typeof import("@/components/ip/hub/IPDiscussionsTab")["IPDiscussionsTab"];
let IPDiscussionsTab: IPDiscussionsTabComponent;

test.before(async () => {
  ({ IPDiscussionsTab } = await import("@/components/ip/hub/IPDiscussionsTab"));
});

const originalGet = api.get;

test.beforeEach(() => {
  installDom();
  pushes.length = 0;
  authGateEmissions.length = 0;
  authStub.user = null;
  authStub.capabilities = { can_interact: false, interaction_denial_reason: "AUTH_STATUS_UNAVAILABLE" };
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
});

test.after(() => {
  Module._load = originalModuleLoad;
});

/* #494：讨论区唯一在役入口 = IP 枢纽 discussions tab（DiscussionBoard 死代码
   已删，本组断言随门语义一并移植到 IPDiscussionsTab）。 */

test("hub discussion entry sends anonymous visitors to the auth gate (SP-17/T2, #494)", async () => {
  const stub = stubDiscussionsFetch([{ id: 1, title: "Existing discussion" }]);
  const view = renderTab();

  fireEvent.click(await view.findByRole("button", { name: "New Post" }));
  assert.equal(authGateEmissions.length, 1, "gate bridge must receive an emission");
  assert.equal(typeof (authGateEmissions[0] as { pendingAction?: unknown })?.pendingAction, "function");
  assert.deepEqual(pushes, [], "must not navigate to /login");
  stub.restore();
});

test("hub discussion entry links eligible users to the IP-scoped composer", async () => {
  authStub.user = { id: 7 };
  authStub.capabilities = { can_interact: true, interaction_denial_reason: "" };
  const stub = stubDiscussionsFetch([
    { id: 1, title: "Existing discussion", author: { id: 5, username: "seed_author", avatar_url: "/avatars/5.png" } },
  ]);
  const view = renderTab();

  const entry = await view.findByRole("link", { name: "New Post" });
  assert.equal(entry.getAttribute("href"), "/ip/42/discussions/new");

  // 作者身份入口（#494）：列表行作者 = UserHoverCard 触发器（头像 + /user/:id）
  const authorEntry = await view.findByRole("link", { name: "seed_author" });
  assert.equal(authorEntry.getAttribute("href"), "/user/5");
  assert.ok(authorEntry.querySelector("img"), "avatar rendered from author.avatar_url");
  stub.restore();
});

test("hub discussion entry fails closed with the server denial reason", async () => {
  authStub.user = { id: 7 };
  authStub.capabilities = {
    can_interact: false,
    interaction_denial_reason: "INSUFFICIENT_REPUTATION",
  };
  const stub = stubDiscussionsFetch([]);
  const view = renderTab();

  const entry = await view.findByRole("button", { name: "New Post" });
  assert.equal(entry.getAttribute("disabled"), "");
  assert.equal(entry.getAttribute("title"), "Insufficient reputation to perform this action.");
  assert.ok(view.getByText("Insufficient reputation to perform this action."));
  stub.restore();
});

test("FollowButton keeps constant width, solid both states, destructive hover unfollow (#415 O1b)", async () => {
  const source = await readFile(new URL("../components/social/FollowButton.tsx", import.meta.url), "utf8");

  // 已关注与未关注底色一致：不再按状态翻转 variant
  assert.match(source, /variant="default"/);
  assert.doesNotMatch(source, /isFollowing \? "outline"/);
  // 恒宽：最长文案「取消关注」隐藏占位（grid 同格叠放），任何状态宽度不变
  assert.match(source, /invisible col-start-1 row-start-1/);
  assert.match(source, /social\.unfollow/);
  // hover 已关注 → 取消关注 + destructive 红边红字
  assert.match(source, /group-hover:hidden group-focus-visible:hidden/);
  assert.match(source, /group-hover:inline group-focus-visible:inline/);
  assert.match(source, /hover:border-destructive!/);
  // 勾号与加号图标移除（无图标，宽度与视觉恒定）
  assert.doesNotMatch(source, /<Check/);
  assert.doesNotMatch(source, /<Plus/);
  // 既有行为保持：信誉禁用仍禁用+原因（SP-17/T2 起未登录改走 auth gate）
  assert.match(source, /disabled=\{interactionBlocked \|\| busy\}/);
  assert.doesNotMatch(source, /router\.push\("\/login"\)/);
  assert.match(source, /useAuthGate\(\)/);
  assert.match(source, /requireAuth\(/);
  // 提案页一键关注收敛：关注成功回调
  assert.match(source, /onFollowed\?\.\(\)/);
});

function stubDiscussionsFetch(discussions: Array<Record<string, unknown>>) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    assert.ok(
      url.includes("/ips/42/discussions"),
      `unexpected fetch from the hub discussions tab: ${url}`,
    );
    return new Response(JSON.stringify({ discussions }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  return {
    restore() {
      globalThis.fetch = originalFetch;
    },
  };
}

function renderTab() {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <IPDiscussionsTab ipId={42} apiBase="/api/v1" query="" sort="latest_reply" onSortChange={() => {}} />
    </IntlProvider>,
  );
}

/* ── #415 O1b：恒宽结构断言（DOM 级） ─────────────────────────────── */

test("FollowButton renders the hidden unfollow sizer in every state so width cannot flap", async () => {
  installDom();
  const { FollowButton } = await import("../components/social/FollowButton");
  const messages = (await import("../messages/en.json")).default;
  const calls: string[] = [];

  // 复用文件既有的可变 authStub 与 api 补丁模式（避免模块缓存下二次 stub 失效）
  authStub.user = { id: 1 };
  authStub.capabilities = { can_interact: true, interaction_denial_reason: "" };
  const realPost = api.post;
  api.post = (async (p: string) => {
    calls.push(p);
    return {};
  }) as typeof api.post;

  try {
    for (const initialFollowing of [false, true]) {
      const view = render(
        <IntlProvider locale="en" messages={messages}>
          <FollowButton targetType="user" targetId={9} initialFollowing={initialFollowing} />
        </IntlProvider>,
      );
      const button = view.getByRole("button");
      // 隐藏占位格（最长文案 Unfollow）在两种状态下都必须存在
      const sizer = button.querySelector("span.invisible");
      assert.ok(sizer, "hidden unfollow sizer present");
      assert.match(sizer.textContent ?? "", /Unfollow/i);
      // 无图标节点（勾号与加号均已移除）
      assert.equal(button.querySelector("svg"), null);
      cleanup();
    }

    // 关注成功触发 onFollowed（提案页解锁契约）
    let unlocked = false;
    const view = render(
      <IntlProvider locale="en" messages={messages}>
        <FollowButton targetType="ip" targetId={3} initialFollowing={false} onFollowed={() => { unlocked = true; }} />
      </IntlProvider>,
    );
    await waitFor(() => view.getByRole("button").click());
    await waitFor(() => assert.equal(unlocked, true));
    assert.deepEqual(calls, ["/api/v1/ips/3/follow"]);
  } finally {
    api.post = realPost;
    cleanup();
  }
});
