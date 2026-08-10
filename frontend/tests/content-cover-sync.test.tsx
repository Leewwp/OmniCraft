import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { act, cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

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

Module._load = function loadWithStubs(request, parent, isMain) {
  if (request === "next/image") {
    /* jsdom + real next/image + a later test error hangs the node:test runner
       (reproduced in dbg8/dbg11); stub the optimizer wrapper so the cover-sync
       logic under test (onLoad/onError, body gating, placeholder, frame) is
       exercised on a plain <img>. Real-browser coverage lives in e2e. */
    return (props: Record<string, unknown>) =>
      React.createElement("img", { ...props, fill: undefined, sizes: undefined });
  }
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

type ContentDetailModule = typeof import("@/components/content/ContentDetail");
let ContentDetail: ContentDetailModule["ContentDetail"];

test.before(async () => {
  const module = await import("@/components/content/ContentDetail");
  ContentDetail = module.ContentDetail;
});

const DETAIL = {
  id: 1,
  title: "Cover synced work",
  zone: "fanwork",
  content_type: "image",
  author: { id: 9, username: "Cover Author" },
  ip: { id: 3, name: "Indigo IP" },
  status: "published",
  description: "Body must wait for the cover",
  like_count: 2,
};

function renderDetail(
  data: Partial<typeof DETAIL> & { cover_image_url?: string } = {},
  coverSync?: boolean,
) {
  installDom();
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <ContentDetail data={{ ...DETAIL, ...data }} coverSync={coverSync} />
    </IntlProvider>,
  );
}

function bodyHidden() {
  const wrapper = document.querySelector('[data-slot="detail-body"]');
  return wrapper !== null && wrapper.classList.contains("invisible");
}

test.afterEach(() => cleanup());

test("without coverSync the standalone detail shows the body immediately", () => {
  const view = renderDetail({ cover_image_url: "/cover.png" });
  assert.equal(bodyHidden(), false);
});

test("coverSync hides the body until the cover image loads", async () => {
  const view = renderDetail({ cover_image_url: "/cover.png" }, true);
  assert.equal(bodyHidden(), true, "body must stay hidden while the cover loads");
  const cover = document.querySelector('[data-slot="detail-cover"]');
  assert.ok(cover, "cover slot must exist in the final cover geometry");

  const img = document.querySelector('img[alt="Cover synced work"]');
  assert.ok(img, "cover image must render inside the slot");
  await act(async () => {
    fireEvent.load(img!);
    await Promise.resolve();
  });
  await waitFor(() => assert.equal(bodyHidden(), false));
});

test("coverSync reveals usable detail with a stable placeholder when the cover fails", async () => {
  const view = renderDetail({ cover_image_url: "/broken.png" }, true);
  const img = document.querySelector('img[alt="Cover synced work"]');
  assert.ok(img, "cover image must render before the failure event");
  await act(async () => {
    fireEvent.error(img!);
    await Promise.resolve();
  });
  await waitFor(() => assert.equal(bodyHidden(), false));
  assert.ok(document.querySelector('img[alt="Cover synced work"]') === null, "failed image must be removed");
  assert.ok(view.queryByText("Image"), "type placeholder stays visible");
});

test("coverSync without a cover URL settles immediately", () => {
  const view = renderDetail({}, true);
  assert.equal(bodyHidden(), false);
});

test("the cover container keeps its full-width horizontal frame under a height cap", () => {
  const view = renderDetail({}, true);
  const frame = document.querySelector('[data-slot="detail-cover"]');
  assert.ok(frame);
  assert.match(frame.className, /w-full/);
  assert.doesNotMatch(frame.className, /max-h/);
  const inner = frame.firstElementChild;
  assert.ok(inner && /max-h-96/.test(inner.className), "height cap must live on an inner frame");
  assert.match(inner.className, /aspect/);
  assert.match(inner.className, /w-full/);
});
