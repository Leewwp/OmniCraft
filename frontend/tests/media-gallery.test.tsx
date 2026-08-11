import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { MediaGallery, selectMediaItems, type MediaGalleryItem } from "@/components/content/MediaGallery";
import { normalizeAttachment, type AttachmentData } from "@/lib/content";
import { act, cleanup, fireEvent, installDom, renderWithIntl } from "./runtime-test-helpers";

function makeItem(id: number, overrides: Partial<MediaGalleryItem> = {}): MediaGalleryItem {
  return {
    id,
    url: `/seed-media/gallery/item-${id}.jpg`,
    type: "image",
    width: 1000,
    height: 750,
    ...overrides,
  };
}

const items = [makeItem(1), makeItem(2), makeItem(3)];

function renderGallery(overrides: Partial<React.ComponentProps<typeof MediaGallery>> = {}) {
  return renderWithIntl(<MediaGallery items={items} {...overrides} />);
}

function mediaScroller(container: HTMLElement): HTMLElement {
  const scroller = container.querySelector<HTMLElement>('[data-slot="detail-cover"] > div');
  assert.ok(scroller, "media scroller must exist");
  return scroller;
}

/** jsdom 没有 TouchEvent 构造器；构造带 touches/changedTouches 的普通事件并冒泡。 */
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

/* Native <dialog> showModal/close are not implemented in jsdom; stub the modal
   lifecycle so the MediaViewer mounted by MediaGallery exercises the same code
   paths as browsers (same pattern as content-detail-overlay.test.tsx). */
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

test.beforeEach(() => {
  installDom();
  installDialogStubs();
});
test.afterEach(() => cleanup());

/* ---------- 媒体集/附件语义拆分（AC3） ---------- */

test("selectMediaItems routes image/video attachments of image content into the gallery", () => {
  const attachments: AttachmentData[] = [
    { id: 1, file_type: "image", oss_key: "/a.jpg", sort_order: 1 },
    { id: 2, file_type: "image", oss_key: "/b.jpg", sort_order: 0 },
    { id: 3, file_type: "pdf", oss_key: "/doc.pdf" },
    { id: 4, file_type: "image", oss_key: "" },
  ];
  const { media, downloads } = selectMediaItems(attachments, "image");
  assert.deepEqual(media.map((item) => item.id), [1, 2], "media set keeps server order, no re-sort");
  assert.deepEqual(downloads.map((item) => item.id), [3, 4], "non-media and url-less rows stay downloadable");
});

test("selectMediaItems keeps all attachments in the download list for non-media content types", () => {
  const attachments: AttachmentData[] = [
    { id: 1, file_type: "image", oss_key: "/a.jpg" },
    { id: 2, file_type: "zip", oss_key: "/mod.zip" },
  ];
  const { media, downloads } = selectMediaItems(attachments, "article");
  assert.deepEqual(media, []);
  assert.deepEqual(downloads.map((item) => item.id), [1, 2]);
});

test("selectMediaItems applies the content cover as poster for video items only", () => {
  const attachments: AttachmentData[] = [{ id: 1, file_type: "video", oss_key: "/v.mp4" }];
  const { media } = selectMediaItems(attachments, "video", "/seed-media/covers/video-poster.jpg");
  assert.equal(media.length, 1);
  assert.equal(media[0].type, "video");
  assert.equal(media[0].posterUrl, "/seed-media/covers/video-poster.jpg");
});

/* ---------- 归一化（AC4 数据面） ---------- */

test("normalizeAttachment picks up width height and sort_order from snake and Pascal keys", () => {
  const parsed = normalizeAttachment({
    id: 7,
    width: "640",
    height: 480,
    sort_order: 0,
  });
  assert.equal(parsed?.width, 640);
  assert.equal(parsed?.height, 480);
  assert.equal(parsed?.sort_order, 0);
  assert.equal(parsed?.file_type, undefined);
});

test("normalizeAttachment drops non-positive dimensions and keeps NULL legacy rows as undefined", () => {
  const parsed = normalizeAttachment({ id: 8, Width: -10, Height: 0, SortOrder: 2 });
  assert.equal(parsed?.width, undefined);
  assert.equal(parsed?.height, undefined);
  assert.equal(parsed?.sort_order, 2);
  const legacy = normalizeAttachment({ id: 9 });
  assert.equal(legacy?.width, undefined);
  assert.equal(legacy?.height, undefined);
  assert.equal(legacy?.sort_order, undefined);
});

/* ---------- 渲染顺序与稳定几何（AC1） ---------- */

