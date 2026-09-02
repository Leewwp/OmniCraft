import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import React from "react";
import { IntlProvider } from "use-intl";
import { api, ApiRequestError } from "@/lib/api";
import { SheetMusicViewer } from "@/components/content/SheetMusicViewer";
import VerdictDetail from "@/components/judge/VerdictDetail";
import { cleanup, fireEvent, installDom, render, waitFor } from "./runtime-test-helpers";

const root = path.resolve(process.cwd());

function read(relativePath: string) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

test("detail pages use stable skeletons, empty states, retry actions, and live status", () => {
  const feedback = read("app/(protected)/feedback/[feedbackId]/page.tsx");
  // #290：讨论帖详情由 /ip/[ipId] Hub 的浮层承载（旧 [discussionId] 页已删）
  const discussion = read("components/ip/hub/DiscussionDetailOverlay.tsx");

  for (const source of [feedback, discussion]) {
    assert.match(source, /<Skeleton/);
    assert.match(source, /<EmptyState/);
    assert.match(source, /common\.retry/);
    // live 状态播报与错误告警语义至少其一（浮层以 role="alert" 承载）
    assert.match(source, /aria-live="polite"|role="alert"/);
  }

  assert.doesNotMatch(feedback, /<p[^>]*>\{t\("common\.loading"\)\}<\/p>/);
  assert.doesNotMatch(discussion, /<div[^>]*>\{t\("common\.loading"\)\}<\/div>/);
});

test("detail viewers keep transient loading/error states retriable and replace raw vote SVGs", () => {
  const sheetMusic = read("components/content/SheetMusicViewer.tsx");
  const verdict = read("components/judge/VerdictDetail.tsx");

  assert.match(sheetMusic, /aria-busy/);
  assert.match(sheetMusic, /common\.retry/);
  assert.match(verdict, /<ThumbsUp/);
  assert.match(verdict, /<ThumbsDown/);
  assert.match(verdict, /common\.retry/);
  assert.doesNotMatch(verdict, /<svg/);
});

test("sheet music empty state is stable and verdict error/success states are actionable", async () => {
  installDom();
  const messages = {
    common: { loading: "Loading", retry: "Retry", loadFailed: "Failed to load", noData: "No data" },
    social: { like: "Like", dislike: "Dislike" },
    content: { noSheetMusic: "No sheet music files" },
    judge: {
      verdict: { loadFailed: "Failed to load verdict", noReasons: "No reasons", title: "Verdict", voteDistribution: "Vote distribution", votes: "{totalVotes} votes", closed: "Closed", approveCount: "Approve {count}", rejectCount: "Reject {count}", result: "Result: {result}", judgeReasons: "Judge reasons" },
      reviewCard: { approve: "Approve", reject: "Reject" },
    },
  };
  const view = render(
    <IntlProvider locale="en" messages={messages}>
      <SheetMusicViewer contentId={1} attachments={[]} />
    </IntlProvider>,
  );
  assert.ok(view.getByText("No sheet music files"));
  cleanup();

  const originalGet = api.get;
  let calls = 0;
  api.get = async () => {
    calls += 1;
    throw new ApiRequestError("NETWORK", "private backend detail", 500);
  };
  const errorView = render(
    <IntlProvider locale="en" messages={messages}>
      <VerdictDetail caseId={7} />
    </IntlProvider>,
  );
  await waitFor(() => assert.ok(errorView.getByRole("alert")));
  assert.ok(errorView.getByText("Failed to load verdict"));
  const retry = errorView.getByRole("button", { name: "Retry" });
  fireEvent.click(retry);
  await waitFor(() => assert.equal(calls, 2));
  cleanup();

  api.get = async function successGet<T>() {
    return {
      case: { id: 7, target_type: "content", target_id: 1, status: "closed", vote_approve: 1, vote_reject: 0, min_votes: 1, created_at: "2026-08-02T00:00:00Z" },
      votes: [{ id: 1, judge_id: 2, judge_name: "Judge", vote: "approve", reason: "Clear", created_at: "2026-08-02T00:00:00Z", upvotes: 1, downvotes: 0 }],
    } as T;
  };
  const successView = render(
    <IntlProvider locale="en" messages={messages}>
      <VerdictDetail caseId={7} />
    </IntlProvider>,
  );
  await waitFor(() => assert.ok(successView.getByText("Clear")));
  assert.ok(successView.getByRole("button", { name: "Like" }));
  assert.ok(successView.getByRole("button", { name: "Dislike" }));
  api.get = originalGet;
  cleanup();
});
