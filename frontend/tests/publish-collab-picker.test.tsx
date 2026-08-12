import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { api } from "@/lib/api";
import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";
import { CollabUserPicker } from "@/components/content/CollabUserPicker";
import { ToastProvider } from "@/components/ui/Toast";
import { clearPublicConfigCache } from "@/lib/public-config";

const originalGet = api.get;
const originalPost = api.post;
const originalFetch = globalThis.fetch;

interface SelectedUser {
  id: number;
  username: string;
  avatarUrl?: string;
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

function searchUser(id: number, username: string, avatarUrl?: string): Record<string, unknown> {
  return { id, username, ...(avatarUrl !== undefined ? { avatar_url: avatarUrl } : {}) };
}

function PickerHarness({
  initial = [],
  maxSelected,
  disabled,
  onChange,
}: {
  initial?: SelectedUser[];
  maxSelected: number;
  disabled?: boolean;
  onChange?: (users: SelectedUser[]) => void;
}) {
  const [selectedUsers, setSelectedUsers] = React.useState<SelectedUser[]>(initial);
  return (
    <IntlProvider locale="en" messages={enMessages}>
      <CollabUserPicker
        selectedUsers={selectedUsers}
        maxSelected={maxSelected}
        disabled={disabled}
        onChange={(users) => {
          setSelectedUsers(users);
          onChange?.(users);
        }}
      />
    </IntlProvider>
  );
}

async function renderPicker(
  props: {
    initial?: SelectedUser[];
    maxSelected?: number;
    disabled?: boolean;
    onChange?: (users: SelectedUser[]) => void;
  } = {},
) {
  const { render } = await import("@testing-library/react");
  const selections: SelectedUser[][] = [];
  const view = render(
    <PickerHarness
      initial={props.initial}
      maxSelected={props.maxSelected ?? 5}
      disabled={props.disabled}
      onChange={(users) => {
        selections.push(users);
        props.onChange?.(users);
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
  clearPublicConfigCache();
  routerPushes.length = 0;
  mockSearchString = "";
  publishMock = null;
});

test("picker searches users by username and calls exactly GET /api/v1/users/search?q=<query>&limit=8", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker();

  const input = view.getByRole("combobox", { name: enMessages.collabUserPicker.label });
  fireEvent.change(input, { target: { value: "Lumi" } });

  await waitFor(() => assert.equal(apiCalls.length, 1));
  assert.equal(apiCalls[0].url, "/api/v1/users/search?q=Lumi&limit=8");

  apiCalls[0].resolve({ users: [searchUser(1, "Luminary", "https://cdn.example/1.png")], total: 1 });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Luminary/ })));
});

test("picker result normalization keeps only id, username, and avatarUrl", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker();

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Lumi" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({
    users: [searchUser(1, "Luminary", "https://cdn.example/1.png")].map((user) => ({
      ...user,
      reputation: 10,
      role: "creator",
    })),
    total: 1,
  });
  const option = await waitFor(() => view.getByRole("option", { name: /Luminary/ }));
  fireEvent.click(option);

  await waitFor(() => assert.equal(selections.length, 1));
  assert.deepEqual(selections[0], [{ id: 1, username: "Luminary", avatarUrl: "https://cdn.example/1.png" }]);
  assert.equal("reputation" in selections[0][0]!, false, "reputation must not leak into the selection");
  assert.equal("role" in selections[0][0]!, false, "role must not leak into the selection");
});

test("picker drops results without numeric id or non-empty username", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker();

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Mix" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({
    users: [
      searchUser(11, "Valid User"),
      { id: 12, username: "" },
      { id: 13 },
      { id: "14", username: "String Id" },
      { id: 15, username: "   " },
    ],
    total: 5,
  });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Valid User/ })));
  assert.equal(view.queryByRole("option", { name: /String Id/ }), null);
  assert.equal(view.queryByRole("option", { name: "" }), null);
});

test("selected collaborators can be removed", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({
    initial: [{ id: 1, username: "Luminary", avatarUrl: "https://cdn.example/1.png" }],
  });

  const removeLabel = enMessages.collabUserPicker.a11y.removeUser.replace("{username}", "Luminary");
  fireEvent.click(view.getByRole("button", { name: removeLabel }));

  await waitFor(() => assert.equal(selections.length, 1));
  assert.deepEqual(selections[0], []);
});

