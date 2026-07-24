import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { IntlProvider } from "use-intl";

import ErrorPage from "@/app/error";
import { ApiRequestError } from "@/lib/api";
import { getUserFacingErrorKey } from "@/lib/user-facing-error";

import { cleanup } from "./runtime-test-helpers";

const rawErrorSurfaces = [
  "../app/error.tsx",
  "../app/(public)/verify-email/page.tsx",
  "../app/(protected)/dashboard/pr-requests/page.tsx",
  "../app/(public)/user/[userId]/UserProfileClient.tsx",
  "../components/content/ContentDetail.tsx",
  "../components/content/VersionHistory.tsx",
  "../components/content/DownloadButton.tsx",
  "../components/feedback/FeedbackForm.tsx",
  "../app/(protected)/admin/agent-config/page.tsx",
  "../app/(protected)/admin/appeal/page.tsx",
  "../app/(protected)/admin/audit-logs/page.tsx",
  "../app/(protected)/admin/categories/page.tsx",
  "../app/(protected)/admin/config/page.tsx",
  "../app/(protected)/admin/feedback/page.tsx",
  "../app/(protected)/admin/ips/page.tsx",
  "../app/(protected)/admin/queue/page.tsx",
  "../app/(protected)/admin/reports/page.tsx",
  "../app/(protected)/admin/users/page.tsx",
  "../app/(protected)/appeals/page.tsx",
  "../app/(protected)/dashboard/contributors/page.tsx",
  "../app/(protected)/dashboard/tag-suggestions/page.tsx",
  "../app/(protected)/feedback/[feedbackId]/page.tsx",
  "../app/(protected)/feedback/mine/page.tsx",
  "../app/(protected)/ip/[ipId]/discussions/new/page.tsx",
  "../app/(protected)/judge/exam/page.tsx",
  "../app/(protected)/judge/queue/page.tsx",
  "../app/(protected)/rehab/page.tsx",
  "../app/(protected)/settings/page.tsx",
  "../app/(protected)/settings/tag-groups/page.tsx",
  "../app/(public)/ip/[ipId]/discussions/[discussionId]/page.tsx",
  "../app/(public)/ip/[ipId]/discussions/page.tsx",
  "../components/agent/UploadAssistPanel.tsx",
  "../components/judge/VerdictDetail.tsx",
  "../components/social/CreatorSupportPanel.tsx",
  "../components/social/ReplyList.tsx",
] as const;

const rawRenderingPatterns = [
  /error\.message\s*\|\|/,
  /instanceof\s+ApiRequestError\s*\?\s*(?:e|err)\.message/,
  /\$\{(?:e|err)\.code\}:\s*\$\{(?:e|err)\.message\}/,
  /toast\(\s*["']error["']\s*,\s*err\.message\s*\)/,
] as const;

test.afterEach(() => {
  cleanup();
});

test("known API codes map to translation keys and unknown messages stay behind a safe fallback", () => {
  const known = new ApiRequestError("RATE_LIMITED", "internal rate-limit detail", 429);
  const unknown = new ApiRequestError("DATABASE_CONSTRAINT_DETAIL", "users_email_key leaked", 409);

  assert.equal(getUserFacingErrorKey(known), "common.rateLimited");
  assert.equal(getUserFacingErrorKey(unknown, "content.downloadFailed"), "content.downloadFailed");
  assert.notEqual(getUserFacingErrorKey(unknown, "content.downloadFailed"), unknown.message);
});

test("the global error boundary renders only localized fallback copy", () => {
  const secret = "postgres://internal-user:password@db/private";
  const originalConsoleError = console.error;
  console.error = () => undefined;

  try {
    const html = renderToStaticMarkup(
      <IntlProvider
        locale="en"
        messages={{
          common: { retry: "Retry" },
          error: {
            pageError: "Something went wrong",
            pageErrorDesc: "An unexpected error occurred. Please try again later.",
          },
        }}
      >
        <ErrorPage error={new Error(secret)} reset={() => undefined} />
      </IntlProvider>,
    );

    assert.doesNotMatch(html, /postgres:\/\/internal-user:password/);
    assert.match(html, /An unexpected error occurred\. Please try again later\./);
  } finally {
    console.error = originalConsoleError;
  }
});

test("audited user-facing surfaces do not render raw ApiRequestError messages", async () => {
  const violations: string[] = [];

  for (const relativePath of rawErrorSurfaces) {
    const source = await readFile(new URL(relativePath, import.meta.url), "utf8");
    for (const pattern of rawRenderingPatterns) {
      if (pattern.test(source)) {
        violations.push(`${relativePath}: ${pattern.source}`);
      }
    }
  }

  assert.deepEqual(violations, []);
});
