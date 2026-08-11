import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { MediaGallery, type MediaGalleryItem } from "@/components/content/MediaGallery";
import { MediaViewer } from "@/components/content/MediaViewer";
import { act, cleanup, fireEvent, installDom, renderWithIntl } from "./runtime-test-helpers";

/* Native <dialog> showModal/close are not implemented in jsdom; stub the modal
   lifecycle so tests exercise the same code paths as browsers (same pattern as
   content-detail-overlay.test.tsx). */
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

/* MediaViewer 经 createPortal 挂到 document.body（脱离浮层 transform 祖先），
   所有查看器断言都查 body 而非渲染容器。 */
function viewerDialog(): HTMLDialogElement {
  const dialog = document.body.querySelector("dialog");
  assert.ok(dialog, "viewer dialog must be mounted");
  return dialog as HTMLDialogElement;
}

function stageOf(): HTMLElement {
  const stage = document.body.querySelector('[data-slot="viewer-stage"]');
  assert.ok(stage, "viewer stage must exist");
  return stage as HTMLElement;
}

function currentImage(): HTMLImageElement {
  const img = viewerDialog().querySelector("img");
  assert.ok(img, "viewer image must exist");
  return img;
}

function zoomOf(img: HTMLImageElement): number {
  const match = img.style.transform.match(/scale\(([\d.]+)\)/);
  return match ? Number.parseFloat(match[1]) : 1;
}

function positionLabel(): string {
  const group = viewerDialog().querySelector('[role="group"]');
  assert.ok(group, "viewer position dots group must exist");
  return group?.getAttribute("aria-label") ?? "";
}

/* 有状态包装：真实使用中由 MediaGallery 持有 open/index 状态，关闭类断言
   需要状态真正翻转才能触发清理副作用（滚动解锁、卸载）。 */
function renderViewer(
  overrides: Partial<React.ComponentProps<typeof MediaViewer>> = {},
  onEvent?: { openChange?: (open: boolean) => void; indexChange?: (index: number) => void },
) {
  const Comp = () => {
    const [open, setOpen] = React.useState(overrides.open ?? true);
    const [index, setIndex] = React.useState(overrides.index ?? 0);
    return (
      <MediaViewer
        items={overrides.items ?? items}
        index={index}
        open={open}
        onOpenChange={(next) => {
          setOpen(next);
          onEvent?.openChange?.(next);
        }}
        onIndexChange={(next) => {
          setIndex(next);
          onEvent?.indexChange?.(next);
        }}
      />
    );
  };
  return renderWithIntl(<Comp />);
}

/** rAF 在测试环境是 setTimeout(0)，等待一个宏任务让焦点恢复回调落定。 */
async function flushRaf() {
  await new Promise((resolve) => window.setTimeout(resolve, 5));
}

function dispatchWheel(stage: HTMLElement, deltaY: number) {
  const event = new window.WheelEvent("wheel", {
    deltaY,
    bubbles: true,
    cancelable: true,
  });
  act(() => {
    stage.dispatchEvent(event);
  });
}

function pointerDown(id: number, x: number, y: number, target?: Element) {
  act(() => {
    fireEvent.pointerDown(target ?? stageOf(), {
      pointerId: id,
      pointerType: "touch",
      clientX: x,
      clientY: y,
      bubbles: true,
    });
  });
}

function pointerMove(id: number, x: number, y: number, target?: Element) {
  act(() => {
    fireEvent.pointerMove(target ?? stageOf(), {
      pointerId: id,
      pointerType: "touch",
      clientX: x,
      clientY: y,
      bubbles: true,
    });
  });
}

function pointerUp(id: number, x: number, y: number, target?: Element) {
  act(() => {
    fireEvent.pointerUp(target ?? stageOf(), {
      pointerId: id,
      pointerType: "touch",
      clientX: x,
      clientY: y,
      bubbles: true,
    });
  });
}

test.beforeEach(() => {
  installDom();
  installDialogStubs();
});
test.afterEach(() => cleanup());

/* ---------- MediaGallery 接入（AC1/AC4） ---------- */

