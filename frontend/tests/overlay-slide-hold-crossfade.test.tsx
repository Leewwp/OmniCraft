import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { OverlayVariantLayout, type OverlayVariantLayoutProps } from "@/components/content/OverlayVariantLayout";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { testMessages } from "./runtime-test-helpers";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * E4 #956 hold-crossfade 就绪交换状态机（弱网零空白的组件级契约）：
 * - 翻页后新 slide 媒体未就绪：旧层保持满不透明（无淡出动画）、新层 opacity 0
 *   被压在下——不露空白；
 * - 就绪（图片 load / 视频 loadedmetadata / 查看器族挂载）后起跑 E2 双层
 *   交叉淡出，240ms 后旧层移除；
 * - 失败（error）同样放行交换（不留永久旧画面）；
 * - 快速连按任意时刻舞台 ≤ 2 层，最新切换接管保持层。
 * jsdom 图片永不自然加载——以 fireEvent.load/error 精确驱动就绪时序。
 * ──────────────────────────────────────────────────────────────────────────── */

const AREA = { width: 1000, height: 800 };

function installResizeObserverStub() {
  class ResizeObserverStub {
    cb: ResizeObserverCallback;
    constructor(cb: ResizeObserverCallback) {
      this.cb = cb;
    }
    observe = (target: Element) => {
      this.cb(
        [
          {
            target,
            contentRect: {
              ...AREA,
              x: 0,
              y: 0,
              top: 0,
              left: 0,
              right: AREA.width,
              bottom: AREA.height,
              toJSON() {},
            },
          } as unknown as ResizeObserverEntry,
        ],
        this as unknown as ResizeObserver,
      );
    };
    unobserve() {}
    disconnect() {}
  }
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = ResizeObserverStub;
}

function installDialogStubs() {
  const prototype = window.HTMLDialogElement?.prototype as unknown as HTMLDialogElement | undefined;
  if (!prototype) return;
  prototype.showModal = function showModalStub(this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  prototype.close = function closeStub(this: HTMLDialogElement) {
    this.removeAttribute("open");
  };
}

/** bucket.example 查看器族 fetch 桩（d6 同款）：确定性 404，不依赖 DNS。 */
function installSlideFetchStub() {
  const g = globalThis as unknown as { fetch?: typeof fetch };
  const original = g.fetch;
  g.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    if (String(input).includes("bucket.example")) {
      return new Response("stub-not-found", { status: 404 });
    }
    return original ? original(input, init) : Promise.reject(new Error("no fetch"));
  }) as typeof fetch;
  return {
    restore: () => {
      g.fetch = original;
    },
  };
}

function imageItem(id: number, overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return { id, url: `/e4-hold/img-${id}.jpg`, type: "image", width: 900, height: 1200, ...overrides };
}

type LayoutProps = Omit<OverlayVariantLayoutProps, "children"> & { children?: React.ReactNode };

function LayoutHarness(props: LayoutProps) {
  const { children, ...rest } = props;
  return (
    <IntlProvider locale="en" messages={testMessages}>
      <OverlayVariantLayout {...rest}>{children ?? <p>e4 harness body</p>}</OverlayVariantLayout>
    </IntlProvider>
  );
}

function renderLayout(props: LayoutProps) {
  installDom();
  installResizeObserverStub();
  installDialogStubs();
  return render(<LayoutHarness {...props} />);
}

function currentLayer(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="variant-media-pane"] > [data-slot="detail-cover"]');
}

function exitLayer(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="variant-media-pane"] > [data-slot="slide-fade-out"]');
}

function currentCanonicalImg(): HTMLImageElement | null {
  const img = currentLayer()?.querySelector<HTMLImageElement>("img:not([data-slot='cover-hold'])") ?? null;
  return img;
}

function countBadgeText(): string {
  return document.querySelector<HTMLElement>('[data-slot="media-count-badge"]')?.textContent ?? "";
}

async function paneNext() {
  const next = document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(next, "pane next arrow must exist");
  await act(async () => {
    fireEvent.click(next);
    await Promise.resolve();
  });
}

/** E2/E4 契约的 240ms 旧层移除是真实定时器：waitFor 兜住。 */
const settleTimer = () => new Promise((resolve) => setTimeout(resolve, 320));

let slideFetchStub: ReturnType<typeof installSlideFetchStub> | undefined;
test.beforeEach(() => {
  slideFetchStub = installSlideFetchStub();
});
test.afterEach(() => {
  slideFetchStub?.restore();
  slideFetchStub = undefined;
  cleanup();
});

test("E4 翻页新图未就绪：旧层保持（无淡出动画）、新层 opacity 0 不露空白", async () => {
  renderLayout({ media: [imageItem(1), imageItem(2), imageItem(3)], contentId: 9201, allowCopy: false });
  await waitFor(() => {
    if (!(countBadgeText().includes("1 / 3"))) throw new Error("waitFor: condition not met");
  });
  await paneNext();
  await waitFor(() => {
    if (!(exitLayer())) throw new Error("pending switch keeps the previous slide as the hold layer");
  });
  const exit = exitLayer()!;
  assert.equal(exit.style.animation, "", "hold layer runs no fade-out while the new media is pending");
  assert.equal(currentLayer()?.style.opacity, "0", "pending current layer stays covered");
  assert.equal(countBadgeText(), "2 / 3", "position updates immediately (interaction is never blocked)");
});

