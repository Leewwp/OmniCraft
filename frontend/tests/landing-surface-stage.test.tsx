import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import {
  HIT_PAD_PX,
  TAP_PREACTIVATION_MS,
  resolveHitPane,
  resolvePaneAction,
  type PaneHitInput,
} from "@/lib/landing-interactions";
import { cleanup, installDom, fireEvent, render, act } from "./runtime-test-helpers";

// 接缝 4（票 #853，R5 Q23′-A + §十一②）：
//   纯函数：动态命中区（配图+配文包围盒、空白不触发、激活粘性、缩放后边界跟随）
//   与点击分流（桌面导航 / 触屏 activate-collapse + 预激活防抖）。
//   组件：键盘 Enter/空格恒导航（修正 R5 原型缺陷）、桌面点击导航、
//   触屏 tap 放大、离舞台复位。几何动效由真 Chromium 验收补足。

const messages = enMessages as unknown as Record<string, Record<string, unknown>>;

function rect(left: number, top: number, right: number, bottom: number) {
  return { left, top, right, bottom };
}

function paneHit(id: string, frame: ReturnType<typeof rect>, label: ReturnType<typeof rect>, isOn = false): PaneHitInput {
  return { id, frameRect: frame, labelRect: label, isOn };
}

// ---------- 纯函数：动态命中区 ----------

test("hit test triggers only inside frame+label bounding boxes, blank space never hits", () => {
  const panes = [
    paneHit("a", rect(0, 100, 200, 300), rect(0, 304, 200, 330)),
    paneHit("b", rect(220, 100, 420, 300), rect(220, 304, 420, 330)),
  ];
  // 配图中心命中 a
  assert.equal(resolveHitPane(panes, 100, 200), "a");
  // 配文也算命中区（R5-②：配图+配文包围盒）
  assert.equal(resolveHitPane(panes, 100, 320), "a");
  // 两窗之间空白（x=210）不命中任何页
  assert.equal(resolveHitPane(panes, 210, 200), null);
  // 窗顶空白带（原型 R4 的误触发区，y=50）不命中
  assert.equal(resolveHitPane(panes, 100, 50), null);
});

test("hit test pads the box and lets the active pane win when boundaries overlap (sticky)", () => {
  const panes = [
    paneHit("a", rect(0, 100, 200, 300), rect(0, 304, 200, 330), true),
    paneHit("b", rect(196, 100, 400, 300), rect(196, 304, 400, 330)),
  ];
  // a 的外扩盒到 208；x=209 已出 a → 命中 b
  assert.equal(resolveHitPane(panes, 209, 200, HIT_PAD_PX), "b");
  // 重叠区（x=207 同时在 a 外扩盒与 b 内）→ 当前激活者 a 优先（粘性防抖）
  assert.equal(resolveHitPane(panes, 207, 200, HIT_PAD_PX), "a");
  // 外扩关闭后 x=201 只落在 b 的框内
  assert.equal(resolveHitPane(panes, 201, 200, 0), "b");
  // 外扩关闭后边界外不再命中
  assert.equal(resolveHitPane(panes, 601, 200, 0), null);
});

test("hit area follows the scaled visual bounds (enlarged pane has a larger hit box)", () => {
  // 同一 pane 激活放大后 rect 变大 → 命中区自动跟随（getBoundingClientRect 含 transform 的调用约定）
  const small = paneHit("a", rect(50, 100, 150, 200), rect(50, 204, 150, 230));
  const large = paneHit("a", rect(0, 60, 220, 260), rect(0, 264, 220, 290));
  assert.equal(resolveHitPane([small], 20, 80), null, "outside small bounds");
  assert.equal(resolveHitPane([large], 20, 80), "a", "inside enlarged bounds");
});

// ---------- 纯函数：点击分流 ----------

test("pane action: fine pointer click always navigates", () => {
  assert.equal(resolvePaneAction({ finePointer: true, isOn: true, msSincePreActivated: 0 }), "navigate");
  assert.equal(resolvePaneAction({ finePointer: true, isOn: false, msSincePreActivated: 99999 }), "navigate");
});

test("pane action: touch tap activates, second tap collapses only after the debounce window", () => {
  assert.equal(resolvePaneAction({ finePointer: false, isOn: false, msSincePreActivated: 99999 }), "activate");
  // 触屏 tap 先合成 focus 再发 click（R4 QA 实证）：刚预激活就来的 click 不得收起
  assert.equal(
    resolvePaneAction({ finePointer: false, isOn: true, msSincePreActivated: TAP_PREACTIVATION_MS - 1 }),
    "ignore",
  );
  assert.equal(
    resolvePaneAction({ finePointer: false, isOn: true, msSincePreActivated: TAP_PREACTIVATION_MS + 1 }),
    "collapse",
  );
});

