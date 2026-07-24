import React, { type ComponentType, useEffect, useRef } from "react";
import { JSDOM } from "jsdom";
import { IntlProvider } from "use-intl";
import assert from "node:assert/strict";

const dom = new JSDOM("<!doctype html><html><body></body></html>", {
  url: "http://localhost/",
});

for (const [key, value] of Object.entries({
  window: dom.window,
  self: dom.window,
  document: dom.window.document,
  navigator: dom.window.navigator,
  HTMLElement: dom.window.HTMLElement,
  Node: dom.window.Node,
  Event: dom.window.Event,
  MutationObserver: dom.window.MutationObserver,
  File: dom.window.File,
  FormData: dom.window.FormData,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
})) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    writable: true,
    value,
  });
}

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
window.requestAnimationFrame = (callback: FrameRequestCallback) =>
  window.setTimeout(() => callback(performance.now()), 0) as unknown as number;
window.cancelAnimationFrame = (handle: number) => window.clearTimeout(handle);
window.requestIdleCallback = ((callback: IdleRequestCallback) =>
  window.setTimeout(() => callback({ didTimeout: false, timeRemaining: () => 0 }), 0)) as typeof window.requestIdleCallback;
window.cancelIdleCallback = ((handle: number) => window.clearTimeout(handle)) as typeof window.cancelIdleCallback;
Object.defineProperty(globalThis, "requestAnimationFrame", {
  configurable: true,
  value: window.requestAnimationFrame.bind(window),
});
Object.defineProperty(globalThis, "cancelAnimationFrame", {
  configurable: true,
  value: window.cancelAnimationFrame.bind(window),
});

// Load testing-library after the DOM globals exist so React attaches event handling to JSDOM.
// eslint-disable-next-line @typescript-eslint/no-require-imports
const testingLibrary = require("@testing-library/react") as typeof import("@testing-library/react");
export const { act, cleanup, fireEvent, render, waitFor } = testingLibrary;

export const testMessages = {
  auth: {
    captchaLoading: "Captcha loading",
    captchaReady: "Captcha ready",
    captchaVerified: "Captcha verified",
    captchaFailed: "Captcha failed",
    errorUsernameTooShort: "Username too short",
    errorUsernameTooLong: "Username too long",
    errorInvalidEmail: "Invalid email",
    errorPasswordTooShort: "Password too short",
    errorPasswordMismatch: "Passwords do not match",
    errorTermsRequired: "Terms required",
    errorPrivacyRequired: "Privacy required",
    errorCaptchaRequired: "Captcha required",
    errorRegisterFailed: "Register failed",
    errorEmailTaken: "Email taken",
    errorUsernameTaken: "Username taken",
    errorEmailSendFailed: "Verification email failed",
    errorTermsVersionMismatch: "Terms version mismatch",
    errorPrivacyVersionMismatch: "Privacy version mismatch",
    joinTitle: "Join",
    registerSubtitle: "Create your account",
    username: "Username",
    displayName: "Display name",
    email: "Email",
    password: "Password",
    passwordMinLength: "At least 8 characters",
    hidePassword: "Hide password",
    showPassword: "Show password",
    confirmPassword: "Confirm password",
    confirmPasswordPlaceholder: "Confirm password",
    hideConfirmPassword: "Hide confirm password",
    showConfirmPassword: "Show confirm password",
    acceptTerms: "Accept",
    termsOfService: "Terms of service",
    acceptPrivacy: "Accept",
    privacyPolicy: "Privacy policy",
    createAccount: "Create account",
    hasAccount: "Already have an account?",
    loginNow: "Log in",
    forgotPassword: "Forgot password",
    forgotPasswordHint: "Send a reset link",
    sendResetLink: "Send reset link",
    checkEmail: "Check your email",
    resetLinkSent: "Reset link sent",
    backToLogin: "Back to login",
    resendCooldown: "{seconds}s",
    resendVerification: "Resend verification",
    verificationResent: "Verification resent",
  },
  feedback: {
    category: "Category",
    selectCategory: "Select category",
    cat_web_bug: "Web bug",
    cat_desktop_deploy: "Desktop deploy",
    cat_content_or_community: "Content or community",
    cat_account_or_security: "Account or security",
    cat_agent_quality: "Agent quality",
    cat_feature_request: "Feature request",
    cat_other: "Other",
    title: "Title",
    description: "Description",
    contactEmail: "Contact email",
    screenshot: "Screenshot",
    uploadScreenshot: "Upload screenshot",
    screenshotHint: "Upload an image if useful",
    includeDiagnostics: "Include diagnostics",
    diagnosticsHint: "Diagnostics help support",
    submit: "Submit feedback",
    categoryRequired: "Category required",
    titleAndDescriptionRequired: "Title and description required",
    contactEmailRequired: "Contact email required",
    captchaRequired: "Captcha required",
    screenshotTooLarge: "Screenshot too large",
    screenshotUploadFailed: "Screenshot upload failed",
    submitSuccess: "Feedback submitted",
    submitSuccessDesc: "We will review it soon",
  },
  settings: {
    verifyEmailTitle: "Verify your email",
    verifyEmailDesc: "Verification is required",
    emailVerified: "Verified",
    emailUnverified: "Unverified",
  },
  content: {
    limitMb: "Limit: {maxMB}MB",
    selectFile: "Select file",
    uploading: "Uploading",
    uploadProgress: "{progress}%",
    fileSizeExceeds: "{name} exceeds {maxMB}MB",
    uploadFailed: "Upload failed",
  },
  common: {
    close: "Close",
    confirm: "Confirm",
    reason: "Reason",
    removeTag: "Remove {tag}",
    removeTagGeneric: "Remove tag",
    processing: "Processing",
    operationFailed: "Operation failed",
    rateLimited: "Rate limited",
    saveSuccess: "Saved",
    saveFailed: "Save failed",
    saving: "Saving",
    cancel: "Cancel",
  },
  footer: {
    feedback: "Feedback",
  },
} as const;

