import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import { SeriesNav } from "@/components/content/SeriesNav";
import type { SeriesMembership } from "@/lib/content";
import { act, cleanup, fireEvent, installDom, renderWithIntl, waitFor } from "./runtime-test-helpers";

const messages = {
  series: {
    nav: {
      tabsLabel: "所属内容系列",
      position: "第 {current} / 共 {total}",
      previous: "上一章：{title}",
      next: "下一章：{title}",
      catalog: "系列目录",
      more: "更多({count})",
      first: "已是第一章",
      last: "已是最后一章",
      previousA11y: "上一章：{title}",
      nextA11y: "下一章：{title}",
      firstA11y: "上一章不可用，已是第一章",
      lastA11y: "下一章不可用，已是最后一章",
      catalogA11y: "查看 {title} 系列目录",
      directory: {
        title: "章节目录",
        position: "第 {current} / 共 {total} 章",
        option: "第 {index} 章：{title}",
        currentOption: "当前章节：第 {index} 章：{title}",
        loading: "目录加载中…",
        empty: "系列暂无公开内容",
        loadFailed: "目录加载失败",
        retry: "重试",
        listLabel: "{title} 章节目录",
      },
    },
  },
};

const memberships: SeriesMembership[] = [
  {
    series_id: 1,
    series_title: "山海纪行",
    series_zone: "original",
    current_index: 2,
    total: 3,
    previous: { id: 10, title: "启程" },
    next: { id: 12, title: "归途" },
  },
  { series_id: 2, series_title: "人物小传", current_index: 1, total: 1 },
  { series_id: 3, series_title: "幕后手记", current_index: 1, total: 2, next: { id: 30, title: "下篇" } },
  { series_id: 4, series_title: "世界设定", current_index: 1, total: 1 },
  { series_id: 5, series_title: "创作札记", current_index: 1, total: 1 },
];

test.beforeEach(() => installDom());
test.afterEach(() => cleanup());

test("SeriesNav renders a single series position and valid navigation links", () => {
  const view = renderSeriesNav([memberships[0]]);

  assert.ok(view.getByText("山海纪行"));
  assert.ok(view.getByText("第 2 / 共 3"));
  assert.equal(view.getByRole("link", { name: "上一章：启程" }).getAttribute("href"), "/original/10");
  assert.equal(view.getByRole("link", { name: "下一章：归途" }).getAttribute("href"), "/original/12");
  assert.equal(view.getByRole("link", { name: "查看 山海纪行 系列目录" }).getAttribute("href"), "/series/1");
});

test("SeriesNav enforces readable first and last boundaries even when targets are inconsistent", () => {
  const first = renderSeriesNav([{ ...memberships[0], current_index: 1 }]);
  const firstDisabled = first.getByRole("button", { name: "上一章不可用，已是第一章" });
  assert.equal(firstDisabled.getAttribute("aria-disabled"), "true");
  assert.match(firstDisabled.className, /text-fg-muted/);
  assert.match(firstDisabled.className, /disabled:opacity-100/);
  assert.ok(first.getByText("已是第一章"));
  cleanup();

  const last = renderSeriesNav([{ ...memberships[0], current_index: 3 }]);
  const lastDisabled = last.getByRole("button", { name: "下一章不可用，已是最后一章" });
  assert.equal(lastDisabled.getAttribute("aria-disabled"), "true");
  assert.ok(last.getByText("已是最后一章"));
});

test("SeriesNav renders three tabs and switches with click and arrow keys", () => {
  const view = renderSeriesNav(memberships.slice(0, 3));
  const tabs = view.getAllByRole("tab");
  assert.equal(tabs.length, 3);
  assert.equal(tabs[0].getAttribute("aria-selected"), "true");

  fireEvent.click(tabs[1]);
  assert.equal(tabs[1].getAttribute("aria-selected"), "true");
  assert.ok(view.getByText("第 1 / 共 1"));

  fireEvent.keyDown(tabs[1], { key: "ArrowRight" });
  assert.equal(tabs[2].getAttribute("aria-selected"), "true");
  fireEvent.keyDown(tabs[2], { key: "ArrowLeft" });
  assert.equal(tabs[1].getAttribute("aria-selected"), "true");
});

