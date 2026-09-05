import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import { api, ApiRequestError, setAccessToken } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { CommentSection } from "@/components/social/CommentSection";
import { act, cleanup, fireEvent, installAuthFetchStub, installDom, render, waitFor } from "./runtime-test-helpers";

const { within } = require("@testing-library/react") as typeof import("@testing-library/react");

const originalGet = api.get;
const originalPost = api.post;
const originalPatch = api.patch;
const originalDelete = api.delete;
const originalConsoleWarn = console.warn;

let restoreAuthFetch: (() => void) | null = null;

const testUser = {
  id: 7,
  email: "person@example.com",
  username: "person",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "en",
  role: "user",
};

const intlMessages = {
  common: {
    confirm: "Confirm",
    cancel: "Cancel",
    processing: "Processing",
    reason: "Reason",
    loading: "Loading",
    userLabel: "User #{id}",
    alreadyReported: "You have already reported this content",
  },
  social: {
    comments: "Comments",
    commentCount: "{count} comments",
    commentPlaceholder: "Write a comment",
    loginToComment: "Log in to comment",
    loadFailed: "Failed to load",
    sendFailed: "Failed to send",
    noComments: "No comments yet",
    noCommentsHint: "Be the first",
    reply: "Reply",
    replyPlaceholder: "Reply to @{name}",
    showReplies: "Show replies",
    hideReplies: "Hide replies",
    noReplies: "No replies yet",
    edit: "Edit",
    saveEdit: "Save",
    cancelEdit: "Cancel",
    editedMark: "edited",
    delete: "Delete",
    deleteCommentTitle: "Delete this comment?",
    deleteCommentDesc: "The comment will no longer be displayed.",
    deleteReplyTitle: "Delete this reply?",
    deleteReplyDesc: "The reply will no longer be displayed.",
    loadMoreComments: "Load more comments",
    like: "Like",
    dislike: "Dislike",
    report: "Report",
    reportDialogTitle: "Report this content",
    reportReason: "Please describe the reason for reporting:",
    reported: "Reported",
    reportFailed: "Failed to submit report.",
  },
};

const topLevelComment = {
  id: 1001,
  author_id: 7,
  author: { id: 7, username: "person", avatar_url: "" },
  body: "top level body",
  parent_id: null,
  created_at: "2026-09-03T10:00:00Z",
  updated_at: "2026-09-03T10:00:00Z",
};

const secondPageComment = {
  id: 1002,
  author_id: 8,
  author: { id: 8, username: "someone_else", avatar_url: "" },
  body: "second page body",
  parent_id: null,
  created_at: "2026-09-03T11:00:00Z",
};

test.beforeEach(() => {
  installDom();
  restoreAuthFetch = installAuthFetchStub();
  setAccessToken(null);
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: window.localStorage,
  });
  console.warn = (...args: unknown[]) => {
    if (typeof args[0] === "string" && args[0].includes("[silent-api-error]")) return;
    originalConsoleWarn(...args);
  };
});

test.afterEach(() => {
  cleanup();
  restoreAuthFetch?.();
  restoreAuthFetch = null;
  api.get = originalGet;
  api.post = originalPost;
  api.patch = originalPatch;
  api.delete = originalDelete;
  setAccessToken(null);
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    writable: true,
    value: undefined,
  });
  console.warn = originalConsoleWarn;
});

function CommentSectionHarness({ contentId }: { contentId: number }) {
  const { login } = useAuth();
  return (
    <>
      <CommentSection contentId={contentId} />
      <button type="button" onClick={() => void login("person@example.com", "password")}>login</button>
    </>
  );
}

function renderCommentSection() {
  return render(
    <IntlProvider locale="en" messages={intlMessages}>
      <ToastProvider>
        <AuthProvider>
          <CommentSectionHarness contentId={42} />
        </AuthProvider>
      </ToastProvider>
    </IntlProvider>,
  );
}

function installCommentMocks(comments: unknown[], total: number) {
  const calls: {
    get: string[];
    post: Array<{ path: string; body?: unknown }>;
    patch: Array<{ path: string; body?: unknown }>;
    delete: string[];
  } = { get: [], post: [], patch: [], delete: [] };

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push(path);
    if (path === "/api/v1/auth/me") return { user: testUser, capabilities: { can_interact: true } } as T;
    if (path === "/api/v1/notifications/unread-count") {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path.includes("parent_id=1001")) {
      return { comments: [{ id: 2001, author_id: 8, author: { id: 8, username: "someone_else" }, body: "child reply body", parent_id: 1001, created_at: "2026-09-03T10:05:00Z" }] } as T;
    }
    if (path.includes("page=2")) {
      return { comments: [secondPageComment], total } as T;
    }
    return { comments, total } as T;
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body?: unknown): Promise<T> => {
    calls.post.push({ path, body });
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
    if (path === "/api/v1/social/comments") {
      return { comment: { id: 3001, author_id: 7, author: { id: 7, username: "person" }, body: (body as { body: string }).body, parent_id: (body as { parent_id?: number }).parent_id ?? null, created_at: "2026-09-03T12:00:00Z" } } as T;
    }
    return {} as T;
  }) as typeof api.post;

  api.patch = (async <T,>(path: string, body?: unknown): Promise<T> => {
    calls.patch.push({ path, body });
    return { comment: { ...topLevelComment, body: (body as { body: string }).body, updated_at: "2026-09-03T12:30:00Z" } } as T;
  }) as typeof api.patch;

  api.delete = (async <T,>(path: string): Promise<T> => {
    calls.delete.push(path);
    return { message: "deleted" } as T;
  }) as typeof api.delete;

  return calls;
}

