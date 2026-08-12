import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError, setAccessToken } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { ReactionBar } from "@/components/social/ReactionBar";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const originalGet = api.get;
const originalPost = api.post;
const originalConsoleWarn = console.warn;

test.beforeEach(() => {
  installDom();
  setAccessToken(null);
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: window.localStorage,
  });
  console.warn = (...args: unknown[]) => {
    if (args.some((arg) => typeof arg === "string" && arg.includes("[silent-api-error] AuthContext:refresh"))) {
      return;
    }
    originalConsoleWarn(...args);
  };
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.post = originalPost;
  setAccessToken(null);
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: undefined,
  });
  console.warn = originalConsoleWarn;
});

const intlMessages = {
  common: {
    confirm: "Confirm",
    cancel: "Cancel",
    processing: "Processing",
    reason: "Reason",
  },
  capabilities: {
    deniedBanned: "Your account is banned and cannot perform interactions.",
    deniedEmailNotVerified: "Please verify your email before interacting.",
    deniedInsufficientReputation: "Insufficient reputation to perform this action.",
    deniedUnavailable: "Interaction is temporarily unavailable.",
    deniedUnknown: "Interaction is not available for this account.",
  },
  social: {
    like: "Like",
    dislike: "Dislike",
    reported: "Reported",
    report: "Report",
    reportDialogTitle: "Report this content",
    reportReason: "Please describe the reason for reporting:",
    reportFailed: "Failed to submit report. Please try again later.",
  },
};

function renderHarness(node: React.ReactNode) {
  return render(
    <IntlProvider locale="en" messages={intlMessages}>
      <ToastProvider>
        <AuthProvider>{node}</AuthProvider>
      </ToastProvider>
    </IntlProvider>,
  );
}

const testUser = {
  id: 7,
  email: "person@example.com",
  username: "person",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "en",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-08-03T00:00:00Z",
  created_at: "2026-08-03T00:00:00Z",
};

interface ReactionCall {
  get: Array<{ path: string }>;
  post: Array<{ path: string; body?: unknown }>;
}

/** Stands in for the backend reactions contract: public aggregates in
 *  `counts` and the viewer state in `viewer_reaction`. */
function installReactionApiMocks(reactionCalls: ReactionCall, viewer: { counts: { like: number; dislike: number }; viewer_reaction: "like" | "dislike" | null }) {
  api.get = (async <T,>(path: string): Promise<T> => {
    reactionCalls.get.push({ path });
    if (path === "/api/v1/auth/me") {
      return { user: testUser } as T;
    }
    if (path === "/api/v1/notifications/unread-count") {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path.startsWith("/api/v1/social/reactions")) {
      return { counts: viewer.counts, viewer_reaction: viewer.viewer_reaction } as T;
    }
    return {} as T;
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    reactionCalls.post.push({ path, body });
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: "test-access-token" },
        capabilities: { can_interact: true },
      } as T;
    }
    return {} as T;
  }) as typeof api.post;
}

function ReactionBarHarness({ contentId }: { contentId: number }) {
  const { login } = useAuth();
  return (
    <>
      <ReactionBar contentId={contentId} initialLikes={2} initialDislikes={1} />
      <button type="button" onClick={() => void login("person@example.com", "password")}>login</button>
    </>
  );
}

async function loginAs(view: ReturnType<typeof render>) {
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "login" }));
    await Promise.resolve();
  });
}

function reactionButton(view: ReturnType<typeof render>, title: string) {
  const button = view.container.querySelector<HTMLButtonElement>(`button[title="${title}"]`);
  assert.ok(button, `expected reaction button titled "${title}"`);
  return button;
}

function assertPressed(view: ReturnType<typeof render>, kind: "Like" | "Dislike", pressed: boolean, count: string) {
  const button = reactionButton(view, kind);
  assert.equal(button.getAttribute("aria-pressed"), String(pressed), `${kind} aria-pressed should be ${pressed}`);
  assert.ok(button.textContent?.includes(count), `${kind} should show count ${count}`);
}

