import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { cleanup, installDom, render } from "./runtime-test-helpers";

// 接缝 1（票 #853 / spec §12.2 双态由真实会话决定）：
//   ① 未登录（无 refresh cookie）→ 落地页，恢复中也不闪二创页；
//   ② 恢复中（有 cookie、isLoading）→ 中性壳，既不闪落地页也不闪二创页；
//   ③ 已登录 → 原二创页（现状零变化）；
//   ④ 伪/过期 cookie（恢复完成仍无 user）→ 归匿名落地页。
// AuthProvider 用桩替代（content-detail-overlay 的 Module._load 拦截先例）。
// 组件必须在补丁安装后动态导入（见文末说明与先例注释）。

const requireForMocks = require("node:module") as unknown as {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const Module = requireForMocks;
const originalModuleLoad = Module._load;

type AuthStub = { user: unknown; isLoading: boolean };
let authStub: AuthStub = { user: null, isLoading: true };
let guestLandingMounts = 0;

Module._load = function patchedLoad(request: string, parent: unknown, isMain: boolean) {
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
    };
  }
  // 落地页本体在门测试里用计数桩替代：本文件只验证门的分支，不验证落地页内容。
  if (request === "@/components/landing/GuestLanding") {
    return {
      GuestLanding: () => {
        guestLandingMounts += 1;
        return React.createElement("div", { "data-testid": "guest-landing" });
      },
    };
  }
  return originalModuleLoad.call(this, request, parent, isMain);
};

const messages = {
  common: { loading: "Loading" },
  landing: (enMessages as { landing: Record<string, unknown> }).landing,
  nav: (enMessages as { nav: Record<string, unknown> }).nav,
};

function AuthenticatedHome() {
  return React.createElement("div", { "data-testid": "authenticated-home" });
}

async function renderGate(hasRefreshCookie: boolean) {
  const { RootAuthGate } = await import("@/components/landing/RootAuthGate");
  return render(
    React.createElement(IntlProvider, {
      locale: "en",
      messages,
      children: React.createElement(RootAuthGate, {
        hasRefreshCookie,
        authenticatedView: React.createElement(AuthenticatedHome),
      }),
    }),
  );
}

test.afterEach(() => {
  // 与姊妹文件卫生一致：无论用例成败都恢复模块加载器，防桩泄漏到其他文件。
  Module._load = originalModuleLoad;
});

test("no refresh cookie renders the landing immediately, even while auth is restoring", async () => {
  installDom();
  authStub = { user: null, isLoading: true };
  const view = await renderGate(false);
  assert.ok(view.getByTestId("guest-landing"), "landing must show without waiting for restore");
  assert.equal(view.queryByTestId("authenticated-home"), null);
  assert.equal(view.queryByTestId("root-auth-shell"), null);
  cleanup();
});

test("refresh cookie present + restore in progress renders the neutral shell only", async () => {
  installDom();
  guestLandingMounts = 0;
  authStub = { user: null, isLoading: true };
  const view = await renderGate(true);
  assert.ok(view.getByTestId("root-auth-shell"), "neutral shell while restoring");
  assert.equal(view.queryByTestId("guest-landing"), null, "must not flash the landing");
  assert.equal(view.queryByTestId("authenticated-home"), null, "must not flash the fanworks page");
  assert.equal(guestLandingMounts, 0);
  cleanup();
});

test("restored signed-in session renders the original fanworks page unchanged", async () => {
  installDom();
  authStub = { user: { id: 7, username: "seed" }, isLoading: false };
  const view = await renderGate(true);
  assert.ok(view.getByTestId("authenticated-home"));
  assert.equal(view.queryByTestId("guest-landing"), null);
  assert.equal(view.queryByTestId("root-auth-shell"), null);
  cleanup();
});

test("fake or expired cookie (restore finished without user) falls back to the landing", async () => {
  installDom();
  authStub = { user: null, isLoading: false };
  const view = await renderGate(true);
  assert.ok(view.getByTestId("guest-landing"));
  assert.equal(view.queryByTestId("root-auth-shell"), null);
  assert.equal(view.queryByTestId("authenticated-home"), null);
  cleanup();
});

test("neutral shell exposes an aria-busy status instead of interactive surface content", async () => {
  installDom();
  authStub = { user: null, isLoading: true };
  const view = await renderGate(true);
  const shell = view.getByTestId("root-auth-shell");
  assert.equal(shell.getAttribute("aria-busy"), "true");
  cleanup();
});
