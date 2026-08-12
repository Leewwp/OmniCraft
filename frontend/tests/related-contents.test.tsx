import test from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api } from "@/lib/api";
import { cleanup, installDom, render, waitFor, fireEvent } from "./runtime-test-helpers";

/* jsdom + real next/image can hang the node:test runner; stub the optimizer
   wrapper so ContentCard renders a plain <img> (same convention as
   source-linkage-components.test.tsx). */
const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

Module._load = function loadWithStubs(request, parent, isMain) {
  if (request === "next/image") {
    return (props: Record<string, unknown>) =>
      React.createElement("img", { ...props, fill: undefined, sizes: undefined });
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type RelatedContentsModule = typeof import("@/components/content/RelatedContents");

let RelatedContents: RelatedContentsModule["RelatedContents"];

const originalGet = api.get;

test.before(async () => {
  const module = await import("@/components/content/RelatedContents");
  RelatedContents = module.RelatedContents;
});

test.after(() => {
  api.get = originalGet;
});

test.afterEach(() => cleanup());

function renderWithEn(node: React.ReactNode) {
  return render(<IntlProvider locale="en" messages={enMessages}>{node}</IntlProvider>);
}

/** jsdom 无 matchMedia：按需注入可配置的假实现（isDesktop 判定 = min-width 1100px）。 */
function stubMatchMedia(matches: boolean) {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      matches,
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    }),
  });
}

function contentCard(id: number, overrides: Record<string, unknown> = {}) {
  return {
    id,
    title: `Item ${id}`,
    zone: "fanwork",
    content_type: "image",
    author: { id: 1, username: "Author" },
    like_count: 2,
    ...overrides,
  };
}

function relatedCard(id: number) {
  return contentCard(id, { title: `Related ${id}` });
}

const RELATED_ENDPOINT = "/api/v1/contents/9/related-fanworks?page=1&page_size=8";

interface MockGetHandler {
  related: unknown[];
  relatedTotal: number;
  similar: unknown[];
  similarTotal: number;
  similarError?: boolean;
  capturedSimilarPath?: string;
}

function installMockGet(handler: MockGetHandler) {
  api.get = (async (requestPath: string) => {
    if (requestPath === RELATED_ENDPOINT) {
      return { contents: handler.related, total: handler.relatedTotal };
    }
    if (requestPath.startsWith("/api/v1/contents?")) {
      handler.capturedSimilarPath = requestPath;
      if (handler.similarError) {
        throw new Error("network error");
      }
      return { contents: handler.similar, total: handler.similarTotal };
    }
    throw new Error(`unexpected api.get call: ${requestPath}`);
  }) as typeof api.get;
}

/* ------------------------------------------------------------------ */
/* 固定 list 合同（AC2）                                                */
/* ------------------------------------------------------------------ */

test("similar request uses the fixed list contract for fanwork with ip (zone/content_type/category/ip_id/sort=hot/page_size=12)", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = { related: [], relatedTotal: 0, similar: [], similarTotal: 0 };
  installMockGet(handler);
  renderWithEn(
    <RelatedContents contentId={9} zone="fanwork" contentType="image" category="art" ipId={3} />,
  );
  await waitFor(() => {
    assert.equal(handler.capturedSimilarPath,
      "/api/v1/contents?zone=fanwork&content_type=image&category=art&ip_id=3&sort=hot&page_size=12");
  });
});

test("similar request omits ip_id for originals and omits empty category", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = { related: [], relatedTotal: 0, similar: [], similarTotal: 0 };
  installMockGet(handler);
  renderWithEn(
    <RelatedContents contentId={9} zone="original" contentType="article" category="literature" />,
  );
  await waitFor(() => {
    assert.equal(handler.capturedSimilarPath,
      "/api/v1/contents?zone=original&content_type=article&category=literature&sort=hot&page_size=12");
  });
});

test("similar request drops the category param when content has no category", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = { related: [], relatedTotal: 0, similar: [], similarTotal: 0 };
  installMockGet(handler);
  renderWithEn(<RelatedContents contentId={9} zone="original" contentType="article" />);
  await waitFor(() => {
    assert.equal(handler.capturedSimilarPath,
      "/api/v1/contents?zone=original&content_type=article&sort=hot&page_size=12");
  });
});

test("similar request never uses sort=recommended when filters are present", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = { related: [], relatedTotal: 0, similar: [], similarTotal: 0 };
  installMockGet(handler);
  renderWithEn(<RelatedContents contentId={9} zone="original" contentType="image" />);
  await waitFor(() => {
    assert.ok(handler.capturedSimilarPath);
    assert.match(handler.capturedSimilarPath, /sort=hot/);
    assert.doesNotMatch(handler.capturedSimilarPath, /sort=recommended/);
  });
});

/* ------------------------------------------------------------------ */
/* 双行呈现 + 去重（AC1）                                               */
/* ------------------------------------------------------------------ */