test("duplicate selected users cannot be added and render a localized disabled state", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({
    initial: [{ id: 1, username: "Luminary" }],
  });

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Lumi" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({ users: [searchUser(1, "Luminary")], total: 1 });
  const option = await waitFor(() => view.getByRole("option", { name: /Luminary/ }));

  assert.equal(option.getAttribute("aria-selected"), "true", "already-selected option must carry aria-selected");
  assert.equal(option.getAttribute("aria-disabled"), "true", "already-selected option must be disabled");
  assert.ok(
    view.getByText(enMessages.collabUserPicker.duplicate.alreadySelected),
    "duplicate option explains the disabled state with localized text",
  );

  fireEvent.click(option);
  assert.equal(selections.length, 0, "clicking a duplicate must not change selection");
});

test("no more than maxSelected users can be selected and the limit message is localized", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({ maxSelected: 2 });

  for (const [index, username] of ["Alice", "Bob"].entries()) {
    const input = view.getByRole("combobox") as HTMLInputElement;
    fireEvent.change(input, { target: { value: username } });
    await waitFor(() => assert.equal(apiCalls.length, index + 1));
    apiCalls[index].resolve({ users: [searchUser(index + 1, username)], total: 1 });
    const option = await waitFor(() => view.getByRole("option", { name: new RegExp(username) }));
    fireEvent.click(option);
  }

  await waitFor(() => assert.equal(selections.length, 2));
  assert.deepEqual(selections[1], [
    { id: 1, username: "Alice" },
    { id: 2, username: "Bob" },
  ]);

  const maxReached = enMessages.collabUserPicker.selected.maxReached.replace("{max}", "2");
  assert.ok(view.getByText(maxReached), "limit message renders once the cap is reached");

  const input = view.getByRole("combobox") as HTMLInputElement;
  assert.equal(input.disabled, true, "search input is disabled when the cap is reached");
  fireEvent.change(input, { target: { value: "Carol" } });
  assert.equal(apiCalls.length, 2, "no search fires while the cap is reached");
});

test("CollabUserPicker receives the public value as maxSelected", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({ maxSelected: 5 });

  for (let index = 0; index < 5; index += 1) {
    const username = `User${index + 1}`;
    const input = view.getByRole("combobox") as HTMLInputElement;
    fireEvent.change(input, { target: { value: username } });
    await waitFor(() => assert.equal(apiCalls.length, index + 1));
    apiCalls[index].resolve({ users: [searchUser(index + 1, username)], total: 1 });
    const option = await waitFor(() => view.getByRole("option", { name: new RegExp(username) }));
    fireEvent.click(option);
  }

  await waitFor(() => assert.equal(selections.length, 5));
  assert.deepEqual(selections[4], Array.from({ length: 5 }, (_, index) => ({ id: index + 1, username: `User${index + 1}` })));
});

test("zero maxSelected keeps selection unavailable with localized explanation and no api calls", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({ maxSelected: 0 });

  const input = view.getByRole("combobox") as HTMLInputElement;
  assert.equal(input.disabled, true, "search input must be disabled when selection is unavailable");
  assert.ok(view.getByText(enMessages.collabUserPicker.disabled.unavailable), "unavailable state renders a localized explanation");

  fireEvent.change(input, { target: { value: "Lumi" } });
  assert.equal(apiCalls.length, 0, "no search api call may fire while selection is unavailable");
});

test("loading, empty, and error states render localized text", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker();
  const input = view.getByRole("combobox");

  fireEvent.change(input, { target: { value: "Wait" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  assert.ok(view.getByText(enMessages.collabUserPicker.search.loading));
  apiCalls[0].resolve({ users: [], total: 0 });
  await waitFor(() => assert.ok(view.getByText(enMessages.collabUserPicker.search.empty)));

  failNextSearch = true;
  fireEvent.change(input, { target: { value: "Fail" } });
  await waitFor(() => assert.equal(apiCalls.length, 2));
  await waitFor(() => assert.ok(view.getByText(enMessages.collabUserPicker.error.searchFailed)));
  fireEvent.click(view.getByRole("button", { name: enMessages.collabUserPicker.error.retry }));
  await waitFor(() => assert.equal(apiCalls.length, 3));
  apiCalls[2].resolve({ users: [searchUser(9, "Recovered")], total: 1 });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Recovered/ })));
});

test("disabled picker disables the search input and chip removal", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({
    initial: [{ id: 1, username: "Luminary" }],
    disabled: true,
  });

  const input = view.getByRole("combobox") as HTMLInputElement;
  assert.equal(input.disabled, true);
  const removeLabel = enMessages.collabUserPicker.a11y.removeUser.replace("{username}", "Luminary");
  const remove = view.getByRole("button", { name: removeLabel });
  assert.equal(remove.hasAttribute("disabled"), true);
});

