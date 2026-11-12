import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";

import { api } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";

import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";

const originalGet = api.get;
const originalDelete = api.delete;
const originalDeleteWithBody = api.deleteWithBody;

const messages = {
  common: {
    close: "Close",
    cancel: "Cancel",
    confirm: "Confirm",
    processing: "Processing",
    home: "Home",
    reason: "Reason",
  },
  history: {
    title: "Browsing History",
    clearAll: "Clear all",
    today: "Today",
    yesterday: "Yesterday",
    date: "{month}/{day}/{year}",
    filters: {
      all: "All",
      article: "Article",
      video: "Video",
      image: "Image",
      audio: "Audio",
      template: "Template",
      sheet_music: "Sheet music",
      mod: "Mod",
      prompt: "Prompt",
      other: "Other",
    },
    dateRange: {
      start: "Start date",
      end: "End date",
      retention: "Showing records from the last {days} days.",
      retentionUnknown: "Showing recent browsing records.",
    },
    bulk: {
      enter: "Batch manage",
      exit: "Exit batch",
      deleteSelected: "Delete selected ({count})",
      select: "Select {title}",
      selectUnavailable: "Select unavailable item from {date}",
      confirmSelectedTitle: "Delete selected records?",
      confirmSelectedDescription: "Selected browsing records will be removed.",
      confirmAllTitle: "Clear browsing history?",
      confirmAllDescription: "All browsing records will be removed.",
      confirmDelete: "Delete",
    },
    empty: {
      title: "No browsing history",
      description: "Go discover something interesting.",
      action: "Go home",
    },
    error: {
      load: "Failed to load browsing history.",
      inline: "Could not refresh. Showing the last successful result.",
      invalidDateRange: "Start date must be before end date.",
    },
    unavailable: {
      title: "Content unavailable",
      description: "This content was deleted or unpublished.",
    },
    toast: {
      deleted: "Browsing history updated.",
      deleteFailed: "Failed to update browsing history.",
    },
    a11y: {
      toolbar: "Browsing history filters",
    },
  },
};

type ApiCall = {
  path: string;
  body?: unknown;
};

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.delete = originalDelete;
  api.deleteWithBody = originalDeleteWithBody;
});

test("history page prefers items and still renders legacy history content_item", async () => {
  installHistoryDom();
  installHistoryApiMock({
    response: {
      items: [historyItem(1, "New item title", "article")],
      history: [legacyHistoryItem(2, "Legacy title", "video")],
      total: 1,
      page: 1,
      page_size: 20,
      retention_days: 7,
    },
  });

  const view = await renderHistoryPage();

  await waitFor(() => {
        if (!(view.getByText("New item title"))) throw new Error("waitFor: condition not met");
        if (view.queryByText("Legacy title") != null) throw new Error("waitFor: values differ");
        if (!(view.getByText("Showing records from the last 7 days."))) throw new Error("waitFor: condition not met");
  });
});

test("history page falls back to legacy history and renders unavailable placeholders", async () => {
  installHistoryDom();
  installHistoryApiMock({
    response: {
      history: [
        legacyHistoryItem(2, "Legacy title", "video"),
        { id: 3, content: null, content_item: null, viewed_at: "2026-07-02T04:00:00Z" },
      ],
      total: 2,
      page: 1,
      page_size: 20,
    },
  });

  const view = await renderHistoryPage();

  await waitFor(() => {
        if (!(view.getByText("Legacy title"))) throw new Error("waitFor: condition not met");
        if (!(view.getByText("Content unavailable"))) throw new Error("waitFor: condition not met");
        if (!(view.getByText("Showing recent browsing records."))) throw new Error("waitFor: condition not met");
  });
});

