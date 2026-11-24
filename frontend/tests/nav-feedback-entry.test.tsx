/* #1022：/feedback/mine 入口补链——用户菜单（桌面下拉 + 移动抽屉）在
 * 「我的申诉」之后提供「我的反馈」，点击导航 /feedback/mine。 */
import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

const routerPushes: string[] = [];
const authStub: { user: unknown } = { user: null };

Module._load = function patchedLoad(request: string, parent: unknown, isMain: boolean) {
  if (request === "@/contexts/AuthContext") {
    return { useAuth: () => authStub };
  }
  if (request === "next/navigation") {
    return {
      useRouter: () => ({
        push: (url: string) => routerPushes.push(url),
        replace: (url: string) => routerPushes.push(url),
      }),
      usePathname: () => "/",
      useSearchParams: () => new URLSearchParams(),
    };
  }
  if (request === "next-themes") {
    return { useTheme: () => ({ theme: "light", resolvedTheme: "light", setTheme: () => {} }) };
  }
  if (request === "@/lib/locale") {
    return { setLocale: async () => undefined };
  }
  if (request === "@/components/agent/AgentFeatureGate") {
    return { AgentFeatureGate: ({ children }: { children?: React.ReactNode }) => children ?? null };
  }
  if (request === "@/components/social/NotificationDropdown") {
    return { NotificationDropdown: () => null };
  }
  if (request === "@/components/search/GlobalSearchInput") {
    return {
      GlobalSearchInput: () => React.createElement("div", { "data-testid": "global-search-stub" }),
    };
  }
  return originalModuleLoad.call(this, request, parent, isMain);
};

test.after(() => {
  Module._load = originalModuleLoad;
});

test.afterEach(() => {
  cleanup();
  routerPushes.length = 0;
});

function withIntl(node: React.ReactNode) {
  return (
    <IntlProvider locale="en" messages={enMessages}>
      {node}
    </IntlProvider>
  );
}

test("desktop user menu lists My Feedback after My Appeals and routes to /feedback/mine", async () => {
  installDom();
  authStub.user = { id: 7, username: "feedbackseed", email: "feedbackseed@example.test", role: "user" };
  const { Header } = await import("@/components/layout/Header");
  const view = render(withIntl(React.createElement(Header)));

  const trigger = await waitFor(() => {
    const button = Array.from(view.container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("feedbackseed"),
    );
    if (!button) throw new Error("desktop user menu trigger not rendered");
    return button as HTMLButtonElement;
  });

  // Radix DropdownMenu 键盘打开（series-nav 先例）→ portal 挂 document.body。
  trigger.focus();
  await act(async () => {
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    await new Promise((resolve) => setTimeout(resolve, 0));
  });

  const menuItems = await waitFor(() => {
    const items = Array.from(document.body.querySelectorAll('[role="menuitem"]'));
    if (items.length === 0) throw new Error("dropdown menu items not rendered");
    return items;
  });
  const labels = menuItems.map((item) => item.textContent ?? "");
  const appealsIndex = labels.findIndex((label) => label.includes("My Appeals"));
  const feedbackIndex = labels.findIndex((label) => label.includes("My Feedback"));
  assert.ok(appealsIndex >= 0, "My Appeals entry must exist");
  assert.ok(feedbackIndex === appealsIndex + 1, `My Feedback must directly follow My Appeals (labels=${labels.join("|")})`);

  routerPushes.length = 0;
  fireEvent.click(menuItems[feedbackIndex]);
  await waitFor(() => {
    if (!routerPushes.includes("/feedback/mine")) throw new Error(`expected push /feedback/mine, got ${routerPushes.join(",")}`);
  });
});

test("mobile drawer lists My Feedback after My Appeals and routes to /feedback/mine", async () => {
  installDom();
  authStub.user = { id: 7, username: "feedbackseed", email: "feedbackseed@example.test", role: "user" };
  const { Header } = await import("@/components/layout/Header");
  const view = render(withIntl(React.createElement(Header)));

  const openMenuButton = view.getByRole("button", { name: "Open menu" });
  fireEvent.click(openMenuButton);

  const drawerButtons = await waitFor(() => {
    const buttons = Array.from(view.container.querySelectorAll("button"));
    if (!buttons.some((b) => b.textContent?.includes("My Appeals"))) throw new Error("drawer entries not rendered");
    return buttons;
  });
  const labels = drawerButtons.map((b) => b.textContent ?? "");
  const appealsIndex = labels.findIndex((label) => label.includes("My Appeals"));
  const feedbackIndex = labels.findIndex((label) => label.includes("My Feedback"));
  assert.ok(appealsIndex >= 0, "drawer My Appeals entry must exist");
  assert.ok(feedbackIndex === appealsIndex + 1, `drawer My Feedback must directly follow My Appeals (labels=${labels.join("|")})`);

  fireEvent.click(drawerButtons[feedbackIndex]);
  await waitFor(() => {
    if (!routerPushes.includes("/feedback/mine")) throw new Error(`expected push /feedback/mine, got ${routerPushes.join(",")}`);
  });
});
