import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";

import { AuthProvider } from "@/contexts/AuthContext";
import { ApiRequestError, api, setAccessToken } from "@/lib/api";
import SettingsPage from "@/app/(protected)/settings/page";
import { ToastProvider } from "@/components/ui/Toast";

import { act, cleanup, fireEvent, installDom, renderWithIntl, waitFor } from "./runtime-test-helpers";

const originalGet = api.get;
const originalPost = api.post;
const originalPatch = api.patch;

const user = {
  id: 1,
  email: "alice@example.test",
  username: "alice",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "en",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-06-30T00:00:00Z",
  created_at: "2026-06-30T00:00:00Z",
  accept_collab_invites: true,
};

const intlMessages = {
  common: {
    close: "Close",
    cancel: "Cancel",
    saving: "Saving",
    processing: "Processing",
    saveSuccess: "Saved",
    saveFailed: "Save failed",
    operationFailed: "Operation failed",
    rateLimited: "Too many attempts, try again later.",
  },
  settings: {
    title: "Account settings",
    subtitle: "Manage your profile",
    username: "Username",
    email: "Email",
    emailHint: "Email cannot be changed",
    bio: "Bio",
    saveButton: "Save changes",
    changePassword: "Change password",
    oldPassword: "Old password",
    newPassword: "New password",
    confirmPassword: "Confirm new password",
    passwordMismatch: "Passwords do not match",
    passwordTooShort: "Password must be at least 8 characters",
    passwordChanged: "Password changed",
    emailVerified: "Verified",
    emailUnverified: "Unverified",
    legalTitle: "Legal",
    deleteAccount: "Delete account",
    deleteAccountDesc: "This action cannot be undone.",
    enterPasswordToConfirm: "Enter your password to confirm",
    deleteIrreversible: "I understand this is irreversible",
    confirmDelete: "Confirm deletion",
    avatar: "Avatar",
    changeAvatar: "Change avatar",
    avatarHint: "JPG or PNG, max 20MB",
    collaboration: {
      title: "Collaboration invites",
      description: "Control who can invite you to collaborate on content.",
      acceptInvites: {
        label: "Accept collaboration invites",
        help: "When off, other users cannot send you collaboration invites.",
      },
      toast: {
        saved: "Setting saved",
        failed: "Could not save this setting.",
      },
      error: {
        save: "Could not save this setting. Please try again.",
      },
      a11y: {
        acceptInvites: "Accept collaboration invites",
      },
    },
  },
  auth: {
    termsOfService: "Terms of service",
    privacyPolicy: "Privacy policy",
  },
  footer: {
    feedback: "Feedback",
  },
};

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.post = originalPost;
  api.patch = originalPatch;
  setAccessToken(null);
});

async function awaitLoadedUser(view: ReturnType<typeof renderWithIntl>) {
  await waitFor(() => {
    assert.ok(view.container.querySelector('input[value="alice"]'), "user profile should load before interacting");
  });
}

test("collaboration switch initializes from user.accept_collab_invites", async () => {
  installSettingsDom();
  installSettingsApiMocks();
  const view = renderSettings();

  const toggle = await waitFor(() => {
    const element = view.getByRole("switch") as HTMLButtonElement;
    assert.equal(element.getAttribute("aria-checked"), "true", "switch should reflect accept_collab_invites=true");
    return element;
  });
  assert.ok(toggle, "collaboration switch should render");
});

test("toggling the collaboration switch sends PATCH with only accept_collab_invites", async () => {
  installSettingsDom();
  const calls = installSettingsApiMocks();
  const view = renderSettings();

  await awaitLoadedUser(view);
  const toggle = await waitFor(() => view.getByRole("switch") as HTMLButtonElement);
  fireEvent.click(toggle);

  await waitFor(() => {
    const patch = calls.patch.find((call) => call.path === "/api/v1/users/1");
    assert.ok(patch, `expected PATCH /api/v1/users/1, got ${JSON.stringify(calls.patch)}`);
    assert.deepEqual(patch.body, { accept_collab_invites: false });
  });
});

test("successful collaboration save refreshes the user from /auth/me", async () => {
  installSettingsDom();
  const calls = installSettingsApiMocks();
  const view = renderSettings();

  await awaitLoadedUser(view);
  const toggle = await waitFor(() => view.getByRole("switch") as HTMLButtonElement);
  fireEvent.click(toggle);

  await waitFor(() => {
    assert.equal(
      calls.get.filter((call) => call.path === "/api/v1/auth/me").length,
      2,
      "refreshUser should fetch /auth/me again after a successful save",
    );
    assert.ok(view.getByText(intlMessages.settings.collaboration.toast.saved), "success toast should render");
  });
});

