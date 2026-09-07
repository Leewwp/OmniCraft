import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { SWRConfig } from "swr";
import { createRequire } from "node:module";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* #410 F2 三瀑布流无限滚动：共享 hook 行为（追加/判尽/重试/无级联）+
   MasonryGrid 哨兵几何回归（哨兵在容器外）+ 二创首页与原创页筛选重置。 */

test.afterEach(() => {
  cleanup();
  restoreModuleStub();
});

const messages = {
  ...enMessages,
  common: { ...enMessages.common, endReached: "You've reached the end", retry: "Retry" },
} as const;

function cardData(id: number, zone: "original" | "fanwork" = "fanwork", title?: string) {
  return {
    id,
    title: title ?? `Feed item ${id}`,
    zone,
    author: { id: 10, username: `author-${id}` },
    like_count: 3,
    ...(zone === "fanwork" ? { content_type: "image", ip: { name: "Indigo IP" }, comment_count: 1, tags: ["art"] } : {}),
  };
}

function makePage(ids: number[], total: number) {
  return { contents: ids.map((id) => cardData(id)), total };
}

/** 带页语义的 fetch 桩：按 URL page 参数回对应页，记录调用。 */
function createPagedFetchStub(pages: Array<{ contents: unknown[]; total: number } | { error: true }>) {
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    calls.push(url);
    const pageParam = Number(new URL(url, "http://api.test").searchParams.get("page") ?? "1");
    const page = pages[Math.min(pageParam - 1, pages.length - 1)];
    if (!page || "error" in page) {
      return { ok: false, status: 500, json: async () => ({}) } as Response;
    }
    return { ok: true, status: 200, json: async () => page } as Response;
  }) as typeof fetch;
  return { calls, restore: () => { globalThis.fetch = originalFetch; } };
}

let activeCallbacks: Set<IntersectionObserverCallback> = new Set();
function installFakeIntersectionObserver() {
  activeCallbacks = new Set();
  class FakeIntersectionObserver {
    private cb: IntersectionObserverCallback;
    constructor(cb: IntersectionObserverCallback) {
      this.cb = cb;
      activeCallbacks.add(cb);
    }
    observe() {}
    unobserve() {}
    /* disconnect 语义必须真实：组件在 isLoadingMore 翻转 / hasMore 变 false
       时会 disconnect 观察器，已被断开的观察器不得再派发回调（级联防护的
       一半在组件、一半在这条语义）。 */
    disconnect() {
      activeCallbacks.delete(this.cb);
    }
    root = null;
    rootMargin = "";
    thresholds = [];
    takeRecords = () => [];
  }
  (globalThis as Record<string, unknown>).IntersectionObserver = FakeIntersectionObserver;
}
const triggerIntersect = (intersecting = true) => {
  for (const cb of [...activeCallbacks]) {
    cb(
      [{ isIntersecting: intersecting } as IntersectionObserverEntry],
      null as unknown as IntersectionObserver,
    );
  }
};

function renderFeed(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={messages}>
      <SWRConfig value={{ provider: () => new Map() }}>{node}</SWRConfig>
    </IntlProvider>,
  );
}

/* ---------- MasonryGrid 哨兵几何回归 ---------- */

test("#410 sentinel renders outside the measured masonry container", async () => {
  installDom();
  const { MasonryGrid } = await import("@/components/content/MasonryGrid");
  const view = renderFeed(
    <MasonryGrid
      items={[cardData(1), cardData(2), cardData(3)]}
      hasMore
      onLoadMore={() => {}}
    />,
  );
  const sentinel = view.container.querySelector('[data-slot="load-more-sentinel"]');
  assert.ok(sentinel, "sentinel must exist with a stable data-slot");
  /* 哨兵不得在卡片容器内：布局完成后容器为固定高度 + relative、卡片绝对
     定位，常规流哨兵会停在容器顶部造成级联追加分页。外置后哨兵块是容器
     的下一个兄弟节点。 */
  const block = sentinel.parentElement;
  const grid = block?.previousElementSibling ?? null;
  assert.ok(grid, "sentinel block must follow the cards container");
  assert.equal(grid.querySelectorAll('[data-slot="card-cover"]').length, 3);
  assert.ok(!grid.contains(sentinel), "sentinel must live outside the cards container");
});

