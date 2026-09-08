import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render, within } from "./runtime-test-helpers";

type FilterPillsModule = typeof import("@/components/ui/filter-pills");
let FilterPills: FilterPillsModule["FilterPills"];
/* #414 O1a：判别联合后单选用具限定 single 分支，多选场景单独内联构造。 */
type FilterPillsSingleProps = Extract<import("@/components/ui/filter-pills").FilterPillsProps, { selectionMode?: "single" }>;

test.before(async () => {
  const mod = await import("@/components/ui/filter-pills");
  FilterPills = mod.FilterPills;
});

test.afterEach(() => {
  cleanup();
});

const OPTIONS = [
  { value: "", label: "All" },
  { value: "image", label: "Image", count: 3 },
  { value: "video", label: "Video" },
];

function renderPills(props: Partial<FilterPillsSingleProps> = {}) {
  const onChange = props.onChange ?? (() => {});
  installDom();
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <FilterPills
        ariaLabel="Content categories"
        options={OPTIONS}
        value="image"
        onChange={onChange}
        {...props}
      />
    </IntlProvider>,
  );
  return { view, onChange };
}

test("renders a labelled navigation of pill buttons with aria-pressed", async () => {
  const { view } = renderPills();
  const nav = view.getByRole("navigation", { name: "Content categories" });
  const buttons = within(nav).getAllByRole("button");
  assert.equal(buttons.length, 3);
  assert.equal(buttons[0].getAttribute("aria-pressed"), "false");
  assert.equal(buttons[1].getAttribute("aria-pressed"), "true");
  assert.equal(buttons[2].getAttribute("aria-pressed"), "false");
});

test("clicking a pill reports the value and keeps single selection semantics", async () => {
  const calls: string[] = [];
  const { view } = renderPills({ onChange: (v: string) => calls.push(v) });
  const nav = view.getByRole("navigation", { name: "Content categories" });
  fireEvent.click(within(nav).getByRole("button", { name: "Video" }));
  assert.deepEqual(calls, ["video"]);
});

test("renders option counts and the disabled group", async () => {
  const { view } = renderPills({ disabled: true });
  const nav = view.getByRole("navigation", { name: "Content categories" });
  assert.ok(within(nav).getByText("3"));
  const buttons = within(nav).getAllByRole("button");
  for (const button of buttons) {
    assert.equal((button as HTMLButtonElement).disabled, true);
  }
});

/* ── #414 O1a 矮药丸新基准（取代 SP-12 44px+勾号） ─────────────────── */

test("compact tier: no 44px height, no check icon, keeps accent trio", async () => {
  const { view } = renderPills();
  const nav = view.getByRole("navigation", { name: "Content categories" });
  const buttons = within(nav).getAllByRole("button");
  const active = buttons[1];
  assert.match(active.className, /py-1\.5/, "compact padding tier");
  assert.doesNotMatch(active.className, /min-h-11/, "44px touch tier retired by O1a");
  assert.match(active.className, /border-accent-emphasis bg-accent-subtle text-accent-emphasis font-semibold/);
  // 勾号图标移除：按钮内不渲染任何 svg（可及性由底色/描边/字重/aria-pressed 承担）
  assert.equal(active.querySelector("svg"), null, "no check icon glyph");
});

test("multiple mode: click adds, click again removes, aria-pressed tracks membership", async () => {
  installDom();
  const value = ["image"];
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <FilterPills
        ariaLabel="Tags"
        selectionMode="multiple"
        options={OPTIONS}
        value={value}
        onChange={(next) => { value.splice(0, value.length, ...next); }}
      />
    </IntlProvider>,
  );
  const nav = view.getByRole("navigation", { name: "Tags" });
  assert.equal(within(nav).getAllByRole("button")[1].getAttribute("aria-pressed"), "true");
  fireEvent.click(within(nav).getByRole("button", { name: /^Video/ }));
  assert.deepEqual(value, ["image", "video"], "click adds");
  fireEvent.click(within(nav).getByRole("button", { name: /^Image/ }));
  assert.deepEqual(value, ["video"], "click again removes");
  cleanup();
});

test("single mode clearable: clicking the selected pill clears the value", async () => {
  installDom();
  const calls: string[] = [];
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <FilterPills
        ariaLabel="Category"
        clearable
        options={OPTIONS}
        value="image"
        onChange={(v) => calls.push(v)}
      />
    </IntlProvider>,
  );
  const nav = view.getByRole("navigation", { name: "Category" });
  fireEvent.click(within(nav).getByRole("button", { name: /^Image/ }));
  assert.deepEqual(calls, [""], "click selected clears in clearable mode");
  fireEvent.click(within(nav).getByRole("button", { name: /^Video/ }));
  assert.deepEqual(calls, ["", "video"], "click other selects it");
  cleanup();
});
