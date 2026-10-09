import test from "node:test";
import assert from "node:assert/strict";
import React, { useRef, useState } from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D1 #858 详情弹窗统一版式（桌面 ≥960 单一版式）行为契约：
 * - 任意内容类型/媒体朝向 → 同一统一版式树（左媒体列 + 右信息列 layer-scroller
 *   + float 壳：无 header 工具栏、sr-only 标题三职、悬浮返回/关闭钮）；
 * - 右栏块序钉死：创作者行 → 标题 → 元信息/来源 → 正文 → 关联 IP/原创/系列 →
 *   评论区 → 推荐区（相关创作 + 你可能也喜欢），各块单实例；
 * - 衍生列表从关联块拆出，评论区之后与相似推荐组成推荐区；
 * - loading/错误态从首帧保持双栏稳定壳（不等待媒体朝向/chainReady）；
 * - 960–1099 推荐区可见（RelatedContents 视口参数，旧 1100 门不适用浮层）；
 * - 移动 <960 单列契约不回归（header/行内媒体/侧栏）。
 * jsdom 测不了的真实断点/滚动几何（滚动条贴缘、× 命中区、面板尺寸）由
 * 真 Chromium 验收覆盖（screenshots/）。
 * ──────────────────────────────────────────────────────────────────────────── */

