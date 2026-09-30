import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import {
  createAgentTurn,
  hasVisibleAssistantContent,
  shouldShowThinkingPlaceholder,
} from "@/lib/agent-turn";
import { AgentThinkingPlaceholder } from "@/components/agent/AgentThinkingPlaceholder";
import { cleanup, render, waitFor } from "./runtime-test-helpers";

/* #727 「正在思考」占位：空窗分支条件（含空 think segment）、计时器
 * 递增与卸载清理、读屏静默、与首个可见内容互斥。 */

function liveTurn() {
  return createAgentTurn("q", { id: "live-t", firstRound: false });
}

test("hasVisibleAssistantContent: empty think segments are NOT visible", () => {
  const turn = liveTurn();
  turn.segments.push({ kind: "think", content: "" });
  turn.segments.push({ kind: "think", content: "   \n  " });
  turn.segments.push({ kind: "tools", tools: [] });
  assert.equal(hasVisibleAssistantContent(turn), false, "whitespace think + empty tools = invisible");
  assert.equal(shouldShowThinkingPlaceholder(turn, true), true, "placeholder stays during the blank window");
});

test("hasVisibleAssistantContent: first visible content of each kind flips visibility", () => {
  const think = liveTurn();
  think.segments.push({ kind: "think", content: "推理…" });
  assert.equal(hasVisibleAssistantContent(think), true);

  const tools = liveTurn();
  tools.segments.push({ kind: "tools", tools: [{ id: "t1", name: "search", status: "running" } as never] });
  assert.equal(hasVisibleAssistantContent(tools), true);

  const answer = liveTurn();
  answer.answer = "答";
  assert.equal(hasVisibleAssistantContent(answer), true);
});

test("placeholder condition: mutual exclusion with visible content and terminal states", () => {
  const blank = liveTurn();
  assert.equal(shouldShowThinkingPlaceholder(blank, true), true);

  const visible = liveTurn();
  visible.segments.push({ kind: "think", content: "x" });
  assert.equal(shouldShowThinkingPlaceholder(visible, true), false, "no placeholder once content is visible");

  const done = liveTurn();
  done.streaming = false; // 空 done 终态：streaming=false 移除占位
  done.settled = true;
  assert.equal(shouldShowThinkingPlaceholder(done, true), false, "empty done removes the placeholder");

  const errored = liveTurn();
  errored.streaming = false;
  errored.terminal.error = true;
  assert.equal(shouldShowThinkingPlaceholder(errored, true), false);

  const replay = liveTurn();
  replay.streaming = false;
  assert.equal(shouldShowThinkingPlaceholder(replay, false), false, "history replay never shows the placeholder");

  const nonLiveBlank = liveTurn();
  assert.equal(shouldShowThinkingPlaceholder(nonLiveBlank, false), false, "isLive=false never shows");
});

function renderWithIntl(ui: React.ReactElement) {
  return render(<IntlProvider locale="en" messages={enMessages as never}>{ui}</IntlProvider>);
}

test("AgentThinkingPlaceholder: elapsed seconds tick up and are screen-reader silent", async () => {
  const startedAt = Date.now() - 5000;
  const { getByTestId, unmount } = renderWithIntl(<AgentThinkingPlaceholder startedAt={startedAt} />);
  const seconds = getByTestId("agent-thinking-seconds");
  assert.equal(seconds.textContent, "5s");
  assert.equal(seconds.getAttribute("aria-hidden"), "true", "the ticking number must stay silent for AT");

  const root = getByTestId("agent-thinking-placeholder");
  assert.equal(root.getAttribute("role"), null, "placeholder itself carries no role=status (transcript already live)");

  await waitFor(
    () => assert.ok(["6s", "7s"].includes(seconds.textContent ?? ""), `tick advanced: ${seconds.textContent}`),
    { timeout: 2500 },
  );
  unmount();
});

test("AgentThinkingPlaceholder: interval cleaned up on unmount", async () => {
  const origSetInterval = globalThis.setInterval;
  let pending = 0;
  (globalThis as Record<string, unknown>).setInterval = ((fn: () => void, ms: number, ...rest: unknown[]) => {
    pending += 1;
    return origSetInterval(fn as never, ms, ...(rest as never[]));
  }) as typeof setInterval;
  const origClear = globalThis.clearInterval;
  let cleared = 0;
  (globalThis as Record<string, unknown>).clearInterval = ((id: unknown) => {
    cleared += 1;
    return origClear(id as never);
  }) as typeof clearInterval;
  try {
    const { unmount } = renderWithIntl(<AgentThinkingPlaceholder startedAt={Date.now()} />);
    unmount();
    await new Promise((resolve) => setTimeout(resolve, 30));
    assert.equal(pending, cleared, "every created interval is cleared on unmount");
  } finally {
    (globalThis as Record<string, unknown>).setInterval = origSetInterval;
    (globalThis as Record<string, unknown>).clearInterval = origClear;
  }
});

test.afterEach(() => cleanup());
