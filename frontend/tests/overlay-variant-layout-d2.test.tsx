import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { OverlayVariantLayout, type OverlayVariantLayoutProps } from "@/components/content/OverlayVariantLayout";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { testMessages } from "./runtime-test-helpers";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D2 #859 统一版式媒体列契约（spec §3.1/§3.3/§9.3）：
 * - 几何时序：冻结窗口（入场动画期）迟到尺寸不得改列宽；解冻后恢复真实比例；
 *   无尺寸视频首帧按防御比例稳定占位，不得闪现后跳宽。
 * - 视频生命周期：活跃可见项 muted+playsInline+autoPlay；挂起（非顶层/查看器
 *   打开/登录浮窗）停播，恢复仅当前项；主体点击单次切播；controls 条点击不切播；
 *   视频不再进 MediaViewer（图片行为不变）。
 * - 翻页三件套：仅多项显示；边界 clamp；hover 与 focus-within 可见；触控 44px；
 *   键盘 ←/→ 只作用于顶层活动媒体区，不劫持输入框/上层查看器；
 *   切位按钮点击不触发放大。
 * 断言失败值绝不传 DOM 节点（node:test 序列化挂死），统一 Boolean()/字符串。
 * ──────────────────────────────────────────────────────────────────────────── */

/* jsdom 无布局：给 ResizeObserver 桩固定可用区（1000×800），paneWidth 公式
   在其上给出可区分像素值：3:4 → 600px；16:9 → 620px（cap=1000-380）。 */
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
  /* 组件代码经 tsx 在 Node 全局解析裸标识符：必须挂 globalThis 而非 window。 */
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = ResizeObserverStub;
}

interface MediaCall {
  op: "play" | "pause";
  key: string;
}

/** HTMLMediaElement play/pause 桩：记录调用序并同步翻转 paused，供切播断言。 */
function installMediaElementStubs() {
  const calls: MediaCall[] = [];
  const proto = window.HTMLMediaElement.prototype as unknown as {
    play: () => Promise<void>;
    pause: () => void;
  };
  const originalPlay = proto.play;
  const originalPause = proto.pause;
  const setPaused = (el: HTMLMediaElement, value: boolean) => {
    /* jsdom 的 paused 是只读 getter：用实例 defineProperty 影子覆写。 */
    Object.defineProperty(el, "paused", { value, configurable: true, writable: true });
  };
  proto.play = function playStub(this: HTMLMediaElement) {
    calls.push({ op: "play", key: mediaKey(this) });
    setPaused(this, false);
    return Promise.resolve();
  };
  proto.pause = function pauseStub(this: HTMLMediaElement) {
    calls.push({ op: "pause", key: mediaKey(this) });
    setPaused(this, true);
  };
  return {
    calls,
    restore: () => {
      /* 恢复原型原始方法（afterEach unmount 的 effect 清理仍可能调用 pause）。 */
      proto.play = originalPlay;
      proto.pause = originalPause;
    },
  };
}

