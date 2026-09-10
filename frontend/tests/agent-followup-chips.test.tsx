import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * SP-15 B #435 推荐追问药丸（ui-spec `## Component: AgentFollowUpChips`）：
 * 动作药丸 = 点击只回调 onFill（填入 composer），不发送；无选中态。
 * ──────────────────────────────────────────────────────────────────────────── */

type Module = typeof import("@/components/agent/AgentFollowUpChips");
let AgentFollowUpChips: Module["AgentFollowUpChips"];

test.before(async () => {
  const module = await import("@/components/agent/AgentFollowUpChips");
  AgentFollowUpChips = module.AgentFollowUpChips;
});

test.afterEach(() => cleanup());

const FOLLOW_UPS = ["What else by this author?", "Show watercolor tutorials"];

function renderChips(followUps: string[], onFill: (q: string) => void) {
  installDom();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <AgentFollowUpChips followUps={followUps} onFill={onFill} />
    </IntlProvider>,
  );
}

test("#435: chips render as a labelled group of action pills", () => {
  const view = renderChips(FOLLOW_UPS, () => {});
  /* 容器 group 名来自 next-intl（真实 en catalog，禁止发明 key）。 */
  assert.equal(view.getByRole("group").getAttribute("aria-label"), "Suggested follow-ups");
  /* 每颗药丸的 aria-label = 追问文本 + 填入动作说明；无 aria-pressed（非筛选控件）。 */
  const chip = view.getByRole("button", { name: "What else by this author? — Fill the composer" });
  assert.equal(chip.getAttribute("aria-pressed"), null);
  assert.ok(view.getByRole("button", { name: "Show watercolor tutorials — Fill the composer" }));
});

test("#435: clicking a chip only reports onFill with that query (no send prop exists)", () => {
  const filled: string[] = [];
  const view = renderChips(FOLLOW_UPS, (q) => filled.push(q));
  fireEvent.click(view.getByRole("button", { name: /Show watercolor tutorials/ }));
  assert.deepEqual(filled, ["Show watercolor tutorials"], "exactly one onFill with the clicked text");
});

test("#435: empty follow-ups render nothing", () => {
  const view = renderChips([], () => {});
  assert.equal(view.container.firstChild, null);
});
