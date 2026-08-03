import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import React from "react";

import {
  ADMIN_SIDEBAR_STORAGE_KEY,
  PUBLIC_SIDEBAR_STORAGE_KEY,
  STUDIO_SIDEBAR_STORAGE_KEY,
  useSidebarCollapse,
} from "@/lib/use-sidebar-collapse";

import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

test.afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

test("sidebar collapse state uses distinct persistence keys and native keyboard buttons", async () => {
  installDom();
  window.localStorage.setItem(PUBLIC_SIDEBAR_STORAGE_KEY, "true");
  window.localStorage.setItem(STUDIO_SIDEBAR_STORAGE_KEY, "false");

  const view = render(<CollapseHarness />);

  await waitFor(() => {
    assert.equal(view.getByTestId("public-state").textContent, "collapsed");
    assert.equal(view.getByTestId("studio-state").textContent, "expanded");
  });

  const publicToggle = view.getByRole("button", { name: "toggle public" });
  const studioToggle = view.getByRole("button", { name: "toggle studio" });
  assert.equal(publicToggle.tagName, "BUTTON");
  assert.equal(publicToggle.getAttribute("type"), "button");
  publicToggle.focus();
  assert.equal(document.activeElement, publicToggle);
  fireEvent.click(publicToggle);
  fireEvent.click(studioToggle);

  assert.equal(window.localStorage.getItem(PUBLIC_SIDEBAR_STORAGE_KEY), "false");
  assert.equal(window.localStorage.getItem(STUDIO_SIDEBAR_STORAGE_KEY), "true");
  assert.notEqual(PUBLIC_SIDEBAR_STORAGE_KEY, STUDIO_SIDEBAR_STORAGE_KEY);
  assert.notEqual(PUBLIC_SIDEBAR_STORAGE_KEY, ADMIN_SIDEBAR_STORAGE_KEY);
  assert.notEqual(STUDIO_SIDEBAR_STORAGE_KEY, ADMIN_SIDEBAR_STORAGE_KEY);
});

test("navigation shells share approved geometry while keeping separate information architectures", async () => {
  const [header, sidebar, studio, admin] = await Promise.all([
    readFrontendSource("components/layout/Header.tsx"),
    readFrontendSource("components/layout/Sidebar.tsx"),
    readFrontendSource("components/studio/StudioSidebar.tsx"),
    readFrontendSource("app/(protected)/admin/layout.tsx"),
  ]);

  assert.match(header, /h-\[var\(--header-h\)\]/);
  assert.match(header, /border-border-default/);
  assert.match(header, /bg-canvas-default/);
  assert.match(header, /w-\[85vw\]/);
  assert.match(header, /bg-black\/50/);
  assert.match(header, /aria-modal="true"/);
  assert.match(header, /aria-expanded=\{mobileMenuOpen\}/);
  assert.match(header, /z-\[60\]/);
  assert.match(header, /event\.key === "Escape"/);
  assert.match(header, /min-\[701px\]:hidden/);

  for (const shell of [sidebar, studio, admin]) {
    assert.match(shell, /w-\[228px\]/);
    assert.match(shell, /w-12/);
    assert.match(shell, /useSidebarCollapse/);
    assert.match(shell, /aria-label=/);
  }

  assert.match(sidebar, /PUBLIC_SIDEBAR_STORAGE_KEY/);
  assert.match(sidebar, /aria-label=\{t\("nav\.siteName"\)\}/);
  assert.doesNotMatch(sidebar, /aria-label=\{t\("studio\.sidebar\.analytics"\)\}/);
  assert.match(studio, /STUDIO_SIDEBAR_STORAGE_KEY/);
  assert.match(studio, /delay-300/);
  assert.match(studio, /left-full/);
  assert.match(studio, /overflow-visible/);
  assert.doesNotMatch(studio, /overflow-x-hidden/);
  assert.match(studio, /w-\[3px\]/);
  assert.match(studio, /gi > 0 && collapsed/);
  assert.match(studio, /event\.key === "Escape"/);
  assert.match(studio, /min-\[701px\]:flex/);
  assert.match(admin, /ADMIN_SIDEBAR_STORAGE_KEY/);
  assert.doesNotMatch(admin, /w-\[220px\]|w-\[52px\]/);
  assert.doesNotMatch(admin, /overflow-x-auto border-b/);
  assert.match(admin, /w-\[85vw\]/);
  assert.match(studio, /w-\[85vw\]/);
  assert.match(admin, /event\.key === "Escape"/);
  assert.match(admin, /min-\[701px\]:block/);
});

function CollapseHarness() {
  const publicSidebar = useSidebarCollapse({ storageKey: PUBLIC_SIDEBAR_STORAGE_KEY });
  const studioSidebar = useSidebarCollapse({ storageKey: STUDIO_SIDEBAR_STORAGE_KEY });

  return (
    <>
      <span data-testid="public-state">{publicSidebar.collapsed ? "collapsed" : "expanded"}</span>
      <button type="button" aria-label="toggle public" onClick={publicSidebar.toggle} />
      <span data-testid="studio-state">{studioSidebar.collapsed ? "collapsed" : "expanded"}</span>
      <button type="button" aria-label="toggle studio" onClick={studioSidebar.toggle} />
    </>
  );
}

async function readFrontendSource(relativePath: string) {
  return readFile(new URL(`../${relativePath}`, import.meta.url), "utf8");
}
