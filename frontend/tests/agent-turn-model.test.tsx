import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* #663 组件级过渡测试：TurnModel 接线后的两条终态路径——首轮 done→历史回载
 * 在途不闪空（显性体验改善）与续问轮 done 只 commit 一次。稳态等价由
 * agent-turn-samples 特征样本与 agent-workspace 组件测试锁定。 */

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

const workspaceMessages = enMessages;

function renderWithIntl(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={workspaceMessages}>
      <ToastProvider>{node}</ToastProvider>
    </IntlProvider>,
  );
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

type ApiStubEntry = { path: string; response?: unknown };

const originalGet = api.get;

function installApiMock(entries: ApiStubEntry[]) {
  api.get = (async (callPath: string) => {
    const match = entries
      .filter((entry) => callPath.includes(entry.path))
      .sort((a, b) => b.path.length - a.path.length)[0];
    if (!match) throw new Error(`unexpected GET ${callPath}`);
    return match.response;
  }) as typeof api.get;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

async function flushAsyncUpdates() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

type WorkspaceComponent = typeof import("@/components/agent/AgentWorkspace")["AgentWorkspace"];
let AgentWorkspace: WorkspaceComponent | null = null;

test.before(async () => {
  installOverlayTestStubs();
  const workspaceModule = await import("@/components/agent/AgentWorkspace");
  AgentWorkspace = workspaceModule.AgentWorkspace;
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  window.localStorage.clear();
  delete (globalThis as Record<string, unknown>).fetch;
});

test("首轮 done→历史回载在途：答案持续可见不闪空，回载落地后服务端历史接管且不双渲染", async () => {
  installDom();
  assert.ok(AgentWorkspace);
  const historyGate = deferred<{ messages?: unknown[] }>();
  installApiMock([
    { path: "/api/v1/agent/conversations", response: { conversations: [] } },
    { path: "/api/v1/agent/conversations/7", response: historyGate.promise },
  ]);
  const restore = installSSEFetch([
    { type: "start", trace_id: "t1", conversation_id: 7, answer_kind: "grounded_content" },
    { type: "think_delta", delta: "思考过程" },
    { type: "delta", delta: "hello world" },
    {
      type: "done",
      conversation_id: 7,
      message_id: 2,
      answer_kind: "grounded_content",
      answer: "hello world",
      citations: [],
      tools: [],
      degraded: false,
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace />);
    const composer = await waitFor(() => view.getByRole("textbox", { name: "Ask the agent" }));
    fireEvent.change(composer, { target: { value: "first question" } });
    fireEvent.keyDown(composer, { key: "Enter" });

    /* done 已到、历史请求挂起：答案与提问行必须持续可见（无骨架屏闪空）。 */
    await waitFor(() => assert.ok(view.getByText("hello world")), { timeout: 3000 });
    await flushAsyncUpdates();
    assert.ok(view.getByText("first question"), "in-flight: query stays visible");
    assert.ok(view.getByText("hello world"), "in-flight: answer stays visible");
    assert.equal(view.container.querySelector('[aria-busy="true"]'), null, "in-flight: no skeleton flash");

    /* 回载落地：服务端行接管树（think 行以折叠块回放），活动轮清空——
       同内容只渲染一份。换树交换窗口内点击可能落在游离节点：轮询内重查
       重点（既有测试同款模式）。 */
    historyGate.resolve({
      messages: [
        { id: 1, conversation_id: 7, role: "user", content: "first question" },
        { id: 2, conversation_id: 7, role: "assistant", content: "先把口语化需求扩展为检索词", phase: "think" },
        { id: 3, conversation_id: 7, role: "assistant", content: "hello world" },
      ],
    });
    await waitFor(
      () => {
        const toggle = view.getByRole("button", { name: /Thought process/ });
        assert.equal(toggle.getAttribute("aria-expanded"), "false", "server think row replays collapsed");
        fireEvent.click(toggle);
        assert.ok(view.getByText(/先把口语化需求扩展为检索词/), "server think content visible after expand");
      },
      { timeout: 3000 },
    );
    assert.equal(view.getAllByText("first question").length, 1, "server takeover: no duplicated query");
    assert.equal(view.getAllByText("hello world").length, 1, "server takeover: no duplicated answer");
  } finally {
    restore();
  }
});

test("续问轮 done 只 commit 一次：树尾追加单轮，无重复提问/正文", async () => {
  installDom();
  assert.ok(AgentWorkspace);
  installApiMock([
    { path: "/api/v1/agent/conversations", response: { conversations: [] } },
    {
      path: "/api/v1/agent/conversations/7",
      response: {
        messages: [
          { id: 1, conversation_id: 7, role: "user", content: "first question" },
          { id: 2, conversation_id: 7, role: "assistant", content: "first answer" },
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
      message_id: 4,
      answer_kind: "grounded_content",
      answer: "second answer",
      citations: [],
      tools: [],
      degraded: false,
    },
  ]);
  try {
    const view = renderWithIntl(<AgentWorkspace initialConversationId={7} />);
    await waitFor(() => assert.ok(view.getByText("first answer")));
    const composer = view.getByRole("textbox", { name: "Ask the agent" });
    fireEvent.change(composer, { target: { value: "second question" } });
    fireEvent.keyDown(composer, { key: "Enter" });

    await waitFor(() => assert.ok(view.getByText("second answer")), { timeout: 3000 });
    /* 多轮 flush 后仍只一份——commit effect 无重复触发。 */
    await flushAsyncUpdates();
    await flushAsyncUpdates();
    await flushAsyncUpdates();
    assert.equal(view.getAllByText("second question").length, 1, "continuation commit happens exactly once");
    assert.equal(view.getAllByText("second answer").length, 1, "answer committed exactly once");
    assert.ok(view.getByText("first question"), "earlier turns stay in the tree");
  } finally {
    restore();
  }
});