/* ────────────────────────────────────────────────────────────────────────────
   Publish-flow integration (collab plan Task 7)
   Renders the real /studio/publish pages under the Module._load stub harness
   (pattern from studio-publish-fanwork.test.tsx): next/navigation,
   AuthContext, MarkdownEditor, and .css interceptions. api.post is deferred
   so each test controls the contents-create and per-invite responses; the
   contents-create response uses the contract-correct { content: { id } } shape.
   ──────────────────────────────────────────────────────────────────────────── */

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

type FanworkPageModule = typeof import("@/app/(protected)/studio/publish/fanwork/page");
type OriginalPageModule = typeof import("@/app/(protected)/studio/publish/original/page");
let PublishFanworkPage: FanworkPageModule["default"];
let PublishOriginalPage: OriginalPageModule["default"];

test.before(async () => {
  const fanwork = await import("@/app/(protected)/studio/publish/fanwork/page");
  PublishFanworkPage = fanwork.default;
  const original = await import("@/app/(protected)/studio/publish/original/page");
  PublishOriginalPage = original.default;
});

const PUBLIC_CONFIG = {
  features: {
    web_agent_enabled: false,
    payment_enabled: false,
    creator_support_enabled: false,
    desktop_deploy_enabled: false,
  },
  captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
  client: { download_enabled: false, download_url: "", latest_version: "" },
  legal: { current_terms_version: "test", current_privacy_version: "test" },
  upload: {
    image_gallery_min_items: 2,
    image_gallery_max_items: 9,
    video_gallery_min_items: 1,
    video_gallery_max_items: 3,
  },
  collaboration: { max_invitees_per_publish: 5 },
};

interface PublishPostCall {
  path: string;
  body: Record<string, unknown>;
  resolve: (data: unknown) => void;
  reject: (error: Error) => void;
}

interface PublishGetCall {
  url: string;
  resolve: (data: unknown) => void;
  reject: (error: Error) => void;
}

interface PublishMockState {
  posts: PublishPostCall[];
  getCalls: PublishGetCall[];
}

let publishMock: PublishMockState | null = null;

function installPublishMocks() {
  publishMock = { posts: [], getCalls: [] };
  api.get = <T,>(path: string): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      publishMock!.getCalls.push({ url: path, resolve: (data: unknown) => resolve(data as T), reject });
    });
  api.post = <T,>(path: string, body: unknown): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      publishMock!.posts.push({
        path,
        body: body as Record<string, unknown>,
        resolve: (data: unknown) => resolve(data as T),
        reject,
      });
    });
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes("/api/v1/config/public")) {
      return new Response(JSON.stringify(PUBLIC_CONFIG));
    }
    throw new Error("test fetch stub: no real network calls");
  }) as typeof fetch;
  return publishMock!;
}

type View = ReturnType<typeof import("@testing-library/react").render>;

const publishButton = (view: View) => view.getByRole("button", { name: /^Publish$/i });
const collabPicker = (view: View) => view.getByRole("combobox", { name: enMessages.collabUserPicker.label });
const invitePosts = (mock: PublishMockState) => mock.posts.filter((post) => post.path.endsWith("/collab-invites"));

async function renderFanworkPage(searchString = "", withToast = false) {
  mockSearchString = searchString;
  installDom();
  const mock = installPublishMocks();
  const { render } = await import("@testing-library/react");
  const page = withToast ? (
    <IntlProvider locale="en" messages={enMessages}>
      <ToastProvider>
        <PublishFanworkPage />
      </ToastProvider>
    </IntlProvider>
  ) : (
    <IntlProvider locale="en" messages={enMessages}>
      <PublishFanworkPage />
    </IntlProvider>
  );
  const view = render(page);
  fireEvent.click(view.getByRole("button", { name: /Article/i }));
  return { view, mock };
}

async function renderOriginalPage() {
  mockSearchString = "";
  installDom();
  const mock = installPublishMocks();
  const { render } = await import("@testing-library/react");
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <PublishOriginalPage />
    </IntlProvider>,
  );
  fireEvent.click(view.getByRole("button", { name: /Article/i }));
  return { view, mock };
}

async function pickIp(view: View, mock: PublishMockState) {
  const before = mock.getCalls.length;
  fireEvent.change(view.getByPlaceholderText("Search and select IP..."), { target: { value: "Star" } });
  await waitFor(() => assert.equal(mock.getCalls.length, before + 1));
  mock.getCalls[before]!.resolve({ ips: [{ id: 42, name: "Star Rail" }] });
  const option = await waitFor(() => view.getByRole("button", { name: "Star Rail" }));
  fireEvent.click(option);
}

