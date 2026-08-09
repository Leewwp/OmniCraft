import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import { IntlProvider } from "use-intl";

import { api } from "@/lib/api";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const pushes: string[] = [];
const authStub = {
  user: null as null | { id: number },
  capabilities: { can_interact: false, interaction_denial_reason: "AUTH_STATUS_UNAVAILABLE" },
};

Module._load = function loadWithSocialStubs(request, parent, isMain) {
  if (request === "next/navigation") {
    return { useRouter: () => ({ push: (path: string) => pushes.push(path) }) };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
      interactionDenialKey: (reason?: string) =>
        reason === "INSUFFICIENT_REPUTATION"
          ? "capabilities.deniedInsufficientReputation"
          : "capabilities.deniedUnavailable",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type DiscussionBoardComponent = typeof import("@/components/social/DiscussionBoard")["DiscussionBoard"];
let DiscussionBoard: DiscussionBoardComponent;

test.before(async () => {
  ({ DiscussionBoard } = await import("@/components/social/DiscussionBoard"));
});

const originalGet = api.get;

test.beforeEach(() => {
  installDom();
  pushes.length = 0;
  authStub.user = null;
  authStub.capabilities = { can_interact: false, interaction_denial_reason: "AUTH_STATUS_UNAVAILABLE" };
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
});

test.after(() => {
  Module._load = originalModuleLoad;
});

test("compact discussion empty state sends anonymous visitors to login", async () => {
  mockDiscussions([]);
  const view = renderBoard();

  fireEvent.click(await view.findByRole("button", { name: "New post" }));
  assert.deepEqual(pushes, ["/login"]);
});

test("compact discussion entry links eligible users to the IP-scoped composer", async () => {
  authStub.user = { id: 7 };
  authStub.capabilities = { can_interact: true, interaction_denial_reason: "AUTH_STATUS_UNAVAILABLE" };
  mockDiscussions([{ id: 1, title: "Existing discussion" }]);
  const view = renderBoard();

  const entry = await view.findByRole("link", { name: "New post" });
  assert.equal(entry.getAttribute("href"), "/ip/42/discussions/new");
});

test("compact discussion entry fails closed with the server denial reason", async () => {
  authStub.user = { id: 7 };
  authStub.capabilities = {
    can_interact: false,
    interaction_denial_reason: "INSUFFICIENT_REPUTATION",
  };
  mockDiscussions([]);
  const view = renderBoard();

  const entry = await view.findByRole("button", { name: "New post" });
  assert.equal(entry.getAttribute("disabled"), "");
  assert.equal(entry.getAttribute("title"), "Insufficient reputation");
  assert.ok(view.getByText("Insufficient reputation"));
});

test("FollowButton preserves primary idle and restrained reversible following states", async () => {
  const source = await readFile(new URL("../components/social/FollowButton.tsx", import.meta.url), "utf8");

  assert.match(source, /variant=\{isFollowing \? "outline" : "default"\}/);
  assert.match(source, /group-hover:hidden group-focus-visible:hidden/);
  assert.match(source, /group-hover:inline group-focus-visible:inline/);
  assert.match(source, /social\.unfollow/);
  assert.match(source, /disabled=\{interactionBlocked \|\| busy\}/);
});

function mockDiscussions(discussions: Array<{ id: number; title: string }>) {
  api.get = (async <T,>(path: string): Promise<T> => {
    assert.equal(path, "/api/v1/ips/42/discussions");
    return { discussions } as T;
  }) as typeof api.get;
}

function renderBoard() {
  const view = render(
    <IntlProvider locale="en" messages={messages}>
      <DiscussionBoard ipId={42} compact />
    </IntlProvider>,
  );
  void waitFor(() => assert.equal(view.queryByText("Loading"), null));
  return view;
}

const messages = {
  common: { viewAll: "View all", retry: "Retry", loadFailed: "Load failed" },
  discussion: {
    title: "Discussions",
    subtitle: "Discuss this IP",
    newPost: "New post",
    loginToStart: "Log in to start",
    searchPlaceholder: "Search discussions",
    search: "Search",
    empty: "No discussions",
    emptyHint: "Start the conversation",
    replyCount: "{count} replies",
  },
  capabilities: {
    deniedInsufficientReputation: "Insufficient reputation",
    deniedUnavailable: "Interaction unavailable",
  },
};