test("MediaGallery opens the internal viewer at the clicked item index", () => {
  const { container } = renderWithIntl(<MediaGallery items={items} />);
  const next = container.querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  fireEvent.click(next);
  fireEvent.click(container.querySelector('[aria-current="true"] img') as Element);
  assert.equal(viewerDialog().getAttribute("open"), "");
  assert.equal(positionLabel(), "2 / 3");
  const dots = viewerDialog().querySelectorAll('[role="group"] > span');
  assert.equal(dots.length, 3);
  assert.match(dots[1].className, /bg-white/);
});

test("MediaGallery forwards onOpenViewer when the prop is consumed and does not self-open", () => {
  const opened: number[] = [];
  const { container } = renderWithIntl(
    <MediaGallery items={items} onOpenViewer={(index) => opened.push(index)} />,
  );
  fireEvent.click(container.querySelector('[aria-current="true"] img') as Element);
  assert.deepEqual(opened, [0]);
  assert.equal(document.body.querySelector("dialog"), null);
});

test("closing the viewer restores focus to the clicked media wrapper (AC4)", async () => {
  const { container } = renderWithIntl(<MediaGallery items={items} />);
  const current = container.querySelector('[aria-current="true"]') as HTMLElement;
  fireEvent.click(current.querySelector("img") as Element);
  assert.ok(document.body.querySelector("dialog"));
  act(() => {
    fireEvent.keyDown(window, { key: "Escape" });
  });
  await flushRaf();
  assert.equal(document.body.querySelector("dialog"), null, "Esc closes the viewer");
  assert.equal(document.activeElement, current, "focus returns to the trigger media area");
});

test("viewer paging inside the gallery keeps the gallery surface mounted below", () => {
  const { container } = renderWithIntl(<MediaGallery items={items} />);
  fireEvent.click(container.querySelector('[aria-current="true"] img') as Element);
  const next = viewerDialog().querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  fireEvent.click(next);
  assert.equal(positionLabel(), "2 / 3");
  assert.ok(container.querySelector('[data-slot="detail-cover"]'), "gallery stays mounted under the viewer");
});

/* ---------- 进入与位置（AC1） ---------- */

test("MediaViewer renders a fullscreen dialog at the given index with position dots", () => {
  renderViewer({ index: 1 });
  assert.equal(viewerDialog().getAttribute("open"), "");
  assert.equal(positionLabel(), "2 / 3");
  assert.equal(currentImage().getAttribute("src"), "/seed-media/gallery/item-2.jpg");
});

test("MediaViewer renders nothing while closed", () => {
  renderViewer({ open: false });
  assert.equal(document.body.querySelector("dialog"), null);
});

test("previous/next buttons page within the media set and disable at boundaries (AC3/AC5)", () => {
  const indexChanges: number[] = [];
  renderViewer({ index: 1 }, { indexChange: (i) => indexChanges.push(i) });
  const previous = viewerDialog().querySelector('button[aria-label="Previous media"]') as HTMLButtonElement;
  const next = viewerDialog().querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  assert.equal(previous.disabled, false);
  assert.equal(next.disabled, false);
  fireEvent.click(next);
  assert.deepEqual(indexChanges, [2]);
  assert.equal(next.disabled, true, "next disabled on the last item: no content-level switching");
  fireEvent.click(previous);
  assert.deepEqual(indexChanges, [2, 1]);
  assert.equal(positionLabel(), "2 / 3");
  cleanup();
  renderViewer({ index: 0 });
  const firstPrev = document.body.querySelector('button[aria-label="Previous media"]') as HTMLButtonElement;
  assert.equal(firstPrev.disabled, true, "previous disabled on the first item");
});

test("single media item hides paging controls and dots", () => {
  renderViewer({ items: [makeItem(1)] });
  const dialog = viewerDialog();
  assert.equal(dialog.querySelector('button[aria-label="Previous media"]'), null);
  assert.equal(dialog.querySelector('button[aria-label="Next media"]'), null);
  assert.equal(dialog.querySelector('[role="group"]'), null);
});

/* ---------- 缩放（AC2） ---------- */

