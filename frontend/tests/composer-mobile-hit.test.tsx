import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { Composer } from "@/components/ui/composer";
import { cleanup, render } from "./runtime-test-helpers";

/* #725：expandMobileHit 参数化——默认 false（私信/评论现状不动）；
 * Agent 启用时发送/停止钮获得移动端 ≥44px 伪元素命中区（视觉仍 36px）；
 * rows prop 传播（Agent 空态 3 行）。 */

function renderWithIntl(ui: React.ReactElement) {
  return render(<IntlProvider locale="en" messages={enMessages as never}>{ui}</IntlProvider>);
}

test("expandMobileHit=false keeps the legacy send button classes (private chat / comments unchanged)", () => {
  const { container } = renderWithIntl(
    <Composer value="hi" onChange={() => undefined} onSubmit={() => undefined} submitLabel="Send" ariaLabel="msg" keyMode="enter" />,
  );
  const send = container.querySelector<HTMLButtonElement>("button[aria-label='Send']");
  assert.ok(send);
  assert.ok(send.className.includes("h-9 w-9"), "36px visual contract stays");
  assert.ok(!send.className.includes("after:-inset-2"), "no hit expansion by default");
});

test("expandMobileHit=true expands the hit area on mobile only, visual stays 36px", () => {
  const { container } = renderWithIntl(
    <Composer
      value="hi" onChange={() => undefined} onSubmit={() => undefined} submitLabel="Send" ariaLabel="msg"
      expandMobileHit stopLabel="Stop" onStop={() => undefined} keyMode="enter"
    />,
  );
  const stop = container.querySelector<HTMLButtonElement>("button[aria-label='Stop']");
  assert.ok(stop, "stop button rendered while streaming controls are requested");
  for (const button of [stop]) {
    assert.ok(button.className.includes("h-9 w-9"), "visual size unchanged");
    assert.ok(button.className.includes("after:-inset-2"), "mobile hit area extended by 8px per side (36→52px)");
    assert.ok(button.className.includes("md:after:inset-0"), "desktop hit area unchanged");
  }
});

test("rows prop reaches the textarea (Agent empty state uses 3)", () => {
  const { container } = renderWithIntl(
    <Composer value="" onChange={() => undefined} onSubmit={() => undefined} submitLabel="Send" ariaLabel="msg" rows={3} keyMode="enter" />,
  );
  const textarea = container.querySelector("textarea");
  assert.equal(textarea?.getAttribute("rows"), "3");
});

test.afterEach(() => cleanup());
