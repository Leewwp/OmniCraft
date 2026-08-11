import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { api, setAccessToken } from "@/lib/api";
import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";
import { SourceContentPicker } from "@/components/studio/SourceContentPicker";

const originalGet = api.get;

interface SourceContent {
  id: number;
  title: string;
  zone: "original" | "fanwork";
}

type PickerApiCall = {
  url: string;
  resolve: (data: unknown) => void;
  reject: (error: Error) => void;
};

let apiCalls: PickerApiCall[] = [];
let failNextSearch = false;

function installSearchMock() {
  apiCalls = [];
  failNextSearch = false;
  api.get = <T,>(path: string): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      apiCalls.push({ url: path, resolve: (data: unknown) => resolve(data as T), reject });
      if (failNextSearch) {
        failNextSearch = false;
        reject(new Error("network error"));
      }
    });
}

function publishedItem(id: number, title: string, zone: "original" | "fanwork"): Record<string, unknown> {
  return { id, title, zone, status: "published" };
}

function PickerHarness({
  sourceKind,
  selected,
  disabled,
  onSelect,
}: {
  sourceKind: "original" | "fanwork";
  selected?: SourceContent;
  disabled?: boolean;
  onSelect?: (content?: SourceContent) => void;
}) {
  const [current, setCurrent] = React.useState<SourceContent | undefined>(selected);
  return (
    <IntlProvider locale="en" messages={enMessages}>
      <SourceContentPicker
        sourceKind={sourceKind}
        selected={current}
        disabled={disabled}
        onSelect={(content) => {
          setCurrent(content);
          onSelect?.(content);
        }}
      />
    </IntlProvider>
  );
}

async function renderPicker(
  props: Partial<{
    sourceKind: "original" | "fanwork";
    selected: SourceContent;
    disabled: boolean;
    onSelect: (content?: SourceContent) => void;
  }> = {},
) {
  const { render } = await import("@testing-library/react");
  const selections: Array<SourceContent | undefined> = [];
  const view = render(
    <PickerHarness
      sourceKind={props.sourceKind ?? "original"}
      selected={props.selected}
      disabled={props.disabled}
      onSelect={(content) => {
        selections.push(content);
        props.onSelect?.(content);
      }}
    />,
  );
  return { view, selections };
}

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  setAccessToken(null);
});

test("sourceKind=original searches GET /api/v1/contents/search?zone=original&q=<query>&limit=8", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({ sourceKind: "original" });

  const input = view.getByRole("combobox", { name: enMessages.sourceContentPicker.original.label });
  fireEvent.change(input, { target: { value: "Original" } });

  await waitFor(() => assert.equal(apiCalls.length, 1));
  assert.equal(apiCalls[0].url, "/api/v1/contents/search?zone=original&q=Original&limit=8");

  apiCalls[0].resolve({ items: [publishedItem(1, "Original Post", "original")] });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Original Post/ })));
});

test("sourceKind=fanwork searches GET /api/v1/contents/search?zone=fanwork&q=<query>&limit=8", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({ sourceKind: "fanwork" });

  const input = view.getByRole("combobox", { name: enMessages.sourceContentPicker.fanwork.label });
  fireEvent.change(input, { target: { value: "Fan" } });

  await waitFor(() => assert.equal(apiCalls.length, 1));
  assert.equal(apiCalls[0].url, "/api/v1/contents/search?zone=fanwork&q=Fan&limit=8");

  apiCalls[0].resolve({ items: [publishedItem(2, "Fanwork Post", "fanwork")] });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Fanwork Post/ })));
});

