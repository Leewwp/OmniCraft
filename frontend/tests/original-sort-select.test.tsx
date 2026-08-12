import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render, within } from "./runtime-test-helpers";

// Base UI components read computed styles from async timers; jsdom exposes the
// API on window but not on globalThis.
if (typeof globalThis.getComputedStyle === "undefined") {
  Object.defineProperty(globalThis, "getComputedStyle", {
    configurable: true,
    value: (element: Element) => window.getComputedStyle(element),
  });
}

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const routerPushes: string[] = [];
let mockSearchString = "";

Module._load = function loadWithNavigationStub(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useRouter: () => ({ push: (value: string) => routerPushes.push(value) }),
      useSearchParams: () => new URLSearchParams(mockSearchString),
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type SortSelectModule = typeof import("@/components/original/SortSelect");
let SortSelect: SortSelectModule["SortSelect"];

test.before(async () => {
  const mod = await import("@/components/original/SortSelect");
  SortSelect = mod.SortSelect;
});

test.afterEach(() => {
  cleanup();
  routerPushes.length = 0;
  mockSearchString = "";
});

function renderSortSelect() {
  installDom();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <SortSelect />
    </IntlProvider>,
  );
}

/** Opens the listbox and presses Enter on the option at the given index. */
function selectOption(view: ReturnType<typeof renderSortSelect>, index: number) {
  const trigger = view.getByRole("combobox", { name: "Sort by" });
  fireEvent.click(trigger);
  const listbox = view.getByRole("listbox");
  const options = within(listbox).getAllByRole("option");
  if (index > 0) {
    fireEvent.keyDown(listbox, { key: "End" });
    for (let i = options.length - 1; i > index; i -= 1) {
      fireEvent.keyDown(options[i], { key: "ArrowUp" });
    }
  } else {
    fireEvent.keyDown(listbox, { key: "Home" });
  }
  const target = within(listbox).getAllByRole("option")[index];
  fireEvent.keyDown(target, { key: "Enter" });
}

test("#72: original sort control keeps existing URL semantics - default is recommended", () => {
  const view = renderSortSelect();
  const trigger = view.getByRole("combobox", { name: "Sort by" });
  assert.match(trigger.textContent ?? "", /Recommended/);
});

test("#72: selecting a non-default sort pushes ?sort=<value>", () => {
  const view = renderSortSelect();
  selectOption(view, 1);
  assert.deepEqual(routerPushes, ["/original?sort=hot"]);
});

test("#72: selecting recommended removes the sort param", () => {
  mockSearchString = "category=film_tv&sort=hot";
  const view = renderSortSelect();
  const trigger = view.getByRole("combobox", { name: "Sort by" });
  assert.match(trigger.textContent ?? "", /Hottest/);
  selectOption(view, 0);
  assert.deepEqual(routerPushes, ["/original?category=film_tv"]);
});

test("#72: changing sort preserves unrelated params like category", () => {
  mockSearchString = "category=film_tv&sort=hot";
  const view = renderSortSelect();
  selectOption(view, 3);
  assert.deepEqual(routerPushes, ["/original?category=film_tv&sort=most_views"]);
});

test("#72: deep-linked sort value is reflected in the trigger label", () => {
  mockSearchString = "sort=most_views";
  const view = renderSortSelect();
  const trigger = view.getByRole("combobox", { name: "Sort by" });
  assert.match(trigger.textContent ?? "", /Most Viewed/);
});
