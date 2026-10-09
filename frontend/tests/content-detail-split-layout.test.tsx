import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React, { useRef, useState } from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

/* Native <dialog> modal lifecycle is not implemented in jsdom; stub it so the
   overlay exercises the same code paths as browsers. matchMedia is also not
   implemented; tests that need the desktop viewport stub it to matches.
   注意：断言失败值避免直接传 jsdom DOM 节点（node:test 序列化 DOM 节点会令
   子进程失去响应），统一用 Boolean()/数值。 */
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
  if (splitViewport) {
    window.matchMedia = ((query: string) => ({
      matches: query === "(min-width: 960px)",
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    })) as typeof window.matchMedia;
  }
}

/* Stub next/navigation + AuthContext so ContentDetail/FollowButton render
   without providers (same Module._load interception as content-detail-overlay). */
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

type OverlayModule = typeof import("@/components/content/ContentDetailOverlay");

let ContentDetailOverlay: OverlayModule["ContentDetailOverlay"];

test.before(async () => {
  const overlayModule = await import("@/components/content/ContentDetailOverlay");
  await import("@/components/content/ContentDetailOverlayLayer");
  ContentDetailOverlay = overlayModule.ContentDetailOverlay;
});

/* 媒体集 image 内容（全横图：1600x900 + 1280x720）。D1 #858 起媒体朝向不再
   分版式——横集与竖集同一统一版式，本夹具取旧 split 分支的代表性输入。 */
const IMAGE_DETAIL = {
  content: {
    id: 7,
    title: "Gallery Image Work",
    zone: "original",
    content_type: "image",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Image body",
    cover_image_url: "/seed-media/real/gallery/g01-landscape.svg",
    like_count: 3,
  },
  attachments: [
    {
      id: 11,
      content_item_id: 7,
      file_type: "image",
      oss_key: "/seed-media/real/gallery/g01-landscape.svg",
      width: 1600,
      height: 900,
      sort_order: 0,
    },
    {
      id: 12,
      content_item_id: 7,
      file_type: "image",
      oss_key: "/seed-media/real/gallery/g02-wide.svg",
      width: 1280,
      height: 720,
      sort_order: 1,
    },
  ],
  tags: [],
};

/* 历史 image 内容：无媒体集附件 → 封面链（真实封面）进统一媒体列。
   cover 尺寸给横图（生产 cover_width/height 全库为 0 由 Image 实测，jsdom
   不加载资源，测试夹具直接提供尺寸走免探测路径）。 */
const LEGACY_IMAGE_DETAIL = {
  content: {
    id: 8,
    title: "Legacy Cover Work",
    zone: "original",
    content_type: "image",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Legacy image body",
    cover_image_url: "/seed-media/covers/cover-02.svg",
    cover_width: 1600,
    cover_height: 900,
    like_count: 1,
  },
  attachments: [],
  tags: [],
};

const detailByPath = new Map<string, unknown>([
  ["/api/v1/contents/7", IMAGE_DETAIL],
  ["/api/v1/contents/8", LEGACY_IMAGE_DETAIL],
]);

const originalGet = api.get;
const originalPost = api.post;

let relatedCallCount = 0;
function installApiMock() {
  relatedCallCount = 0;
  api.get = async function mockedGet<T>(requestPath: string): Promise<T> {
    if (requestPath.includes("/related-fanworks")) {
      relatedCallCount += 1;
      const id = 100 + relatedCallCount;
      return {
        contents: [
          {
            id,
            title: `Related ${id}`,
            zone: "fanwork",
            content_type: "image",
            author: { id: 11, username: "Related Author" },
            like_count: 3,
          },
        ],
        total: 1,
      } as T;
    }
    if (requestPath.startsWith("/api/v1/social/comments")) {
      return { comments: [] } as T;
    }
    /* RelatedContents 相似内容行：固定 list 合同，测试给空数据即可。 */
    if (requestPath.startsWith("/api/v1/contents?")) {
      return { contents: [], total: 0 } as T;
    }
    const contentIdMatch = requestPath.match(/^\/api\/v1\/contents\/(\d+)$/);
    if (contentIdMatch) {
      const contentId = Number(contentIdMatch[1]);
      const found = detailByPath.get(`/api/v1/contents/${contentId}`);
      if (found) return found as T;
      if (contentId >= 101 && contentId <= 199) {
        return {
          content: {
            id: contentId,
            title: `Related ${contentId}`,
            zone: "fanwork",
            content_type: "image",
            author: { id: 11, username: "Related Author" },
            status: "published",
            description: `Related ${contentId} body`,
            cover_image_url: "/seed-media/covers/related-landscape.svg",
            cover_width: 1600,
            cover_height: 900,
            like_count: 3,
          },
          attachments: [],
          tags: [],
        } as T;
      }
      throw new ApiRequestError("NOT_FOUND", "raw secret backend error", 404);
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

async function openOverlay(view: ReturnType<typeof render>, entryId: number, expectedTitle?: string) {
  const trigger = view.getByRole("button", { name: "Open overlay" });
  await act(async () => {
    fireEvent.click(trigger);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  /* 桌面视口下 #90 相关内容块会额外渲染标题行（h2），等待时必须按名称区分。 */
  if (expectedTitle) {
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 1, name: expectedTitle })));
  } else {
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 2 })));
  }
  return trigger;
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

