import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import React from "react";
import { IntlProvider } from "use-intl";
import { SWRConfig } from "swr";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return readFile(path.join(root, relativePath), "utf8");
}

test.afterEach(() => {
  cleanup();
});

const feedMessages = {
  ...enMessages,
  recommend: {
    title: "Recommended for you",
    subtitle: "A blended feed of originals and fanworks.",
    emptyTitle: "No recommendations yet",
    emptyDescription: "You have seen everything here. Come back later.",
    emptyAction: "Browse originals",
    errorTitle: "Recommendations are temporarily unavailable",
    errorDescription: "The request failed. Please try reloading.",
    retryAction: "Reload",
    loadingLabel: "Loading recommendations",
  },
  common: {
    ...enMessages.common,
    endReached: "You've reached the end",
  },
} as const;

function createFetchStub(pages: { contents: unknown[]; total: number }[]) {
  const calls: string[] = [];
  let index = 0;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    calls.push(String(input));
    const page = pages[Math.min(index, pages.length - 1)];
    index += 1;
    return {
      ok: true,
      status: 200,
      json: async () => page,
    } as Response;
  }) as typeof fetch;
  return {
    calls,
    restore: () => {
      globalThis.fetch = originalFetch;
    },
  };
}

function cardData(id: number, zone: "original" | "fanwork" = "original") {
  return {
    id,
    title: `Feed item ${id}`,
    zone,
    author: { id: 10, username: `author-${id}` },
    like_count: 3,
    ...(zone === "fanwork" ? { content_type: "image", ip: { name: "Indigo IP" }, comment_count: 1, tags: ["art"] } : {}),
  };
}

function renderWithIntl(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={feedMessages}>
      {node}
    </IntlProvider>,
  );
}

function renderFeed(node: React.ReactNode) {
  return renderWithIntl(
    <SWRConfig value={{ provider: () => new Map() }}>{node}</SWRConfig>,
  );
}

/* ---------- 最短列布局算法（lib/masonry-layout.ts） ---------- */

test("shortest-column layout preserves item order and fills the shortest column", async () => {
  installDom();
  const { computeShortestColumnLayout } = await import("@/lib/masonry-layout");

  const heights = [100, 50, 80, 60];
  const gap = 16;
  const layout = computeShortestColumnLayout(heights, 2, gap, 100);

  // item0 -> col0(top0,left0); item1 -> col1(top0,left116); item2 -> col1(top66); item3 -> col0(top116)
  assert.deepEqual(
    layout.positions.map((p) => [p.left, p.top]),
    [
      [0, 0],
      [116, 0],
      [116, 66],
      [0, 116],
    ],
  );
  // container height = max column height: col0 = 100 + 16 + 60 = 176
  assert.equal(layout.height, 176);
});

test("shortest-column layout distributes into 4 columns on desktop", async () => {
  installDom();
  const { computeShortestColumnLayout } = await import("@/lib/masonry-layout");
  const layout = computeShortestColumnLayout([10, 20, 30, 40], 4, 16, 50);
  assert.equal(layout.positions.length, 4);
  assert.deepEqual(layout.positions.map((p) => p.left), [0, 66, 132, 198]);
  assert.deepEqual(layout.positions.map((p) => p.top), [0, 0, 0, 0]);
});

/* ---------- MasonryGrid ---------- */

test("MasonryGrid renders items in DOM order and shows end-of-feed marker", async () => {
  installDom();
  const { MasonryGrid } = await import("@/components/content/MasonryGrid");
  const view = renderWithIntl(
    <MasonryGrid
      items={[cardData(1), cardData(2, "fanwork"), cardData(3)]}
      hasMore={false}
      onLoadMore={() => {}}
    />,
  );

  const buttons = view.container.querySelectorAll("a, button");
  const titles = Array.from(buttons).map((el) => el.getAttribute("aria-label") || el.textContent);
  assert.deepEqual(titles, ["Feed item 1", "Feed item 2", "Feed item 3"]);
  assert.ok(view.getByText("You've reached the end"));
});

