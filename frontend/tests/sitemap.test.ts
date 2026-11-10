import test from "node:test";
import assert from "node:assert/strict";

import sitemap from "@/app/sitemap";

const originalSiteURL = process.env.NEXT_PUBLIC_SITE_URL;
const originalInternal = process.env.INTERNAL_API_URL;

function restoreEnvironment() {
  if (originalSiteURL === undefined) delete process.env.NEXT_PUBLIC_SITE_URL;
  else process.env.NEXT_PUBLIC_SITE_URL = originalSiteURL;
  if (originalInternal === undefined) delete process.env.INTERNAL_API_URL;
  else process.env.INTERNAL_API_URL = originalInternal;
}

test("sitemap emits numeric-id /ip URLs matching the canonical form (#958)", async () => {
  // /ips/:id 仅收数字 id（INVALID_ID 契约），/ip/[ipId] 参数直接透传——
  // sitemap 与 canonical 都必须是 /ip/{数字 id}，slug 形式站内不可达。
  process.env.INTERNAL_API_URL = "http://api.test";
  delete process.env.NEXT_PUBLIC_SITE_URL;

  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes("/ips")) {
      return new Response(
        JSON.stringify({
          ips: [
            { id: 12, slug: "fitness", updated_at: "2026-11-01T00:00:00Z" },
            { id: 34, slug: "starlit", updated_at: "2026-11-02T00:00:00Z" },
          ],
          total: 2,
        }),
        { status: 200 },
      );
    }
    if (url.includes("/contents")) {
      return new Response(JSON.stringify({ contents: [], total: 0 }), { status: 200 });
    }
    return new Response("{}", { status: 200 });
  }) as typeof fetch;

  try {
    const entries = await sitemap();
    const ipURLs = entries
      .map((e) => e.url)
      .filter((u) => u.includes("/ip/"))
      .sort();
    assert.deepEqual(ipURLs, ["/ip/12", "/ip/34"]);
    for (const url of entries.map((e) => e.url)) {
      assert.ok(!/\/ip\/(fitness|starlit)/.test(url), `slug 形式 URL 不应出现: ${url}`);
    }
  } finally {
    globalThis.fetch = originalFetch;
    restoreEnvironment();
  }
});

test("sitemap keeps content entries on numeric-id URLs (#958 回归保护)", async () => {
  process.env.INTERNAL_API_URL = "http://api.test";
  delete process.env.NEXT_PUBLIC_SITE_URL;

  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes("zone=original")) {
      return new Response(
        JSON.stringify({ contents: [{ id: 7, updated_at: "2026-11-01T00:00:00Z" }], total: 1 }),
        { status: 200 },
      );
    }
    if (url.includes("zone=fanwork")) {
      return new Response(
        JSON.stringify({ contents: [{ id: 9, updated_at: "2026-11-02T00:00:00Z" }], total: 1 }),
        { status: 200 },
      );
    }
    return new Response(JSON.stringify({ ips: [], total: 0 }), { status: 200 });
  }) as typeof fetch;

  try {
    const entries = await sitemap();
    const urls = entries.map((e) => e.url);
    assert.ok(urls.includes("/original/7"));
    assert.ok(urls.includes("/content/9"));
  } finally {
    globalThis.fetch = originalFetch;
    restoreEnvironment();
  }
});
