import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import { ToastProvider } from "@/components/ui/Toast";
import { api } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";

type ApiCall = { path: string; body?: unknown };

const originalGet = api.get;
const originalPost = api.post;
const originalPut = api.put;
const originalDelete = api.delete;
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
  api.post = originalPost;
  api.put = originalPut;
  api.delete = originalDelete;
  console.error = originalConsoleError;
});

test("studio series page exposes create, edit, item, reorder, and delete actions", () => {
  const source = fs.readFileSync(new URL("../app/(protected)/studio/series/page.tsx", import.meta.url), "utf8");
  for (const contract of [
    "listOwnedSeries",
    "createSeries",
    "updateSeries",
    "deleteSeries",
    "addSeriesItem",
    "removeSeriesItem",
    "reorderSeriesItems",
    "item_ids",
    "cover_content_id",
    "aria-label",
  ]) {
    assert.match(source, new RegExp(contract.replace(/[.*+?^${}()|[\\]\\]/g, "\\$&")), `missing ${contract}`);
  }
  assert.doesNotMatch(source, /draggable=|onDrag(Start|End|Over)/);
});

test("studio series creates a public series through the API", async () => {
  const calls = installSeriesApiMocks();
  const view = await renderStudioSeriesPage();

  await waitFor(() => assert.ok(view.getAllByText("Mountain chapters").length >= 1));
  fireEvent.click(view.getByRole("button", { name: "Create series" }));
  fireEvent.change(view.getAllByLabelText("Series title")[0], { target: { value: "Coastal chapters" } });
  fireEvent.click(view.getByRole("button", { name: "Create" }));

  await waitFor(() => {
    const create = calls.post.find((call) => call.path === "/api/v1/series");
    assert.deepEqual(create?.body, { title: "Coastal chapters", description: "", zone: "original" });
  });
});

test("studio series edits, adds, reorders, removes, and deletes with exact payloads", async () => {
  const calls = installSeriesApiMocks();
  const view = await renderStudioSeriesPage();

  await waitFor(() => assert.ok(view.getByDisplayValue("Mountain chapters")));
  fireEvent.change(view.getByLabelText("Series title"), { target: { value: "Mountain chapters revised" } });
  fireEvent.click(view.getByRole("button", { name: "Save changes" }));
  await waitFor(() => {
    const update = calls.put.find((call) => call.path === "/api/v1/series/7");
    assert.deepEqual(update?.body, {
      title: "Mountain chapters revised",
      description: "Ordered field notes",
      cover_content_id: null,
    });
  });

  fireEvent.change(view.getByLabelText("Search content to add"), { target: { value: "Third" } });
  await waitFor(() => assert.ok(view.getByText("Third chapter")));
  fireEvent.click(view.getByRole("button", { name: "Add" }));
  await waitFor(() => {
    assert.deepEqual(calls.post.find((call) => call.path === "/api/v1/series/7/items")?.body, { content_item_id: 503 });
  });

  fireEvent.click(view.getByRole("button", { name: "Move Second chapter up" }));
  await waitFor(() => {
    assert.deepEqual(calls.put.find((call) => call.path === "/api/v1/series/7/items/reorder")?.body, { item_ids: [102, 101] });
  });

  fireEvent.click(view.getByRole("button", { name: "Remove Second chapter from series" }));
  await waitFor(() => assert.ok(calls.delete.some((call) => call.path === "/api/v1/series/7/items/102")));

  fireEvent.click(view.getByRole("button", { name: "Delete series" }));
  const deleteButtons = view.getAllByRole("button", { name: "Delete series" });
  fireEvent.click(deleteButtons[deleteButtons.length - 1]);
  await waitFor(() => assert.ok(calls.delete.some((call) => call.path === "/api/v1/series/7")));
});

