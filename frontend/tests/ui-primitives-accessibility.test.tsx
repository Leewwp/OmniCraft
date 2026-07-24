import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import React, { useState } from "react";
import { IntlProvider } from "use-intl";

import { Button } from "@/components/ui/button";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { TagBadge } from "@/components/ui/TagBadge";
import { ToastProvider, useToast } from "@/components/ui/Toast";

import {
  cleanup,
  fireEvent,
  installDom,
  render,
  renderWithIntl,
  testMessages,
} from "./runtime-test-helpers";

test.afterEach(() => {
  cleanup();
});

test("Button makes titled icon actions explicit, coarse-pointer safe, and natively disabled", () => {
  installDom();
  let clicks = 0;
  const view = renderWithIntl(
    <>
      <Button size="icon" title="Edit item">
        <span aria-hidden="true">×</span>
      </Button>
      <Button disabled onClick={() => { clicks += 1; }}>
        Save
      </Button>
    </>,
  );

  const iconButton = view.getByRole("button", { name: "Edit item" });
  assert.equal(iconButton.getAttribute("aria-label"), "Edit item");
  assert.match(iconButton.className, /\[@media\(pointer:coarse\)\]:size-11/);

  const disabledButton = view.getByRole("button", { name: "Save" });
  assert.equal(disabledButton.getAttribute("disabled"), "");
  fireEvent.click(disabledButton);
  assert.equal(clicks, 0);
});

test("TagBadge exposes a localized remove action and uses the established icon system", async () => {
  installDom();
  let removes = 0;
  const view = renderWithIntl(
    <TagBadge onRemove={() => { removes += 1; }}>Music</TagBadge>,
  );

  fireEvent.click(view.getByRole("button", { name: "Remove Music" }));
  assert.equal(removes, 1);

  const source = await readFile(new URL("../components/ui/TagBadge.tsx", import.meta.url), "utf8");
  assert.match(source, /import\s*\{\s*X\s*\}\s*from\s*["']lucide-react["']/);
  assert.doesNotMatch(source, /<svg\b/);
});

test("Toast items use assertive or polite roles and label their close actions", async () => {
  installDom();
  const view = renderWithIntl(
    <ToastProvider>
      <ToastHarness />
    </ToastProvider>,
  );

  fireEvent.click(view.getByRole("button", { name: "Show error" }));
  const alert = await view.findByRole("alert");
  assert.match(alert.textContent ?? "", /Could not save/);
  const closeAlert = view.getByRole("button", { name: "Close" });
  assert.match(closeAlert.className, /\[@media\(pointer:coarse\)\]:size-11/);

  fireEvent.click(view.getByRole("button", { name: "Show success" }));
  const status = await view.findByRole("status");
  assert.match(status.textContent ?? "", /Saved/);
});

test("ConfirmModal validates a required reason and traps focus", async () => {
  installDom();
  const confirmations: string[] = [];
  const view = renderWithIntl(<ModalHarness onConfirm={(reason) => confirmations.push(reason)} />);
  const trigger = view.getByRole("button", { name: "Open confirmation" });

  fireEvent.click(trigger);
  await new Promise((resolve) => window.setTimeout(resolve, 0));
  const dialog = view.getByRole("dialog", { name: "Reject request" });
  const reason = view.getByRole("textbox", { name: "Reason" });
  const confirm = view.getByRole("button", { name: "Reject" });

  assert.equal(document.activeElement, reason);
  assert.equal(reason.getAttribute("required"), "");
  assert.equal(confirm.getAttribute("disabled"), "");

  fireEvent.change(reason, { target: { value: "Missing attribution" } });
  assert.equal(confirm.getAttribute("disabled"), null);
  confirm.focus();
  fireEvent.keyDown(document, { key: "Tab" });
  assert.equal(document.activeElement, reason);
  assert.equal(confirmations.length, 0);
  assert.ok(dialog.getAttribute("aria-describedby"));
});

test("ConfirmModal closes with Escape and restores focus", async () => {
  installDom();
  const openChanges: boolean[] = [];
  const modalTree = (open: boolean) => (
    <IntlProvider locale="en" messages={testMessages}>
      <button type="button">Open confirmation</button>
      <ConfirmModal
        open={open}
        onOpenChange={(nextOpen) => openChanges.push(nextOpen)}
        title="Reject request"
        description="This action cannot be undone."
        confirmLabel="Reject"
        onConfirm={() => undefined}
      />
    </IntlProvider>
  );
  const view = render(modalTree(false));
  const trigger = view.getByRole("button", { name: "Open confirmation" });
  trigger.focus();
  view.rerender(modalTree(true));

  await new Promise((resolve) => window.setTimeout(resolve, 0));
  const dialog = view.getByRole("dialog", { name: "Reject request" });

  fireEvent.keyDown(dialog, { key: "Escape" });
  assert.deepEqual(openChanges, [false]);
  view.rerender(modalTree(false));
  assert.equal(view.queryByRole("dialog"), null);
  assert.equal(document.activeElement, trigger);
});

function ToastHarness() {
  const { toast } = useToast();
  return (
    <>
      <button type="button" onClick={() => toast("error", "Could not save")}>Show error</button>
      <button type="button" onClick={() => toast("success", "Saved")}>Show success</button>
    </>
  );
}

function ModalHarness({ onConfirm }: { onConfirm: (reason: string) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>Open confirmation</button>
      <ConfirmModal
        open={open}
        onOpenChange={setOpen}
        title="Reject request"
        description="This action cannot be undone."
        confirmLabel="Reject"
        requireReason
        onConfirm={onConfirm}
      />
    </>
  );
}
