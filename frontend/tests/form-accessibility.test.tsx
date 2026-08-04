import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { api } from "@/lib/api";
import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());
const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const routerPushes: string[] = [];
const authStub = {
  user: { id: 1, role: "admin", email: "admin@example.com" },
  isLoading: false,
  unreadCounts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 },
  login: async () => undefined,
  logout: async () => undefined,
  refresh: async () => true,
  refreshUser: async () => undefined,
};

Module._load = function loadWithNavigationStub(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({ ipId: "7" }),
      useRouter: () => ({ push: (value: string) => routerPushes.push(value) }),
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

const originalPost = api.post;

const messages = {
  common: {
    submit: "Submit",
    submitting: "Submitting",
    operationFailed: "Operation failed",
  },
  discussion: {
    newPost: "New discussion",
    titleLabel: "Title",
    titlePlaceholder: "Discussion title",
    titleRequired: "Title is required.",
    bodyLabel: "Content",
    bodyPlaceholder: "Write your thoughts",
  },
};

function read(relativePath: string) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

test.afterEach(() => {
  cleanup();
  api.post = originalPost;
  routerPushes.length = 0;
});

test("remaining form surfaces expose names, described errors, alerts, and mobile targets", () => {
  const tagGroups = read("app/(protected)/settings/tag-groups/page.tsx");
  const discussion = read("app/(protected)/ip/[ipId]/discussions/new/page.tsx");
  const notifications = read("app/(protected)/admin/notifications/page.tsx");
  const categories = read("app/(protected)/admin/categories/page.tsx");
  const queue = read("app/(protected)/admin/queue/page.tsx");
  const ips = read("app/(protected)/admin/ips/page.tsx");
  const feedback = read("app/(protected)/admin/feedback/page.tsx");
  const config = read("app/(protected)/admin/config/page.tsx");

  assert.match(tagGroups, /htmlFor="tag-group-name"/);
  assert.match(tagGroups, /aria-describedby=\{modalError \? "tag-group-error"/);
  assert.match(tagGroups, /aria-invalid=/);
  assert.match(tagGroups, /role="dialog"/);
  assert.match(tagGroups, /\[@media\(pointer:coarse\)\]:min-h-11 \[@media\(pointer:coarse\)\]:min-w-11/);

  assert.match(discussion, /<form/);
  assert.match(discussion, /htmlFor="discussion-title"/);
  assert.match(discussion, /aria-describedby=\{[^}]*discussion-title-error/);
  assert.match(discussion, /role="alert"/);

  assert.match(notifications, /titleInputRef/);
  assert.match(notifications, /role="alert"/);

  assert.match(categories, /htmlFor="category-name-zh"/);
  assert.match(categories, /aria-describedby=\{[^}]*category-create-error/);
  assert.match(categories, /aria-invalid=/);
  assert.match(categories, /\[@media\(pointer:coarse\)\]:min-h-11/);

  assert.match(queue, /role="alert"/);
  assert.match(ips, /aria-label=.*ip\.name/);
  assert.match(ips, /min-h-11/);

  assert.match(feedback, /htmlFor="feedback-reply"/);
  assert.match(feedback, /aria-describedby=\{replyError \|\| \(replyAttempted/);
  assert.match(feedback, /role="alert"/);

  assert.match(config, /role="switch"/);
  assert.match(config, /aria-checked=/);
  assert.match(config, /htmlFor=\{id\}/);
  assert.match(config, /aria-invalid=/);
  assert.match(config, /id="config-error"/);
});

test("discussion submission announces a missing title and focuses the title field", async () => {
  installDom();
  const view = await renderDiscussionPage();

  const title = view.getByRole("textbox", { name: messages.discussion.titleLabel });
  const body = view.getByRole("textbox", { name: messages.discussion.bodyLabel });
  assert.ok(title);
  assert.ok(body);

  fireEvent.click(view.getByRole("button", { name: messages.common.submit }));

  await waitFor(() => {
    assert.equal(title.getAttribute("aria-invalid"), "true");
    assert.equal(title.getAttribute("aria-describedby"), "discussion-title-error");
    assert.equal(document.activeElement, title);
    assert.equal(view.getByRole("alert").textContent, messages.discussion.titleRequired);
  });
});

test("tag-group and category forms expose runtime names and first-error focus", async () => {
  installDom();
  const originalGet = api.get;
  api.get = async <T,>(request: string) => {
    if (request.includes("tag-groups")) return { tag_groups: [] } as T;
    return { categories: [] } as T;
  };
  try {
    const { render } = await import("@testing-library/react");
    const TagGroupsPage = (await import("../app/(protected)/settings/tag-groups/page")).default;
    const CategoriesPage = (await import("../app/(protected)/admin/categories/page")).default;
    const tagView = render(
      <IntlProvider locale="en" messages={enMessages}>
        <TagGroupsPage />
      </IntlProvider>,
    );
    await waitFor(() => assert.ok(tagView.getByRole("button", { name: /new group/i })));
    fireEvent.click(tagView.getByRole("button", { name: /new group/i }));
    assert.ok(tagView.getByRole("dialog", { name: /new group/i }));
    assert.ok(tagView.getByRole("textbox", { name: /group name/i }));
    assert.ok(tagView.getByRole("textbox", { name: /tags/i }));
    assert.ok(tagView.getByRole("button", { name: /add tag/i }));
    fireEvent.click(tagView.getByRole("button", { name: /^save$/i }));
    await waitFor(() => {
      const name = tagView.getByRole("textbox", { name: /group name/i });
      assert.equal(document.activeElement, name);
      assert.equal(name.getAttribute("aria-invalid"), "true");
      assert.ok(tagView.getByRole("alert"));
    });
    tagView.unmount();

    const categoryView = render(
      <IntlProvider locale="en" messages={enMessages}>
        <CategoriesPage />
      </IntlProvider>,
    );
    await waitFor(() => assert.ok(categoryView.getByRole("button", { name: /new category/i })));
    fireEvent.click(categoryView.getByRole("button", { name: /new category/i }));
    assert.ok(categoryView.getByRole("textbox", { name: /chinese name/i }));
    assert.ok(categoryView.getByRole("textbox", { name: /english name/i }));
    assert.ok(categoryView.getByRole("textbox", { name: /^slug$/i }));
    fireEvent.click(categoryView.getByRole("button", { name: /^create$/i }));
    await waitFor(() => {
      const name = categoryView.getByRole("textbox", { name: /chinese name/i });
      assert.equal(document.activeElement, name);
      assert.equal(name.getAttribute("aria-invalid"), "true");
      assert.ok(categoryView.getByRole("alert"));
    });
  } finally {
    api.get = originalGet;
  }
});

test("category English edit field names the English value", async () => {
  installDom();
  const originalGet = api.get;
  api.get = async <T,>() => ({
    categories: [{
      id: 9,
      zone: "fanwork",
      level: "category",
      parent_id: null,
      name_i18n: { zh: "中文分类", en: "English category" },
      slug: "english-category",
      sort_order: 0,
      is_active: true,
    }],
  } as T);
  try {
    const { render } = await import("@testing-library/react");
    const CategoriesPage = (await import("../app/(protected)/admin/categories/page")).default;
    const view = render(
      <IntlProvider locale="en" messages={enMessages}>
        <CategoriesPage />
      </IntlProvider>,
    );
    const edit = await waitFor(() => view.getByRole("button", { name: /^edit$/i }));
    fireEvent.click(edit);
    assert.ok(view.getByRole("textbox", { name: "English Name: English category" }));
  } finally {
    api.get = originalGet;
  }
});

test("system config exposes a label for every field and switch", async () => {
  installDom();
  const originalGet = api.get;
  api.get = async <T,>() => ({ config: {} } as T);
  try {
    const { render } = await import("@testing-library/react");
    const ConfigPage = (await import("../app/(protected)/admin/config/page")).default;
    const view = render(
      <IntlProvider locale="en" messages={enMessages}>
        <ConfigPage />
      </IntlProvider>,
    );
    await waitFor(() => assert.equal(view.getAllByRole("switch").length, 3));
    const fields = view.getAllByRole("spinbutton") as HTMLInputElement[];
    assert.equal(fields.length, 19);
    for (const field of fields) assert.ok(field.labels?.length, field.id);
    for (const control of view.getAllByRole("switch")) assert.ok(control.getAttribute("aria-label"));
  } finally {
    api.get = originalGet;
  }
});

async function renderDiscussionPage() {
  const { render } = await import("@testing-library/react");
  const pageModule = await import("../app/(protected)/ip/[ipId]/discussions/new/page");
  const NewDiscussionPage = pageModule.default;
  const originalConsoleError = console.error;
  console.error = (...args: unknown[]) => {
    if (args.some((arg) => typeof arg === "object" && arg !== null && "code" in arg && arg.code === "ENVIRONMENT_FALLBACK")) {
      return;
    }
    originalConsoleError(...args);
  };
  try {
    return render(
      <IntlProvider locale="en" messages={messages}>
        <NewDiscussionPage />
      </IntlProvider>,
    );
  } finally {
    console.error = originalConsoleError;
  }
}
