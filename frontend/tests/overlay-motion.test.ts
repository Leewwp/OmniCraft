import test from "node:test";
import assert from "node:assert/strict";
import {
  OVERLAY_MOTION,
  computeFlipTransform,
  flipTransformToCss,
  measureSourceRect,
  nextFrame,
  rectHasArea,
  rectIntersectsViewport,
  reducedMotionEnabled,
  selectMotionPath,
  viewTransitionAvailable,
  type OverlayRect,
} from "@/lib/overlay-motion";
import { installDom } from "./runtime-test-helpers";

/* ---------- 路径选择合同（原型 §5）：每次操作独立判定，不缓存 ---------- */

test("selectMotionPath: reduced motion wins over every other path", () => {
  assert.equal(selectMotionPath(true, { x: 0, y: 0, width: 100, height: 100 }, true), "fade");
  assert.equal(selectMotionPath(true, null, false), "fade");
  assert.equal(selectMotionPath(true, { x: 0, y: 0, width: 100, height: 100 }, false), "fade");
});

test("selectMotionPath: missing or degenerate source rect falls back", () => {
  assert.equal(selectMotionPath(false, null, true), "fallback");
  assert.equal(selectMotionPath(false, { x: 0, y: 0, width: 0, height: 0 }, true), "fallback");
  assert.equal(selectMotionPath(false, { x: 0, y: 0, width: 10, height: 0 }, false), "fallback");
});

test("selectMotionPath: VT is a progressive enhancement over FLIP", () => {
  assert.equal(selectMotionPath(false, { x: 0, y: 0, width: 10, height: 10 }, true), "vt");
  assert.equal(selectMotionPath(false, { x: 0, y: 0, width: 10, height: 10 }, false), "flip");
});

/* ---------- FLIP 几何（原型 §4.3：Invert 数学与方向不可反） ---------- */

test("computeFlipTransform maps the target rect onto the source rect", () => {
  const source: OverlayRect = { x: 0, y: 0, width: 100, height: 100 };
  const target: OverlayRect = { x: 100, y: 50, width: 200, height: 200 };
  assert.deepEqual(computeFlipTransform(source, target), {
    x: -100,
    y: -50,
    scaleX: 0.5,
    scaleY: 0.5,
  });
});

test("computeFlipTransform degrades to identity when the target has no area", () => {
  const source: OverlayRect = { x: 0, y: 0, width: 100, height: 100 };
  const target: OverlayRect = { x: 5, y: 5, width: 0, height: 0 };
  assert.deepEqual(computeFlipTransform(source, target), { x: -5, y: -5, scaleX: 1, scaleY: 1 });
});

test("flipTransformToCss emits translate + scale with pixel values", () => {
  assert.equal(
    flipTransformToCss({ x: -100, y: -50, scaleX: 0.5, scaleY: 0.5 }),
    "translate(-100px, -50px) scale(0.5, 0.5)",
  );
});

test("rectHasArea and rectIntersectsViewport implement the measurement gate", () => {
  assert.equal(rectHasArea(null), false);
  assert.equal(rectHasArea({ x: 0, y: 0, width: 1, height: 1 }), true);
  assert.equal(rectIntersectsViewport({ x: -50, y: 0, width: 100, height: 100 }, 800, 600), true);
  assert.equal(rectIntersectsViewport({ x: 800, y: 0, width: 100, height: 100 }, 800, 600), false);
  assert.equal(rectIntersectsViewport({ x: 0, y: 600, width: 100, height: 100 }, 800, 600), false);
  assert.equal(rectIntersectsViewport({ x: 100, y: 100, width: 10, height: 10 }, 800, 600), true);
});

/* ---------- measureSource 门（原型 §3：isConnected + 尺寸 > 0 + 视口交集） ---------- */

function stubRect(el: Element, rect: OverlayRect) {
  const proto = el.getBoundingClientRect as unknown as () => DOMRect;
  el.getBoundingClientRect = () =>
    ({ x: rect.x, y: rect.y, width: rect.width, height: rect.height, left: rect.x, top: rect.y, right: rect.x + rect.width, bottom: rect.y + rect.height, toJSON: () => ({}) }) as DOMRect;
  return proto;
}

test("measureSourceRect returns the anchor rect when connected, sized and in viewport", () => {
  installDom();
  const card = document.createElement("button");
  document.body.appendChild(card);
  stubRect(card, { x: 10, y: 20, width: 300, height: 400 });
  assert.deepEqual(measureSourceRect(card), { x: 10, y: 20, width: 300, height: 400 });
});

test("measureSourceRect prefers the card cover slot as the visual anchor", () => {
  installDom();
  const card = document.createElement("button");
  const cover = document.createElement("span");
  cover.setAttribute("data-slot", "card-cover");
  card.appendChild(cover);
  document.body.appendChild(card);
  stubRect(card, { x: 10, y: 20, width: 300, height: 400 });
  stubRect(cover, { x: 10, y: 20, width: 300, height: 180 });
  assert.deepEqual(measureSourceRect(card), { x: 10, y: 20, width: 300, height: 180 });
});

test("measureSourceRect rejects detached, zero-size and offscreen sources", () => {
  installDom();
  const detached = document.createElement("button");
  stubRect(detached, { x: 0, y: 0, width: 300, height: 400 });
  assert.equal(measureSourceRect(detached), null, "detached must be rejected");

  const zero = document.createElement("button");
  document.body.appendChild(zero);
  stubRect(zero, { x: 0, y: 0, width: 0, height: 0 });
  assert.equal(measureSourceRect(zero), null, "zero-size must be rejected");

  const offscreen = document.createElement("button");
  document.body.appendChild(offscreen);
  stubRect(offscreen, { x: 0, y: -500, width: 300, height: 400 });
  assert.equal(measureSourceRect(offscreen), null, "offscreen must be rejected");
});

test("measureSourceRect returns null for nullish triggers", () => {
  installDom();
  assert.equal(measureSourceRect(null), null);
  assert.equal(measureSourceRect(undefined), null);
});

/* ---------- 环境判定与帧保险 ---------- */

test("reducedMotionEnabled reads prefers-reduced-motion from matchMedia", () => {
  installDom();
  const original = window.matchMedia;
  assert.equal(reducedMotionEnabled(), false, "jsdom default must be no-preference");
  window.matchMedia = ((query: string) => ({
    matches: query.includes("reduce"),
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
  assert.equal(reducedMotionEnabled(), true);
  window.matchMedia = original;
});

test("viewTransitionAvailable is feature-detected on the document", () => {
  installDom();
  assert.equal(viewTransitionAvailable(), false, "jsdom must not expose startViewTransition");
  const original = document.startViewTransition;
  document.startViewTransition = (() => ({})) as unknown as typeof document.startViewTransition;
  assert.equal(viewTransitionAvailable(), true);
  delete (document as Partial<Document>).startViewTransition;
  assert.equal(viewTransitionAvailable(), false);
  (document as Partial<Document>).startViewTransition = original;
});

test("nextFrame resolves through double rAF with a timeout safety net", async () => {
  installDom();
  await nextFrame();
  assert.ok(true);
});

test("motion timing constants match the confirmed overlay contract", () => {
  assert.equal(OVERLAY_MOTION.openDuration, 300);
  assert.equal(OVERLAY_MOTION.closeDuration, 240);
  assert.equal(OVERLAY_MOTION.reducedDuration, 100);
  assert.equal(OVERLAY_MOTION.easing, "cubic-bezier(0.22, 0.61, 0.36, 1)");
  assert.equal(OVERLAY_MOTION.fallbackScale, 0.96);
});
