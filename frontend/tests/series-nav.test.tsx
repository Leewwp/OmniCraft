import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import { SeriesNav } from "@/components/content/SeriesNav";
import type { SeriesMembership } from "@/lib/content";
import { act, cleanup, fireEvent, installDom, renderWithIntl } from "./runtime-test-helpers";

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

function renderSeriesNav(items: SeriesMembership[]) {
  return renderWithIntl(
    <IntlProvider locale="zh" messages={messages}>
      <SeriesNav memberships={items} />
    </IntlProvider>,
  );
}
