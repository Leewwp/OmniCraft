import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api } from "@/lib/api";
import { clearPublicConfigCache } from "@/lib/public-config";
import { ToastProvider } from "@/components/ui/Toast";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/**
 * A-07（#289）：搜索页 Agent 入口与新工作台统一。
 * - 搜索页主体输入 keyword-only（受控 GlobalSearchInput），不再有 Agent 模式
 *   切换与旧 /api/v1/agent/search 调用；降级/no_evidence 语义统一由 /agent
 *   工作台（A-06）承载。
 * - Agent 入口经 AgentFeatureGate 门控（未登录/flag 关闭不渲染），带当前
 *   query 跳 /agent?q=；工作台 initialQuery 预填 composer。
 */

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return readFile(path.join(root, relativePath), "utf8");
}

/* next/navigation + AuthContext stubs：GlobalSearchInput 用 useRouter，搜索页与
 * AgentFeatureGate 用 useAuth。组件必须在 patch 之后动态 import。 */
const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const routerPushes: string[] = [];
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
      useSearchParams: () => new URLSearchParams(""),
      useRouter: () => ({
        push: (target: string) => {
          routerPushes.push(target);
        },
      }),
      usePathname: () => "/search",
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
      interactionDenialKey: () => "capabilities.deniedUnknown",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

function renderWithIntl(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <ToastProvider>{node}</ToastProvider>
    </IntlProvider>,
  );
}

const originalGet = api.get;

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  authUser = null;
  routerPushes.length = 0;
  window.localStorage.clear();
  delete (globalThis as Record<string, unknown>).fetch;
});

function installPublicConfigFetch(features: { web_agent_enabled: boolean }) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    if (String(input).includes("/config")) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          features: {
            web_agent_enabled: features.web_agent_enabled,
            payment_enabled: false,
            creator_support_enabled: false,
            desktop_deploy_enabled: false,
          },
          captcha: {},
          client: {},
          legal: {},
        }),
      } as Response;
    }
    return originalFetch(input as RequestInfo);
  }) as typeof fetch;
  return {
    restore() {
      globalThis.fetch = originalFetch;
    },
  };
}

type ApiGetStub = { path: string; response: unknown };

/* 搜索页会发若干 GET（建议/分面标签/保存搜索/内容搜索）。按前缀路由，未匹配
 * 返回空对象（各 effect 均 fail-safe），并记录调用供断言。 */
function installApiGetMock(stubs: ApiGetStub[]) {
  const calls: string[] = [];
  api.get = (async <T,>(callPath: string): Promise<T> => {
    calls.push(callPath);
    const match = [...stubs]
      .sort((a, b) => b.path.length - a.path.length)
      .find((entry) => callPath.includes(entry.path));
    return (match ? match.response : {}) as T;
  }) as typeof api.get;
  return { calls };
}

const verifiedUser = {
  id: 1,
  username: "tester",
  email: "t@example.com",
  email_verified_at: "2026-01-01T00:00:00Z",
};

/* ---------- 源码契约 ---------- */

test("search page source: keyword-only GlobalSearchInput, no legacy agent search surface", async () => {
  const page = await read("app/(public)/search/page.tsx");
  assert.match(page, /GlobalSearchInput/, "search page must use the keyword-only GlobalSearchInput");
  assert.ok(!page.includes("SearchAgentInput"), "search page must not mount the legacy SearchAgentInput");
  assert.ok(!page.includes("/api/v1/agent/search"), "search page must not call the legacy agent NL search");
  assert.ok(!page.includes('mode === "agent"') && !page.includes('"agent" | "keyword"'), "no agent/keyword mode toggle");
  assert.match(page, /AgentFeatureGate/, "agent entry stays behind the feature gate");
  assert.match(page, /fallback=\{null\}/, "gate fallback hides the entry without touching keyword search");
  assert.ok(await read("components/agent/SearchAgentInput.tsx").then(
    () => false,
    () => true,
  ), "legacy SearchAgentInput component must be removed");
});

test("/agent page source: reads ?q= for the first-turn prefill behind Suspense", async () => {
  const page = await read("app/(protected)/agent/page.tsx");
  assert.match(page, /useSearchParams/, "agent page must read the q param");
  assert.match(page, /searchParams\.get\("q"\)/, "q param feeds the workspace");
  assert.match(page, /Suspense/, "useSearchParams is wrapped in a Suspense boundary");
  assert.match(page, /initialQuery/, "q is passed down as initialQuery");
});

test("search page keeps the top keyword search bar responsibility (ui-spec /search)", async () => {
  const workspace = await read("components/agent/AgentWorkspace.tsx");
  assert.match(workspace, /initialQuery\?\.trim\(\)|\(initialQuery \?\? ""\)\.trim\(\)/, "workspace trims initialQuery into the composer state");
});

/* ---------- GlobalSearchInput 受控模式 ---------- */

test("GlobalSearchInput controlled mode submits via callback without routing", async () => {
  installDom();
  const { GlobalSearchInput } = await import("../components/search/GlobalSearchInput");

  const values: string[] = [];
  const submitted: string[] = [];
  let query = "furniture mods";
  const view = renderWithIntl(
    <GlobalSearchInput
      size="lg"
      value={query}
      onValueChange={(next) => {
        query = next;
        values.push(next);
      }}
      onSubmit={(q) => submitted.push(q)}
      placeholder="Search by keyword..."
      submitLabel="Search"
    />,
  );

  const searchbox = view.getByRole("combobox");
  assert.equal((searchbox as HTMLInputElement).value, "furniture mods", "value is controlled");
  assert.equal(searchbox.getAttribute("placeholder"), "Search by keyword...");

  fireEvent.change(searchbox, { target: { value: "lamp mods" } });
  assert.deepEqual(values, ["lamp mods"], "onValueChange reports edits");
  assert.equal((searchbox as HTMLInputElement).value, "furniture mods", "displayed value follows the controlled prop");

  fireEvent.submit(searchbox.closest("form") as HTMLFormElement);
  assert.deepEqual(submitted, ["furniture mods"], "submit reports the controlled value, not the stale internal copy");
  assert.equal(routerPushes.length, 0, "controlled mode must not route by itself");
});