test("MasonryGrid sentinel triggers onLoadMore only while hasMore", async () => {
  installDom();
  const { MasonryGrid } = await import("@/components/content/MasonryGrid");

  let observed = 0;
  let callback: IntersectionObserverCallback | null = null;
  class FakeIntersectionObserver {
    constructor(cb: IntersectionObserverCallback) {
      callback = cb;
      observed += 1;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
    root = null;
    rootMargin = "";
    thresholds = [];
    takeRecords = () => [];
  }
  (globalThis as Record<string, unknown>).IntersectionObserver = FakeIntersectionObserver;

  let loadCalls = 0;
  renderWithIntl(
    <MasonryGrid items={[cardData(1)]} hasMore onLoadMore={() => (loadCalls += 1)} />,
  );
  assert.equal(observed, 1);
  const triggerIntersect = () =>
    callback?.([{ isIntersecting: true } as IntersectionObserverEntry], null as unknown as IntersectionObserver);
  triggerIntersect();
  await waitFor(() => assert.equal(loadCalls, 1));

  cleanup();
  observed = 0;
  callback = null;
  renderWithIntl(
    <MasonryGrid items={[cardData(1)]} hasMore={false} onLoadMore={() => (loadCalls += 1)} />,
  );
  assert.equal(observed, 0, "sentinel must not observe when hasMore is false");
});

test("MasonryGrid shows empty state when no items and emptyText given", async () => {
  installDom();
  const { MasonryGrid } = await import("@/components/content/MasonryGrid");
  const view = renderWithIntl(<MasonryGrid items={[]} emptyText="No content yet" />);
  assert.ok(view.getByText("No content yet"));
});

/* ---------- ContentCard 浮窗模式（作者入口分离） ---------- */

test("ContentCard default mode keeps whole-card detail link", async () => {
  installDom();
  const { ContentCard } = await import("@/components/content/ContentCard");
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentCard data={cardData(7, "fanwork")} />
      <ContentCard data={cardData(8, "original")} />
    </IntlProvider>,
  );
  assert.ok(view.getByRole("link", { name: "Feed item 7" }));
  assert.ok(view.getByRole("link", { name: "Feed item 8" }));
  const hrefs = view.getAllByRole("link").map((l) => l.getAttribute("href"));
  assert.ok(hrefs.includes("/content/7"));
  assert.ok(hrefs.includes("/original/8"));
});

test("ContentCard overlay mode splits main click area from author entry", async () => {
  installDom();
  const { ContentCard } = await import("@/components/content/ContentCard");
  const opened: Array<{ id: number }> = [];
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentCard
        data={cardData(7, "fanwork")}
        onOpenDetail={(data, trigger) => {
          opened.push({ id: data.id });
          assert.ok(trigger instanceof HTMLElement);
        }}
      />
    </IntlProvider>,
  );

  // 主点击区是按钮而非链接，点击触发浮窗回调
  const main = view.getByRole("button", { name: "Feed item 7" });
  assert.equal(main.tagName, "BUTTON");
  assert.equal(main.getAttribute("href"), null);
  fireEvent.click(main);
  assert.deepEqual(opened, [{ id: 7 }]);

  // 作者身份入口是独立链接，指向用户主页
  const author = view.getByRole("link", { name: /author-7/ });
  assert.equal(author.getAttribute("href"), "/user/10");
  assert.ok(!author.closest("button"), "author entry must not live inside the main click area");
});

/* ---------- RecommendFeedClient 页面状态 ---------- */

test("RecommendFeedClient renders feed cards without zone tabs", async () => {
  installDom();
  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const view = renderFeed(
    <RecommendFeedClient apiBase="http://api.test/api/v1" initialItems={[cardData(1), cardData(2, "fanwork")]} initialTotal={2} initialError={false} />,
  );
  assert.equal(view.queryByRole("tablist"), null, "recommend stream must not render zone tabs");
  assert.equal(view.queryByRole("tab"), null);
  assert.ok(view.getByRole("button", { name: "Feed item 1" }));
  assert.ok(view.getByRole("button", { name: "Feed item 2" }));
});

