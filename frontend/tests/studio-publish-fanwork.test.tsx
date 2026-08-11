import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { api, setAccessToken } from "@/lib/api";
import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";
import { SourceContentPicker } from "@/components/studio/SourceContentPicker";

const originalGet = api.get;
const originalPost = api.post;
const originalFetch = globalThis.fetch;

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
  api.post = originalPost;
  globalThis.fetch = originalFetch;
  setAccessToken(null);
  mockSearchString = "";
  routerPushes.length = 0;
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

/* Stub next/navigation + AuthContext + MarkdownEditor so the publish page
   and PublishForm render without Next.js providers or the CSS-importing
   markdown package (Module._load interception pattern from
   original-category-tabs.test.tsx / admin-notifications-page.test.tsx).
   The page module must be imported dynamically after this patch. */
const requireForMocks = createRequire(import.meta.url) as NodeRequire & {
  extensions: NodeJS.RequireExtensions;
};
requireForMocks.extensions[".css"] = () => undefined;
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
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => ({
        user: { id: 5, email: "creator@example.com", email_verified_at: "2026-01-01T00:00:00Z" },
      }),
    };
  }
  if (request === "@/components/content/MarkdownEditor") {
    return {
      MarkdownEditor({
        value,
        onChange,
        disabled,
      }: {
        value: string;
        onChange: (value: string) => void;
        disabled?: boolean;
      }) {
        return (
          <textarea
            aria-label="content"
            data-testid="markdown-editor"
            disabled={disabled}
            value={value}
            onChange={(event) => onChange(event.currentTarget.value)}
          />
        );
      },
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type PublishPageModule = typeof import("@/app/(protected)/studio/publish/fanwork/page");
let PublishFanworkPage: PublishPageModule["default"];

test.before(async () => {
  const mod = await import("@/app/(protected)/studio/publish/fanwork/page");
  PublishFanworkPage = mod.default;
});

interface PublishGetCall {
  url: string;
  resolve: (data: unknown) => void;
  reject: (error: Error) => void;
}

interface PublishMockState {
  posts: Array<{ path: string; body: Record<string, unknown> }>;
  getCalls: PublishGetCall[];
}

let publishMock: PublishMockState | null = null;

function installPublishMocks() {
  publishMock = { posts: [], getCalls: [] };
  api.get = <T,>(path: string): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      publishMock!.getCalls.push({ url: path, resolve: (data: unknown) => resolve(data as T), reject });
    });
  api.post = async <T,>(path: string, body: unknown): Promise<T> => {
    publishMock!.posts.push({ path, body: body as Record<string, unknown> });
    return { id: 999 } as T;
  };
  globalThis.fetch = (async () => {
    throw new Error("test fetch stub: no real network calls");
  }) as typeof fetch;
  return publishMock!;
}

function publishedContentDetail(id: number, title: string, zone: "original" | "fanwork"): Record<string, unknown> {
  return {
    content: { id, title, zone, status: "published", author: { id: 1, username: "tester" } },
    attachments: [],
    tags: [],
    series_memberships: [],
  };
}

async function renderPublishPage(searchString = "") {
  mockSearchString = searchString;
  installDom();
  const mock = installPublishMocks();
  const { render } = await import("@testing-library/react");
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <PublishFanworkPage />
    </IntlProvider>,
  );
  fireEvent.click(view.getByRole("button", { name: /Article/i }));
  return { view, mock };
}

const publishButton = (view: ReturnType<typeof import("@testing-library/react").render>) =>
  view.getByRole("button", { name: /^Publish$/i });

test("publish fanwork with IP only sends ip_id and no source ids", async () => {
  const { view, mock } = await renderPublishPage();

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "IP only fanwork" } });
  fireEvent.change(view.getByPlaceholderText("Search and select IP..."), { target: { value: "Star" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  assert.equal(mock.getCalls[0].url, "/api/v1/ips?q=Star");
  mock.getCalls[0].resolve({ ips: [{ id: 42, name: "Star Rail" }] });
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Star Rail" })));
  fireEvent.click(view.getByRole("button", { name: "Star Rail" }));

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].path, "/api/v1/contents");
  assert.equal(mock.posts[0].body.zone, "fanwork");
  assert.equal(mock.posts[0].body.content_type, "article");
  assert.equal(mock.posts[0].body.ip_id, 42);
  assert.equal("source_original_id" in mock.posts[0].body, false);
  assert.equal("source_fanwork_id" in mock.posts[0].body, false);
});

