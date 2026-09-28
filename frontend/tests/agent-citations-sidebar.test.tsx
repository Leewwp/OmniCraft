import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, installDom, render } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * FT-4（#696）参考来源侧栏：原内联折叠列表（AgentCitationList）退役；
 * 侧栏显示所点回答的引用卡片，双通道关闭（X），卡片点击回调透传。
 * ──────────────────────────────────────────────────────────────────────────── */

type SidebarModule = typeof import("@/components/agent/AgentCitationsSidebar");
let AgentCitationsSidebar: SidebarModule["AgentCitationsSidebar"];

test.before(async () => {
  const module = await import("@/components/agent/AgentCitationsSidebar");
  AgentCitationsSidebar = module.AgentCitationsSidebar;
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

function renderSidebar(overrides: Partial<React.ComponentProps<typeof AgentCitationsSidebar>> = {}) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <AgentCitationsSidebar
        open
        onClose={() => undefined}
        citations={[citation(0), citation(1), citation(2), citation(3), citation(4), citation(5)]}
        onOpen={() => undefined}
        {...overrides}
      />
    </IntlProvider>,
  );
}

test("FT-4 sidebar renders every citation card under the unified reference-sources title", () => {
  installDom();
  const view = renderSidebar();
  assert.ok(view.getByText("Reference sources"), "unified naming (was the old inline list title)");
  assert.ok(view.getByText("6"), "count is projected");
  for (let i = 0; i < 6; i += 1) {
    assert.ok(view.getByRole("button", { name: new RegExp(`Reference ${i + 1}`) }), `card ${i + 1} renders`);
  }
});

test("FT-4 sidebar hides when closed or empty (no dead panel)", () => {
  installDom();
  const closed = renderSidebar({ open: false });
  assert.equal(closed.container.firstChild, null);
  const empty = renderSidebar({ citations: [] });
  assert.equal(empty.container.firstChild, null);
});

test("FT-4 close affordance exists (X channel)", () => {
  installDom();
  const view = renderSidebar();
  assert.ok(view.getAllByRole("button", { name: "Close reference sources" }).length >= 1);
});