export function installDom() {
  cleanup();
  document.head.innerHTML = "";
  document.body.innerHTML = "";
  window.history.replaceState({}, "", "http://localhost/");

  return dom;
}

export function renderWithIntl(node: React.ReactNode) {
  const originalConsoleError = console.error;
  console.error = (...args: unknown[]) => {
    if (
      args.some(
        (arg) =>
          typeof arg === "object" &&
          arg !== null &&
          "code" in arg &&
          (arg as { code?: string }).code === "ENVIRONMENT_FALLBACK",
      )
    ) {
      return;
    }
    originalConsoleError(...args);
  };

  try {
    return render(
      <IntlProvider locale="en" messages={testMessages}>
        {node}
      </IntlProvider>,
    );
  } finally {
    console.error = originalConsoleError;
  }
}

export function unwrapDefaultExport<T>(value: T | { default: T }): T {
  if (value && typeof value === "object" && "default" in (value as { default?: T })) {
    return (value as { default: T }).default;
  }
  return value as T;
}

export function createFakeCaptcha() {
  let mounts = 0;
  let nextInstance = 0;

  function FakeCaptcha({
    onToken,
    onError,
  }: {
    onToken: (token: string) => void;
    onError?: (message: string) => void;
  }) {
    const instanceId = useRef(++nextInstance);
    useEffect(() => {
      mounts += 1;
    }, []);
    return (
      <div data-testid="fake-captcha" data-instance={String(instanceId.current)}>
        <button type="button" onClick={() => onToken(`ticket-${instanceId.current}`)}>
          solve captcha
        </button>
        <button type="button" onClick={() => onError?.(`captcha-error-${instanceId.current}`)}>
          fail captcha
        </button>
      </div>
    );
  }

  return {
    FakeCaptcha: FakeCaptcha as ComponentType<any>,
    getMountCount: () => mounts,
  };
}

export async function typeInto(
  element: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement,
  value: string,
) {
  if (element instanceof HTMLSelectElement) {
    fireEvent.change(element, { target: { value } });
  } else {
    fireEvent.change(element, { target: { value } });
  }
  await waitFor(() => {
    assert.equal(element.value, value);
  });
}

export async function toggleCheckbox(element: HTMLInputElement, checked: boolean) {
  fireEvent.click(element);
  await waitFor(() => {
    assert.equal(element.checked, checked);
  });
}
