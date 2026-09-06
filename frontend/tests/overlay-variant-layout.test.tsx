import test from "node:test";
import assert from "node:assert/strict";
import React, { useRef, useState } from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * #397 R2 竖屏集新版布局（variant）测试：
 * - lib 纯函数：朝向判定 16:9 边界 / 超高图 / 媒体源链优先级；
 * - 组件层：portrait 集（≥1100px 视口）渲染 variant 版式（贴边媒体列 + 右栏
 *   layer-scroller + 壳层去 header 悬浮返回/关闭 + sr-only 标题三职）；
 * - 全横集 / 非 desktop 视口不进 variant；
 * - 关联内容块（布局钉死：原创 → 同系列 → 衍生二创；无关联不渲染）；
 * - 点击媒体（pane 中心）打开 MediaViewer（H4 契约：原生 dialog 叠加在浮层上）。
 * jsdom 不加载图片资源：夹具全部自带尺寸走免探测路径；matchMedia stub 决定
 * desktop 判定（与 content-detail-split-layout.test.tsx 同一模式）。
 * ──────────────────────────────────────────────────────────────────────────── */

function installOverlayTestStubs({ splitViewport = false }: { splitViewport?: boolean } = {}) {
  const prototype = window.HTMLDialogElement?.prototype as unknown as HTMLDialogElement | undefined;
  if (!prototype) return;
  prototype.showModal = function showModalStub(this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  prototype.close = function closeStub(this: HTMLDialogElement) {
    this.removeAttribute("open");
  };
  window.scrollTo = () => undefined;
  /* matchMedia 复原：jsdom 原生无实现（undefined）；上一个用例的 desktop stub
     必须还原，否则后续「非 desktop」用例被污染走 variant 路径。 */
  if (splitViewport) {
    window.matchMedia = ((query: string) => ({
      matches: query === "(min-width: 1100px)",
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    })) as typeof window.matchMedia;
  } else {
    delete (window as { matchMedia?: typeof window.matchMedia }).matchMedia;
  }
}

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const authStub = {
  user: null,
  isLoading: false,
  unreadCounts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 },
  capabilities: { can_interact: false, interaction_denial_reason: "unavailable" },
  login: async () => undefined,
  logout: async () => undefined,
  refresh: async () => true,
  refreshUser: async () => undefined,
};
Module._load = function loadWithNavigationStub(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({ push: () => undefined }),
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
      interactionDenialKey: () => "capabilities.deniedUnknown",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

/* ── lib 纯函数 ─────────────────────────────────────────────────────────── */

test("isPortraitMediaSet applies the 16:9 boundary (exact 16:9 is landscape)", async () => {
  const { isPortraitMediaSet } = await import("@/lib/overlay-media");
  const item = (w: number, h: number) => ({ id: w * 100 + h, url: "data:image/svg+xml,x", type: "image" as const, width: w, height: h });
  assert.equal(isPortraitMediaSet([item(1600, 900)]), false, "exact 16:9 → landscape → keep current design");
  assert.equal(isPortraitMediaSet([item(1601, 900)]), false, ">16:9 → landscape");
  assert.equal(isPortraitMediaSet([item(1599, 900)]), true, "<16:9 → portrait");
  assert.equal(isPortraitMediaSet([item(900, 1200)]), true, "3:4 → portrait");
  assert.equal(isPortraitMediaSet([item(1100, 1100)]), true, "square → portrait");
  assert.equal(isPortraitMediaSet([item(1600, 900), item(900, 1200)]), true, "mixed set → variant");
  assert.equal(isPortraitMediaSet([{ id: 1, url: "u", type: "image" }]), false, "no dims → defensive keep current design");
});

test("isUltraTallItem and itemAspectRatio defaults", async () => {
  const { isUltraTallItem, itemAspectRatio } = await import("@/lib/overlay-media");
  const item = (w: number, h: number) => ({ id: 1, url: "u", type: "image" as const, width: w, height: h });
  assert.equal(isUltraTallItem(item(600, 1400)), true);
  assert.equal(isUltraTallItem(item(900, 1200)), false);
  assert.equal(isUltraTallItem({ id: 2, url: "u", type: "image" }), false);
  assert.equal(itemAspectRatio({ id: 3, url: "u", type: "image" }), 3 / 4);
  assert.equal(itemAspectRatio(item(1600, 900)), 1600 / 900);
});

test("buildOverlayMedia chain: real media set → content cover → text cover", async () => {
  const { buildOverlayMedia } = await import("@/lib/overlay-media");
  const mediaSet = buildOverlayMedia({
    content: { id: 1, title: "T", content_type: "image" },
    attachments: [{ id: 5, file_type: "image", oss_key: "/a.svg", width: 900, height: 1200 }],
    tags: [],
    series_memberships: [],
  });
  assert.equal(mediaSet.length, 1);
  assert.equal(mediaSet[0].url, "/a.svg");

  const coverOnly = buildOverlayMedia({
    content: { id: 2, title: "T", content_type: "article", cover_image_url: "/cover.svg", cover_width: 900, cover_height: 1200 },
    attachments: [],
    tags: [],
    series_memberships: [],
  });
  assert.equal(coverOnly.length, 1);
  assert.equal(coverOnly[0].url, "/cover.svg");
  assert.equal(coverOnly[0].width, 900);

  const textCover = buildOverlayMedia({
    content: { id: 3, title: "T", content_type: "mod" },
    attachments: [],
    tags: [],
    series_memberships: [],
  });
  assert.equal(textCover.length, 1);
  assert.ok(textCover[0].url.startsWith("data:image/svg+xml"), "falls back to the 3:4 text cover placeholder");
  assert.equal(textCover[0].width, 300);
  assert.equal(textCover[0].height, 400);
});

/* ── 组件层 ─────────────────────────────────────────────────────────────── */

type OverlayModule = typeof import("@/components/content/ContentDetailOverlay");
let ContentDetailOverlay: OverlayModule["ContentDetailOverlay"];

test.before(async () => {
  const overlayModule = await import("@/components/content/ContentDetailOverlay");
  await import("@/components/content/ContentDetailOverlayLayer");
  ContentDetailOverlay = overlayModule.ContentDetailOverlay;
});

/* 竖图集（3:4 ×2，多图有控件位）。 */
const PORTRAIT_DETAIL = {
  content: {
    id: 21,
    title: "Portrait Set Work",
    zone: "original",
    content_type: "image",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Portrait body",
    cover_image_url: "/seed-media/real/gallery/p01.svg",
    like_count: 3,
  },
  attachments: [
    { id: 31, content_item_id: 21, file_type: "image", oss_key: "/seed-media/real/gallery/p01.svg", width: 900, height: 1200, sort_order: 0 },
    { id: 32, content_item_id: 21, file_type: "image", oss_key: "/seed-media/real/gallery/p02.svg", width: 900, height: 1200, sort_order: 1 },
  ],
  tags: [],
};

/* 混合集（16:9 + 3:4）→ 任一竖图走新版布局。 */
const MIXED_DETAIL = {
  content: {
    id: 23,
    title: "Mixed Set Work",
    zone: "original",
    content_type: "image",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Mixed body",
  },
  attachments: [
    { id: 41, content_item_id: 23, file_type: "image", oss_key: "/seed-media/real/gallery/m01.svg", width: 1600, height: 900, sort_order: 0 },
    { id: 42, content_item_id: 23, file_type: "image", oss_key: "/seed-media/real/gallery/m02.svg", width: 900, height: 1200, sort_order: 1 },
  ],
  tags: [],
};

/* 全横集（≥16:9 ×2）→ 保留现设计（split-media）。 */
const LANDSCAPE_DETAIL = {
  content: {
    id: 24,
    title: "Landscape Set Work",
    zone: "original",
    content_type: "image",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Landscape body",
  },
  attachments: [
    { id: 51, content_item_id: 24, file_type: "image", oss_key: "/seed-media/real/gallery/l01.svg", width: 1600, height: 900, sort_order: 0 },
    { id: 52, content_item_id: 24, file_type: "image", oss_key: "/seed-media/real/gallery/l02.svg", width: 1280, height: 720, sort_order: 1 },
  ],
  tags: [],
};

/* fanwork + 关联原创 + 系列 + 衍生二创（关联内容块三行全出）。 */
const RELATED_DETAIL = {
  content: {
    id: 25,
    title: "Fanwork With Relations",
    zone: "fanwork",
    content_type: "image",
    author: { id: 12, username: "Fan Author" },
    status: "published",
    description: "Fanwork body",
    ip: { id: 3, name: "Indigo IP" },
  },
  attachments: [
    { id: 61, content_item_id: 25, file_type: "image", oss_key: "/seed-media/real/gallery/f01.svg", width: 900, height: 1200, sort_order: 0 },
  ],
  tags: [],
  series_memberships: [
    {
      series_id: 77,
      series_title: "Color Studies",
      current_index: 2,
      total: 5,
      previous: { id: 100, title: "Chapter One" },
      next: { id: 102, title: "Chapter Three" },
    },
  ],
  source_original: { id: 200, title: "The Original Work", zone: "original" },
};

const detailByPath = new Map<string, unknown>([
  ["/api/v1/contents/21", PORTRAIT_DETAIL],
  ["/api/v1/contents/23", MIXED_DETAIL],
  ["/api/v1/contents/24", LANDSCAPE_DETAIL],
  ["/api/v1/contents/25", RELATED_DETAIL],
]);

const originalGet = api.get;
const originalPost = api.post;

function installApiMock() {
  api.get = async function mockedGet<T>(requestPath: string): Promise<T> {
    if (requestPath.includes("/related-fanworks")) {
      const id = Number(requestPath.match(/\/contents\/(\d+)\//)?.[1] ?? 0);
      if (id === 25) {
        return {
          contents: [
            {
              id: 300,
              title: "Derivative One",
              zone: "fanwork",
              content_type: "image",
              author: { id: 11, username: "Related Author" },
              cover_image_url: "/seed-media/covers/related.svg",
              like_count: 2,
            },
          ],
          total: 1,
        } as T;
      }
      return { contents: [], total: 0 } as T;
    }
    if (requestPath.startsWith("/api/v1/social/comments")) {
      return { comments: [] } as T;
    }
    const contentIdMatch = requestPath.match(/^\/api\/v1\/contents\/(\d+)$/);
    if (contentIdMatch) {
      const found = detailByPath.get(`/api/v1/contents/${contentIdMatch[1]}`);
      if (found) return found as T;
    }
    throw new ApiRequestError("NOT_FOUND", "raw secret backend error", 404);
  };
  api.post = async function mockedPost<T>(): Promise<T> {
    throw new ApiRequestError("UNAUTHORIZED", "no session", 401);
  };
}

function restoreApiMocks() {
  api.get = originalGet;
  api.post = originalPost;
}

function OverlayHarness({ entryId, zone }: { entryId: number; zone: "original" | "fanwork" }) {
  const [entry, setEntry] = useState<{ id: number; zone: "original" | "fanwork" } | null>(null);
  const triggerRef = useRef<HTMLElement | null>(null);
  return (
    <>
      <button
        type="button"
        onClick={(event) => {
          triggerRef.current = event.currentTarget;
          setEntry({ id: entryId, zone });
        }}
      >
        Open overlay
      </button>
      {entry && (
        <ContentDetailOverlay
          key={`${entry.zone}:${entry.id}`}
          contentId={entry.id}
          zone={entry.zone}
          source="zone-page"
          open
          onOpenChange={(open) => {
            if (!open) setEntry(null);
          }}
          returnFocusRef={triggerRef}
        />
      )}
    </>
  );
}

function renderOverlay(node: React.ReactNode, splitViewport = false) {
  installDom();
  installOverlayTestStubs({ splitViewport });
  return render(<IntlProvider locale="en" messages={enMessages}>{node}</IntlProvider>);
}

async function openOverlay(view: ReturnType<typeof render>, expectedTitle?: string) {
  const trigger = view.getByRole("button", { name: "Open overlay" });
  await act(async () => {
    fireEvent.click(trigger);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  if (expectedTitle) {
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: expectedTitle })));
  }
  return trigger;
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

test("#397 portrait media set renders the variant layout on desktop viewport", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={21} zone="original" />, true);
  await openOverlay(view, "Portrait Set Work");

  const pane = document.querySelector('[data-slot="variant-media-pane"]');
  assert.ok(pane, "variant layout must render the flush media pane");
  const cover = pane?.querySelector('[data-slot="detail-cover"]');
  assert.ok(cover, "media pane carries the detail-cover motion anchor");

  /* 布局不变量：锚点盒不含翻页控件（箭头/角标/指示点是 pane 的子节点而非锚点子节点）。 */
  const controlsInsideAnchor = cover?.querySelectorAll("button");
  assert.equal(controlsInsideAnchor?.length ?? 0, 0, "detail-cover anchor box must not contain paging controls");
  assert.ok(pane?.querySelectorAll("button").length >= 4, "arrows + side zones live outside the anchor");

  /* 右栏 = 唯一滚动容器（layer-scroller），正文在其中。 */
  const scroller = document.querySelector('[data-slot="layer-scroller"]');
  assert.ok(scroller, "variant right column must expose layer-scroller");
  assert.ok(scroller?.textContent?.includes("Portrait body"));

  /* 壳层方案二 float：header 移除、标题 sr-only 三职保留、悬浮返回/关闭钮。 */
  assert.equal(document.querySelector("header"), null, "variant top layer removes the header");
  const dialog = view.getByRole("dialog") as HTMLElement;
  assert.ok(dialog.querySelector("h2.sr-only"), "sr-only title keeps aria-labelledby/focus duties");
  assert.ok(view.getByRole("button", { name: "Back to: the content list" }), "floating back button hover/aria text");
  assert.ok(view.getByRole("button", { name: "Close content detail" }), "floating close button");
  assert.ok(document.querySelector('[data-slot="overlay-scroller"]'));
});

