import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";

const IP_VISIT_PATH = "/api/v1/users/me/ip-visits";

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body></body></html>", {
    url: "https://app.leeppp.online/",
  });

  for (const [key, value] of Object.entries({
    window: dom.window,
    document: dom.window.document,
    navigator: dom.window.navigator,
    HTMLElement: dom.window.HTMLElement,
    Node: dom.window.Node,
    Event: dom.window.Event,
    localStorage: dom.window.localStorage,
  })) {
    Object.defineProperty(globalThis, key, {
      configurable: true,
      writable: true,
      value,
    });
  }
  return dom;
}

interface RecordedCall {
  url: string;
  method: string;
  headers: HeadersInit | undefined;
  body: string | null;
}

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

async function freshModules() {
  // Plain specifiers resolve to the shared cached instances: the lib imports
  // "./api" statically, so both sides must observe the same setAccessToken
  // state and the same fetch stub.
  const apiModule = await import("../lib/api");
  const ipvModule = await import("../lib/ip-visit-history");
  return { api: apiModule.api, setAccessToken: apiModule.setAccessToken, ipv: ipvModule };
}

function seedLocalIps(items: Array<{ id: number; name: string }>) {
  localStorage.setItem("recent_ips", JSON.stringify(items));
}

function localIps(): Array<{ id: number; name: string }> {
  return JSON.parse(localStorage.getItem("recent_ips") ?? "[]");
}

test("readLocalRecentIps parses the legacy structure with dedupe and a six-item cap", async () => {
  const dom = installDom();
  const { ipv } = await freshModules();
  seedLocalIps([
    { id: 1, name: "A" },
    { id: 2, name: "B" },
    { id: 1, name: "A dup" },
    { id: 3, name: "C" },
    { id: 4, name: "D" },
    { id: 5, name: "E" },
    { id: 6, name: "F" },
    { id: 7, name: "G" },
    { id: 8, name: "H" },
  ]);

  const items = ipv.readLocalRecentIps();
  assert.deepEqual(
    items.map((it) => it.id),
    [1, 2, 3, 4, 5, 6]
  );
  assert.equal(items[0].name, "A");
  dom.window.close();
});

test("recordIpVisit writes local dedupe and cap for anonymous users without API calls", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken(null);
  document.cookie = "csrf-token=cookie-token";

  const calls: RecordedCall[] = [];
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(_input), method: (init?.method ?? "GET").toUpperCase(), headers: init?.headers, body: init?.body ? String(init.body) : null });
    return jsonResponse(204, null);
  }) as typeof fetch;

  for (let i = 1; i <= 8; i++) {
    ipv.recordIpVisit({ id: i, name: `IP${i}` });
  }
  ipv.recordIpVisit({ id: 3, name: "IP3 again" });
  await new Promise((resolve) => setTimeout(resolve, 10));

  assert.deepEqual(
    localIps().map((it) => it.id),
    [3, 8, 7, 6, 5, 4]
  );
  assert.deepEqual(calls, []);
  dom.window.close();
});

test("recordIpVisit records locally and best-effort PUTs when signed in", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken("test-token");
  document.cookie = "csrf-token=cookie-token";

  const calls: RecordedCall[] = [];
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(_input), method: (init?.method ?? "GET").toUpperCase(), headers: init?.headers, body: init?.body ? String(init.body) : null });
    return jsonResponse(204, null);
  }) as typeof fetch;

  ipv.recordIpVisit({ id: 9, name: "IP9" });
  await new Promise((resolve) => setTimeout(resolve, 10));

  assert.deepEqual(localIps(), [{ id: 9, name: "IP9" }]);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "PUT");
  assert.ok(calls[0].url.endsWith("/api/v1/users/me/ip-visits/9"));

  setAccessToken(null);
  dom.window.close();
});

test("recordIpVisit keeps the local record when the PUT fails", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken("test-token");
  document.cookie = "csrf-token=cookie-token";
  globalThis.fetch = (async () => jsonResponse(500, { code: "IP_VISIT_RECORD_FAILED", message: "failed" })) as typeof fetch;

  ipv.recordIpVisit({ id: 10, name: "IP10" });
  await new Promise((resolve) => setTimeout(resolve, 10));

  assert.deepEqual(localIps(), [{ id: 10, name: "IP10" }]);
  setAccessToken(null);
  dom.window.close();
});

