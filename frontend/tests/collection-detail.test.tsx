import assert from "node:assert/strict";
import test from "node:test";
import React, { Suspense } from "react";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import { IntlProvider } from "use-intl";

import { api, ApiRequestError, setAccessToken } from "@/lib/api";
import { AuthProvider } from "@/contexts/AuthContext";
import { ToastProvider } from "@/components/ui/Toast";
import { act, cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";

type ApiCall = {
  path: string;
  body?: unknown;
};

const originalGet = api.get;
const originalPost = api.post;
const originalDelete = api.delete;
const originalConsoleError = console.error;
const originalConsoleWarn = console.warn;

test.beforeEach(() => {
  console.error = (...args: unknown[]) => {
    if (
      args.some(
        (arg) =>
          typeof arg === "object" &&
          arg !== null &&
          "code" in arg &&
          (arg as { code?: string }).code === "ENVIRONMENT_FALLBACK",
      )
    ) {
      return;
    }
    originalConsoleError(...args);
  };
  console.warn = (...args: unknown[]) => {
    if (args.some((arg) => typeof arg === "string" && arg.includes("[silent-api-error] AuthContext:refresh"))) {
      return;
    }
    originalConsoleWarn(...args);
  };
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.post = originalPost;
  api.delete = originalDelete;
  setAccessToken(null);
  console.error = originalConsoleError;
  console.warn = originalConsoleWarn;
});

test("public collection detail renders for logged-out users", async () => {
  installCollectionDom("/collections/9");
  const calls = installCollectionApiMocks({
    collectionResponse: collectionDetailResponse({
      title: "Public references",
      is_public: true,
      user_id: 7,
      owner: { id: 7, username: "Ada" },
    }),
  });

  const view = await renderCollectionDetailPage("9");

  await waitFor(() => {
    assert.ok(view.getByText("Public references"));
    assert.ok(view.getByText("A careful shelf"));
    assert.ok(view.getAllByText("Ada").length >= 1);
    assert.ok(view.getByText("Story board"));
    assert.equal(calls.get.find((call) => call.path.startsWith("/api/v1/collections/9"))?.path, "/api/v1/collections/9?page=1&page_size=20");
  });
});

test("collection detail not found state does not leak private titles", async () => {
  installCollectionDom("/collections/404");
  installCollectionApiMocks({
    collectionError: new ApiRequestError("COLLECTION_NOT_FOUND", "collection not found", 404),
  });

  const view = await renderCollectionDetailPage("404");

  await waitFor(() => {
    assert.ok(view.getByText("Collection unavailable"));
    assert.equal(view.queryByText("Private strategy board"), null);
  });
});

test("CollectionInfoCard shows summary metadata and owner-only disabled default delete", async () => {
  installCollectionDom("/collections/9");
  setAccessToken(validAccessToken());
  installCollectionApiMocks({
    authUser: { id: 7, username: "Ada" },
    collectionResponse: collectionDetailResponse({
      title: "Default originals",
      zone: "original",
      is_public: false,
      is_default: true,
      user_id: 7,
      owner: { id: 7, username: "Ada" },
    }),
  });

  const view = await renderCollectionDetailPage("9");

  await waitFor(() => {
    assert.ok(view.getByText("Default originals"));
    assert.ok(view.getByText("Private"));
    assert.ok(view.getByText("Original"));
    assert.ok(view.getByText("2 items"));
    assert.ok(view.getByRole("button", { name: "Edit collection" }));
    assert.equal((view.getByRole("button", { name: "Delete collection" }) as HTMLButtonElement).disabled, true);
  });
});

test("ContentTypeFilter updates the content_type query and refetches", async () => {
  installCollectionDom("/collections/9");
  const calls = installCollectionApiMocks({
    collectionResponse: collectionDetailResponse({ title: "Public references", is_public: true }),
  });

  const view = await renderCollectionDetailPage("9");
  await waitFor(() => assert.ok(view.getByText("Public references")));

  fireEvent.click(view.getByRole("tab", { name: "Video" }));

  await waitFor(() => {
    assert.ok(window.location.search.includes("content_type=video"));
    assert.ok(calls.get.some((call) => call.path === "/api/v1/collections/9?page=1&page_size=20&content_type=video"));
  });
});

test("non-owner does not see owner controls", async () => {
  installCollectionDom("/collections/9");
  setAccessToken(validAccessToken());
  installCollectionApiMocks({
    authUser: { id: 42, username: "Visitor" },
    collectionResponse: collectionDetailResponse({
      title: "Shared references",
      is_public: true,
      user_id: 7,
      owner: { id: 7, username: "Ada" },
    }),
  });

  const view = await renderCollectionDetailPage("9");

  await waitFor(() => {
    assert.ok(view.getByText("Shared references"));
    assert.equal(view.queryByRole("button", { name: "Edit collection" }), null);
    assert.equal(view.queryByRole("button", { name: "Delete collection" }), null);
  });
});

async function renderCollectionDetailPage(collectionId: string) {
  const { render } = await import("@testing-library/react");
  const pageModule = await import("../app/(public)/collections/[id]/page");
  const CollectionDetailPage = pageModule.default;

  let view: ReturnType<typeof render> | undefined;
  await act(async () => {
    view = render(
      <IntlProvider locale="en" messages={messages}>
        <AppRouterContext.Provider value={testRouter}>
          <ToastProvider>
            <AuthProvider>
              <Suspense fallback={<div>Loading suspense</div>}>
                <CollectionDetailPage
                  params={{ id: collectionId }}
                  searchParams={Object.fromEntries(new URLSearchParams(window.location.search))}
                />
              </Suspense>
            </AuthProvider>
          </ToastProvider>
        </AppRouterContext.Provider>
      </IntlProvider>,
    );
    await Promise.resolve();
  });

  assert.ok(view);
  return view;
}

function installCollectionDom(path: string) {
  const dom = installDom();
  dom.window.history.replaceState({}, "", path);
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: dom.window.localStorage,
  });
  return dom;
}