test("#397 mixed media set (any portrait item) goes variant", async () => {
  installApiMock();
  const mixed = renderOverlay(<OverlayHarness entryId={23} zone="original" />, true);
  await openOverlay(mixed, "Mixed Set Work");
  assert.ok(document.querySelector('[data-slot="variant-media-pane"]'), "mixed set must use variant");
});

test("#397 all-landscape media set keeps the split-media design", async () => {
  installApiMock();
  const landscape = renderOverlay(<OverlayHarness entryId={24} zone="original" />, true);
  await openOverlay(landscape, "Landscape Set Work");
  assert.equal(document.querySelector('[data-slot="variant-media-pane"]'), null, "all-landscape set keeps current design");
  assert.ok(document.querySelector("header"), "non-variant keeps the header");
  assert.ok(document.querySelector('[data-slot="layer-scroller"]'), "split-media info column still present");
});

test("#397 portrait set on non-desktop viewport stays on the legacy path", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={21} zone="original" />, false);
  await openOverlay(view, "Portrait Set Work");
  assert.equal(document.querySelector('[data-slot="variant-media-pane"]'), null, "<1100px keeps the single-column design");
  assert.ok(document.querySelector("header"));
});

test("#397 related block pins order: source original → series → derivatives, before comments", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={25} zone="fanwork" />, true);
  await openOverlay(view, "Fanwork With Relations");

  const block = document.querySelector('[data-slot="overlay-related-block"]');
  assert.ok(block, "related block renders when relations exist");

  const sourceBtn = block?.querySelector('[data-slot="related-source-btn"]');
  assert.ok(sourceBtn, "source original row renders");
  assert.ok(sourceBtn?.textContent?.includes("The Original Work"));
  assert.ok(block?.textContent?.includes("Original"), "original badge");
  assert.ok(block?.textContent?.includes("Color Studies"), "series row (first series)");
  assert.ok(block?.textContent?.includes("Derivative One"), "derivative list row");

  /* 系列边界：current_index=2/5 → 上一章可用；首章无更前章节的禁用由下条覆盖。 */
  const chapterButtons = Array.from(block?.querySelectorAll("button") ?? []);
  const prevChapter = chapterButtons.find((b) => b.textContent?.includes("Previous"));
  const nextChapter = chapterButtons.find((b) => b.textContent?.includes("Next"));
  assert.ok(prevChapter && !prevChapter.disabled, "has previous chapter → enabled");
  assert.ok(nextChapter && !nextChapter.disabled, "has next chapter → enabled");

  /* 评论区 = 右栏末块：关联块之后出现评论区标题（匿名无输入框，以标题定位）。 */
  const scroller = document.querySelector('[data-slot="layer-scroller"]');
  const commentHeading = Array.from(scroller?.querySelectorAll("h3") ?? []).find((h) =>
    h.textContent?.includes("Comments"),
  );
  assert.ok(commentHeading, "comment section is the last block of the right column");
  assert.equal(
    (block as Node).compareDocumentPosition(commentHeading as Node) & Node.DOCUMENT_POSITION_FOLLOWING,
    Node.DOCUMENT_POSITION_FOLLOWING,
    "related block sits before the comments",
  );
});

test("#397 no relations → related block not rendered; comments still last", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={21} zone="original" />, true);
  await openOverlay(view, "Portrait Set Work");
  assert.equal(document.querySelector('[data-slot="overlay-related-block"]'), null);
  assert.ok(
    Array.from(document.querySelector('[data-slot="layer-scroller"]')?.querySelectorAll("h3") ?? []).some((h) =>
      h.textContent?.includes("Comments"),
    ),
  );
});

test("#397 clicking the media (pane center) opens MediaViewer above the overlay", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={21} zone="original" />, true);
  await openOverlay(view, "Portrait Set Work");

  const cover = document.querySelector<HTMLElement>('[data-slot="variant-media-pane"] [data-slot="detail-cover"]');
  assert.ok(cover);
  assert.equal(document.querySelectorAll("dialog[open]").length, 1, "only the overlay dialog before viewer");

  await act(async () => {
    fireEvent.click(cover);
    await Promise.resolve();
  });
  await waitFor(() => assert.equal(document.querySelectorAll("dialog[open]").length, 2, "viewer dialog stacks on top"));
  assert.ok(document.querySelector('[data-slot="variant-media-pane"] img'), "pane media still rendered underneath");
});
