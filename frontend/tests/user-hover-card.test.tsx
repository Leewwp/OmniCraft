import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { AuthProvider } from "@/contexts/AuthContext";
import { api, setAccessToken } from "@/lib/api";
import { saveTokens } from "@/lib/auth";
import { UserHoverCard } from "@/components/social/UserHoverCard";
import {
  cleanup,
  fireEvent,
  installDom,
  render,
  waitFor,
} from "./runtime-test-helpers";

const { within } = require("@testing-library/react") as typeof import("@testing-library/react");

// SP-17/T3 (#492)：UserHoverCard——P-01 UserIdentity 生产版。
// 覆盖：触发元链接 / 悬停 200ms 开卡（SWR 取数）/ Esc 关闭归还焦点 /
// 自视角不显示关注私信（查看主页）/ 无 userId 退化纯展示。

{
  const globalStore = globalThis as unknown as { localStorage?: Storage; window: typeof window };
  if (!globalStore.localStorage) {
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      value: globalStore.window.localStorage,
    });
  }
}

const PROFILE_USER_ID = 9206;

function profileResponse(overrides: Partial<{ bio: string; followers_count: number; is_following: boolean }> = {}) {
  return {
    user: {
      id: PROFILE_USER_ID,
      username: "sp16a_author",
      avatar_url: "",
      bio: "seed author bio",
      followers_count: 3,
      is_following: false,
      stats: { contents_count: 12, likes_received: 34 },
      ...overrides,
    },
  };
}

function validAccessToken() {
  const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64url");
  return `header.${payload}.signature`;
}

/** 钉 matchMedia（jsdom 无实现，组件对缺省按触屏处理）。 */
function installHoverMatchMedia(matches: boolean) {
  const original = window.matchMedia;
  window.matchMedia = ((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addListener() {},
    removeListener() {},
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
  return () => {
    window.matchMedia = original;
  };
}

let usersResponse = profileResponse();

function stubApiGet(extra?: (path: string) => unknown) {
  const originalGet = api.get;
  api.get = (async <T,>(path: string): Promise<T> => {
    if (path === `/api/v1/users/${PROFILE_USER_ID}`) {
      return usersResponse as T;
    }
    if (extra) {
      const hit = extra(path);
      if (hit !== undefined) return hit as T;
    }
    return {} as T;
  }) as typeof api.get;
  return () => {
    api.get = originalGet;
  };
}

function renderCard(props: Partial<Parameters<typeof UserHoverCard>[0]> = {}) {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <AuthProvider>
        <UserHoverCard userId={PROFILE_USER_ID} username="sp16a_author" size={40} placement="detail-creator" {...props} />
      </AuthProvider>
    </IntlProvider>,
  );
}

test.afterEach(async () => {
  cleanup();
  window.localStorage.clear();
  const { setAccessToken } = await import("@/lib/api");
  setAccessToken(null);
});

test("user hover card", async (t) => {
  await t.test("trigger shows avatar + username and links to the user page", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(false);
    const restoreGet = stubApiGet();

    const view = renderCard();
    const link = view.getByRole("link", { name: /sp16a_author/ });
    assert.equal(link.getAttribute("href"), `/user/${PROFILE_USER_ID}`);
    // 触屏（hover:none）不挂浮卡语义。
    assert.equal(link.getAttribute("aria-expanded"), null);

    restoreGet();
    restoreMedia();
  });

  await t.test("hover opens the card after the delay with fetched profile actions", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(true);
    const restoreGet = stubApiGet();
    usersResponse = profileResponse();

    const view = renderCard();
    const link = view.getByRole("link", { name: /sp16a_author/ });
    fireEvent.pointerEnter(link.closest("span")!);

    const card = await waitFor(() => view.getByRole("dialog"), { timeout: 2000 });
    assert.ok(within(card).getByText("seed author bio"), "bio must render");
    assert.ok(within(card).getByText("12"), "contents count must render");
    assert.ok(within(card).getByRole("button", { name: "Follow" }), "follow action must render");
    assert.ok(within(card).getByRole("button", { name: "Message" }), "message action must render");

    restoreGet();
    restoreMedia();
  });

  await t.test("Escape closes the card and returns focus to the trigger", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(true);
    const restoreGet = stubApiGet();

    const view = renderCard();
    const link = view.getByRole("link", { name: /sp16a_author/ });
    link.focus();
    fireEvent.focus(link);
    const card = await waitFor(() => view.getByRole("dialog"), { timeout: 2000 });

    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => {
      let gone = false;
      try {
        view.getByRole("dialog");
      } catch {
        gone = true;
      }
      assert.ok(gone, "card must close on Escape");
    });
    assert.ok(document.activeElement === link, "focus must return to the trigger");
    assert.ok(card, "card was open before Escape");

    restoreGet();
    restoreMedia();
  });

  await t.test("self view hides follow/message and shows view-profile", async () => {
    installDom();
    const restoreMedia = installHoverMatchMedia(true);
    // fetchMe 走模块内存 token（getAccessToken），localStorage 与内存都要有。
    saveTokens(validAccessToken());
    setAccessToken(validAccessToken());
    const restoreGet = stubApiGet((path) => {
      if (path === "/api/v1/auth/me") {
        return {
          user: {
            id: PROFILE_USER_ID,
            email: "self@example.com",
            username: "sp16a_author",
            avatar_url: "",
            bio: "",
            reputation: 10,
            preferred_locale: "en",
            role: "user",
            is_banned: false,
            email_verified_at: "2026-01-01T00:00:00Z",
            accept_collab_invites: true,
            created_at: "2026-01-01T00:00:00Z",
          },
        };
      }
      if (path === "/api/v1/notifications/unread-count") {
        return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0, broadcast: 0 } };
      }
      return undefined;
    });

    const view = renderCard();
    const link = view.getByRole("link", { name: /sp16a_author/ });
    fireEvent.pointerEnter(link.closest("span")!);
    const card = await waitFor(() => view.getByRole("dialog"), { timeout: 2000 });

    assert.ok(within(card).getByText("View profile"), "self view must show view-profile");
    let followGone = false;
    try {
      within(card).getByRole("button", { name: "Follow" });
    } catch {
      followGone = true;
    }
    assert.ok(followGone, "self view must not show follow");
    let messageGone = false;
    try {
      within(card).getByRole("button", { name: "Message" });
    } catch {
      messageGone = true;
    }
    assert.ok(messageGone, "self view must not show message");

    restoreGet();
    restoreMedia();
  });

  await t.test("missing userId degrades to a plain identity block", async () => {
    installDom();
    const view = renderCard({ userId: undefined, username: "legacy_user" });
    assert.ok(view.getByText("legacy_user"));
    let noLink = false;
    try {
      view.getByRole("link");
    } catch {
      noLink = true;
    }
    assert.ok(noLink, "no navigation without userId");
  });
});
