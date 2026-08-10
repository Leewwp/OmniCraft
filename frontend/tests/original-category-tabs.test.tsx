import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";

/* Stub next/navigation so CategoryTabs renders without Next.js providers.
   Module._load interception pattern follows content-detail-overlay.test.tsx;
   the component must be imported dynamically after this patch. */
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

type CategoryTabsModule = typeof import("@/components/original/CategoryTabs");
let CategoryTabs: CategoryTabsModule["CategoryTabs"];

test.before(async () => {
  const mod = await import("@/components/original/CategoryTabs");
  CategoryTabs = mod.CategoryTabs;
});

test.afterEach(() => {
  cleanup();
  routerPushes.length = 0;
  mockSearchString = "";
});

const CATEGORIES = [
  { slug: "", i18n: "home.categoryRecommended" },
  { slug: "film_tv", i18n: "home.categoryFilmTv" },
  { slug: "gaming", i18n: "home.categoryGaming" },
];

function renderTabs(currentCategory = "") {
  installDom();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <CategoryTabs categories={CATEGORIES} currentCategory={currentCategory} />
    </IntlProvider>,
  );
}

function assertPushes(want: string[]) {
  assert.equal(routerPushes.length, want.length, `pushes = ${JSON.stringify(routerPushes)}`);
  for (let i = 0; i < want.length; i += 1) {
    const gotParams = new URLSearchParams(routerPushes[i].replace(/^\/original\??/, ""));
    const wantParams = new URLSearchParams(want[i].replace(/^\/original\??/, ""));
    assert.deepEqual(
      Object.fromEntries(gotParams),
      Object.fromEntries(wantParams),
      `push ${i}: got ${routerPushes[i]}, want ${want[i]}`,
    );
  }
}

test("#81: clicking a category tab pushes ?category=<slug>", () => {
  const view = renderTabs();
  fireEvent.click(view.getByRole("tab", { name: "Film & TV" }));
  assertPushes(["/original?category=film_tv"]);

  fireEvent.click(view.getByRole("tab", { name: "Gaming" }));
  assertPushes(["/original?category=film_tv", "/original?category=gaming"]);
});

test("#81: clicking the recommended tab removes the category param", () => {
  const view = renderTabs("film_tv");
  fireEvent.click(view.getByRole("tab", { name: "Recommended" }));
  assertPushes(["/original"]);
});

test("#81: clicking a category drops an explicit sort=recommended so the request never carries it", () => {
  mockSearchString = "sort=recommended";
  const view = renderTabs();
  fireEvent.click(view.getByRole("tab", { name: "Film & TV" }));
  assertPushes(["/original?category=film_tv"]);
});

test("#81: clicking a category preserves an explicit non-recommended sort", () => {
  mockSearchString = "sort=hot";
  const view = renderTabs();
  fireEvent.click(view.getByRole("tab", { name: "Film & TV" }));
  assertPushes(["/original?category=film_tv&sort=hot"]);
});

test("#81: switching category keeps an explicit most_views sort", () => {
  mockSearchString = "category=film_tv&sort=most_views";
  const view = renderTabs("film_tv");
  fireEvent.click(view.getByRole("tab", { name: "Gaming" }));
  assertPushes(["/original?category=gaming&sort=most_views"]);
});
