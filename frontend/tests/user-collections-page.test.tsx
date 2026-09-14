import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { api, ApiRequestError, setAccessToken } from "@/lib/api";
import { AuthProvider } from "@/contexts/AuthContext";
import { ToastProvider } from "@/components/ui/Toast";
import { act, cleanup, installDom, waitFor } from "./runtime-test-helpers";
import { ProfileSummaryCard } from "../app/(public)/user/[userId]/ProfileSummaryCard";
import { UserProfileClient } from "../app/(public)/user/[userId]/UserProfileClient";
import { normalizeProfileTab } from "../app/(public)/user/[userId]/profile-tab";

let rtl: typeof import("@testing-library/react") | null = null;

test.beforeEach(async () => {
  rtl = await import("@testing-library/react");
});

// SP-18 #508：收藏集独立路由收敛为个人主页页内 tab（旧路由 next.config 301）。
// 覆盖：tab 内 owner/访客/匿名可见性、空态 CTA、卡片详情链接、tab 切换 URL 同步、
// 主页头部三项统计 + 真实头像、?tab= 归一化。
// （原独立页面用例迁移：行为断言不变，渲染入口改为 UserProfileClient initialTab。）

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

test("profile collections tab", async (t) => {
  await t.test("own collections tab shows private and public collections", async () => {
    installProfileDom("/user/7?tab=collections");
    setAccessToken(validAccessToken());
    const calls = installProfileApiMocks({
      authUser: { id: 7, username: "Ada" },
      collections: [
        collectionSummary({ id: 1, title: "Private research", is_public: false }),
        collectionSummary({ id: 2, title: "Public shelf", is_public: true }),
      ],
    });

    const view = renderProfileClient(7, "Ada", "collections");

    await waitFor(() => {
      assert.ok(view.getByText("Private research"));
      assert.ok(view.getByText("Public shelf"));
      assert.ok(view.getByRole("button", { name: "New collection" }));
      assert.ok(calls.get.some((call) => call.path === "/api/v1/collections?owner_id=7"));
    });
  });

  await t.test("another user's collections tab shows public collections only", async () => {
    installProfileDom("/user/7?tab=collections");
    setAccessToken(validAccessToken());
    installProfileApiMocks({
      authUser: { id: 42, username: "Visitor" },
      collections: [collectionSummary({ id: 2, title: "Public shelf", is_public: true })],
    });

    const view = renderProfileClient(7, "Ada", "collections");

    await waitFor(() => {
      assert.ok(view.getByText("Public shelf"));
      assert.equal(view.queryByText("Private research"), null);
      assert.equal(view.queryByRole("button", { name: "New collection" }), null);
    });
  });

  await t.test("logged-out collections tab shows public collections only", async () => {
    installProfileDom("/user/7?tab=collections");
    const calls = installProfileApiMocks({
      collections: [collectionSummary({ id: 2, title: "Public shelf", is_public: true })],
    });

    const view = renderProfileClient(7, "Ada", "collections");

    await waitFor(() => {
      assert.ok(view.getByText("Public shelf"));
      assert.equal(view.queryByRole("button", { name: "New collection" }), null);
      assert.ok(calls.get.some((call) => call.path === "/api/v1/collections?owner_id=7"));
    });
  });

  await t.test("empty own tab shows create CTA", async () => {
    installProfileDom("/user/7?tab=collections");
    setAccessToken(validAccessToken());
    installProfileApiMocks({
      authUser: { id: 7, username: "Ada" },
      collections: [],
    });

    const view = renderProfileClient(7, "Ada", "collections");

    await waitFor(() => {
      assert.ok(view.getByText("No collections yet"));
      assert.ok(view.getAllByRole("button", { name: "New collection" }).length >= 1);
    });
  });

  await t.test("empty visitor tab shows read-only EmptyState", async () => {
    installProfileDom("/user/7?tab=collections");
    installProfileApiMocks({ collections: [] });

    const view = renderProfileClient(7, "Ada", "collections");

    await waitFor(() => {
      assert.ok(view.getByText("No public collections"));
      assert.equal(view.queryByRole("button", { name: "New collection" }), null);
    });
  });

  await t.test("collection cards link to collection detail pages", async () => {
    installProfileDom("/user/7?tab=collections");
    installProfileApiMocks({
      collections: [collectionSummary({ id: 123, title: "Public shelf", is_public: true })],
    });

    const view = renderProfileClient(7, "Ada", "collections");

    await waitFor(() => {
      const cardLink = view.getByRole("link", { name: /Public shelf/ });
      assert.equal(cardLink.getAttribute("href"), "/collections/123");
    });
  });

  await t.test("switching tabs keeps everything in-page and syncs the URL", async () => {
    installProfileDom("/user/7");
    installProfileApiMocks({
      collections: [collectionSummary({ id: 2, title: "Public shelf", is_public: true })],
      contents: [{ id: 9, title: "Piano score", zone: "original" }],
    });

    const view = renderProfileClient(7, "Ada", "contents");

    await waitFor(() => {
      assert.ok(view.getByText("Piano score"));
    });

    const collectionsTab = view.getByRole("tab", { name: "Collections" });
    await act(async () => {
      collectionsTab.click();
    });
    await waitFor(() => {
      assert.ok(view.getByText("Public shelf"), "collections grid must render in-page after tab switch");
    });
    assert.equal(
      window.location.search,
      "?tab=collections",
      "tab switch must sync ?tab= for shareable deep links",
    );

    const discussionsTab = view.getByRole("tab", { name: "Discussions" });
    await act(async () => {
      discussionsTab.click();
    });
    await waitFor(() => {
      assert.equal(window.location.search, "?tab=discussions");
    });
  });
});