test("RecommendFeedClient empty state offers browse-originals CTA", async () => {
  installDom();
  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const view = renderFeed(
    <RecommendFeedClient apiBase="http://api.test/api/v1" initialItems={[]} initialTotal={0} initialError={false} />,
  );
  assert.ok(view.getByText("No recommendations yet"));
  const cta = view.getByRole("link", { name: "Browse originals" });
  assert.equal(cta.getAttribute("href"), "/original");
});

test("RecommendFeedClient error state keeps position and retries page 1", async () => {
  installDom();
  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const stub = createFetchStub([{ contents: [cardData(1), cardData(2, "fanwork")], total: 2 }]);
  try {
    const view = renderFeed(
      <RecommendFeedClient apiBase="http://api.test/api/v1" initialItems={[]} initialTotal={null} initialError />,
    );
    assert.ok(view.getByText("Recommendations are temporarily unavailable"));
    assert.ok(view.getByRole("button", { name: "Reload" }));

    fireEvent.click(view.getByRole("button", { name: "Reload" }));
    await waitFor(() => assert.ok(view.getByRole("button", { name: "Feed item 1" })));
    assert.equal(stub.calls.length, 1);
    assert.match(stub.calls[0], /sort=recommended/);
    assert.match(stub.calls[0], /page=1/);
  } finally {
    stub.restore();
  }
});

test("RecommendFeedClient appends page 2 via sentinel with sort=recommended", async () => {
  installDom();
  let callback: IntersectionObserverCallback | null = null;
  class FakeIntersectionObserver {
    constructor(cb: IntersectionObserverCallback) {
      callback = cb;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
    root = null;
    rootMargin = "";
    thresholds = [];
    takeRecords = () => [];
  }
  (globalThis as Record<string, unknown>).IntersectionObserver = FakeIntersectionObserver;

  const { RecommendFeedClient } = await import("@/components/recommend/RecommendFeedClient");
  const stub = createFetchStub([
    { contents: [cardData(1), cardData(2, "fanwork")], total: 3 },
    { contents: [cardData(3)], total: 3 },
  ]);
  try {
    renderFeed(
      <RecommendFeedClient apiBase="http://api.test/api/v1" initialItems={[cardData(1), cardData(2, "fanwork")]} initialTotal={3} initialError={false} />,
    );
    const triggerIntersect = () =>
      callback?.([{ isIntersecting: true } as IntersectionObserverEntry], null as unknown as IntersectionObserver);
    triggerIntersect();
    await waitFor(() => assert.equal(stub.calls.length, 1));
    assert.match(stub.calls[0], /sort=recommended/);
    assert.match(stub.calls[0], /page=2/);
  } finally {
    stub.restore();
  }
});

/* ---------- Header 入口 + 页面源码契约 ---------- */

test("Header adds recommend entry to desktop nav and mobile menu", async () => {
  installDom();
  const header = await read("components/layout/Header.tsx");
  assert.match(header, /t\("nav\.recommend"\)/);
  // 桌面导航：推荐 / 二创 / 原创
  assert.ok(header.indexOf('t("nav.recommend")') < header.indexOf('t("nav.fanworkZone")'));
  assert.ok(header.indexOf('href="/recommend"') < header.indexOf('href="/original"'));
  // 移动菜单同样有入口
  assert.match(header, /href="\/recommend"/);
  // 选中态
  assert.match(header, /pathname\.startsWith\("\/recommend"\)/);
});

test("recommend page shell follows approved contract", async () => {
  installDom();
  const page = await read("app/(public)/recommend/page.tsx");
  assert.match(page, /sort=recommended/);
  assert.match(page, /page_size=24/);
  assert.match(page, /getServerApiBase\(\)/);
  assert.match(page, /getBrowserApiBase\(\)/);
  assert.match(page, /normalizeContentList/);
  assert.match(page, /bg-canvas-subtle/);
  assert.match(page, /max-w-\[1280px\]/);
  assert.doesNotMatch(page, /category=/, "recommend page must not carry zone/category params");
});

test("MasonryGrid source avoids column-major fill and CSS columns", async () => {
  installDom();
  const source = await read("components/content/MasonryGrid.tsx");
  assert.doesNotMatch(source, /react-masonry-css/);
  assert.doesNotMatch(source, /columns-2|columns-3|columns-4/);
  assert.doesNotMatch(source, /i % columnCount/);
});
