import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * #399 R1 引用列表折叠：>FOLD_THRESHOLD（4）默认收起（citation_max_count=12
 * 配套），≤阈值默认展开；开关为纯展示层（引用持久化/回填合并不受影响）。
 * ──────────────────────────────────────────────────────────────────────────── */

type CitationListModule = typeof import("@/components/agent/AgentCitationList");
let AgentCitationList: CitationListModule["AgentCitationList"];

test.before(async () => {
  const module = await import("@/components/agent/AgentCitationList");
  AgentCitationList = module.AgentCitationList;
});

test.afterEach(() => cleanup());

function citation(index: number) {
  return {
    contentId: 100 + index,
    title: `Reference ${index + 1}`,
    zone: "original" as const,
    excerpt: `excerpt ${index + 1}`,
  };
}

function renderList(count: number) {
  installDom();
  const onOpen = () => {};
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <AgentCitationList citations={Array.from({ length: count }, (_, i) => citation(i))} onOpen={onOpen} />
    </IntlProvider>,
  );
}

test("R1: five citations collapse by default with a count toggle", () => {
  const view = renderList(5);
  /* 标题与计数可见；卡片默认不渲染；开关文案带条数与 aria-expanded。 */
  assert.ok(view.getByText("Site references"));
  assert.ok(view.getByText("5 verified references"));
  assert.equal(document.querySelectorAll('[id^="agent-citation-"]').length, 0, "cards hidden when collapsed");
  const toggle = view.getByRole("button", { name: /Show citations \(5\)/ });
  assert.equal(toggle.getAttribute("aria-expanded"), "false");
});

test("R1: toggling expands and collapses the card grid", () => {
  const view = renderList(5);
  const toggle = view.getByRole("button", { name: /Show citations/ });
  fireEvent.click(toggle);
  assert.equal(document.querySelectorAll('[id^="agent-citation-"]').length, 5, "all cards render after expand");
  assert.equal(toggle.getAttribute("aria-expanded"), "true");
  const collapse = view.getByRole("button", { name: /Collapse citations/ });
  fireEvent.click(collapse);
  assert.equal(document.querySelectorAll('[id^="agent-citation-"]').length, 0, "cards hidden again");
});

test("R1: at or below the threshold the list stays expanded by default", () => {
  const view = renderList(4);
  assert.equal(document.querySelectorAll('[id^="agent-citation-"]').length, 4);
  assert.ok(view.getByRole("button", { name: /Collapse citations/ }));
});
