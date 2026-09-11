import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, render, waitFor } from "./runtime-test-helpers";

/**
 * T32（FIX-32）：未登录访问带 query 的受保护页时，redirect 必须保留
 * query 参数（现状只带 pathname，登录后丢筛选/预填状态）。
 */

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

const state = {
  pathname: "/settings",
  search: "tab=notifications&sort=asc",
  user: null as null | { is_banned: boolean },
  isLoading: false,
  replaces: [] as string[],
};

Module._load = function loadWithT32Stubs(request: string, parent: unknown, isMain: boolean) {
  if (request === "next/navigation") {
    return {
      useRouter: () => ({
        push: () => undefined,
        replace: (target: string) => {
          state.replaces.push(target);
        },
      }),
      usePathname: () => state.pathname,
      useSearchParams: () => new URLSearchParams(state.search),
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => ({
        user: state.user,
        isLoading: state.isLoading,
        unreadCounts: {},
        capabilities: { can_interact: false },
        login: async () => undefined,
        logout: async () => undefined,
        refresh: async () => false,
        refreshUser: async () => undefined,
      }),
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

let layoutComponent: ((props: { children: React.ReactNode }) => React.ReactElement) | null = null;

async function loadLayout() {
  if (!layoutComponent) {
    const mod = (await import("@/app/(protected)/layout")) as {
      default: (props: { children: React.ReactNode }) => React.ReactElement;
    };
    layoutComponent = mod.default;
  }
  return layoutComponent;
}

test("unauthenticated redirect preserves the query string", async () => {
  const ProtectedLayout = await loadLayout();
  state.user = null;
  state.isLoading = false;
  state.pathname = "/settings";
  state.search = "tab=notifications&sort=asc";
  state.replaces = [];

  render(
    <IntlProvider locale="en" messages={enMessages}>
      <ProtectedLayout>
        <div>SECRET</div>
      </ProtectedLayout>
    </IntlProvider>,
  );

  await waitFor(() => assert.ok(state.replaces.length > 0, "replace must be called"));
  assert.equal(
    state.replaces[0],
    `/login?redirect=${encodeURIComponent("/settings?tab=notifications&sort=asc")}`,
    `redirect must carry the query, got ${state.replaces[0]}`,
  );
  cleanup();
});

test("unauthenticated redirect without query keeps the plain path", async () => {
  const ProtectedLayout = await loadLayout();
  state.user = null;
  state.isLoading = false;
  state.pathname = "/appeals";
  state.search = "";
  state.replaces = [];

  render(
    <IntlProvider locale="en" messages={enMessages}>
      <ProtectedLayout>
        <div>SECRET</div>
      </ProtectedLayout>
    </IntlProvider>,
  );

  await waitFor(() => assert.ok(state.replaces.length > 0));
  assert.equal(state.replaces[0], `/login?redirect=${encodeURIComponent("/appeals")}`);
  cleanup();
});