test("publish fanwork with source original only sends source_original_id", async () => {
  const { view, mock } = await renderPublishPage();

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Original source fanwork" } });
  fireEvent.change(view.getByPlaceholderText("Search original content title..."), { target: { value: "Original" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  assert.equal(mock.getCalls[0].url, "/api/v1/contents/search?zone=original&q=Original&limit=8");
  mock.getCalls[0].resolve({ items: [publishedItem(77, "Original Lightcone", "original")] });
  const option = await waitFor(() => view.getByRole("option", { name: /Original Lightcone/ }));
  fireEvent.click(option);

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].body.source_original_id, 77);
  assert.equal("ip_id" in mock.posts[0].body, false);
  assert.equal("source_fanwork_id" in mock.posts[0].body, false);
});

test("publish fanwork with source fanwork only sends source_fanwork_id", async () => {
  const { view, mock } = await renderPublishPage();

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Fanwork source fanwork" } });
  fireEvent.change(view.getByPlaceholderText("Search fanwork content title..."), { target: { value: "Fan" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  assert.equal(mock.getCalls[0].url, "/api/v1/contents/search?zone=fanwork&q=Fan&limit=8");
  mock.getCalls[0].resolve({ items: [publishedItem(88, "Fanwork Piece", "fanwork")] });
  const option = await waitFor(() => view.getByRole("option", { name: /Fanwork Piece/ }));
  fireEvent.click(option);

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].body.source_fanwork_id, 88);
  assert.equal("ip_id" in mock.posts[0].body, false);
  assert.equal("source_original_id" in mock.posts[0].body, false);
});

test("submit with no IP/source is blocked before API call and shows localized validation", async () => {
  const { view, mock } = await renderPublishPage();

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "No source fanwork" } });

  const submit = publishButton(view);
  assert.equal(submit.hasAttribute("disabled"), true);
  fireEvent.click(submit);
  await waitFor(() => assert.equal(mock.posts.length, 0));
  assert.ok(view.getByText(enMessages.studio.publish.fanwork.validation.sourceRequired));
});

test("selecting source original clears source fanwork before submit", async () => {
  const { view, mock } = await renderPublishPage();

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Mutual exclusion" } });
  fireEvent.change(view.getByPlaceholderText("Search fanwork content title..."), { target: { value: "Fan" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  mock.getCalls[0].resolve({ items: [publishedItem(88, "Fanwork Piece", "fanwork")] });
  const fanworkOption = await waitFor(() => view.getByRole("option", { name: /Fanwork Piece/ }));
  fireEvent.click(fanworkOption);
  await waitFor(() => assert.ok(view.getByText("Fanwork Piece")));

  fireEvent.change(view.getByPlaceholderText("Search original content title..."), { target: { value: "Original" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 2));
  mock.getCalls[1].resolve({ items: [publishedItem(77, "Original Lightcone", "original")] });
  const originalOption = await waitFor(() => view.getByRole("option", { name: /Original Lightcone/ }));
  fireEvent.click(originalOption);
  await waitFor(() => assert.equal(view.queryByText("Fanwork Piece"), null));
  assert.ok(view.getByText("Original Lightcone"));

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].body.source_original_id, 77);
  assert.equal("source_fanwork_id" in mock.posts[0].body, false);
});

test("selecting source fanwork clears source original before submit", async () => {
  const { view, mock } = await renderPublishPage();

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Mutual exclusion" } });
  fireEvent.change(view.getByPlaceholderText("Search original content title..."), { target: { value: "Original" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  mock.getCalls[0].resolve({ items: [publishedItem(77, "Original Lightcone", "original")] });
  const originalOption = await waitFor(() => view.getByRole("option", { name: /Original Lightcone/ }));
  fireEvent.click(originalOption);
  await waitFor(() => assert.ok(view.getByText("Original Lightcone")));

  fireEvent.change(view.getByPlaceholderText("Search fanwork content title..."), { target: { value: "Fan" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 2));
  mock.getCalls[1].resolve({ items: [publishedItem(88, "Fanwork Piece", "fanwork")] });
  const fanworkOption = await waitFor(() => view.getByRole("option", { name: /Fanwork Piece/ }));
  fireEvent.click(fanworkOption);
  await waitFor(() => assert.equal(view.queryByText("Original Lightcone"), null));
  assert.ok(view.getByText("Fanwork Piece"));

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].body.source_fanwork_id, 88);
  assert.equal("source_original_id" in mock.posts[0].body, false);
});

test("query prefill ?source_original_id loads the original source summary", async () => {
  const { view, mock } = await renderPublishPage("source_original_id=77");

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Prefill fanwork" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  assert.equal(mock.getCalls[0].url, "/api/v1/contents/77");
  mock.getCalls[0].resolve(publishedContentDetail(77, "Original Lightcone", "original"));
  await waitFor(() => assert.ok(view.getByText("Original Lightcone")));
  assert.ok(view.getByText(enMessages.sourceContentPicker.selected.label));

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].body.source_original_id, 77);
  assert.equal("source_fanwork_id" in mock.posts[0].body, false);
});

test("query prefill ?source_fanwork_id loads the fanwork source summary", async () => {
  const { view, mock } = await renderPublishPage("source_fanwork_id=88");

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Prefill fanwork" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  assert.equal(mock.getCalls[0].url, "/api/v1/contents/88");
  mock.getCalls[0].resolve(publishedContentDetail(88, "Fanwork Piece", "fanwork"));
  await waitFor(() => assert.ok(view.getByText("Fanwork Piece")));
  assert.ok(view.getByText(enMessages.sourceContentPicker.selected.label));

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].body.source_fanwork_id, 88);
  assert.equal("source_original_id" in mock.posts[0].body, false);
});