test("E4 就绪即交换：图片 load 后起跑交叉淡出并移除旧层", async () => {
  renderLayout({ media: [imageItem(1), imageItem(2)], contentId: 9201, allowCopy: false });
  await waitFor(() => {
    if (!(countBadgeText().includes("1 / 2"))) throw new Error("waitFor: condition not met");
  });
  await paneNext();
  assert.ok(exitLayer());
  await act(async () => {
    fireEvent.load(currentCanonicalImg()!);
    await Promise.resolve();
  });
  await waitFor(() => {
    if (!(exitLayer()?.style.animation.includes("overlay-slide-fade-out"))) throw new Error("waitFor: condition not met");
  });
  await settleTimer();
  await waitFor(() => {
    if (exitLayer()) throw new Error("ready swap removes the old layer after the exit window");
  });
  assert.equal(countBadgeText(), "2 / 2");
});

test("E4 失败放行：图片 error 不永久保持旧画面", async () => {
  renderLayout({ media: [imageItem(1), imageItem(2)], contentId: 9201, allowCopy: false });
  await waitFor(() => {
    if (!(countBadgeText().includes("1 / 2"))) throw new Error("waitFor: condition not met");
  });
  await paneNext();
  assert.ok(exitLayer());
  await act(async () => {
    fireEvent.error(currentCanonicalImg()!);
    await Promise.resolve();
  });
  await settleTimer();
  await waitFor(() => {
    if (exitLayer()) throw new Error("error releases the hold (never a stale-picture dead end)");
  });
});

test("E4 查看器族挂载即就绪：无需图载入即完成交换", async () => {
  renderLayout({
    media: [
      imageItem(1),
      {
        id: 2,
        url: "https://bucket.example/e4-hold/sheet.csv",
        type: "document",
        width: 300,
        height: 400,
        fileName: "sheet.csv",
        fileSize: 120,
      },
    ],
    contentId: 9201,
    allowCopy: false,
  });
  await waitFor(() => {
    if (!(countBadgeText().includes("1 / 2"))) throw new Error("waitFor: condition not met");
  });
  await paneNext();
  await waitFor(() => {
    if (!(exitLayer())) throw new Error("waitFor: condition not met");
  });
  await settleTimer();
  await waitFor(() => {
    if (exitLayer()) throw new Error("viewer-family slides are mount-ready");
  });
  assert.equal(countBadgeText(), "2 / 2");
});

test("E4 视频位就绪门：loadedmetadata 前保持，元数据到达即交换", async () => {
  renderLayout({
    media: [imageItem(1), { id: 2, url: "https://bucket.example/e4-hold/clip.mp4", type: "video", width: 640, height: 360 }],
    contentId: 9201,
    allowCopy: false,
  });
  await waitFor(() => {
    if (!(countBadgeText().includes("1 / 2"))) throw new Error("waitFor: condition not met");
  });
  await paneNext();
  await waitFor(() => {
    if (!(exitLayer())) throw new Error("video slide holds the old frame until metadata arrives");
  });
  const video = currentLayer()?.querySelector("video") ?? null;
  assert.ok(video, "pending current layer mounts the video element (loading underneath the hold)");
  await act(async () => {
    video!.dispatchEvent(new window.Event("loadedmetadata"));
    await Promise.resolve();
  });
  await settleTimer();
  await waitFor(() => {
    if (exitLayer()) throw new Error("metadata readiness completes the swap");
  });
});

test("E4 快速连按：保持层被最新切换接管，任意时刻 ≤ 2 层，最终单层", async () => {
  renderLayout({ media: [imageItem(1), imageItem(2), imageItem(3)], contentId: 9201, allowCopy: false });
  await waitFor(() => {
    if (!(countBadgeText().includes("1 / 3"))) throw new Error("waitFor: condition not met");
  });
  await paneNext();
  await paneNext();
  await waitFor(() => {
    if (!(exitLayer())) throw new Error("waitFor: condition not met");
  });
  const layers = document.querySelectorAll(
    '[data-slot="variant-media-pane"] > [data-slot="detail-cover"], [data-slot="variant-media-pane"] > [data-slot="slide-fade-out"]',
  ).length;
  assert.ok(layers <= 2, `at most 2 layers at any time (got ${layers})`);
  assert.equal(countBadgeText(), "3 / 3");
  /* 最新位的就绪驱动交换；中间位（从未就绪）不产生第二保持层。 */
  await act(async () => {
    fireEvent.load(currentCanonicalImg()!);
    await Promise.resolve();
  });
  await settleTimer();
  await waitFor(() => {
    if (document.querySelectorAll(
        '[data-slot="variant-media-pane"] > [data-slot="detail-cover"], [data-slot="variant-media-pane"] > [data-slot="slide-fade-out"]',
      ).length != 1) throw new Error("latest switch owns the cleanup; stage settles on a single layer");
  });
});

test("E4 首挂载无就绪门：无淡出层时当前层不隐藏（E2 入场语义不回归）", async () => {
  renderLayout({ media: [imageItem(1), imageItem(2)], contentId: 9201, allowCopy: false });
  await waitFor(() => {
    if (!(countBadgeText().includes("1 / 2"))) throw new Error("waitFor: condition not met");
  });
  assert.ok(!exitLayer(), "first mount renders no exit layer");
  assert.notEqual(currentLayer()?.style.opacity, "0", "current layer is visible without any readiness gate");
});
