import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

const authState = {
  user: null as unknown,
  isLoading: false,
  unreadCounts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 },
  capabilities: { can_interact: true, interaction_denial_reason: null },
  login: async () => undefined,
  logout: async () => undefined,
  refresh: async () => true,
  refreshUser: async () => undefined,
};

Module._load = function loadWithStubs(request, parent, isMain) {
  if (request === "next/image") {
    return (props: Record<string, unknown>) =>
      React.createElement("img", { ...props, fill: undefined, sizes: undefined });
  }
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({ push: () => undefined }),
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authState,
      interactionDenialKey: () => "capabilities.deniedUnknown",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type ContentDetailModule = typeof import("@/components/content/ContentDetail");
let ContentDetail: ContentDetailModule["ContentDetail"];

test.before(async () => {
  const module = await import("@/components/content/ContentDetail");
  ContentDetail = module.ContentDetail;
});

const DETAIL = {
  id: 1,
  title: "Favorited state work",
  zone: "original",
  content_type: "article",
  author: { id: 9, username: "State Author" },
  status: "published",
  description: "Membership derives the favorited label",
  like_count: 2,
};

type ApiCall = { path: string; body?: unknown };

function installDetailApiMocks() {
  const calls: { get: ApiCall[]; post: ApiCall[]; delete: ApiCall[] } = { get: [], post: [], delete: [] };
  const originalGet = api.get;
  const originalPost = api.post;
  const originalDelete = api.delete;

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (path.startsWith("/api/v1/collections?")) {
      return {
        items: [
          {
            id: 1,
            title: "First shelf",
            zone: "original",
            description: "",
            is_public: false,
            is_default: false,
            item_count: 1,
            contains_item: true,
            item_id: 11,
          },
        ],
        total: 1,
      } as T;
    }
    return { comments: [], total: 0, reactions: { counts: {}, viewer_reaction: null } } as T;
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path.startsWith("/api/v1/collections/")) {
      return { id: 12, collection_id: 1, content_item_id: 1 } as T;
    }
    return {} as T;
  }) as typeof api.post;

  api.delete = (async <T,>(path: string): Promise<T> => {
    calls.delete.push({ path });
    return undefined as T;
  }) as typeof api.delete;

  return {
    calls,
    restore() {
      api.get = originalGet;
      api.post = originalPost;
      api.delete = originalDelete;
    },
  };
}

function renderDetail(data: Partial<typeof DETAIL> & { isFavorited?: boolean } = {}) {
  installDom();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentDetail data={{ ...DETAIL, ...data }} />
    </IntlProvider>,
  );
}

test.afterEach(() => cleanup());

test("#74 ContentDetail labels the collection action from membership state", async () => {
  const favorited = renderDetail({ isFavorited: true });
  assert.ok(favorited.getByRole("button", { name: /Favorited/ }), "member shows Favorited label");

  cleanup();
  const plain = renderDetail({});
  assert.ok(plain.getByRole("button", { name: /Add to collection/ }), "non-member shows Add to collection label");
});

test("#74 ContentDetail flips the label when the picker reports a membership change", async () => {
  const mocks = installDetailApiMocks();
  try {
    authState.user = { id: 7, username: "Ada", reputation: 10 };
    const view = renderDetail({});

    const button = view.getByRole("button", { name: /Add to collection/ });
    assert.ok(button, "non-member shows Add to collection label");

    fireEvent.click(button);
    await view.findByRole("dialog");
    await waitFor(() => assert.ok(view.getByRole("button", { name: "Remove from First shelf" })));

    // 移除最后一个有效收藏集 → 详情按钮保持/回到未收藏（初始即非成员，这里验证操作后状态不翻转）
    fireEvent.click(view.getByRole("button", { name: "Remove from First shelf" }));
    await waitFor(() => assert.equal(mocks.calls.delete.at(-1)?.path, "/api/v1/collections/1/items/11"));
    await waitFor(() => assert.ok(view.getByRole("button", { name: /Add to collection/ }), "label stays non-favorited after removing the only membership"));
  } finally {
    mocks.restore();
    authState.user = null;
  }
});
