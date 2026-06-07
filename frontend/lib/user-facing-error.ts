import { ApiRequestError } from "@/lib/api";

export function getUserFacingErrorKey(error: unknown, fallbackKey = "common.operationFailed"): string {
  if (!(error instanceof ApiRequestError)) return fallbackKey;

  switch (error.code) {
    case "INVALID_CREDENTIALS":
      return "auth.errorInvalidCredentials";
    case "USER_BANNED":
      return "auth.errorBanned";
    case "EMAIL_NOT_VERIFIED":
      return "auth.errorEmailNotVerified";
    case "USER_EXISTS":
      return "auth.errorEmailTaken";
    case "USERNAME_TAKEN":
      return "auth.errorUsernameTaken";
    case "TERMS_VERSION_MISMATCH":
      return "auth.errorTermsVersionMismatch";
    case "PRIVACY_VERSION_MISMATCH":
      return "auth.errorPrivacyVersionMismatch";
    case "TOKEN_EXPIRED":
    case "UNAUTHORIZED":
      return "auth.errorSessionExpired";
    case "RATE_LIMITED":
    case "CREDENTIAL_RATE_LIMIT_EXCEEDED":
      return "common.rateLimited";
    case "RESEND_COOLDOWN":
      return "auth.errorResendCooldown";
    default:
      return fallbackKey;
  }
}
