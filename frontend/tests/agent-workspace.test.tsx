import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { clearPublicConfigCache } from "@/lib/public-config";
import { ToastProvider } from "@/components/ui/Toast";
import { within } from "@testing-library/react";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return readFile(path.join(root, relativePath), "utf8");
}

/* Native <dialog> showModal/close are not implemented in jsdom; stub the
   modal lifecycle (same pattern as content-detail-overlay.test.tsx). */
function installOverlayTestStubs() {
  const prototype = window.HTMLDialogElement?.prototype as unknown as HTMLDialogElement | undefined;
  if (!prototype) return;
  prototype.showModal = function showModalStub(this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  prototype.close = function closeStub(this: HTMLDialogElement) {
    this.removeAttribute("open");
  };
  window.scrollTo = () => undefined;
}

/* Stub next/navigation + AuthContext so ContentDetail/FollowButton (inside the
   shared ContentDetailOverlay) render without providers, following the
   established Module._load interception pattern. Components must be imported
   dynamically after this patch (see test.before). */
const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
let authUser: {
  id: number;
  username: string;
  email: string;
  email_verified_at: string | null;
} | null = null;

Module._load = function loadWithNavigationStub(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({ push: () => undefined }),
      usePathname: () => "/agent",
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => ({
        user: authUser,
        isLoading: false,
        unreadCounts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 },
        capabilities: { can_interact: true, interaction_denial_reason: null },
        login: async () => undefined,
        logout: async () => undefined,
        refresh: async () => true,
        refreshUser: async () => undefined,
      }),
      interactionDenialKey: (reason?: string) => {
        switch (reason) {
          case "user_banned":
            return "capabilities.deniedBanned";
          case "email_not_verified":
            return "capabilities.deniedEmailNotVerified";
          case "insufficient_reputation":
            return "capabilities.deniedInsufficientReputation";
          default:
            return "capabilities.deniedUnknown";
        }
      },
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

const workspaceMessages = {
  ...enMessages,
  agent: {
    ...(enMessages.agent ?? {}),
    featureDisabledTitle: "Agent unavailable",
    featureDisabledDescription: "Agent is not enabled for this account.",
    workspace: {
      sidebarLabel: "Conversation history",
      collapseSidebar: "Collapse sidebar",
      expandSidebar: "Expand sidebar",
      newConversation: "Start new conversation",
      openConversations: "Open conversation list",
      closeConversations: "Close conversation list",
      emptyConversations: "No conversations yet",
      untitled: "Conversation",
      privacyHint: "Only published content visible to your account is searched.",
      groupToday: "Today",
      groupYesterday: "Yesterday",
      groupEarlier: "Earlier",
      transcriptLabel: "Chat transcript",
      composerLabel: "Ask the agent",
      inputPlaceholder: "Describe what you want to find",
      send: "Send",
      stop: "Stop",
      sendMessage: "Send message",
      stopGenerating: "Stop generating",
      clearHistory: "Clear conversation",
      clearHistoryConfirmTitle: "Clear this conversation?",
      clearHistoryConfirmDescription: "All messages in this conversation will be deleted.",
      clearHistoryConfirmAction: "Clear history",
      clearHistorySuccess: "Conversation cleared",
      clearHistoryFailed: "Failed to clear conversation",
      emptyTitle: "Start researching from site content",
      emptyDescription: "Ask about works, sources, usage or directions.",
      jumpToLatest: "Jump to latest",
      errorTitle: "This request was not completed",
      errorRetry: "Resend",
      conversationLoadFailed: "Failed to load conversation",
      composerHint: "Enter to send · Shift+Enter for newline",
      stoppedNotice: "Stopped generating",
      suggestionLayout:
        'Looking for "Empty Platform" themed visual references, ideally with both atmosphere images and text narration.',
      suggestionMusic: "Want to understand how chords relate to the melody with the shortest practice.",
      suggestionMod: "Find beginner-friendly furniture mods",
    },
    tools: {
      title: "Tool activity",
      searchContent: "Search site content",
      getContentDetail: "Fetch content detail",
      getUsageGuide: "Fetch usage guide",
      suggestPublishMetadata: "Suggest publish metadata",
      unknown: "Tool",
      statusRunning: "Running",
      statusSuccess: "Done",
      statusFailed: "Failed",
    },
    citations: {
      title: "Site references",
      count: "{count} verified references",
      invalid: "Reference unavailable",
    },
    noEvidence: {
      title: "Not enough evidence",
      description: "No public content supports this answer.",
    },
    a11y: {
      conversationItem: "Conversation #{id}",
      streamStatus: "Generating answer",
      jumpToLatest: "Jump to latest message",
    },
  },
} as const;

function renderWithIntl(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={workspaceMessages}>
      <ToastProvider>{node}</ToastProvider>
    </IntlProvider>,
  );
}

