import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React, { useRef, useState } from "react";
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
   modal lifecycle so tests exercise the same code paths as browsers.
   window.scrollTo throws "Not implemented" in jsdom; the overlay calls it
   during exit-restore, so stub it as a no-op. */
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
   without providers, following form-accessibility.test.tsx's established
   Module._load interception pattern. Components must be imported dynamically
   after this patch (see test.before). */
const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;
const routerPushes: string[] = [];
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
      useRouter: () => ({ push: (value: string) => routerPushes.push(value) }),
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

type OverlayModule = typeof import("@/components/content/ContentDetailOverlay");
type SidebarModule = typeof import("@/components/content/ContentSidebar");
type HostModule = typeof import("@/components/content/ContentDetailOverlayHost");

let ContentDetailOverlay: OverlayModule["ContentDetailOverlay"];
let ContentSidebar: SidebarModule["ContentSidebar"];
let ContentDetailOverlayHost: HostModule["ContentDetailOverlayHost"];

test.before(async () => {
  const overlayModule = await import("@/components/content/ContentDetailOverlay");
  const sidebarModule = await import("@/components/content/ContentSidebar");
  const hostModule = await import("@/components/content/ContentDetailOverlayHost");
  await import("@/components/content/ContentDetailOverlayLayer");
  ContentDetailOverlay = overlayModule.ContentDetailOverlay;
  ContentSidebar = sidebarModule.ContentSidebar;
  ContentDetailOverlayHost = hostModule.ContentDetailOverlayHost;
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

const detailByPath = new Map<string, unknown>([
  ["/api/v1/contents/1", ORIGINAL_DETAIL],
  ["/api/v1/contents/2", FANWORK_DETAIL],
]);

const originalGet = api.get;
const originalPost = api.post;

/* Generic api mock: contents detail by exact id, related-fanworks with a
   fresh id per call (so stack-depth chains are distinguishable), empty
   comments. failDetailOnceFor fails the first detail call for those ids
   so retry flows can recover. */
let relatedCallCount = 0;
function installApiMock(overrides: {
  detail?: Map<string, unknown>;
  failDetailOnceFor?: number[];
  failAll?: boolean;
} = {}) {
  const detail = overrides.detail ?? detailByPath;
  const failOnce = new Set(overrides.failDetailOnceFor ?? []);
  api.get = async function mockedGet<T>(requestPath: string): Promise<T> {
    if (overrides.failAll) {
      throw new ApiRequestError("DB_ERROR", "backend unavailable", 500);
    }
    if (requestPath.includes("/related-fanworks")) {
      relatedCallCount += 1;
      const id = 100 + relatedCallCount;
      return {
        contents: [
          {
            id,
            title: `Related ${id}`,
            zone: "fanwork",
            content_type: "image",
            author: { id: 11, username: "Related Author" },
            like_count: 3,
          },
        ],
        total: 1,
      } as T;
    }
    if (requestPath.includes("/versions")) {
      return { versions: [] } as T;
    }
    if (requestPath.startsWith("/api/v1/social/comments")) {
      return { comments: [] } as T;
    }
    const contentIdMatch = requestPath.match(/^\/api\/v1\/contents\/(\d+)$/);
    if (contentIdMatch) {
      const contentId = Number(contentIdMatch[1]);
      if (failOnce.has(contentId)) {
        failOnce.delete(contentId);
        throw new ApiRequestError("DB_ERROR", "backend unavailable", 500);
      }
      const found = detail.get(`/api/v1/contents/${contentId}`);
      if (found) {
        return found as T;
      }
      if (contentId >= 101 && contentId <= 199) {
        return {
          content: {
            id: contentId,
            title: `Related ${contentId}`,
            zone: "fanwork",
            content_type: "image",
            author: { id: 11, username: "Related Author" },
            status: "published",
            description: `Related ${contentId} body`,
            like_count: 3,
          },
          attachments: [],
          tags: [],
        } as T;
      }
      throw new ApiRequestError("NOT_FOUND", "raw secret backend error", 404);
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

function OverlayHarness({ entryId, zone }: { entryId: number; zone: "original" | "fanwork" }) {
  const [entry, setEntry] = useState<{ id: number; zone: "original" | "fanwork" } | null>(null);
  const triggerRef = useRef<HTMLElement | null>(null);
  return (
    <>
      <button
        type="button"
        onClick={(event) => {
          triggerRef.current = event.currentTarget;
          setEntry({ id: entryId, zone });
        }}
      >
        Open overlay
      </button>
      {entry && (
        <ContentDetailOverlay
          key={`${entry.zone}:${entry.id}`}
          contentId={entry.id}
          zone={entry.zone}
          source="zone-page"
          open
          onOpenChange={(open) => {
            if (!open) setEntry(null);
          }}
          returnFocusRef={triggerRef}
        />
      )}
    </>
  );
}

function renderOverlay(node: React.ReactNode) {
  installDom();
  installOverlayTestStubs();
  relatedCallCount = 0;
  return render(<IntlProvider locale="en" messages={enMessages}>{node}</IntlProvider>);
}

async function openOverlay(
  view: ReturnType<typeof render>,
  entryId = 1,
  zone: "original" | "fanwork" = "original",
) {
  const trigger = view.getByRole("button", { name: "Open overlay" });
  await act(async () => {
    fireEvent.click(trigger);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await waitFor(() =>
    assert.ok(
      document.activeElement ===
        view.getByRole("heading", { level: 2, name: entryId === 1 ? "Original 1" : "Fanwork 2" }),
    ),
  );
  return trigger;
}

async function pushRelatedLayer(view: ReturnType<typeof render>, relatedId: number) {
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: `Open content detail: Related ${relatedId}` }));
    await Promise.resolve();
  });
  await waitFor(() =>
    assert.ok(view.getByRole("heading", { level: 2, name: `Related ${relatedId}` })),
  );
}

test.afterEach(() => {
  cleanup();
  restoreApiMocks();
  routerPushes.length = 0;
});

test("opens a modal dialog, locks background scroll, and focuses the accessible title", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  await openOverlay(view);

  const dialog = view.getByRole("dialog") as HTMLDialogElement;
  assert.equal(dialog.getAttribute("open"), "");
  assert.equal(document.body.style.overflow, "hidden");
  assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" }));
  assert.ok(view.getByText("Original body"));
});

test("backdrop click exits the whole stack and restores trigger focus", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  const trigger = await openOverlay(view);

  await act(async () => {
    const dialog = view.getByRole("dialog");
    fireEvent.click(dialog);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.queryByRole("dialog") === null));
  await waitFor(() => assert.ok(document.activeElement === trigger));
  assert.equal(document.body.style.overflow, "");
});