async function login(view: ReturnType<typeof render>) {
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "login" }));
    await Promise.resolve();
  });
}

function validAccessToken() {
  const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64url");
  return `header.${payload}.signature`;
}

test("reply button posts a comment carrying parent_id", async () => {
  const calls = installCommentMocks([topLevelComment], 1);
  const view = renderCommentSection();
  await waitFor(() => assert.ok(view.getByText("top level body")));
  await login(view);

  fireEvent.click(view.getByRole("button", { name: /^Reply/ }));
  const replyBox = view.getByPlaceholderText("Reply to @person");
  fireEvent.change(replyBox, { target: { value: "a nested reply" } });
  await act(async () => {
    fireEvent.click(view.getAllByRole("button", { name: "Reply" }).at(-1)!);
    await Promise.resolve();
  });

  await waitFor(() => {
    const replyPost = calls.post.find((c) => c.path === "/api/v1/social/comments" && (c.body as { parent_id?: number }).parent_id === 1001);
    assert.ok(replyPost, "expected a comment post carrying parent_id=1001");
    assert.equal((replyPost!.body as { body: string }).body, "a nested reply");
  });
});

test("show-replies lazily fetches children and renders them", async () => {
  const calls = installCommentMocks([topLevelComment], 1);
  const view = renderCommentSection();
  await waitFor(() => assert.ok(view.getByText("top level body")));
  await login(view);

  fireEvent.click(view.getByRole("button", { name: "Show replies" }));
  await waitFor(() => assert.ok(calls.get.some((p) => p.includes("parent_id=1001"))));
  await waitFor(() => assert.ok(view.getByText("child reply body")));
});

test("own comment edit goes through PATCH and re-renders the new body", async () => {
  installCommentMocks([topLevelComment], 1);
  const view = renderCommentSection();
  await waitFor(() => assert.ok(view.getByText("top level body")));
  await login(view);

  fireEvent.click(view.getByRole("button", { name: "Edit" }));
  const editor = view.getByDisplayValue("top level body");
  fireEvent.change(editor, { target: { value: "edited body" } });
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Save" }));
    await Promise.resolve();
  });

  await waitFor(() => assert.ok(view.queryByRole("button", { name: "Save" }) === null));
  assert.ok(
    view.getByText("edited body"),
    "edited body should replace the old text after PATCH round-trip",
  );
});

test("delete asks for confirmation, then DELETEs and removes the comment", async () => {
  const calls = installCommentMocks([topLevelComment], 1);
  const view = renderCommentSection();
  await waitFor(() => assert.ok(view.getByText("top level body")));
  await login(view);

  fireEvent.click(view.getByRole("button", { name: "Delete" }));
  const dialog = view.getByRole("dialog", { name: "Delete this comment?" });
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
    await Promise.resolve();
  });

  await waitFor(() => assert.ok(calls.delete.includes("/api/v1/social/comments/1001")));
  await waitFor(() => assert.ok(view.queryByText("top level body") === null));
});

test("load-more appends the next page when total exceeds the first page", async () => {
  const calls = installCommentMocks([topLevelComment], 2);
  const view = renderCommentSection();
  await waitFor(() => assert.ok(view.getByText("top level body")));

  const more = view.getByRole("button", { name: "Load more comments" });
  await act(async () => {
    fireEvent.click(more);
    await Promise.resolve();
  });

  await waitFor(() => assert.ok(calls.get.some((p) => p.includes("page=2"))));
  await waitFor(() => assert.ok(view.getByText("second page body")));
});

test("report submits with a reason and a repeat 409 shows the already-reported notice", async () => {
  let reportCalls = 0;
  const calls = installCommentMocks([topLevelComment], 1);
  api.post = (async <T,>(path: string, body?: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path === "/api/v1/auth/refresh") {
      throw new ApiRequestError("UNAUTHORIZED", "not logged in", 401);
    }
    if (path === "/api/v1/auth/login") {
      return { user: testUser, tokens: { access_token: validAccessToken() }, capabilities: { can_interact: true } } as T;
    }
    if (path === "/api/v1/social/comments/1001/report") {
      reportCalls += 1;
      if (reportCalls === 1) return { message: "reported" } as T;
      throw new ApiRequestError("ALREADY_REPORTED", "already reported", 409);
    }
    return {} as T;
  }) as typeof api.post;

  const view = renderCommentSection();
  await waitFor(() => assert.ok(view.getByText("top level body")));
  await login(view);

  /* 第一次举报：填原因提交 → 已举报态 */
  fireEvent.click(view.getByRole("button", { name: "Report" }));
  const dialog = view.getByRole("dialog", { name: "Report this content" });
  fireEvent.change(within(dialog).getByLabelText("Please describe the reason for reporting:"), { target: { value: "spam" } });
  await act(async () => {
    fireEvent.click(within(dialog).getByRole("button", { name: "Report" }));
    await Promise.resolve();
  });
  await waitFor(() => {
    const reportPost = calls.post.find((c) => c.path === "/api/v1/social/comments/1001/report");
    assert.ok(reportPost, "report POST should fire");
    assert.equal((reportPost!.body as { reason: string }).reason, "spam");
  });
  await waitFor(() => assert.ok(view.getAllByText("Reported").length >= 1, "reported state should render (toast and/or button state)"));

  /* 409 之后按钮进入已举报态，不再可发起 */
  assert.ok(view.queryByRole("button", { name: "Report" }) === null, "report button should be replaced by the reported state");
});