test("MediaGallery renders all items in server order with current item visible and others hidden", () => {
  const { container } = renderGallery();
  const images = Array.from(container.querySelectorAll("img"));
  assert.deepEqual(
    images.map((img) => img.getAttribute("src")),
    ["/seed-media/gallery/item-1.jpg", "/seed-media/gallery/item-2.jpg", "/seed-media/gallery/item-3.jpg"],
  );
  const wrappers = Array.from(mediaScroller(container).children) as HTMLElement[];
  assert.equal(wrappers.length, 3);
  const current = wrappers.filter((el) => el.getAttribute("aria-current") === "true");
  assert.equal(current.length, 1);
  assert.equal(current[0]?.querySelector("img")?.getAttribute("src"), "/seed-media/gallery/item-1.jpg");
  const nonCurrent = wrappers.filter((el) => el.getAttribute("aria-current") !== "true");
  assert.equal(nonCurrent.length, 2);
  for (const el of nonCurrent) {
    assert.ok(el.classList.contains("hidden"), "non-current items are removed from the a11y/focus order");
  }
});

function parseAspectRatio(scroller: HTMLElement): number {
  return Number.parseFloat(scroller.style.aspectRatio);
}

test("MediaGallery derives container geometry from the first item and keeps it stable", () => {
  const wideFirst = renderGallery({ items: [makeItem(1, { width: 1600, height: 900 }), makeItem(2)] });
  assert.equal(parseAspectRatio(mediaScroller(wideFirst.container)), 16 / 9);
  cleanup();
  const portraitFirst = renderGallery({ items: [makeItem(1, { width: 750, height: 1000 }), makeItem(2)] });
  assert.equal(parseAspectRatio(mediaScroller(portraitFirst.container)), 3 / 4);
});

test("MediaGallery falls back to a defensive 3:4 ratio when width/height are missing (AC4)", () => {
  const { container } = renderGallery({ items: [makeItem(1, { width: undefined, height: undefined }), makeItem(2)] });
  assert.equal(parseAspectRatio(mediaScroller(container)), 3 / 4);
});

test("MediaGallery caps ultra-tall first items with internal scroll and leaves others uncapped", () => {
  const tall = renderGallery({ items: [makeItem(1, { width: 400, height: 1200 })] });
  const scroller = mediaScroller(tall.container);
  assert.equal(scroller.style.maxHeight, "70vh");
  assert.ok(scroller.classList.contains("overflow-y-auto"));
  cleanup();
  const normal = renderGallery({ items: [makeItem(1, { width: 400, height: 600 })] });
  assert.equal(mediaScroller(normal.container).style.maxHeight, "");
});

/* ---------- 指示点与翻页（AC2） ---------- */

test("MediaGallery shows subtle position dots with current solid and a position label", () => {
  const { container } = renderGallery();
  const group = container.querySelector('[role="group"]');
  assert.equal(group?.getAttribute("aria-label"), "1 / 3");
  const dots = container.querySelectorAll('[role="group"] > span');
  assert.equal(dots.length, 3);
  assert.match(dots[0].className, /bg-foreground/);
  assert.match(dots[1].className, /bg-muted-foreground\/30/);
});

test("MediaGallery previous/next buttons page through and disable at boundaries", () => {
  const { container } = renderGallery();
  const previous = container.querySelector('button[aria-label="Previous media"]') as HTMLButtonElement;
  const next = container.querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  assert.ok(previous.disabled, "previous disabled on first item");
  assert.equal(next.disabled, false);

  fireEvent.click(next);
  assert.equal(container.querySelector('[role="group"]')?.getAttribute("aria-label"), "2 / 3");
  assert.equal(previous.disabled, false);

  fireEvent.click(next);
  assert.equal(container.querySelector('[role="group"]')?.getAttribute("aria-label"), "3 / 3");
  assert.ok(next.disabled, "next disabled on last item");

  fireEvent.click(previous);
  assert.equal(container.querySelector('[role="group"]')?.getAttribute("aria-label"), "2 / 3");
});

test("MediaGallery hides the control row for a single media item", () => {
  const { container } = renderGallery({ items: [makeItem(1)] });
  assert.equal(container.querySelector('[role="group"]'), null);
  assert.equal(container.querySelector("button"), null);
});