// ---------- 组件接线 ----------

const Module = require("node:module") as { _load: (request: string, parent: unknown, isMain: boolean) => unknown };
const originalModuleLoad = Module._load;

const routerPushes: string[] = [];
let finePointer = true;

function installStageStubs() {
  Module._load = function patchedLoad(request: string, parent: unknown, isMain: boolean) {
    if (request === "next/navigation") {
      return {
        useRouter: () => ({
          push: (url: string) => routerPushes.push(url),
          replace: (url: string) => routerPushes.push(url),
        }),
        usePathname: () => "/",
        useSearchParams: () => new URLSearchParams(),
      };
    }
    return originalModuleLoad.call(this, request, parent, isMain);
  };
}

function stubMatchMedia(fine: boolean) {
  const w = window as unknown as { matchMedia?: unknown };
  w.matchMedia = (query: string) => ({
    matches: query.includes("(hover: hover)") ? fine : false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  });
}

async function renderStage() {
  const { SurfaceStage } = await import("@/components/landing/SurfaceStage");
  const view = render(
    React.createElement(IntlProvider, {
      locale: "en",
      messages,
      children: React.createElement(SurfaceStage),
    }),
  );
  return view;
}

test("desktop: clicking a pane navigates to its route via the single exit", async () => {
  installDom();
  installStageStubs();
  stubMatchMedia(true);
  routerPushes.length = 0;
  const view = await renderStage();
  fireEvent.click(view.getByTestId("surface-pane-original"));
  assert.deepEqual(routerPushes, ["/original"]);
  cleanup();
  Module._load = originalModuleLoad;
});

test("keyboard Enter navigates unconditionally — even on touch-semantics devices (R5 defect fix)", async () => {
  installDom();
  installStageStubs();
  stubMatchMedia(false); // 无 fine pointer：原型在此把 Enter 变成预览切换
  routerPushes.length = 0;
  const view = await renderStage();
  const pane = view.getByTestId("surface-pane-fanworks");
  fireEvent.keyDown(pane, { key: "Enter" });
  assert.deepEqual(routerPushes, ["/fanworks"], "Enter must navigate regardless of pointer style");
  // 空格同样导航
  fireEvent.keyDown(pane, { key: " " });
  assert.deepEqual(routerPushes, ["/fanworks", "/fanworks"]);
  cleanup();
  Module._load = originalModuleLoad;
});

test("touch: first tap enlarges the pane without navigating; immediate second tap keeps it open", async () => {
  installDom();
  installStageStubs();
  stubMatchMedia(false);
  routerPushes.length = 0;
  const view = await renderStage();
  const pane = view.getByTestId("surface-pane-agent");
  const stage = view.getByTestId("surface-stage");

  fireEvent.click(pane);
  assert.deepEqual(routerPushes, [], "touch tap must not navigate");
  assert.ok(pane.className.includes("on"), "pane must be activated");
  assert.ok(stage.className.includes("live"), "stage must be live");

  // 合成 focus+click 的第二击（600ms 防抖窗口内）不得收起
  fireEvent.click(pane);
  assert.ok(pane.className.includes("on"), "pre-activated debounce must swallow the synthetic click");

  cleanup();
  Module._load = originalModuleLoad;
});

test("desktop: leaving the stage resets the active pane (mouseout reset)", async () => {
  installDom();
  installStageStubs();
  stubMatchMedia(true);
  const view = await renderStage();
  const pane = view.getByTestId("surface-pane-original");
  const stage = view.getByTestId("surface-stage");

  act(() => {
    pane.dispatchEvent(new window.MouseEvent("mouseover", { bubbles: true }));
  });
  // 桌面下 hover 激活走 mousemove 命中测试：模拟命中原点（jsdom rect 全 0，
  // 命中区为 [-8,8]²，客户端坐标 0,0 命中第一个 pane）。命中测试经 rAF 合并，
  // 断言前等待下一帧（helpers 把 rAF 装配为 setTimeout 0）。
  fireEvent.mouseMove(stage, { clientX: 0, clientY: 0 });
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  assert.ok(pane.className.includes("on"), "mousemove inside bounds must activate");

  fireEvent.mouseLeave(stage);
  assert.ok(!pane.className.includes("on"), "leaving the stage must reset");
  assert.ok(!stage.className.includes("live"));
  cleanup();
  Module._load = originalModuleLoad;
});
