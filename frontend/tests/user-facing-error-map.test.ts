import test from "node:test";
import assert from "node:assert/strict";
import zhMessages from "@/messages/zh.json";
import enMessages from "@/messages/en.json";
import { ApiRequestError } from "@/lib/api";
import { ERROR_CODE_MESSAGE_KEYS, getUserFacingErrorKey } from "@/lib/user-facing-error";

type Messages = Record<string, unknown>;

function resolveCopy(messages: Messages, key: string): string | null {
  let current: unknown = messages;
  for (const segment of key.split(".")) {
    if (typeof current !== "object" || current === null) return null;
    current = (current as Messages)[segment];
  }
  return typeof current === "string" ? current : null;
}

test("error-code map covers the audited high-frequency codes (FIX-26)", () => {
  const expected: Array<[string, string]> = [
    ["PUBLISH_FROZEN", "publish.frozen"],
    ["APPEAL_EXISTS", "appeals.exists"],
    ["SOURCE_IMMUTABLE", "publish.sourceImmutable"],
    ["SOURCE_NOT_ALLOWED_FOR_ORIGINAL", "publish.sourceNotAllowedForOriginal"],
    ["FANWORK_SOURCE_REQUIRED", "publish.fanworkSourceRequired"],
    ["MULTIPLE_SOURCE_CONFLICT", "publish.multipleSourceConflict"],
    ["SOURCE_ORIGINAL_UNAVAILABLE", "publish.sourceOriginalUnavailable"],
    ["SOURCE_FANWORK_UNAVAILABLE", "publish.sourceFanworkUnavailable"],
    ["MEDIA_SET_INVALID", "publish.mediaSetInvalid"],
    ["FILE_TOO_LARGE", "publish.fileTooLarge"],
    ["INVALID_MIME_TYPE", "publish.invalidMimeType"],
    ["LOW_REPUTATION", "common.insufficientReputation"],
    ["INSUFFICIENT_REPUTATION", "common.insufficientReputation"],
    ["BLOCKED", "common.blocked"],
    ["ALREADY_REPORTED", "common.alreadyReported"],
    ["CAPTCHA_REQUIRED", "auth.captchaRequired"],
    ["CAPTCHA_FAILED", "auth.captchaFailed"],
    ["CAPTCHA_UNAVAILABLE", "auth.captchaUnavailable"],
    ["EMAIL_SEND_FAILED", "auth.errorEmailSendFailed"],
    ["PASSWORD_TOO_SHORT", "auth.errorPasswordTooShort"],
    ["INVALID_PASSWORD", "auth.errorInvalidPassword"],
    ["MISSING_QUERY", "search.missingQuery"],
    ["QUERY_TOO_LONG", "search.queryTooLong"],
    ["INSUFFICIENT_QUESTIONS", "judge.insufficientQuestions"],
    ["READING_TIME_TOO_SHORT", "judge.readingTimeTooShort"],
    ["EXAM_SESSION_EXPIRED", "judge.examSessionExpired"],
    ["ALREADY_QUALIFIED", "judge.examAlreadyQualified"],
    ["REASON_SELF_VOTE", "judge.verdict.selfVoteForbidden"],
    ["JUDGE_QUALIFICATION_REQUIRED", "judge.verdict.qualificationRequired"],
    ["AGENT_RATE_LIMIT_EXCEEDED", "agent.rateLimited"],
  ];

  for (const [code, key] of expected) {
    assert.equal(ERROR_CODE_MESSAGE_KEYS[code], key, `${code} must map to ${key}`);
  }
});

test("every mapped key resolves to real copy in both locales (no fabricated keys)", () => {
  const entries = Object.entries(ERROR_CODE_MESSAGE_KEYS);
  assert.ok(entries.length >= 38, `expected the extended table (~38 codes), got ${entries.length}`);
  for (const [code, key] of entries) {
    assert.ok(resolveCopy(zhMessages as Messages, key) !== null, `zh.json missing key for ${code} -> ${key}`);
    assert.ok(resolveCopy(enMessages as Messages, key) !== null, `en.json missing key for ${code} -> ${key}`);
  }
});

test("frozen publish copy carries the unfreeze guidance required by the ticket", () => {
  const zh = resolveCopy(zhMessages as Messages, "publish.frozen");
  const en = resolveCopy(enMessages as Messages, "publish.frozen");
  assert.ok(zh && /7 天/.test(zh) && /素质课程/.test(zh), `zh frozen copy must mention the 7-day auto unfreeze and the rehab course: ${zh}`);
  assert.ok(en && /7 days/i.test(en) && /course/i.test(en), `en frozen copy must mention the 7-day auto unfreeze and the rehab course: ${en}`);
});

test("getUserFacingErrorKey resolves mapped codes and keeps safe fallbacks", () => {
  assert.equal(getUserFacingErrorKey(new ApiRequestError("PUBLISH_FROZEN", "internal", 403)), "publish.frozen");
  assert.equal(
    getUserFacingErrorKey(new ApiRequestError("DATABASE_CONSTRAINT_DETAIL", "users_email_key leaked", 409), "content.downloadFailed"),
    "content.downloadFailed",
  );
  assert.equal(getUserFacingErrorKey(new Error("plain"), "common.loadFailed"), "common.loadFailed");
});

test("error namespace keeps only the in-use page keys (dead backend-code entries removed)", () => {
  const expected = ["backHome", "notFound", "notFoundDesc", "pageError", "pageErrorDesc"];
  assert.deepEqual(Object.keys((zhMessages as Messages).error as Messages).sort(), expected);
  assert.deepEqual(Object.keys((enMessages as Messages).error as Messages).sort(), expected);
});
