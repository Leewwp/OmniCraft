/* #1022：/judge/history 我的投票——行契约渲染（类型/我的投票/理由赞踩/
 * 案件终态口径与 VerdictDetail 同源）、行展开加载判决详情、空态引导去队列。 */
import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { createRequire } from "node:module";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const requireForMocks = createRequire(import.meta.url) as NodeRequire;
const Module = requireForMocks("node:module") as typeof import("node:module") & {
  _load: (request: string, parent: unknown, isMain: boolean) => unknown;
};
const originalModuleLoad = Module._load;

const state = {
  user: { id: 7, username: "judge_history_user" } as object | null,
  mineItems: [] as Array<Record<string, unknown>>,
};

Module._load = function loadWithStubs(request: string, parent: unknown, isMain: boolean) {
  if (request === "@/lib/api") {
    return {
      api: {
        get: async (url: string) => {
          if (url.includes("/api/v1/judge/votes/mine")) {
            return {
              items: state.mineItems,
              total: state.mineItems.length,
              page: 1,
              page_size: 20,
            };
          }
          if (url.includes("/verdict")) {
            return {
              case: {
                id: 501,
                target_type: "article",
                target_id: 9001,
                status: "closed_approve",
                vote_approve: 12,
                vote_reject: 3,
                min_votes: 10,
                created_at: "2026-11-01T00:00:00Z",
                closed_at: "2026-11-02T00:00:00Z",
              },
              votes: [],
            };
          }
          throw new Error(`unexpected api.get ${url}`);
        },
        post: async () => ({}),
      },
      ApiRequestError: class ApiRequestError extends Error {},
    };
  }
  if (request === "@/contexts/AuthContext") {
    return {
      useAuth: () => ({
        user: state.user,
        isLoading: false,
        capabilities: { can_interact: true },
      }),
    };
  }
  if (request === "@/lib/user-facing-error") {
    return { getUserFacingErrorKey: (_e: unknown, fallback: string) => fallback };
  }
  if (request === "@/lib/error-handler") {
    return { silentError: () => undefined };
  }
  return originalModuleLoad.apply(this, [request, parent, isMain]);
};

test.after(() => {
  Module._load = originalModuleLoad;
});

test.afterEach(() => {
  cleanup();
  state.mineItems = [];
});

function HistoryHost() {
  const Page = require("@/app/(protected)/(headered)/judge/history/page").default as React.ComponentType;
  return <Page />;
}

function renderHistory() {
  return render(
    <IntlProvider locale="en" messages={enMessages}>
      <HistoryHost />
    </IntlProvider>,
  );
}

const closedRow = {
  case_id: 501,
  target_type: "article",
  my_vote: "approve",
  reason: "nothing violates the guidelines",
  created_at: "2026-11-01T12:00:00Z",
  case_status: "closed_approve",
  vote_approve: 12,
  vote_reject: 3,
  min_votes: 10,
  closed_at: "2026-11-02T00:00:00Z",
  upvotes: 4,
  downvotes: 1,
};

const openRow = {
  case_id: 502,
  target_type: "comment",
  my_vote: "reject",
  reason: null,
  created_at: "2026-11-20T12:00:00Z",
  case_status: "open",
  vote_approve: 1,
  vote_reject: 0,
  min_votes: 5,
  closed_at: null,
  upvotes: 0,
  downvotes: 0,
};

test("renders my vote rows with verdict outcome, match marker and progress state", async () => {
  installDom();
  state.mineItems = [closedRow, openRow];
  const view = renderHistory();

  await waitFor(() => {
    if (!view.container.textContent?.includes("2 votes in total")) throw new Error("rows not loaded");
  });

  // 已结案行：类型标签 + 我的投票 + 判决结果 + 「与判决一致」。
  assert.ok(view.container.textContent?.includes("Article"), "type label must render");
  assert.ok(view.container.textContent?.includes("Not a violation"), "my vote label must render");
  assert.ok(view.container.textContent?.includes("Verdict: Not a violation"), "closed verdict must render");
  assert.ok(view.container.textContent?.includes("Matches the verdict"), "match marker must render");
  assert.ok(view.container.textContent?.includes("nothing violates the guidelines"), "my reason must render");

  // 进行中行：x/y 票进度，不出现判决结果。
  assert.ok(view.container.textContent?.includes("In progress 1 / 5 votes"), "open case progress must render");
  assert.ok(!view.container.textContent?.includes("Opposite of the verdict"));
});

test("expanding a row loads the verdict detail for that case", async () => {
  installDom();
  state.mineItems = [closedRow];
  const view = renderHistory();

  const expandButton = await waitFor(() => {
    const button = Array.from(view.container.querySelectorAll("button")).find((b) =>
      b.textContent?.includes("Show verdict details"),
    );
    if (!button) throw new Error("expand button not rendered yet");
    return button as HTMLButtonElement;
  });
  assert.equal(expandButton.getAttribute("aria-expanded"), "false");

  await fireEvent.click(expandButton);

  await waitFor(() => {
    if (!view.container.textContent?.includes("Verdict Details")) throw new Error("verdict detail not loaded");
  });
  assert.equal(expandButton.getAttribute("aria-expanded"), "true");
});

test("empty history shows the empty state with a queue CTA", async () => {
  installDom();
  state.mineItems = [];
  const view = renderHistory();

  await waitFor(() => {
    if (!view.container.textContent?.includes("No votes yet")) throw new Error("empty state not rendered");
  });
  const queueLink = view.container.querySelector('a[href="/judge/queue"]');
  assert.ok(queueLink, "empty state must link to the review queue");
  assert.ok(view.container.textContent?.includes("Go to review queue"));
});