type ApiCall = { path: string; method: string; body?: unknown };

async function flushAsyncUpdates() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

type ApiStubEntry = {
  method: "GET" | "DELETE";
  path: string;
  response?: unknown;
  error?: ApiRequestError;
};

const originalGet = api.get;
const originalDelete = api.delete;

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.delete = originalDelete;
  authUser = null;
  window.localStorage.clear();
  delete (globalThis as Record<string, unknown>).fetch;
});

function installApiMock(entries: ApiStubEntry[]) {
  const calls: ApiCall[] = [];
  function route(callPath: string, method: "GET" | "DELETE"): unknown {
    calls.push({ path: callPath, method });
    const candidates = entries.filter(
      (entry) => (entry.method ?? "GET") === method && callPath.includes(entry.path),
    );
    const exact = candidates.find((entry) => callPath === entry.path);
    const match = exact ?? candidates.sort((a, b) => b.path.length - a.path.length)[0];
    if (!match) throw new ApiRequestError("NOT_FOUND", "not found", 404);
    if (match.error) throw match.error;
    return match.response;
  }
  api.get = (async (callPath: string) => route(callPath, "GET")) as typeof api.get;
  api.delete = (async (callPath: string) => route(callPath, "DELETE")) as typeof api.delete;
  return calls;
}

function conversation(id: number, updatedAt: string) {
  return { id, context_type: "general", created_at: updatedAt, updated_at: updatedAt };
}

function sseResponse(events: Array<Record<string, unknown>>): Response {
  const payload = events.map((event) => `data: ${JSON.stringify(event)}\n`).join("");
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode(payload));
      controller.close();
    },
  });
  return { ok: true, status: 200, body: stream } as unknown as Response;
}

function installSSEFetch(events: Array<Record<string, unknown>>) {
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    calls.push(String(input));
    return sseResponse(events);
  }) as typeof fetch;
  return { calls, restore: () => (globalThis.fetch = originalFetch) };
}

/* SSE stub that keeps the stream open and errors it when the AbortSignal
   fires, so the workspace can be stopped mid-stream (stop button path). */
function installAbortableSSEFetch(events: Array<Record<string, unknown>>) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    const signal = init?.signal ?? null;
    const payload = events.map((event) => `data: ${JSON.stringify(event)}\n`).join("");
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode(payload));
        signal?.addEventListener("abort", () =>
          controller.error(new DOMException("Aborted", "AbortError")),
        );
      },
    });
    return { ok: true, status: 200, body: stream } as unknown as Response;
  }) as typeof fetch;
  return { restore: () => (globalThis.fetch = originalFetch) };
}

function streamEvents(): Array<Record<string, unknown>> {
  return [
    { type: "start", trace_id: "t1", conversation_id: 7, answer_kind: "grounded_content" },
    { type: "tool_status", tool: { name: "search_content", status: "success", duration_ms: 42 } },
    { type: "delta", delta: "hello " },
    { type: "delta", delta: "world" },
    {
      type: "done",
      conversation_id: 7,
      answer_kind: "grounded_content",
      answer: "hello world",
      citations: [
        { content_id: 3, title: "Cited content", zone: "original", excerpt: "excerpt line" },
      ],
      tools: [{ name: "search_content", status: "success", duration_ms: 42 }],
      degraded: false,
    },
  ];
}