function installOverlayTestStubs({ desktopViewport = false }: { desktopViewport?: boolean } = {}) {
  const prototype = window.HTMLDialogElement?.prototype as unknown as HTMLDialogElement | undefined;
  if (!prototype) return;
  prototype.showModal = function showModalStub(this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  prototype.close = function closeStub(this: HTMLDialogElement) {
    this.removeAttribute("open");
  };
  window.scrollTo = () => undefined;
  /* matchMedia 复原：jsdom 原生无实现（undefined）。desktopViewport=true 模拟
     960–1099 视口（960 命中、1100 不命中）——本文件桌面用例全部取该最严带宽，
     同时证明统一版式与推荐区可见性在 960–1099 成立。 */
  if (desktopViewport) {
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

type OverlayModule = typeof import("@/components/content/ContentDetailOverlay");
let ContentDetailOverlay: OverlayModule["ContentDetailOverlay"];

test.before(async () => {
  const overlayModule = await import("@/components/content/ContentDetailOverlay");
  await import("@/components/content/ContentDetailOverlayLayer");
  ContentDetailOverlay = overlayModule.ContentDetailOverlay;
});

/* 夹具（全部自带宽高 → 免 Image 探测路径，jsdom 不加载资源）。 */
const LANDSCAPE_SET = {
  content: {
    id: 61,
    title: "Landscape Set Work",
    zone: "original",
    content_type: "image",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Landscape body",
  },
  attachments: [
    { id: 511, content_item_id: 61, file_type: "image", oss_key: "/seed/l01.svg", width: 1600, height: 900, sort_order: 0 },
    { id: 512, content_item_id: 61, file_type: "image", oss_key: "/seed/l02.svg", width: 1280, height: 720, sort_order: 1 },
  ],
  tags: [],
};

const PORTRAIT_SET = {
  content: {
    id: 62,
    title: "Portrait Set Work",
    zone: "original",
    content_type: "image",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Portrait body",
  },
  attachments: [
    { id: 521, content_item_id: 62, file_type: "image", oss_key: "/seed/p01.svg", width: 900, height: 1200, sort_order: 0 },
    { id: 522, content_item_id: 62, file_type: "image", oss_key: "/seed/p02.svg", width: 900, height: 1200, sort_order: 1 },
  ],
  tags: [],
};

const VIDEO_CONTENT = {
  content: {
    id: 63,
    title: "Video Work",
    zone: "original",
    content_type: "video",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Video body",
    cover_image_url: "/seed/video-poster.svg",
  },
  attachments: [
    { id: 531, content_item_id: 63, file_type: "video", oss_key: "/seed/v01.mp4", width: 1280, height: 720, sort_order: 0 },
  ],
  tags: [],
};

const ARTICLE = {
  content: {
    id: 64,
    title: "Article Work",
    zone: "original",
    content_type: "article",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Article body",
  },
  attachments: [],
  tags: [],
};

const MOD_CONTENT = {
  content: {
    id: 65,
    title: "Mod Work",
    zone: "original",
    content_type: "mod",
    author: { id: 9, username: "Media Author" },
    status: "published",
    description: "Mod body",
  },
  attachments: [
    { id: 541, content_item_id: 65, file_type: "mod", oss_key: "/seed/m01.zip", sort_order: 0 },
  ],
  tags: [],
};

/* fanwork：IP + 关联原创 + 系列 + 衍生列表（右栏块序断言的全量载体）。 */
const FANWORK_RELATED = {
  content: {
    id: 66,
    title: "Fanwork Relations Work",
    zone: "fanwork",
    content_type: "image",
    author: { id: 12, username: "Fan Author" },
    status: "published",
    description: "Fanwork relations body",
    ip: { id: 5, name: "Indigo IP" },
  },
  attachments: [
    { id: 551, content_item_id: 66, file_type: "image", oss_key: "/seed/f01.svg", width: 900, height: 1200, sort_order: 0 },
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
  source_original: { id: 60, title: "The Source Original", zone: "original" },
};

const detailByPath = new Map<string, unknown>([
  ["/api/v1/contents/61", LANDSCAPE_SET],
  ["/api/v1/contents/62", PORTRAIT_SET],
  ["/api/v1/contents/63", VIDEO_CONTENT],
  ["/api/v1/contents/64", ARTICLE],
  ["/api/v1/contents/65", MOD_CONTENT],
  ["/api/v1/contents/66", FANWORK_RELATED],
]);

const originalGet = api.get;
const originalPost = api.post;

function installApiMock(options: { hangDetail?: boolean; failAll?: boolean } = {}) {
  api.get = async function mockedGet<T>(requestPath: string): Promise<T> {
    if (options.failAll) {
      throw new ApiRequestError("DB_ERROR", "backend unavailable", 500);
    }
    if (requestPath.includes("/related-fanworks")) {
      const id = Number(requestPath.match(/\/contents\/(\d+)\//)?.[1] ?? 0);
      if (id === 66) {
        return {
          contents: [
            {
              id: 300,
              title: "Derivative One",
              zone: "fanwork",
              content_type: "image",
              author: { id: 11, username: "Related Author" },
              cover_image_url: "/seed/related.svg",
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
    /* RelatedContents 相似内容行：固定 list 合同，返回一条保证推荐区非空渲染。 */
    if (requestPath.startsWith("/api/v1/contents?")) {
      return {
        contents: [
          {
            id: 400,
            title: "Similar One",
            zone: "fanwork",
            content_type: "image",
            author: { id: 13, username: "Similar Author" },
            cover_image_url: "/seed/similar.svg",
            like_count: 1,
          },
        ],
        total: 1,
      } as T;
    }
    const contentIdMatch = requestPath.match(/^\/api\/v1\/contents\/(\d+)$/);
    if (contentIdMatch && !options.hangDetail) {
      const found = detailByPath.get(`/api/v1/contents/${contentIdMatch[1]}`);
      if (found) return found as T;
    }
    if (options.hangDetail) return new Promise<void>(() => {}) as T;
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

function renderOverlay(node: React.ReactNode, desktopViewport = false) {
  installDom();
  installOverlayTestStubs({ desktopViewport });
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
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 1, name: expectedTitle })));
  }
  return trigger;
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

/* ── 桌面统一版式树：任意类型同一版式 ─────────────────────────────────── */

async function assertUnifiedDesktopTree(view: ReturnType<typeof render>, title: string) {
  await openOverlay(view, title);

  const pane = document.querySelector('[data-slot="variant-media-pane"]');
  assert.ok(pane, "unified layout must render the flush media pane for every type");
  const scrollers = document.querySelectorAll('[data-slot="layer-scroller"]');
  assert.equal(scrollers.length, 1, "exactly one right-column scroller (layer-scroller)");
  const dialog = view.getByRole("dialog") as HTMLElement;
  assert.ok(!dialog.querySelector("header"), "float shell removes the header toolbar row");
  assert.ok(dialog.querySelector("h2.sr-only"), "sr-only title keeps aria-labelledby/focus duties");
  assert.ok(view.getByRole("button", { name: "Back to: the content list" }), "floating back button");
  assert.ok(view.getByRole("button", { name: "Close content detail" }), "floating close button");
  const h1 = view.getByRole("heading", { level: 1, name: title });
  assert.ok(
    (scrollers[0] as Node).contains(h1 as Node),
    "content title lives inside the right-column scroller",
  );
}

test("desktop unified tree: landscape media set uses the same layout as portrait sets", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={61} zone="original" />, true);
  await assertUnifiedDesktopTree(view, "Landscape Set Work");
});

test("desktop unified tree: portrait media set", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={62} zone="original" />, true);
  await assertUnifiedDesktopTree(view, "Portrait Set Work");
});

test("desktop unified tree: video content", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={63} zone="original" />, true);
  await assertUnifiedDesktopTree(view, "Video Work");
});

test("desktop unified tree: text-only article uses the text-cover media column", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={64} zone="original" />, true);
  await assertUnifiedDesktopTree(view, "Article Work");
  const pane = document.querySelector('[data-slot="variant-media-pane"]');
  assert.ok(pane?.querySelector("img"), "text cover placeholder chain feeds the media pane");
});

test("desktop unified tree: attachment-family content (mod)", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={65} zone="original" />, true);
  await assertUnifiedDesktopTree(view, "Mod Work");
});

