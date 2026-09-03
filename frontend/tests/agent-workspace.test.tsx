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
import { act, cleanup, fireEvent, installDom, render, waitFor, within } from "./runtime-test-helpers";

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
const routerPushes: string[] = [];

Module._load = function loadWithNavigationStub(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({
        push: (path: string) => {
          routerPushes.push(path);
        },
      }),
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
      messageHiddenByModeration: "This reply was hidden after failing a safety check",
      turnDetails: "Turn details",
      turnUsage: "Token usage: {prompt} in / {completion} out",
      traceLabel: "Trace ID",
      rateLimitTitle: "Too many AI requests",
      rateLimitHint: "Rate limited: retry later.",
      groupPinned: "Pinned",
      menuLabel: "Conversation actions",
      menuRename: "Rename",
      menuPin: "Pin",
      menuUnpin: "Unpin",
      menuDelete: "Delete conversation",
      renameInputLabel: "Rename conversation {id}",
      renameFailed: "Rename failed, please try again later",
      pinFailed: "Pin action failed, please try again later",
      deleteConfirmTitle: "Delete this conversation?",
      deleteConfirmDescription: "All messages in this conversation will be deleted.",
      deleteConfirmAction: "Delete conversation",
      deleteSuccess: "Conversation deleted",
      deleteFailed: "Delete failed, please try again later",
      copyMessage: "Copy response",
      copySuccess: "Copied to clipboard",
      copyFailed: "Copy failed, please select the text manually",
      regenerate: "Regenerate",
    },
    thinking: {
      label: "Thought process",
      streamingLabel: "Thinking…",
    },
    markdown: {
      copyCode: "Copy code",
      citationJump: "Jump to citation {index}",
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
      searchContentRunning: "Searching site content",
      searchContentSuccess: "Searched site content",
      searchContentFailed: "Site content search failed",
      searchContentSkipped: "Skipped site content search",
      getContentDetailRunning: "Fetching content detail",
      getContentDetailSuccess: "Fetched content detail",
      getContentDetailFailed: "Content detail fetch failed",
      getContentDetailSkipped: "Skipped content detail fetch",
      getUsageGuideRunning: "Fetching usage guide",
      getUsageGuideSuccess: "Fetched usage guide",
      getUsageGuideFailed: "Usage guide fetch failed",
      getUsageGuideSkipped: "Skipped usage guide fetch",
      suggestPublishMetadataRunning: "Suggesting publish metadata",
      suggestPublishMetadataSuccess: "Suggested publish metadata",
      suggestPublishMetadataFailed: "Publish metadata suggestion failed",
      suggestPublishMetadataSkipped: "Skipped publish metadata suggestion",
      unknownRunning: "Tool running",
      unknownSuccess: "Tool finished",
      unknownFailed: "Tool failed",
      unknownSkipped: "Tool skipped",
      duration: "{seconds}s",
      stepsSummary: "{count} tool steps",
      hits: "{count} hits",
    },
    citations: {
      title: "Site references",
      count: "{count} verified references",
      invalid: "Reference unavailable",
      zoneOriginal: "Original",
      zoneFanwork: "Fanwork",
    },
    noEvidence: {
      title: "Not enough evidence",
      description: "No public content supports this answer.",
      searchCta: "Search site content",
    },
    degraded: {
      title: "Search fallback active",
      description: "The answer was not generated. Review the available site references.",
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
  method: "GET" | "DELETE" | "PATCH";
  path: string;
  response?: unknown;
  error?: ApiRequestError;
};

const originalGet = api.get;
const originalDelete = api.delete;
const originalPatch = api.patch;

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.delete = originalDelete;
  api.patch = originalPatch;
  authUser = null;
  routerPushes.length = 0;
  window.localStorage.clear();
  delete (globalThis as Record<string, unknown>).fetch;
});

function installApiMock(entries: ApiStubEntry[]) {
  const calls: ApiCall[] = [];
  function route(callPath: string, method: "GET" | "DELETE" | "PATCH", body?: unknown): unknown {
    calls.push({ path: callPath, method, body });
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
  api.patch = (async (callPath: string, body: unknown) =>
    route(callPath, "PATCH", body)) as typeof api.patch;
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
    const url = String(input);
    if (url.includes("/api/v1/auth/csrf")) {
      return {
        ok: true,
        status: 200,
        json: async () => ({ csrf_token: "test-csrf-token" }),
      } as unknown as Response;
    }
    calls.push(url);
    return sseResponse(events);
  }) as typeof fetch;
  return { calls, restore: () => (globalThis.fetch = originalFetch) };
}

/* SSE stub that keeps the stream open and errors it when the AbortSignal
   fires, so the workspace can be stopped mid-stream (stop button path). */
