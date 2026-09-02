import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render, within } from "./runtime-test-helpers";

type FilterPillsModule = typeof import("@/components/ui/filter-pills");
let FilterPills: FilterPillsModule["FilterPills"];

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

function renderPills(props: Partial<Parameters<typeof FilterPills>[0]> = {}) {
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
  const { view } = renderPills({ onChange: (v) => calls.push(v) });
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
