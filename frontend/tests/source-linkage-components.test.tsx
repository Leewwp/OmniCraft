import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import zhMessages from "@/messages/zh.json";
import { api } from "@/lib/api";
import { cleanup, installDom, render, waitFor } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

/* jsdom + real next/image can hang the node:test runner; stub the optimizer
   wrapper so ContentCard renders a plain <img> (same convention as
   content-card-natural-ratio.test.tsx). */
const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

Module._load = function loadWithStubs(request, parent, isMain) {
  if (request === "next/image") {
    return (props: Record<string, unknown>) =>
      React.createElement("img", { ...props, fill: undefined, sizes: undefined });
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type SourceAttributionModule = typeof import("@/components/content/SourceAttribution");
type RelatedFanworksModule = typeof import("@/components/content/RelatedFanworks");

let SourceAttribution: SourceAttributionModule["SourceAttribution"];
let RelatedFanworks: RelatedFanworksModule["RelatedFanworks"];

const originalGet = api.get;

test.before(async () => {
  const attributionModule = await import("@/components/content/SourceAttribution");
  const relatedModule = await import("@/components/content/RelatedFanworks");
  SourceAttribution = attributionModule.SourceAttribution;
  RelatedFanworks = relatedModule.RelatedFanworks;
});

test.after(() => {
  api.get = originalGet;
});

test.afterEach(() => cleanup());

function renderWithEn(node: React.ReactNode) {
  return render(<IntlProvider locale="en" messages={enMessages}>{node}</IntlProvider>);
}

function mockRelatedResponse(response: { contents: unknown[]; total: number }) {
  api.get = (async (requestPath: string) => {
    assert.match(requestPath, /\/related-fanworks\?page=1&page_size=8$/);
    return response;
  }) as typeof api.get;
}

function relatedCard(id: number) {
  return {
    id,
    title: `Related ${id}`,
    zone: "fanwork" as const,
    content_type: "image",
    author: { id: 1, username: "Author" },
    like_count: 2,
  };
}

/* ------------------------------------------------------------------ */
/* SourceAttribution — 7 项计划断言（前 4 项）                            */
/* ------------------------------------------------------------------ */

test("SourceAttribution links original sources to /original/:id", () => {
  installDom();
  const { container } = renderWithEn(
    <SourceAttribution
      zone="fanwork"
      sourceOriginalId={5}
      sourceOriginal={{ id: 5, title: "Source Original Title", zone: "original" }}
    />,
  );
  const link = container.querySelector('a[href="/original/5"]');
  assert.ok(link, "expected a link to /original/5");
  assert.match(link?.textContent ?? "", /Source Original Title/);
});

test("SourceAttribution links fanwork sources to /content/:id", () => {
  installDom();
  const { container } = renderWithEn(
    <SourceAttribution
      zone="fanwork"
      sourceFanworkId={9}
      sourceFanwork={{ id: 9, title: "Source Fanwork Title", zone: "fanwork" }}
    />,
  );
  const link = container.querySelector('a[href="/content/9"]');
  assert.ok(link, "expected a link to /content/9");
  assert.match(link?.textContent ?? "", /Source Fanwork Title/);
});

test("IP-only fanwork renders no attribution row", () => {
  installDom();
  const { container } = renderWithEn(<SourceAttribution zone="fanwork" />);
  assert.equal(container.textContent, "");
});

test("source id without summary renders unavailable gray text with no link", () => {
  installDom();
  const { container } = renderWithEn(<SourceAttribution zone="fanwork" sourceOriginalId={5} />);
  const text = container.textContent ?? "";
  assert.match(text, new RegExp(enMessages.sourceAttribution.unavailable));
  assert.equal(container.querySelector("a"), null);
});

/* ------------------------------------------------------------------ */
/* RelatedFanworks — 7 项计划断言（后 3 项 + 创建链接）                   */
/* ------------------------------------------------------------------ */

test("RelatedFanworks hides when total is zero", async () => {
  installDom();
  mockRelatedResponse({ contents: [], total: 0 });
  const { container } = renderWithEn(
    <RelatedFanworks
      sourceContentId={137}
      sourceZone="original"
      titleKey="relatedFanworks.original.title"
      viewAllHref="/original/137/fanworks"
    />,
  );
  await waitFor(() => {
    assert.equal(container.querySelector("[data-slot=related-fanworks]"), null);
  });
});

test("RelatedFanworks shows viewAll only when total is greater than 8 and viewAllHref is present", async () => {
  installDom();
  mockRelatedResponse({
    contents: Array.from({ length: 8 }, (_, index) => relatedCard(index + 1)),
    total: 9,
  });
  const { container } = renderWithEn(
    <RelatedFanworks
      sourceContentId={137}
      sourceZone="original"
      titleKey="relatedFanworks.original.title"
      viewAllHref="/original/137/fanworks"
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector("[data-slot=related-fanworks]"), "row should render");
  });
  const viewAll = container.querySelector(`a[href="/original/137/fanworks"]`);
  assert.ok(viewAll, "expected 查看全部 link when total > 8 and viewAllHref present");
  assert.match(viewAll?.textContent ?? "", new RegExp(enMessages.relatedFanworks.actions.viewAll));
});