test("profile summary card", async (t) => {
  await t.test("renders real avatar plus the three hovercard-synced stats", async () => {
    installDom();

    let view: ReturnType<NonNullable<typeof rtl>["render"]> | undefined;
    await act(async () => {
      view = requireRtl().render(
        <IntlProvider locale="en" messages={enMessages}>
          <ProfileSummaryCard
            displayName="Ada"
            avatarUrl="https://example.com/a.png"
            bio="creator bio"
            meta={<span>Reputation: 60</span>}
            stats={{ contents: 12, likes: 34, followers: 7 }}
          />
        </IntlProvider>,
      );
    });
    assert.ok(view);
    const img = view.container.querySelector("img");
    assert.ok(img, "real avatar must render when avatar_url exists");
    assert.equal(img?.getAttribute("src"), "https://example.com/a.png");
    assert.ok(view.getByText("12"));
    assert.ok(view.getByText("Contents"));
    assert.ok(view.getByText("34"));
    assert.ok(view.getByText("Likes"));
    assert.ok(view.getByText("7"));
    assert.ok(view.getByText("Followers"));
  });

  await t.test("falls back to the initial letter without avatar_url", async () => {
    installDom();

    let view: ReturnType<NonNullable<typeof rtl>["render"]> | undefined;
    await act(async () => {
      view = requireRtl().render(
        <IntlProvider locale="en" messages={enMessages}>
          <ProfileSummaryCard displayName="Ada" bio="" meta={<span>meta</span>} stats={{ contents: 0, likes: 0, followers: 0 }} />
        </IntlProvider>,
      );
    });
    assert.ok(view);
    assert.equal(view.container.querySelector("img"), null);
    assert.ok(view.getByText("A"), "initial-letter avatar fallback");
  });
});

test("normalizeProfileTab maps URL tab values", () => {
  assert.equal(normalizeProfileTab(undefined), "contents");
  assert.equal(normalizeProfileTab("contents"), "contents");
  assert.equal(normalizeProfileTab("discussions"), "discussions");
  assert.equal(normalizeProfileTab("collections"), "collections");
  assert.equal(normalizeProfileTab("nonsense"), "contents");
});

function requireRtl() {
  assert.ok(rtl, "rtl must be loaded in beforeEach");
  return rtl;
}

function renderProfileClient(userId: number, displayName: string, initialTab: "contents" | "discussions" | "collections") {
  let view: ReturnType<NonNullable<typeof rtl>["render"]> | undefined;
  void act(() => {
    view = requireRtl().render(
      <IntlProvider locale="en" messages={enMessages}>
        <AppRouterContext.Provider value={testRouter}>
          <ToastProvider>
            <AuthProvider>
              <UserProfileClient userId={userId} displayName={displayName} initialTab={initialTab} />
            </AuthProvider>
          </ToastProvider>
        </AppRouterContext.Provider>
      </IntlProvider>,
    );
  });
  assert.ok(view);
  return view;
}

function installProfileDom(path: string) {
  const dom = installDom();
  dom.window.history.replaceState({}, "", path);
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: dom.window.localStorage,
  });
  return dom;
}

function installProfileApiMocks(options: {
  authUser?: { id: number; username: string };
  collections?: unknown[];
  contents?: unknown[];
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
    if (path.startsWith("/api/v1/users/") && path.includes("/contents?")) {
      return { contents: options.contents ?? [] } as T;
    }
    if (path.startsWith("/api/v1/users/") && path.includes("/discussions?")) {
      return { discussions: [] } as T;
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