function mediaKey(el: HTMLMediaElement): string {
  return el.getAttribute("data-d2-video-key") ?? el.getAttribute("src") ?? "unknown";
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

function videoItem(overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return {
    id: 1,
    url: "https://bucket.example/d2/pane-video-1009.mp4",
    type: "video",
    width: undefined,
    height: undefined,
    ...overrides,
  };
}

function imageItem(id: number, overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return {
    id,
    url: `/seed-media/d2/pane-image-${id}.svg`,
    type: "image",
    width: 900,
    height: 1200,
    ...overrides,
  };
}

type LayoutProps = Omit<OverlayVariantLayoutProps, "children"> & { children?: React.ReactNode };

function LayoutHarness({
  media,
  freezeGeometry,
  active,
  onMediaDimensions,
  children,
}: LayoutProps & { active?: boolean }) {
  return (
    <IntlProvider locale="en" messages={testMessages}>
      <div>
        <input data-testid="d2-input" aria-label="comment box" />
        <OverlayVariantLayout
          media={media}
          active={active}
          freezeGeometry={freezeGeometry}
          onMediaDimensions={onMediaDimensions}
        >
          {children ?? <p>d2 harness body</p>}
        </OverlayVariantLayout>
      </div>
    </IntlProvider>
  );
}

function renderLayout(props: LayoutProps & { active?: boolean }) {
  installDom();
  installResizeObserverStub();
  installDialogStubs();
  /* Provider 放进 Harness 内部：RTL 的 rerender 不重新套外部包装，
     rerender 后 useTranslations 必须仍有 IntlContext。 */
  return render(<LayoutHarness {...props} />);
}

function paneElement(): HTMLElement {
  const pane = document.querySelector<HTMLElement>('[data-slot="variant-media-pane"]');
  assert.ok(pane, "media pane must render");
  return pane;
}

function paneWidth(): string {
  return paneElement().style.width;
}

function countBadgeText(): string {
  const badge = document.querySelector<HTMLElement>('[data-slot="media-count-badge"]');
  return badge?.textContent ?? "";
}

function viewerDialogCount(): number {
  return document.querySelectorAll("dialog[open]").length;
}

function pressArrowKey(key: "ArrowLeft" | "ArrowRight", target?: Element) {
  const event = new window.KeyboardEvent("keydown", { key, bubbles: true, cancelable: true });
  act(() => {
    (target ?? window).dispatchEvent(event);
  });
  return event;
}

test.afterEach(() => cleanup());

/* ── 几何时序（两段式根除） ─────────────────────────────────────────────── */

test("D2 冻结窗口：迟到尺寸不改列宽（入场动画零跳宽），解冻后恢复真实比例", async () => {
  const view = renderLayout({
    media: [videoItem()],
    freezeGeometry: true,
  });
  await waitFor(() => assert.ok(paneWidth() !== "", "pane measured"));
  assert.equal(paneWidth(), "600px", "dims-less video holds the defensive 3:4 geometry");

  /* 冻结窗口内元数据迟到：列宽必须保持，不得在入场动画中跳宽。 */
  await act(async () => {
    view.rerender(
      <LayoutHarness media={[videoItem({ width: 1600, height: 900 })]} freezeGeometry={true} />,
    );
  });
  assert.equal(paneWidth(), "600px", "frozen window must not apply late dimensions");

  /* 解冻（入场落定）后恢复真实比例（16:9 触发 380px 右栏保留上限 → 620px）。 */
  await act(async () => {
    view.rerender(
      <LayoutHarness
        media={[videoItem({ width: 1600, height: 900 })]}
        freezeGeometry={false}
      />
      ,
    );
  });
  assert.equal(paneWidth(), "620px", "post-settle the live true ratio applies");
});

test("D2 已知尺寸直接就绪：冻结起跑即锁真实比例，全程无 3:4 闪现", async () => {
  renderLayout({
    media: [videoItem({ width: 1600, height: 900 })],
    freezeGeometry: true,
  });
  await waitFor(() => assert.ok(paneWidth() !== ""));
  assert.equal(paneWidth(), "620px", "known dims lock the true ratio from the frozen first frame");
});

/* ── 视频生命周期 ───────────────────────────────────────────────────────── */

function setupLifecycleTest() {
  const mediaStubs = installMediaElementStubs();
  const view = renderLayout({ media: [videoItem({ width: 1600, height: 900 })] });
  const video = document.querySelector("video");
  assert.ok(video, "video element must render in the pane");
  return { mediaStubs, view, video: video as HTMLVideoElement };
}

test("D2 活跃可见视频：muted + playsInline + autoPlay 且挂载即自动播放", async () => {
  const { mediaStubs, video } = setupLifecycleTest();
  try {
    await waitFor(() => assert.ok(mediaStubs.calls.some((c) => c.op === "play")));
    assert.equal(video.muted, true, "autoplay must be muted (browser policy compliant)");
    assert.equal(video.hasAttribute("playsinline"), true, "playsinline keeps inline playback");
    assert.equal(video.hasAttribute("autoplay"), true, "active video autoplays");
    assert.equal(
      mediaStubs.calls.filter((c) => c.op === "play").length,
      1,
      "exactly one autoplay attempt on mount",
    );
  } finally {
    mediaStubs.restore();
  }
});

test("D2 挂起矩阵：非顶层压层停播，恢复仅当前项续播（不自动起播多次）", async () => {
  const { mediaStubs, view } = setupLifecycleTest();
  try {
    await waitFor(() => assert.ok(mediaStubs.calls.some((c) => c.op === "play")));
    await act(async () => {
      view.rerender(<LayoutHarness media={[videoItem({ width: 1600, height: 900 })]} active={false} />);
    });
    const playsBefore = mediaStubs.calls.filter((c) => c.op === "play").length;
    assert.ok(
      mediaStubs.calls.some((c) => c.op === "pause"),
      "being pushed under a new layer pauses the hidden media",
    );
    await act(async () => {
      view.rerender(<LayoutHarness media={[videoItem({ width: 1600, height: 900 })]} active={true} />);
    });
    assert.ok(
      mediaStubs.calls.filter((c) => c.op === "play").length > playsBefore,
      "popping back resumes only the current item",
    );
  } finally {
    mediaStubs.restore();
  }
});

test("D2 主体点击单次切播；controls 条区域点击不切播；视频不进查看器", async () => {
  const { mediaStubs, video } = setupLifecycleTest();
  try {
    await waitFor(() => assert.ok(mediaStubs.calls.some((c) => c.op === "play")));
    const opsAfterMount = mediaStubs.calls.length;

    await act(async () => {
      fireEvent.click(video);
    });
    assert.equal(
      mediaStubs.calls.length,
      opsAfterMount + 1,
      "one body click toggles exactly once (no double fire)",
    );
    assert.equal(mediaStubs.calls[mediaStubs.calls.length - 1]?.op, "pause");

    await act(async () => {
      fireEvent.click(video);
    });
    assert.equal(mediaStubs.calls.length, opsAfterMount + 2);
    assert.equal(mediaStubs.calls[mediaStubs.calls.length - 1]?.op, "play");

    /* controls 条（底部 44px）点击交给原生控件：不切播、不冒泡成主体点击。 */
    const anchor = document.querySelector<HTMLElement>('[data-slot="detail-cover"]');
    assert.ok(anchor);
    const originalRect = anchor.getBoundingClientRect.bind(anchor);
    anchor.getBoundingClientRect = () =>
      ({ height: 500, top: 0, bottom: 500, width: 600, left: 0, right: 600, x: 0, y: 0, toJSON() {} }) as DOMRect;
    const opsBeforeStrip = mediaStubs.calls.length;
    await act(async () => {
      fireEvent.click(video, { clientY: 480 });
    });
    assert.equal(
      mediaStubs.calls.length,
      opsBeforeStrip,
      "clicks inside the native controls strip must not toggle playback",
    );
    anchor.getBoundingClientRect = originalRect;

    assert.equal(viewerDialogCount(), 0, "video body click must never open MediaViewer");
  } finally {
    mediaStubs.restore();
  }
});

test("D2 视频挂载元数据回填：onMediaDimensions 回调上报真实尺寸（探针失败后的迟到修正）", async () => {
  installMediaElementStubs();
  const reported: Array<{ id: number; width: number; height: number }> = [];
  renderLayout({
    media: [videoItem()],
    onMediaDimensions: (id, width, height) => reported.push({ id, width, height }),
  });
  const video = document.querySelector("video");
  assert.ok(video);
  Object.defineProperty(video, "videoWidth", { value: 1280, configurable: true });
  Object.defineProperty(video, "videoHeight", { value: 720, configurable: true });
  await act(async () => {
    fireEvent.loadedMetadata(video);
  });
  await waitFor(() => assert.ok(reported.length === 1));
  assert.deepEqual(reported[0], { id: 1, width: 1280, height: 720 });
});

/* ── 翻页三件套 ─────────────────────────────────────────────────────────── */

test("D2 三件套：多项显示、边界 clamp 不循环、hover/focus-within 可见、触控 44px", async () => {
  const view = renderLayout({ media: [imageItem(1), imageItem(2), imageItem(3)] });
  await waitFor(() => assert.ok(countBadgeText() !== ""));

  const previous = document.querySelector<HTMLButtonElement>('button[aria-label="Previous media"]');
  const next = document.querySelector<HTMLButtonElement>('button[aria-label="Next media"]');
  assert.ok(previous && next, "arrows render for multi-item media");
  assert.equal(countBadgeText(), "1 / 3", "count badge shows current/total");
  assert.ok(document.querySelector('[data-slot="media-paging-dots"]'), "bottom dots render");

  /* 可见性契约：hover 与 focus-within 都能显现（半透明箭头+徽标），默认只显圆点。 */
  const arrowClass = `${previous?.className ?? ""} ${next?.className ?? ""}`;
  assert.ok(arrowClass.includes("group-hover/media:opacity-100"), "arrows appear on pane hover");
  assert.ok(arrowClass.includes("group-focus-within/media:opacity-100"), "arrows appear on focus-within");
  const badgeClass = document.querySelector<HTMLElement>('[data-slot="media-count-badge"]')?.className ?? "";
  assert.ok(badgeClass.includes("group-hover/media:opacity-100"), "badge appears on pane hover");
  assert.ok(badgeClass.includes("group-focus-within/media:opacity-100"), "badge appears on focus-within");
  assert.ok(arrowClass.includes("h-11") && arrowClass.includes("w-11"), "arrow touch target is 44px");

  /* 边界 clamp：first 不可再往前，last 不可再往后，不循环。 */
  assert.ok(previous?.disabled, "previous disabled at the first item");
  await act(async () => {
    fireEvent.click(next!);
    fireEvent.click(next!);
  });
  assert.equal(countBadgeText(), "3 / 3");
  assert.ok(next?.disabled, "next disabled at the last item (clamp, no loop)");
  await act(async () => {
    fireEvent.click(previous!);
  });
  assert.equal(countBadgeText(), "2 / 3");

  /* 切位按钮点击不触发放大（事件不穿透到主体点击）。 */
  assert.equal(viewerDialogCount(), 0, "paging controls must not open the viewer");
});

test("D2 单项媒体不渲染任何翻页控件", async () => {
  renderLayout({ media: [imageItem(1)] });
  await waitFor(() => assert.ok(document.querySelector('[data-slot="detail-cover"]')));
  assert.equal(document.querySelectorAll('button[aria-label="Previous media"]').length, 0);
  assert.equal(document.querySelectorAll('button[aria-label="Next media"]').length, 0);
  assert.equal(document.querySelector('[data-slot="media-count-badge"]'), null, "no badge for a single item");
});

test("D2 键盘 ←/→：只作用于顶层活动媒体区，clamp 到边界，不劫持输入框与上层查看器", async () => {
  const mediaStubs = installMediaElementStubs();
  try {
    const view = renderLayout({ media: [imageItem(1), imageItem(2), imageItem(3)] });
    await waitFor(() => assert.ok(countBadgeText() !== ""));

    pressArrowKey("ArrowRight");
    assert.equal(countBadgeText(), "2 / 3", "ArrowRight pages forward");
    pressArrowKey("ArrowLeft");
    assert.equal(countBadgeText(), "1 / 3", "ArrowLeft pages backward");
    pressArrowKey("ArrowLeft");
    assert.equal(countBadgeText(), "1 / 3", "ArrowLeft clamps at the first item");

    /* 输入框聚焦时不劫持：光标移动语义留给输入框。 */
    const input = document.querySelector<HTMLInputElement>('[data-testid="d2-input"]');
    assert.ok(input);
    await act(async () => {
      input.focus();
    });
    pressArrowKey("ArrowRight", input);
    assert.equal(countBadgeText(), "1 / 3", "arrow keys must not page while an input is focused");

    /* 上层查看器打开时不劫持：查看器内翻页语义独立。 */
    const cover = document.querySelector<HTMLElement>('[data-slot="detail-cover"]');
    assert.ok(cover);
    await act(async () => {
      fireEvent.click(cover);
    });
    await waitFor(() => assert.ok(viewerDialogCount() === 1, "image click still opens the viewer"));
    pressArrowKey("ArrowRight");
    assert.equal(countBadgeText(), "1 / 3", "arrow keys must not page the pane while the viewer is open");

    /* 非顶层（压层）媒体区不响应键盘。 */
    await act(async () => {
      view.rerender(<LayoutHarness media={[imageItem(1), imageItem(2), imageItem(3)]} active={false} />);
    });
    pressArrowKey("ArrowRight");
    assert.equal(countBadgeText(), "1 / 3", "a hidden layer's media area is not keyboard active");
  } finally {
    mediaStubs.restore();
  }
});
