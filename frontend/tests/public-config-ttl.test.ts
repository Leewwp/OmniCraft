import assert from "node:assert/strict";
import test from "node:test";

import { clearPublicConfigCache, fetchPublicConfig } from "@/lib/public-config";

const sampleConfig = {
  features: {
    web_agent_enabled: true,
    payment_enabled: false,
    creator_support_enabled: false,
    desktop_deploy_enabled: false,
  },
  captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
  client: { download_enabled: false, download_url: "", latest_version: "" },
  legal: { current_terms_version: "1", current_privacy_version: "1" },
  upload: {
    image_gallery_min_items: 1,
    image_gallery_max_items: 9,
    video_gallery_min_items: 0,
    video_gallery_max_items: 3,
  },
  collaboration: { max_invitees_per_publish: 3 },
  oss_domain: "",
};

function okResponse(): Response {
  return new Response(JSON.stringify(sampleConfig), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

test("public config cache expires after the TTL window", async () => {
  clearPublicConfigCache();
  const originalFetch = globalThis.fetch;
  const originalNow = Date.now;
  let calls = 0;
  globalThis.fetch = (async () => {
    calls += 1;
    return okResponse();
  }) as typeof fetch;
  try {
    let now = 1_000_000;
    Date.now = () => now;

    const first = await fetchPublicConfig();
    assert.equal(calls, 1);
    assert.equal(first.features.web_agent_enabled, true);

    // within the TTL window the cache is served without a network call
    now += 4 * 60 * 1000;
    await fetchPublicConfig();
    assert.equal(calls, 1, "cached config within 5min must not refetch");

    // past the TTL the next call refetches (runtime flag flips become visible)
    now += 2 * 60 * 1000;
    await fetchPublicConfig();
    assert.equal(calls, 2, "expired cache must refetch");

    // explicit invalidation also forces a refetch
    clearPublicConfigCache();
    await fetchPublicConfig();
    assert.equal(calls, 3);
  } finally {
    globalThis.fetch = originalFetch;
    Date.now = originalNow;
    clearPublicConfigCache();
  }
});

test("failed fetch does not poison the cache with stale-success state", async () => {
  clearPublicConfigCache();
  const originalFetch = globalThis.fetch;
  let fail = true;
  globalThis.fetch = (async () => {
    if (fail) return new Response("boom", { status: 503 });
    return okResponse();
  }) as typeof fetch;
  try {
    await assert.rejects(() => fetchPublicConfig(), /503/);
    fail = false;
    const cfg = await fetchPublicConfig();
    assert.equal(cfg.features.web_agent_enabled, true);
  } finally {
    globalThis.fetch = originalFetch;
    clearPublicConfigCache();
  }
});