test("ReactionBar renders public aggregates only for anonymous visitors and never fetches a viewer reaction", async () => {
  const calls: ReactionCall = { get: [], post: [] };
  installReactionApiMocks(calls, { counts: { like: 2, dislike: 1 }, viewer_reaction: "like" });

  const view = renderHarness(<ReactionBar contentId={47} initialLikes={2} initialDislikes={1} />);

  await waitFor(() => {
    const buttons = view.container.querySelectorAll<HTMLButtonElement>(`button[title="${intlMessages.capabilities.deniedUnavailable}"]`);
    assert.equal(buttons.length, 3, "like, dislike and report buttons are disabled for anonymous visitors");
    for (const button of buttons) {
      assert.ok(button.disabled, "anonymous buttons must be disabled");
    }
    assert.equal(buttons[0].getAttribute("aria-pressed"), "false", "anonymous must not expose a viewer pressed state");
    assert.equal(buttons[1].getAttribute("aria-pressed"), "false", "anonymous must not expose a viewer pressed state");
    assert.ok(buttons[0].textContent?.includes("2"), "like count should render from public aggregates");
    assert.ok(buttons[1].textContent?.includes("1"), "dislike count should render from public aggregates");
  });
  assert.equal(calls.get.some((c) => c.path.startsWith("/api/v1/social/reactions")), false, "anonymous must not fetch viewer reaction state");
});

test("ReactionBar renders viewer_reaction from the read contract and re-displays it after refresh", async () => {
  const calls: ReactionCall = { get: [], post: [] };
  installReactionApiMocks(calls, { counts: { like: 3, dislike: 1 }, viewer_reaction: "like" });

  const view = renderHarness(<ReactionBarHarness contentId={47} />);
  await loginAs(view);

  await waitFor(() => {
    assertPressed(view, "Like", true, "3");
    assertPressed(view, "Dislike", false, "1");
  });

  const reactionGets = calls.get.filter((c) => c.path.startsWith("/api/v1/social/reactions"));
  assert.equal(reactionGets.length, 1, "viewer state fetched once from the read contract");
});

test("ReactionBar removes the reaction when repeating the current one and reconciles from the server snapshot", async () => {
  const calls: ReactionCall = { get: [], post: [] };
  installReactionApiMocks(calls, { counts: { like: 3, dislike: 1 }, viewer_reaction: "like" });

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: "test-access-token" },
        capabilities: { can_interact: true },
      } as T;
    }
    if (path === "/api/v1/social/reactions") {
      return { action: "removed", counts: { like: 2, dislike: 1 }, viewer_reaction: null } as T;
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderHarness(<ReactionBarHarness contentId={47} />);
  await loginAs(view);
  await waitFor(() => {
    assertPressed(view, "Like", true, "3");
  });

  await act(async () => {
    fireEvent.click(reactionButton(view, "Like"));
  });

  await waitFor(() => {
    assertPressed(view, "Like", false, "2");
  });

  const reactCall = calls.post.find((c) => c.path === "/api/v1/social/reactions");
  assert.ok(reactCall, "reaction mutation must be posted");
  assert.deepEqual(reactCall?.body, { target_type: "content", target_id: 47, reaction: "like" });
});

test("ReactionBar switches like to dislike atomically using the server snapshot", async () => {
  const calls: ReactionCall = { get: [], post: [] };
  installReactionApiMocks(calls, { counts: { like: 3, dislike: 1 }, viewer_reaction: "like" });

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: "test-access-token" },
        capabilities: { can_interact: true },
      } as T;
    }
    if (path === "/api/v1/social/reactions") {
      return { action: "updated", counts: { like: 2, dislike: 2 }, viewer_reaction: "dislike" } as T;
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderHarness(<ReactionBarHarness contentId={47} />);
  await loginAs(view);
  await waitFor(() => {
    assertPressed(view, "Like", true, "3");
  });

  await act(async () => {
    fireEvent.click(reactionButton(view, "Dislike"));
  });

  await waitFor(() => {
    assertPressed(view, "Dislike", true, "2");
    assertPressed(view, "Like", false, "2");
  });
});

test("ReactionBar rolls back to the previous state when the mutation fails", async () => {
  const calls: ReactionCall = { get: [], post: [] };
  installReactionApiMocks(calls, { counts: { like: 3, dislike: 1 }, viewer_reaction: "like" });

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: "test-access-token" },
        capabilities: { can_interact: true },
      } as T;
    }
    if (path === "/api/v1/social/reactions") {
      throw new ApiRequestError("INTERNAL_ERROR", "database error", 500);
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderHarness(<ReactionBarHarness contentId={47} />);
  await loginAs(view);
  await waitFor(() => {
    assertPressed(view, "Like", true, "3");
  });

  await act(async () => {
    fireEvent.click(reactionButton(view, "Dislike"));
  });

  await waitFor(() => {
    assertPressed(view, "Like", true, "3");
    assertPressed(view, "Dislike", false, "1");
  });
});