test("SeriesNav preserves the selected series identity when memberships reorder", () => {
  function Harness() {
    const [items, setItems] = React.useState(memberships.slice(0, 3));
    return (
      <>
        <button type="button" onClick={() => setItems([memberships[1], memberships[2]])}>refresh memberships</button>
        <SeriesNav memberships={items} />
      </>
    );
  }
  const view = renderWithIntl(
    <IntlProvider locale="zh" messages={messages}>
      <Harness />
    </IntlProvider>,
  );
  fireEvent.click(view.getByRole("tab", { name: "人物小传" }));

  fireEvent.click(view.getByRole("button", { name: "refresh memberships" }));

  assert.equal(view.getByRole("tab", { name: "人物小传" }).getAttribute("aria-selected"), "true");
  assert.equal(view.getByRole("link", { name: "查看 人物小传 系列目录" }).getAttribute("href"), "/series/2");
});

test("SeriesNav exposes every overflow series through a keyboard menu", async () => {
  const view = renderSeriesNav(memberships);
  assert.equal(view.getAllByRole("tab").length, 3);

  const trigger = view.getByRole("button", { name: "更多(2)" });
  assert.equal(trigger.closest(".overflow-x-auto"), null, "overflow menu trigger must live outside the clipped tab scroller");
  trigger.focus();
  await act(async () => {
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  const first = view.getByRole("menuitem", { name: "世界设定" });
  assert.equal(first.getAttribute("href"), "/series/4");
  assert.equal(document.activeElement?.getAttribute("href"), "/series/4");
  const second = view.getByRole("menuitem", { name: "创作札记" });
  assert.equal(second.getAttribute("href"), "/series/5");
  act(() => fireEvent.keyDown(first, { key: "ArrowDown" }));
  assert.equal(document.activeElement?.getAttribute("href"), "/series/5");
  act(() => fireEvent.keyDown(second, { key: "ArrowUp" }));
  assert.equal(document.activeElement?.getAttribute("href"), "/series/4");
  act(() => fireEvent.keyDown(first, { key: "Escape" }));
  assert.equal(view.queryByRole("menu"), null);
  assert.equal(document.activeElement?.getAttribute("aria-label"), "更多(2)");

  act(() => fireEvent.click(trigger));
  assert.ok(view.getByRole("menu"));
  act(() => fireEvent.pointerDown(document.body));
  assert.equal(view.queryByRole("menu"), null);
});

test("ContentDetail places SeriesNav after ReactionBar and before comments", () => {
  const source = fs.readFileSync(new URL("../components/content/ContentDetail.tsx", import.meta.url), "utf8");
  const reaction = source.indexOf("<ReactionBar");
  const seriesNav = source.indexOf("<SeriesNav");
  const comments = source.indexOf("<CommentSection");
  assert.ok(reaction >= 0 && seriesNav > reaction && comments > seriesNav, "expected ReactionBar -> SeriesNav -> CommentSection");
});

/* ------------------------------------------------------------------ */
/* #69 浮层内系列目录：有界高度 + 可滚动 + listbox 语义，章节选择压栈。 */

const directorySeriesDetail = {
  series: {
    id: 1,
    title: "山海纪行",
    description: "三章故事",
    zone: "original",
    owner: { id: 1, username: "writer" },
    cover: null,
    item_count: 3,
  },
  items: [
    { id: 11, sort_order: 0, content_item_id: 10, content: { id: 10, title: "启程", zone: "original", status: "published" } },
    { id: 12, sort_order: 1, content_item_id: 11, content: { id: 11, title: "山雨", zone: "original", status: "published" } },
    { id: 13, sort_order: 2, content_item_id: 12, content: { id: 12, title: "归途", zone: "original", status: "published" } },
  ],
};

function stubSeriesDetailFetch(payload: unknown = directorySeriesDetail, status = 200) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url.includes("/api/v1/series/1")) {
      return new Response(JSON.stringify(payload), {
        status,
        headers: { "Content-Type": "application/json" },
      });
    }
    return new Response(JSON.stringify({ code: "NOT_FOUND", message: url }), { status: 404 });
  }) as typeof fetch;
  return () => {
    globalThis.fetch = originalFetch;
  };
}

