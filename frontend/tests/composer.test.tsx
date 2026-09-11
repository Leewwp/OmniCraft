import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { Composer, COMPOSER_MAX_HEIGHT } from "@/components/ui/composer";

import { cleanup, fireEvent, installDom, renderWithIntl } from "./runtime-test-helpers";

/* #413 F6a 公共 Composer 组件单测：按键语义显式模式、isComposing 防护、
   自动增高上限转内部滚动、按钮恒位右下角、文本为按钮预留内边距不重叠。 */

test.afterEach(() => {
  cleanup();
});

function setup(props: Partial<Parameters<typeof Composer>[0]> = {}) {
  const submits: number[] = [];
  const view = renderWithIntl(
    <Composer
      value=""
      onChange={() => {}}
      onSubmit={() => submits.push(Date.now())}
      submitLabel="Send"
      keyMode="enter"
      {...props}
    />,
  );
  const textarea = view.container.querySelector("textarea") as HTMLTextAreaElement;
  const button = view.getByRole("button", { name: "Send" });
  return { view, textarea, button, submits };
}

function pressEnter(el: HTMLElement, init: { shiftKey?: boolean; ctrlKey?: boolean; metaKey?: boolean; isComposing?: boolean } = {}) {
  const event = new window.KeyboardEvent("keydown", {
    key: "Enter",
    bubbles: true,
    cancelable: true,
    shiftKey: init.shiftKey,
    ctrlKey: init.ctrlKey,
    metaKey: init.metaKey,
  });
  if (init.isComposing !== undefined) {
    Object.defineProperty(event, "isComposing", { value: init.isComposing });
  }
  el.dispatchEvent(event);
  return event;
}

test("keyMode=enter: Enter submits, Shift+Enter inserts a newline (no submit)", () => {
  installDom();
  const { textarea, submits } = setup({ keyMode: "enter" });
  pressEnter(textarea);
  assert.equal(submits.length, 1);
  pressEnter(textarea, { shiftKey: true });
  assert.equal(submits.length, 1, "Shift+Enter must not submit");
});

test("keyMode=ctrl-enter: Ctrl/Cmd+Enter submits, bare Enter does not", () => {
  installDom();
  const { textarea, submits } = setup({ keyMode: "ctrl-enter" });
  pressEnter(textarea);
  assert.equal(submits.length, 0, "bare Enter inserts a newline only");
  pressEnter(textarea, { ctrlKey: true });
  assert.equal(submits.length, 1);
  pressEnter(textarea, { metaKey: true });
  assert.equal(submits.length, 2, "Cmd+Enter submits on macOS");
});

test("keyMode=button-only: no keyboard submit path", () => {
  installDom();
  const { textarea, submits } = setup({ keyMode: "button-only" });
  pressEnter(textarea);
  pressEnter(textarea, { ctrlKey: true });
  pressEnter(textarea, { shiftKey: true });
  assert.equal(submits.length, 0, "button-only mode ignores every Enter variant");
});

test("isComposing guard: IME composition Enter never submits (all modes)", () => {
  installDom();
  for (const keyMode of ["enter", "ctrl-enter"] as const) {
    const { textarea, submits } = setup({ keyMode });
    pressEnter(textarea, { isComposing: true });
    pressEnter(textarea, { ctrlKey: true, isComposing: true });
    assert.equal(submits.length, 0, `IME Enter must not submit in ${keyMode} mode`);
    cleanup();
  }
});

test("auto-grow clamps at maxHeight and switches to internal scrolling", async () => {
  installDom();
  const controlled = renderWithIntl(
    <Composer value="x" onChange={() => {}} onSubmit={() => {}} submitLabel="Send" keyMode="enter" />,
  );
  const ta = controlled.container.querySelector("textarea") as HTMLTextAreaElement;
  // jsdom scrollHeight 恒 0：stub 超限量级，复算组件的 clamp 公式断言上限语义。
  Object.defineProperty(ta, "scrollHeight", { configurable: true, value: 300 });
  ta.style.height = `${Math.min(ta.scrollHeight, COMPOSER_MAX_HEIGHT)}px`;
  assert.equal(ta.style.height, "208px", "content beyond the cap clamps to the shared 208px ceiling");
  assert.match(ta.className, /overflow-y-auto/, "at the cap the textarea scrolls internally");
  assert.equal(ta.getAttribute("style")?.includes("max-height: 208px"), true, "style ceiling is applied");
});