const CONTENT_DETAIL = {
  content: {
    id: 3,
    title: "Cited content",
    zone: "original",
    content_type: "article",
    author: { id: 9, username: "Original Author" },
    status: "published",
    description: "Original body",
    like_count: 7,
  },
  attachments: [],
  tags: [],
};

let AgentWorkspace: typeof import("@/components/agent/AgentWorkspace")["AgentWorkspace"];
let AgentFeatureGate: typeof import("@/components/agent/AgentFeatureGate")["AgentFeatureGate"];
import type { AgentStreamEvent } from "@/lib/agent-stream";
let parseAgentStreamLine: typeof import("@/lib/agent-stream")["parseAgentStreamLine"];
let startAgentStream: typeof import("@/lib/agent-stream")["startAgentStream"];

test.before(async () => {
  installOverlayTestStubs();
  const workspaceModule = await import("@/components/agent/AgentWorkspace");
  const gateModule = await import("@/components/agent/AgentFeatureGate");
  const streamModule = await import("@/lib/agent-stream");
  AgentWorkspace = workspaceModule.AgentWorkspace;
  AgentFeatureGate = gateModule.AgentFeatureGate;
  parseAgentStreamLine = streamModule.parseAgentStreamLine;
  startAgentStream = streamModule.startAgentStream;
});

/* ---------- SSE 行解析（lib/agent-stream.ts） ---------- */

test("parseAgentStreamLine decodes typed SSE events and ignores non-data lines", () => {
  assert.deepEqual(parseAgentStreamLine('data: {"type":"start","conversation_id":7}'), {
    type: "start",
    conversation_id: 7,
  });
  assert.deepEqual(parseAgentStreamLine('data: {"type":"delta","delta":"hi"}'), {
    type: "delta",
    delta: "hi",
  });
  assert.deepEqual(
    parseAgentStreamLine(
      'data: {"type":"done","answer_kind":"grounded_content","citations":[{"content_id":3,"title":"T","zone":"original"}]}',
    ),
    {
      type: "done",
      answer_kind: "grounded_content",
      citations: [{ content_id: 3, title: "T", zone: "original" }],
    },
  );
  assert.deepEqual(parseAgentStreamLine('data: {"type":"error","error_code":"AGENT_PROVIDER_ERROR"}'), {
    type: "error",
    error_code: "AGENT_PROVIDER_ERROR",
  });
  assert.equal(parseAgentStreamLine("data: [DONE]"), null);
  assert.equal(parseAgentStreamLine("event: ping"), null);
  assert.equal(parseAgentStreamLine("data: not-json"), null);
});

test("startAgentStream POSTs the surface contract and emits events in order", async () => {
  const stub = installSSEFetch(streamEvents());
  const events: AgentStreamEvent[] = [];
  let closed = false;
  await startAgentStream(
    fetch,
    "http://api.test/api/v1/agent/chat/stream",
    {
      messages: [{ role: "user", content: "hi" }],
      context: { surface: "global" },
    },
    {
      onEvent: (event) => events.push(event),
      onClose: () => {
        closed = true;
      },
    },
  );
  assert.match(stub.calls[0], /\/api\/v1\/agent\/chat\/stream$/);
  assert.equal(events.length, 5);
  assert.deepEqual(events[0], { type: "start", trace_id: "t1", conversation_id: 7, answer_kind: "grounded_content" });
  assert.deepEqual(events[3], { type: "delta", delta: "world" });
  assert.equal(events[4].type, "done");
  assert.equal((events[4] as { citations?: unknown[] }).citations?.length, 1);
  assert.ok(closed);
});

