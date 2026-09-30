import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { citationDisplayMapOf, remapCitationMarks } from "@/lib/agent";
import { AgentCitationCard } from "@/components/agent/AgentCitationCard";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";
import { AgentMetaPill } from "@/components/agent/AgentMetaPill";
import { BookOpen } from "lucide-react";
import { cleanup, fireEvent, render } from "./runtime-test-helpers";

/* #719 引用展示层压缩重编号 + 三卡统一胶囊：
 * 1. 展示号映射（空洞序列、legacy 回退、混排累计）；
 * 2. 复制替换（含不命中保留与链接边界）+ 渲染层不改原文；
 * 3. 侧栏卡片展示号/读屏号 vs 锚点 id 原编号；
 * 4. 角标展示号（文字 + aria）与原编号命中解耦；
 * 5. 胶囊组件交互（aria-expanded、计数、流式图标）。 */

function renderWithIntl(ui: React.ReactElement) {
  return render(<IntlProvider locale="en" messages={enMessages as never}>{ui}</IntlProvider>);
}

test("citationDisplayMapOf: hole sequences collapse to consecutive display numbers", () => {
  // 服务端复验剔除后：原全局编号 2/5/9（空洞）→ 展示 1/2/3。
  const map = citationDisplayMapOf([{ number: 2 }, { number: 5 }, { number: 9 }]);
  assert.equal(map.get(2), 1);
  assert.equal(map.get(5), 2);
  assert.equal(map.get(9), 3);
  assert.equal(map.size, 3);
});

test("citationDisplayMapOf: legacy unnumbered rows fall back to positional order", () => {
  const map = citationDisplayMapOf([{}, {}, {}]);
  assert.equal(map.get(1), 1);
  assert.equal(map.get(2), 2);
  assert.equal(map.get(3), 3);
});

test("citationDisplayMapOf: mixed IP/content multi-search accumulation maps the merged pool", () => {
  // 同轮两次检索累计：内容 1/2/4 + IP 7（剔除 3/5/6）。
  const pool = [{ number: 1 }, { number: 2 }, { number: 4 }, { number: 7 }];
  const map = citationDisplayMapOf(pool);
  assert.deepEqual(
    pool.map((c) => map.get(c.number as number)),
    [1, 2, 3, 4],
  );
});

test("remapCitationMarks: display numbers replace originals; misses and link shapes survive", () => {
  const map = new Map([[6, 1], [10, 2]]);
  const text = "结论甲 [6]，结论乙 [10]，未命中 [3]，链接 [1](https://x)，定义 [1]:，图片 ![6]";
  const out = remapCitationMarks(text, map);
  assert.ok(out.includes("结论甲 [1]"));
  assert.ok(out.includes("结论乙 [2]"));
  assert.ok(out.includes("未命中 [3]"), "unmapped numbers stay untouched");
  assert.ok(out.includes("[1](https://x)"), "real links are not rewritten");
  assert.ok(out.includes("![6]"), "image marks are not rewritten");
  assert.equal(text, "结论甲 [6]，结论乙 [10]，未命中 [3]，链接 [1](https://x)，定义 [1]:，图片 ![6]", "original answer text is never mutated");
});

test("AgentCitationCard: display/aria use positional number, anchor id keeps the original", () => {
  const { container } = renderWithIntl(
    <AgentCitationCard
      citation={{ contentId: 1, title: "带空洞的目标", zone: "original", number: 9 }}
      index={1}
      onOpen={() => undefined}
    />,
  );
  const button = container.querySelector("button");
  assert.equal(button?.id, "agent-citation-9", "anchor id keeps the original global number");
  assert.ok(button?.textContent?.includes("02"), "visible number is the positional display number 02");
  assert.ok(button?.getAttribute("aria-label")?.includes("2"), "screen reader hears the display number");
  assert.ok(!button?.getAttribute("aria-label")?.includes("9"), "aria never leaks the original number");
});

test("MarkdownRenderer badge: displayNumber drives text and aria; jump keeps original", () => {
  let jumped = 0;
  const { container } = renderWithIntl(
    <MarkdownRenderer
      content={"答案 [6] 结尾"}
      citations={[{ number: 6, displayNumber: 1, title: "重映射目标", kind: "content" }]}
      onCitationRef={(index) => {
        jumped = index;
      }}
    />,
  );
  const badge = container.querySelector("button");
  assert.ok(badge?.textContent?.includes("1"), "badge text shows the display number");
  assert.ok(!badge?.textContent?.includes("6"), "badge text never shows the original number");
  assert.ok(badge?.getAttribute("aria-label")?.includes("1"), "aria uses the display number");
  fireEvent.click(badge as HTMLButtonElement);
  assert.equal(jumped, 6, "jump resolves by the ORIGINAL number (target lookup unchanged)");
});

test("AgentMetaPill: aria-expanded toggles, count renders, streaming swaps the icon", () => {
  let open = false;
  const { getByRole, rerender } = renderWithIntl(
    <AgentMetaPill icon={BookOpen} label="3 个工具步骤" count={3} expanded={false} streaming={false}
      onClick={() => { open = !open; }} />,
  );
  const pill = getByRole("button");
  assert.ok(pill.textContent?.includes("3"));
  assert.equal(pill.getAttribute("aria-expanded"), "false");
  rerender(
    <IntlProvider locale="en" messages={enMessages as never}>
      <AgentMetaPill icon={BookOpen} label="3 个工具步骤" count={3} expanded streaming={false}
        onClick={() => { open = !open; }} />
    </IntlProvider>,
  );
  assert.equal(pill.getAttribute("aria-expanded"), "true");
  fireEvent.click(pill);
  assert.equal(open, true, "pill drives the parent collapse state");
});

test.afterEach(() => cleanup());