test("history filters call the API with content type and date query", async () => {
  installHistoryDom();
  const calls = installHistoryApiMock({
    response: { items: [historyItem(1, "Article title", "article")], history: [], total: 1, page: 1, page_size: 20, retention_days: 7 },
  });

  const view = await renderHistoryPage();
  await waitFor(() => {
    if (!(view.getByText("Article title"))) throw new Error("waitFor: condition not met");
  });

  fireEvent.click(view.getByRole("button", { name: "Article" }));
  await waitFor(() => {
        if (!(calls.get.some((call) => call.path.includes("content_type=article")))) throw new Error("waitFor: condition not met");
  });

  fireEvent.change(view.getByLabelText("Start date"), { target: { value: "2026-07-01" } });
  fireEvent.change(view.getByLabelText("End date"), { target: { value: "2026-07-02" } });
  await waitFor(() => {
        if (!(calls.get.some((call) => call.path.includes("start_date=2026-07-01") && call.path.includes("end_date=2026-07-02")))) throw new Error("waitFor: condition not met");
  });
});

test("history page initializes filters from URL query", async () => {
  installHistoryDom("/history?content_type=video&start_date=2026-07-01&end_date=2026-07-02");
  const calls = installHistoryApiMock({
    response: { items: [historyItem(1, "Video title", "video")], history: [], total: 1, page: 1, page_size: 20, retention_days: 7 },
  });

  const view = await renderHistoryPage();

  await waitFor(() => {
        if (!(view.getByText("Video title"))) throw new Error("waitFor: condition not met");
        if (!(calls.get[0].path.includes("content_type=video"))) throw new Error("waitFor: condition not met");
        if (!(calls.get[0].path.includes("start_date=2026-07-01"))) throw new Error("waitFor: condition not met");
        if (!(calls.get[0].path.includes("end_date=2026-07-02"))) throw new Error("waitFor: condition not met");
  });
});

test("history page rejects invalid date ranges without requesting", async () => {
  installHistoryDom();
  const calls = installHistoryApiMock({
    response: { items: [historyItem(1, "Stable title", "article")], history: [], total: 1, page: 1, page_size: 20, retention_days: 7 },
  });
  const view = await renderHistoryPage();
  await waitFor(() => {
    if (!(view.getByText("Stable title"))) throw new Error("waitFor: condition not met");
  });

  fireEvent.change(view.getByLabelText("End date"), { target: { value: "2026-07-02" } });
  await waitFor(() => {
    if (!(calls.get.some((call) => call.path.includes("end_date=2026-07-02")))) throw new Error("waitFor: condition not met");
  });
  const validRangeCalls = calls.get.length;

  fireEvent.change(view.getByLabelText("Start date"), { target: { value: "2026-07-03" } });

  await waitFor(() => {
        if (!(view.getByText("Start date must be before end date."))) throw new Error("waitFor: condition not met");
        if (calls.get.length != validRangeCalls) throw new Error("waitFor: values differ");
  });
});

test("history batch delete sends selected ids in a DELETE body", async () => {
  installHistoryDom();
  const calls = installHistoryApiMock({
    response: {
      items: [historyItem(1, "First title", "article"), historyItem(2, "Second title", "video")],
      history: [],
      total: 2,
      page: 1,
      page_size: 20,
      retention_days: 7,
    },
  });
  const view = await renderHistoryPage();

  await waitFor(() => {
    if (!(view.getByText("First title"))) throw new Error("waitFor: condition not met");
  });
  fireEvent.click(view.getByRole("button", { name: "Batch manage" }));
  fireEvent.click(view.getByLabelText("Select First title"));
  fireEvent.click(view.getByRole("button", { name: "Delete selected (1)" }));
  fireEvent.click(await findDialogConfirmButton(view));

  await waitFor(() => {
    if (calls.deleteWithBody.length < 1) throw new Error("waitFor: expected a DELETE call");
    if (!(document.body.textContent?.includes("Browsing history updated."))) throw new Error("waitFor: condition not met");
  });
  assert.deepEqual(calls.deleteWithBody.at(-1), {
    path: "/api/v1/users/me/history",
    body: { ids: [1] },
  });
});

test("history clear-all waits for confirmation and sends the explicit clear tag", async () => {
  installHistoryDom();
  const calls = installHistoryApiMock({
    response: {
      items: [historyItem(1, "Protected title", "article")],
      history: [],
      total: 1,
      page: 1,
      page_size: 20,
      retention_days: 7,
    },
  });
  const view = await renderHistoryPage();

  await waitFor(() => {
    if (!(view.getByText("Protected title"))) throw new Error("waitFor: condition not met");
  });
  fireEvent.click(view.getByRole("button", { name: "Clear all" }));

  assert.equal(calls.deleteWithBody.length, 0, "opening the confirmation must not delete history");

  fireEvent.click(await findDialogConfirmButton(view));

  await waitFor(() => {
    if (calls.deleteWithBody.length < 1) throw new Error("waitFor: expected a DELETE call");
  });
  assert.deepEqual(calls.deleteWithBody.at(-1), {
    path: "/api/v1/users/me/history",
    body: { clear_all: true },
  });
});

