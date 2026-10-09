import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { MediaGallery, type MediaGalleryItem } from "@/components/content/MediaGallery";
import { act, cleanup, fireEvent, installDom, renderWithIntl, waitFor } from "./runtime-test-helpers";

/* ────────────────────────────────────────────────────────────────────────────
 * D2 #859 MediaGallery 视频契约同步（spec §9.3「移动 MediaGallery 路径同样修复，
 * 宿主详情页既有契约单独回归」）：
 * - 视频项有正数尺寸 → 容器 aspect-ratio 按真实比例稳定（根除两段式：元数据
 *   前后容器几何不变）；缺尺寸维持自然流（既有防御不回归）；
 * - 活跃视频 muted+playsInline+autoPlay，非活跃停播（任何时刻至多一段播放）；
 * - 视频主体点击 = 单次切播（controls 条除外），不再打开 MediaViewer；
 *   图片点击查看器契约不回归。
 * 断言失败值绝不传 DOM 节点（node:test 序列化挂死），统一 Boolean()/字符串。
 * ──────────────────────────────────────────────────────────────────────────── */

function makeItem(id: number, overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return {
    id,
    url: `/seed-media/d2/gallery-item-${id}.jpg`,
    type: "image",
    width: 1000,
    height: 750,
    ...overrides,
  };
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

interface MediaCall {
  op: "play" | "pause";
  src: string;
}

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
    calls.push({ op: "play", src: this.getAttribute("src") ?? "" });
    setPaused(this, false);
    return Promise.resolve();
  };
  proto.pause = function pauseStub(this: HTMLMediaElement) {
    calls.push({ op: "pause", src: this.getAttribute("src") ?? "" });
    setPaused(this, true);
  };
  return {
    calls,
    restore: () => {
      proto.play = originalPlay;
      proto.pause = originalPause;
    },
  };
}

function scroller(container: HTMLElement): HTMLElement {
  const el = container.querySelector<HTMLElement>('[data-slot="detail-cover"] > div');
  assert.ok(el, "media scroller must exist");
  return el;
}

test.afterEach(() => cleanup());

test.beforeEach(() => {
  installDom();
  installDialogStubs();
  /* 全文件统一播放桩：几何用例也会触发活跃视频自动播放（消 notImplemented 噪声）。 */
  installMediaElementStubs();
});

test("D2 视频容器几何：有正数尺寸按真实比例稳定，不再两段式；缺尺寸维持自然流", () => {
  const dimmed = renderWithIntl(
    <MediaGallery
      items={[makeItem(1, { type: "video", url: "/seed-media/d2/v1.mp4", width: 1280, height: 720 })]}
    />,
  );
  /* React 把数值 aspectRatio 序列化为 "1.7777... / 1"——按数值比较。 */
  assert.equal(
    Number.parseFloat(scroller(dimmed.container).style.aspectRatio),
    1280 / 720,
    "known dims pin the container to the true video ratio before metadata arrives",
  );
  cleanup();

  const legacy = renderWithIntl(
    <MediaGallery
      items={[makeItem(2, { type: "video", url: "/seed-media/d2/v2.mp4", width: undefined, height: undefined })]}
    />,
  );
  assert.equal(
    scroller(legacy.container).style.aspectRatio,
    "",
    "legacy dims-less videos keep the natural-flow container (no fabricated ratio)",
  );
});

test("D2 活跃画廊视频 muted+playsInline+autoPlay；非活跃项停播；翻页停旧播新", async () => {
  const mediaStubs = installMediaElementStubs();
  try {
    const { container } = renderWithIntl(
      <MediaGallery
        items={[
          makeItem(1, { type: "video", url: "/seed-media/d2/gv1.mp4" }),
          makeItem(2, { type: "video", url: "/seed-media/d2/gv2.mp4" }),
        ]}
      />,
    );
    const videos = container.querySelectorAll("video");
    assert.equal(videos.length, 2);
    const active = videos[0] as HTMLVideoElement;
    assert.equal(active.muted, true, "gallery autoplay is muted");
    assert.equal(active.hasAttribute("playsinline"), true);
    await waitFor(() =>
      assert.ok(
        mediaStubs.calls.some((c) => c.op === "play" && c.src === "/seed-media/d2/gv1.mp4"),
        "active gallery video attempts muted autoplay",
      ),
    );

    await act(async () => {
      fireEvent.click(container.querySelector('button[aria-label="Next media"]') as Element);
    });
    assert.ok(
      mediaStubs.calls.some((c) => c.op === "pause" && c.src === "/seed-media/d2/gv1.mp4"),
      "paging away pauses the previous video",
    );
    await waitFor(() =>
      assert.ok(
        mediaStubs.calls.some((c) => c.op === "play" && c.src === "/seed-media/d2/gv2.mp4"),
        "newly active video starts muted playback",
      ),
    );
  } finally {
    mediaStubs.restore();
  }
});

test("D2 画廊视频主体点击单次切播（controls 条除外）且不进查看器；图片查看器契约不回归", async () => {
  const mediaStubs = installMediaElementStubs();
  try {
    const { container } = renderWithIntl(
      <MediaGallery items={[makeItem(1, { type: "video", url: "/seed-media/d2/toggle.mp4" })]} />,
    );
    const video = container.querySelector("video") as HTMLVideoElement;
    assert.ok(Boolean(video));
    await waitFor(() => assert.ok(mediaStubs.calls.some((c) => c.op === "play")));
    const opsAfterMount = mediaStubs.calls.length;

    await act(async () => {
      fireEvent.click(video);
    });
    assert.equal(mediaStubs.calls.length, opsAfterMount + 1, "one click toggles exactly once");
    assert.equal(mediaStubs.calls[mediaStubs.calls.length - 1]?.op, "pause");
    assert.equal(document.querySelectorAll("dialog[open]").length, 0, "video click never opens the viewer");

    /* controls 条区域：原生控件语义，不切播。handler 的 currentTarget = 项包装
       div（onClick 挂载处），须对它打矩形桩。 */
    const wrapper = video.closest<HTMLElement>('[aria-current="true"]') ?? video.parentElement;
    assert.ok(wrapper, "item wrapper carries the click handler");
    const originalRect = wrapper.getBoundingClientRect.bind(wrapper);
    wrapper.getBoundingClientRect = () =>
      ({ height: 400, top: 0, bottom: 400, width: 300, left: 0, right: 300, x: 0, y: 0, toJSON() {} }) as DOMRect;
    await act(async () => {
      fireEvent.click(video, { clientY: 380 });
    });
    assert.equal(mediaStubs.calls.length, opsAfterMount + 1, "controls-strip clicks do not toggle");
    wrapper.getBoundingClientRect = originalRect;
  } finally {
    mediaStubs.restore();
  }

  /* 图片契约回归：点击仍进查看器（jsdom rect=0 即在条区之外）。 */
  const imageCase = renderWithIntl(<MediaGallery items={[makeItem(9)]} />);
  await act(async () => {
    fireEvent.click(imageCase.container.querySelector("img") as Element);
  });
  assert.equal(document.querySelectorAll("dialog[open]").length, 1, "image click still opens the viewer");
});
