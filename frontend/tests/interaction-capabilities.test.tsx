import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { readFile } from "node:fs/promises";
import { IntlProvider } from "use-intl";

import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError, setAccessToken } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { ReactionBar } from "@/components/social/ReactionBar";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const { within } = require("@testing-library/react") as typeof import("@testing-library/react");

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

test("AuthContext fails closed when the login response omits capabilities", async () => {
  api.post = (async <T,>(path: string): Promise<T> => {
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: validAccessToken() },
      } as T;
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderHarness(
    <AuthProvider>
      <CapabilityProbe />
    </AuthProvider>,
  );

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "login" }));
  });

  await waitFor(() => {
    assert.equal(view.getByTestId("interaction-capability").textContent, "blocked:AUTH_STATUS_UNAVAILABLE");
  });
});

test("AuthContext keeps a server-derived can_interact capability after login", async () => {
  api.post = (async <T,>(path: string): Promise<T> => {
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: validAccessToken() },
        capabilities: { can_interact: true },
      } as T;
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderHarness(<CapabilityProbe />);

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "login" }));
  });

  await waitFor(() => {
    assert.equal(view.getByTestId("interaction-capability").textContent, "allowed");
  });
});

test("AuthContext exposes the server denial reason for a blocked login", async () => {
  api.post = (async <T,>(path: string): Promise<T> => {
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: validAccessToken() },
        capabilities: { can_interact: false, interaction_denial_reason: "INSUFFICIENT_REPUTATION" },
      } as T;
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderHarness(<CapabilityProbe />);

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "login" }));
  });

  await waitFor(() => {
    assert.equal(view.getByTestId("interaction-capability").textContent, "blocked:INSUFFICIENT_REPUTATION");
  });
});

test("ReactionBar disables interaction buttons with a localized reason when blocked", async () => {
  installInteractionMocks("blocked");

  const view = renderHarness(<ReactionBarHarness contentId={5} />);

  await loginAs(view, "blocked");
  await waitFor(() => {
    const actionButtons = view.container.querySelectorAll<HTMLButtonElement>(
      'button[title="Insufficient reputation to perform this action."]',
    );
    assert.equal(actionButtons.length, 3);
    for (const button of actionButtons) {
      assert.equal(button.disabled, true);
    }
  });
});

test("ReactionBar report uses ConfirmModal reason input and submits on success", async () => {
  const calls = installInteractionMocks("allowed");

  const view = renderHarness(<ReactionBarHarness contentId={5} />);
  await loginAs(view, "allowed");

  fireEvent.click(view.getByRole("button", { name: "Report" }));
  const dialog = view.getByRole("dialog", { name: "Report this content" });

  const reasonInput = within(dialog).getByLabelText(intlMessages.social.reportReason);
  fireEvent.change(reasonInput, { target: { value: "spam" } });

  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: "Report" }));
    await Promise.resolve();
  });

  await waitFor(() => {
    assert.equal(view.queryByRole("dialog"), null);
    const reportCall = calls.post.find((call) => call.path === "/api/v1/contents/5/report");
    assert.ok(reportCall, "expected POST /api/v1/contents/5/report");
    assert.deepEqual(reportCall.body, { reason: "spam" });
  });
  assert.ok(view.getByRole("button", { name: "Reported" }));
});

test("ReactionBar report cancel closes the modal without submitting", async () => {
  const calls = installInteractionMocks("allowed");

  const view = renderHarness(<ReactionBarHarness contentId={5} />);
  await loginAs(view, "allowed");

  fireEvent.click(view.getByRole("button", { name: "Report" }));
  const dialog = view.getByRole("dialog", { name: "Report this content" });
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
  });

  assert.equal(view.queryByRole("dialog"), null);
  assert.equal(calls.post.some((call) => call.path === "/api/v1/contents/5/report"), false);
});

