import assert from "node:assert/strict";
import test from "node:test";
import React, { Suspense } from "react";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import { IntlProvider } from "use-intl";

import { api, ApiRequestError, setAccessToken } from "@/lib/api";
import { AuthProvider } from "@/contexts/AuthContext";
import { ToastProvider } from "@/components/ui/Toast";
import { act, cleanup, installDom, waitFor } from "./runtime-test-helpers";

type ApiCall = {
  path: string;
  body?: unknown;
};

const originalGet = api.get;
const originalPost = api.post;
const originalPut = api.put;
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
  api.put = originalPut;
  api.delete = originalDelete;
  setAccessToken(null);
  console.error = originalConsoleError;
  console.warn = originalConsoleWarn;
});

test("own user collections page shows private and public collections", async () => {
  installUserCollectionsDom("/user/7/collections");
  setAccessToken(validAccessToken());
  const calls = installUserCollectionsApiMocks({
    authUser: { id: 7, username: "Ada" },
    collections: [
      collectionSummary({ id: 1, title: "Private research", is_public: false }),
      collectionSummary({ id: 2, title: "Public shelf", is_public: true }),
    ],
  });

  const view = await renderUserCollectionsPage("7");

  await waitFor(() => {
    assert.ok(view.getByText("Private research"));
    assert.ok(view.getByText("Public shelf"));
    assert.ok(view.getByRole("button", { name: "New collection" }));
    assert.ok(calls.get.some((call) => call.path === "/api/v1/collections?owner_id=7"));
  });
});

test("another user's collections page shows public collections only", async () => {
  installUserCollectionsDom("/user/7/collections");
  setAccessToken(validAccessToken());
  installUserCollectionsApiMocks({
    authUser: { id: 42, username: "Visitor" },
    collections: [collectionSummary({ id: 2, title: "Public shelf", is_public: true })],
  });

  const view = await renderUserCollectionsPage("7");

  await waitFor(() => {
    assert.ok(view.getByText("Public shelf"));
    assert.equal(view.queryByText("Private research"), null);
    assert.equal(view.queryByRole("button", { name: "New collection" }), null);
  });
});

test("logged-out user collections page shows public collections only", async () => {
  installUserCollectionsDom("/user/7/collections");
  const calls = installUserCollectionsApiMocks({
    collections: [collectionSummary({ id: 2, title: "Public shelf", is_public: true })],
  });

  const view = await renderUserCollectionsPage("7");

  await waitFor(() => {
    assert.ok(view.getByText("Public shelf"));
    assert.equal(view.queryByRole("button", { name: "New collection" }), null);
    assert.ok(calls.get.some((call) => call.path === "/api/v1/collections?owner_id=7"));
  });
});

test("empty own page shows create CTA", async () => {
  installUserCollectionsDom("/user/7/collections");
  setAccessToken(validAccessToken());
  installUserCollectionsApiMocks({
    authUser: { id: 7, username: "Ada" },
    collections: [],
  });

  const view = await renderUserCollectionsPage("7");

  await waitFor(() => {
    assert.ok(view.getByText("No collections yet"));
    assert.ok(view.getByText("Create a collection to organize saved content."));
    assert.ok(view.getAllByRole("button", { name: "New collection" }).length >= 1);
  });
});

test("empty visitor page shows read-only EmptyState", async () => {
  installUserCollectionsDom("/user/7/collections");
  installUserCollectionsApiMocks({ collections: [] });

  const view = await renderUserCollectionsPage("7");

  await waitFor(() => {
    assert.ok(view.getByText("No public collections"));
    assert.ok(view.getByText("This user has not shared any collections yet."));
    assert.equal(view.queryByRole("button", { name: "New collection" }), null);
  });
});

test("collection cards link to collection detail pages", async () => {
  installUserCollectionsDom("/user/7/collections");
  installUserCollectionsApiMocks({
    collections: [collectionSummary({ id: 123, title: "Public shelf", is_public: true })],
  });

  const view = await renderUserCollectionsPage("7");

  await waitFor(() => {
    const cardLink = view.getByRole("link", { name: /Public shelf/ });
    assert.equal(cardLink.getAttribute("href"), "/collections/123");
  });
});