function installCollectionApiMocks(options: {
  authUser?: { id: number; username: string };
  collectionResponse?: unknown;
  collectionError?: Error;
}) {
  const calls: { get: ApiCall[]; post: ApiCall[]; delete: ApiCall[] } = {
    get: [],
    post: [],
    delete: [],
  };

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (path === "/api/v1/auth/me") {
      if (!options.authUser) {
        throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
      }
      return { user: { ...options.authUser, email: "", avatar_url: "", bio: "", reputation: 10, preferred_locale: "en", role: "user", is_banned: false, email_verified_at: null, created_at: "" } } as T;
    }
    if (path.startsWith("/api/v1/notifications/unread-count")) {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path.startsWith("/api/v1/collections/")) {
      if (options.collectionError) {
        throw options.collectionError;
      }
      return (options.collectionResponse ?? collectionDetailResponse({})) as T;
    }
    return {} as T;
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    return {} as T;
  }) as typeof api.post;

  api.delete = (async <T,>(path: string): Promise<T> => {
    calls.delete.push({ path });
    return undefined as T;
  }) as typeof api.delete;

  return calls;
}

function collectionDetailResponse(overrides: Record<string, unknown>) {
  return {
    collection: {
      id: 9,
      user_id: 7,
      title: "Private strategy board",
      description: "A careful shelf",
      zone: "original",
      is_public: true,
      is_default: false,
      item_count: 2,
      owner: { id: 7, username: "Ada" },
      ...overrides,
    },
    items: [
      {
        id: 101,
        content_item: {
          id: 501,
          title: "Story board",
          zone: "original",
          content_type: "article",
          cover_image_url: "",
          author: { id: 7, username: "Ada" },
        },
      },
    ],
    total: 1,
    page: 1,
    page_size: 20,
  };
}

const testRouter = {
  back() {},
  forward() {},
  prefetch() {},
  push(path: string) {
    window.history.pushState({}, "", path);
  },
  refresh() {},
  replace(path: string) {
    window.history.replaceState({}, "", path);
  },
};

function validAccessToken() {
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }));
  return `header.${payload}.signature`;
}

const messages = {
  common: {
    close: "Close",
    cancel: "Cancel",
    confirm: "Confirm",
    processing: "Processing",
    reason: "Reason",
    retry: "Retry",
    edit: "Edit",
    delete: "Delete",
    userLabel: "User #{id}",
  },
  content: {
    emptyContentMsg: "No content",
    emptyContentHint: "New work will appear here.",
    basedOnIp: "Based on {name}",
    text: "Text",
  },
  home: {
    article: "Article",
    image: "Image",
    video: "Video",
    audio: "Audio",
    template: "Template",
    sheetMusic: "Sheet Music",
    aiPrompt: "AI Prompt",
    mod: "Mod",
    text: "Text",
  },
  collections: {
    detail: {
      empty: {
        title: "Empty collection",
        description: "No content has been saved here yet.",
      },
      error: {
        title: "Collection unavailable",
        description: "This collection is private or no longer exists.",
      },
      ownerActions: {
        edit: "Edit collection",
        delete: "Delete collection",
        deleteTitle: "Delete collection?",
        deleteDescription: "{title} will be removed.",
        deleteConfirm: "Delete",
      },
      a11y: {
        grid: "Collection contents",
      },
    },
    info: {
      public: "Public",
      private: "Private",
      default: "Default",
      original: "Original",
      fanwork: "Fan creation",
      itemCount: "{count} items",
      author: "{name}",
      noDescription: "No description",
      defaultDeleteDisabled: "Default collections cannot be deleted",
    },
    filters: {
      label: "Content type",
      all: "All",
      article: "Article",
      image: "Image",
      video: "Video",
      audio: "Audio",
      template: "Template",
      sheet_music: "Sheet music",
      mod: "Mod",
      prompt: "Prompt",
      other: "Other",
    },
  },
};
