import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import test from "node:test";

function readSource(...parts) {
  return readFileSync(new URL(`../${parts.join("/")}`, import.meta.url), "utf8");
}

const registerSource = readSource("app", "(public)", "register", "page.tsx");
const forgotPasswordSource = readSource("app", "(public)", "forgot-password", "page.tsx");
const settingsSource = readSource("app", "(protected)", "settings", "page.tsx");
const feedbackSource = readSource("components", "feedback", "FeedbackForm.tsx");

function assertResetAfterRequest(source, endpoint, resetCall, label) {
  const requestIndex = source.indexOf(endpoint);
  const resetIndex = source.indexOf(resetCall, requestIndex);

  assert.notEqual(requestIndex, -1, `${label} must submit to ${endpoint}`);
  assert.ok(resetIndex > requestIndex, `${label} must reset captcha after submitting a one-time ticket`);
}

test("register resets captcha after register submission consumes a ticket", () => {
  assert.match(registerSource, /const \[captchaResetKey, setCaptchaResetKey\]/);
  assert.match(registerSource, /function resetCaptcha\(\)/);
  assert.match(registerSource, /<CaptchaWidget\s+key=\{captchaResetKey\}/);
  assertResetAfterRequest(
    registerSource,
    'api.post<RegisterResponse>("/api/v1/auth/register"',
    "resetCaptcha();",
    "register page",
  );
});

test("forgot password resets captcha after reset-link submission consumes a ticket", () => {
  assert.match(forgotPasswordSource, /const \[captchaResetKey, setCaptchaResetKey\]/);
  assert.match(forgotPasswordSource, /function resetCaptcha\(\)/);
  assert.match(forgotPasswordSource, /<CaptchaWidget key=\{captchaResetKey\}/);
  assertResetAfterRequest(
    forgotPasswordSource,
    'api.post("/api/v1/auth/forgot-password"',
    "resetCaptcha();",
    "forgot password page",
  );
});

test("settings resend verification resets captcha after resend consumes a ticket", () => {
  assert.match(settingsSource, /const \[resendCaptchaResetKey, setResendCaptchaResetKey\]/);
  assert.match(settingsSource, /function resetResendCaptcha\(\)/);
  assert.match(settingsSource, /<CaptchaWidget key=\{resendCaptchaResetKey\}/);
  assertResetAfterRequest(
    settingsSource,
    'api.post("/api/v1/auth/resend-verification"',
    "resetResendCaptcha();",
    "settings resend verification",
  );
});

test("anonymous feedback resets captcha after ticket submit attempt", () => {
  assert.match(feedbackSource, /const \[captchaResetKey, setCaptchaResetKey\]/);
  assert.match(feedbackSource, /function resetAnonymousCaptcha\(\)/);
  assert.match(feedbackSource, /<CaptchaWidget key=\{captchaResetKey\}/);
  assertResetAfterRequest(
    feedbackSource,
    'api.post("/api/v1/feedback"',
    "resetAnonymousCaptcha();",
    "anonymous feedback form",
  );
});