test("MediaGallery swipe pages horizontally and ignores vertical swipes so page scroll is not fought", () => {
  const { container } = renderGallery();
  const scroller = mediaScroller(container);

  dispatchTouch(scroller, "touchstart", 150, 100);
  dispatchTouch(scroller, "touchend", 40, 105);
  assert.equal(container.querySelector('[role="group"]')?.getAttribute("aria-label"), "2 / 3");

  dispatchTouch(scroller, "touchstart", 150, 100);
  dispatchTouch(scroller, "touchend", 300, 105);
  assert.equal(container.querySelector('[role="group"]')?.getAttribute("aria-label"), "1 / 3");

  dispatchTouch(scroller, "touchstart", 150, 100);
  dispatchTouch(scroller, "touchend", 160, 300);
  assert.equal(container.querySelector('[role="group"]')?.getAttribute("aria-label"), "1 / 3", "vertical swipe must not page");
});

test("MediaGallery fires onReachEnd only when the last item swipes upward (AC2 #89 hook)", () => {
  let reached = 0;
  const { container } = renderGallery({ onReachEnd: () => (reached += 1) });
  const scroller = mediaScroller(container);
  dispatchTouch(scroller, "touchstart", 150, 100);
  dispatchTouch(scroller, "touchend", 40, 105);
  dispatchTouch(scroller, "touchstart", 150, 100);
  dispatchTouch(scroller, "touchend", 40, 105);
  assert.equal(reached, 0, "no reach-end on non-last items");
  dispatchTouch(scroller, "touchstart", 150, 260);
  dispatchTouch(scroller, "touchend", 160, 100);
  assert.equal(reached, 1);
});

/* ---------- 视频项（AC1/AC3） ---------- */

test("MediaGallery renders video items as sequential players with controls metadata preload and poster", () => {
  const { container } = renderGallery({
    items: [makeItem(1, { type: "video", width: 1280, height: 720, posterUrl: "/poster.jpg" })],
  });
  const video = container.querySelector("video");
  assert.ok(video, "video element rendered");
  assert.equal(video?.getAttribute("controls"), "");
  assert.equal(video?.getAttribute("preload"), "metadata");
  assert.equal(video?.getAttribute("poster"), "/poster.jpg");
  assert.equal(video?.getAttribute("src"), "/seed-media/gallery/item-1.jpg");
});

/* ---------- 错误占位与查看器入口（AC2/AC5） ---------- */

test("MediaGallery shows a stable error placeholder for a failed media item without blocking switching", () => {
  const { container } = renderGallery();
  const firstImage = container.querySelector("img");
  assert.ok(firstImage);
  fireEvent.error(firstImage);
  assert.ok(container.textContent?.includes("Failed to load media"));
  const next = container.querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  fireEvent.click(next);
  const currentImage = container.querySelector('[aria-current="true"] img');
  assert.equal(currentImage?.getAttribute("src"), "/seed-media/gallery/item-2.jpg");
});

test("MediaGallery invokes onOpenViewer on image click with the current index", () => {
  const opened: number[] = [];
  const { container } = renderGallery({ onOpenViewer: (index) => opened.push(index) });
  fireEvent.click(container.querySelector("img") as Element);
  const next = container.querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  fireEvent.click(next);
  fireEvent.click(container.querySelector('[aria-current="true"] img') as Element);
  assert.deepEqual(opened, [0, 1]);
});

test("MediaGallery opens the viewer for video clicks outside the controls strip and self-opens without handler", () => {
  const opened: number[] = [];
  const { container } = renderGallery({
    items: [makeItem(1, { type: "video" })],
    onOpenViewer: (index) => opened.push(index),
  });
  fireEvent.click(container.querySelector("video") as Element);
  assert.deepEqual(opened, [0], "jsdom rect is 0 so the click lands above the controls strip");
  cleanup();

  // 无上层消费时（当前唯一路径）内部自持状态渲染 MediaViewer（#86）。
  const noHandler = renderGallery({ items: [makeItem(1)] });
  fireEvent.click(noHandler.container.querySelector("img") as Element);
  assert.ok(
    document.body.querySelector("dialog"),
    "without an external handler the internal viewer opens",
  );
});

test("MediaGallery geometry stays stable while switching between different aspect ratios", () => {
  const mixed = [
    makeItem(1, { width: 1600, height: 900 }),
    makeItem(2, { width: 600, height: 1200 }),
    makeItem(3, { width: 800, height: 800 }),
  ];
  const { container } = renderGallery({ items: mixed });
  const scroller = mediaScroller(container);
  const geometryBefore = `${parseAspectRatio(scroller)}:${scroller.style.maxHeight}`;
  const next = container.querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  fireEvent.click(next);
  fireEvent.click(next);
  assert.equal(`${parseAspectRatio(scroller)}:${scroller.style.maxHeight}`, geometryBefore, "container geometry must not jump");
});
