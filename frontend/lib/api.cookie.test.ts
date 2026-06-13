import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { JSDOM } from "jsdom";

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
  })) {
    Object.defineProperty(globalThis, key, {
      configurable: true,
      writable: true,
      value,
    });
  }

  return dom;
}

async function loadFreshApiModule() {
  process.env.NEXT_PUBLIC_API_URL = "https://api.leeppp.online";
  const url = pathToFileURL(path.join(process.cwd(), "lib", "api.ts")).href;
  return import(`${url}?t=${Date.now()}-${Math.random()}`);
}

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("api.post uses csrf-token cookie without bootstrapping a csrf fetch first", async () => {
  const dom = installDom();
  document.cookie = "csrf-token=cookie-token";

  const calls: Array<{ url: string; headers: HeadersInit | undefined; credentials: RequestCredentials | undefined }> = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    calls.push({
      url: String(input),
      headers: init?.headers,
      credentials: init?.credentials,
    });
    return jsonResponse(202, { verification_required: true });
  }) as typeof fetch;

  const { api } = await loadFreshApiModule();
  await api.post("/api/v1/auth/register", { email: "new@example.com" });

  assert.equal(calls.length, 1);
  assert.equal(calls[0]?.url, "https://api.leeppp.online/api/v1/auth/register");
  assert.equal((calls[0]?.headers as Record<string, string>)["X-CSRF-Token"], "cookie-token");
  assert.equal(calls[0]?.credentials, "include");

  dom.window.close();
});

test("api.post decodes __Host-csrf cookie values before sending the csrf header", async () => {
  const dom = installDom();
  Object.defineProperty(document, "cookie", {
    configurable: true,
    get: () => "__Host-csrf=encoded%20token",
  });

  const calls: Array<{ headers: HeadersInit | undefined }> = [];
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    calls.push({ headers: init?.headers });
    return jsonResponse(202, { verification_required: true });
  }) as typeof fetch;

  const { api } = await loadFreshApiModule();
  await api.post("/api/v1/auth/register", { email: "new@example.com" });

  assert.equal(calls.length, 1);
  assert.equal((calls[0]?.headers as Record<string, string>)["X-CSRF-Token"], "encoded token");

  dom.window.close();
});