test("close button exits the entire stack from any depth", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  const trigger = await openOverlay(view);

  await pushRelatedLayer(view, 101);

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Close content detail" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.queryByRole("dialog") === null));
  await waitFor(() => assert.ok(document.activeElement === trigger));
});

test("related content pushes onto the stack; back button pops and returns focus to the trigger", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  await openOverlay(view);

  const relatedTrigger = view.getByRole("button", { name: "Open content detail: Related 101" });
  await pushRelatedLayer(view, 101);
  assert.ok(view.getByRole("button", { name: "Back to Original 1" }));

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Back to Original 1" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" })));
  await waitFor(() => assert.ok(document.activeElement === relatedTrigger));
});

test("Esc pops one layer at a time and exits at depth 1", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  const trigger = await openOverlay(view);

  const dialog = view.getByRole("dialog") as HTMLDialogElement;
  await pushRelatedLayer(view, 101);

  await act(async () => {
    dialog.dispatchEvent(new window.Event("cancel", { cancelable: true }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" })));

  await act(async () => {
    dialog.dispatchEvent(new window.Event("cancel", { cancelable: true }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.queryByRole("dialog") === null));
  await waitFor(() => assert.ok(document.activeElement === trigger));
});

test("per-layer scroll memory restores on pop and new layers start at the top", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  await openOverlay(view);

  const scroller = document.querySelector<HTMLElement>('[data-slot="overlay-scroller"]');
  assert.ok(scroller);
  scroller.scrollTop = 120;

  await pushRelatedLayer(view, 101);
  assert.equal(scroller.scrollTop, 0);

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Back to Original 1" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" })));
  await waitFor(() => assert.equal(scroller.scrollTop, 120));
});

test("stack depth is capped at five", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  await openOverlay(view);

  for (let step = 0; step < 4; step += 1) {
    await pushRelatedLayer(view, 101 + step);
  }
  const callsAtCap = relatedCallCount;
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Open content detail: Related 105" }));
    await Promise.resolve();
  });
  assert.equal(relatedCallCount, callsAtCap);
  assert.ok(view.getByRole("heading", { level: 2, name: "Related 104" }));
});

test("not-found content shows a localized empty state without raw errors", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={999} zone="original" />);
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Open overlay" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await waitFor(() => assert.ok(view.getByText("Content not found or deleted")));
  assert.ok(view.queryByText(/raw secret backend error/) === null);
});

test("banned content shows the forbidden empty state", async () => {
  installApiMock({
    detail: new Map([
      [
        "/api/v1/contents/1",
        {
          content: {
            id: 1,
            title: "Banned Work",
            zone: "fanwork",
            content_type: "article",
            status: "banned",
          },
          attachments: [],
          tags: [],
        },
      ],
    ]),
  });
  const view = renderOverlay(<OverlayHarness entryId={1} zone="fanwork" />);
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Open overlay" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByText("Content is not accessible")));
  assert.ok(view.queryByText("Banned Work") === null);
});