test("failed collaboration save rolls back the switch and shows toast plus inline error", async () => {
  installSettingsDom();
  let rejectPatch: (error: Error) => void = () => {};
  const failingPatch = new Promise((_, reject) => {
    rejectPatch = reject;
  });
  installSettingsApiMocks({ patchPromise: failingPatch });
  const view = renderSettings();

  await awaitLoadedUser(view);
  const toggle = await waitFor(() => view.getByRole("switch") as HTMLButtonElement);
  assert.equal(toggle.getAttribute("aria-checked"), "true");
  fireEvent.click(toggle);

  await waitFor(() => assert.equal(toggle.getAttribute("aria-checked"), "false", "switch should optimistically toggle"));

  await act(async () => {
    rejectPatch(new ApiRequestError("DB_ERROR", "database error", 500));
    await failingPatch.catch(() => undefined);
  });

  await waitFor(() => {
    assert.equal(toggle.getAttribute("aria-checked"), "true", "switch should roll back to the server value after failure");
    assert.ok(
      view.container.querySelector('[role="alert"]')?.textContent?.includes(intlMessages.settings.collaboration.error.save),
      "inline error should render",
    );
    assert.ok(document.body.textContent?.includes(intlMessages.settings.collaboration.toast.failed), "error toast should render");
  });
});

test("password and delete controls stay enabled while the collaboration switch is saving", async () => {
  installSettingsDom();
  let resolvePatch: (value: unknown) => void = () => {};
  const pendingPatch = new Promise((resolve) => {
    resolvePatch = resolve;
  });
  installSettingsApiMocks({ patchPromise: pendingPatch });
  const view = renderSettings();

  await awaitLoadedUser(view);
  const toggle = await waitFor(() => view.getByRole("switch") as HTMLButtonElement);
  const passwordButton = view.getByRole("button", { name: intlMessages.settings.changePassword }) as HTMLButtonElement;
  const deleteButton = view.getByRole("button", { name: intlMessages.settings.deleteAccount }) as HTMLButtonElement;
  const passwordDisabledBefore = passwordButton.disabled;
  const deleteDisabledBefore = deleteButton.disabled;

  fireEvent.click(toggle);

  await waitFor(() => assert.equal(toggle.disabled, true, "collaboration switch should be disabled while saving"));
  assert.equal(passwordButton.disabled, passwordDisabledBefore, "password controls must not be locked by the collaboration save");
  assert.equal(deleteButton.disabled, deleteDisabledBefore, "delete-account controls must not be locked by the collaboration save");

  await act(async () => {
    resolvePatch({});
    await pendingPatch;
  });
});

test("collaboration switch has an explicit visible label", async () => {
  installSettingsDom();
  installSettingsApiMocks();
  const view = renderSettings();

  await waitFor(() => assert.ok(view.getByText(intlMessages.settings.collaboration.acceptInvites.label)));
  assert.ok(view.getByText(intlMessages.settings.collaboration.acceptInvites.help));
  assert.ok(view.getByText(intlMessages.settings.collaboration.title));
});

function renderSettings() {
  setValidAccessToken();
  return renderWithIntl(
    <IntlProvider locale="en" messages={intlMessages}>
      <AppRouterContext.Provider value={testRouter}>
        <ToastProvider>
          <AuthProvider>
            <SettingsPage />
          </AuthProvider>
        </ToastProvider>
      </AppRouterContext.Provider>
    </IntlProvider>,
  );
}

const testRouter = {
  back() {},
  forward() {},
  prefetch() {},
  bfcacheId: "test-bfcache",
  push() {},
  refresh() {},
  replace() {},
};

function installSettingsDom() {
  installDom();
}

type SettingsMockOptions = {
  patchError?: Error;
  patchPromise?: Promise<unknown>;
};

function installSettingsApiMocks(options: SettingsMockOptions = {}) {
  const calls: { get: Array<{ path: string }>; post: Array<{ path: string; body?: unknown }>; patch: Array<{ path: string; body?: unknown }> } = {
    get: [],
    post: [],
    patch: [],
  };

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (path === "/api/v1/auth/me") {
      return { user } as T;
    }
    if (path === "/api/v1/notifications/unread-count") {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    throw new Error(`unexpected api.get path ${path}`);
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    throw new Error(`unexpected api.post path ${path}`);
  }) as typeof api.post;

  api.patch = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.patch.push({ path, body });
    if (options.patchError) {
      throw options.patchError;
    }
    if (options.patchPromise) {
      return (await options.patchPromise) as T;
    }
    return {} as T;
  }) as typeof api.patch;

  return calls;
}

function setValidAccessToken() {
  const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64");
  setAccessToken(`test.${payload}.signature`);
}