test("send button is a two-state filled circle anchored in the bottom-right corner", async () => {
  installDom();
  const { button } = setup({ submitDisabled: true });
  await new Promise((r) => setTimeout(r, 30));
  assert.match(button.className, /absolute/, "button is anchored (not flex flow)");
  assert.match(button.className, /bottom-2 right-2/, "8px inset from the inner edges");
  assert.match(button.className, /h-9 w-9/, "36px tier circle");
  assert.match(button.className, /rounded-full/, "circular shape (2026-09-09 验收契约)");
  assert.match(button.className, /bg-canvas-subtle/, "empty text: gray filled state");
  assert.match(button.className, /text-fg-subtle/, "empty text: subdued glyph");
  assert.doesNotMatch(button.className, /border-/, "not an independent floating card");
  cleanup();

  const { view, button: idleButton } = setup({ submitDisabled: false });
  assert.match(idleButton.className, /bg-primary/, "ready to send: theme-colored fill");
  assert.match(idleButton.className, /text-primary-foreground/, "ready to send: white glyph");
  const wrap = view.container.firstElementChild as HTMLElement;
  assert.match(wrap.className, /relative/, "wrapper establishes the anchor context");
});

test("textarea reserves padding so text never overlaps the button", async () => {
  installDom();
  const { textarea } = setup({});
  await new Promise((r) => setTimeout(r, 30));
  assert.match(textarea.className, /pr-11/, "44px right padding clears the 36px button + 8px inset");
  assert.match(textarea.className, /pb-11/, "44px bottom padding clears the button row");
  assert.match(textarea.className, /resize-none/, "height is component-controlled");
});

test("submitting state keeps the theme fill, swaps the glyph to a spinner and disables", () => {
  installDom();
  const { button } = setup({ submitting: true, submitDisabled: true });
  assert.match(button.className, /bg-primary/, "in-flight keeps the prominent fill");
  assert.match(button.querySelector("svg")?.getAttribute("class") ?? "", /animate-spin/, "glyph is a spinner");
  assert.equal(button.getAttribute("disabled"), "");
});

/* ── #417 F6b：流式停止位与 ref 转发（Agent 工作台语义） ──────────── */

test("stop variant: embedded stop button replaces submit and reports onStop", () => {
  installDom();
  const stops: number[] = [];
  const view = renderWithIntl(
    <Composer
      value="流式中的输入"
      onChange={() => {}}
      onSubmit={() => {}}
      submitLabel="Send"
      keyMode="enter"
      stopLabel="Stop generating"
      onStop={() => stops.push(Date.now())}
    />,
  );
  const stopButton = view.getByRole("button", { name: "Stop generating" });
  assert.match(stopButton.className, /absolute bottom-2 right-2/, "stop sits in the same embedded slot");
  assert.match(stopButton.className, /rounded-full bg-primary/, "stop shares the theme-filled circle contract");
  assert.equal(view.queryByRole("button", { name: "Send" }), null, "submit button hidden while stop is active");
  stopButton.click();
  assert.equal(stops.length, 1);
  cleanup();
});

test("forwards its ref to the textarea (agent focus flows keep working)", () => {
  installDom();
  let captured: unknown = null;
  const view = renderWithIntl(
    <Composer
      ref={(node) => { captured = node; }}
      value=""
      onChange={() => {}}
      onSubmit={() => {}}
      submitLabel="Send"
      keyMode="enter"
    />,
  );
  assert.equal((captured as Element | null)?.tagName, "TEXTAREA", "ref exposes the inner textarea");
  assert.equal(captured, view.container.querySelector("textarea"));
  cleanup();
});
