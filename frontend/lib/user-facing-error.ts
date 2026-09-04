import { ApiRequestError } from "@/lib/api";

/**
 * 后端业务错误码 → 本地化文案 key 的统一出口（FIX-26 / T23）。
 * 每个 key 必须在 zh/en 两个语言包同时存在，由
 * tests/user-facing-error-map.test.ts 全表校验。各模块新增专用码
 * 一律 append 进本表（含 SP-10 的 AGENT_RATE_LIMIT_EXCEEDED、
 * FIX-41 的 FILE_TOO_LARGE），不要在调用点内联映射。
 */
export const ERROR_CODE_MESSAGE_KEYS: Record<string, string> = {
  // 认证与会话
  INVALID_CREDENTIALS: "auth.errorInvalidCredentials",
  USER_BANNED: "auth.errorBanned",
  EMAIL_NOT_VERIFIED: "auth.errorEmailNotVerified",
  USER_EXISTS: "auth.errorEmailTaken",
  USERNAME_TAKEN: "auth.errorUsernameTaken",
  TERMS_VERSION_MISMATCH: "auth.errorTermsVersionMismatch",
  PRIVACY_VERSION_MISMATCH: "auth.errorPrivacyVersionMismatch",
  TOKEN_EXPIRED: "auth.errorSessionExpired",
  UNAUTHORIZED: "auth.errorSessionExpired",
  RESEND_COOLDOWN: "auth.errorResendCooldown",
  CAPTCHA_REQUIRED: "auth.captchaRequired",
  CAPTCHA_FAILED: "auth.captchaFailed",
  CAPTCHA_UNAVAILABLE: "auth.captchaUnavailable",
  EMAIL_SEND_FAILED: "auth.errorEmailSendFailed",
  PASSWORD_TOO_SHORT: "auth.errorPasswordTooShort",
  INVALID_PASSWORD: "auth.errorInvalidPassword",
  // 频控
  RATE_LIMITED: "common.rateLimited",
  CREDENTIAL_RATE_LIMIT_EXCEEDED: "common.rateLimited",
  AGENT_RATE_LIMIT_EXCEEDED: "agent.rateLimited",
  // 发布门禁与内容创建
  PUBLISH_FROZEN: "publish.frozen",
  LOW_REPUTATION: "common.insufficientReputation",
  INSUFFICIENT_REPUTATION: "common.insufficientReputation",
  BLOCKED: "common.blocked",
  FILE_TOO_LARGE: "publish.fileTooLarge",
  INVALID_MIME_TYPE: "publish.invalidMimeType",
  MEDIA_SET_INVALID: "publish.mediaSetInvalid",
  SOURCE_NOT_ALLOWED_FOR_ORIGINAL: "publish.sourceNotAllowedForOriginal",
  FANWORK_SOURCE_REQUIRED: "publish.fanworkSourceRequired",
  MULTIPLE_SOURCE_CONFLICT: "publish.multipleSourceConflict",
  SOURCE_ORIGINAL_UNAVAILABLE: "publish.sourceOriginalUnavailable",
  SOURCE_FANWORK_UNAVAILABLE: "publish.sourceFanworkUnavailable",
  SOURCE_IMMUTABLE: "publish.sourceImmutable",
  // 申诉与举报
  APPEAL_EXISTS: "appeals.exists",
  ALREADY_REPORTED: "common.alreadyReported",
  // 判官理由投票守卫（T38/FIX-36b）
  REASON_SELF_VOTE: "judge.verdict.selfVoteForbidden",
  JUDGE_QUALIFICATION_REQUIRED: "judge.verdict.qualificationRequired",
  // 评论审核门（A4 语义：422 拦截 / 503 不可用）
  CONTENT_BLOCKED: "social.commentBlocked",
  MODERATION_UNAVAILABLE: "social.moderationUnavailable",
  // 搜索
  MISSING_QUERY: "search.missingQuery",
  QUERY_TOO_LONG: "search.queryTooLong",
  // 判官考核
  INSUFFICIENT_QUESTIONS: "judge.insufficientQuestions",
  READING_TIME_TOO_SHORT: "judge.readingTimeTooShort",
  EXAM_SESSION_EXPIRED: "judge.examSessionExpired",
  ALREADY_QUALIFIED: "judge.examAlreadyQualified",
};

export function getUserFacingErrorKey(error: unknown, fallbackKey = "common.operationFailed"): string {
  if (!(error instanceof ApiRequestError)) return fallbackKey;
  return ERROR_CODE_MESSAGE_KEYS[error.code] ?? fallbackKey;
}