test("query prefill with both IDs keeps source_original_id, clears source_fanwork_id, and shows the localized warning", async () => {
  const { view, mock } = await renderPublishPage("source_original_id=77&source_fanwork_id=88");

  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Prefill fanwork" } });
  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  assert.equal(mock.getCalls[0].url, "/api/v1/contents/77");
  mock.getCalls[0].resolve(publishedContentDetail(77, "Original Lightcone", "original"));
  await waitFor(() => assert.ok(view.getByText("Original Lightcone")));
  assert.ok(view.getByText(enMessages.studio.publish.fanwork.prefill.bothSources));

  assert.equal(mock.getCalls.length, 1);
  assert.equal(view.queryByText("Fanwork Piece"), null);

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0].body.source_original_id, 77);
  assert.equal("source_fanwork_id" in mock.posts[0].body, false);
});

test("invalid query prefill id shows a localized non-blocking warning and leaves the picker empty", async () => {
  const { view, mock } = await renderPublishPage("source_original_id=abc");

  await waitFor(() => assert.equal(mock.getCalls.length, 0));
  assert.ok(view.getByText(enMessages.studio.publish.fanwork.prefill.invalidId));
  assert.equal(view.queryByText(enMessages.sourceContentPicker.selected.label), null);

  const dismiss = view.getByRole("button", { name: enMessages.studio.publish.fanwork.a11y.dismissWarning });
  fireEvent.click(dismiss);
  await waitFor(() => assert.equal(view.queryByText(enMessages.studio.publish.fanwork.prefill.invalidId), null));
});

test("prefill source that cannot be loaded shows a dismissible inline error and leaves the picker empty", async () => {
  const { view, mock } = await renderPublishPage("source_original_id=404");

  await waitFor(() => assert.equal(mock.getCalls.length, 1));
  assert.equal(mock.getCalls[0].url, "/api/v1/contents/404");
  mock.getCalls[0].reject(new Error("not found"));
  await waitFor(() => assert.ok(view.getByText(enMessages.studio.publish.fanwork.prefill.loadFailed)));
  assert.equal(view.queryByText(enMessages.sourceContentPicker.selected.label), null);

  fireEvent.click(view.getByRole("button", { name: enMessages.studio.publish.fanwork.a11y.dismissWarning }));
  await waitFor(() => assert.equal(view.queryByText(enMessages.studio.publish.fanwork.prefill.loadFailed), null));

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 0));
});