test("SeriesNav overlay mode turns prev, catalog and next into in-overlay buttons", () => {
  const view = renderSeriesNav([memberships[0]], { onNavigateInOverlay: () => {} });

  assert.equal(view.queryByRole("link", { name: "上一章：启程" }), null);
  assert.equal(view.queryByRole("link", { name: "下一章：归途" }), null);
  assert.equal(view.queryByRole("link", { name: "查看 山海纪行 系列目录" }), null);

  const prev = view.getByRole("button", { name: "上一章：启程" });
  const catalog = view.getByRole("button", { name: "查看 山海纪行 系列目录" });
  const nextButton = view.getByRole("button", { name: "下一章：归途" });
  assert.equal(catalog.getAttribute("aria-haspopup"), "listbox");
  assert.equal(prev.getAttribute("href"), null);
  assert.equal(nextButton.getAttribute("href"), null);
});

test("SeriesNav overlay prev/next push the target with the clicked trigger", () => {
  const calls: Array<{ id: number; trigger: HTMLElement | null }> = [];
  const view = renderSeriesNav([memberships[0]], {
    onNavigateInOverlay: (id, trigger) => calls.push({ id, trigger: trigger ?? null }),
  });

  fireEvent.click(view.getByRole("button", { name: "下一章：归途" }));
  assert.equal(calls.length, 1);
  assert.equal(calls[0]?.id, 12);
  assert.equal(calls[0]?.trigger?.getAttribute("aria-label"), "下一章：归途");

  fireEvent.click(view.getByRole("button", { name: "上一章：启程" }));
  assert.equal(calls[1]?.id, 10);
  assert.equal(calls[1]?.trigger?.getAttribute("aria-label"), "上一章：启程");
});

test("SeriesNav overlay directory opens a bounded scrollable listbox with the current chapter marked", async () => {
  const restoreFetch = stubSeriesDetailFetch();
  try {
    const view = renderSeriesNav([memberships[0]], { onNavigateInOverlay: () => {} });

    fireEvent.click(view.getByRole("button", { name: "查看 山海纪行 系列目录" }));
    /* 布尔恒等断言：失败时不序列化 DOM 节点数组（c52cea4 教训）。 */
    await waitFor(() => assert.ok(view.getAllByRole("option").length === 3));

    const listbox = view.getByRole("listbox");
    assert.match(listbox.className, /max-h-72/, "directory must be height-bounded");
    assert.match(listbox.className, /overflow-y-auto/, "directory must scroll internally");

    const options = view.getAllByRole("option");
    assert.ok(options.length === 3);
    assert.equal(options[0]?.getAttribute("aria-selected"), "false");
    assert.equal(options[1]?.getAttribute("aria-selected"), "true", "current chapter must be aria-selected");
    assert.ok(options[1]?.textContent?.includes("山雨"));
    /* 聚焦当前章节项在被动 effect 中执行（提交后异步冲刷，CI 负载下可能晚于
       DOM 出现）——#313/#315/#342 的闪断点：必须等焦点真正落位再断言，不能在
       选项出现后立即断言（与 keyboard flow 测试的 round3 加固同款）。 */
    await waitFor(() => assert.ok(document.activeElement === options[1]));
    assert.ok(document.activeElement === options[1], "focus must enter the selector at the current chapter");
  } finally {
    restoreFetch();
  }
});

