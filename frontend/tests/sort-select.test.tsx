import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { cleanup, fireEvent, installDom, render, within } from "./runtime-test-helpers";
import { SortSelect } from "@/components/ui/SortSelect";

// Base UI components read computed styles and media queries from async timers;
// jsdom exposes the APIs on window but not on globalThis.
if (typeof globalThis.getComputedStyle === "undefined") {
  Object.defineProperty(globalThis, "getComputedStyle", {
    configurable: true,
    value: (element: Element) => window.getComputedStyle(element),
  });
}

const OPTIONS = [
  { value: "recommended", label: "Recommended" },
  { value: "hot", label: "Hottest" },
  { value: "newest", label: "Newest" },
  { value: "most_views", label: "Most Viewed" },
];

function renderSort(value = "hot", onChange: (v: string) => void = () => {}) {
  installDom();
  return render(
    <SortSelect ariaLabel="Sort by" value={value} options={OPTIONS} onChange={onChange} />,
  );
}

function open(view: ReturnType<typeof renderSort>) {
  const trigger = view.getByRole("combobox", { name: "Sort by" });
  fireEvent.click(trigger);
  return { trigger, listbox: view.getByRole("listbox") };
}

test("#72: trigger exposes combobox/listbox semantics and the current option label", () => {
  const view = renderSort("hot");
  const trigger = view.getByRole("combobox", { name: "Sort by" });
  assert.equal(trigger.getAttribute("aria-haspopup"), "listbox");
  assert.equal(trigger.getAttribute("aria-expanded"), "false");
  assert.match(trigger.textContent ?? "", /Hottest/);
});

test("#72: clicking the trigger opens a listbox whose options expose aria-selected", () => {
  const view = renderSort("hot");
  const { listbox } = open(view);
  const options = within(listbox).getAllByRole("option");
  assert.equal(options.length, 4);
  const selected = options.find((o) => o.getAttribute("aria-selected") === "true");
  assert.equal(selected?.textContent, "Hottest");
  assert.match(options[0].textContent ?? "", /Recommended/);
});

test("#72: ArrowDown/ArrowUp move the highlighted option inside the listbox", () => {
  const view = renderSort("hot");
  const { listbox } = open(view);
  const options = within(listbox).getAllByRole("option");
  fireEvent.keyDown(listbox, { key: "ArrowDown" });
  assert.equal(options[2].getAttribute("tabindex"), "0", "down from Hottest reaches Newest");
  fireEvent.keyDown(options[2], { key: "ArrowDown" });
  assert.equal(options[3].getAttribute("tabindex"), "0", "down again reaches Most Viewed");
  fireEvent.keyDown(options[3], { key: "ArrowUp" });
  assert.equal(options[2].getAttribute("tabindex"), "0", "up returns to Newest");
});

test("#72: Home and End jump to first and last option", () => {
  const view = renderSort("hot");
  const { listbox } = open(view);
  const options = within(listbox).getAllByRole("option");
  fireEvent.keyDown(listbox, { key: "End" });
  assert.equal(options[3].getAttribute("tabindex"), "0");
  fireEvent.keyDown(options[3], { key: "Home" });
  assert.equal(options[0].getAttribute("tabindex"), "0");
});

test("#72: Enter selects the highlighted option, closes and reports the new value", () => {
  const changes: string[] = [];
  const view = renderSort("hot", (v) => changes.push(v));
  const { trigger, listbox } = open(view);
  const options = within(listbox).getAllByRole("option");
  fireEvent.keyDown(listbox, { key: "ArrowDown" });
  fireEvent.keyDown(options[2], { key: "Enter" });
  assert.deepEqual(changes, ["newest"]);
  assert.equal(trigger.getAttribute("aria-expanded"), "false");
});

test("#72: Space selects the highlighted option and closes", () => {
  const changes: string[] = [];
  const view = renderSort("hot", (v) => changes.push(v));
  const { trigger, listbox } = open(view);
  const options = within(listbox).getAllByRole("option");
  fireEvent.keyDown(listbox, { key: "End" });
  fireEvent.keyDown(options[3], { key: " " });
  assert.deepEqual(changes, ["most_views"]);
  assert.equal(trigger.getAttribute("aria-expanded"), "false");
});

test("#72: Escape cancels without onChange", () => {
  const changes: string[] = [];
  const view = renderSort("hot", (v) => changes.push(v));
  const { trigger, listbox } = open(view);
  const options = within(listbox).getAllByRole("option");
  fireEvent.keyDown(listbox, { key: "ArrowDown" });
  fireEvent.keyDown(options[2], { key: "Escape" });
  assert.deepEqual(changes, []);
  assert.equal(trigger.getAttribute("aria-expanded"), "false");
});

test("#72: clicking outside closes the listbox", () => {
  const view = renderSort("hot");
  const { trigger } = open(view);
  fireEvent.mouseDown(document.body);
  assert.equal(trigger.getAttribute("aria-expanded"), "false");
});
