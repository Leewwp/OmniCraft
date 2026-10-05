import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { usePersistentState } from "@/lib/use-persistent-state";
import { act, cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";

test.beforeEach(() => {
  installDom();
  window.localStorage.clear();
});

test.afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

/* 探针组件：显示当前值 + 一个写新值的按钮（编码由用例注入）。 */
function Probe({
  storageKey,
  fallback,
  parse,
  serialize,
}: {
  storageKey: string;
  fallback: string;
  parse: (raw: string) => string;
  serialize: (value: string) => string;
}) {
  const [value, update] = usePersistentState<string>({ storageKey, fallback, parse, serialize });
  return (
    <div>
      <output data-testid="value">{value}</output>
      <button type="button" data-testid="write" onClick={() => update(`${value}-next`)}>
        write
      </button>
    </div>
  );
}

test("存储值在 effect 后置应用（首帧 fallback，hydration 安全）", () => {
  window.localStorage.setItem("probe.key", "stored");
  const view = render(<Probe storageKey="probe.key" fallback="fb" parse={(raw) => `s:${raw}`} serialize={(v) => v} />);
  assert.equal(view.getByTestId("value").textContent, "s:stored");
});

test("缺键时保持 fallback，不写回存储", () => {
  const view = render(<Probe storageKey="probe.missing" fallback="fb" parse={(raw) => raw} serialize={(v) => v} />);
  assert.equal(view.getByTestId("value").textContent, "fb");
  assert.equal(window.localStorage.getItem("probe.missing"), null);
});

test("setter 先持久化再更新状态（编码经 serialize）", () => {
  const view = render(<Probe storageKey="probe.write" fallback="a" parse={(raw) => raw} serialize={(v) => `e:${v}`} />);
  act(() => {
    fireEvent.click(view.getByTestId("write"));
  });
  assert.equal(view.getByTestId("value").textContent, "a-next");
  assert.equal(window.localStorage.getItem("probe.write"), "e:a-next");
});

test("存储不可用时退化为纯内存状态（读写均不抛）", () => {
  const originalGetItem = window.localStorage.getItem.bind(window.localStorage);
  const originalSetItem = window.localStorage.setItem.bind(window.localStorage);
  Object.defineProperty(window.localStorage, "getItem", {
    configurable: true,
    value: () => {
      throw new Error("storage unavailable");
    },
  });
  Object.defineProperty(window.localStorage, "setItem", {
    configurable: true,
    value: () => {
      throw new Error("storage unavailable");
    },
  });
  try {
    const view = render(<Probe storageKey="probe.quota" fallback="fb" parse={(raw) => raw} serialize={(v) => v} />);
    assert.equal(view.getByTestId("value").textContent, "fb");
    act(() => {
      fireEvent.click(view.getByTestId("write"));
    });
    assert.equal(view.getByTestId("value").textContent, "fb-next");
  } finally {
    Object.defineProperty(window.localStorage, "getItem", { configurable: true, value: originalGetItem });
    Object.defineProperty(window.localStorage, "setItem", { configurable: true, value: originalSetItem });
  }
});
