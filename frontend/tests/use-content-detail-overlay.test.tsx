import test from "node:test";
import assert from "node:assert/strict";
import React, { useRef, useState } from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

/* Native <dialog> showModal/close are not implemented in jsdom; stub the
   modal lifecycle so tests exercise the same code paths as browsers. */
function installOverlayTestStubs() {
  const prototype = window.HTMLDialogElement?.prototype as unknown as HTMLDialogElement | undefined;
  if (!prototype) return;
  prototype.showModal = function showModalStub(this: HTMLDialogElement) {
    this.setAttribute("open", "");
  };
  prototype.close = function closeStub(this: HTMLDialogElement) {
    this.removeAttribute("open");
  };
  window.scrollTo = () => undefined;
}

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const authStub = {
  user: null,
  isLoading: false,
  unreadCounts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 },
  capabilities: { can_interact: false, interaction_denial_reason: "unavailable" },
  login: async () => undefined,
  logout: async () => undefined,
  refresh: async () => true,
  refreshUser: async () => undefined,
};

Module._load = function loadWithNavigationStub(request, parent, isMain) {
  if (request === "next/navigation") {
    return {
      useParams: () => ({}),
      useRouter: () => ({ push: () => undefined }),
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
      interactionDenialKey: () => "capabilities.deniedUnknown",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type HookModule = typeof import("@/components/content/use-content-detail-overlay");
let useContentDetailOverlay: HookModule["useContentDetailOverlay"];

test.before(async () => {
  const module = await import("@/components/content/use-content-detail-overlay");
  await import("@/components/content/ContentDetailOverlayLayer");
  useContentDetailOverlay = module.useContentDetailOverlay;
});

const CONTENT_DETAIL = {
  content: {
    id: 1,
    title: "Shared entry work",
    zone: "original",
    content_type: "article",
    author: { id: 9, username: "Entry Author" },
    status: "published",
    description: "Entry body",
    like_count: 7,
  },
  attachments: [],
  tags: [],
};

const originalGet = api.get;
const originalPost = api.post;

function installApiMock() {
  api.get = async function mockedGet<T>(requestPath: string): Promise<T> {
    if (requestPath.includes("/related-fanworks")) {
      return { contents: [], total: 0 } as T;
    }
    if (requestPath.includes("/versions")) {
      return { versions: [] } as T;
    }
    if (requestPath.startsWith("/api/v1/social/comments")) {
      return { comments: [] } as T;
    }
    if (requestPath === "/api/v1/contents/1") {
      return CONTENT_DETAIL as T;
    }
    throw new ApiRequestError("NOT_FOUND", "raw secret backend error", 404);
  };
  api.post = async function mockedPost<T>(): Promise<T> {
    throw new ApiRequestError("UNAUTHORIZED", "no session", 401);
  };
}

function restoreApiMocks() {
  api.get = originalGet;
  api.post = originalPost;
}

function HookHarness({
  source,
}: {
  source: "recommendation" | "zone-page" | "ip-page" | "agent-citation";
}) {
  const { open, overlayElement } = useContentDetailOverlay({ source });
  return (
    <>
      <button
        type="button"
        onClick={(event) => open({ contentId: 1, zone: "original" }, event.currentTarget)}
      >
        Open from {source}
      </button>
      {overlayElement}
    </>
  );
}

function renderHarness(source: React.ComponentProps<typeof HookHarness>["source"]) {
  installDom();
  installOverlayTestStubs();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <HookHarness source={source} />
    </IntlProvider>,
  );
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

test("the shared entry controller opens the overlay and restores trigger focus", async () => {
  installApiMock();
  const view = renderHarness("recommendation");
  const trigger = view.getByRole("button", { name: "Open from recommendation" });
  await act(async () => {
    fireEvent.click(trigger);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await waitFor(() =>
    assert.ok(view.getByRole("heading", { level: 2, name: "Shared entry work" })),
  );
  assert.ok(view.getByRole("button", { name: "Back to recommendations" }));

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Close content detail" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.queryByRole("dialog") === null));
  await waitFor(() => assert.ok(document.activeElement === trigger));
});

test("entries keep only the source-param difference in the return label", async () => {
  installApiMock();
  const cases: Array<[React.ComponentProps<typeof HookHarness>["source"], string]> = [
    ["recommendation", "Back to recommendations"],
    ["zone-page", "Back to content page"],
    ["ip-page", "Back to IP detail"],
    ["agent-citation", "Back to conversation"],
  ];
  for (const [source, label] of cases) {
    const view = renderHarness(source);
    await act(async () => {
      fireEvent.click(view.getByRole("button", { name: `Open from ${source}` }));
      await Promise.resolve();
    });
    await waitFor(() => assert.ok(view.getByRole("dialog")));
    assert.ok(view.getByRole("button", { name: label }));
    cleanup();
  }
});

test("the shared entry controller does not copy open/close state machines into entries", () => {
  const fs = requireForMocks("node:fs") as typeof import("node:fs");
  const hook = fs.readFileSync(
    requireForMocks("node:path").join(process.cwd(), "components/content/use-content-detail-overlay.tsx"),
    "utf8",
  );
  assert.match(hook, /ContentDetailOverlay/);
  assert.match(hook, /onOpenChange/);
  assert.match(hook, /returnFocusRef/);

  for (const file of [
    "components/recommend/RecommendFeedClient.tsx",
    "components/content/OverlayMasonryGrid.tsx",
    "components/agent/AgentWorkspace.tsx",
    "components/content/ContentDetailOverlayHost.tsx",
  ]) {
    const source = fs.readFileSync(requireForMocks("node:path").join(process.cwd(), file), "utf8");
    assert.match(source, /useContentDetailOverlay/);
    assert.doesNotMatch(source, /pushHistoryState|popstate|showModal|finalizeClose/);
    assert.doesNotMatch(source, /const \[overlayEntry|setOverlayEntry/);
    assert.doesNotMatch(source, /overlayTriggerRef = useRef/);
  }
});