async function pickOriginalSource(view: View, mock: PublishMockState) {
  const before = mock.getCalls.length;
  fireEvent.change(view.getByPlaceholderText("Search original content title..."), { target: { value: "Original" } });
  await waitFor(() => assert.equal(mock.getCalls.length, before + 1));
  assert.equal(mock.getCalls[before]!.url, "/api/v1/contents/search?zone=original&q=Original&limit=8");
  mock.getCalls[before]!.resolve({ items: [{ id: 77, title: "Original Lightcone", zone: "original", status: "published" }] });
  const option = await waitFor(() => view.getByRole("option", { name: /Original Lightcone/ }));
  fireEvent.click(option);
}

async function pickCollabUser(view: View, mock: PublishMockState, id: number, username: string) {
  const input = collabPicker(view);
  const before = mock.getCalls.length;
  fireEvent.change(input, { target: { value: username } });
  await waitFor(() => assert.equal(mock.getCalls.length, before + 1));
  const call = mock.getCalls[before]!;
  assert.ok(call.url.startsWith("/api/v1/users/search"), `expected user search, got ${call.url}`);
  call.resolve({ users: [{ id, username }], total: 1 });
  const option = await waitFor(() => view.getByRole("option", { name: new RegExp(username) }));
  fireEvent.click(option);
}

test("publish flow: after content creation succeeds one invite POST is sent per selected collaborator", async () => {
  const { view, mock } = await renderFanworkPage();
  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Collab fanwork" } });
  await pickIp(view, mock);
  await pickCollabUser(view, mock, 1, "Luminary");
  await pickCollabUser(view, mock, 2, "Lumina");

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0]!.path, "/api/v1/contents");
  assert.equal(mock.posts[0]!.body.zone, "fanwork");
  mock.posts[0]!.resolve({ content: { id: 999 } });

  await waitFor(() => assert.equal(invitePosts(mock).length, 2));
  assert.ok(invitePosts(mock).every((post) => post.path === "/api/v1/contents/999/collab-invites"));
  assert.deepEqual(invitePosts(mock).map((post) => post.body), [{ invitee_id: 1 }, { invitee_id: 2 }]);

  for (const post of invitePosts(mock)) post.resolve({ invite: { id: 0 } });
  await waitFor(() => assert.deepEqual(routerPushes, ["/studio/contents"]));
});

test("publish flow: invite failure shows a warning toast without marking the publish itself failed", async () => {
  const { view, mock } = await renderFanworkPage("", true);
  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Collab fail fanwork" } });
  await pickIp(view, mock);
  await pickCollabUser(view, mock, 1, "Luminary");

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  mock.posts[0]!.resolve({ content: { id: 999 } });
  await waitFor(() => assert.equal(invitePosts(mock).length, 1));
  invitePosts(mock)[0]!.reject(new Error("INVITE_BLOCKED"));

  await waitFor(() => assert.ok(view.getByText(enMessages.studio.publish.success)));
  await waitFor(() => {
    const warning = enMessages.studio.publish.collab.inviteFailed
      .replace("{usernames}", "Luminary")
      .replace("{url}", "/content/999");
    assert.ok(view.getByText(warning), "warning toast must list the failed username and the content link");
  });
  await waitFor(() => assert.deepEqual(routerPushes, ["/studio/contents"]));
});

test("publish flow: duplicate selected collaborators cannot be added and only unique invites are sent", async () => {
  const { view, mock } = await renderFanworkPage();
  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Duplicate collab fanwork" } });
  await pickIp(view, mock);
  await pickCollabUser(view, mock, 1, "Luminary");

  const input = collabPicker(view);
  const before = mock.getCalls.length;
  fireEvent.change(input, { target: { value: "Lumi" } });
  await waitFor(() => assert.equal(mock.getCalls.length, before + 1));
  mock.getCalls[before]!.resolve({ users: [{ id: 1, username: "Luminary" }], total: 1 });
  const duplicate = await waitFor(() => view.getByRole("option", { name: /Luminary/ }));
  assert.equal(duplicate.getAttribute("aria-disabled"), "true", "duplicate option must be disabled");
  fireEvent.click(duplicate);
  const removeLabel = enMessages.collabUserPicker.a11y.removeUser.replace("{username}", "Luminary");
  assert.equal(view.queryAllByRole("button", { name: removeLabel }).length, 1, "duplicate click must not add a second chip");

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  mock.posts[0]!.resolve({ content: { id: 999 } });
  await waitFor(() => assert.equal(invitePosts(mock).length, 1));
  assert.deepEqual(invitePosts(mock).map((post) => post.body), [{ invitee_id: 1 }]);
  invitePosts(mock)[0]!.resolve({ invite: { id: 0 } });
  await waitFor(() => assert.deepEqual(routerPushes, ["/studio/contents"]));
});