test("startAgentStream reports non-OK responses as errors", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => ({ ok: false, status: 503 })) as unknown as typeof fetch;
  try {
    let error: Error | null = null;
    await startAgentStream(
      fetch,
      "http://api.test/api/v1/agent/chat/stream",
      {},
      { onEvent: () => undefined, onError: (err) => (error = err) },
    );
    assert.ok(error, "non-OK stream must surface an error");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

/* ---------- AgentWorkspace：会话列表与切换 ---------- */

test("workspace lists conversations and loads messages on select", async () => {
  installDom();
  const now = new Date();
  const today = now.toISOString();
  const yesterday = new Date(now.getTime() - 86400000).toISOString();
  const calls = installApiMock([
    {
      method: "GET",
      path: "/api/v1/agent/conversations",
      response: { conversations: [conversation(1, today), conversation(2, yesterday)] },
    },
    {
      method: "GET", path: "/api/v1/agent/conversations/2",
      response: {
        conversation: conversation(2, yesterday),
        messages: [
          { id: 11, conversation_id: 2, role: "user", content: "where can I find mod guides" },
          { id: 12, conversation_id: 2, role: "assistant", content: "the usage guide is on the mod page" },
        ],
      },
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByText("Today")));
  assert.ok(view.getByText("Yesterday"));
  assert.ok(view.getByRole("button", { name: "Conversation #1" }));

  fireEvent.click(view.getByRole("button", { name: "Conversation #2" }));
  await waitFor(() => assert.ok(view.getByText("where can I find mod guides")));
  assert.ok(view.getByText("the usage guide is on the mod page"));
  assert.ok(calls.some((call) => call.path.includes("/api/v1/agent/conversations/2")));
});

test("empty conversation list shows the empty state", async () => {
  installDom();
  installApiMock([{ method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } }]);
  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByText("No conversations yet")));
});

/* ---------- AgentWorkspace：新对话 / 清空历史生命周期（§4.6） ---------- */