test("zoom buttons zoom in/out with the reset button restoring scale and pan", () => {
  renderViewer();
  const zoomIn = viewerDialog().querySelector('button[aria-label="Zoom in"]') as HTMLButtonElement;
  const zoomOut = viewerDialog().querySelector('button[aria-label="Zoom out"]') as HTMLButtonElement;
  const reset = viewerDialog().querySelector('button[aria-label="Reset zoom"]') as HTMLButtonElement;
  assert.equal(zoomOut.disabled, true, "zoom out disabled at scale 1");
  assert.equal(reset.disabled, true, "reset disabled at scale 1");

  fireEvent.click(zoomIn);
  assert.equal(zoomOf(currentImage()), 1.5);
  fireEvent.click(zoomIn);
  assert.equal(zoomOf(currentImage()), 2.25);
  assert.equal(zoomOut.disabled, false);
  fireEvent.click(zoomOut);
  assert.equal(zoomOf(currentImage()), 1.5);
  fireEvent.click(reset);
  assert.equal(zoomOf(currentImage()), 1);
  assert.match(currentImage().style.transform, /translate\(0px, 0px\)/);
});

test("zoom clamps to the configured maximum", () => {
  renderViewer();
  const zoomIn = viewerDialog().querySelector('button[aria-label="Zoom in"]') as HTMLButtonElement;
  for (let i = 0; i < 5; i += 1) fireEvent.click(zoomIn);
  assert.equal(zoomOf(currentImage()), 5);
  assert.ok(zoomIn.disabled, "zoom in disabled at max");
});

test("wheel zooms in and out around the center and resets on double-click", () => {
  renderViewer();
  const stage = stageOf();
  dispatchWheel(stage, -100);
  assert.equal(zoomOf(currentImage()), 1.2);
  dispatchWheel(stage, 100);
  assert.equal(zoomOf(currentImage()), 1);
  fireEvent.click(viewerDialog().querySelector('button[aria-label="Zoom in"]') as Element);
  assert.equal(zoomOf(currentImage()), 1.5);
  fireEvent.doubleClick(stage);
  assert.equal(zoomOf(currentImage()), 1, "double-click resets zoom");
});

test("two-pointer pinch zooms by the distance ratio", () => {
  renderViewer();
  const stage = stageOf();
  pointerDown(1, 100, 100, stage);
  pointerDown(2, 140, 100, stage);
  pointerMove(2, 200, 100, stage);
  assert.equal(zoomOf(currentImage()), 2.5);
  pointerUp(2, 200, 100, stage);
  pointerUp(1, 100, 100, stage);
  assert.equal(zoomOf(currentImage()), 2.5, "pinch release keeps the zoom");
});

test("zoomed images pan with a single pointer", () => {
  renderViewer();
  const stage = stageOf();
  const img = currentImage();
  fireEvent.click(viewerDialog().querySelector('button[aria-label="Zoom in"]') as Element);
  pointerDown(1, 200, 200, img);
  pointerMove(1, 260, 220, img);
  assert.match(img.style.transform, /translate\(60px, 20px\)/);
  pointerUp(1, 260, 220, img);
});

test("wheel and buttons do not zoom video items", () => {
  renderViewer({
    items: [makeItem(1, { type: "video", width: 1280, height: 720, posterUrl: "/poster.jpg" })],
  });
  const dialog = viewerDialog();
  const video = dialog.querySelector("video");
  assert.ok(video, "video player rendered in the viewer");
  assert.equal(video?.getAttribute("controls"), "");
  assert.equal(video?.getAttribute("poster"), "/poster.jpg");
  assert.equal(dialog.querySelector('button[aria-label="Zoom in"]'), null);
  dispatchWheel(stageOf(), -100);
  assert.ok(dialog.querySelector("video"), "wheel over a video does not unmount the player");
});

/* ---------- 翻页手势（AC3） ---------- */

