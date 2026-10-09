import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import {
  installDom,
  render,
  fireEvent,
  waitFor,
  cleanup,
} from "./runtime-test-helpers";
import { ToastProvider } from "@/components/ui/Toast";

/* #854 游客 Agent 面（T2）前端专项：双态判定纯逻辑、余量终局判定、
   登录续做交接（consume-once）、用尽转化卡与工作台游客形态（caption、
   输入锁、登录续做事件）。 */

installDom();

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

Module._load = function loadWithNavigationStub(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({
        push: () => undefined,
        replace: () => undefined,
      }),
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
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

const guestMessages = {
  ...enMessages,
  agent: {
    ...(enMessages.agent ?? {}),
    guest: (enMessages.agent as Record<string, unknown>)?.guest ?? {},
    workspace: {
      ...((enMessages.agent as unknown as Record<string, Record<string, unknown>>)?.workspace ?? {}),
      composerHint: "Enter to send",
      untitled: "Conversation",
      emptyTitle: "Start researching",
      emptyDescription: "Ask about works.",
      composerLabel: "Ask the agent",
      inputPlaceholder: "Describe what you want to find",
      sendMessage: "Send message",
      stopGenerating: "Stop generating",
      deepThink: "Deep think",
      deepThinkHint: "Reason before answering",
      modelLabel: "Model",
      sidebarLabel: "Conversation history",
      transcriptLabel: "Chat transcript",
      openConversations: "Open conversation list",
      closeConversations: "Close conversation list",
      collapseSidebar: "Collapse sidebar",
      expandSidebar: "Expand sidebar",
      newConversation: "Start new conversation",
      menuLabel: "Conversation actions",
      jumpToLatest: "Jump to latest",
      errorTitle: "This request was not completed",
      errorRetry: "Resend",
      stoppedNotice: "Stopped generating",
      suggestionLayout: "Suggest one",
      suggestionMusic: "Suggest two",
      suggestionMod: "Suggest three",
      conversationLoadFailed: "Failed to load conversation",
      emptyConversations: "No conversations yet",
      groupToday: "Today",
      groupYesterday: "Yesterday",
      groupEarlier: "Earlier",
      groupPinned: "Pinned",
      editTitleLabel: "Edit conversation title",
      turnDetails: "Turn details",
    },
  },
  common: {
    ...((enMessages as Record<string, Record<string, unknown>>).common ?? {}),
    loading: "Loading…",
  },
};

function renderWithIntl(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={guestMessages}>
      <ToastProvider>{node}</ToastProvider>
    </IntlProvider>,
  );
}

/* ---------- 纯逻辑 ---------- */

test("resolveAgentSurface picks the surface from auth recovery and feature flags", async () => {
  const { resolveAgentSurface } = await import("@/lib/agent-guest");
  const flagsOn = { web_agent_enabled: true, guest_agent_enabled: true };
  const guestOff = { web_agent_enabled: true, guest_agent_enabled: false };
  const webAgentOff = { web_agent_enabled: false, guest_agent_enabled: true };

  assert.equal(resolveAgentSurface(true, false, flagsOn), "pending");
  assert.equal(resolveAgentSurface(false, true, flagsOn), "user");
  assert.equal(resolveAgentSurface(false, true, webAgentOff), "login-guide");
  assert.equal(resolveAgentSurface(false, false, flagsOn), "guest");
  assert.equal(resolveAgentSurface(false, false, guestOff), "login-guide");
  assert.equal(resolveAgentSurface(false, false, webAgentOff), "login-guide");
});

test("agentEntryVisible gates the header entry without leaking turn counts", async () => {
  const { agentEntryVisible } = await import("@/lib/agent-guest");
  const flagsOn = { web_agent_enabled: true, guest_agent_enabled: true };
  const guestOff = { web_agent_enabled: true, guest_agent_enabled: false };

  assert.equal(agentEntryVisible(true, true, flagsOn), true);
  assert.equal(agentEntryVisible(true, false, flagsOn), false, "unverified users keep the webAgent gate");
  assert.equal(agentEntryVisible(false, false, flagsOn), true);
  assert.equal(agentEntryVisible(false, false, guestOff), false);
});

test("isGuestQuotaTerminal recognises the server-authoritative budget refusals", async () => {
  const { isGuestQuotaTerminal } = await import("@/lib/agent-guest");
  assert.equal(isGuestQuotaTerminal("GUEST_QUOTA_EXHAUSTED"), true);
  assert.equal(isGuestQuotaTerminal("GUEST_QUOTA_STATE_LOST"), true);
  assert.equal(isGuestQuotaTerminal("GUEST_RATE_LIMIT_EXCEEDED"), false);
  assert.equal(isGuestQuotaTerminal("AGENT_PROVIDER_ERROR"), false);
  assert.equal(isGuestQuotaTerminal(undefined), false);
});

test("guest handover is consume-once and tolerates broken storage shapes", async () => {
  const { saveGuestHandover, takeGuestHandover } = await import("@/lib/agent-guest");
  window.sessionStorage.clear();

  assert.equal(takeGuestHandover(), null, "no handover saved yet");
  saveGuestHandover("draft question");
  assert.equal(takeGuestHandover(), "draft question");
  assert.equal(takeGuestHandover(), null, "consume-once: refresh cannot replay");

  window.sessionStorage.setItem("omnicraft.guestAgentHandover", "{broken");
  assert.equal(takeGuestHandover(), null, "malformed record reads as absent");
  window.sessionStorage.clear();
});