test("start new conversation resets the transcript without confirmation", async () => {
  installDom();
  const now = new Date();
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(1, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/1",
      response: {
        conversation: conversation(1, now.toISOString()),
        messages: [{ id: 1, conversation_id: 1, role: "user", content: "old question" }],
      },
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Conversation #1" })));
  fireEvent.click(view.getByRole("button", { name: "Conversation #1" }));
  await waitFor(() => assert.ok(view.getByText("old question")));

  fireEvent.click(view.getByRole("button", { name: "Start new conversation" }));
  await waitFor(() => assert.equal(view.queryByText("old question"), null));
  assert.equal(view.queryByRole("dialog"), null, "new conversation must not open a confirm dialog");
  assert.ok(view.getByText("Start researching from site content"));
});

test("clear history asks for confirmation; cancel sends no DELETE request", async () => {
  installDom();
  const now = new Date();
  const calls = installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(1, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/1",
      response: {
        conversation: conversation(1, now.toISOString()),
        messages: [{ id: 1, conversation_id: 1, role: "user", content: "to be cleared" }],
      },
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Conversation #1" })));
  fireEvent.click(view.getByRole("button", { name: "Conversation #1" }));
  await waitFor(() => assert.ok(view.getByText("to be cleared")));

  fireEvent.click(view.getByRole("button", { name: "Clear conversation" }));
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  assert.ok(view.getByText("Clear this conversation?"));

  fireEvent.click(view.getByRole("button", { name: "Cancel" }));
  await waitFor(() => assert.equal(view.queryByRole("dialog"), null));
  assert.equal(calls.filter((call) => call.path.includes("/conversations/1")).length, 1, "cancel must not send DELETE");
  assert.ok(view.getByText("to be cleared"), "messages survive cancel");
});

test("clear history success (204) empties the transcript and focuses the input", async () => {
  installDom();
  const now = new Date();
  const calls = installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(1, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/1",
      response: {
        conversation: conversation(1, now.toISOString()),
        messages: [{ id: 1, conversation_id: 1, role: "user", content: "to be cleared" }],
      },
    },
    { method: "DELETE", path: "/api/v1/agent/conversations/1", response: undefined },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Conversation #1" })));
  fireEvent.click(view.getByRole("button", { name: "Conversation #1" }));
  await waitFor(() => assert.ok(view.getByText("to be cleared")));

  fireEvent.click(view.getByRole("button", { name: "Clear conversation" }));
  const confirmButton = await waitFor(() => view.getByRole("button", { name: "Clear history" }));
  fireEvent.click(confirmButton);
  await flushAsyncUpdates();

  await waitFor(() => assert.equal(view.queryByText("to be cleared"), null), { timeout: 3000 });
  const deletes = calls.filter(
    (call) => call.path.includes("/api/v1/agent/conversations/1") && !call.path.includes("messages"),
  );
  assert.ok(deletes.length >= 1, "confirmed clear must call the owner-scoped DELETE");
  assert.ok(view.getByText("Start researching from site content"));
  await waitFor(
    () =>
      assert.equal(
        document.activeElement,
        view.getByRole("textbox", { name: "Ask the agent" }),
      ),
    { timeout: 3000 },
  );
});

test("clear history failure keeps messages and returns focus to the trigger", async () => {
  installDom();
  const now = new Date();
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(1, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/1",
      response: {
        conversation: conversation(1, now.toISOString()),
        messages: [{ id: 1, conversation_id: 1, role: "user", content: "keep me" }],
      },
    },
    {
      method: "DELETE", path: "/api/v1/agent/conversations/1",
      error: new ApiRequestError("AGENT_ERROR", "boom", 500),
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Conversation #1" })));
  fireEvent.click(view.getByRole("button", { name: "Conversation #1" }));
  await waitFor(() => assert.ok(view.getByText("keep me")));

  const trigger = view.getByRole("button", { name: "Clear conversation" });
  trigger.focus();
  fireEvent.click(trigger);
  const confirmButton = await waitFor(() => view.getByRole("button", { name: "Clear history" }));
  fireEvent.click(confirmButton);
  await flushAsyncUpdates();
  await waitFor(() => assert.ok(view.getByText("keep me"), "failed clear must keep messages"), { timeout: 3000 });
  await waitFor(() => assert.ok(view.getByText("Failed to clear conversation")), { timeout: 3000 });
  await waitFor(() => assert.equal(document.activeElement, trigger), { timeout: 3000 });
});

/* ---------- AgentWorkspace：流式回答与引用浮窗（复用 Ticket 02 底座） ---------- */

test("workspace streams an answer and renders citation cards", async () => {
  installDom();
  const now = new Date();
  const stub = installSSEFetch(streamEvents());
  const calls = installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(7, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/7",
      response: {
        conversation: conversation(7, now.toISOString()),
        messages: [
          { id: 1, conversation_id: 7, role: "user", content: "find me a guide" },
          { id: 2, conversation_id: 7, role: "assistant", content: "hello world" },
        ],
      },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const suggestion = await waitFor(() =>
      view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
    );
    fireEvent.click(suggestion);

    await waitFor(() => assert.ok(view.getByText("hello world")), { timeout: 3000 });
    await waitFor(() => assert.ok(view.getByRole("button", { name: /Cited content/ })));
    assert.ok(view.getByText("Search site content"));
    assert.match(stub.calls[0], /\/api\/v1\/agent\/chat\/stream$/);
    assert.ok(calls.some((call) => call.path.includes("/api/v1/agent/conversations")));
  } finally {
    stub.restore();
  }
});

test("clicking a citation opens the shared ContentDetailOverlay with agent source", async () => {
  installDom();
  const now = new Date();
  installSSEFetch(streamEvents());
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(7, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/7",
      response: {
        conversation: conversation(7, now.toISOString()),
        messages: [
          { id: 1, conversation_id: 7, role: "user", content: "find me a guide" },
          { id: 2, conversation_id: 7, role: "assistant", content: "hello world" },
        ],
      },
    },
    { method: "GET", path: "/api/v1/contents/3", response: CONTENT_DETAIL },
    { method: "GET", path: "/api/v1/contents/3/related-fanworks", response: { contents: [], total: 0 } },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  const suggestion = await waitFor(() =>
    view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
  );
  fireEvent.click(suggestion);
  await flushAsyncUpdates();

  const citation = await waitFor(() => view.getByRole("button", { name: /Cited content/ }));
  fireEvent.click(citation);

  const dialog = await waitFor(() => view.getByRole("dialog"));
  assert.ok(view.getByText("Back to conversation"), "overlay must use the agent-citation return label");
  await waitFor(
    () =>
      assert.ok(
        within(dialog).getAllByRole("heading", { name: "Cited content" }).length >= 1,
        "overlay shows the cited content title",
      ),
  );
});

/* ---------- AgentWorkspace：流式失败与重试 ---------- */

test("stream error shows a localized banner and retry resends", async () => {
  installDom();
  const now = new Date();
  let attempt = 0;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => {
    attempt += 1;
    if (attempt === 1) {
      return sseResponse([{ type: "error", error_code: "AGENT_PROVIDER_ERROR", error_message: "provider unavailable" }]);
    }
    return sseResponse([
      { type: "start", conversation_id: 9, answer_kind: "grounded_content" },
      { type: "delta", delta: "recovered answer" },
      {
        type: "done",
        conversation_id: 9,
        answer_kind: "grounded_content",
        answer: "recovered answer",
        citations: [],
        tools: [],
        degraded: false,
      },
    ]);
  }) as typeof fetch;
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(9, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/9",
      response: {
        conversation: conversation(9, now.toISOString()),
        messages: [
          { id: 1, conversation_id: 9, role: "user", content: "find me a guide" },
          { id: 2, conversation_id: 9, role: "assistant", content: "recovered answer" },
        ],
      },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const suggestion = await waitFor(() =>
      view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
    );
    fireEvent.click(suggestion);
    await flushAsyncUpdates();

    await waitFor(() => assert.ok(view.getByText("This request was not completed")));
    assert.equal(attempt, 1);

    fireEvent.click(view.getByRole("button", { name: "Resend" }));
    await waitFor(() => assert.ok(view.getByText("recovered answer")));
    assert.equal(attempt, 2, "retry must issue a second stream request");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

/* ---------- AgentWorkspace：侧栏折叠与移动抽屉 ---------- */

test("sidebar collapse persists to localStorage and expands back", async () => {
  installDom();
  installApiMock([{ method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } }]);
  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByText("No conversations yet")));

  fireEvent.click(view.getByRole("button", { name: "Collapse sidebar" }));
  assert.equal(window.localStorage.getItem("agentSidebarCollapsed"), "collapsed");
  assert.ok(view.getByRole("button", { name: "Expand sidebar" }));

  fireEvent.click(view.getByRole("button", { name: "Expand sidebar" }));
  assert.equal(window.localStorage.getItem("agentSidebarCollapsed"), "expanded");
  assert.ok(view.getByRole("button", { name: "Collapse sidebar" }));
});

test("mobile conversation drawer opens from the menu button and closes on Escape", async () => {
  installDom();
  installApiMock([{ method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } }]);
  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByText("No conversations yet")));

  fireEvent.click(view.getByRole("button", { name: "Open conversation list" }));
  await waitFor(() => assert.ok(view.getByRole("dialog")));

  fireEvent.keyDown(document, { key: "Escape" });
  await waitFor(() => assert.equal(view.queryByRole("dialog"), null));
});

