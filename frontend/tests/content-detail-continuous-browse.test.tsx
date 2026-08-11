import test from "node:test";
import assert from "node:assert/strict";
import React, { useRef } from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* Native <dialog> showModal/close are not implemented in jsdom; stub the modal
   lifecycle so tests exercise the same code paths as browsers. matchMedia is
   not implemented either; continuous browsing requires the mobile viewport
   (<1100px), which is the default jsdom state (no stub) unless splitViewport. */
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

/* Stub next/navigation + AuthContext so ContentDetail renders without
   providers (same Module._load interception as content-detail-overlay tests). */
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

type HookModule = typeof import("@/components/content/use-content-detail-overlay");
let useContentDetailOverlay: HookModule["useContentDetailOverlay"];

test.before(async () => {
  const module = await import("@/components/content/use-content-detail-overlay");
  await import("@/components/content/ContentDetailOverlayLayer");
  useContentDetailOverlay = module.useContentDetailOverlay;
});

/* 3 篇上下文内容（AC1 列表）：A 单媒体、B 双媒体、C 单媒体 —— 用于验证
   切篇后媒体区索引/加载态重置与列表到底提示。 */
const DETAILS = new Map<number, unknown>([
  [
    1,
    {
      content: {
        id: 1,
        title: "Context A",
        zone: "original",
        content_type: "image",
        author: { id: 9, username: "Browse Author" },
        status: "published",
        description: "A body",
        like_count: 3,
      },
      attachments: [
        {
          id: 11,
          content_item_id: 1,
          file_type: "image",
          oss_key: "/seed-media/gallery/a-1.svg",
          width: 1200,
          height: 900,
          sort_order: 0,
        },
      ],
      tags: [],
    },
  ],
  [
    2,
    {
      content: {
        id: 2,
        title: "Context B",
        zone: "original",
        content_type: "image",
        author: { id: 9, username: "Browse Author" },
        status: "published",
        description: "B body",
        like_count: 5,
      },
      attachments: [
        {
          id: 21,
          content_item_id: 2,
          file_type: "image",
          oss_key: "/seed-media/gallery/b-1.svg",
          width: 1200,
          height: 900,
          sort_order: 0,
        },
        {
          id: 22,
          content_item_id: 2,
          file_type: "image",
          oss_key: "/seed-media/gallery/b-2.svg",
          width: 900,
          height: 1200,
          sort_order: 1,
        },
      ],
      tags: [],
    },
  ],
  [
    3,
    {
      content: {
        id: 3,
        title: "Context C",
        zone: "original",
        content_type: "image",
        author: { id: 9, username: "Browse Author" },
        status: "published",
        description: "C body",
        like_count: 7,
      },
      attachments: [
        {
          id: 31,
          content_item_id: 3,
          file_type: "image",
          oss_key: "/seed-media/gallery/c-1.svg",
          width: 1200,
          height: 900,
          sort_order: 0,
        },
      ],
      tags: [],
    },
  ],
]);

const CONTEXT_LIST = [
  { id: 1, zone: "original" as const },
  { id: 2, zone: "original" as const },
  { id: 3, zone: "original" as const },
];

const originalGet = api.get;
const originalPost = api.post;

let detailCallCount = 0;
function installApiMock() {
  detailCallCount = 0;
  api.get = async function mockedGet<T>(requestPath: string): Promise<T> {
    if (requestPath.includes("/related-fanworks")) {
      return { contents: [], total: 0 } as T;
    }
    if (requestPath.includes("/versions")) {
      return { versions: [] } as T;
    }
    if (requestPath.startsWith("/api/v1/social/comments")) {
      return { comments: [] } as T;
    }
    const contentIdMatch = requestPath.match(/^\/api\/v1\/contents\/(\d+)$/);
    if (contentIdMatch) {
      detailCallCount += 1;
      const contentId = Number(contentIdMatch[1]);
      const found = DETAILS.get(contentId);
      if (found) return found as T;
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

/* jsdom 没有 TouchEvent 构造器；构造带 touches/changedTouches 的普通事件并冒泡。 */
function dispatchTouch(
  element: Element,
  type: "touchstart" | "touchend",
  x: number,
  y: number,
) {
  const event = new Event(type, { bubbles: true, cancelable: true }) as unknown as TouchEvent;
  const touchList = [{ clientX: x, clientY: y }];
  Object.defineProperty(event, type === "touchstart" ? "touches" : "changedTouches", {
    value: touchList,
  });
  act(() => {
    element.dispatchEvent(event);
  });
}

function HookHarness({ entryIndex }: { entryIndex: number }) {
  const { open, overlayElement } = useContentDetailOverlay({ source: "zone-page" });
  const triggerRef = useRef<HTMLButtonElement>(null);
  return (
    <>
      <button
        type="button"
        ref={triggerRef}
        onClick={() =>
          open(
            {
              contentId: CONTEXT_LIST[entryIndex].id,
              zone: "original",
              contextList: CONTEXT_LIST,
              contextIndex: entryIndex,
            },
            triggerRef.current,
          )
        }
      >
        Open context {entryIndex}
      </button>
      {overlayElement}
    </>
  );
}

function renderHarness(entryIndex: number, splitViewport = false) {
  installDom();
  installOverlayTestStubs({ splitViewport });
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <HookHarness entryIndex={entryIndex} />
    </IntlProvider>,
  );
}

function mediaScroller(which: "first" | "last" = "last"): HTMLElement {
  const covers = document.querySelectorAll<HTMLElement>('[data-slot="detail-cover"]');
  const scroller = covers[which === "last" ? covers.length - 1 : 0]?.firstElementChild as HTMLElement | null;
  assert.ok(scroller, "media scroller must exist");
  return scroller;
}

async function openContext(view: ReturnType<typeof render>, entryIndex: number) {
  const trigger = view.getByRole("button", { name: `Open context ${entryIndex}` });
  await act(async () => {
    fireEvent.click(trigger);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2 })));
}

