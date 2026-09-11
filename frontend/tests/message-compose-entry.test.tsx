import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import path from "node:path";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/**
 * T35（#255，FIX-30① Phase 6 修正版）：用户主页「发私信」入口（纯前端）。
 * 走既有 POST /messages 冷启动 guard——首条天然放行（201），连续第二条由后端
 * DM_REPLY_REQUIRED 拦截；本票后端 diff 必须为零，前端只如实转达错误。
 */

const root = path.resolve(process.cwd());

async function read(relativePath: string) {
  return readFile(path.join(root, relativePath), "utf8");
}

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const routerPushes: string[] = [];
const authStub = {
  user: null as null | { id: number; email_verified_at: string },
  capabilities: { can_interact: true, interaction_denial_reason: null as string | null },
};

Module._load = function loadWithT35Stubs(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({
        push: (target: string) => {
          routerPushes.push(target);
        },
      }),
      usePathname: () => "/user/2",
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
      interactionDenialKey: (reason?: string) => `capabilities.denied${reason ?? "Unknown"}`,
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
const originalPost = api.post;
let posted: { path: string; body: unknown }[] = [];
let postError: ApiRequestError | null = null;

function installPostMock() {
  posted = [];
  postError = null;
  api.post = (async <T,>(callPath: string, body?: unknown): Promise<T> => {
    posted.push({ path: callPath, body });
    if (postError) throw postError;
    return { message: { id: 1 } } as T;
  }) as typeof api.post;
}

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.post = originalPost;
  authStub.user = null;
  authStub.capabilities = { can_interact: true, interaction_denial_reason: null };
  routerPushes.length = 0;
});

async function mountButton() {
  const { MessageComposeButton } = await import("../components/social/MessageComposeButton");
  return renderWithIntl(<MessageComposeButton userId={2} displayName="creator" />);
}

test("source: profile wires the compose entry next to FollowButton for other users only", async () => {
  const source = await read("app/(public)/user/[userId]/UserProfileClient.tsx");
  assert.match(source, /MessageComposeButton/, "profile must mount the compose entry");
  assert.match(source, /<FollowButton[^/]*\/>/, "follow button stays");
  assert.match(
    source,
    /\{!isOwnProfile && \([\s\S]*?MessageComposeButton/,
    "compose entry renders only on other users' profiles",
  );
});

test("logged-out click routes to /login without opening the dialog", async () => {
  installDom();
  authStub.user = null;
  const view = await mountButton();

  fireEvent.click(view.getByRole("button", { name: "Message" }));
  assert.deepEqual(routerPushes, ["/login"]);
  assert.equal(view.queryByRole("dialog"), null, "no dialog for anonymous visitors");
});

test("interaction-blocked users get a disabled entry with the denial hint", async () => {
  installDom();
  authStub.user = { id: 1, email_verified_at: "2026-01-01T00:00:00Z" };
  authStub.capabilities = { can_interact: false, interaction_denial_reason: "user_banned" };
  const view = await mountButton();

  const entry = view.getByRole("button", { name: "Message" }) as HTMLButtonElement;
  assert.ok(entry.disabled, "entry disabled while interaction-blocked");
  assert.ok(entry.title.length > 0, "denial reason exposed via title");
});

test("first DM posts recipient_id + text, toasts, closes the dialog and routes to /messages", async () => {
  installDom();
  authStub.user = { id: 1, email_verified_at: "2026-01-01T00:00:00Z" };
  installPostMock();
  const view = await mountButton();

  fireEvent.click(view.getByRole("button", { name: "Message" }));
  const dialog = await view.findByRole("dialog");
  const textarea = view.getByLabelText("Message");
  assert.ok(textarea);
  assert.equal(document.activeElement, textarea, "dialog opens focused on the textarea");

  fireEvent.click(view.getByRole("button", { name: "Send message" }));
  assert.equal(posted.length, 0, "empty text must not send");

  fireEvent.change(textarea, { target: { value: "hello, love your work!" } });
  fireEvent.click(view.getByRole("button", { name: "Send message" }));

  await waitFor(() => {
    assert.deepEqual(posted, [
      { path: "/api/v1/messages", body: { recipient_id: 2, text: "hello, love your work!" } },
    ], "first message posts the exact POST /messages contract");
  });
  await waitFor(() => {
    assert.deepEqual(routerPushes, ["/messages"], "routes to the message center after success");
  });
  await waitFor(() => {
    assert.equal(view.queryByRole("dialog"), null, "dialog closes on success");
  });
  assert.ok(document.body.textContent?.includes("Message sent"), "success toast shown");
});

test("DM_REPLY_REQUIRED keeps the dialog open with the localized guard notice", async () => {
  installDom();
  authStub.user = { id: 1, email_verified_at: "2026-01-01T00:00:00Z" };
  installPostMock();
  postError = new ApiRequestError("DM_REPLY_REQUIRED", "guard", 403);
  const view = await mountButton();

  fireEvent.click(view.getByRole("button", { name: "Message" }));
  const textarea = view.getByLabelText("Message");
  fireEvent.change(textarea, { target: { value: "second ping" } });
  fireEvent.click(view.getByRole("button", { name: "Send message" }));

  await waitFor(() => {
    assert.match(
      view.getByRole("alert").textContent ?? "",
      /Wait for the recipient to reply/,
      "guard error surfaces the existing localized notice",
    );
  });
  assert.ok(view.getByRole("dialog"), "dialog stays open for retry");
  assert.equal(routerPushes.length, 0, "no navigation on failure");
});

test("Escape closes the dialog and restores focus to the entry button", async () => {
  installDom();
  authStub.user = { id: 1, email_verified_at: "2026-01-01T00:00:00Z" };
  const view = await mountButton();

  const entry = view.getByRole("button", { name: "Message" });
  entry.focus();
  fireEvent.click(entry);
  await view.findByRole("dialog");

  fireEvent.keyDown(document, { key: "Escape" });
  await waitFor(() => {
    assert.equal(view.queryByRole("dialog"), null, "Escape closes the dialog");
  });
  await waitFor(() => {
    assert.equal(document.activeElement, entry, "focus returns to the entry button");
  });
});

test("profile integration: entry appears for other users and never on the own profile", async () => {
  installDom();
  installPostMock();
  api.get = (async <T,>(): Promise<T> => ({ contents: [] }) as T) as typeof api.get;
  const { UserProfileClient } = await import("../app/(public)/user/[userId]/UserProfileClient");

  authStub.user = { id: 1, email_verified_at: "2026-01-01T00:00:00Z" };
  const other = renderWithIntl(<UserProfileClient userId={2} displayName="creator" />);
  await waitFor(() => assert.ok(other.getByRole("button", { name: "Message" })));
  cleanup();

  const own = renderWithIntl(<UserProfileClient userId={1} displayName="me" />);
  await waitFor(() => assert.ok(own.getByRole("button", { name: /Edit/i })));
  assert.equal(own.queryByRole("button", { name: "Message" }), null, "own profile shows edit instead");
  assert.equal(own.queryByRole("dialog"), null);
});
