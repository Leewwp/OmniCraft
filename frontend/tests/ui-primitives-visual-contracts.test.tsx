import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import React from "react";
import { Inbox } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { SkeletonCard } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { TagBadge } from "@/components/ui/TagBadge";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { ToastProvider, useToast } from "@/components/ui/Toast";

import { cleanup, fireEvent, installDom, render, renderWithIntl } from "./runtime-test-helpers";

test.afterEach(() => {
  cleanup();
});

test("approved Indigo actions use exact radii, semantic hover, and visible focus tokens", () => {
  installDom();
  const view = renderWithIntl(
    <>
      <Button>Publish</Button>
      <Button variant="outline">Cancel</Button>
      <Badge variant="destructive">Blocked</Badge>
      <TagBadge onClick={() => undefined}>Music</TagBadge>
    </>,
  );

  const publish = view.getByRole("button", { name: "Publish" });
  assert.match(publish.className, /rounded-md/);
  assert.match(publish.className, /hover:bg-accent-hover/);
  assert.match(publish.className, /focus-visible:ring-2/);
  assert.match(publish.className, /focus-visible:ring-offset-2/);
  assert.match(publish.className, /\[@media\(pointer:coarse\)\]:min-h-11/);
  assert.match(publish.className, /motion-reduce:active:translate-y-0/);
  assert.match(publish.className, /disabled:cursor-not-allowed/);

  const cancel = view.getByRole("button", { name: "Cancel" });
  assert.match(cancel.className, /hover:border-border-strong/);

  const destructiveBadge = view.getByText("Blocked");
  assert.match(destructiveBadge.className, /bg-destructive/);
  assert.match(destructiveBadge.className, /border-border-destructive/);
  assert.match(destructiveBadge.className, /text-primary-foreground/);
  assert.doesNotMatch(destructiveBadge.className, /tag-rose/);

  const tag = view.getByRole("button", { name: "Music" });
  assert.match(tag.className, /rounded-full/);
  assert.match(tag.className, /focus-visible:ring-2/);
});

test("cards and form controls consume the approved radius, elevation, border, and focus contracts", () => {
  installDom();
  const view = render(
    <>
      <Card data-interactive="true">Panel</Card>
      <Input aria-label="Title" />
      <Select aria-label="Category"><option>All</option></Select>
      <Textarea aria-label="Description" />
      <Checkbox aria-label="Accept" />
      <Switch aria-label="Published" checked={false} onCheckedChange={() => undefined} />
    </>,
  );

  const card = view.getByText("Panel");
  assert.match(card.className, /rounded-lg/);
  assert.match(card.className, /shadow-sm/);
  assert.match(card.className, /data-\[interactive=true\]:hover:border-border-strong/);
  assert.match(card.className, /data-\[interactive=true\]:hover:shadow-\[var\(--elevation-2\)\]/);
  assert.match(card.className, /data-\[interactive=true\]:focus-within:border-border-strong/);
  assert.match(card.className, /data-\[interactive=true\]:focus-within:shadow-\[var\(--elevation-2\)\]/);

  for (const control of [
    view.getByRole("textbox", { name: "Title" }),
    view.getByRole("combobox", { name: "Category" }),
    view.getByRole("textbox", { name: "Description" }),
  ]) {
    assert.match(control.className, /rounded-md/);
    assert.match(control.className, /hover:border-border-strong/);
    assert.match(control.className, /focus-visible:ring-2/);
    assert.match(control.className, /focus-visible:ring-offset-2/);
  }

  assert.match(view.getByRole("checkbox", { name: "Accept" }).className, /rounded-sm/);
  const switchControl = view.getByRole("switch", { name: "Published" });
  assert.match(switchControl.className, /focus-visible:ring-2/);
  assert.match(switchControl.querySelector("span")?.className ?? "", /shadow-sm/);
});

test("approved empty and loading states mirror content without obsolete bordered placeholders", () => {
  installDom();
  const view = render(
    <>
      <EmptyState icon={Inbox} title="Nothing here" description="Try another filter." />
      <SkeletonCard count={2} zone="original" />
    </>,
  );

  const empty = view.getByText("Nothing here").parentElement;
  assert.ok(empty);
  assert.match(empty.className, /py-20/);
  assert.match(empty.className, /md:py-24/);
  assert.doesNotMatch(empty.className, /\bborder\b/);
  const iconSurface = empty.querySelector("[data-slot='empty-state-icon']");
  assert.ok(iconSurface);
  assert.match(iconSurface.className, /size-14/);
  assert.match(iconSurface.className, /bg-accent-subtle/);

  const skeletons = document.querySelectorAll<HTMLElement>("[data-slot='skeleton-card']");
  assert.equal(skeletons.length, 2);
  for (const skeleton of skeletons) {
    assert.equal(skeleton.dataset.zone, "original");
    assert.match(skeleton.className, /rounded-lg/);
    assert.match(skeleton.className, /shadow-sm/);
    assert.match(skeleton.className, /motion-reduce:animate-none/);
    assert.match(skeleton.className, /pulse_1\.6s/);
    assert.match(skeleton.querySelector<HTMLElement>("[data-slot='skeleton']")?.className ?? "", /min-h-\[150px\]/);
  }
});

