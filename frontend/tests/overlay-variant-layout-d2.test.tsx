import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { OverlayVariantLayout, type OverlayVariantLayoutProps } from "@/components/content/OverlayVariantLayout";
import type { MediaGalleryItem } from "@/components/content/MediaGallery";
import { testMessages } from "./runtime-test-helpers";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D2 #859 统一版式媒体列契约（spec §3.1/§3.3/§9.3）+ E1 #953 几何引擎：
 * - 几何（E1）：按内容冻结（宽度只随首 slide 比例与档位变化，内容内翻页零
 *   变化）；文档族 slide = reading 宽舞台档（pane data-tier）；首帧宽度即
 *   终值（px 无百分比兜底）；首项迟到回填一次平滑修正；几何规格上报
 *   （onGeometrySpec：壳宽 = 媒体列 + 固定右列 440）。
 * - 视频生命周期：活跃可见项 muted+playsInline+autoPlay；挂起（非顶层/查看器
 *   打开/登录浮窗）停播，恢复仅当前项；主体点击单次切播；controls 条点击不切播；
 *   视频不再进 MediaViewer（图片行为不变）。
 * - 翻页三件套：仅多项显示；边界 clamp；hover 与 focus-within 可见；触控 44px；
 *   键盘 ←/→ 只作用于顶层活动媒体区，不劫持输入框/上层查看器；
 *   切位按钮点击不触发放大。
 * 断言失败值绝不传 DOM 节点（node:test 序列化挂死），统一 Boolean()/字符串。
 * ──────────────────────────────────────────────────────────────────────────── */

/* jsdom 无布局：ResizeObserver 桩固定根区（1000×800，高为几何基准）；
   视口桩 1440 → 壳宽上限 min(1320,1408)=1320、可用宽 1320-440=880。
   宽度期望值：3:4 → 800×0.75=600；1:1 → 800；16:9 → clamp 880；9:16 → 450。 */