test("publish flow: original publish also supports selected collaborators and sends invites", async () => {
  const { view, mock } = await renderOriginalPage();
  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Collab original" } });
  fireEvent.click(view.getByRole("button", { name: "Film & TV" }));
  await pickCollabUser(view, mock, 1, "Luminary");

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  assert.equal(mock.posts[0]!.path, "/api/v1/contents");
  assert.equal(mock.posts[0]!.body.zone, "original");
  mock.posts[0]!.resolve({ content: { id: 999 } });

  await waitFor(() => assert.equal(invitePosts(mock).length, 1));
  assert.deepEqual(invitePosts(mock).map((post) => post.body), [{ invitee_id: 1 }]);
  invitePosts(mock)[0]!.resolve({ invite: { id: 0 } });
  await waitFor(() => assert.deepEqual(routerPushes, ["/studio/contents"]));
});

test("publish flow: source-linkage fields still submit exactly one source id with collaborators selected", async () => {
  const { view, mock } = await renderFanworkPage();
  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Source + collab fanwork" } });
  await pickOriginalSource(view, mock);
  await pickCollabUser(view, mock, 1, "Luminary");
  await pickCollabUser(view, mock, 2, "Lumina");

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  const body = mock.posts[0]!.body;
  assert.equal(body.source_original_id, 77);
  assert.equal("source_fanwork_id" in body, false, "fanwork source must stay cleared");
  assert.equal("ip_id" in body, false);
  assert.equal("collaborators" in body, false, "collaborators must not leak into the contents payload");
  assert.equal("invitee_ids" in body, false, "invitees must not leak into the contents payload");
  mock.posts[0]!.resolve({ content: { id: 999 } });
  await waitFor(() => assert.equal(invitePosts(mock).length, 2));
  for (const post of invitePosts(mock)) post.resolve({ invite: { id: 0 } });
  await waitFor(() => assert.deepEqual(routerPushes, ["/studio/contents"]));
});

test("publish flow: collaborator picker sits after the fanwork source fields and before submit actions in both zones", async () => {
  const { view } = await renderFanworkPage();
  const fanworkPicker = await waitFor(() => collabPicker(view));
  const fieldset = view.getByText(enMessages.studio.publish.fanwork.source.legend).closest("fieldset");
  assert.ok(fieldset, "fanwork source fieldset must exist");
  assert.ok(
    (fieldset as HTMLElement).compareDocumentPosition(fanworkPicker) & Node.DOCUMENT_POSITION_FOLLOWING,
    "picker must render after the fanwork source fieldset",
  );
  assert.ok(
    fanworkPicker.compareDocumentPosition(publishButton(view)) & Node.DOCUMENT_POSITION_FOLLOWING,
    "picker must render before the submit button (fanwork)",
  );

  const original = await renderOriginalPage();
  const originalPicker = await waitFor(() => collabPicker(original.view));
  assert.ok(
    originalPicker.compareDocumentPosition(publishButton(original.view)) & Node.DOCUMENT_POSITION_FOLLOWING,
    "picker must render before the submit button (original)",
  );
});

test("publish flow: invite requests run at most three concurrently", async () => {
  const { view, mock } = await renderFanworkPage();
  fireEvent.change(view.getByPlaceholderText("Enter work title"), { target: { value: "Concurrency fanwork" } });
  await pickIp(view, mock);
  for (let id = 1; id <= 4; id += 1) {
    await pickCollabUser(view, mock, id, `User${id}`);
  }

  fireEvent.click(publishButton(view));
  await waitFor(() => assert.equal(mock.posts.length, 1));
  mock.posts[0]!.resolve({ content: { id: 999 } });

  await waitFor(() => assert.equal(invitePosts(mock).length, 3));
  assert.equal(invitePosts(mock).length, 3, "batch one must not exceed three concurrent invites");
  for (const post of invitePosts(mock)) post.resolve({ invite: { id: post.body.invitee_id as number } });

  await waitFor(() => assert.equal(invitePosts(mock).length, 4));
  assert.deepEqual(invitePosts(mock).map((post) => post.body.invitee_id).sort(), [1, 2, 3, 4]);
  invitePosts(mock)
    .filter((post) => post.body.invitee_id === 4)[0]!
    .resolve({ invite: { id: 4 } });

  await waitFor(() => assert.deepEqual(routerPushes, ["/studio/contents"]));
});