test("approved overlays use elevation three, status tokens, and responsive viewport gutters", async () => {
  installDom();
  const view = renderWithIntl(
    <ToastProvider>
      <ConfirmModal
        open
        onOpenChange={() => undefined}
        title="Delete item"
        description="This action cannot be undone."
        onConfirm={() => undefined}
      />
      <ToastHarness />
    </ToastProvider>,
  );

  const dialog = view.getByRole("dialog", { name: "Delete item" });
  assert.match(dialog.className, /rounded-lg/);
  assert.match(dialog.className, /border-border/);
  assert.match(dialog.className, /shadow-md/);
  assert.match(dialog.className, /max-w-md/);

  fireEvent.click(view.getByRole("button", { name: "Show success" }));
  const status = await view.findByRole("status");
  assert.match(status.className, /rounded-lg/);
  assert.match(status.className, /shadow-md/);
  assert.match(status.className, /bg-\[var\(--tag-green-bg\)\]/);
  assert.match(status.className, /border-\[var\(--tag-green-fg\)\]/);

  const toastSource = await readComponentSource("Toast.tsx");
  assert.match(toastSource, /left-4 right-4/);
  assert.match(toastSource, /sm:left-auto sm:max-w-sm/);
  assert.match(toastSource, /motion-reduce:translate-x-0/);
  assert.match(toastSource, /motion-reduce:transition-none/);
  assert.match(toastSource, /matchMedia\("\(prefers-reduced-motion: reduce\)"\)/);
  for (const statusColor of ["green", "rose", "orange", "blue"]) {
    assert.match(toastSource, new RegExp(`--tag-${statusColor}-bg`));
    assert.match(toastSource, new RegExp(`--tag-${statusColor}-fg`));
  }

  const dropdownSource = await readComponentSource("dropdown-menu.tsx");
  assert.match(dropdownSource, /border border-border/);
  assert.match(dropdownSource, /shadow-md/);
  assert.match(dropdownSource, /motion-reduce:data-open:animate-none/);
  assert.match(dropdownSource, /motion-reduce:data-closed:animate-none/);
  assert.doesNotMatch(dropdownSource, /slide-in-from/);
  assert.match(dropdownSource, /data-\[variant=destructive\]:focus:bg-destructive/);
  assert.doesNotMatch(dropdownSource, /data-\[variant=destructive\]:focus:bg-\[var\(--tag-rose-bg\)\]/);
  assert.doesNotMatch(dropdownSource, /ring-1 ring-foreground/);
});

test("tabs and separators use the approved navigation surface and active-state contracts", () => {
  installDom();
  const view = render(
    <>
      <Tabs defaultValue="one">
        <TabsList aria-label="Default tabs">
          <TabsTrigger value="one">One</TabsTrigger>
        </TabsList>
      </Tabs>
      <Tabs defaultValue="line-one">
        <TabsList aria-label="Line tabs" variant="line">
          <TabsTrigger value="line-one">Line one</TabsTrigger>
        </TabsList>
      </Tabs>
      <Separator />
    </>,
  );

  const defaultList = view.getByRole("tablist", { name: "Default tabs" });
  assert.match(defaultList.className, /rounded-lg/);
  assert.match(defaultList.className, /bg-canvas-subtle/);
  assert.match(view.getByRole("tab", { name: "One" }).className, /data-active:shadow-sm/);

  const lineList = view.getByRole("tablist", { name: "Line tabs" });
  assert.match(lineList.className, /bg-transparent/);
  const lineTrigger = view.getByRole("tab", { name: "Line one" });
  assert.match(lineTrigger.className, /data-active:font-semibold/);
  assert.match(lineTrigger.className, /after:h-0\.5/);
  assert.match(lineTrigger.className, /data-active:shadow-none/);

  const separator = document.querySelector<HTMLElement>("[data-slot='separator']");
  assert.ok(separator);
  assert.match(separator.className, /bg-border-default/);
  assert.match(separator.className, /data-horizontal:h-px/);
  assert.doesNotMatch(separator.className, /rounded|shadow/);
});

test("the visual authority records every U-02B primitive family before implementation", async () => {
  const spec = await readFile(new URL("../../design/ui-spec.md", import.meta.url), "utf8");
  for (const heading of [
    "Component: Button 与 Badge 共享动作原语",
    "Component: Card 共享容器原语",
    "Component: Form Controls 表单原语",
    "Component: DropdownMenu 浮层菜单原语",
    "Component: Tabs 与 Separator 导航原语",
    "Component: Loading 与反馈原语",
  ]) {
    assert.match(spec, new RegExp(heading));
  }
});

function ToastHarness() {
  const { toast } = useToast();
  return <button type="button" onClick={() => toast("success", "Saved")}>Show success</button>;
}

async function readComponentSource(fileName: string) {
  return readFile(new URL(`../components/ui/${fileName}`, import.meta.url), "utf8");
}