test("GlobalSearchInput listbox ids stay unique when two instances share the screen", async () => {
  installDom();
  const { GlobalSearchInput } = await import("../components/search/GlobalSearchInput");

  const view = renderWithIntl(
    <div>
      <GlobalSearchInput />
      <GlobalSearchInput size="lg" value="" onValueChange={() => undefined} onSubmit={() => undefined} />
    </div>,
  );

  const combos = view.getAllByRole("combobox");
  assert.equal(combos.length, 2);
  const ids = combos.map((node) => node.getAttribute("aria-controls"));
  assert.notEqual(ids[0], ids[1], "aria-controls must be instance-unique (Header + search page)");
  assert.ok(ids.every((id) => id && id.length > 0), "aria-controls always set");
});

/* ---------- 搜索页行为 ---------- */

test("anonymous users keep keyword search; no agent entry and no mode toggle", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = null;
  const stub = installPublicConfigFetch({ web_agent_enabled: false });
  const apiMock = installApiGetMock([
    { path: "/search/suggestions", response: { suggestions: [] } },
    { path: "/tags/faceted", response: { tags: [] } },
    { path: "/contents/search", response: { items: [], total: 0 } },
  ]);
  try {
    const pageModule = await import("../app/(public)/search/page");
    const view = renderWithIntl(<pageModule.default />);

    const searchbox = await view.findByRole("combobox");
    assert.ok(searchbox, "keyword search stays available regardless of feature state");
    assert.equal(view.queryByRole("link", { name: /Ask the AI agent/ }), null, "no agent entry while gated");

    fireEvent.change(searchbox, { target: { value: "desk lamp" } });
    fireEvent.submit(searchbox.closest("form") as HTMLFormElement);
    await waitFor(() => {
      /* URLSearchParams 将空格编码为「+」，不能断言 %20。 */
      const searchCall = apiMock.calls.find((callPath) => callPath.includes("/contents/search"));
      assert.ok(searchCall, "keyword submit runs the content search");
      assert.ok(
        searchCall?.includes("q=desk+lamp") || searchCall?.includes("q=desk%20lamp"),
        `query reaches the content search: ${searchCall}`,
      );
    });
    assert.ok(
      !apiMock.calls.some((callPath) => callPath.includes("/agent/search")),
      "the legacy agent NL search is never called",
    );
    assert.equal(routerPushes.length, 0, "search page does not navigate away on submit");
  } finally {
    stub.restore();
  }
});

test("gated agent entry links to /agent carrying the current query", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = verifiedUser;
  const stub = installPublicConfigFetch({ web_agent_enabled: true });
  installApiGetMock([
    { path: "/search/suggestions", response: { suggestions: [] } },
    { path: "/tags/faceted", response: { tags: [] } },
    { path: "/users/me/tag-groups", response: { tag_groups: [] } },
    { path: "/users/me/saved-searches", response: { saved_searches: [] } },
    { path: "/contents/search", response: { items: [], total: 0 } },
  ]);
  try {
    const pageModule = await import("../app/(public)/search/page");
    const view = renderWithIntl(<pageModule.default />);

    const entry = await view.findByRole("link", { name: /Ask the AI agent/ });
    assert.equal(entry.getAttribute("href"), "/agent", "empty query links to the bare workspace");

    const searchbox = view.getByRole("combobox");
    fireEvent.change(searchbox, { target: { value: "beginner furniture mods" } });
    await waitFor(() => {
      assert.equal(
        view.getByRole("link", { name: /Ask the AI agent/ }).getAttribute("href"),
        "/agent?q=beginner%20furniture%20mods",
        "entry carries the typed query for the workspace prefill",
      );
    });
    assert.equal(view.queryByRole("button", { name: /AI Search|Keyword Search/i }), null, "no legacy mode toggle buttons");
  } finally {
    stub.restore();
  }
});

test("unverified or logged-out users never see the search-page agent entry", async () => {
  installDom();
  clearPublicConfigCache();
  authUser = { ...verifiedUser, email_verified_at: null };
  const stub = installPublicConfigFetch({ web_agent_enabled: true });
  installApiGetMock([
    { path: "/search/suggestions", response: { suggestions: [] } },
    { path: "/tags/faceted", response: { tags: [] } },
  ]);
  try {
    const pageModule = await import("../app/(public)/search/page");
    const view = renderWithIntl(<pageModule.default />);
    await view.findByRole("combobox");
    assert.equal(view.queryByRole("link", { name: /Ask the AI agent/ }), null, "entry hidden for unverified users");
  } finally {
    stub.restore();
  }
});

/* ---------- 工作台预填 ---------- */

test("AgentWorkspace initialQuery prefills the composer for the first turn", async () => {
  installDom();
  installApiGetMock([
    { path: "/agent/conversations", response: { conversations: [] } },
  ]);
  const { AgentWorkspace } = await import("../components/agent/AgentWorkspace");

  const view = renderWithIntl(<AgentWorkspace initialQuery="beginner furniture mods" />);
  const composer = await view.findByRole("textbox");
  assert.equal(
    (composer as HTMLTextAreaElement).value,
    "beginner furniture mods",
    "search page entry query lands in the composer",
  );
});
