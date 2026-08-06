import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React from "react";
import enMessages from "@/messages/en.json";
import { IntlProvider } from "use-intl";
import { ContentCard } from "@/components/content/ContentCard";
import { IPCard } from "@/components/ip/IPCard";
import { cleanup, installDom, render } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

test.afterEach(() => cleanup());

test("content and IP cards render distinct zones and keyboard links", () => {
  installDom();
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentCard data={{ id: 1, title: "Original work", zone: "original", author: { username: "Ada" }, like_count: 2 }} />
      <ContentCard data={{ id: 2, title: "Fanwork remix", zone: "fanwork", content_type: "image", author: { username: "Bea" }, ip: { name: "Indigo IP" }, tags: ["art", "study", "ignored"], like_count: 3, comment_count: 1 }} />
      <IPCard data={{ id: 7, name: "Browse IP" }} variant="browse" />
      <IPCard data={{ id: 8, name: "List IP" }} variant="list" />
    </IntlProvider>,
  );

  const original = view.getByRole("link", { name: "Original work" });
  const fanwork = view.getByRole("link", { name: "Fanwork remix" });
  assert.doesNotMatch(original.className, /border border-border/);
  assert.match(fanwork.className, /border border-border/);
  assert.match(fanwork.textContent ?? "", /Indigo IP/);
  assert.ok(view.getByText("art"));
  assert.ok(view.getByText("study"));
  assert.equal(view.queryByText("ignored"), null);
  assert.equal(view.getAllByRole("link").filter((link) => link.getAttribute("href")?.startsWith("/ip/")).length, 2);
  original.focus();
  assert.equal(document.activeElement, original);
});

test("approved IP library contract fills responsive tracks without fixed card widths", () => {
  const spec = read("../design/ui-spec.md");
  const browse = read("components/ip/IPBrowseClient.tsx");
  const card = read("components/ip/IPCard.tsx");

  assert.match(spec, /## Page: \/ips IP 库/);
  assert.match(spec, /320\/375px.*两列/);
  assert.match(spec, /1280\/1440px/);
  assert.match(spec, /16:10/);
  assert.doesNotMatch(card, /w-\[156px\]/);
  assert.match(card, /w-full min-w-0/);
  assert.match(browse, /grid-cols-2/);
  assert.match(browse, /auto-(?:fit|fill).*minmax/);
  assert.match(browse, /gap-3/);
  assert.match(browse, /gap-4/);
  assert.match(browse, /setIPs\(\(current\) => append \? \[\.\.\.current, \.\.\.incoming\]/);
  assert.match(browse, /parseIPListResponse\(await res\.json\(\)\)/);
  assert.match(browse, /if \(await fetchIPs\(category, sort, search, nextPage, true\)\) \{\s*setPage\(nextPage\)/);
});

test("content discovery uses approved typography, icons, and semantic Indigo tokens", () => {
  const card = read("components/content/ContentCard.tsx");
  const sidebar = read("components/content/ContentSidebar.tsx");
  const browse = read("components/ip/IPBrowseClient.tsx");

  assert.doesNotMatch(card, /<svg/);
  assert.doesNotMatch(sidebar, /<svg/);
  assert.match(card, /text-sm/);
  assert.match(card, /text-xs/);
  assert.match(card, /!isOriginal && data\.ip\?\.name/);
  assert.match(card, /!isOriginal && data\.description/);
  assert.match(card, /!isOriginal && tags\.length > 0/);
  assert.match(card, /<TagBadge key=\{tag\}>/);
  assert.match(card, /isOriginal \? "group-hover:scale-105" : "group-hover:scale-\[1\.03\]"/);
  assert.match(card, /hover:shadow-\[var\(--elevation-2\)\]/);
  assert.doesNotMatch(card, /text-\[(?:10|10\.5|11|11\.5|12\.5|13\.5)px\]/);
  assert.doesNotMatch(sidebar, /text-\[(?:10|10\.5|11|11\.5|12\.5|13\.5|15)px\]/);
  assert.doesNotMatch(sidebar, /(?:text|bg|border)-(?:sky|violet)-/);
  assert.doesNotMatch(sidebar, /rounded-xl/);
  assert.doesNotMatch(browse, /accent-subtle\/5/);
  assert.doesNotMatch(browse, /text-\[22px\]|text-\[13px\]/);
});

test("search and home preserve responsive, keyboard, loading, empty, and error contracts", () => {
  const home = read("components/home/HomePageClient.tsx");
  const facets = read("components/layout/FacetedSearchSidebar.tsx");
  const search = read("app/(public)/search/page.tsx");

  assert.match(search, /w-\[85vw\]/);
  assert.match(search, /shadow-\[var\(--elevation-3\)\]/);
  assert.match(search, /w-\[228px\].*min-\[1101px\]:w-\[260px\]/);
  assert.match(search, /role="dialog"/);
  assert.match(search, /focus:ring-2/);
  assert.match(search, /search\.gridView/);
  assert.match(search, /search\.listView/);
  assert.match(search, /pointer:coarse.*min-h-11/);
  assert.match(search, /e\.key === "Tab"/);
  assert.match(search, /<Skeleton/);
  assert.match(search, /<EmptyState/);
  assert.match(search, /common\.retry/);
  assert.match(home, /<OverlayMasonryGrid/);
  assert.match(facets, /focus:ring-2/);
  assert.match(facets, /shadow-none/);
  assert.doesNotMatch(facets, /"flex flex-col border border-border rounded-md bg-card p-4 gap-4[^\"]*shadow-/);
  assert.match(read("components/ip/IPCard.tsx"), /dark:bg-canvas-subtle/);
  assert.match(read("components/ip/IPCard.tsx"), /focus-visible:ring-2/);
  assert.match(read("components/ip/IPCard.tsx"), /motion-reduce:transform-none/);
  assert.match(read("components/ip/IPCard.tsx"), /variant === "browse"/);
  assert.match(read("components/ip/IPCard.tsx"), /hover:shadow-\[var\(--elevation-2\)\]/);
});
