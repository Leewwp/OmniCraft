import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { resolveDefaultSort } from "@/lib/search-filters";

// #81 排序默认值语义：无分类且未显式选排序 → recommended；有分类 → hot；
// 显式排序原样保留（深链 recommended+分类 由后端防御性降级）。
test("resolveDefaultSort: no category and no explicit sort defaults to recommended", () => {
  assert.equal(resolveDefaultSort({}), "recommended");
  assert.equal(resolveDefaultSort({ category: "" }), "recommended");
  assert.equal(resolveDefaultSort({ sort: "" }), "recommended");
});

test("resolveDefaultSort: category without explicit sort defaults to hot", () => {
  assert.equal(resolveDefaultSort({ category: "film_tv" }), "hot");
  assert.equal(resolveDefaultSort({ category: "gaming" }), "hot");
  assert.equal(resolveDefaultSort({ category: "film_tv", sort: "" }), "hot");
  assert.equal(resolveDefaultSort({ category: " film_tv " }), "hot");
});

test("resolveDefaultSort: explicit non-recommended sort is preserved with or without category", () => {
  assert.equal(resolveDefaultSort({ category: "film_tv", sort: "hot" }), "hot");
  assert.equal(resolveDefaultSort({ category: "film_tv", sort: "most_views" }), "most_views");
  assert.equal(resolveDefaultSort({ category: "film_tv", sort: "newest" }), "newest");
  assert.equal(resolveDefaultSort({ sort: "newest" }), "newest");
  assert.equal(resolveDefaultSort({ sort: " most_views " }), "most_views");
});

test("resolveDefaultSort: explicit recommended passes through (stale deep link, backend degrades)", () => {
  assert.equal(resolveDefaultSort({ category: "film_tv", sort: "recommended" }), "recommended");
  assert.equal(resolveDefaultSort({ sort: "recommended" }), "recommended");
});

test("original page delegates sort resolution to resolveDefaultSort and drops the dead code", async () => {
  const source = await readFile(new URL("../app/(public)/original/page.tsx", import.meta.url), "utf8");
  assert.match(source, /resolveDefaultSort/);
  assert.match(source, /sort: raw\.sort \|\| ""/);
  assert.doesNotMatch(source, /\(search\.sort \|\| "hot"\)/);
  assert.doesNotMatch(source, /sort: raw\.sort \|\| "recommended"/);
});
