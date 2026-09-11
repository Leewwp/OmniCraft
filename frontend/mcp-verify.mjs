#!/usr/bin/env node
// SP-16 #449 MCP smoke rig: a dependency-free Streamable HTTP JSON-RPC client
// that exercises the anonymous tools against a live OmniCraft stack.
//
//   node frontend/mcp-verify.mjs [baseURL]      # default http://127.0.0.1:8080
//
// Acceptance (#449): a real MCP client connects to the local stack and runs
// the natural-language smoke — 「找乐谱」→ search results → usage guide with
// requirements/steps/safety. Non-public invisibility is enforced by the Go
// test suite (internal/mcpserver/server_test.go).

const baseURL = process.argv[2] || "http://127.0.0.1:8080";
const endpoint = `${baseURL.replace(/\/$/, "")}/api/v1/mcp`;

let sessionId = null;
let nextId = 1;

async function rpc(method, params, { notification = false } = {}) {
  const body = notification
    ? { jsonrpc: "2.0", method }
    : { jsonrpc: "2.0", id: nextId++, method, params };
  const res = await fetch(endpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json, text/event-stream",
      ...(sessionId ? { "Mcp-Session-Id": sessionId } : {}),
    },
    body: JSON.stringify(body),
  });
  const newSession = res.headers.get("mcp-session-id");
  if (newSession) sessionId = newSession;
  if (!res.ok) {
    throw new Error(`${method}: HTTP ${res.status}: ${(await res.text()).slice(0, 200)}`);
  }
  if (notification) return null;
  const contentType = res.headers.get("content-type") || "";
  const raw = await res.text();
  let payload = null;
  if (contentType.includes("text/event-stream")) {
    for (const line of raw.split("\n")) {
      if (!line.startsWith("data:")) continue;
      const data = line.slice(5).trim();
      if (!data) continue;
      try {
        const parsed = JSON.parse(data);
        if (parsed.id === body.id) payload = parsed;
      } catch {
        /* keep scanning SSE frames */
      }
    }
  } else {
    payload = JSON.parse(raw);
  }
  if (!payload) throw new Error(`${method}: no JSON-RPC response with id ${body.id}`);
  if (payload.error) throw new Error(`${method}: ${JSON.stringify(payload.error)}`);
  return payload.result;
}

function toolText(result) {
  if (!result || result.isError) {
    throw new Error(`tool error: ${JSON.stringify(result).slice(0, 200)}`);
  }
  return (result.content || [])
    .filter((c) => c.type === "text")
    .map((c) => c.text)
    .join("\n");
}

const failures = [];
function check(label, ok, detail) {
  if (ok) console.log(`ok    ${label}`);
  else {
    console.error(`FAIL  ${label}${detail ? ` — ${detail}` : ""}`);
    failures.push(label);
  }
}

// 1. initialize
const init = await rpc("initialize", {
  protocolVersion: "2025-06-18",
  capabilities: {},
  clientInfo: { name: "omnicraft-mcp-verify", version: "0.0.1" },
});
check("initialize answered", Boolean(init && init.serverInfo), JSON.stringify(init).slice(0, 120));
await rpc("notifications/initialized", undefined, { notification: true });

// 2. tools/list — exactly the four anonymous read-only tools
const tools = await rpc("tools/list", {});
const names = (tools.tools || []).map((t) => t.name).sort();
check(
  "tools/list = 4 anonymous read-only tools",
  JSON.stringify(names) ===
    JSON.stringify(
      ["omnicraft_get_content", "omnicraft_get_usage_guide", "omnicraft_list_categories", "omnicraft_search"].sort(),
    ),
  names.join(","),
);

// 3. 「找乐谱」→ search
const keyword = "乐谱";
const search = await rpc("tools/call", {
  name: "omnicraft_search",
  arguments: { query: keyword, page_size: 5 },
});
const searchText = toolText(search);
let searchPayload = null;
try {
  searchPayload = JSON.parse(searchText);
} catch {
  /* fall through to text assertions */
}
check(
  "search returned items",
  Boolean(searchPayload && Array.isArray(searchPayload.items) && searchPayload.items.length > 0),
  searchText.slice(0, 160),
);

// 4. usage guide for the first hit — requirements/steps/safety present
const first = searchPayload && searchPayload.items[0];
const guide = await rpc("tools/call", {
  name: "omnicraft_get_usage_guide",
  arguments: { content_id: first.id, locale: "zh" },
});
const guideText = toolText(guide);
let guidePayload = null;
try {
  guidePayload = JSON.parse(guideText);
} catch {
  /* text assertions below */
}
check("guide has requirements", guidePayload ? Array.isArray(guidePayload.requirements) : /前置要求|requirements/i.test(guideText));
check("guide has steps", guidePayload ? Array.isArray(guidePayload.steps) && guidePayload.steps.length > 0 : /使用步骤|steps/i.test(guideText));
check("guide keeps safety floor", guidePayload ? Array.isArray(guidePayload.safety) && guidePayload.safety.length > 0 : /安全提示|safety/i.test(guideText));

// 5. categories answer
const cats = await rpc("tools/call", { name: "omnicraft_list_categories", arguments: {} });
check("categories answered", cats !== null, "");

if (failures.length > 0) {
  console.error(`\nmcp-verify: ${failures.length} failure(s)`);
  process.exit(1);
}
console.log("\nmcp-verify: all checks passed");
