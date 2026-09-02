import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { api, ApiRequestError } from "@/lib/api";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

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

/* Stub next/navigation + AuthContext so ContentDetail/FollowButton render
   without providers (same interception pattern as content-detail-overlay.test.tsx). */
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
      interactionDenialKey: (reason?: string) => {
        switch (reason) {
          case "user_banned":
            return "capabilities.deniedBanned";
          case "email_not_verified":
            return "capabilities.deniedEmailNotVerified";
          case "insufficient_reputation":
            return "capabilities.deniedInsufficientReputation";
          case "config_error":
          case "unavailable":
            return "capabilities.deniedUnavailable";
          default:
            return "capabilities.deniedUnknown";
        }
      },
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

type OverlayMasonryGridModule = typeof import("@/components/content/OverlayMasonryGrid");
let OverlayMasonryGrid: OverlayMasonryGridModule["OverlayMasonryGrid"];

test.before(async () => {
  const module = await import("@/components/content/OverlayMasonryGrid");
  await import("@/components/content/ContentDetailOverlayLayer");
  OverlayMasonryGrid = module.OverlayMasonryGrid;
});

const ORIGINAL_DETAIL = {
  content: {
    id: 1,
    title: "Original 1",
    zone: "original",
    content_type: "article",
    author: { id: 9, username: "Original Author" },
    status: "published",
    description: "Original body",
    like_count: 7,
  },
  attachments: [],
  tags: [],
};

const FANWORK_DETAIL = {
  content: {
    id: 2,
    title: "Fanwork 2",
    zone: "fanwork",
    content_type: "article",
    author: { id: 10, username: "Fanwork Author" },
    ip: { id: 3, name: "Indigo IP" },
    status: "published",
    description: "Fanwork body",
    like_count: 5,
  },
  attachments: [],
  tags: [],
  source_original: { id: 1, title: "Original 1" },
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
      return ORIGINAL_DETAIL as T;
    }
    if (requestPath === "/api/v1/contents/2") {
      return FANWORK_DETAIL as T;
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

function cardData(id: number, zone: "original" | "fanwork") {
  return {
    id,
    title: `Feed item ${id}`,
    zone,
    author: { id: 10, username: `author-${id}` },
    like_count: 3,
    ...(zone === "fanwork" ? { content_type: "image", ip: { name: "Indigo IP" }, comment_count: 1, tags: ["art"] } : {}),
  };
}

function renderGrid(props: {
  source: "zone-page" | "ip-page";
  items?: ReturnType<typeof cardData>[];
}) {
  installDom();
  installOverlayTestStubs();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <OverlayMasonryGrid
        items={props.items ?? [cardData(1, "original")]}
        emptyText="No content yet"
        source={props.source}
      />
    </IntlProvider>,
  );
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
});

/* ---------- OverlayMasonryGrid 共享浮窗接线 ---------- */

test("card entries render as overlay buttons, not detail links", () => {
  installApiMock();
  const view = renderGrid({ source: "zone-page", items: [cardData(1, "original"), cardData(2, "fanwork")] });
  const main = view.getByRole("button", { name: "Feed item 1" });
  assert.equal(main.tagName, "BUTTON");
  assert.equal(main.getAttribute("href"), null);
  assert.ok(view.getByRole("button", { name: "Feed item 2" }));
});

test("card click opens the shared ContentDetailOverlay without leaving the page", async () => {
  installApiMock();
  const view = renderGrid({ source: "zone-page" });
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Feed item 1" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" })));
  assert.ok(view.getByRole("button", { name: "Back to content page" }));
});

test("IP page surfaces label the overlay return as IP detail", async () => {
  installApiMock();
  const view = renderGrid({ source: "ip-page", items: [cardData(2, "fanwork")] });
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Feed item 2" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Fanwork 2" })));
  assert.ok(view.getByRole("button", { name: "Back to IP detail" }));
});

test("exiting the overlay restores focus to the triggering card", async () => {
  installApiMock();
  const view = renderGrid({ source: "zone-page" });
  const trigger = view.getByRole("button", { name: "Feed item 1" });
  await act(async () => {
    fireEvent.click(trigger);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Close content detail" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.queryByRole("dialog") === null));
  await waitFor(() => assert.ok(document.activeElement === trigger));
});

/* ---------- 页面接线源码契约（二创 / 原创 / IP 详情 / IP 类目） ---------- */

test("home feed and original zone wire the shared overlay with zone-page source", () => {
  const home = read("components/home/HomePageClient.tsx");
  const originalFeed = read("components/original/OriginalFeedClient.tsx");
  for (const source of [home, originalFeed]) {
    assert.match(source, /<OverlayMasonryGrid/);
    assert.match(source, /source="zone-page"/);
    assert.doesNotMatch(source, /<MasonryGrid/, "pages must not use the raw grid without overlay wiring");
  }
  assert.match(home, /items=\{contents\}/);
  assert.match(originalFeed, /emptyText=\{t\("home\.noOriginalContent"\)\}/);
});

test("IP detail surfaces wire the shared overlay with ip-page source", () => {
  const ipDetailContents = read("components/ip/IPDetailContents.tsx");
  assert.match(ipDetailContents, /<OverlayMasonryGrid/);
  assert.match(ipDetailContents, /source="ip-page"/);
  assert.doesNotMatch(ipDetailContents, /<MasonryGrid/, "pages must not use the raw grid without overlay wiring");
  assert.match(ipDetailContents, /items=\{contents\}/);

  // The legacy /ip/[id]/[category] route now redirects to the query form (U-03).
  const legacyCategoryRoute = read("app/(public)/ip/[ipId]/[category]/page.tsx");
  assert.match(legacyCategoryRoute, /redirect\(/);
});

test("direct-URL detail pages keep full-page rendering (deep links preserved)", () => {
  const fanworkDetail = read("app/(public)/content/[contentId]/page.tsx");
  const originalDetail = read("app/(public)/original/[contentId]/page.tsx");
  assert.match(fanworkDetail, /ContentDetailOverlayHost/);
  assert.match(originalDetail, /ContentDetail/);
});
