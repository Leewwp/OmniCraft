import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, render } from "./runtime-test-helpers";

/**
 * T29（FIX-15）：封禁屏与 /appeals 放行矩阵。
 * banned 用户访问 /appeals 前缀时 layout 必须放行 children（否则申诉链接
 * 自循环，F-098）；其余受保护路径渲染封禁屏；封禁屏链接预填
 * /appeals?target_type=account。
 */

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

const state = {
  pathname: "/settings",
  user: null as null | { is_banned: boolean; username: string },
  isLoading: false,
};

Module._load = function loadWithT29Stubs(request: string, parent: unknown, isMain: boolean) {
  if (request === "next/navigation") {
    return {
      useRouter: () => ({ push: () => undefined, replace: () => undefined }),
      usePathname: () => state.pathname,
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

function renderWithIntl(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      {node}
    </IntlProvider>,
  );
}

test("banned user on regular protected path sees the ban screen, not children", async () => {
  const ProtectedLayout = await loadLayout();
  state.user = { is_banned: true, username: "banned" };
  state.pathname = "/settings";
  const { container, queryByText } = renderWithIntl(
    <ProtectedLayout>
      <div data-testid="children">PAGE CONTENT</div>
    </ProtectedLayout>,
  );
  assert.ok(queryByText("Account suspended"), "ban screen title must render");
  assert.equal(queryByText("PAGE CONTENT"), null, "children must be replaced");
  cleanup();
});

test("banned user on /appeals gets children rendered (ban screen must not self-loop)", async () => {
  const ProtectedLayout = await loadLayout();
  state.user = { is_banned: true, username: "banned" };
  state.pathname = "/appeals";
  const { container, queryByText } = renderWithIntl(
    <ProtectedLayout>
      <div data-testid="children">APPEALS PAGE</div>
    </ProtectedLayout>,
  );
  assert.equal(queryByText("Account suspended"), null, "ban screen must not replace children on /appeals");
  assert.ok(container.textContent?.includes("APPEALS PAGE"), "children must render on /appeals");
  cleanup();
});

test("ban screen appeal link pre-fills target_type=account", async () => {
  const ProtectedLayout = await loadLayout();
  state.user = { is_banned: true, username: "banned" };
  state.pathname = "/studio";
  const { container } = renderWithIntl(
    <ProtectedLayout>
      <div>PAGE</div>
    </ProtectedLayout>,
  );
  const link = container.querySelector('a[href="/appeals?target_type=account"]');
  assert.ok(link, "ban screen must link to /appeals?target_type=account");
  assert.ok(link?.textContent?.includes("Submit an appeal"), "link label uses i18n copy");
  cleanup();
});

test("non-banned user keeps children on regular paths", async () => {
  const ProtectedLayout = await loadLayout();
  state.user = { is_banned: false, username: "normal" };
  state.pathname = "/settings";
  const { container, queryByText } = renderWithIntl(
    <ProtectedLayout>
      <div>SETTINGS PAGE</div>
    </ProtectedLayout>,
  );
  assert.equal(queryByText("Account suspended"), null);
  assert.ok(container.textContent?.includes("SETTINGS PAGE"));
  cleanup();
});