test("mergeLocalIpsIntoAccount sends local visits and clears only acknowledged ids on 200", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken("test-token");
  document.cookie = "csrf-token=cookie-token";
  seedLocalIps([
    { id: 1, name: "A" },
    { id: 2, name: "B" },
    { id: 3, name: "C" },
    { id: 4, name: "D" },
  ]);

  let mergeBody: unknown = null;
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    if (String(_input).includes("/ip-visits/merge")) {
      mergeBody = JSON.parse(String(init?.body));
      return jsonResponse(200, { accepted_ip_ids: [1, 2], discarded_ip_ids: [4], items: [] });
    }
    return jsonResponse(200, {});
  }) as typeof fetch;

  const result = await ipv.mergeLocalIpsIntoAccount();

  assert.deepEqual(result, { accepted: [1, 2], discarded: [4] });
  assert.ok(mergeBody && typeof mergeBody === "object" && Array.isArray((mergeBody as { visits: unknown[] }).visits));
  assert.deepEqual(
    ((mergeBody as { visits: Array<{ ip_id: number }> }).visits).map((v) => v.ip_id).sort(),
    [1, 2, 3, 4]
  );
  assert.deepEqual(localIps().map((it) => it.id), [3]);
  setAccessToken(null);
  dom.window.close();
});

test("mergeLocalIpsIntoAccount retains local records on failure", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken("test-token");
  document.cookie = "csrf-token=cookie-token";
  seedLocalIps([{ id: 1, name: "A" }]);

  globalThis.fetch = (async () => jsonResponse(500, { code: "IP_VISIT_MERGE_FAILED", message: "failed" })) as typeof fetch;
  assert.equal(await ipv.mergeLocalIpsIntoAccount(), null);
  assert.deepEqual(localIps(), [{ id: 1, name: "A" }]);

  globalThis.fetch = (async () => {
    throw new TypeError("Failed to fetch");
  }) as typeof fetch;
  assert.equal(await ipv.mergeLocalIpsIntoAccount(), null);
  assert.deepEqual(localIps(), [{ id: 1, name: "A" }]);
  setAccessToken(null);
  dom.window.close();
});

test("mergeLocalIpsIntoAccount is a no-op without local items or auth", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  document.cookie = "csrf-token=cookie-token";
  setAccessToken("test-token");
  localStorage.clear();
  assert.equal(await ipv.mergeLocalIpsIntoAccount(), null);

  seedLocalIps([{ id: 1, name: "A" }]);
  setAccessToken(null);
  assert.equal(await ipv.mergeLocalIpsIntoAccount(), null);
  assert.deepEqual(localIps(), [{ id: 1, name: "A" }]);
  dom.window.close();
});

test("loadRecentIps reads local history for anonymous users", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken(null);
  seedLocalIps([{ id: 1, name: "A" }]);

  let fetched = false;
  globalThis.fetch = (async () => {
    fetched = true;
    return jsonResponse(200, {});
  }) as typeof fetch;

  const items = await ipv.loadRecentIps();
  assert.deepEqual(items, [{ id: 1, name: "A" }]);
  assert.equal(fetched, false);
  dom.window.close();
});

test("loadRecentIps reads account history for signed-in users", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken("test-token");
  seedLocalIps([{ id: 1, name: "A" }]);

  globalThis.fetch = (async (_input: string | URL | Request) => {
    if (String(_input).includes(IP_VISIT_PATH)) {
      return jsonResponse(200, {
        items: [
          { ip: { id: 2, name: "B" }, visited_at: "2026-08-12T10:00:00Z" },
          { ip: { id: 3, name: "C" }, visited_at: "2026-08-12T09:00:00Z" },
        ],
        limit: 6,
      });
    }
    return jsonResponse(200, {});
  }) as typeof fetch;

  const items = await ipv.loadRecentIps();
  assert.deepEqual(items, [
    { id: 2, name: "B" },
    { id: 3, name: "C" },
  ]);
  setAccessToken(null);
  dom.window.close();
});

test("loadRecentIps falls back to local on server failure or empty server history", async () => {
  const dom = installDom();
  const { ipv, setAccessToken } = await freshModules();
  setAccessToken("test-token");
  seedLocalIps([{ id: 1, name: "A" }]);

  globalThis.fetch = (async () => jsonResponse(500, { code: "IP_VISIT_LIST_FAILED", message: "failed" })) as typeof fetch;
  assert.deepEqual(await ipv.loadRecentIps(), [{ id: 1, name: "A" }]);

  globalThis.fetch = (async (_input: string | URL | Request) => {
    if (String(_input).includes(IP_VISIT_PATH)) {
      return jsonResponse(200, { items: [], limit: 6 });
    }
    return jsonResponse(200, {});
  }) as typeof fetch;
  assert.deepEqual(await ipv.loadRecentIps(), [{ id: 1, name: "A" }]);

  localStorage.clear();
  assert.deepEqual(await ipv.loadRecentIps(), []);
  setAccessToken(null);
  dom.window.close();
});