/* ---------- 共享 hook 行为（经推荐页客户端全链） ---------- */

test("#410 feed appends one page per sentinel hit and stops at total exhaustion", async () => {
  installDom();
  installFakeIntersectionObserver();
  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const firstPage = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map((id) => cardData(id));
  const stub = createPagedFetchStub([makePage([13], 13)]);
  try {
    const view = renderFeed(
      <RecommendFeedClient
        apiBase="http://api.test/api/v1"
        initialItems={firstPage}
        initialTotal={13}
        initialError={false}
      />,
    );
    /* SSR 首屏不触发任何 fetch。 */
    await new Promise((r) => setTimeout(r, 50));
    assert.equal(stub.calls.length, 0, "first page must come from SSR fallbackData");

    triggerIntersect();
    await waitFor(() => assert.equal(stub.calls.length, 1));
    assert.match(stub.calls[0], /page=2/);
    assert.match(stub.calls[0], /page_size=12/);
    await waitFor(() => assert.ok(view.getByRole("button", { name: "Feed item 13" })));

    /* 判尽终态 + 再次命中哨兵不再发请求。 */
    await waitFor(() => assert.ok(view.getByText("You've reached the end")));
    triggerIntersect();
    await new Promise((r) => setTimeout(r, 120));
    assert.equal(stub.calls.length, 1, "exhausted feed must not fetch again");
  } finally {
    stub.restore();
  }
});