test("anonymous and authenticated searches render the same source-selectable published subset", async () => {
  installDom();
  installSearchMock();

  const { view: anonView } = await renderPicker({ sourceKind: "original" });
  fireEvent.change(anonView.getByRole("combobox"), { target: { value: "Mix" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({
    items: [
      publishedItem(1, "Visible Original", "original"),
      { id: 2, title: "Banned Row", zone: "original", status: "banned" },
    ],
  });
  await waitFor(() => assert.ok(anonView.getByRole("option", { name: /Visible Original/ })));
  assert.equal(anonView.queryByRole("option", { name: /Banned Row/ }), null);
  anonView.unmount();

  setAccessToken("test-access-token");
  const { view: authView } = await renderPicker({ sourceKind: "original" });
  fireEvent.change(authView.getByRole("combobox"), { target: { value: "Mix" } });
  await waitFor(() => assert.equal(apiCalls.length, 2));
  apiCalls[1].resolve({
    items: [
      publishedItem(1, "Visible Original", "original"),
      { id: 2, title: "Banned Row", zone: "original", status: "banned" },
    ],
  });
  await waitFor(() => assert.ok(authView.getByRole("option", { name: /Visible Original/ })));
  assert.equal(authView.queryByRole("option", { name: /Banned Row/ }), null);
});

test("banned, author-deleted, soft-deleted, and under-review rows never appear as picker results", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({ sourceKind: "original" });

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Mix" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({
    items: [
      publishedItem(1, "Selectable Original", "original"),
      { id: 2, title: "Banned Status", zone: "original", status: "banned" },
      { id: 3, title: "Under Review", zone: "original", status: "pending" },
      { id: 4, title: "Soft Deleted", zone: "original", status: "published", deleted_at: "2026-01-01T00:00:00Z" },
      { id: 5, title: "Author Deleted", zone: "original", status: "published", author: { deleted_at: "2026-01-01T00:00:00Z" } },
      { id: 6, title: "Banned Author", zone: "original", status: "published", author: { is_banned: true } },
    ],
  });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Selectable Original/ })));
  for (const title of ["Banned Status", "Under Review", "Soft Deleted", "Author Deleted", "Banned Author"]) {
    assert.equal(view.queryByRole("option", { name: new RegExp(title) }), null, title);
  }
});

test("result rows require numeric id, non-empty title, and matching zone", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({ sourceKind: "fanwork" });

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Mix" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({
    items: [
      publishedItem(11, "Valid Fanwork", "fanwork"),
      { id: 12, title: "", zone: "fanwork", status: "published" },
      { id: 13, zone: "fanwork", status: "published" },
      { id: "14", title: "String Id", zone: "fanwork", status: "published" },
      publishedItem(15, "Wrong Zone", "original"),
    ],
  });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Valid Fanwork/ })));
  assert.equal(view.queryByRole("option", { name: /String Id/ }), null);
  assert.equal(view.queryByRole("option", { name: /Wrong Zone/ }), null);
});

test("selecting a result emits the selected content summary", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({ sourceKind: "original" });

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Lumi" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({ items: [publishedItem(42, "Luminary", "original")] });
  const option = await waitFor(() => view.getByRole("option", { name: /Luminary/ }));
  fireEvent.click(option);

  await waitFor(() => assert.equal(selections.length, 1));
  assert.deepEqual(selections[0], { id: 42, title: "Luminary", zone: "original" });
  await waitFor(() =>
    assert.ok(view.getByRole("button", { name: enMessages.sourceContentPicker.selected.clear })),
  );
});

test("clearing the selection emits undefined", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({
    sourceKind: "original",
    selected: { id: 7, title: "Chosen Original", zone: "original" },
  });

  fireEvent.click(view.getByRole("button", { name: enMessages.sourceContentPicker.selected.clear }));
  await waitFor(() => assert.equal(selections.length, 1));
  assert.equal(selections[0], undefined);
});

test("loading, empty, and error states render localized text", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({ sourceKind: "original" });
  const input = view.getByRole("combobox");

  fireEvent.change(input, { target: { value: "Wait" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  assert.ok(view.getByText(enMessages.sourceContentPicker.search.loading));
  apiCalls[0].resolve({ items: [] });
  await waitFor(() => assert.ok(view.getByText(enMessages.sourceContentPicker.search.empty)));

  failNextSearch = true;
  fireEvent.change(input, { target: { value: "Fail" } });
  await waitFor(() => assert.equal(apiCalls.length, 2));
  await waitFor(() => assert.ok(view.getByText(enMessages.sourceContentPicker.error.searchFailed)));
  fireEvent.click(view.getByRole("button", { name: enMessages.sourceContentPicker.error.retry }));
  await waitFor(() => assert.equal(apiCalls.length, 3));
  apiCalls[2].resolve({ items: [publishedItem(9, "Recovered", "original")] });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Recovered/ })));
});

test("disabled picker disables the search input and clear control", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({
    sourceKind: "original",
    selected: { id: 3, title: "Fixed Original", zone: "original" },
    disabled: true,
  });

  const input = view.getByRole("combobox") as HTMLInputElement;
  assert.equal(input.disabled, true);
  const clear = view.getByRole("button", { name: enMessages.sourceContentPicker.selected.clear });
  assert.equal(clear.hasAttribute("disabled"), true);
});
