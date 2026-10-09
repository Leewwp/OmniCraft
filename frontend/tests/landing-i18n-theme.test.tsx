import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import zhMessages from "@/messages/zh.json";

import { cleanup, installDom, fireEvent, render, act, waitFor } from "./runtime-test-helpers";

// 接缝 5（票 #853）：
//   ① landing 命名空间 zh/en 键集合一致（无漏翻）；
//   ② 顶栏语言/主题控件 = 产品 Header 同款 DropdownMenu 形态：Globe 触发、
//      文案键 nav.langZh/langEn、主题三态 nav.themeLight/Dark/System、
//      当前项高亮、点选才切换（点触发钮不立即切换）、setTheme 三态可达。

const en = enMessages as unknown as Record<string, Record<string, string>>;
const zh = zhMessages as unknown as Record<string, unknown>;

function leafKeys(value: unknown, prefix = ""): Set<string> {
  const keys = new Set<string>();
  if (value !== null && typeof value === "object") {
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const path = prefix ? `${prefix}.${k}` : k;
      if (v !== null && typeof v === "object") {
        for (const child of leafKeys(v, path)) keys.add(child);
      } else {
        keys.add(path);
      }
    }
  }
  return keys;
}

test("landing namespace has identical key sets in zh and en", () => {
  const zhKeys = leafKeys(zh.landing, "landing");
  const enKeys = leafKeys(en.landing, "landing");
  const missingInEn = [...zhKeys].filter((k) => !enKeys.has(k));
  const missingInZh = [...enKeys].filter((k) => !zhKeys.has(k));
  assert.deepEqual(missingInEn, [], "keys missing in en");
  assert.deepEqual(missingInZh, [], "keys missing in zh");
  assert.ok(
    zhKeys.size >= 140,
    `landing namespace should be fully populated (incl. landing.demo.*), got ${zhKeys.size}`,
  );
  assert.ok(
    [...zhKeys].some((k) => k.startsWith("landing.demo.surfaces.")),
    "demo surfaces keys must exist",
  );
  assert.ok(
    [...zhKeys].some((k) => k.startsWith("landing.demo.mechanics.")),
    "demo mechanics keys must exist",
  );
});

test("landing never reuses demo/prototype-only wording", () => {
  const zhKeys = leafKeys(zh.landing, "landing");
  for (const key of zhKeys) {
    const value = key
      .split(".")
      .reduce<unknown>((acc, part) => (acc as Record<string, unknown>)?.[part], zh);
    const text = String(value);
    assert.doesNotMatch(text, /原型|占位|演示环境|测试语料|app\.leeppp/, `${key} carries prototype-only wording`);
  }
});

// —— 主题三态控件 ——
const Module = require("node:module") as { _load: (request: string, parent: unknown, isMain: boolean) => unknown };
const originalModuleLoad = Module._load;

const setThemeCalls: string[] = [];
let themeStubValue = "light";

function installTopBarStubs() {
  Module._load = function patchedLoad(request: string, parent: unknown, isMain: boolean) {
    if (request === "next-themes") {
      return {
        useTheme: () => ({
          theme: themeStubValue,
          resolvedTheme: themeStubValue === "system" ? "light" : themeStubValue,
          setTheme: (value: string) => setThemeCalls.push(value),
        }),
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
}

async function renderTopBar() {
  const { LandingTopBar } = await import("@/components/landing/LandingTopBar");
  return render(
    React.createElement(IntlProvider, {
      locale: "en",
      messages: en as unknown as Record<string, Record<string, unknown>>,
      children: React.createElement(LandingTopBar),
    }),
  );
}

test("theme switcher exposes the three-state product menu and only switches on item click", async () => {
  installDom();
  installTopBarStubs();
  setThemeCalls.length = 0;
  themeStubValue = "light";
  const view = await renderTopBar();

  const trigger = view.getByRole("button", { name: en.nav.themeSwitch });
  assert.equal(trigger.getAttribute("aria-haspopup"), "menu");

  // 点触发钮 = 只开菜单，不切换（R5-③ 与产品逻辑一致）
  await act(async () => {
    fireEvent.click(trigger);
  });
  assert.deepEqual(setThemeCalls, [], "opening the menu must not switch the theme");

  // 菜单展开后三态可达（Globe/主题两个菜单复用产品 DropdownMenu）
  const darkItem = await waitFor(() => {
    const item = view.getByText(en.nav.themeDark);
    assert.ok(item);
    return item;
  });
  await act(async () => {
    fireEvent.click(darkItem);
  });
  assert.deepEqual(setThemeCalls, ["dark"]);

  cleanup();
  Module._load = originalModuleLoad;
});

test("surface/mechanics demo innards are i18n-rendered, never hardcoded (MAJOR-2 spot check)", async () => {
  installDom();
  installTopBarStubs();
  const { SurfaceStage } = await import("@/components/landing/SurfaceStage");
  const { MechanicsGrid } = await import("@/components/landing/MechanicsGrid");

  const cjk = /[一-鿿]/;

  const enMsgs = enMessages as unknown as Record<string, Record<string, unknown>>;
  for (const [locale, messages] of [
    ["en", enMsgs] as const,
    ["zh", zhMessages as unknown as Record<string, Record<string, unknown>>] as const,
  ]) {
    const view = render(
      React.createElement(
        IntlProvider,
        { locale, messages: messages as never, children: React.createElement(React.Fragment, null, React.createElement(SurfaceStage), React.createElement(MechanicsGrid)) },
      ),
    );
    const text = view.container.textContent ?? "";
    const zhResidue = [...text.matchAll(/[一-鿿]{1,12}/g)].map((m) => m[0]);
    if (locale === "en") {
      assert.equal(
        zhResidue.length,
        0,
        `en render must contain no hardcoded zh demo text, got: ${zhResidue.slice(0, 8).join(" / ")}`,
      );
      assert.ok(text.includes("Mountain Marks"), "en demo card copy must come from messages");
      assert.ok(text.includes("Judge vote in progress"), "en mechanics demo copy must come from messages");
    } else {
      assert.ok(zhResidue.length > 0, "zh render should show zh demo copy");
      assert.ok(text.includes("山痕 · 版画系列 No.7"));
      assert.ok(text.includes("判官投票进行中"));
    }
    cleanup();
  }
  Module._load = originalModuleLoad;
});

test("language switcher uses the product nav.langZh/langEn menu keys", async () => {
  installDom();
  installTopBarStubs();
  const view = await renderTopBar();

  const trigger = view.getByRole("button", { name: en.nav.language });
  await act(async () => {
    fireEvent.click(trigger);
  });
  assert.ok(view.getByText(en.nav.langZh), "langZh item visible");
  assert.ok(view.getByText(en.nav.langEn), "langEn item visible");

  cleanup();
  Module._load = originalModuleLoad;
});
