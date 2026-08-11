import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, installDom, render } from "./runtime-test-helpers";
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
       optimizer wrapper so the natural-ratio logic under test (aspect-ratio
       container, object-contain, height cap) is exercised on a plain <img>. */
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

test("cover without cover size falls back to the defensive 3:4 ratio without cap", () => {
  renderCard({ content_type: "video" });
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "3 / 4");
  assert.equal(frame.style.maxHeight, "");
});

test("cover uses the data-driven aspect ratio and object-contain (no crop)", () => {
  renderCard({ content_type: "image", cover_image_url: "/cover.png", cover_width: 1200, cover_height: 800 });
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "1200 / 800");
  assert.equal(frame.style.maxHeight, "", "normal ratios are not height-capped");
  const img = document.querySelector('img[alt="Ratio card"]');
  assert.ok(img, "cover image must render inside the frame");
  const imageClass = img.getAttribute("class") ?? "";
  assert.match(imageClass, /object-contain/);
  assert.doesNotMatch(imageClass, /object-cover/, "list covers must never crop");
});

test("extreme portrait cover is height-capped without cropping", () => {
  renderCard({ cover_image_url: "/tall.png", cover_width: 600, cover_height: 2400 });
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "600 / 2400");
  assert.equal(frame.style.maxHeight, "400px");
});

test("extreme landscape cover is height-capped as well", () => {
  renderCard({ cover_image_url: "/wide.png", cover_width: 4800, cover_height: 600 });
  const frame = coverAspectSlot();
  assert.equal(frame.style.aspectRatio, "4800 / 600");
  assert.equal(frame.style.maxHeight, "400px");
});

test("cover at the exact 2:1 boundary is not treated as extreme", () => {
  renderCard({ cover_image_url: "/b1.png", cover_width: 2000, cover_height: 1000 });
  assert.equal(coverAspectSlot().style.maxHeight, "");
});

test("cover at the exact 1:2 boundary is not treated as extreme", () => {
  renderCard({ cover_image_url: "/b2.png", cover_width: 1000, cover_height: 2000 });
  assert.equal(coverAspectSlot().style.maxHeight, "");
});

test("video cover follows the poster ratio instead of a forced 16:9", () => {
  renderCard({ content_type: "video", cover_image_url: "/poster.png", cover_width: 720, cover_height: 1280 });
  assert.equal(coverAspectSlot().style.aspectRatio, "720 / 1280");
});

test("fanwork zone shares the same ratio-driven cover fact source", () => {
  renderCard({ zone: "fanwork", content_type: "image", cover_image_url: "/square.png", cover_width: 1000, cover_height: 1000 });
  assert.equal(coverAspectSlot().style.aspectRatio, "1000 / 1000");
  const card = document.querySelector('[aria-label="Ratio card"]');
  assert.ok(card, "fanwork card stays keyboard-focusable");
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
