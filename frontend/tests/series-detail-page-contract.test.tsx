import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import { ToastProvider } from "@/components/ui/Toast";
import { api, ApiRequestError } from "@/lib/api";
import { act, cleanup, installDom, waitFor } from "./runtime-test-helpers";

const originalGet = api.get;
const originalConsoleError = console.error;

test.beforeEach(() => {
  console.error = (...args: unknown[]) => {
    if (args.some((arg) => typeof arg === "object" && arg !== null && "code" in arg && (arg as { code?: string }).code === "ENVIRONMENT_FALLBACK")) return;
    originalConsoleError(...args);
  };
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  console.error = originalConsoleError;
});

test("public series detail route renders an ordered, accessible list", () => {
  const source = fs.readFileSync(new URL("../app/(public)/series/[id]/page.tsx", import.meta.url), "utf8");
  assert.match(source, /getSeriesDetail/);
  assert.match(source, /visibleItems\.map/);
  assert.doesNotMatch(source, /\.sort\(/, "the public page must preserve backend sort_order instead of re-sorting");
  assert.match(source, /count: series\.item_count/, "the summary must use the backend-visible item_count contract");
  assert.match(source, /series\.detail\.items\.ariaLabel/);
  assert.match(source, /EmptyState/);
  assert.match(source, /item\.content\.zone === "original" \? `\/original\//);
});

test("public series detail uses backend count and preserves backend item order", async () => {
  api.get = (async <T,>() => seriesResponse({ itemCount: 9 }) as T) as typeof api.get;
  const view = await renderSeriesDetailPage("7");

  await waitFor(() => {
    assert.ok(view.getByRole("heading", { name: "Mountain chapters" }));
    assert.ok(view.getByText("9 items"));
    const links = view.getAllByRole("link").filter((link) => link.getAttribute("href")?.startsWith("/original/"));
    assert.deepEqual(links.map((link) => link.textContent?.trim()), ["Item 1First chapterOriginal", "Item 2Second chapterOriginal"]);
  });
});

test("public series detail renders empty and not-found states without authentication", async (t) => {
  await t.test("empty", async () => {
    api.get = (async <T,>() => seriesResponse({ itemCount: 0, items: [] }) as T) as typeof api.get;
    const view = await renderSeriesDetailPage("7");
    await waitFor(() => assert.ok(view.getByText("No public content")));
    cleanup();
  });

  await t.test("not found", async () => {
    api.get = (async () => {
      throw new ApiRequestError("SERIES_NOT_FOUND", "missing", 404);
    }) as typeof api.get;
    const view = await renderSeriesDetailPage("404");
    await waitFor(() => assert.ok(view.getByText("Series unavailable")));
  });
});

async function renderSeriesDetailPage(id: string) {
  installDom();
  Object.defineProperty(globalThis, "requestAnimationFrame", { configurable: true, value: window.requestAnimationFrame.bind(window) });
  Object.defineProperty(globalThis, "cancelAnimationFrame", { configurable: true, value: window.cancelAnimationFrame.bind(window) });
  const { render } = await import("@testing-library/react");
  const { default: SeriesDetailPage } = await import("../app/(public)/series/[id]/page");
  let view: ReturnType<typeof render> | undefined;
  await act(async () => {
    view = render(
      <IntlProvider locale="en" messages={messages}>
        <ToastProvider>
          <SeriesDetailPage params={{ id }} />
        </ToastProvider>
      </IntlProvider>,
    );
    await Promise.resolve();
  });
  assert.ok(view);
  return view;
}

function seriesResponse(options: { itemCount: number; items?: unknown[] }) {
  return {
    series: { id: 7, title: "Mountain chapters", description: "Ordered field notes", zone: "original", owner: { id: 42, username: "Ada" }, cover: null, item_count: options.itemCount },
    items: options.items ?? [
      { id: 101, sort_order: 8, content_item_id: 501, content: { id: 501, title: "First chapter", zone: "original", status: "published" } },
      { id: 102, sort_order: 2, content_item_id: 502, content: { id: 502, title: "Second chapter", zone: "original", status: "published" } },
    ],
  };
}

const messages = {
  common: { close: "Close", retry: "Retry" },
  series: {
    detail: {
      header: { zoneOriginal: "Original", zoneFanwork: "Fanwork", itemCount: "{count} items", owner: "By {username}" },
      items: { title: "Series content", ariaLabel: "Series content list", itemLabel: "Item {index}" },
      empty: { title: "No public content", description: "Nothing published yet." },
      error: { title: "Series unavailable", description: "This series is unavailable.", loadFailed: "Load failed" },
      a11y: { cover: "{title} cover" },
    },
  },
};
