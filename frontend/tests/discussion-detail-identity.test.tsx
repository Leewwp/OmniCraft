import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";

import enMessages from "@/messages/en.json";
import { api } from "@/lib/api";
import { cleanup, installDom, render } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

// #494：讨论帖详情浮层（DiscussionDetailOverlay）楼主与回帖作者（ReplyList）
// = UserHoverCard 触发器——头像（avatar_url 真图 / 首字母兜底）+ /user/:id 链接。
const authStub = {
  user: { id: 1 } as { id: number } | null,
  capabilities: { can_interact: true, interaction_denial_reason: "" },
};

Module._load = function loadWithAuthStub(request, parent, isMain) {
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => authStub,
      interactionDenialKey: (reason?: string) =>
        reason === "INSUFFICIENT_REPUTATION"
          ? "capabilities.deniedInsufficientReputation"
          : "capabilities.deniedUnavailable",
    };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

test.after(() => {
  Module._load = originalModuleLoad;
});

const DISCUSSION_ID = 77;
const originalGet = api.get;

function stubDiscussionDetail() {
  api.get = (async <T,>(path: string): Promise<T> => {
    assert.equal(path, `/api/v1/discussions/${DISCUSSION_ID}`);
    return {
      discussion: {
        id: DISCUSSION_ID,
        title: "Seeded thread",
        body: "thread body",
        created_at: "2026-01-01T00:00:00Z",
        author: { id: 11, username: "thread_author", avatar_url: "https://cdn.example.com/a.png" },
      },
      comments: [
        { id: 501, body: "top reply", created_at: "2026-01-02T00:00:00Z", author: { id: 12, username: "reply_author", avatar_url: "https://cdn.example.com/b.png" } },
        { id: 502, body: "nested reply", parent_id: 501, created_at: "2026-01-03T00:00:00Z", author: { id: 13, username: "nested_author" } },
      ],
      total: 1,
    } as T;
  }) as typeof api.get;
}

test.beforeEach(() => {
  installDom();
  stubDiscussionDetail();
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
});

test("discussion detail overlay renders OP and reply authors as hovercard triggers (#494)", async () => {
  const { DiscussionDetailOverlay } = await import("@/components/ip/hub/DiscussionDetailOverlay");
  const view = render(
    <IntlProvider locale="en" messages={enMessages}>
      <DiscussionDetailOverlay discussionId={DISCUSSION_ID} onClose={() => {}} />
    </IntlProvider>,
  );

  // 楼主：/user/:id 链接 + avatar_url 真图
  const opLink = await view.findByRole("link", { name: "thread_author" });
  assert.equal(opLink.getAttribute("href"), "/user/11");
  assert.ok(opLink.querySelector("img"), "OP avatar rendered from author.avatar_url");

  // 顶层回帖作者
  const replyLink = await view.findByRole("link", { name: "reply_author" });
  assert.equal(replyLink.getAttribute("href"), "/user/12");
  assert.ok(replyLink.querySelector("img"), "reply avatar rendered from author.avatar_url");

  // 嵌套回复作者：无 avatar_url → 首字母兜底（不渲染 <img>）
  const nestedLink = await view.findByRole("link", { name: "nested_author" });
  assert.equal(nestedLink.getAttribute("href"), "/user/13");
  assert.equal(nestedLink.querySelector("img"), null, "no avatar img when avatar_url missing");
});
