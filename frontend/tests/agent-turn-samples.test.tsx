import test from "node:test";
import assert from "node:assert/strict";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import path from "node:path";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* #663 迁移特征样本（characterization）：在改动 AgentWorkspace 之前，用旧实现
 * 对固定 mocked SSE / 历史 DTO 输入固化「稳定渲染结果」（transcript 直接子元素
 * 的结构化签名：tag/class/文本/折叠态/链接），迁移后同一设施逐场景比对——
 * 稳态内容等价的验收锚点。临时 ID / 时钟噪声不进样本（mock 全量控制输入）。
 *
 * 首轮 done→历史回载的「在途窗口」是本批显性设计改善（旧=骨架屏闪空、新=在途
 * 保持），不进特征样本；该过渡路径由组件级新测试单独锁定（见
 * agent-turn-model.test.tsx）。样本只锁稳态。
 *
 * 基线再生成：UPDATE_AGENT_TURN_SAMPLES=1 单跑本文件。 */

const FIXTURE_PATH = path.resolve(process.cwd(), "tests/fixtures/agent-turn-samples.json");

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

/* 与 agent-workspace.test.tsx 同源的导航/鉴权 stub（共享浮层依赖）。 */
const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
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
        user: null,
        isLoading: false,
        unreadCounts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 },
        capabilities: { can_interact: true, interaction_denial_reason: null },
        login: async () => undefined,
        logout: async () => undefined,
        refresh: async () => true,
        refreshUser: async () => undefined,
      }),
      interactionDenialKey: () => "capabilities.deniedUnknown",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

