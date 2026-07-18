import assert from "node:assert/strict";
import test from "node:test";

import { getSeriesDetail, listSeriesCandidates } from "@/lib/series";

test("getSeriesDetail fetches the public series detail endpoint", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ input: string; init?: RequestInit }> = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    calls.push({ input: String(input), init });
    return new Response(
      JSON.stringify({
        series: {
          id: 7,
          title: "山海纪行",
          description: "三章故事",
          zone: "original",
          owner: { id: 1, username: "writer" },
          cover: null,
          cover_content_id: 101,
          item_count: 1,
        },
        items: [
          {
            id: 11,
            sort_order: 0,
            content_item_id: 101,
            content: { id: 101, title: "第一章", zone: "original", status: "published" },
          },
        ],
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  }) as typeof fetch;

  try {
    const detail = await getSeriesDetail(7);
    assert.equal(calls.length, 1);
    assert.equal(calls[0]?.input, "http://localhost:8080/api/v1/series/7");
    assert.equal(calls[0]?.init?.credentials, "include");
    assert.equal(detail.series.title, "山海纪行");
    assert.equal(detail.series.cover_content_id, 101);
    assert.equal(detail.items[0]?.content.title, "第一章");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("listSeriesCandidates calls the authenticated eligible-content endpoint", async () => {
  const originalFetch = globalThis.fetch;
  let requestedURL = "";
  globalThis.fetch = (async (input: string | URL | Request) => {
    requestedURL = String(input);
    return new Response(JSON.stringify({ items: [{ id: 3, title: "Contributed", zone: "original", status: "published" }] }), { status: 200, headers: { "Content-Type": "application/json" } });
  }) as typeof fetch;
  try {
    const items = await listSeriesCandidates("original", "Contributed");
    assert.match(requestedURL, /\/api\/v1\/series\/candidates\?zone=original&q=Contributed/);
    assert.equal(items[0]?.title, "Contributed");
  } finally {
    globalThis.fetch = originalFetch;
  }
});