test("#410 feed shows retry on append failure and recovers", async () => {
  installDom();
  installFakeIntersectionObserver();
  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const firstPage = [1, 2].map((id) => cardData(id));
  let failFirst = true;
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    calls.push(url);
    if (new URL(url, "http://api.test").searchParams.get("page") === "2" && failFirst) {
      failFirst = false;
      return { ok: false, status: 500, json: async () => ({}) } as Response;
    }
    return { ok: true, status: 200, json: async () => makePage([3], 3) } as Response;
  }) as typeof fetch;
  try {
    const view = renderFeed(
      <RecommendFeedClient
        apiBase="http://api.test/api/v1"
        initialItems={firstPage}
        initialTotal={3}
        initialError={false}
      />,
    );
    triggerIntersect();
    await waitFor(() => assert.ok(view.getByRole("button", { name: "Retry" })));
    fireEvent.click(view.getByRole("button", { name: "Retry" }));
    await waitFor(() => assert.ok(view.getByRole("button", { name: "Feed item 3" })));
    await waitFor(() => assert.ok(view.getByText("You've reached the end")));
    assert.equal(calls.filter((c) => c.includes("page=2")).length, 2, "retry must refetch page 2");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("#410 sentinel hit during an in-flight append does not queue a second page", async () => {
  installDom();
  installFakeIntersectionObserver();
  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const firstPage = [1, 2].map((id) => cardData(id));
  const gate: { release: (() => void) | null } = { release: null };
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    calls.push(url);
    if (url.includes("page=2")) {
      await new Promise<void>((resolve) => { gate.release = resolve; });
    }
    return { ok: true, status: 200, json: async () => makePage([3], 3) } as Response;
  }) as typeof fetch;
  try {
    renderFeed(
      <RecommendFeedClient
        apiBase="http://api.test/api/v1"
        initialItems={firstPage}
        initialTotal={3}
        initialError={false}
      />,
    );
    triggerIntersect();
    await waitFor(() => assert.equal(calls.length, 1));
    /* 追加在途时哨兵再次进入视口（isLoadingMore=true，观察器未激活）——
       不得触发 page=3 请求。 */
    triggerIntersect();
    triggerIntersect();
    await new Promise((r) => setTimeout(r, 80));
    assert.equal(calls.length, 1, "in-flight append must absorb extra sentinel hits");
    gate.release?.();
    await waitFor(() => assert.equal(calls.length, 1));
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("#410 sentinel that stays intersecting after an append completes does not fetch the next page", async () => {
  installDom();
  installFakeIntersectionObserver();
  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const firstPage = [1, 2].map((id) => cardData(id));
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    calls.push(url);
    return { ok: true, status: 200, json: async () => makePage([3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14], 26) } as Response;
  }) as typeof fetch;
  try {
    renderFeed(
      <RecommendFeedClient
        apiBase="http://api.test/api/v1"
        initialItems={firstPage}
        initialTotal={26}
        initialError={false}
      />,
    );
    triggerIntersect(); // 进入视口 → 追加第 2 页
    await waitFor(() => assert.equal(calls.length, 1));
    /* 追加完成后观察器重建，浏览器会以当前状态派发初始回调——哨兵仍在
       视口内（滚动锚定跟随文档底部增长的场景），不得触发第 3 页。 */
    await waitFor(() => assert.ok(globalThis.document.querySelector('[aria-label="Feed item 14"]')));
    triggerIntersect();
    triggerIntersect();
    await new Promise((r) => setTimeout(r, 150));
    assert.equal(calls.length, 1, "still-intersecting sentinel must not retrigger");
    /* 滚离（退出视口）后再进入 → 允许下一次追加。 */
    triggerIntersect(false);
    triggerIntersect();
    await waitFor(() => assert.equal(calls.length, 2));
    assert.match(calls[1], /page=3/);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

/* ---------- 原创页：筛选重置回第 1 页 + 携带 page_size ---------- */

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
let navigationStubActive = false;
function installNavigationStub() {
  navigationStubActive = true;
  Module._load = function loadWithNavigationStub(request, parent, isMain) {
    if (request === "next/navigation" && navigationStubActive) {
      return { useParams: () => ({}), useRouter: () => ({ push: () => undefined, replace: () => undefined }) };
    }
    return originalModuleLoad(request, parent, isMain);
  };
}
function restoreModuleStub() {
  navigationStubActive = false;
  Module._load = originalModuleLoad;
}

test("#410 original feed resets to page 1 with page_size on category switch and keeps appending under the filter", async () => {
  installDom();
  installFakeIntersectionObserver();
  installNavigationStub();
  const { OriginalFeedClient } = await import("@/components/original/OriginalFeedClient");
  const initial = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map((id) => cardData(id, "original"));
  const stub = createPagedFetchStub([
    makePage([21, 22], 3), // film_tv page 1（total=3 → 追加一页后判尽）
    makePage([23], 3),     // film_tv page 2
  ]);
  try {
    const view = renderFeed(
      <OriginalFeedClient
        apiBase="http://api.test/api/v1"
        categories={[{ slug: "", i18n: "home.categoryRecommended" }, { slug: "film_tv", i18n: "home.categoryFilmTv" }]}
        initialContents={initial}
        initialTotal={12}
        initialCategory=""
        initialSort="recommended"
      />,
    );
    assert.ok(view.getByRole("button", { name: "Feed item 1" }));
    await new Promise((r) => setTimeout(r, 50));
    assert.equal(stub.calls.length, 0, "initial signature must reuse SSR first page");

    /* 切换类目 → 重置回第 1 页，请求带 zone/page/page_size/category。 */
    fireEvent.click(view.getByRole("button", { name: "Film & TV" }));
    await waitFor(() => assert.equal(stub.calls.length, 1));
    assert.match(stub.calls[0], /zone=original/);
    assert.match(stub.calls[0], /page=1(&|$)/);
    assert.match(stub.calls[0], /page_size=12/);
    assert.match(stub.calls[0], /category=film_tv/);
    await waitFor(() => assert.ok(view.getByRole("button", { name: "Feed item 21" })));
    assert.ok(!view.queryByRole("button", { name: "Feed item 1" }), "grid must reset to the filtered page 1");

    /* 筛选态下哨兵继续追加（page=2 同筛选参数）。 */
    triggerIntersect();
    await waitFor(() => assert.equal(stub.calls.length, 2));
    assert.match(stub.calls[1], /page=2/);
    assert.match(stub.calls[1], /category=film_tv/);
    await waitFor(() => assert.ok(view.getByRole("button", { name: "Feed item 23" })));
    await waitFor(() => assert.ok(view.getByText("You've reached the end")));
  } finally {
    stub.restore();
  }
});