const workspaceMessages = {
  ...enMessages,
  agent: {
    ...(enMessages.agent ?? {}),
    workspace: {
      ...(enMessages.agent?.workspace ?? {}),
      emptyTitle: "Start researching from site content",
      emptyDescription: "Ask about works, sources, usage or directions.",
      suggestionLayout: "Looking for references",
      suggestionMusic: "Chords practice",
      suggestionMod: "Find beginner-friendly furniture mods",
      messageHiddenByModeration: "This reply was hidden after failing a safety check",
      turnDetails: "Turn details",
      turnUsage: "Token usage: {prompt} in / {completion} out",
      traceLabel: "Trace ID",
      stoppedNotice: "Stopped generating",
      errorTitle: "This request was not completed",
      errorRetry: "Resend",
      noEvidenceSearchCta: "Search site content",
      followUpsLabel: "Suggested follow-ups",
      jumpToLatest: "Jump to latest message",
    },
    noEvidence: {
      title: "Not enough evidence",
      description: "No public content supports this answer.",
      searchCta: "Search site content",
    },
    emptyTurn: {
      title: "No content this turn",
      description: "This turn completed no search or tool and produced no answer.",
    },
    degraded: {
      title: "Search fallback active",
      description: "The answer was not generated. Review the available site references.",
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

type ApiStubEntry = {
  method?: "GET" | "DELETE" | "PATCH";
  path: string;
  response?: unknown;
  error?: ApiRequestError;
};

const originalGet = api.get;
const originalDelete = api.delete;
const originalPatch = api.patch;

function installApiMock(entries: ApiStubEntry[]) {
  function route(callPath: string, method: "GET" | "DELETE" | "PATCH"): unknown {
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
  api.patch = (async (callPath: string, body: unknown) => route(callPath, "PATCH")) as typeof api.patch;
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
    return sseResponse(events);
  }) as typeof fetch;
  return () => (globalThis.fetch = originalFetch);
}

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
  return () => (globalThis.fetch = originalFetch);
}

function conversation(id: number, updatedAt: string) {
  return { id, context_type: "general", created_at: updatedAt, updated_at: updatedAt };
}

/* ---------- 稳定渲染签名提取 ---------- */

function normalizeWs(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

const SIGNATURE_ATTRS = new Set([
  "aria-expanded",
  "aria-label",
  "aria-live",
  "aria-busy",
  "aria-hidden",
  "role",
  "href",
  "rows",
  "data-slot",
]);

function elementSignature(el: Element): Record<string, unknown> {
  const attrs: Record<string, string> = {};
  for (const attr of Array.from(el.attributes)) {
    if (attr.name !== "class" && SIGNATURE_ATTRS.has(attr.name)) attrs[attr.name] = attr.value;
  }
  /* 折叠态（think/tools/引用折叠按钮）与链接目标是等价性判定的一部分。 */
  const toggles = Array.from(el.querySelectorAll("[aria-expanded]")).map((node) => ({
    name: normalizeWs(node.textContent ?? ""),
    expanded: node.getAttribute("aria-expanded"),
  }));
  const links = Array.from(el.querySelectorAll("a[href]")).map((node) => node.getAttribute("href"));
  return {
    tag: el.tagName.toLowerCase(),
    cls: el.getAttribute("class") ?? "",
    text: normalizeWs(el.textContent ?? ""),
    attrs,
    ...(toggles.length > 0 ? { toggles } : {}),
    ...(links.length > 0 ? { links } : {}),
  };
}

function transcriptOutline(): Record<string, unknown> {
  const transcript = document.querySelector('[data-slot="agent-transcript"]');
  if (!transcript) return { missing: "transcript" };
  const inner = transcript.querySelector(".max-w-3xl");
  if (!inner) {
    /* 骨架屏 / 加载失败分支（稳态场景不应命中；命中即结构回归）。 */
    return { fallback: normalizeWs(transcript.textContent ?? "") };
  }
  return { children: Array.from(inner.children).map(elementSignature) };
}

async function settle() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

async function sendFromComposer(view: ReturnType<typeof renderWithIntl>, text: string) {
  const composer = await waitFor(() => view.getByRole("textbox", { name: "Ask the agent" }));
  fireEvent.change(composer, { target: { value: text } });
  fireEvent.keyDown(composer, { key: "Enter" });
}

/* ---------- 场景定义（固定输入 → 稳定渲染） ---------- */

type Scenario = { id: string; run: () => Promise<Record<string, unknown>> };

const now = () => new Date().toISOString();
const CITED = { content_id: 3, title: "Cited content", zone: "original", excerpt: "excerpt line" };

const scenarios: Scenario[] = [
  {
    id: "first-round-grounded-full",
    async run() {
      installApiMock([
        { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
        {
          method: "GET",
          path: "/api/v1/agent/conversations/7",
          response: {
            conversation: conversation(7, now()),
            messages: [
              { id: 1, conversation_id: 7, role: "user", content: "first question" },
              { id: 2, conversation_id: 7, role: "assistant", content: "先思考 A再思考 B", phase: "think" },
              {
                id: 3,
                conversation_id: 7,
                role: "assistant",
                phase: "tools",
                tools: [{ name: "search_content", args_summary: "检索词", hits: 3, status: "success", duration_ms: 1500 }],
              },
              { id: 4, conversation_id: 7, role: "assistant", content: "hello world", citations: [CITED] },
            ],
          },
        },
      ]);
      const restore = installSSEFetch([
        { type: "start", trace_id: "t1", conversation_id: 7, answer_kind: "grounded_content" },
        { type: "think_delta", delta: "先思考 A" },
        { type: "think_delta", delta: "再思考 B" },
        { type: "tool_status", tool: { name: "search_content", args_summary: "检索词", hits: 3, status: "success", duration_ms: 1500 } },
        { type: "citation", citation: CITED },
        { type: "delta", delta: "hello " },
        { type: "delta", delta: "world" },
        {
          type: "done",
          conversation_id: 7,
          message_id: 4,
          trace_id: "t1",
          answer_kind: "grounded_content",
          answer: "hello world",
          citations: [CITED],
          tools: [{ name: "search_content", args_summary: "检索词", hits: 3, status: "success", duration_ms: 1500 }],
          usage: { prompt_tokens: 10, completion_tokens: 20 },
          degraded: false,
          follow_ups: ["追问一", "追问二"],
        },
      ]);
      try {
        const view = renderWithIntl(<Workspace />);
        await sendFromComposer(view, "first question");
        await waitFor(() => assert.ok(view.getByText("hello world")), { timeout: 3000 });
        await waitFor(() => assert.ok(view.getByRole("group", { name: "Suggested follow-ups" })), { timeout: 3000 });
        await settle();
        return transcriptOutline();
      } finally {
        restore();
      }
    },
  },
  {
    id: "continuation-commit",
    async run() {
      installApiMock([
        {
          method: "GET",
          path: "/api/v1/agent/conversations",
          response: { conversations: [{ ...conversation(7, now()), title: "旧会话" }] },
        },
        {
          method: "GET",
          path: "/api/v1/agent/conversations/7",
          response: {
            conversation: conversation(7, now()),
            messages: [
              { id: 11, conversation_id: 7, role: "user", content: "旧问题" },
              { id: 12, conversation_id: 7, role: "assistant", content: "旧回答" },
            ],
          },
        },
      ]);
      const restore = installSSEFetch([
        { type: "start", trace_id: "t2", conversation_id: 7, answer_kind: "grounded_content" },
        { type: "delta", delta: "second answer" },
        {
          type: "done",
          conversation_id: 7,
          message_id: 22,
          answer_kind: "grounded_content",
          answer: "second answer",
          citations: [{ content_id: 4, title: "Second reference", zone: "fanwork" }],
          tools: [],
          degraded: false,
        },
      ]);
      try {
        const view = renderWithIntl(<Workspace initialConversationId={7} />);
        await waitFor(() => assert.ok(view.getByText("旧回答")));
        await sendFromComposer(view, "second question");
        await waitFor(() => assert.ok(view.getByText("second answer")), { timeout: 3000 });
        await settle();
        return transcriptOutline();
      } finally {
        restore();
      }
    },
  },
  {
    id: "no-evidence-retrieval-miss",
    async run() {
      installApiMock([
        { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
        {
          method: "GET",
          path: "/api/v1/agent/conversations/13",
          response: {
            conversation: conversation(13, now()),
            messages: [
              { id: 1, conversation_id: 13, role: "user", content: "Find beginner-friendly furniture mods" },
              { id: 2, conversation_id: 13, role: "assistant", content: "" },
            ],
          },
        },
      ]);
      const restore = installSSEFetch([
        { type: "start", conversation_id: 13, answer_kind: "no_evidence" },
        { type: "delta", delta: "I did not find enough material." },
        {
          type: "done",
          conversation_id: 13,
          answer_kind: "no_evidence",
          answer: "",
          citations: [],
          tools: [{ name: "search_content", status: "success", duration_ms: 41, hits: 0 }],
        },
      ]);
      try {
        const view = renderWithIntl(<Workspace />);
        const suggestion = await waitFor(() =>
          view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
        );
        fireEvent.click(suggestion);
        await waitFor(() => assert.ok(view.getByText("Not enough evidence")), { timeout: 3000 });
        await settle();
        return transcriptOutline();
      } finally {
        restore();
      }
    },
  },
  {
    id: "no-evidence-empty-turn",
    async run() {
      installApiMock([
        { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
        {
          method: "GET",
          path: "/api/v1/agent/conversations/14",
          response: {
            conversation: conversation(14, now()),
            messages: [
              { id: 1, conversation_id: 14, role: "user", content: "Draw another one in portrait" },
              { id: 2, conversation_id: 14, role: "assistant", content: "" },
            ],
          },
        },
      ]);
      const restore = installSSEFetch([
        { type: "start", conversation_id: 14, answer_kind: "no_evidence" },
        { type: "delta", delta: "Sure, here is another one in portrait." },
        { type: "done", conversation_id: 14, answer_kind: "no_evidence", answer: "", citations: [], tools: [] },
      ]);
      try {
        const view = renderWithIntl(<Workspace />);
        const suggestion = await waitFor(() =>
          view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
        );
        fireEvent.click(suggestion);
        await waitFor(() => assert.ok(view.getByText("No content this turn")), { timeout: 3000 });
        await settle();
        return transcriptOutline();
      } finally {
        restore();
      }
    },
  },
  {
    id: "provider-error-keyword-fallback",
    async run() {
      installApiMock([
        { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
        {
          method: "GET",
          path: "/api/v1/contents/search",
          response: {
            items: [
              { id: 44, title: "Keyword fallback result", zone: "original", excerpt: "Matched by keyword search" },
              { id: 45, title: "Second keyword hit", zone: "fanwork", excerpt: "Another match" },
            ],
          },
        },
      ]);
      const restore = installSSEFetch([
        { type: "start", conversation_id: 0, answer_kind: "grounded_content" },
        { type: "delta", delta: "model summary that must stay hidden" },
        {
          type: "error",
          error_code: "AGENT_PROVIDER_ERROR",
          error_message: "provider unavailable",
          degraded: true,
          degraded_reason: "provider_error",
        },
      ]);
      try {
        const view = renderWithIntl(<Workspace />);
        const suggestion = await waitFor(() =>
          view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
        );
        fireEvent.click(suggestion);
        await waitFor(() => assert.ok(view.getByText("Search fallback active")), { timeout: 3000 });
        await waitFor(() => assert.ok(view.getByRole("button", { name: /Keyword fallback result/ })), { timeout: 3000 });
        await settle();
        return transcriptOutline();
      } finally {
        restore();
      }
    },
  },
  {
    id: "moderation-blocked-history",
    async run() {
      installApiMock([
        {
          method: "GET",
          path: "/api/v1/agent/conversations",
          response: { conversations: [{ ...conversation(6, now()), title: "被脱敏会话" }] },
        },
        {
          method: "GET",
          path: "/api/v1/agent/conversations/6",
          response: {
            conversation: conversation(6, now()),
            messages: [
              { id: 61, conversation_id: 6, role: "user", content: "历史提问" },
              { id: 62, conversation_id: 6, role: "assistant", content: "思考行", phase: "think" },
              { id: 63, conversation_id: 6, role: "assistant", moderation: "blocked" },
              { id: 64, conversation_id: 6, role: "assistant", content: "   " },
            ],
          },
        },
      ]);
      const view = renderWithIntl(<Workspace />);
      const item = await waitFor(() => view.getByRole("button", { name: /被脱敏会话/ }));
      fireEvent.click(item);
      await waitFor(() => assert.ok(view.getByText("This reply was hidden after failing a safety check")));
      await settle();
      return transcriptOutline();
    },
  },
  {
    id: "error-banner",
    async run() {
      installApiMock([{ method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } }]);
      const restore = installSSEFetch([
        { type: "start", trace_id: "t-err", conversation_id: 0, answer_kind: "grounded_content" },
        { type: "error", error_code: "AGENT_PROVIDER_ERROR", error_message: "provider unavailable" },
      ]);
      try {
        const view = renderWithIntl(<Workspace />);
        const suggestion = await waitFor(() =>
          view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
        );
        fireEvent.click(suggestion);
        await waitFor(() => assert.ok(view.getByText("This request was not completed")), { timeout: 3000 });
        await settle();
        return transcriptOutline();
      } finally {
        restore();
      }
    },
  },
  {
    id: "stop-partial-answer",
    async run() {
      installApiMock([{ method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } }]);
      const restore = installAbortableSSEFetch([
        { type: "start", conversation_id: 0, answer_kind: "grounded_content" },
        { type: "think_delta", delta: "开始思考" },
        { type: "delta", delta: "partial answer" },
      ]);
      try {
        const view = renderWithIntl(<Workspace />);
        const suggestion = await waitFor(() =>
          view.getByRole("button", { name: "Find beginner-friendly furniture mods" }),
        );
        fireEvent.click(suggestion);
        await waitFor(() => assert.ok(view.getByText("partial answer")));
        fireEvent.click(view.getByRole("button", { name: "Stop generating" }));
        await waitFor(() => assert.ok(view.getByText("Stopped generating")));
        await settle();
        return transcriptOutline();
      } finally {
        restore();
      }
    },
  },
  {
    id: "history-replay-only",
    async run() {
      installApiMock([
        {
          method: "GET",
          path: "/api/v1/agent/conversations",
          response: { conversations: [{ ...conversation(5, now()), title: "回放会话" }] },
        },
        {
          method: "GET",
          path: "/api/v1/agent/conversations/5",
          response: {
            conversation: conversation(5, now()),
            messages: [
              { id: 1, conversation_id: 5, role: "user", content: "history question" },
              { id: 2, conversation_id: 5, role: "assistant", content: "历史思考内容", phase: "think" },
              {
                id: 3,
                conversation_id: 5,
                role: "assistant",
                phase: "tools",
                tools: [{ name: "search_content", status: "success", duration_ms: 42 }],
              },
              { id: 4, conversation_id: 5, role: "assistant", content: "历史正式回答", citations: [CITED] },
            ],
          },
        },
      ]);
      const view = renderWithIntl(<Workspace />);
      const item = await waitFor(() => view.getByRole("button", { name: /回放会话/ }));
      fireEvent.click(item);
      await waitFor(() => assert.ok(view.getByText("历史正式回答")));
      await settle();
      return transcriptOutline();
    },
  },
  {
    id: "multi-turn-static-history",
    async run() {
      installApiMock([
        { method: "GET", path: "/api/v1/agent/conversations", response: { conversations: [] } },
        {
          method: "GET",
          path: "/api/v1/agent/conversations/7",
          response: {
            conversation: conversation(7, now()),
            messages: [
              { id: 101, conversation_id: 7, role: "user", content: "first question" },
              { id: 102, conversation_id: 7, role: "assistant", content: "first answer", citations: [{ content_id: 3, title: "Cited content", zone: "original" }] },
              { id: 103, conversation_id: 7, role: "user", content: "second question" },
              { id: 104, conversation_id: 7, role: "assistant", content: "second answer", citations: [{ content_id: 4, title: "Second reference", zone: "fanwork" }] },
            ],
          },
        },
      ]);
      let streamCalls = 0;
      const originalFetch = globalThis.fetch;
      globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        if (url.includes("/api/v1/auth/csrf")) {
          return { ok: true, status: 200, json: async () => ({ csrf_token: "tok" }) } as unknown as Response;
        }
        if (url.includes("/api/v1/agent/chat/stream")) {
          streamCalls += 1;
          return streamCalls === 1
            ? sseResponse([
                { type: "start", trace_id: "t1", conversation_id: 7, answer_kind: "grounded_content" },
                { type: "delta", delta: "first answer" },
                {
                  type: "done",
                  conversation_id: 7,
                  message_id: 102,
                  answer_kind: "grounded_content",
                  answer: "first answer",
                  citations: [{ content_id: 3, title: "Cited content", zone: "original" }],
                  tools: [],
                  degraded: false,
                },
              ])
            : sseResponse([
                { type: "start", trace_id: "t2", conversation_id: 7, answer_kind: "grounded_content" },
                { type: "delta", delta: "second answer" },
                {
                  type: "done",
                  conversation_id: 7,
                  message_id: 104,
                  answer_kind: "grounded_content",
                  answer: "second answer",
                  citations: [{ content_id: 4, title: "Second reference", zone: "fanwork" }],
                  tools: [],
                  degraded: false,
                },
              ]);
        }
        return originalFetch(input, init);
      }) as typeof fetch;
      try {
        const view = renderWithIntl(<Workspace />);
        await sendFromComposer(view, "first question");
        await waitFor(() => assert.ok(view.getByText("first answer")), { timeout: 3000 });
        await settle();
        await sendFromComposer(view, "second question");
        await waitFor(() => assert.ok(view.getAllByText("second answer").length >= 2), { timeout: 3000 });
        await settle();
        return transcriptOutline();
      } finally {
        globalThis.fetch = originalFetch;
      }
    },
  },
];

/* ---------- 驱动与断言 ---------- */

type WorkspaceComponent = typeof import("@/components/agent/AgentWorkspace")["AgentWorkspace"];
let workspaceComponent: WorkspaceComponent | null = null;

function Workspace(props: React.ComponentProps<WorkspaceComponent>) {
  assert.ok(workspaceComponent, "AgentWorkspace loaded in test.before");
  const W = workspaceComponent;
  return <W {...props} />;
}

test.before(async () => {
  installOverlayTestStubs();
  const workspaceModule = await import("@/components/agent/AgentWorkspace");
  workspaceComponent = workspaceModule.AgentWorkspace;
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.delete = originalDelete;
  api.patch = originalPatch;
  window.localStorage.clear();
  delete (globalThis as Record<string, unknown>).fetch;
});

test("#663 迁移特征样本：固定输入的稳态渲染与基线逐场景一致", async () => {
  installDom();
  assert.ok(workspaceComponent, "AgentWorkspace loaded in test.before");
  const updating = process.env.UPDATE_AGENT_TURN_SAMPLES === "1";
  let baseline: Record<string, unknown> = {};
  if (!updating) {
    baseline = JSON.parse(await readFile(FIXTURE_PATH, "utf8")) as Record<string, unknown>;
  }
  const collected: Record<string, unknown> = {};
  for (const scenario of scenarios) {
    collected[scenario.id] = await scenario.run();
    /* 单 test 驱动多场景：场景间必须清 DOM，否则跨场景元素累积。 */
    cleanup();
  }
  if (updating) {
    await mkdir(path.dirname(FIXTURE_PATH), { recursive: true });
    await writeFile(FIXTURE_PATH, `${JSON.stringify(collected, null, 2)}\n`, "utf8");
    console.log(`[agent-turn-samples] baseline written: ${FIXTURE_PATH}`);
    return;
  }
  const failures: string[] = [];
  for (const scenario of scenarios) {
    if (!(scenario.id in baseline)) {
      failures.push(`${scenario.id}: baseline 缺失（先 UPDATE_AGENT_TURN_SAMPLES=1 再生成）`);
      continue;
    }
    try {
      assert.deepEqual(collected[scenario.id], baseline[scenario.id]);
    } catch (error) {
      failures.push(`${scenario.id}: ${(error as Error).message.split("\n").slice(0, 40).join("\n")}`);
    }
  }
  assert.deepEqual(failures, []);
});