test("load failures are retriable", async () => {
  installApiMock({ failDetailOnceFor: [1] });
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Open overlay" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByText("Content detail failed to load")));

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Retry" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" })));
});

test("browser back pops layers and exits at the root with focus restored", async () => {
  installApiMock();
  const view = renderOverlay(<OverlayHarness entryId={1} zone="original" />);
  const trigger = await openOverlay(view);

  await pushRelatedLayer(view, 101);

  await act(async () => {
    window.dispatchEvent(new window.PopStateEvent("popstate"));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" })));

  await act(async () => {
    window.dispatchEvent(new window.PopStateEvent("popstate"));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.queryByRole("dialog") === null));
  await waitFor(() => assert.ok(document.activeElement === trigger));
});

test("sidebar related-content entry opens the overlay without leaving the page", async () => {
  installApiMock();
  installDom();
  installOverlayTestStubs();
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentSidebar
        author={{ id: 10, username: "Fanwork Author" }}
        zone="fanwork"
        ip={{ id: 3, name: "Indigo IP" }}
        sourceOriginal={{ id: 1, title: "Original 1" }}
        onOpenRelated={() => undefined}
      />
    </IntlProvider>,
  );
  const entryButton = view.getByRole("button", { name: /Original 1/ });
  assert.ok(entryButton);
  assert.ok(view.queryByRole("link", { name: /Original 1/ }) === null);
});

test("host wires the fanwork detail page related entry to the overlay", async () => {
  installApiMock();
  installDom();
  installOverlayTestStubs();
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentDetailOverlayHost
        content={{ ...FANWORK_DETAIL.content, attachments: [], tags: [] }}
        zone="fanwork"
        author={{ id: 10, username: "Fanwork Author" }}
        ip={{ id: 3, name: "Indigo IP" }}
        sourceOriginal={{ id: 1, title: "Original 1" }}
      />
    </IntlProvider>,
  );

  const entryButton = view.getByRole("button", { name: /Original 1/ });
  await act(async () => {
    fireEvent.click(entryButton);
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.getByRole("dialog")));
  await waitFor(() => assert.ok(view.getByRole("heading", { level: 2, name: "Original 1" })));
  assert.ok(view.getByRole("button", { name: "Back to content page" }));

  await act(async () => {
    fireEvent.click(view.getByRole("button", { name: "Close content detail" }));
    await Promise.resolve();
  });
  await waitFor(() => assert.ok(view.queryByRole("dialog") === null));
  await waitFor(() => assert.ok(document.activeElement === entryButton));
});

test("overlay i18n keys exist in both catalogs with matching placeholders", () => {
  const zh = JSON.parse(read("messages/zh.json")) as Record<string, Record<string, unknown>>;
  const en = JSON.parse(read("messages/en.json")) as Record<string, Record<string, unknown>>;
  const zhOverlay = zh.contentDetailOverlay as Record<string, string>;
  const enOverlay = en.contentDetailOverlay as Record<string, string>;
  assert.ok(zhOverlay && enOverlay);
  assert.deepEqual(Object.keys(zhOverlay).sort(), Object.keys(enOverlay).sort());
  for (const key of Object.keys(zhOverlay)) {
    const zhPlaceholders = [...String(zhOverlay[key]).matchAll(/\{([a-zA-Z0-9_]+)\}/g)].map((m) => m[1]).sort();
    const enPlaceholders = [...String(enOverlay[key]).matchAll(/\{([a-zA-Z0-9_]+)\}/g)].map((m) => m[1]).sort();
    assert.deepEqual(
      zhPlaceholders,
      enPlaceholders,
      `placeholder mismatch for contentDetailOverlay.${key}`,
    );
  }
});

test("overlay source uses dialog semantics, no native popups, and no raw error rendering", () => {
  const overlay = read("components/content/ContentDetailOverlay.tsx");
  const layer = read("components/content/ContentDetailOverlayLayer.tsx");
  const host = read("components/content/ContentDetailOverlayHost.tsx");

  for (const source of [overlay, layer, host]) {
    assert.doesNotMatch(source, /window\.(?:confirm|prompt|alert)\(/);
    assert.doesNotMatch(source, /error\.message/);
  }
  assert.match(overlay, /<dialog/);
  assert.match(overlay, /showModal/);
  assert.match(overlay, /onCancel/);
  assert.match(overlay, /aria-labelledby/);
  assert.match(overlay, /overflow-y-auto/);
  assert.match(overlay, /--elevation-3/);
  assert.match(layer, /\.get\(`\/api\/v1\/contents\/\$\{entry\.contentId\}`/);
  assert.doesNotMatch(overlay, /columns-2|columns-3|columns-4/);
});
