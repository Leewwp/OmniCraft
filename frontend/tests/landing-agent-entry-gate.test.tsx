import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { cleanup, installDom, act } from "./runtime-test-helpers";

// 搭车 #855（#854 §12.3 总闸覆盖落地页入口）：
//   ① guest_agent_enabled=true（服务端投影含 web_agent_enabled 联动）→
//      hero 与尾章「体验 AI 助手」渲染；
//   ② 总闸关闭 → 两处 AI 入口隐藏（§五验收「入口消失」分支）；
//   ③ 配置未知（取数中/失败，config=null）→ 同样隐藏（与 Header
//      agentEntry「未加载不出入口」同构）；
//   ④ 三大页面章 AI 演示窗不受开关影响——目的地 /agent 关闸时即登录引导页
//      （「转登录引导」分支，语义收口见 commit message）。
// 配置源用 Module._load 桩替代（RootAuthGate 门测试先例）；组件必须在
// 补丁安装后动态导入。
const requireForMocks = require("node:module") as unknown as {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const Module = requireForMocks;
const originalModuleLoad = Module._load;

// 桩形态与 PublicConfig.features 对齐；测试只消费 features。
type ConfigStub = { features: Record<string, boolean> } | null;
let configStub: ConfigStub = null;

Module._load = function patchedLoad(request: string, parent: unknown, isMain: boolean) {
  if (request === "@/lib/use-public-config") {
    return {
      usePublicConfig: () => configStub,
    };
  }
  if (request === "next-themes") {
    return {
      useTheme: () => ({ theme: "light", resolvedTheme: "light", setTheme: () => {} }),
    };
  }
  if (request === "next/navigation") {
    return {
      useRouter: () => ({ push: () => {}, replace: () => {} }),
      usePathname: () => "/",
      useSearchParams: () => new URLSearchParams(),
    };
  }
  return originalModuleLoad.call(this, request, parent, isMain);
};

const messages = {
  common: { loading: "Loading" },
  landing: (enMessages as { landing: Record<string, unknown> }).landing,
  nav: (enMessages as { nav: Record<string, unknown> }).nav,
};

async function renderLanding() {
  const { GuestLanding } = await import("@/components/landing/GuestLanding");
  const view = (
    require("./runtime-test-helpers") as typeof import("./runtime-test-helpers")
  ).render(
    React.createElement(IntlProvider, {
      locale: "en",
      messages: messages as unknown as Record<string, Record<string, unknown>>,
      children: React.createElement(GuestLanding),
    }),
  );
  // 让挂载期 effect（scroll-snap 类、IntersectionObserver 兜底）走完；
  // ShowcaseSection 的 fetch 被钉成永不 resolve，不触发 setState、不触网。
  await act(async () => {});
  return view;
}

// 精选陈列取数钉死：本文件只验证 Agent 入口开关，不验证陈列数据；
// 永不 resolve 的 fetch 让 ShowcaseSection 停在未取数态（渲染 null）。
const originalFetch = globalThis.fetch;
function installPendingFetchStub() {
  globalThis.fetch = (async () => new Promise<Response>(() => {})) as typeof fetch;
}
function restoreFetch() {
  globalThis.fetch = originalFetch;
}

function guestFeatures(guestAgentEnabled: boolean): ConfigStub {
  return {
    features: {
      web_agent_enabled: true,
      guest_agent_enabled: guestAgentEnabled,
      payment_enabled: false,
      creator_support_enabled: false,
      desktop_deploy_enabled: false,
    },
  };
}

test.afterEach(() => {
  // 与姊妹文件卫生一致：无论用例成败都恢复模块加载器与 fetch，防桩泄漏。
  Module._load = originalModuleLoad;
  restoreFetch();
});

test("agent entry visible: hero and closing CTA render the AI assistant buttons", async () => {
  installDom();
  installPendingFetchStub();
  configStub = guestFeatures(true);
  const view = await renderLanding();

  assert.ok(view.getByTestId("hero-ai"), "hero AI entry must render when the gate is on");
  const cta = view.getByTestId("landing-cta-section");
  assert.ok(cta.querySelector('a[href="/agent"]'), "closing CTA AI entry must render when the gate is on");
  cleanup();
});

test("gate off: hero and closing CTA AI entries disappear, demo pane untouched", async () => {
  installDom();
  installPendingFetchStub();
  configStub = guestFeatures(false);
  const view = await renderLanding();

  // 注意：断言一律用布尔形式（assert.ok(!query(...))）——assert.equal 的
  // actual 为 jsdom 元素时，失败消息会 inspect 整棵 ownerDocument 树，
  // 在单测进程内准挂起（本文件初版实测 20s+ 不出结果）。
  assert.ok(!view.queryByTestId("hero-ai"), "hero AI entry must hide when the gate is off");
  const cta = view.getByTestId("landing-cta-section");
  assert.ok(
    !cta.querySelector('a[href="/agent"]'),
    "closing CTA AI entry must hide when the gate is off",
  );
  // 三大页面 AI 演示窗（pane 链接/进入按钮）保持展示：目的地 /agent 关闸时
  // 为登录引导页，不随入口隐藏。
  const surfaces = view.getByTestId("landing-surfaces");
  assert.ok(
    surfaces.querySelector('a[href="/agent"]'),
    "AI demo pane link must stay regardless of the gate",
  );
  cleanup();
});

test("config unknown (loading/failed): AI entries stay hidden", async () => {
  installDom();
  installPendingFetchStub();
  configStub = null;
  const view = await renderLanding();

  assert.ok(!view.queryByTestId("hero-ai"), "unknown config must not render the hero AI entry");
  const cta = view.getByTestId("landing-cta-section");
  assert.ok(
    !cta.querySelector('a[href="/agent"]'),
    "unknown config must not render the closing CTA AI entry",
  );
  cleanup();
});