/* ── 右栏块序钉死 + 各块单实例 + 衍生列表拆出 ─────────────────────────── */

test("right column order is pinned: creator row → title → relations → comments → recommendations", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={66} zone="fanwork" />, true);
  await openOverlay(view, "Fanwork Relations Work");

  /* #430 挂载分帧：关联块/评论区/推荐区经 DeferredMount 延后一拍，先等落定。 */
  const scroller = await waitFor(() => {
    const el = document.querySelector('[data-slot="overlay-related-block"]');
    assert.ok(el, "related block renders when relations exist");
    const contents = document.querySelector('[data-slot="related-contents"]');
    assert.ok(contents, "recommendation section renders after comments");
    return document.querySelector<HTMLElement>('[data-slot="layer-scroller"]');
  });
  assert.ok(scroller, "layer-scroller present");

  const followButton = view.getByRole("button", { name: "Follow" });
  const h1 = view.getByRole("heading", { level: 1, name: "Fanwork Relations Work" });
  const ipLink = document.querySelector('[data-slot="related-ip-link"]');
  assert.ok(ipLink, "related IP row renders");
  const block = document.querySelector('[data-slot="overlay-related-block"]') as HTMLElement;
  assert.ok(block.textContent?.includes("The Source Original"), "source original row inside the block");
  assert.ok(block.textContent?.includes("Color Studies"), "series row inside the block");
  const commentsHeading = Array.from(scroller.querySelectorAll("h3")).find((h) =>
    h.textContent?.includes("Comments"),
  );
  assert.ok(commentsHeading, "comment section renders");
  const recommendations = document.querySelector('[data-slot="related-contents"]') as HTMLElement;

  const FOLLOWING = Node.DOCUMENT_POSITION_FOLLOWING;
  assert.equal(
    (followButton as Node).compareDocumentPosition(h1 as Node) & FOLLOWING,
    FOLLOWING,
    "creator row (follow) precedes the title",
  );
  assert.equal(
    (h1 as Node).compareDocumentPosition(ipLink as Node) & FOLLOWING,
    FOLLOWING,
    "title precedes the relations block",
  );
  assert.equal(
    (block as Node).compareDocumentPosition(commentsHeading as Node) & FOLLOWING,
    FOLLOWING,
    "relations block precedes comments",
  );
  assert.equal(
    (commentsHeading as Node).compareDocumentPosition(recommendations as Node) & FOLLOWING,
    FOLLOWING,
    "comments precede the recommendation section",
  );

  /* 各块单实例：无重复关联块/推荐区/评论区/关注入口。 */
  assert.equal(document.querySelectorAll('[data-slot="overlay-related-block"]').length, 1);
  assert.equal(document.querySelectorAll('[data-slot="related-contents"]').length, 1);
  assert.equal(
    Array.from(scroller.querySelectorAll("h3")).filter((h) => h.textContent?.includes("Comments")).length,
    1,
    "exactly one comment section",
  );
  assert.equal(document.querySelectorAll("aside").length, 0, "ContentSidebar exits the desktop unified column");
});

