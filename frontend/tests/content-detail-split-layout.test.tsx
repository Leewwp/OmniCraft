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
   implemented; tests that need the desktop split viewport stub it to matches. */
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
      matches: query === "(min-width: 1100px)",
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

/* Media-set image content（#88 双栏适用）：媒体集 = image 附件（首项 1600x900 横图，
   次项 900x1200 竖图 → 有翻页控件位）。 */
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
      oss_key: "/seed-media/real/gallery/g02-portrait.svg",
      width: 900,
      height: 1200,
      sort_order: 1,
    },
  ],
  tags: [],
};

/* 历史 image 内容：无媒体集附件 → 维持单栏（行内 CoverImage）。 */
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
    /* #90 相关内容块的相似内容行：固定 list 合同，测试给空数据即可。 */
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
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: expectedTitle })));
  } else {
    await waitFor(() => assert.ok(view.getByRole("heading", { level: 2 })));
  }
  return trigger;
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

test("#88 media-set content renders the split layout: left media column + right layer-scroller", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={7} zone="original" />);
  await openOverlay(view, 7);

  const layerScroller = document.querySelector('[data-slot="layer-scroller"]');
  assert.ok(layerScroller, "split layout must expose the info-column scroller");
  /* 行内媒体区（单列兜底）+ 左栏媒体列 = 两个 detail-cover 锚点。 */
  assert.equal(document.querySelectorAll('[data-slot="detail-cover"]').length, 2);
  /* 左栏 aspect 盒：按首项 1600x900 自适应比例。 */
  const box = document.querySelector('[data-slot="layer-scroller"]')
    ?.parentElement?.firstElementChild?.firstElementChild as HTMLElement | null;
  assert.ok(box, "aspect box must exist in the media column");
  assert.equal(parseFloat(box.style.aspectRatio), 1600 / 900);
  assert.ok(view.getByRole("heading", { level: 2, name: "Gallery Image Work" }));
  assert.ok(view.getByText("Image body"));
});

test("#88 split scroll memory routes to the layer-scroller and restores on pop", async () => {
  installApiMock();
  /* jsdom 视口视为桌面（≥1100px）：滚动路由走层内信息列。 */
  const view = renderOverlay(<OverlayHarness entryId={7} zone="original" />, true);
  await openOverlay(view, 7, "Gallery Image Work");

  const layerScroller = document.querySelector<HTMLElement>('[data-slot="layer-scroller"]');
  assert.ok(layerScroller);
  const overlayScroller = document.querySelector<HTMLElement>('[data-slot="overlay-scroller"]');
  assert.ok(overlayScroller);

  layerScroller.scrollTop = 120;

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Open content detail: Related 101" }));
    await Promise.resolve();
  });
  await waitFor(() =>
    assert.ok(view.getByRole("heading", { level: 2, name: "Related 101" })),
  );

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Back to Gallery Image Work" }));
    await Promise.resolve();
  });
  await waitFor(() =>
    assert.ok(view.getByRole("heading", { level: 2, name: "Gallery Image Work" })),
  );
  await waitFor(() => assert.equal(layerScroller.scrollTop, 120));
});

test("#88 legacy image content without a media set stays single-column", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={8} zone="original" />, true);
  await openOverlay(view, 8, "Legacy Cover Work");

  assert.ok(document.querySelector('[data-slot="layer-scroller"]') === null);
  assert.equal(document.querySelectorAll('[data-slot="detail-cover"]').length, 1);
  assert.ok(document.querySelector('[data-slot="overlay-scroller"]'));
  assert.ok(view.getByRole("heading", { level: 2, name: "Legacy Cover Work" }));
});

test("#88 dual-column contracts exist in source: split media slot, layer-scroller, hidden inline media", () => {
  const detail = read("components/content/ContentDetail.tsx");
  const layer = read("components/content/ContentDetailOverlayLayer.tsx");
  const overlay = read("components/content/ContentDetailOverlay.tsx");

  assert.match(detail, /mediaSlot\?: "inline" \| "split"/);
  assert.match(detail, /coverReady\?: boolean/);
  assert.match(detail, /min-\[1100px\]:hidden/);
  assert.match(layer, /data-slot="layer-scroller"/);
  assert.match(layer, /min-\[1100px\]:grid-cols-\[minmax\(0,3fr\)_minmax\(0,2fr\)\]/);
  assert.match(layer, /aspectRatio: String\(splitRatio\)/);
  assert.match(overlay, /min-\[1100px\]:overflow-hidden/);
  assert.match(overlay, /data-slot="overlay-scroller"/);
});
