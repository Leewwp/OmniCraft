import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { fetchContents, fetchIPs } from "@/lib/home-page-data";
import { cleanup, installDom, render } from "./runtime-test-helpers";

// 接缝 2（票 #853 / spec §12.2 二创入口补闭环）：
//   ① 共享取数逻辑 = 原 `/` 页同款请求（/fanworks 复用，不复制实现）；
//   ② 全站 Header「二创区」入口：游客 → /fanworks，登录用户 → /（现状）；
//   ③ 落地页三大页面章的二创入口（点击路由/简介链接/触屏进入按钮）全部
//      指向 /fanworks，无「游客点回落地页」自循环。

const Module = require("node:module") as { _load: (request: string, parent: unknown, isMain: boolean) => unknown };
const originalModuleLoad = Module._load;

const messages = enMessages as unknown as Record<string, Record<string, unknown>>;

function withIntl(node: React.ReactNode) {
  return React.createElement(
    IntlProvider,
    { locale: "en", messages, children: node },
  );
}

test("shared fanworks fetchers reuse the original / request contract", async () => {
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    calls.push(url);
    if (url.includes("/contents")) {
      return new Response(
        JSON.stringify({
          contents: [
            { id: 11, title: "同人一", zone: "fanwork", cover_image_url: "/c/11.jpg" },
          ],
          total: 1,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }
    return new Response(JSON.stringify({ ips: [{ id: 3, name: "IP三" }] }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  try {
    const contents = await fetchContents("http://api.test/api/v1");
    const ips = await fetchIPs("http://api.test/api/v1");
    assert.equal(calls[0], "http://api.test/api/v1/contents?zone=fanwork&sort=hot&time_range=all&page=1&page_size=12");
    assert.equal(calls[1], "http://api.test/api/v1/ips?sort=hot&page_size=20");
    assert.equal(contents.items[0].id, 11);
    assert.equal(contents.total, 1);
    assert.equal(ips[0].id, 3);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

// —— Header 二创入口分流 ——
let authStub: { user: unknown; isLoading: boolean } = { user: null, isLoading: false };
const routerPushes: string[] = [];

function installLandingStubs() {
  Module._load = function patchedLoad(request: string, parent: unknown, isMain: boolean) {
    if (request === "@/contexts/AuthContext") {
      return { useAuth: () => authStub };
    }
    if (request === "next/navigation") {
      return {
        useRouter: () => ({
          push: (url: string) => routerPushes.push(url),
          replace: (url: string) => routerPushes.push(url),
        }),
        usePathname: () => "/fanworks",
        useSearchParams: () => new URLSearchParams(),
      };
    }
    if (request === "next-themes") {
      return { useTheme: () => ({ theme: "light", resolvedTheme: "light", setTheme: () => {} }) };
    }
    // AgentFeatureGate / NotificationDropdown 在单测里退化为最小桩（本票只断言入口 href）。
    if (request === "@/components/agent/AgentFeatureGate") {
      return { AgentFeatureGate: ({ children }: { children?: React.ReactNode }) => children ?? null };
    }
    if (request === "@/components/social/NotificationDropdown") {
      return { NotificationDropdown: () => null };
    }
    if (request === "@/components/search/GlobalSearchInput") {
      return {
        GlobalSearchInput: (_props: Record<string, unknown>) =>
          React.createElement("div", { "data-testid": "global-search-stub" }),
      };
    }
    return originalModuleLoad.call(this, request, parent, isMain);
  };
}

test("header fanwork-zone entry sends guests to /fanworks (no landing self-loop)", async () => {
  installDom();
  installLandingStubs();
  authStub = { user: null, isLoading: false };
  const { Header } = await import("@/components/layout/Header");
  const view = render(withIntl(React.createElement(Header)));
  const entry = view.getByTestId("header-fanworks-entry");
  assert.equal(entry.getAttribute("href"), "/fanworks");
  cleanup();
});

test("header fanwork-zone entry keeps the signed-in / route", async () => {
  installDom();
  installLandingStubs();
  authStub = { user: { id: 7, username: "seed" }, isLoading: false };
  const { Header } = await import("@/components/layout/Header");
  const view = render(withIntl(React.createElement(Header)));
  const entry = view.getByTestId("header-fanworks-entry");
  assert.equal(entry.getAttribute("href"), "/");
  cleanup();
  Module._load = originalModuleLoad;
});

// —— 落地页二创入口全部指向 /fanworks ——
test("landing surface pane routes fanworks entries to /fanworks (click route, desc link, touch enter)", async () => {
  installDom();
  installLandingStubs();
  routerPushes.length = 0;
  // JSDOM 的 matchMedia 恒返回 matches:false，isFinePointerDevice 稳定归 false
  // （触屏语义），正好验证触屏路径。
  const { GuestLanding } = await import("@/components/landing/GuestLanding");
  const view = render(withIntl(React.createElement(GuestLanding)));

  const fanworksPane = view.getByTestId("surface-pane-fanworks");
  assert.equal(fanworksPane.getAttribute("data-goto"), "/fanworks", "pane click route must be /fanworks");

  const descLink = view.getByTestId("surface-link-fanworks");
  assert.equal(descLink.getAttribute("href"), "/fanworks", "desc arrow link must be /fanworks");

  // 触屏「进入」按钮：点击 → 单出口导航到 /fanworks
  const enterButton = view.getByTestId("surface-enter-fanworks");
  enterButton.click();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.ok(routerPushes.includes("/fanworks"), `touch enter must navigate to /fanworks, pushes=${routerPushes.join(",")}`);

  // 原创区/AI 入口保持既有路由
  assert.equal(view.getByTestId("surface-pane-original").getAttribute("data-goto"), "/original");
  assert.equal(view.getByTestId("surface-pane-agent").getAttribute("data-goto"), "/agent");

  // hero 的 AI 入口不带轮数文案（Q21），登录门由 /agent 既有守卫承担
  const heroAi = view.getByTestId("hero-ai");
  assert.equal(heroAi.getAttribute("href"), "/agent");
  assert.doesNotMatch(heroAi.textContent ?? "", /3|三/, "hero copy must not mention round counts");

  // 主 CTA 与品牌入口 → /recommend
  assert.equal(view.getByTestId("hero-enter").getAttribute("href"), "/recommend");
  cleanup();
  Module._load = originalModuleLoad;
});