test("RelatedFanworks hides viewAll when total is not greater than 8", async () => {
  installDom();
  mockRelatedResponse({
    contents: Array.from({ length: 5 }, (_, index) => relatedCard(index + 1)),
    total: 5,
  });
  const { container } = renderWithEn(
    <RelatedFanworks
      sourceContentId={137}
      sourceZone="original"
      titleKey="relatedFanworks.original.title"
      viewAllHref="/original/137/fanworks"
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector("[data-slot=related-fanworks]"), "row should render");
  });
  assert.equal(container.querySelector(`a[href="/original/137/fanworks"]`), null);
});

test("RelatedFanworks hides viewAll when viewAllHref is absent even if total > 8", async () => {
  installDom();
  mockRelatedResponse({
    contents: Array.from({ length: 8 }, (_, index) => relatedCard(index + 1)),
    total: 9,
  });
  const { container } = renderWithEn(
    <RelatedFanworks
      sourceContentId={229}
      sourceZone="fanwork"
      titleKey="relatedFanworks.derivatives.title"
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector("[data-slot=related-fanworks]"), "row should render");
  });
  const text = container.textContent ?? "";
  assert.ok(!text.includes(enMessages.relatedFanworks.actions.viewAll), "no 查看全部 without viewAllHref");
});

test("derivative row label uses i18n 衍生作品 and never renders 三创", async () => {
  installDom();
  mockRelatedResponse({
    contents: Array.from({ length: 8 }, (_, index) => relatedCard(index + 1)),
    total: 8,
  });
  const { container } = renderWithEn(
    <RelatedFanworks
      sourceContentId={229}
      sourceZone="fanwork"
      titleKey="relatedFanworks.derivatives.title"
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector("[data-slot=related-fanworks]"), "row should render");
  });
  const title = container.querySelector("[data-slot=related-fanworks-title]");
  assert.ok(title, "expected a title element");
  assert.ok(
    (title?.textContent ?? "").startsWith(enMessages.relatedFanworks.derivatives.title),
    "title should use the i18n derivatives label",
  );
  assert.ok(!(container.textContent ?? "").includes("三创"), "never renders 三创");
  assert.equal(zhMessages.relatedFanworks.derivatives.title, "衍生作品");
  const componentSource = read("components/content/RelatedFanworks.tsx");
  assert.ok(!componentSource.includes("三创"), "component source must not hardcode 三创");
});

/* ------------------------------------------------------------------ */
/* RelatedFanworks — 创建链接（Step 4 行为）                             */
/* ------------------------------------------------------------------ */

test("RelatedFanworks original create link points to source_original_id", async () => {
  installDom();
  mockRelatedResponse({
    contents: Array.from({ length: 8 }, (_, index) => relatedCard(index + 1)),
    total: 8,
  });
  const { container } = renderWithEn(
    <RelatedFanworks
      sourceContentId={137}
      sourceZone="original"
      titleKey="relatedFanworks.original.title"
      createHref="/studio/publish/fanwork?source_original_id=137"
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector("[data-slot=related-fanworks]"), "row should render");
  });
  const create = container.querySelector('a[href="/studio/publish/fanwork?source_original_id=137"]');
  assert.ok(create, "expected create link with source_original_id");
  assert.match(create?.textContent ?? "", new RegExp(enMessages.relatedFanworks.actions.create));
});

test("RelatedFanworks fanwork create link points to source_fanwork_id", async () => {
  installDom();
  mockRelatedResponse({
    contents: Array.from({ length: 8 }, (_, index) => relatedCard(index + 1)),
    total: 8,
  });
  const { container } = renderWithEn(
    <RelatedFanworks
      sourceContentId={229}
      sourceZone="fanwork"
      titleKey="relatedFanworks.derivatives.title"
      createHref="/studio/publish/fanwork?source_fanwork_id=229"
    />,
  );
  await waitFor(() => {
    assert.ok(container.querySelector("[data-slot=related-fanworks]"), "row should render");
  });
  const create = container.querySelector('a[href="/studio/publish/fanwork?source_fanwork_id=229"]');
  assert.ok(create, "expected create link with source_fanwork_id");
});