const AREA = { width: 1000, height: 800 };
const VIEWPORT_WIDTH = 1440;

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
  /* 视口桩：宽度推导的上限基准（computeOverlayGeometry 的 viewportWidth）。 */
  Object.defineProperty(window, "innerWidth", { value: VIEWPORT_WIDTH, configurable: true, writable: true });
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
  onGeometrySpec,
  active,
  onMediaDimensions,
  exitResetToken,
  children,
}: LayoutProps & { active?: boolean; exitResetToken?: number }) {
  return (
    <IntlProvider locale="en" messages={testMessages}>
      <div>
        <input data-testid="d2-input" aria-label="comment box" />
        <OverlayVariantLayout
          media={media}
          active={active}
          onGeometrySpec={onGeometrySpec}
          onMediaDimensions={onMediaDimensions}
          exitResetToken={exitResetToken}
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

function paneTier(): string {
  return paneElement().dataset.tier ?? "";
}

function paneTransition(): string {
  return paneElement().style.transition;
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

/* ── 几何引擎（E1 #953：按内容冻结 + 阅读宽舞台档 + 首帧即终值） ────────── */

test("E1 按内容冻结：混排比例内容翻页宽度零变化（3:4/1:1/9:16）", async () => {
  const media = [
    imageItem(1, { width: 900, height: 1200 }),
    imageItem(2, { width: 800, height: 800 }),
    imageItem(3, { width: 720, height: 1280 }),
  ];
  renderLayout({ media });
  await waitFor(() => assert.ok(paneWidth() !== "", "pane measured"));
  assert.equal(paneWidth(), "600px", "冻结比例 = 首 slide 3:4（800×0.75）");
  assert.equal(paneTier(), "visual", "image 位 = visual 档");

  /* 跨比例翻页：宽度读数零变化（后续 slide 比例不参与几何）。 */
  await act(async () => {
    pressArrowKey("ArrowRight");
  });
  assert.equal(paneWidth(), "600px", "1:1 位宽度不变（按内容冻结）");
  await act(async () => {
    pressArrowKey("ArrowRight");
  });
  assert.equal(paneWidth(), "600px", "9:16 位宽度不变（按内容冻结）");
  await act(async () => {
    pressArrowKey("ArrowLeft");
  });
  assert.equal(paneWidth(), "600px", "翻回首页宽度仍不变");
});

test("E1 阅读宽舞台档：文档族 slide 取满可用宽，档位切换一次（visual↔reading）", async () => {
  const docSlide: MediaGalleryItem = {
    id: 90,
    url: "https://bucket.example/e1/sheet.csv",
    type: "document",
    fileName: "sheet.csv",
    width: 300,
    height: 400,
  };
  renderLayout({ media: [imageItem(1, { width: 900, height: 1200 }), docSlide] });
  await waitFor(() => assert.ok(paneWidth() !== ""));
  assert.equal(paneWidth(), "600px", "首 slide 3:4 → visual 档 600px");
  assert.equal(paneTier(), "visual");

  await act(async () => {
    pressArrowKey("ArrowRight");
  });
  assert.equal(paneTier(), "reading", "document 位 = reading 档");
  assert.equal(paneWidth(), "880px", "宽舞台 = 可用宽（1320-440）");

  await act(async () => {
    pressArrowKey("ArrowLeft");
  });
  assert.equal(paneTier(), "visual", "翻回影像位回 visual 档");
  assert.equal(paneWidth(), "600px", "档位切换一次，无逐张漂移");
});

test("E1 首帧宽度即终值（px 无百分比兜底）+ 宽度过渡常量与测量落地帧语义", async () => {
  renderLayout({ media: [imageItem(1, { width: 900, height: 1200 })] });
  await waitFor(() => assert.ok(paneWidth() !== "", "first frame already carries px width"));
  assert.equal(paneWidth(), "600px", "测量落地帧即终值（无 58% 兜底路径）");
  /* 测量落地帧关闭过渡（亚像素修正不产生动画），随后启用 240ms SP-12 过渡
     （rAF 定时器由 act 异步泵送）。 */
  await waitFor(() => assert.ok(paneTransition().startsWith("width 240ms"), "settled frames enable the 240ms width transition"));
});

test("E1 首项迟到回填：宽度随新冻结比例一次修正（后续位回填不参与几何）", async () => {
  const view = renderLayout({ media: [videoItem()] });
  await waitFor(() => assert.ok(paneWidth() !== "", "pane measured"));
  assert.equal(paneWidth(), "600px", "dims-less video holds the defensive 3:4 geometry");

  /* 首 slide 元数据迟到回填（探针超时后的 loadedmetadata 修正）：按新冻结
     比例一次修正（16:9 → clamp 到可用宽 880）。 */
  await act(async () => {
    view.rerender(<LayoutHarness media={[videoItem({ width: 1600, height: 900 })]} />);
  });
  assert.equal(paneWidth(), "880px", "first-slide backfill re-derives the frozen ratio once");

  /* 非首项回填：几何零变化（宽度只依赖首 slide）。 */
  await act(async () => {
    view.rerender(
      <LayoutHarness
        media={[
          videoItem({ width: 1600, height: 900 }),
          imageItem(2, { width: 720, height: 1280 }),
        ]}
      />,
    );
  });
  assert.equal(paneWidth(), "880px", "later slides' dims never touch the geometry");
});

test("E1 几何规格上报：壳宽 = 媒体列 + 固定右列 440，同档翻页零重复上报", async () => {
  const specs: Array<{ tier: string; paneWidth: number; shellWidth: number }> = [];
  const view = renderLayout({
    media: [imageItem(1, { width: 900, height: 1200 }), imageItem(2, { width: 720, height: 1280 })],
    onGeometrySpec: (spec) => specs.push({ tier: spec.tier, paneWidth: spec.paneWidth, shellWidth: spec.shellWidth }),
  });
  await waitFor(() => assert.ok(specs.length >= 1, "geometry spec reported once measured"));
  assert.equal(specs[0].tier, "visual");
  assert.equal(specs[0].paneWidth, 600);
  assert.equal(specs[0].shellWidth, 1040, "shell width = pane + 440");

  const reportsBefore = specs.length;
  await act(async () => {
    pressArrowKey("ArrowRight");
  });
  assert.equal(paneWidth(), "600px");
  assert.equal(specs.length, reportsBefore, "same-tier paging reports nothing");

  /* 非活动（压层）布局不上报；恢复活动后按需重报。 */
  await act(async () => {
    view.rerender(
      <LayoutHarness
        media={[imageItem(1, { width: 900, height: 1200 }), imageItem(2, { width: 720, height: 1280 })]}
        active={false}
        onGeometrySpec={(spec) => specs.push({ tier: spec.tier, paneWidth: spec.paneWidth, shellWidth: spec.shellWidth })}
      />,
    );
  });
  const countAfterInactive = specs.length;
  await act(async () => {
    pressArrowKey("ArrowRight");
  });
  assert.equal(specs.length, countAfterInactive, "inactive layer never reports");
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

/* ── E2 #954：舞台 slide 双层交叉淡出 + 关窗回源令牌 ─────────────────────── */

function stageLayerCount(): number {
  return document.querySelectorAll(
    '[data-slot="variant-media-pane"] > [data-slot="detail-cover"], [data-slot="variant-media-pane"] > [data-slot="slide-fade-out"]',
  ).length;
}

function exitLayer(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="slide-fade-out"]');
}

test("E2 交叉淡出：翻页 = 1 新层 + 1 淡出层，240ms 后收敛回单层", async () => {
  renderLayout({ media: [imageItem(1), imageItem(2), imageItem(3)] });
  await waitFor(() => assert.ok(countBadgeText() !== ""));
  assert.equal(stageLayerCount(), 1, "初始舞台单层");

  await act(async () => {
    pressArrowKey("ArrowRight");
  });
  assert.equal(countBadgeText(), "2 / 3");
  assert.equal(stageLayerCount(), 2, "翻页瞬间 = 新层 + 淡出层（双层真交叉淡出）");
  const exit = exitLayer();
  assert.ok(exit, "exit layer renders");
  assert.equal(exit?.getAttribute("aria-hidden"), "true", "exit layer is aria-hidden");
  assert.ok(exit?.className.includes("pointer-events-none"), "exit layer never intercepts interactions");
  assert.ok(
    (exit?.style.animation ?? "").includes("overlay-slide-fade-out"),
    `exit layer plays the fade-out animation (got ${exit?.style.animation})`,
  );
  const current = document.querySelector<HTMLElement>('[data-slot="variant-media-pane"] > [data-slot="detail-cover"]');
  assert.ok(
    (current?.style.animation ?? "").includes("overlay-slide-fade-in"),
    `incoming layer plays the fade-in animation (got ${current?.style.animation})`,
  );
  /* 旧层移除窗口 = 发号牌定时器（240ms）。 */
  await waitFor(() => assert.equal(stageLayerCount(), 1, "fade-out layer is removed after the exit window"));
  assert.equal(exitLayer(), null, "exit layer unmounts");
});

test("E2 快速连按：任意时刻舞台 ≤2 层，最新淡出层接管清理权，动画结束单层无残留", async () => {
  renderLayout({ media: [imageItem(1), imageItem(2), imageItem(3)] });
  await waitFor(() => assert.ok(countBadgeText() !== ""));

  await act(async () => {
    pressArrowKey("ArrowRight");
    pressArrowKey("ArrowRight");
  });
  assert.equal(countBadgeText(), "3 / 3", "连按两步到位");
  assert.ok(stageLayerCount() <= 2, `任意时刻至多 2 层（1 新 + 1 淡出中），got ${stageLayerCount()}`);
  assert.equal(stageLayerCount(), 2, "最新一次切换的旧层仍在淡出");
  /* 再连按回翻：第三次切换接管清理权，第一份淡出层立即被替换（无叠层残留）。 */
  await act(async () => {
    pressArrowKey("ArrowLeft");
  });
  assert.ok(stageLayerCount() <= 2, `中断再切换仍 ≤2 层，got ${stageLayerCount()}`);
  await waitFor(() => assert.equal(stageLayerCount(), 1, "连按结束后舞台收敛单层"));
  assert.equal(countBadgeText(), "2 / 3");
});

test("E2 关窗回源令牌：翻离首屏后 token 递增 → 瞬时回 slide 0 档位并重报几何（无 240ms 过渡）", async () => {
  const docSlide: MediaGalleryItem = {
    id: 90,
    url: "https://bucket.example/e2/sheet.csv",
    type: "document",
    fileName: "sheet.csv",
    width: 300,
    height: 400,
  };
  const specs: Array<{ tier: string; paneWidth: number; shellWidth: number }> = [];
  const view = renderLayout({
    media: [imageItem(1, { width: 900, height: 1200 }), docSlide],
    onGeometrySpec: (spec) => specs.push({ tier: spec.tier, paneWidth: spec.paneWidth, shellWidth: spec.shellWidth }),
  });
  await waitFor(() => assert.ok(paneWidth() !== ""));
  assert.equal(paneWidth(), "600px", "开窗 visual 档（3:4）");

  await act(async () => {
    pressArrowKey("ArrowRight");
  });
  assert.equal(paneTier(), "reading", "翻到文档页 = reading 档");
  assert.equal(paneWidth(), "880px", "宽舞台 880");

  /* 关窗回源令牌：active 已关（关窗态），常规上报门关闭——重置路径必须自行
     重报 slide 0 档位规格（morphBack 测量稳定）。 */
  const reportsBefore = specs.length;
  await act(async () => {
    view.rerender(
      <LayoutHarness
        media={[imageItem(1, { width: 900, height: 1200 }), docSlide]}
        active={false}
        exitResetToken={1}
        onGeometrySpec={(spec) => specs.push({ tier: spec.tier, paneWidth: spec.paneWidth, shellWidth: spec.shellWidth })}
      />,
    );
  });
  assert.equal(countBadgeText(), "1 / 2", "舞台瞬时回 slide 0");
  assert.equal(paneTier(), "visual", "档位瞬时回源（reading → visual）");
  assert.equal(paneWidth(), "600px", "宽度瞬时回源（无 240ms 档位动画，morphBack 测量稳定）");
  assert.equal(stageLayerCount(), 2, "回源以交叉淡出呈现（封面态淡入、文档位淡出）");
  assert.ok(specs.length > reportsBefore, "reset re-reports the geometry spec despite inactive");
  const lastSpec = specs[specs.length - 1];
  assert.equal(lastSpec.tier, "visual");
  assert.equal(lastSpec.shellWidth, 1040, "重报壳宽 = 媒体列 600 + 固定右列 440");

  /* 两帧后恢复常规宽度过渡。 */
  await waitFor(() => assert.ok(paneTransition().startsWith("width 240ms"), "exit instant window ends"));
  await view.unmount();
});