test("history page keeps last successful data when refresh fails", async () => {
  installHistoryDom();
  let failNext = false;
  installHistoryApiMock({
    response: { items: [historyItem(1, "Stable title", "article")], history: [], total: 1, page: 1, page_size: 20, retention_days: 7 },
    shouldFail: () => failNext,
  });
  const view = await renderHistoryPage();
  await waitFor(() => {
    if (!(view.getByText("Stable title"))) throw new Error("waitFor: condition not met");
  });

  failNext = true;
  fireEvent.click(view.getByRole("button", { name: "Video" }));

  await waitFor(() => {
        if (!(view.getByText("Stable title"))) throw new Error("waitFor: condition not met");
        if (!(view.getByText("Could not refresh. Showing the last successful result."))) throw new Error("waitFor: condition not met");
        if (!(document.body.textContent?.includes("Failed to load browsing history."))) throw new Error("waitFor: condition not met");
  });
});

async function renderHistoryPage() {
  const { render } = await import("@testing-library/react");
  const pageModule = await import("../app/(protected)/(headered)/history/page");
  const HistoryPage = pageModule.default;
  return render(
    <IntlProvider locale="en" messages={messages}>
      <AppRouterContext.Provider value={testRouter}>
        <ToastProvider>
          <HistoryPage />
        </ToastProvider>
      </AppRouterContext.Provider>
    </IntlProvider>,
  );
}

const testRouter = {
  back() {},
  forward() {},
  prefetch() {},
  bfcacheId: "test-bfcache",
  push() {},
  refresh() {},
  replace() {},
};

function installHistoryDom(path = "/history") {
  const dom = installDom();
  dom.window.history.replaceState({}, "", path);
  Object.defineProperty(globalThis, "requestAnimationFrame", {
    configurable: true,
    writable: true,
    value: dom.window.requestAnimationFrame,
  });
  Object.defineProperty(globalThis, "cancelAnimationFrame", {
    configurable: true,
    writable: true,
    value: dom.window.cancelAnimationFrame,
  });
}

function installHistoryApiMock(options: {
  response: unknown;
  shouldFail?: () => boolean;
}) {
  const calls: { get: ApiCall[]; deleteWithBody: ApiCall[] } = { get: [], deleteWithBody: [] };
  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (options.shouldFail?.()) {
      throw new Error("network failed");
    }
    return options.response as T;
  }) as typeof api.get;
  api.deleteWithBody = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.deleteWithBody.push({ path, body });
    return { message: "cleared" } as T;
  }) as typeof api.deleteWithBody;
  return calls;
}

function historyItem(id: number, title: string, contentType: string) {
  return {
    id,
    content: {
      id: id + 100,
      title,
      zone: "original",
      content_type: contentType,
      cover_image_url: "",
      author: { id: 1, username: "author", avatar_url: "" },
    },
    content_item: {
      id: id + 100,
      title,
      zone: "original",
      content_type: contentType,
      cover_image_url: "",
      author: { id: 1, username: "author", avatar_url: "" },
    },
    viewed_at: "2026-07-02T04:00:00Z",
  };
}

function legacyHistoryItem(id: number, title: string, contentType: string) {
  const item = historyItem(id, title, contentType);
  return {
    id: item.id,
    content_item: item.content_item,
    viewed_at: item.viewed_at,
  };
}

async function findDialogConfirmButton(view: Awaited<ReturnType<typeof renderHistoryPage>>) {
  let button: HTMLButtonElement | undefined;
  await waitFor(() => {
    const dialog = view.getByRole("dialog");
    button = Array.from(dialog.querySelectorAll("button")).find((candidate) => candidate.textContent === "Delete");
  });
  assert.ok(button);
  return button;
}