test("block renders related row and similar row with dedupe: current content and related ids excluded, at most 8 cards", async () => {
  installDom();
  stubMatchMedia(true);
  const similarList = [
    contentCard(9), // 当前内容自身 → 去重
    relatedCard(61), // 关联行已有 → 去重
    ...Array.from({ length: 10 }, (_, i) => contentCard(100 + i)), // 10 条其余
  ];
  const handler: MockGetHandler = {
    related: [relatedCard(61), relatedCard(62)],
    relatedTotal: 2,
    similar: similarList,
    similarTotal: similarList.length,
  };
  installMockGet(handler);
  const { container } = renderWithEn(
    <RelatedContents
      contentId={9}
      zone="fanwork"
      contentType="image"
      category="art"
      ipId={3}
      relatedFanworks={[{ id: 61, title: "Related 61", zone: "fanwork" }]}
      relatedFanworksSlot={{
        sourceContentId: 9,
        sourceZone: "fanwork",
        titleKey: "media.related.relatedTitle",
      }}
    />,
  );

  await waitFor(() => {
    assert.ok(container.querySelector('[data-slot="related-contents"]'));
  });

  const block = container.querySelector('[data-slot="related-contents"]');
  assert.ok(block);
  assert.match(block.textContent ?? "", /Related creations/); // 关联行标题
  assert.match(block.textContent ?? "", /You might also like/); // 相似行标题

  // 关联行 2 张（61/62），相似行去重后 10 条 → 上限 8 张
  const relatedCards = block.querySelectorAll('[data-slot="related-fanworks"] [data-slot="card-cover"]');
  assert.equal(relatedCards.length, 2);
  const similarCards = block.querySelectorAll('[data-slot="related-contents-similar"] [data-slot="card-cover"]');
  assert.equal(similarCards.length, 8);

  // 当前内容与关联行 id 不得出现在相似行中
  const similarTitles = Array.from(similarCards, (card) => card.getAttribute("alt") ?? "");
  assert.ok(!similarTitles.some((title) => title === "Item 9" || title === "Related 61"));

  // 到底提示
  assert.match(block.textContent ?? "", /You've reached the end/);
});

test("embedded related row shares the single block container (no nested bordered section)", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = {
    related: [relatedCard(61)],
    relatedTotal: 1,
    similar: [contentCard(71)],
    similarTotal: 1,
  };
  installMockGet(handler);
  const { container } = renderWithEn(
    <RelatedContents
      contentId={9}
      zone="original"
      contentType="image"
      relatedFanworksSlot={{
        sourceContentId: 9,
        sourceZone: "original",
        titleKey: "media.related.relatedTitle",
      }}
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector('[data-slot="related-contents"]'));
  });
  const block = container.querySelector('[data-slot="related-contents"]');
  const relatedRow = block?.querySelector('[data-slot="related-fanworks"]');
  assert.ok(relatedRow);
  assert.match(block?.querySelector('[data-slot="related-contents-box"]')?.className ?? "", /rounded-lg/);
  assert.doesNotMatch(relatedRow.className ?? "", /border/);
});

/* ------------------------------------------------------------------ */
/* 空分支降级（AC1）                                                    */
/* ------------------------------------------------------------------ */

test("empty branch: both rows empty renders only the end hint, no block titles", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = { related: [], relatedTotal: 0, similar: [], similarTotal: 0 };
  installMockGet(handler);
  const { container } = renderWithEn(
    <RelatedContents
      contentId={9}
      zone="original"
      contentType="article"
      relatedFanworksSlot={{
        sourceContentId: 9,
        sourceZone: "original",
        titleKey: "media.related.relatedTitle",
      }}
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector('[data-slot="related-contents-end"]'));
  });
  assert.equal(container.querySelector('[data-slot="related-contents"]'), null);
  const text = container.textContent ?? "";
  assert.match(text, /You've reached the end/);
  assert.doesNotMatch(text, /Related creations/);
  assert.doesNotMatch(text, /You might also like/);
});

/* ------------------------------------------------------------------ */
/* 错误 + 重试                                                          */
/* ------------------------------------------------------------------ */

test("similar error renders inline error and retry refetches", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = {
    related: [],
    relatedTotal: 0,
    similar: [],
    similarTotal: 0,
    similarError: true,
  };
  installMockGet(handler);
  const { container } = renderWithEn(
    <RelatedContents contentId={9} zone="original" contentType="article" />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector('[data-slot="related-contents-similar-error"]'));
  });
  assert.match(container.textContent ?? "", /Failed to load similar content/);

  handler.similarError = false;
  handler.similar = [contentCard(71)];
  handler.similarTotal = 1;
  const retryButton = container.querySelector('[data-slot="related-contents-similar-error"] button');
  assert.ok(retryButton);
  fireEvent.click(retryButton);
  await waitFor(() => {
    assert.ok(container.querySelector('[data-slot="related-contents-similar"]'));
  });
});

/* ------------------------------------------------------------------ */
/* 移动端不渲染（Key Constraints）                                      */
/* ------------------------------------------------------------------ */

test("mobile viewport renders nothing and issues no API requests", async () => {
  installDom();
  stubMatchMedia(false);
  let calls = 0;
  api.get = (async () => {
    calls += 1;
    return { contents: [] };
  }) as typeof api.get;
  const { container } = renderWithEn(
    <RelatedContents contentId={9} zone="original" contentType="article" />,
  );
  assert.equal(container.textContent, "");
  await new Promise((resolve) => setTimeout(resolve, 20));
  assert.equal(calls, 0);
});

/* ------------------------------------------------------------------ */
/* 卡片打开（AC3 栈接入的回调面）                                        */
/* ------------------------------------------------------------------ */

test("cards invoke onOpenDetail with card data and trigger", async () => {
  installDom();
  stubMatchMedia(true);
  const handler: MockGetHandler = {
    related: [],
    relatedTotal: 0,
    similar: [contentCard(71)],
    similarTotal: 1,
  };
  installMockGet(handler);
  let opened: { id: number; zone?: string } | null = null;
  const { container } = renderWithEn(
    <RelatedContents
      contentId={9}
      zone="original"
      contentType="image"
      onOpenDetail={(data) => {
        opened = { id: data.id, zone: data.zone };
      }}
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector('[data-slot="related-contents-similar"]'));
  });
  const button = container.querySelector('[data-slot="related-contents-similar"] button');
  assert.ok(button);
  fireEvent.click(button);
  assert.deepEqual(opened, { id: 71, zone: "fanwork" });
});