test("#858 media-set content renders the unified desktop layout: flush media pane + right layer-scroller", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={7} zone="original" />, true);
  await openOverlay(view, 7, "Gallery Image Work");

  const layerScroller = document.querySelector('[data-slot="layer-scroller"]');
  assert.ok(layerScroller, "unified layout must expose the info-column scroller");
  assert.ok(document.querySelector('[data-slot="variant-media-pane"]'), "media pane present for media-set content");
  /* 行内媒体区（单列兜底，min-[960px]:hidden）+ 左栏媒体列 = 两个 detail-cover 锚点。 */
  assert.equal(document.querySelectorAll('[data-slot="detail-cover"]').length, 2);
  assert.ok(view.getByRole("heading", { level: 1, name: "Gallery Image Work" }));
  assert.ok(view.getByText("Image body"));
});

test("#858 unified scroll memory routes to the layer-scroller and restores on pop", async () => {
  installApiMock();
  /* jsdom 视口视为桌面（≥960px stub）：滚动路由走层内信息列。 */
  const view = renderOverlay(<OverlayHarness entryId={7} zone="original" />, true);
  await openOverlay(view, 7, "Gallery Image Work");

  const layerScroller = document.querySelector<HTMLElement>('[data-slot="layer-scroller"]');
  assert.ok(layerScroller);
  const overlayScroller = document.querySelector<HTMLElement>('[data-slot="overlay-scroller"]');
  assert.ok(overlayScroller);

  layerScroller.scrollTop = 120;

  /* related 推荐区经 DeferredMount 延后挂载；关联行 = RelatedFanworks 卡片
     （ContentCard 可访问名 = 标题本体，不同于旧侧栏行的 open-detail 标签）。 */
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Related 101" })));
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Related 101" }));
    await Promise.resolve();
  });
  await waitFor(() =>
    assert.ok(view.getByRole("heading", { level: 2, name: "Related 101" })),
  );

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Back to: Gallery Image Work" }));
    await Promise.resolve();
  });
  await waitFor(() =>
    assert.ok(view.getByRole("heading", { level: 1, name: "Gallery Image Work" })),
  );
  await waitFor(() => assert.equal(layerScroller.scrollTop, 120));
});

test("#858 legacy image content without a media set joins the unified layout via the cover chain", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={8} zone="original" />, true);
  await openOverlay(view, 8, "Legacy Cover Work");

  /* D1：历史无媒体集内容桌面不再回退全宽单列——封面链进统一媒体列。 */
  assert.ok(document.querySelector('[data-slot="variant-media-pane"]'), "cover chain feeds the unified pane");
  assert.ok(document.querySelector('[data-slot="layer-scroller"]'), "unified info column present");
  assert.ok(view.getByRole("heading", { level: 1, name: "Legacy Cover Work" }));
});

test("#858 unified desktop contracts exist in source: 960 panel, flush shell, viewport-driven scroller", () => {
  const detail = read("components/content/ContentDetail.tsx");
  const layout = read("components/content/OverlayVariantLayout.tsx");
  const overlay = read("components/content/ContentDetailOverlay.tsx");
  const layer = read("components/content/ContentDetailOverlayLayer.tsx");
  const relatedContents = read("components/content/RelatedContents.tsx");

  /* ContentDetail：媒体槽与行内媒体隐藏契约保持；creatorFirst（右栏块序）。 */
  assert.match(detail, /mediaSlot\?: "inline" \| "split" \| "variant"/);
  assert.match(detail, /coverReady\?: boolean/);
  assert.match(detail, /min-\[960px\]:hidden/);
  assert.match(detail, /creatorFirst\?: boolean/);

  /* 统一版式几何：媒体列直贴面板（无负 margin 抵消）、右栏唯一滚动列。 */
  assert.match(layout, /data-slot="layer-scroller"/);
  assert.match(layout, /data-slot="variant-media-pane"/);
  assert.doesNotMatch(layout, /min-\[960px\]:-mx-6/);
  assert.doesNotMatch(layout, /-mb-6/);
  assert.doesNotMatch(layout, /pt-14/);

  /* 浮层壳层：面板断点 960 与内部双栏对齐（960–1023 也收居中面板）。 */
  assert.match(overlay, /min-\[960px\]:m-auto/);
  assert.match(overlay, /min-\[960px\]:h-\[min\(92dvh,900px\)\]/);
  assert.doesNotMatch(overlay, /lg:h-\[min\(92dvh/);
  assert.match(overlay, /isDesktopViewport && "h-full overflow-hidden p-0"/);
  assert.match(overlay, /data-slot="overlay-scroller"/);

  /* 三分支退役：split 布局判定与 64px 控件条不再存在。 */
  assert.doesNotMatch(layer, /SPLIT_MEDIA_CONTROLS_HEIGHT/);
  assert.doesNotMatch(layer, /split-media/);
  assert.doesNotMatch(layer, /grid-cols-\[minmax\(0,3fr\)/);

  /* 推荐区视口门下浮（浮层统一右栏 960 起可见）。 */
  assert.match(relatedContents, /minViewportPx\?: number/);
});