test("touch swipe pages within the set and respects zoom priority (gesture: zoom > pan > page)", () => {
  renderViewer();
  const stage = stageOf();
  const img = currentImage();
  pointerDown(1, 200, 150, img);
  pointerMove(1, 60, 152, img);
  pointerUp(1, 60, 152, img);
  assert.equal(positionLabel(), "2 / 3", "swipe left pages forward");

  pointerDown(1, 60, 150, img);
  pointerMove(1, 220, 148, img);
  pointerUp(1, 220, 148, img);
  assert.equal(positionLabel(), "1 / 3", "swipe right pages back");

  pointerDown(1, 200, 150, img);
  pointerMove(1, 210, 300, img);
  pointerUp(1, 210, 300, img);
  assert.equal(positionLabel(), "1 / 3", "vertical swipe does not page");

  fireEvent.click(viewerDialog().querySelector('button[aria-label="Zoom in"]') as Element);
  pointerDown(1, 200, 150, img);
  pointerMove(1, 60, 152, img);
  pointerUp(1, 60, 152, img);
  assert.equal(positionLabel(), "1 / 3", "zoomed swipe pans instead of paging");
});

test("paging resets the zoom and pan state for the next item", () => {
  renderViewer();
  fireEvent.click(viewerDialog().querySelector('button[aria-label="Zoom in"]') as Element);
  assert.equal(zoomOf(currentImage()), 1.5);
  const next = viewerDialog().querySelector('button[aria-label="Next media"]') as HTMLButtonElement;
  fireEvent.click(next);
  assert.equal(zoomOf(currentImage()), 1, "next item opens at base zoom");
  assert.match(currentImage().style.transform, /translate\(0px, 0px\)/);
});

/* ---------- 三种退出（AC4） ---------- */

test("backdrop click on the empty stage closes the viewer", () => {
  const openCalls: boolean[] = [];
  renderViewer({}, { openChange: (open) => openCalls.push(open) });
  const stage = stageOf();
  pointerDown(1, 400, 300, stage);
  pointerUp(1, 402, 302, stage);
  assert.deepEqual(openCalls, [false], "backdrop pointerup closes the viewer");
});

test("a drag gesture over the stage does not close the viewer", () => {
  const openCalls: boolean[] = [];
  renderViewer({}, { openChange: (open) => openCalls.push(open) });
  const stage = stageOf();
  pointerDown(1, 400, 300, stage);
  pointerMove(1, 480, 360, stage);
  pointerUp(1, 500, 380, stage);
  assert.deepEqual(openCalls, [], "moved gesture is not a backdrop click");
});

test("the close button exits the viewer", () => {
  const openCalls: boolean[] = [];
  renderViewer({}, { openChange: (open) => openCalls.push(open) });
  fireEvent.click(viewerDialog().querySelector('button[aria-label="Close viewer"]') as Element);
  assert.deepEqual(openCalls, [false]);
});

test("Esc closes via a capture-phase listener that never propagates to lower layers", () => {
  const openCalls: boolean[] = [];
  let bubbledEsc = 0;
  renderViewer({}, { openChange: (open) => openCalls.push(open) });
  window.addEventListener("keydown", () => (bubbledEsc += 1));
  act(() => {
    fireEvent.keyDown(window, { key: "Escape" });
  });
  assert.deepEqual(openCalls, [false], "Esc closes the top-most viewer only");
  assert.equal(bubbledEsc, 0, "stopPropagation keeps Esc away from any lower-layer handler");
});

test("the viewer locks background scroll while open and restores the previous value on close", () => {
  document.body.style.overflow = "auto";
  document.documentElement.style.overflow = "auto";
  renderViewer({});
  assert.equal(document.body.style.overflow, "hidden");
  assert.equal(document.documentElement.style.overflow, "hidden");
  fireEvent.click(viewerDialog().querySelector('button[aria-label="Close viewer"]') as Element);
  assert.equal(document.body.style.overflow, "auto");
  assert.equal(document.documentElement.style.overflow, "auto");
});

/* ---------- 错误占位（ui-spec 状态变体） ---------- */

test("failed media shows a stable placeholder with a retry action", () => {
  renderViewer();
  const img = currentImage();
  fireEvent.error(img);
  const dialog = viewerDialog();
  assert.ok(dialog.textContent?.includes("Failed to load media"));
  const retry = Array.from(dialog.querySelectorAll("button")).find((button) =>
    button.textContent?.includes("Retry"),
  );
  assert.ok(retry, "retry button rendered");
  fireEvent.click(retry as Element);
  assert.ok(dialog.querySelector("img"), "retry restores the image element");
});
