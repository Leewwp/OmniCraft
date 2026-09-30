import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { act, cleanup, fireEvent, installDom, render } from "./runtime-test-helpers";
import {
  normalizeContentItem,
  normalizeContentListResponse,
} from "@/lib/content";
import type { ContentCardData } from "@/components/content/ContentCard";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

Module._load = function loadWithStubs(request, parent, isMain) {
  if (request === "next/image") {
    /* jsdom + real next/image can hang the node:test runner; stub the
       optimizer wrapper so the two-tier logic under test (aspect-ratio
       container, object-cover) is exercised on a plain <img>. */
    return (props: Record<string, unknown>) =>
      React.createElement("img", { ...props, fill: undefined, sizes: undefined });
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type ContentCardModule = typeof import("@/components/content/ContentCard");
let ContentCard: ContentCardModule["ContentCard"];

test.before(async () => {
  const module = await import("@/components/content/ContentCard");
  ContentCard = module.ContentCard;
});

test.afterEach(() => cleanup());

function renderCard(data: Partial<ContentCardData>) {
  installDom();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentCard data={{ id: 1, title: "Ratio card", zone: "original", ...data }} />
    </IntlProvider>,
  );
}

function coverAspectSlot(): HTMLElement {
  const frame = document.querySelector('[data-slot="card-cover-aspect"]');
  assert.ok(frame, "cover aspect frame must render");
  return frame as HTMLElement;
}

test("cover without cover size falls back to the defensive 3:4 tier", () => {
  renderCard({ content_type: "video" });
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "3 / 4");
});

/* #753 两档封面：h/w ≤ 4/3（含边界）→ 3:4；h/w > 4/3 → 9:16；
   object-cover 中心裁切（仅显示比例，不改原图）。 */
test("landscape cover (h/w <= 4/3) lands the 3:4 tier with object-cover center crop", () => {
  renderCard({ content_type: "image", cover_image_url: "/cover.png", cover_width: 1200, cover_height: 800 });
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "3 / 4");
  const img = document.querySelector('img[alt="Ratio card"]');
  assert.ok(img, "cover image must render inside the frame");
  const imageClass = img.getAttribute("class") ?? "";
  assert.match(imageClass, /object-cover/, "list covers crop to the tier box");
  assert.match(imageClass, /object-center/, "center crop anchor");
});

test("portrait cover beyond the boundary (h/w > 4/3) lands the 9:16 tier", () => {
  renderCard({ cover_image_url: "/tall.png", cover_width: 600, cover_height: 2400 });
  assert.equal(coverAspectSlot().style.aspectRatio, "9 / 16");
});

test("wide landscape cover stays in the 3:4 tier (no height cap anymore)", () => {
  renderCard({ cover_image_url: "/wide.png", cover_width: 4800, cover_height: 600 });
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "3 / 4");
  assert.equal(frame.style.maxHeight, "", "two-tier boxes are ratio-driven, not capped");
});

test("cover at the exact h/w = 4/3 boundary lands the 3:4 tier (boundary inclusive)", () => {
  renderCard({ cover_image_url: "/b1.png", cover_width: 1200, cover_height: 1600 });
  assert.equal(coverAspectSlot().style.aspectRatio, "3 / 4");
});

test("cover just above the boundary lands the 9:16 tier", () => {
  renderCard({ cover_image_url: "/b2.png", cover_width: 1000, cover_height: 1334 });
  assert.equal(coverAspectSlot().style.aspectRatio, "9 / 16");
});

test("video cover follows the poster ratio into the 9:16 tier (no forced 16:9)", () => {
  renderCard({ content_type: "video", cover_image_url: "/poster.png", cover_width: 720, cover_height: 1280 });
  assert.equal(coverAspectSlot().style.aspectRatio, "9 / 16");
});

test("fanwork zone shares the same tier fact source", () => {
  renderCard({ zone: "fanwork", content_type: "image", cover_image_url: "/square.png", cover_width: 1000, cover_height: 1000 });
  assert.equal(coverAspectSlot().style.aspectRatio, "3 / 4");
  const card = document.querySelector('[aria-label="Ratio card"]');
  assert.ok(card, "fanwork card stays keyboard-focusable");
});

