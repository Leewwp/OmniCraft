import assert from "node:assert/strict";
import test from "node:test";

import { buildCreativeWorkJsonLd, canonicalAlternates, serializeJsonLd } from "./seo";
import { absoluteUrl } from "./site-url";

test("canonical alternates point at the page's own canonical URL", () => {
  assert.deepEqual(canonicalAlternates("/original/12"), { canonical: absoluteUrl("/original/12") });
});

test("canonical alternates go absolute when the site URL is configured", async () => {
  const previous = process.env.NEXT_PUBLIC_SITE_URL;
  process.env.NEXT_PUBLIC_SITE_URL = "https://example.com/";
  try {
    // site-url 在模块加载时冻结 SITE_BASE_URL，用 query 隔离出独立模块实例
    // （运行时 tsx 支持该 specifier，tsc 无对应类型声明）
    // @ts-expect-error -- query-bust import for an isolated module instance
    const mod = await import("./site-url?absolute-canonical");
    assert.equal(mod.absoluteUrl("/ip/3"), "https://example.com/ip/3");
  } finally {
    if (previous === undefined) delete process.env.NEXT_PUBLIC_SITE_URL;
    else process.env.NEXT_PUBLIC_SITE_URL = previous;
  }
});

test("creative work json-ld carries name/headline/url/dateModified", () => {
  const node = buildCreativeWorkJsonLd({
    name: "示例作品",
    url: "https://example.com/content/1",
    dateModified: "2026-11-07T00:00:00Z",
  });
  assert.equal(node["@context"], "https://schema.org");
  assert.equal(node["@type"], "CreativeWork");
  assert.equal(node.name, "示例作品");
  assert.equal(node.headline, "示例作品");
  assert.equal(node.url, "https://example.com/content/1");
  assert.equal(node.dateModified, "2026-11-07T00:00:00Z");
});

test("creative work json-ld omits dateModified when the page has no date", () => {
  const node = buildCreativeWorkJsonLd({ name: "示例 IP", url: "https://example.com/ip/3" });
  assert.ok(!("dateModified" in node));
});

test("json-ld serialization escapes '<' against </script> breakout", () => {
  const json = serializeJsonLd({ name: "a</script><b>", url: "/x" });
  assert.ok(!json.includes("<"));
  assert.equal(JSON.parse(json).name, "a</script><b>");
});