test("SeriesNav overlay directory keyboard flow selects chapters and Escape restores the trigger", async () => {
  const restoreFetch = stubSeriesDetailFetch();
  const pushed: number[] = [];
  try {
    const view = renderSeriesNav([memberships[0]], { onNavigateInOverlay: (id) => pushed.push(id) });
    const trigger = view.getByRole("button", { name: "查看 山海纪行 系列目录" });

    fireEvent.click(trigger);
    await waitFor(() => assert.ok(view.getAllByRole("option").length === 3));
    const options = view.getAllByRole("option");
    /* 聚焦当前章节项在被动 effect 中执行（提交后异步冲刷，CI 负载下可能晚于
       DOM 出现）：先等焦点真正落位再发键，避免按键发在 body 上。 */
    await waitFor(() => assert.ok(document.activeElement === options[1]));

    act(() => fireEvent.keyDown(document.activeElement as Element, { key: "ArrowDown" }));
    assert.ok(document.activeElement === options[2], "ArrowDown must move to the next option");
    act(() => fireEvent.keyDown(document.activeElement as Element, { key: "ArrowUp" }));
    assert.ok(document.activeElement === options[1], "ArrowUp must move back");
    act(() => fireEvent.keyDown(document.activeElement as Element, { key: "ArrowUp" }));
    assert.ok(document.activeElement === options[0], "ArrowUp must wrap around");

    act(() => fireEvent.keyDown(document.activeElement as Element, { key: "Enter" }));
    assert.deepEqual(pushed, [10], "Enter on an option must push that chapter");

    fireEvent.click(trigger);
    await waitFor(() => assert.ok(view.getAllByRole("option").length === 3));
    /* 重开后焦点效应同样异步落位：Escape 必须发在选项上，否则关不掉目录。 */
    await waitFor(() =>
      assert.ok(view.getAllByRole("option").some((option) => option === document.activeElement)),
    );
    await act(async () => {
      fireEvent.keyDown(document.activeElement as Element, { key: "Escape" });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    assert.equal(view.queryByRole("listbox"), null, "Escape must close the directory");
    /* 布尔恒等断言：失败时不序列化整棵 Fiber 树（c52cea4 的既定教训）。 */
    assert.ok(document.activeElement === trigger, "Escape must return focus to the catalog trigger");
  } finally {
    restoreFetch();
  }
});

test("SeriesNav overlay directory never re-pushes the current chapter", async () => {
  const restoreFetch = stubSeriesDetailFetch();
  const pushed: number[] = [];
  try {
    const view = renderSeriesNav([memberships[0]], { onNavigateInOverlay: (id) => pushed.push(id) });

    fireEvent.click(view.getByRole("button", { name: "查看 山海纪行 系列目录" }));
    await waitFor(() => assert.ok(view.getAllByRole("option").length === 3));
    const options = view.getAllByRole("option");
    fireEvent.click(options[1] as Element);

    assert.deepEqual(pushed, [], "selecting the current chapter must not push a duplicate layer");
    assert.equal(view.queryByRole("listbox"), null);
  } finally {
    restoreFetch();
  }
});

test("SeriesNav overlay keeps first/last disabled states readable", () => {
  const first = renderSeriesNav([{ ...memberships[0], current_index: 1 }], { onNavigateInOverlay: () => {} });
  const prevDisabled = first.getByRole("button", { name: "上一章不可用，已是第一章" });
  assert.equal(prevDisabled.getAttribute("aria-disabled"), "true");
  assert.ok(first.getByText("已是第一章"));
  cleanup();

  const last = renderSeriesNav([{ ...memberships[0], current_index: 3 }], { onNavigateInOverlay: () => {} });
  const nextDisabled = last.getByRole("button", { name: "下一章不可用，已是最后一章" });
  assert.equal(nextDisabled.getAttribute("aria-disabled"), "true");
  assert.ok(last.getByText("已是最后一章"));
});

test("SeriesNav overlay directory shows a retryable error state when the chapter fetch fails", async () => {
  const restoreFetch = stubSeriesDetailFetch({ code: "NOT_FOUND", message: "no such series" }, 404);
  try {
    const view = renderSeriesNav([memberships[0]], { onNavigateInOverlay: () => {} });

    fireEvent.click(view.getByRole("button", { name: "查看 山海纪行 系列目录" }));
    await waitFor(() => assert.ok(view.getByText("目录加载失败")));
    assert.ok(view.getByRole("button", { name: "重试" }));
  } finally {
    restoreFetch();
  }
});

function renderSeriesNav(items: SeriesMembership[], props?: { onNavigateInOverlay?: (contentId: number, trigger?: HTMLElement | null) => void }) {
  return renderWithIntl(
    <IntlProvider locale="zh" messages={messages}>
      <SeriesNav memberships={items} onNavigateInOverlay={props?.onNavigateInOverlay} />
    </IntlProvider>,
  );
}