function installAbortableSSEFetch(events: Array<Record<string, unknown>>) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (_input: string | URL | Request, init?: RequestInit) => {
    if (String(_input).includes("/api/v1/auth/csrf")) {
      return {
        ok: true,
        status: 200,
        json: async () => ({ csrf_token: "test-csrf-token" }),
      } as unknown as Response;
    }
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
let Header: typeof import("@/components/layout/Header")["Header"];
import type { AgentStreamEvent } from "@/lib/agent-stream";
let parseAgentStreamLine: typeof import("@/lib/agent-stream")["parseAgentStreamLine"];
let startAgentStream: typeof import("@/lib/agent-stream")["startAgentStream"];
let normalizeAgentEvent: typeof import("@/lib/agent")["normalizeAgentEvent"];
let normalizeAgentCitation: typeof import("@/lib/agent")["normalizeAgentCitation"];
let normalizeAgentTool: typeof import("@/lib/agent")["normalizeAgentTool"];

test.before(async () => {
  installOverlayTestStubs();
  const workspaceModule = await import("@/components/agent/AgentWorkspace");
  const gateModule = await import("@/components/agent/AgentFeatureGate");
  const streamModule = await import("@/lib/agent-stream");
  const headerModule = await import("@/components/layout/Header");
  const normalizerModule = await import("@/lib/agent");
  AgentWorkspace = workspaceModule.AgentWorkspace;
  AgentFeatureGate = gateModule.AgentFeatureGate;
  Header = headerModule.Header;
  parseAgentStreamLine = streamModule.parseAgentStreamLine;
  startAgentStream = streamModule.startAgentStream;
  normalizeAgentEvent = normalizerModule.normalizeAgentEvent;
  normalizeAgentCitation = normalizerModule.normalizeAgentCitation;
  normalizeAgentTool = normalizerModule.normalizeAgentTool;
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
  assert.deepEqual(
    parseAgentStreamLine('data: {"type":"error","error_code":"AGENT_PROVIDER_ERROR","degraded":true,"degraded_reason":"provider_error"}'),
    {
      type: "error",
      error_code: "AGENT_PROVIDER_ERROR",
      degraded: true,
      degraded_reason: "provider_error",
    },
  );
  assert.equal(parseAgentStreamLine("data: [DONE]"), null);
  assert.equal(parseAgentStreamLine("event: ping"), null);
  assert.equal(parseAgentStreamLine("data: not-json"), null);
});

test("parseAgentStreamLine decodes SSE v2 events: think_delta, tool step details, done.message_id/usage", () => {
  assert.deepEqual(parseAgentStreamLine('data: {"type":"think_delta","delta":"先想一下"}'), {
    type: "think_delta",
    delta: "先想一下",
  });
  assert.equal(parseAgentStreamLine('data: {"type":"think_delta"}'), null);
  assert.deepEqual(
    parseAgentStreamLine(
      'data: {"type":"tool_status","tool":{"name":"search_content","args_summary":"像素风 游戏 +expanded: 治愈 素材","hits":3,"status":"success","duration_ms":42}}',
    ),
    {
      type: "tool_status",
      tool: {
        name: "search_content",
        args_summary: "像素风 游戏 +expanded: 治愈 素材",
        hits: 3,
        status: "success",
        duration_ms: 42,
      },
    },
  );
  assert.deepEqual(
    parseAgentStreamLine(
      'data: {"type":"done","trace_id":"t9","conversation_id":21,"message_id":66,"answer_kind":"grounded_content","usage":{"prompt_tokens":812,"completion_tokens":240},"degraded":false}',
    ),
    {
      type: "done",
      trace_id: "t9",
      conversation_id: 21,
      message_id: 66,
      answer_kind: "grounded_content",
      usage: { prompt_tokens: 812, completion_tokens: 240 },
      degraded: false,
    },
  );
  /* v1 形状的 done（无 message_id/usage）仍可解析——历史回放兼容。 */
  assert.deepEqual(parseAgentStreamLine('data: {"type":"done","conversation_id":7}'), {
    type: "done",
    conversation_id: 7,
  });
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

/* A-06：删除入口移至侧边栏 ⋯ 菜单（反冗余——头部不再重复）。 */
async function openConversationMenu(view: ReturnType<typeof renderWithIntl>) {
  const menuTrigger = await waitFor(() =>
    view.getAllByRole("button", { name: "Conversation actions" })[0],
  );
  fireEvent.click(menuTrigger);
  return waitFor(() => view.getByRole("menuitem", { name: "Delete conversation" }));
}

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

  const deleteItem = await openConversationMenu(view);
  fireEvent.click(deleteItem);
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  assert.ok(view.getByText("Delete this conversation?"));

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

  const deleteItem = await openConversationMenu(view);
  fireEvent.click(deleteItem);
  const confirmButton = await waitFor(() =>
    view.getByRole("button", { name: "Delete conversation" }),
  );
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

test("clear history failure keeps messages and shows the localized error", async () => {
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

  const deleteItem = await openConversationMenu(view);
  fireEvent.click(deleteItem);
  const confirmButton = await waitFor(() =>
    view.getByRole("button", { name: "Delete conversation" }),
  );
  fireEvent.click(confirmButton);
  await flushAsyncUpdates();
  await waitFor(() => assert.ok(view.getByText("keep me"), "failed clear must keep messages"), { timeout: 3000 });
  await waitFor(() => assert.ok(view.getByText("Delete failed, please try again later")), { timeout: 3000 });
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
    /* 工具步骤区完成后自动折叠（A-06）：先展开再断言步骤明细。 */
    fireEvent.click(view.getByRole("button", { name: "Tool activity" }));
    await waitFor(() => assert.ok(view.getByText("Searched site content")));
    assert.ok(view.getByText("Original"), "citation card exposes the zone label");
    assert.match(stub.calls[0], /\/api\/v1\/agent\/chat\/stream$/);
    assert.ok(calls.some((call) => call.path.includes("/api/v1/agent/conversations")));
  } finally {
    stub.restore();
  }
});

test("degraded stream hides the model summary and shows fallback references", async () => {
  installDom();
  const now = new Date();
  const stub = installSSEFetch([
    { type: "start", conversation_id: 7, answer_kind: "grounded_content" },
    { type: "delta", delta: "model summary that must stay hidden" },
    {
      type: "done",
      conversation_id: 7,
      answer_kind: "grounded_content",
      answer: "model summary that must stay hidden",
      citations: [{ content_id: 3, title: "Fallback result", zone: "original" }],
      degraded: true,
    },
  ]);
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(7, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/7",
      response: {
        conversation: conversation(7, now.toISOString()),
        messages: [{ id: 1, conversation_id: 7, role: "user", content: "Find beginner-friendly furniture mods" }],
      },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const suggestion = await waitFor(() =>
      view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
    );
    fireEvent.click(suggestion);
    await waitFor(() => assert.ok(view.getByText("Search fallback active")), { timeout: 3000 });
    assert.equal(view.queryByText("model summary that must stay hidden"), null);
    assert.ok(view.getByRole("button", { name: /Fallback result/ }));
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

    const composer = view.getByRole("textbox", { name: "Ask the agent" }) as HTMLTextAreaElement;
    fireEvent.change(composer, { target: { value: "extra input typed after the failure" } });
    fireEvent.click(view.getByRole("button", { name: "Resend" }));
    await waitFor(() => assert.ok(view.getByText("recovered answer")));
    assert.equal(attempt, 2, "retry must issue a second stream request");
    assert.equal(
      composer.value,
      "extra input typed after the failure",
      "retry must preserve the user's current form input",
    );
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("provider error falls back to ordinary keyword results without showing the generic error", async () => {
  installDom();
  const stub = installSSEFetch([
    {
      type: "error",
      error_code: "AGENT_PROVIDER_ERROR",
      degraded: true,
      degraded_reason: "provider_error",
    },
  ]);
  const calls = installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
    {
      method: "GET",
      path: "/api/v1/contents/search?q=Find+beginner-friendly+furniture+mods",
      response: {
        items: [{ id: 44, title: "Keyword fallback result", zone: "original", excerpt: "Matched by keyword search" }],
      },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const suggestion = await waitFor(() =>
      view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
    );
    fireEvent.click(suggestion);
    await waitFor(() => assert.ok(view.getByText("Search fallback active")));
    assert.ok(view.getByRole("button", { name: /Keyword fallback result/ }));
    assert.equal(view.queryByText("This request was not completed"), null);
    assert.ok(calls.some((call) => call.path.includes("/api/v1/contents/search?q=Find+beginner-friendly+furniture+mods")));
  } finally {
    stub.restore();
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
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
    /* done.conversation_id=7 触发历史回载（A-01）：服务端已持久化该轮。 */
    {
      method: "GET", path: "/api/v1/agent/conversations/7",
      response: {
        messages: [
          { id: 1, conversation_id: 7, role: "user", content: "Find beginner-friendly furniture mods" },
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
  assert.match(source, /source:\s*"agent-citation"/);
  assert.match(source, /useContentDetailOverlay/);
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

/* ---------- 全局产品面：Root Layout / Header 入口与关键词搜索（Task 4） ---------- */

test("Root Layout does not mount a global Agent trigger or panel", async () => {
  const layout = await read("app/layout.tsx");
  assert.ok(!layout.includes("AgentChatWidget"), "Root Layout must not mount the legacy Agent widget");
  assert.ok(!layout.includes("AgentFeatureGate"), "Root Layout must not gate any global Agent surface");
});

function installPublicConfigFetch(features: {
  web_agent_enabled: boolean;
  payment_enabled?: boolean;
  creator_support_enabled?: boolean;
  desktop_deploy_enabled?: boolean;
}) {
  const configFetches: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    configFetches.push(String(input));
    return {
      ok: true,
      status: 200,
      json: async () => ({
        features: {
          web_agent_enabled: features.web_agent_enabled,
          payment_enabled: features.payment_enabled ?? false,
          creator_support_enabled: features.creator_support_enabled ?? false,
          desktop_deploy_enabled: features.desktop_deploy_enabled ?? false,
        },
        captcha: {},
        client: {},
        legal: {},
      }),
    } as Response;
  }) as typeof fetch;
  return { configFetches, restore: () => (globalThis.fetch = originalFetch) };
}

test("Header hides the /agent entry when web_agent_enabled=false but keeps keyword search", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = { id: 1, username: "tester", email: "t@example.com", email_verified_at: "2026-01-01T00:00:00Z" };
  const stub = installPublicConfigFetch({ web_agent_enabled: false });
  try {
    const view = renderWithIntl(<Header />);
    await waitFor(() => assert.ok(stub.configFetches.length >= 1, "Header must consult the feature config"));
    await waitFor(() => assert.equal(view.queryByRole("link", { name: "AI Agent" }), null));
    assert.equal(view.queryByRole("button", { name: /agent/i }), null, "no Agent mode switch anywhere in Header");

    const searchbox = view.getByRole("combobox");
    fireEvent.change(searchbox, { target: { value: "mod guide" } });
    fireEvent.submit(searchbox.closest("form") as HTMLFormElement);
    await waitFor(() =>
      assert.ok(
        routerPushes.some((path) => path.includes("/search?q=mod%20guide")),
        "keyword search must still navigate to /search",
      ),
    );
  } finally {
    stub.restore();
  }
});

test("Header shows the /agent entry only for a verified user when the feature is enabled", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = { id: 1, username: "tester", email: "t@example.com", email_verified_at: "2026-01-01T00:00:00Z" };
  const stub = installPublicConfigFetch({ web_agent_enabled: true });
  try {
    const view = renderWithIntl(<Header />);
    await waitFor(() => assert.ok(view.getByRole("link", { name: "AI Agent" })));
  } finally {
    stub.restore();
  }
});

test("anonymous users get no /agent entry and the protected route redirects to login", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = null;
  const stub = installPublicConfigFetch({ web_agent_enabled: true });
  try {
    const view = renderWithIntl(<Header />);
    await waitFor(() => assert.ok(stub.configFetches.length >= 1));
    await waitFor(() => assert.equal(view.queryByRole("link", { name: "AI Agent" }), null));
  } finally {
    stub.restore();
  }
  const protectedLayout = await read("app/(protected)/layout.tsx");
  assert.match(protectedLayout, /login\?redirect/, "protected routes must follow the login redirect contract");
});

test("Header search is keyword-only via GlobalSearchInput with no Agent mode toggle", async () => {
  const header = await read("components/layout/Header.tsx");
  assert.match(header, /GlobalSearchInput/, "Header search must use the keyword-only GlobalSearchInput");
  assert.ok(!header.includes("SearchAgentInput"), "Header must not mount the agent search surface");
  assert.ok(!header.includes("agentMode") && !header.includes('mode="agent"'), "no Agent mode state in Header");

  installDom();
  clearPublicConfigCache();
  authUser = { id: 1, username: "tester", email: "t@example.com", email_verified_at: "2026-01-01T00:00:00Z" };
  const stub = installPublicConfigFetch({ web_agent_enabled: true });
  try {
    const view = renderWithIntl(<Header />);
    await waitFor(() => assert.ok(view.getByRole("link", { name: "AI Agent" })));
    assert.equal(view.getAllByRole("combobox").length, 1, "exactly one keyword search box");
    assert.equal(view.queryByRole("button", { name: /agent mode|AI 助手|keyword search/i }), null);

    const searchbox = view.getByRole("combobox");
    fireEvent.change(searchbox, { target: { value: "furniture mods" } });
    fireEvent.submit(searchbox.closest("form") as HTMLFormElement);
    await waitFor(() =>
      assert.ok(routerPushes.some((path) => path.includes("/search?q=furniture%20mods"))),
    );
  } finally {
    stub.restore();
  }
});

/* ---------- typed normalizer（lib/agent.ts，Task 4 Step 2） ---------- */

test("normalizer rejects malformed citations", () => {
  assert.equal(normalizeAgentCitation({ content_id: 0, title: "x", zone: "original" }), null);
  assert.equal(normalizeAgentCitation({ content_id: 3, title: "", zone: "original" }), null);
  assert.equal(normalizeAgentCitation({ content_id: 3, title: "x", zone: "spam" }), null);
  assert.equal(normalizeAgentCitation(null), null);
  assert.equal(normalizeAgentCitation("nope"), null);
  assert.deepEqual(
    normalizeAgentCitation({ content_id: 3, title: "T", zone: "original", excerpt: "e" }),
    { content_id: 3, title: "T", zone: "original", excerpt: "e" },
  );
});

test("normalizer preserves expanded citation fields and rejects a forged route", () => {
  const chunkKey = "a".repeat(64);
  assert.deepEqual(
    normalizeAgentCitation({
      content_id: 3,
      content_version: 7,
      chunk_key: chunkKey,
      chunk_index: 2,
      title: "T",
      zone: "original",
      route: "/original/3",
      excerpt: "e",
      source: "hybrid_rrf",
    }),
    {
      content_id: 3,
      content_version: 7,
      chunk_key: chunkKey,
      chunk_index: 2,
      title: "T",
      zone: "original",
      route: "/original/3",
      excerpt: "e",
      source: "hybrid_rrf",
    },
  );
  assert.equal(
    normalizeAgentCitation({
      content_id: 3,
      content_version: 7,
      chunk_key: chunkKey,
      chunk_index: 2,
      title: "T",
      zone: "original",
      route: "/original/999",
      source: "hybrid_rrf",
    }),
    null,
  );
  assert.equal(
    normalizeAgentCitation({
      content_id: 3,
      content_version: 7,
      title: "T",
      zone: "original",
    }),
    null,
    "expanded citation fields must be supplied as one complete contract",
  );
});

test("normalizer rejects malformed tool events", () => {
  assert.equal(normalizeAgentTool({ name: "", status: "success" }), null);
  assert.equal(normalizeAgentTool({ name: "search_content", status: "bogus" }), null);
  assert.equal(normalizeAgentTool(null), null);
  assert.deepEqual(normalizeAgentTool({ name: "search_content", status: "success", duration_ms: 42 }), {
    name: "search_content",
    status: "success",
    duration_ms: 42,
  });
  assert.deepEqual(normalizeAgentTool({ name: "search_content", status: "error" }), {
    name: "search_content",
    status: "error",
  });
  assert.deepEqual(normalizeAgentTool({ name: "search_content", status: "skipped" }), {
    name: "search_content",
    status: "skipped",
  });
});

test("normalizeAgentEvent drops malformed citation/tool events and filters done lists", () => {
  assert.equal(
    normalizeAgentEvent({ type: "citation", citation: { content_id: 0, title: "x", zone: "original" } }),
    null,
  );
  assert.equal(
    normalizeAgentEvent({ type: "tool_status", tool: { name: "search_content", status: "nope" } }),
    null,
  );
  assert.equal(normalizeAgentEvent({ type: "delta", delta: 42 }), null);
  assert.equal(normalizeAgentEvent({ type: "mystery" }), null);

  const done = normalizeAgentEvent({
    type: "done",
    answer_kind: "grounded_content",
    citations: [
      { content_id: 3, title: "Good", zone: "original" },
      { content_id: 0, title: "", zone: "bad" },
    ],
    tools: [
      { name: "search_content", status: "success" },
      { name: "", status: "success" },
    ],
  });
  assert.ok(done && done.type === "done");
  assert.equal((done as { citations?: unknown[] }).citations?.length, 1);
  assert.equal((done as { tools?: unknown[] }).tools?.length, 1);
});

test("parseAgentStreamLine never surfaces malformed citation or tool events", () => {
  assert.equal(
    parseAgentStreamLine('data: {"type":"citation","citation":{"content_id":-1,"title":"x","zone":"original"}}'),
    null,
  );
  assert.equal(
    parseAgentStreamLine('data: {"type":"tool_status","tool":{"name":"search_content","status":"weird"}}'),
    null,
  );
  const done = parseAgentStreamLine(
    'data: {"type":"done","citations":[{"content_id":1,"title":"ok","zone":"fanwork"},{"content_id":0,"title":"","zone":"nope"}]}',
  );
  assert.ok(done && done.type === "done");
  assert.equal((done as { citations?: unknown[] }).citations?.length, 1);
});

/* ---------- AgentWorkspace：Task 4 补充契约 ---------- */

test("malformed citation objects are never clickable", async () => {
  installDom();
  const now = new Date();
  const stub = installSSEFetch([
    { type: "start", conversation_id: 11, answer_kind: "grounded_content" },
    { type: "delta", delta: "answer text" },
    {
      type: "done",
      conversation_id: 11,
      answer_kind: "grounded_content",
      answer: "answer text",
      citations: [
        { content_id: 5, title: "Valid ref", zone: "original" },
        { content_id: 0, title: "", zone: "original" },
        { content_id: 6, title: "Bad zone", zone: "not-a-zone" },
      ],
      tools: [],
    },
  ]);
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(11, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/11",
      response: {
        conversation: conversation(11, now.toISOString()),
        messages: [
          { id: 1, conversation_id: 11, role: "user", content: "Find beginner-friendly furniture mods" },
          { id: 2, conversation_id: 11, role: "assistant", content: "answer text" },
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
    await waitFor(() => assert.ok(view.getByText("answer text")));

    await waitFor(() => assert.ok(view.getByRole("button", { name: /Valid ref/ })));
    assert.equal(view.queryByRole("button", { name: /Bad zone/ }), null, "invalid citation must never be interactive");
    assert.equal(view.queryByRole("button", { name: /Reference unavailable/ }), null);
  } finally {
    stub.restore();
  }
});

test("starting a new conversation preserves the old conversation's server history", async () => {
  installDom();
  const now = new Date();
  const calls = installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(1, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/1",
      response: {
        conversation: conversation(1, now.toISOString()),
        messages: [{ id: 1, conversation_id: 1, role: "user", content: "old server question" }],
      },
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Conversation #1" })));
  fireEvent.click(view.getByRole("button", { name: "Conversation #1" }));
  await waitFor(() => assert.ok(view.getByText("old server question")));

  fireEvent.click(view.getByRole("button", { name: "Start new conversation" }));
  await waitFor(() => assert.equal(view.queryByText("old server question"), null));
  assert.ok(
    view.getByRole("button", { name: "Conversation #1" }),
    "the old conversation must stay listed after starting a new one",
  );
  assert.equal(
    calls.filter((call) => call.method === "DELETE").length,
    0,
    "starting a new conversation must not delete server history",
  );
});

test("no-evidence turn shows the notice without fabricating an answer", async () => {
  installDom();
  const now = new Date();
  const stub = installSSEFetch([
    { type: "start", conversation_id: 13, answer_kind: "no_evidence" },
    { type: "delta", delta: "I did not find enough material." },
    {
      type: "done",
      conversation_id: 13,
      answer_kind: "no_evidence",
      answer: "",
      citations: [],
      tools: [],
    },
  ]);
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(13, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/13",
      response: {
        conversation: conversation(13, now.toISOString()),
        messages: [
          { id: 1, conversation_id: 13, role: "user", content: "Find beginner-friendly furniture mods" },
          { id: 2, conversation_id: 13, role: "assistant", content: "" },
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
    await waitFor(() => assert.ok(view.getByText("Not enough evidence")));
    assert.ok(view.getByText("No public content supports this answer."));
    assert.equal(view.queryByText("I did not find enough material."), null);
    const searchLink = view.getByRole("link", { name: "Search site content" });
    assert.equal(searchLink.getAttribute("href"), "/search?q=Find%20beginner-friendly%20furniture%20mods");
  } finally {
    stub.restore();
  }
});

test("closing the citation overlay restores citation focus and the transcript scroll anchor", async () => {
  installDom();
  const now = new Date();
  const stub = installSSEFetch(streamEvents());
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
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const suggestion = await waitFor(() =>
      view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
    );
    fireEvent.click(suggestion);
    await flushAsyncUpdates();

    const citation = await waitFor(() => view.getByRole("button", { name: /Cited content/ }));
    const transcript = view.getByRole("log");
    transcript.scrollTop = 240;
    const anchorBefore = transcript.scrollTop;

    fireEvent.click(citation);
    await waitFor(() => assert.ok(view.getByRole("dialog")));

    const backButton = await waitFor(() => view.getByRole("button", { name: "Back to conversation" }));
    fireEvent.click(backButton);
    const deadline = Date.now() + 5000;
    while (document.activeElement !== citation && Date.now() < deadline) {
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    assert.equal(document.activeElement, citation, "closing the overlay must return focus to the citation trigger");
    assert.equal(
      transcript.scrollTop,
      anchorBefore,
      "closing the overlay must not move the transcript scroll anchor",
    );
  } finally {
    stub.restore();
  }
});

/* ---------- A-06：续写契约 + 三层生成形态 + 侧边栏三交互 + 消息操作 ---------- */

function installSSEFetchWithBodies(events: Array<Record<string, unknown>>) {
  const bodies: Array<Record<string, unknown>> = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url.includes("/api/v1/agent/chat/stream")) {
      bodies.push(JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>);
      return sseResponse(events);
    }
    if (typeof originalFetch === "function") return originalFetch(input, init);
    throw new Error("unexpected fetch");
  }) as typeof fetch;
  return { bodies, restore: () => (globalThis.fetch = originalFetch) };
}

test("chat requests use the A-01 continuation body and carry conversation_id on follow-up turns", async () => {
  installDom();
  const now = new Date();
  const events = [
    { type: "start", trace_id: "t1", conversation_id: 9, answer_kind: "grounded_content" },
    { type: "delta", delta: "first answer" },
    { type: "done", conversation_id: 9, message_id: 91, answer_kind: "grounded_content", answer: "first answer", citations: [], tools: [] },
  ];
  const stub = installSSEFetchWithBodies(events);
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/9",
      response: {
        messages: [
          { id: 1, conversation_id: 9, role: "user", content: "first question" },
          { id: 2, conversation_id: 9, role: "assistant", content: "first answer" },
        ],
      },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const composer = await waitFor(() => view.getByRole("textbox", { name: "Ask the agent" }));
    fireEvent.change(composer, { target: { value: "first question" } });
    fireEvent.submit(composer.closest("form")!);
    await waitFor(() => assert.ok(view.getByText("first answer")), { timeout: 3000 });

    assert.equal(stub.bodies.length, 1);
    assert.equal(stub.bodies[0].message, "first question");
    assert.equal(stub.bodies[0].conversation_id, undefined, "first turn must omit conversation_id");
    assert.deepEqual(stub.bodies[0].context, { surface: "global" });
    assert.equal(stub.bodies[0].messages, undefined, "legacy full-history body must be gone");
  } finally {
    stub.restore();
  }
});

test("three-layer generation: thinking block streams open then auto-collapses, tool steps expose args and hits", async () => {
  installDom();
  const events = [
    { type: "start", trace_id: "t1", conversation_id: 11, answer_kind: "grounded_content" },
    { type: "think_delta", delta: "先把口语化需求" },
    { type: "think_delta", delta: "扩展为检索词" },
    {
      type: "tool_status",
      tool: { name: "search_content", args_summary: "治愈 素材 +expanded: 温柔 治愈系", hits: 3, status: "success", duration_ms: 1500 },
    },
    { type: "delta", delta: "这是带思考过程的回答。" },
    { type: "done", conversation_id: 11, message_id: 111, answer_kind: "grounded_content", answer: "这是带思考过程的回答。", citations: [], tools: [] },
  ];
  const stub = installSSEFetch(events);
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/11",
      response: {
        messages: [
          { id: 1, conversation_id: 11, role: "user", content: "最近有点 emo" },
          { id: 2, conversation_id: 11, role: "assistant", content: "先把口语化需求扩展为检索词", phase: "think" },
          { id: 3, conversation_id: 11, role: "assistant", content: "这是带思考过程的回答。" },
        ],
      },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const composer = await waitFor(() => view.getByRole("textbox", { name: "Ask the agent" }));
    fireEvent.change(composer, { target: { value: "最近有点 emo" } });
    fireEvent.submit(composer.closest("form")!);
    await waitFor(() => assert.ok(view.getByText("这是带思考过程的回答。")), { timeout: 3000 });

    /* 思考折叠区：完成后自动折叠，可重新展开。 */
    const thinkToggle = view.getByRole("button", { name: /Thought process/ });
    assert.equal(thinkToggle.getAttribute("aria-expanded"), "false", "thinking must auto-collapse on done");
    fireEvent.click(thinkToggle);
    await waitFor(() => assert.ok(view.getByText(/先把口语化需求扩展为检索词/)));

    /* 工具步骤区：折叠态展示计数，展开可见参数摘要与命中数。 */
    fireEvent.click(view.getByRole("button", { name: "Tool activity" }));
    await waitFor(() => assert.ok(view.getByText("Searched site content")));
    assert.ok(view.getByText(/治愈 素材/), "args summary incl. expansion terms is visible");
    assert.ok(view.getByText("3 hits"), "hit count is visible");
    assert.ok(view.getByText("2s"), "duration summary is visible");
  } finally {
    stub.restore();
  }
});

test("conversation history replays persisted think rows as collapsed thinking blocks", async () => {
  installDom();
  const now = new Date();
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [conversation(5, now.toISOString())] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/5",
      response: {
        messages: [
          { id: 1, conversation_id: 5, role: "user", content: "history question" },
          { id: 2, conversation_id: 5, role: "assistant", content: "历史思考内容", phase: "think" },
          { id: 3, conversation_id: 5, role: "assistant", content: "历史正式回答" },
        ],
      },
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByRole("button", { name: "Conversation #5" })));
  fireEvent.click(view.getByRole("button", { name: "Conversation #5" }));
  await waitFor(() => assert.ok(view.getByText("history question")));
  await waitFor(() => assert.ok(view.getByText("历史正式回答")));

  const thinkToggle = view.getByRole("button", { name: /Thought process/ });
  assert.equal(thinkToggle.getAttribute("aria-expanded"), "false", "replayed thinking starts collapsed");
  fireEvent.click(thinkToggle);
  await waitFor(() => assert.ok(view.getByText("历史思考内容")));
});

test("sidebar ⋯ menu renames via PATCH and shows the pinned group first", async () => {
  installDom();
  const now = new Date();
  const later = new Date(now.getTime() + 60000).toISOString();
  const calls = installApiMock([
    {
      method: "GET", path: "/api/v1/agent/conversations",
      response: {
        conversations: [
          { id: 21, context_type: "general", title: "旧标题", pinned_at: later, updated_at: later },
          { id: 22, context_type: "general", title: "普通会话", updated_at: now.toISOString() },
        ],
      },
    },
    {
      method: "PATCH", path: "/api/v1/agent/conversations/21",
      response: { conversation: { id: 21, context_type: "general", title: "新标题", pinned_at: later, updated_at: later } },
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByText("旧标题")));
  assert.ok(view.getByText("Pinned"), "pinned group label renders");
  const todayLabel = view.getByText("Today");
  const pinnedTitle = view.getByText("旧标题");
  assert.ok(pinnedTitle.compareDocumentPosition(todayLabel) & Node.DOCUMENT_POSITION_FOLLOWING, "pinned group renders before time groups");

  const menuTriggers = await waitFor(() => view.getAllByRole("button", { name: "Conversation actions" }));
  fireEvent.click(menuTriggers[0]);
  fireEvent.click(await waitFor(() => view.getByRole("menuitem", { name: "Rename" })));

  const renameInput = await waitFor(() => view.getByRole("textbox", { name: "Rename conversation 21" }));
  fireEvent.change(renameInput, { target: { value: "新标题" } });
  fireEvent.keyDown(renameInput, { key: "Enter" });
  await flushAsyncUpdates();

  await waitFor(() => assert.ok(view.getByText("新标题")));
  const patch = calls.find((call) => call.method === "PATCH");
  assert.ok(patch, "rename must call PATCH");
  assert.deepEqual(patch?.body, { title: "新标题" });
});

test("sidebar ⋯ menu pin toggle PATCHes pinned state and refreshes the list", async () => {
  installDom();
  const now = new Date();
  let pinState: string | null = null;
  const calls = installApiMock([
    {
      method: "GET", path: "/api/v1/agent/conversations",
      response: {
        conversations: [
          { id: 31, context_type: "general", title: "待置顶", pinned_at: pinState, updated_at: now.toISOString() },
        ],
      },
    },
    {
      method: "PATCH", path: "/api/v1/agent/conversations/31",
      response: { conversation: { id: 31, context_type: "general", title: "待置顶", pinned_at: now.toISOString(), updated_at: now.toISOString() } },
    },
  ]);

  const view = renderWithIntl(<AgentWorkspace />);
  await waitFor(() => assert.ok(view.getByText("待置顶")));
  const menuTriggers = await waitFor(() => view.getAllByRole("button", { name: "Conversation actions" }));
  fireEvent.click(menuTriggers[0]);
  fireEvent.click(await waitFor(() => view.getByRole("menuitem", { name: "Pin" })));
  await flushAsyncUpdates();

  const patch = calls.find((call) => call.method === "PATCH");
  assert.ok(patch, "pin must call PATCH");
  assert.deepEqual(patch?.body, { pinned: true });
});

test("assistant message actions: copy writes to the clipboard with a toast", async () => {
  installDom();
  const writes: string[] = [];
  /* jsdom 的 navigator.clipboard 存在与否随版本/平台变化：先摘除再以可配置
     属性注入 stub，避免 CI（Node 20）上对只读属性 defineProperty 抛错。 */
  const navigatorRecord = window.navigator as unknown as Record<string, unknown>;
  try {
    delete navigatorRecord.clipboard;
  } catch {
    /* 属性可能定义在原型链上（不可直接 delete），忽略后仍尝试 defineProperty。 */
  }
  Object.defineProperty(navigatorRecord, "clipboard", {
    configurable: true,
    value: { writeText: async (text: string) => { writes.push(text); } },
  });
  const events = [
    { type: "start", trace_id: "t1", conversation_id: 13, answer_kind: "grounded_content" },
    { type: "delta", delta: "可复制的回答。" },
    { type: "done", conversation_id: 13, message_id: 131, answer_kind: "grounded_content", answer: "可复制的回答。", citations: [], tools: [] },
  ];
  const stub = installSSEFetch(events);
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/13",
      response: { messages: [{ id: 1, conversation_id: 13, role: "user", content: "复制我" }, { id: 2, conversation_id: 13, role: "assistant", content: "可复制的回答。" }] },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const composer = await waitFor(() => view.getByRole("textbox", { name: "Ask the agent" }));
    fireEvent.change(composer, { target: { value: "复制我" } });
    fireEvent.submit(composer.closest("form")!);
    await waitFor(() => assert.ok(view.getByText("可复制的回答。")), { timeout: 3000 });

    /* 操作行只在 !streaming 且历史回载完成后出现（done→activeId→回载存在
       中间窗口）：waitFor 等按钮，避免流式/骨架窗口内的时序竞态。 */
    const copyButton = await waitFor(() => view.getByRole("button", { name: "Copy response" }), { timeout: 3000 });
    fireEvent.click(copyButton);
    await waitFor(() => assert.equal(writes[0], "可复制的回答。"));
    await waitFor(() => assert.ok(view.getByText("Copied to clipboard")));
  } finally {
    stub.restore();
  }
});

test("regenerate keeps the user message, drops the previous answer rows and re-sends the same query", async () => {
  installDom();
  let attempt = 0;
  const originalFetch = globalThis.fetch;
  const bodies: Array<Record<string, unknown>> = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url.includes("/api/v1/agent/chat/stream")) {
      attempt += 1;
      bodies.push(JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>);
      const answer = attempt === 1 ? "第一版回答" : "第二版回答";
      return sseResponse([
        { type: "start", trace_id: `t${attempt}`, conversation_id: 15, answer_kind: "grounded_content" },
        { type: "delta", delta: answer },
        { type: "done", conversation_id: 15, message_id: 150 + attempt, answer_kind: "grounded_content", answer, citations: [], tools: [] },
      ]);
    }
    throw new Error(`unexpected fetch: ${url}`);
  }) as typeof fetch;
  installApiMock([
    { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
    {
      method: "GET", path: "/api/v1/agent/conversations/15",
      response: { messages: [{ id: 1, conversation_id: 15, role: "user", content: "重新生成我" }, { id: 2, conversation_id: 15, role: "assistant", content: "第一版回答" }] },
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const composer = await waitFor(() => view.getByRole("textbox", { name: "Ask the agent" }));
    fireEvent.change(composer, { target: { value: "重新生成我" } });
    fireEvent.submit(composer.closest("form")!);
    await waitFor(() => assert.ok(view.getByText("第一版回答")), { timeout: 3000 });

    fireEvent.click(view.getByRole("button", { name: "Regenerate" }));
    await waitFor(() => assert.ok(view.getByText("第二版回答")), { timeout: 3000 });
    assert.equal(view.queryByText("第一版回答"), null, "old answer rows are dropped");
    assert.ok(view.getByText("重新生成我"), "the user message stays");
    assert.equal(bodies[1].message, "重新生成我");
    assert.equal(bodies[1].conversation_id, 15, "regenerate continues the same conversation");
  } finally {
    globalThis.fetch = originalFetch;
  }
});