/** 在最后一项媒体上向上滑动（移动端连续浏览手势；目标 = 移动端可见的行内画廊）。 */
function swipeUpOnMedia(which: "first" | "last" = "last") {
  const scroller = mediaScroller(which);
  dispatchTouch(scroller, "touchstart", 150, 260);
  dispatchTouch(scroller, "touchend", 160, 100);
}

/** 水平滑动翻到下一媒体（媒体集内翻页）。 */
function swipeLeftOnMedia(which: "first" | "last" = "last") {
  const scroller = mediaScroller(which);
  dispatchTouch(scroller, "touchstart", 150, 100);
  dispatchTouch(scroller, "touchend", 40, 105);
}

/** 当前媒体（aria-current）的图片地址：切篇后媒体区重置到新内容首项的断言锚点。 */
function currentMediaSrc(): string | null {
  return (
    document.querySelector('[aria-current="true"] img')?.getAttribute("src") ??
    document.querySelector('[aria-current="true"] video')?.getAttribute("src") ??
    null
  );
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

test("#89 AC1: trigger entries thread contextList/contextIndex into the overlay (via the shared hook)", async () => {
  installApiMock();
  const view = renderHarness(1);
  await openContext(view, 1);

  /* 触发的是第 2 项（B，双媒体）：标题直接显示 B；翻到末项再上滑 → C，
     证明 contextList/contextIndex 贯通到了浮层层内。 */
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context B" })));
  await waitFor(() => assert.ok(view.getAllByRole("group", { name: "1 / 2" })[0]));
  swipeLeftOnMedia();
  await waitFor(() => assert.ok(view.getAllByRole("group", { name: "2 / 2" })[0]));
  swipeUpOnMedia();
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context C" })));
});

test("#89 AC2: last-media upward swipe switches to the next context item and resets media + info state", async () => {
  installApiMock();
  const view = renderHarness(0);
  await openContext(view, 0);
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context A" })));
  await waitFor(() => assert.equal(currentMediaSrc(), "/seed-media/gallery/a-1.svg"));

  /* 上滑切到 B（双媒体），翻到第 2 项，再上滑切篇。 */
  swipeUpOnMedia();
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context B" })));
  await waitFor(() => assert.ok(view.getAllByRole("group", { name: "1 / 2" })[0]));
  swipeLeftOnMedia();
  await waitFor(() => assert.ok(view.getAllByRole("group", { name: "2 / 2" })[0]));

  const overlayScroller = document.querySelector<HTMLElement>('[data-slot="overlay-scroller"]');
  assert.ok(overlayScroller);
  overlayScroller.scrollTop = 80;

  swipeUpOnMedia();
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context C" })));

  /* 切篇后媒体区重置：回到新内容首项（媒体加载态随层 remount 重建）；信息区滚动回顶。 */
  await waitFor(() => assert.equal(currentMediaSrc(), "/seed-media/gallery/c-1.svg"));
  assert.equal(overlayScroller.scrollTop, 0, "info scroll resets after switching content");
  assert.ok(view.getByText("C body"));

  /* 连续浏览不压栈、不重复取数：A/B/C 各取一次 detail。 */
  assert.equal(detailCallCount, 3, "detail fetched once per switched content");
});

test("#89 AC3: at the end of the context list the end hint shows and no switch fires", async () => {
  installApiMock();
  const view = renderHarness(2);
  await openContext(view, 2);
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context C" })));
  await waitFor(() => assert.equal(currentMediaSrc(), "/seed-media/gallery/c-1.svg"));

  /* 列表最后一项：持续上滑只提示「已经到底了」，不切换内容。 */
  swipeUpOnMedia();
  await waitFor(() => assert.ok(view.getByText("You've reached the end")));
  swipeUpOnMedia();
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context C" })));
  assert.equal(detailCallCount, 1, "no refetch when the list is exhausted");
});

test("#89 AC4: desktop (>=1100px) never triggers continuous browsing", async () => {
  installApiMock();
  const view = renderHarness(0, true);
  await openContext(view, 0);
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context A" })));

  /* 桌面端：左栏媒体列（首个 detail-cover）上滑也不触发切篇。 */
  swipeUpOnMedia("first");
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Context A" })));
  assert.equal(detailCallCount, 1, "desktop must not switch content");
  assert.ok(view.queryByText("You've reached the end") === null);
});

test("#89 source contracts: context fields live on the entry, not as a second state machine", () => {
  const fs = requireForMocks("node:fs") as typeof import("node:fs");
  const hook = fs.readFileSync(
    requireForMocks("node:path").join(process.cwd(), "components/content/use-content-detail-overlay.tsx"),
    "utf8",
  );
  const layer = fs.readFileSync(
    requireForMocks("node:path").join(process.cwd(), "components/content/ContentDetailOverlayLayer.tsx"),
    "utf8",
  );
  const overlay = fs.readFileSync(
    requireForMocks("node:path").join(process.cwd(), "components/content/ContentDetailOverlay.tsx"),
    "utf8",
  );

  assert.match(hook, /contextList\?: ContentOverlayContextItem\[\]/);
  assert.match(hook, /contextList=\{entry\.contextList\}/);
  assert.match(layer, /onSwitchNext\?: \(entry: OverlayEntry\) => void/);
  assert.match(layer, /isMobileViewport/);
  assert.match(overlay, /switchTopLayer/);
  /* 连续浏览不复制关闭状态机，不压栈（复用既有 close/open 契约）。 */
  assert.doesNotMatch(overlay, /pushHistoryState\(.*switch/);
});