test("ReactionBar report keeps the modal open after an API failure", async () => {
  installInteractionMocks("allowed");
  api.post = (async <T,>(path: string): Promise<T> => {
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: validAccessToken() },
        capabilities: { can_interact: true },
      } as T;
    }
    if (path === "/api/v1/contents/5/report") {
      throw new ApiRequestError("INTERNAL_ERROR", "report failed", 500);
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderHarness(<ReactionBarHarness contentId={5} />);
  await loginAs(view, "allowed");

  fireEvent.click(view.getByRole("button", { name: "Report" }));
  const dialog = view.getByRole("dialog", { name: "Report this content" });
  const reasonInput = within(dialog).getByLabelText(intlMessages.social.reportReason);
  fireEvent.change(reasonInput, { target: { value: "spam" } });

  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: "Report" }));
    await Promise.resolve();
  });

  assert.ok(view.getByRole("dialog", { name: "Report this content" }));
  assert.equal(view.queryByRole("button", { name: "Reported" }), null);
});

test("interaction consumers use server capabilities instead of hard-coded thresholds", async () => {
  const audits: Array<[string, string]> = [
    ["../components/social/CommentSection.tsx", "canInteract"],
    ["../components/content/DownloadButton.tsx", "interactionBlocked"],
    ["../app/(protected)/judge/queue/page.tsx", "capabilities"],
    ["../app/(protected)/judge/exam/page.tsx", "capabilities"],
    ["../contexts/AuthContext.tsx", "interaction_denial_reason"],
    ["../components/social/ReactionBar.tsx", "capabilities"],
  ];

  for (const [relativePath, needle] of audits) {
    const source = await readFile(new URL(relativePath, import.meta.url), "utf8");
    assert.match(source, new RegExp(needle), `${relativePath} should consume the server capability`);
    assert.doesNotMatch(source, /reputation\s*<\s*3|reputation\s*>=\s*3/, `${relativePath} must not hard-code the threshold`);
  }

  for (const relativePath of [
    "../app/(protected)/judge/queue/page.tsx",
    "../app/(protected)/judge/exam/page.tsx",
    "../components/social/ReactionBar.tsx",
    "../components/content/DownloadButton.tsx",
  ]) {
    const source = await readFile(new URL(relativePath, import.meta.url), "utf8");
    assert.doesNotMatch(source, /window\.(confirm|prompt)\(/, `${relativePath} must not use native dialogs`);
  }
});

function CapabilityProbe() {
  const { capabilities, login } = useAuth();
  return (
    <div>
      <span data-testid="interaction-capability">
        {capabilities.can_interact
          ? "allowed"
          : `blocked:${capabilities.interaction_denial_reason ?? "AUTH_STATUS_UNAVAILABLE"}`}
      </span>
      <button type="button" onClick={() => void login("person@example.com", "password")}>login</button>
    </div>
  );
}

function ReactionBarHarness({ contentId }: { contentId: number }) {
  const { login } = useAuth();
  return (
    <>
      <ReactionBar contentId={contentId} />
      <button type="button" onClick={() => void login("person@example.com", "password")}>login</button>
    </>
  );
}

async function loginAs(view: ReturnType<typeof render>, capability: "allowed" | "blocked") {
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "login" }));
    await Promise.resolve();
  });
}

function installInteractionMocks(capability: "allowed" | "blocked" = "blocked") {
  const calls: { get: Array<{ path: string }>; post: Array<{ path: string; body?: unknown }> } = {
    get: [],
    post: [],
  };
  const capabilities =
    capability === "allowed"
      ? { can_interact: true }
      : { can_interact: false, interaction_denial_reason: "INSUFFICIENT_REPUTATION" };

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (path === "/api/v1/auth/me") {
      return { user: testUser } as T;
    }
    if (path === "/api/v1/notifications/unread-count") {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path.startsWith("/api/v1/social/reactions")) {
      return { reaction: null } as T;
    }
    return {} as T;
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return {
        user: testUser,
        tokens: { access_token: validAccessToken() },
        capabilities,
      } as T;
    }
    if (path === "/api/v1/contents/5/report") {
      return {} as T;
    }
    return {} as T;
  }) as typeof api.post;

  return calls;
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

function validAccessToken() {
  const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64url");
  return `header.${payload}.signature`;
}
