import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";

import { ApiRequestError, api } from "@/lib/api";
import { ToastProvider } from "@/components/ui/Toast";

import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire & {
  extensions: NodeJS.RequireExtensions;
};
requireForMocks.extensions[".css"] = () => undefined;

const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

Module._load = function loadWithMarkdownStubs(request, parent, isMain) {
  if (request === "@/components/content/MarkdownEditor") {
    return {
      MarkdownEditor({
        id,
        value,
        onChange,
        disabled,
        ariaLabel,
        ariaDescribedBy,
        ariaInvalid,
      }: {
        id?: string;
        value: string;
        onChange: (value: string) => void;
        disabled?: boolean;
        ariaLabel?: string;
        ariaDescribedBy?: string;
        ariaInvalid?: boolean;
      }) {
        return (
          <textarea
            id={id}
            aria-label={ariaLabel}
            aria-describedby={ariaDescribedBy}
            aria-invalid={ariaInvalid}
            data-testid="markdown-editor"
            disabled={disabled}
            value={value}
            onChange={(event) => onChange(event.currentTarget.value)}
          />
        );
      },
    };
  }
  if (request === "@/components/content/MarkdownRenderer") {
    return {
      MarkdownRenderer({ content }: { content: string; className?: string }) {
        return <div data-content={content} data-testid="markdown-renderer">{content}</div>;
      },
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

const originalPost = api.post;
const originalConsoleWarn = console.warn;

const intlMessages = {
  common: {
    cancel: "Cancel",
    confirm: "Confirm",
    reason: "Reason",
    processing: "Processing",
    operationFailed: "Operation failed",
  },
  adminNotifications: {
    title: "System broadcast",
    subtitle: "Send a Markdown notification to active users.",
    form: {
      titleLabel: "Title",
      titlePlaceholder: "Maintenance notice",
      titleHint: "1-120 characters",
      bodyLabel: "Body",
      bodyHint: "Markdown supported, 1-5000 characters",
      send: "Send broadcast",
      sending: "Sending...",
      titleCount: "{count}/120",
      bodyCount: "{count}/5000",
    },
    preview: {
      title: "Preview",
      emptyTitle: "Nothing to preview",
      emptyDescription: "Write a title and Markdown body to preview the notification.",
    },
    confirm: {
      title: "Send broadcast?",
      description: "Broadcast notifications are sent to active users and cannot be recalled.",
      confirm: "Send broadcast",
    },
    toast: {
      success: "Broadcast sent to {count} users at {time}.",
      failure: "Failed to send broadcast. Please try again.",
    },
    validation: {
      titleRequired: "Title is required.",
      bodyRequired: "Body is required.",
      titleTooLong: "Title must be 120 characters or fewer.",
      bodyTooLong: "Body must be 5000 characters or fewer.",
    },
    a11y: {
      preview: "Broadcast notification preview",
    },
  },
};

type ApiCall = {
  path: string;
  body: unknown;
};

test.afterEach(() => {
  cleanup();
  api.post = originalPost;
  console.warn = originalConsoleWarn;
});

test("requires title and body before the broadcast can be submitted", async () => {
  installAdminDom();
  const calls = installApiPostMock();
  const view = await renderPage();

  const sendButton = view.getByRole("button", { name: intlMessages.adminNotifications.form.send }) as HTMLButtonElement;
  assert.equal(sendButton.disabled, true);

  fireEvent.change(view.getByLabelText(intlMessages.adminNotifications.form.titleLabel), {
    target: { value: "Maintenance" },
  });

  await waitFor(() => {
    assert.equal(sendButton.disabled, true);
    assert.equal(calls.length, 0);
  });

  fireEvent.change(view.getByLabelText(intlMessages.adminNotifications.form.bodyLabel), {
    target: { value: "Maintenance starts at 02:00." },
  });

  await waitFor(() => {
    assert.equal(sendButton.disabled, false);
  });
});

test("renders the live preview through MarkdownRenderer", async () => {
  installAdminDom();
  installApiPostMock();
  const view = await renderPage();

  fireEvent.change(view.getByLabelText(intlMessages.adminNotifications.form.titleLabel), {
    target: { value: "Preview title" },
  });
  fireEvent.change(view.getByLabelText(intlMessages.adminNotifications.form.bodyLabel), {
    target: { value: "Body with **Markdown**" },
  });

  await waitFor(() => {
    const renderer = view.getByTestId("markdown-renderer");
    assert.equal(renderer.getAttribute("data-content"), "Body with **Markdown**");
    assert.equal(view.getByLabelText(intlMessages.adminNotifications.a11y.preview).getAttribute("aria-live"), "polite");
  });
});

test("opens ConfirmModal before making the broadcast API call", async () => {
  installAdminDom();
  const calls = installApiPostMock();
  const view = await renderValidPage();

  fireEvent.click(view.getByRole("button", { name: intlMessages.adminNotifications.form.send }));

  await waitFor(() => {
    assert.ok(view.getByRole("dialog", { name: intlMessages.adminNotifications.confirm.title }));
    assert.equal(calls.length, 0);
    assert.ok(document.body.textContent?.includes("cannot be recalled"));
  });
});

test("traps focus inside the confirmation modal", async () => {
  installAdminDom();
  installApiPostMock();
  const view = await renderValidPage();

  fireEvent.click(view.getByRole("button", { name: intlMessages.adminNotifications.form.send }));

  const dialog = await findDialog(view);
  const cancelButton = getDialogButton(dialog, intlMessages.common.cancel);
  const confirmButton = getDialogButton(dialog, intlMessages.adminNotifications.confirm.confirm);

  confirmButton.focus();
  fireEvent.keyDown(dialog, { key: "Tab" });
  assert.equal(document.activeElement, cancelButton);

  cancelButton.focus();
  fireEvent.keyDown(dialog, { key: "Tab", shiftKey: true });
  assert.equal(document.activeElement, confirmButton);
});

test("closes the confirmation modal with Esc without calling the API", async () => {
  installAdminDom();
  const calls = installApiPostMock();
  const view = await renderValidPage();

  fireEvent.click(view.getByRole("button", { name: intlMessages.adminNotifications.form.send }));
  const dialog = await findDialog(view);

  fireEvent.keyDown(dialog, { key: "Escape" });

  await waitFor(() => {
    assert.equal(view.queryByRole("dialog", { name: intlMessages.adminNotifications.confirm.title }), null);
    assert.equal(calls.length, 0);
  });
});

test("confirming the modal posts the broadcast request and shows recipient count", async () => {
  installAdminDom();
  const calls = installApiPostMock({
    data: {
      recipient_count: 37,
      broadcast_at: "2026-07-01T10:15:00Z",
    },
  });
  const view = await renderValidPage();

  fireEvent.click(view.getByRole("button", { name: intlMessages.adminNotifications.form.send }));
  fireEvent.click(await findConfirmButton(view));

  await waitFor(() => {
    assert.deepEqual(calls, [
      {
        path: "/api/v1/admin/notifications/broadcast",
        body: {
          title: "Maintenance",
          body: "Maintenance starts at 02:00.",
          channel: "broadcast",
        },
      },
    ]);
    assert.ok(document.body.textContent?.includes("Broadcast sent to 37 users"));
  });
});

test("shows a localized failure toast when the broadcast API fails", async () => {
  installAdminDom();
  suppressExpectedSilentErrorLog();
  installApiPostMock({
    error: new ApiRequestError("VALIDATION_ERROR", "invalid broadcast", 400),
  });
  const view = await renderValidPage();

  fireEvent.click(view.getByRole("button", { name: intlMessages.adminNotifications.form.send }));
  fireEvent.click(await findConfirmButton(view));

  await waitFor(() => {
    assert.ok(document.body.textContent?.includes(intlMessages.adminNotifications.toast.failure));
    assert.equal(
      (view.getByLabelText(intlMessages.adminNotifications.form.titleLabel) as HTMLInputElement).value,
      "Maintenance",
    );
    assert.equal(
      (view.getByLabelText(intlMessages.adminNotifications.form.bodyLabel) as HTMLTextAreaElement).value,
      "Maintenance starts at 02:00.",
    );
  });
});

test("ConfirmModal labels the required reason textarea", async () => {
  installAdminDom();
  const { render } = await import("@testing-library/react");
  const { ConfirmModal } = await import("@/components/ui/confirm-modal");

  const view = render(
    <IntlProvider locale="en" messages={intlMessages}>
      <ConfirmModal
        open
        onOpenChange={() => undefined}
        title="Archive item?"
        description="Please confirm this moderation action."
        requireReason
        onConfirm={() => undefined}
      />
    </IntlProvider>,
  );

  const reasonInput = view.getByLabelText(intlMessages.common.reason) as HTMLTextAreaElement;
  assert.equal(reasonInput.placeholder, intlMessages.common.reason);
});

async function renderValidPage() {
  const view = await renderPage();
  fireEvent.change(view.getByLabelText(intlMessages.adminNotifications.form.titleLabel), {
    target: { value: "Maintenance" },
  });
  fireEvent.change(view.getByLabelText(intlMessages.adminNotifications.form.bodyLabel), {
    target: { value: "Maintenance starts at 02:00." },
  });
  await waitFor(() => {
    assert.equal(
      (view.getByRole("button", { name: intlMessages.adminNotifications.form.send }) as HTMLButtonElement).disabled,
      false,
    );
  });
  return view;
}

async function renderPage() {
  const { render } = await import("@testing-library/react");
  const pageModule = await import("../app/(protected)/admin/notifications/page");
  const AdminNotificationsPage = pageModule.default;
  const originalConsoleError = console.error;
  console.error = (...args: unknown[]) => {
    if (
      args.some(
        (arg) =>
          typeof arg === "object" &&
          arg !== null &&
          "code" in arg &&
          (arg as { code?: string }).code === "ENVIRONMENT_FALLBACK",
      )
    ) {
      return;
    }
    originalConsoleError(...args);
  };

  try {
    return render(
      <IntlProvider locale="en" messages={intlMessages}>
        <ToastProvider>
          <AdminNotificationsPage />
        </ToastProvider>
      </IntlProvider>,
    );
  } finally {
    console.error = originalConsoleError;
  }
}

async function findDialog(view: Awaited<ReturnType<typeof renderPage>>) {
  let dialog: HTMLElement | undefined;
  await waitFor(() => {
    dialog = view.getByRole("dialog", { name: intlMessages.adminNotifications.confirm.title });
  });
  assert.ok(dialog);
  return dialog;
}

async function findConfirmButton(view: Awaited<ReturnType<typeof renderPage>>) {
  let button: HTMLButtonElement | undefined;
  await waitFor(() => {
    const dialog = view.getByRole("dialog", { name: intlMessages.adminNotifications.confirm.title });
    button = getDialogButton(dialog, intlMessages.adminNotifications.confirm.confirm);
  });
  assert.ok(button);
  return button;
}

function installApiPostMock(options: { data?: { recipient_count: number; broadcast_at: string }; error?: Error } = {}) {
  const calls: ApiCall[] = [];
  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.push({ path, body });
    if (options.error) {
      throw options.error;
    }
    return { data: options.data ?? { recipient_count: 12, broadcast_at: "2026-07-01T10:00:00Z" } } as T;
  }) as typeof api.post;
  return calls;
}

function installAdminDom() {
  const dom = installDom();
  Object.defineProperty(globalThis, "requestAnimationFrame", {
    configurable: true,
    writable: true,
    value: dom.window.requestAnimationFrame,
  });
  Object.defineProperty(globalThis, "cancelAnimationFrame", {
    configurable: true,
    writable: true,
    value: dom.window.cancelAnimationFrame,
  });
}

function suppressExpectedSilentErrorLog() {
  console.warn = (...args: unknown[]) => {
    if (
      args.some(
        (arg) =>
          typeof arg === "string" &&
          arg.includes("[silent-api-error] AdminNotificationsPage:handleConfirm"),
      )
    ) {
      return;
    }
    originalConsoleWarn(...args);
  };
}

function getDialogButton(dialog: HTMLElement, name: string): HTMLButtonElement {
  const button = Array.from(dialog.querySelectorAll("button")).find(
    (candidate) => candidate.textContent?.trim() === name,
  );
  assert.ok(button, `expected dialog button named ${name}`);
  return button;
}