async function renderStudioSeriesPage() {
  installDom();
  Object.defineProperty(globalThis, "requestAnimationFrame", { configurable: true, value: window.requestAnimationFrame.bind(window) });
  Object.defineProperty(globalThis, "cancelAnimationFrame", { configurable: true, value: window.cancelAnimationFrame.bind(window) });
  const { render } = await import("@testing-library/react");
  const { default: StudioSeriesPage } = await import("../app/(protected)/studio/series/page");
  let view: ReturnType<typeof render> | undefined;
  await act(async () => {
    view = render(
      <IntlProvider locale="en" messages={messages}>
        <ToastProvider>
          <StudioSeriesPage />
        </ToastProvider>
      </IntlProvider>,
    );
    await Promise.resolve();
  });
  assert.ok(view);
  return view;
}

function installSeriesApiMocks() {
  const calls = { get: [] as ApiCall[], post: [] as ApiCall[], put: [] as ApiCall[], delete: [] as ApiCall[] };
  let summaries = [{ id: 7, title: "Mountain chapters", description: "Ordered field notes", zone: "original" }];

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (path === "/api/v1/series") return { items: summaries } as T;
    if (path.startsWith("/api/v1/series/candidates")) {
      return { items: [{ id: 503, title: "Third chapter", zone: "original", status: "published" }] } as T;
    }
    const id = path.includes("/8?") ? 8 : 7;
    return seriesDetail(id, summaries.find((item) => item.id === id)?.title ?? "Mountain chapters") as T;
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path === "/api/v1/series") {
      const created = { id: 8, title: "Coastal chapters", description: "", zone: "original" };
      summaries = [...summaries, created];
      return { series: created } as T;
    }
    return { item: { id: 103, content_item_id: 503, sort_order: 2 } } as T;
  }) as typeof api.post;

  api.put = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.put.push({ path, body });
    return { message: "ok", series: summaries[0] } as T;
  }) as typeof api.put;

  api.delete = (async <T,>(path: string): Promise<T> => {
    calls.delete.push({ path });
    return { message: "ok" } as T;
  }) as typeof api.delete;

  return calls;
}

function seriesDetail(id: number, title: string) {
  return {
    series: { id, title, description: "Ordered field notes", zone: "original", owner: { id: 42, username: "Ada" }, cover: null, item_count: 2 },
    items: [
      { id: 101, sort_order: 0, content_item_id: 501, content: { id: 501, title: "First chapter", zone: "original", status: "published" } },
      { id: 102, sort_order: 1, content_item_id: 502, content: { id: 502, title: "Second chapter", zone: "original", status: "published" } },
    ],
  };
}

const messages = {
  common: { cancel: "Cancel", retry: "Retry", reason: "Reason", processing: "Processing" },
  series: { detail: { header: { zoneOriginal: "Original", zoneFanwork: "Fanwork" } } },
  studio: {
    series: {
      title: "Content series",
      subtitle: "Organize public content and maintain its chapter order.",
      create: "Create series",
      delete: "Delete series",
      empty: { title: "No content series yet", description: "Create a series." },
      list: { ariaLabel: "My content series" },
      detail: { ariaLabel: "Series details" },
      form: { title: "Series title", description: "Series description", zone: "Zone", cover: "Series cover", coverAutomatic: "Automatic cover", createSubmit: "Create", save: "Save changes" },
      items: { title: "Added content", ariaLabel: "Series ordering list", empty: "No content yet", add: "Add" },
      search: { title: "Add content", label: "Search content to add", placeholder: "Search content" },
      confirm: { deleteTitle: "Delete series?", deleteDescription: "Delete {title}?" },
      toast: { created: "Created", saved: "Saved", deleted: "Deleted", itemAdded: "Added", itemRemoved: "Removed", reordered: "Reordered" },
      error: { loadFailed: "Load failed", saveFailed: "Save failed", deleteFailed: "Delete failed", itemFailed: "Item failed" },
      a11y: { backToList: "Back to series list", moveUp: "Move {title} up", moveDown: "Move {title} down", removeItem: "Remove {title} from series" },
    },
  },
};
