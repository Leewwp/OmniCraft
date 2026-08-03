import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import React, { useState } from "react";

import { ConfirmModal } from "@/components/ui/confirm-modal";
import { act, cleanup, fireEvent, installDom, renderWithIntl } from "./runtime-test-helpers";

const auditedSources = [
  "../app/(protected)/dashboard/contents/page.tsx",
  "../app/(protected)/dashboard/contributors/page.tsx",
  "../app/(protected)/dashboard/pr-requests/page.tsx",
  "../app/(protected)/settings/page.tsx",
  "../app/(protected)/admin/agent-config/page.tsx",
  "../components/content/VersionHistory.tsx",
  "../components/social/ReactionBar.tsx",
] as const;

test.afterEach(() => cleanup());

test("all U-04/U-11 surfaces use ConfirmModal and no production source owns a native dialog", async () => {
  for (const relativePath of auditedSources) {
    const source = await readFile(new URL(relativePath, import.meta.url), "utf8");
    assert.doesNotMatch(source, /window\.(confirm|prompt)\(/);
    if (!relativePath.endsWith("settings/page.tsx")) {
      assert.match(source, /ConfirmModal/);
    } else {
      assert.match(source, /deleteIrreversible/);
      assert.match(source, /deleteOpen/);
    }
  }
});

test("ConfirmModal keeps destructive actions open after an API failure and allows retry", async () => {
  installDom();
  const view = renderWithIntl(<FailureRetryHarness />);

  fireEvent.click(view.getByRole("button", { name: "Open destructive action" }));
  const confirm = view.getByRole("button", { name: "Delete" });
  await act(async () => {
    fireEvent.click(confirm);
    await Promise.resolve();
  });

  assert.equal(view.getByTestId("attempt-count").textContent, "1");
  assert.ok(view.getByRole("dialog", { name: "Delete item" }));
  assert.equal((confirm as HTMLButtonElement).disabled, false);

  await act(async () => {
    fireEvent.click(confirm);
    await Promise.resolve();
  });
  assert.equal(view.getByTestId("attempt-count").textContent, "2");
  assert.equal(view.queryByRole("dialog"), null);
});

function FailureRetryHarness() {
  const [open, setOpen] = useState(false);
  const [, forceRender] = React.useReducer((value) => value + 1, 0);
  const attempts = React.useRef(0);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>Open destructive action</button>
      <span data-testid="attempt-count">{attempts.current}</span>
      <ConfirmModal
        open={open}
        onOpenChange={setOpen}
        title="Delete item"
        description="This cannot be undone."
        confirmLabel="Delete"
        onConfirm={async () => {
          attempts.current += 1;
          forceRender();
          if (attempts.current === 1) {
            throw new Error("simulated API failure");
          }
        }}
      />
    </>
  );
}