/* ---------- 组件 ---------- */

type FetchCall = { url: string; init?: RequestInit };

function installGuestFetch(respond: (url: string, init?: RequestInit) => unknown | Response) {
  const calls: FetchCall[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input instanceof Request ? input.url : input);
    if (url.includes("/api/v1/auth/csrf")) {
      return {
        ok: true,
        status: 200,
        json: async () => ({ csrf_token: "test-csrf-token" }),
      } as unknown as Response;
    }
    calls.push({ url, init });
    return respond(url, init) as Response;
  }) as typeof fetch;
  return { calls, restore: () => (globalThis.fetch = originalFetch) };
}

function jsonResponse(payload: unknown): Response {
  return {
    ok: true,
    status: 200,
    json: async () => payload,
  } as unknown as Response;
}

test("AgentGuestExhaustedCard renders bilingual copy and fires the login continuation", async () => {
  const { AgentGuestExhaustedCard } = await import("@/components/agent/AgentGuestExhaustedCard");
  let clicked = 0;
  const view = renderWithIntl(
    <AgentGuestExhaustedCard onLogin={() => (clicked += 1)} />,
  );
  const card = view.getByTestId("guest-exhausted-card");
  assert.match(card.textContent ?? "", /Guest turns are used up/);
  fireEvent.click(view.getByRole("button", { name: /Sign in/ }));
  assert.equal(clicked, 1);
  cleanup();
});

test("guest workspace shows the server quota caption under the composer", async () => {
  const { AgentWorkspace } = await import("@/components/agent/AgentWorkspace");
  const fetchStub = installGuestFetch((url) => {
    if (url.includes("/agent/guest/quota")) {
      return jsonResponse({ remaining: 2, max_turns: 3, exhausted: false });
    }
    if (url.includes("/agent/guest/conversations")) return jsonResponse({ conversations: [] });
    if (url.includes("/agent/guest/models")) return jsonResponse({ models: [] });
    throw new Error(`unexpected fetch ${url}`);
  });
  const view = renderWithIntl(<AgentWorkspace variant="guest" />);
  const caption = await waitFor(() => view.getByTestId("guest-quota-caption"), { timeout: 4000 });
  assert.match(caption.textContent ?? "", /Guest mode · 2 turns left/);
  assert.ok(fetchStub.calls.some((call) => call.url.includes("/agent/guest/quota")));
  fetchStub.restore();
  cleanup();
});

test("guest workspace swaps to the conversion card and locks the composer when exhausted", async () => {
  const { AgentWorkspace } = await import("@/components/agent/AgentWorkspace");
  const fetchStub = installGuestFetch((url) => {
    if (url.includes("/agent/guest/quota")) {
      return jsonResponse({ remaining: 0, max_turns: 3, exhausted: true });
    }
    if (url.includes("/agent/guest/conversations")) return jsonResponse({ conversations: [] });
    if (url.includes("/agent/guest/models")) return jsonResponse({ models: [] });
    throw new Error(`unexpected fetch ${url}`);
  });
  const view = renderWithIntl(<AgentWorkspace variant="guest" />);
  await waitFor(() => view.getByTestId("guest-exhausted-card"), { timeout: 4000 });
  assert.equal(view.queryByTestId("guest-quota-caption"), null, "no caption when the budget is gone");
  const composer = view.getByRole("textbox", { name: /Ask the agent/ }) as HTMLTextAreaElement;
  assert.equal(composer.disabled, true, "composer is locked once the budget is gone");
  fetchStub.restore();
  cleanup();
});

test("sending on an exhausted guest budget opens the SP-17 login wall, never a request", async () => {
  const { AgentWorkspace } = await import("@/components/agent/AgentWorkspace");
  const { AuthGateProvider } = await import("@/components/auth/AuthGateProvider");
  window.sessionStorage.clear();
  const fetchStub = installGuestFetch((url) => {
    if (url.includes("/agent/guest/quota")) {
      return jsonResponse({ remaining: 0, max_turns: 3, exhausted: true });
    }
    if (url.includes("/agent/guest/conversations")) return jsonResponse({ conversations: [] });
    if (url.includes("/agent/guest/models")) return jsonResponse({ models: [] });
    if (url.includes("/agent/guest/chat/stream")) {
      throw new Error("exhausted guest must not open a stream");
    }
    throw new Error(`unexpected fetch ${url}`);
  });

  const view = renderWithIntl(
    <AuthGateProvider>
      <AgentWorkspace variant="guest" />
    </AuthGateProvider>,
  );
  await waitFor(() => view.getByTestId("guest-exhausted-card"), { timeout: 4000 });

  // The conversion card's CTA opens the SP-17 login modal (the continuation
  // carrier): the not-yet-executed draft rides the pending action, and the
  // account session executes it after login (consume-once handover covered
  // by the lib tests above).
  const cardButton = view.getByTestId("guest-exhausted-card").querySelector("button");
  assert.ok(cardButton, "exhausted card renders a CTA button");
  cardButton!.click();
  await waitFor(() => view.getByTestId("login-modal"), { timeout: 4000 });

  assert.ok(!fetchStub.calls.some((call) => call.url.includes("/agent/guest/chat/stream")),
    "no generation request may leave the browser on an exhausted budget");
  window.sessionStorage.clear();
  fetchStub.restore();
  cleanup();
});