/* ---------- AgentFeatureGate：feature 开关 ---------- */

test("workspace page gate shows fallback while web_agent_enabled=false", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = { id: 1, username: "tester", email: "t@example.com", email_verified_at: "2026-01-01T00:00:00Z" };
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => ({
    ok: true,
    status: 200,
    json: async () => ({
      features: {
        web_agent_enabled: false,
        payment_enabled: false,
        creator_support_enabled: false,
        desktop_deploy_enabled: false,
      },
      captcha: {},
      client: {},
      legal: {},
    }),
  })) as unknown as typeof fetch;
  try {
    const view = renderWithIntl(
      <AgentFeatureGate capability="webAgent" fallback={<p>FALLBACK-BLOCK</p>}>
        <p>WORKSPACE-BLOCK</p>
      </AgentFeatureGate>,
    );
    await waitFor(() => assert.ok(view.getByText("FALLBACK-BLOCK")));
    assert.equal(view.queryByText("WORKSPACE-BLOCK"), null);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("workspace page gate renders children while web_agent_enabled=true", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = { id: 1, username: "tester", email: "t@example.com", email_verified_at: "2026-01-01T00:00:00Z" };
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () => ({
    ok: true,
    status: 200,
    json: async () => ({
      features: {
        web_agent_enabled: true,
        payment_enabled: false,
        creator_support_enabled: false,
        desktop_deploy_enabled: false,
      },
      captcha: {},
      client: {},
      legal: {},
    }),
  })) as unknown as typeof fetch;
  try {
    const view = renderWithIntl(
      <AgentFeatureGate capability="webAgent" fallback={<p>FALLBACK-BLOCK</p>}>
        <p>WORKSPACE-BLOCK</p>
      </AgentFeatureGate>,
    );
    await waitFor(() => assert.ok(view.getByText("WORKSPACE-BLOCK")));
    assert.equal(view.queryByText("FALLBACK-BLOCK"), null);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

/* ---------- AgentWorkspace：空态建议问题与停止生成 ---------- */

test("empty state suggestions send the suggested question directly", async () => {
  installDom();
  const stub = installSSEFetch(streamEvents());
  installApiMock([{ method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } }]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const suggestion = await waitFor(() =>
      view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
    );
    fireEvent.click(suggestion);
    await waitFor(() => assert.ok(view.getByText("hello world")));
    assert.ok(view.getByText("Find beginner-friendly furniture mods"), "suggestion is sent as the user message");
    assert.match(stub.calls[0], /\/api\/v1\/agent\/chat\/stream$/);
  } finally {
    stub.restore();
  }
});

test("stop aborts the stream, keeps partial content and shows the stopped notice", async () => {
  installDom();
  installApiMock([{ method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } }]);
  const stub = installAbortableSSEFetch([{ type: "delta", delta: "partial answer" }]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const suggestion = await waitFor(() =>
      view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
    );
    fireEvent.click(suggestion);
    await waitFor(() => assert.ok(view.getByText("partial answer")));

    fireEvent.click(view.getByRole("button", { name: "Stop generating" }));
    await waitFor(() => assert.ok(view.getByText("Stopped generating")));
    assert.ok(view.getByText("partial answer"), "partial content is preserved after stop");
    const textbox = view.getByRole("textbox", { name: "Ask the agent" }) as HTMLTextAreaElement;
    await waitFor(() => assert.equal(textbox.disabled, false), { timeout: 3000 });
  } finally {
    stub.restore();
  }
});

/* ---------- 页面接线契约（源码） ---------- */

test("protected /agent page wires Header, feature gate and workspace", async () => {
  const page = await read("app/(protected)/agent/page.tsx");
  assert.match(page, /<Header \/>/);
  assert.match(page, /AgentFeatureGate/);
  assert.match(page, /capability="webAgent"/);
  assert.match(page, /<AgentWorkspace \/>/);
});

test("workspace wires citations to the shared overlay with agent source", async () => {
  const source = await read("components/agent/AgentWorkspace.tsx");
  assert.match(source, /ContentDetailOverlay/);
  assert.match(source, /source="agent-citation"/);
  assert.match(source, /surface:\s*"global"/);
  assert.match(source, /agentSidebarCollapsed/);
});

test("Header exposes the /agent entry in desktop nav and mobile menu", async () => {
  const header = await read("components/layout/Header.tsx");
  const agentLinks = header.match(/href="\/agent"/g) ?? [];
  assert.ok(agentLinks.length >= 2, "desktop nav and mobile menu both link to /agent");
  assert.match(header, /nav\.agent/);
  assert.match(header, /pathname\.startsWith\("\/agent"\)/);
});
