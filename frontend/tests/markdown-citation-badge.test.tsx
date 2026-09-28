import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { MarkdownRenderer, type CitationBadgeInfo } from "@/components/content/MarkdownRenderer";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* FT-5 (#697)：行内 [n] 角标的标题小卡形态——citations 命中轮内全局编号时
   渲染标题小卡（截断 ~10 字），点击回调传编号本身；未命中回退纯文本 sup；
   未提供 citations 时保留旧数字角标（citationCount 上限、点击传 0 基序）。
   桌面悬停浮窗：标题 + 简介节选 + 查看提示；触屏不挂悬停。 */

function installHoverMatchMedia(matches: boolean) {
  const original = window.matchMedia;
  window.matchMedia = ((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addListener() {},
    removeListener() {},
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
  return () => {
    window.matchMedia = original;
  };
}

const CITED_TITLE = "星轨下的制琴师与十二座灯塔";

const badgeCitations: CitationBadgeInfo[] = [
  { number: 1, title: CITED_TITLE, excerpt: "一个关于守望与琴弦的故事。", kind: "content" },
  { number: 3, title: "Quiet Ledger", excerpt: "Small storms, quiet pages.", kind: "ip" },
];

function renderMarkdown(props: Partial<Parameters<typeof MarkdownRenderer>[0]> = {}) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <MarkdownRenderer
        content="引用一 [1] 引用二 [2] 引用三 [3] 越界 [9]"
        onCitationRef={() => {}}
        citations={badgeCitations}
        {...props}
      />
    </IntlProvider>,
  );
}

function chipLabels(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll("button")).map((button) => button.textContent ?? "");
}

test.afterEach(async () => {
  cleanup();
});

test("markdown citation badge", async (t) => {
  await t.test("matched number renders a truncated title chip; click passes the global number", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(false);
    let jumped: number[] = [];
    const view = render(
      <IntlProvider locale="en" messages={enMessages}>
        <MarkdownRenderer
          content="开头 [1] 中间 [3] 结尾"
          onCitationRef={(n) => {
            jumped.push(n);
          }}
          citations={badgeCitations}
        />
      </IntlProvider>,
    );
    const buttons = view.getAllByRole("button");
    assert.equal(buttons.length, 2);
    // 小卡文本 = 编号 + 截断标题（~10 字，超出省略）。
    assert.match(buttons[0].textContent ?? "", /^1/);
    assert.ok((buttons[0].textContent ?? "").includes("星轨下的制琴师与十二…"));
    assert.ok((buttons[1].textContent ?? "").includes("Quiet Ledg"));

    fireEvent.click(buttons[1]);
    await waitFor(() => assert.deepEqual(jumped, [3]));
    restoreMedia();
  });

  await t.test("unmatched numbers fall back to plain sup text", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(false);
    const view = renderMarkdown();
    // [2] 与 [9] 不在编号集合 → 纯文本 sup，非可点击。
    assert.equal(chipLabels(view.container).length, 2);
    const sups = view.container.querySelectorAll("sup");
    assert.equal(sups.length, 2, "both unmatched numbers render plain sup");
    assert.match(sups[0]?.textContent ?? "", /2/);
    assert.match(sups[1]?.textContent ?? "", /9/);
    restoreMedia();
  });

  await t.test("without citations the legacy number chip and count bound stay", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(false);
    let legacyIndex: number[] = [];
    const view = render(
      <IntlProvider locale="en" messages={enMessages}>
        <MarkdownRenderer
          content="一 [1] 二 [2] 三 [3]"
          citationCount={2}
          onCitationRef={(i) => {
            legacyIndex.push(i);
          }}
        />
      </IntlProvider>,
    );
    const buttons = view.getAllByRole("button");
    assert.equal(buttons.length, 2);
    assert.equal(buttons[0].textContent, "1");
    fireEvent.click(buttons[0]);
    await waitFor(() => assert.deepEqual(legacyIndex, [0]));
    restoreMedia();
  });

  await t.test("desktop hover opens the tooltip; Escape closes it", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(true);
    const view = renderMarkdown();
    const first = view.getAllByRole("button")[0];
    fireEvent.pointerEnter(first);
    await waitFor(() => {
      const tooltip = view.container.querySelector('[role="tooltip"]');
      assert.ok(tooltip, "tooltip opens after hover delay");
      // 完整标题（不截断）+ 简介 + 查看提示。
      assert.ok((tooltip?.textContent ?? "").includes(CITED_TITLE));
      assert.ok((tooltip?.textContent ?? "").includes("守望与琴弦"));
    });
    // jsdom 对 pointerleave 的合成派发不可靠（UserHoverCard 同款取舍）：
    // 关闭路径走 Escape 键。
    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => {
      assert.equal(view.container.querySelector('[role="tooltip"]'), null);
    });
    restoreMedia();
  });

  await t.test("touch devices never attach hover handlers", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(false);
    const view = renderMarkdown();
    const first = view.getAllByRole("button")[0];
    fireEvent.pointerEnter(first);
    fireEvent.focus(first);
    await new Promise((resolve) => setTimeout(resolve, 60));
    assert.equal(view.container.querySelector('[role="tooltip"]'), null, "no tooltip on hover-incapable devices");
    restoreMedia();
  });
});