test("derivatives list leaves the related block and joins the recommendation section after comments", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={66} zone="fanwork" />, true);
  await openOverlay(view, "Fanwork Relations Work");

  const block = await waitFor(() => {
    const el = document.querySelector('[data-slot="overlay-related-block"]');
    assert.ok(el, "related block renders");
    return el as HTMLElement;
  });
  const recommendations = await waitFor(() => {
    const el = document.querySelector('[data-slot="related-contents"]');
    assert.ok(el, "recommendation section renders");
    return el as HTMLElement;
  });

  assert.equal(
    block.textContent?.includes("Derivative One"),
    false,
    "derivatives no longer sit in the pre-comment related block",
  );
  assert.ok(
    recommendations.textContent?.includes("Derivative One"),
    "derivatives render in the post-comment recommendation section",
  );
  /* 相似行为异步拉取（RelatedContents 挂载后请求 list 合同），等落定再断言。 */
  await waitFor(() => {
    assert.ok(
      recommendations.textContent?.includes("Similar One"),
      "similar recommendations share the recommendation section",
    );
  });
});

/* ── loading / 错误态：首帧稳定双栏壳 ─────────────────────────────────── */

test("loading state keeps the stable two-column shell from the first frame", async () => {
  installApiMock({ hangDetail: true });
  const view = renderOverlay(<OverlayHarness entryId={62} zone="original" />, true);
  await openOverlay(view);

  assert.ok(document.querySelector('[data-slot="variant-media-pane"]'), "media pane exists while loading");
  assert.ok(document.querySelector('[data-slot="layer-scroller"]'), "right scroller exists while loading");
  assert.ok(document.querySelector('[aria-busy="true"]'), "skeleton renders inside the shell");
  const dialog = view.getByRole("dialog") as HTMLElement;
  assert.ok(!dialog.querySelector("header"), "no header flash before data arrives");
});

test("error state keeps the two-column shell and never falls back to full-width single column", async () => {
  installApiMock({ failAll: true });
  const view = renderOverlay(<OverlayHarness entryId={62} zone="original" />, true);
  await openOverlay(view);
  await waitFor(() => assert.ok(view.getByText("Content detail failed to load")));

  assert.ok(document.querySelector('[data-slot="variant-media-pane"]'), "media pane stays mounted on error");
  const scroller = document.querySelector('[data-slot="layer-scroller"]');
  assert.ok(scroller, "right scroller stays mounted on error");
  assert.ok(scroller?.textContent?.includes("Content detail failed to load"), "error state lives in the right column");
  const dialog = view.getByRole("dialog") as HTMLElement;
  assert.ok(!dialog.querySelector("header"), "error keeps the float shell");
});

/* ── 960–1099 推荐区可见 ─────────────────────────────────────────────── */

test("recommendation section is visible at 960–1099 (old 1100 gate retired for the overlay)", async () => {
  installApiMock();
  /* 本文件桌面 stub 即 960–1099 视口（1100 不命中）——推荐区必须照常渲染。 */
  const view = renderOverlay(<OverlayHarness entryId={66} zone="fanwork" />, true);
  await openOverlay(view, "Fanwork Relations Work");

  await waitFor(() => {
    assert.ok(document.querySelector('[data-slot="related-contents"]'), "recommendation section renders below 1100");
  });
  await waitFor(() => {
    assert.ok(
      document.querySelector('[data-slot="related-contents-similar"]'),
      "similar row renders with data at 960–1099",
    );
  });
});

/* ── 移动 <960 单列契约不回归 ─────────────────────────────────────────── */

test("mobile (<960) keeps the single-column contract: header, inline media, sidebar", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={62} zone="original" />, false);
  await openOverlay(view, "Portrait Set Work");

  const dialog = view.getByRole("dialog") as HTMLElement;
  assert.ok(dialog.querySelector("header"), "mobile keeps the header toolbar (back/close/title)");
  assert.ok(!document.querySelector('[data-slot="variant-media-pane"]'), "no media pane on mobile");
  assert.ok(document.querySelector("aside"), "ContentSidebar stays in the mobile tree");
  assert.ok(document.querySelector('[data-slot="overlay-scroller"]'), "single column scrolls in overlay-scroller");
  /* 行内画廊（detail-cover 行内锚点）存在于单列流。 */
  const inlineCover = document.querySelector('[data-slot="overlay-scroller"] [data-slot="detail-cover"]');
  assert.ok(inlineCover, "inline gallery stays in the mobile flow");
});