/* #753 视频右上播放角标（仅视频；无多图数量角标）。 */
test("video card shows the top-right play badge", () => {
  renderCard({ content_type: "video", cover_image_url: "/poster.png", cover_width: 720, cover_height: 1280 });
  const badge = document.querySelector('[data-slot="card-cover"] span.absolute.right-2.top-2');
  assert.ok(badge, "play badge renders top-right");
});

test("image card does not show the play badge", () => {
  renderCard({ content_type: "image", cover_image_url: "/cover.png", cover_width: 1200, cover_height: 800 });
  const badge = document.querySelector('[data-slot="card-cover"] span.absolute.right-2.top-2');
  assert.equal(badge, null);
});

test("card link is keyboard focusable", () => {
  renderCard({});
  const link = document.querySelector('[aria-label="Ratio card"]') as HTMLElement | null;
  assert.ok(link, "card must be reachable as a focusable element");
  link.focus();
  assert.equal(document.activeElement, link);
});

test("normalizeContentItem picks up snake_case cover size", () => {
  const item = normalizeContentItem({ id: 1, title: "A", zone: "original", cover_width: 1200, cover_height: 800 });
  assert.equal(item?.cover_width, 1200);
  assert.equal(item?.cover_height, 800);
});

test("normalizeContentItem picks up PascalCase cover size", () => {
  const item = normalizeContentItem({ ID: 2, Title: "B", Zone: "fanwork", CoverWidth: "600", CoverHeight: "900" });
  assert.equal(item?.cover_width, 600);
  assert.equal(item?.cover_height, 900);
});

test("invalid or absent cover size stays undefined so the card falls back to 3:4", () => {
  const invalid = normalizeContentItem({ id: 3, title: "C", zone: "original", cover_width: 0, cover_height: null });
  assert.equal(invalid?.cover_width, undefined);
  assert.equal(invalid?.cover_height, undefined);
  const legacy = normalizeContentItem({ id: 4, title: "D", zone: "original" });
  assert.equal(legacy?.cover_width, undefined);
  assert.equal(legacy?.cover_height, undefined);
});

test("normalizeContentListResponse keeps cover size on list items", () => {
  const items = normalizeContentListResponse({
    contents: [{ id: 5, title: "E", zone: "original", cover_width: 1600, cover_height: 900 }],
  });
  assert.equal(items[0]?.cover_width, 1600);
  assert.equal(items[0]?.cover_height, 900);
});

test("missing cover size adopts the measured ratio via the tier mapping after load (#398 C2)", () => {
  const view = renderCard({ content_type: "image", cover_image_url: "/probe.png" });
  void view;
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "3 / 4", "defensive 3:4 before the cover loads");
  const img = document.querySelector('img[alt="Ratio card"]') as HTMLImageElement;
  Object.defineProperty(img, "naturalWidth", { value: 1600, configurable: true });
  Object.defineProperty(img, "naturalHeight", { value: 900, configurable: true });
  act(() => {
    fireEvent.load(img);
  });
  assert.equal(frame.style.aspectRatio, "3 / 4", "measured 1600x900 (h/w=0.5625 <= 4/3) lands the 3:4 tier");
});

test("metadata cover size is never overridden by the load measurement", () => {
  renderCard({ content_type: "image", cover_image_url: "/meta.png", cover_width: 1000, cover_height: 1000 });
  const img = document.querySelector('img[alt="Ratio card"]') as HTMLImageElement;
  Object.defineProperty(img, "naturalWidth", { value: 1600, configurable: true });
  Object.defineProperty(img, "naturalHeight", { value: 900, configurable: true });
  act(() => {
    fireEvent.load(img);
  });
  assert.equal(coverAspectSlot().style.aspectRatio, "3 / 4", "contract metadata stays authoritative (1000x1000 → 3:4 tier)");
});