async function renderUserCollectionsPage(userId: string) {
  const { render } = await import("@testing-library/react");
  const pageModule = await import("../app/(public)/user/[userId]/collections/page");
  const UserCollectionsPage = pageModule.default;

  let view: ReturnType<typeof render> | undefined;
  await act(async () => {
    view = render(
      <IntlProvider locale="en" messages={messages}>
        <AppRouterContext.Provider value={testRouter}>
          <ToastProvider>
            <AuthProvider>
              <Suspense fallback={<div>Loading suspense</div>}>
                <UserCollectionsPage params={Promise.resolve({ userId })} />
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

function installUserCollectionsDom(path: string) {
  const dom = installDom();
  dom.window.history.replaceState({}, "", path);
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: dom.window.localStorage,
  });
  return dom;
}

function installUserCollectionsApiMocks(options: {
  authUser?: { id: number; username: string };
  collections?: unknown[];
}) {
  const calls: { get: ApiCall[]; post: ApiCall[]; put: ApiCall[]; delete: ApiCall[] } = {
    get: [],
    post: [],
    put: [],
    delete: [],
  };

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (path === "/api/v1/auth/me") {
      if (!options.authUser) {
        throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
      }
      return {
        user: {
          ...options.authUser,
          email: "",
          avatar_url: "",
          bio: "",
          reputation: 10,
          preferred_locale: "en",
          role: "user",
          is_banned: false,
          email_verified_at: null,
          created_at: "",
        },
      } as T;
    }
    if (path.startsWith("/api/v1/notifications/unread-count")) {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path.startsWith("/api/v1/collections?owner_id=")) {
      return { items: options.collections ?? [], total: options.collections?.length ?? 0 } as T;
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

  api.put = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.put.push({ path, body });
    return {} as T;
  }) as typeof api.put;

  api.delete = (async <T,>(path: string): Promise<T> => {
    calls.delete.push({ path });
    return undefined as T;
  }) as typeof api.delete;

  return calls;
}

function collectionSummary(overrides: Record<string, unknown>) {
  return {
    id: 1,
    user_id: 7,
    title: "Public shelf",
    description: "Shared bookmarks",
    zone: "original",
    is_default: false,
    is_public: true,
    item_count: 3,
    contains_item: false,
    ...overrides,
  };
}

const testRouter = {
  back() {},
  forward() {},
  prefetch() {},
  bfcacheId: "test-bfcache",
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
    retry: "Retry",
    edit: "Edit",
    delete: "Delete",
    save: "Save",
    reason: "Reason",
    userLabel: "User #{id}",
  },
  collections: {
    card: {
      public: "Public",
      private: "Private",
      default: "Default",
      itemCount: "{count} items",
      edit: "Edit {title}",
      delete: "Delete {title}",
      deleteDisabled: "Default collections cannot be deleted",
    },
    userList: {
      header: {
        title: "{name}'s collections",
        subtitle: "{count} visible collections",
      },
      actions: {
        create: "New collection",
        refresh: "Refresh",
      },
      form: {
        createTitle: "Create collection",
        editTitle: "Edit collection",
        title: "Title",
        description: "Description",
        isPublic: "Public collection",
      },
      empty: {
        ownerTitle: "No collections yet",
        ownerDescription: "Create a collection to organize saved content.",
        visitorTitle: "No public collections",
        visitorDescription: "This user has not shared any collections yet.",
      },
      error: {
        title: "Collections unavailable",
        description: "Please try again later.",
      },
      delete: {
        title: "Delete collection?",
        description: "{title} will be removed.",
        confirm: "Delete",
      },
      toast: {
        loadFailed: "Failed to load collections",
        created: "Collection created",
        updated: "Collection updated",
        deleted: "Collection deleted",
        saveFailed: "Failed to save collection",
        deleteFailed: "Failed to delete collection",
      },
      a11y: {
        grid: "User collections",
      },
    },
  },
};
