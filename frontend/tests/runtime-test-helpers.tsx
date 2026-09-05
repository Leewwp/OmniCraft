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
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  Element: dom.window.Element,
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
export const { act, cleanup, fireEvent, render, waitFor, within } = testingLibrary;

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
  media: {
    gallery: {
      position: "{current} / {total}",
      previous: "Previous media",
      next: "Next media",
      imageAlt: "Media {current} of {total}",
      error: {
        loadFailed: "Failed to load media",
      },
    },
    viewer: {
      title: "Media viewer",
      position: "{current} / {total}",
      close: "Close viewer",
      previous: "Previous media",
      next: "Next media",
      zoomIn: "Zoom in",
      zoomOut: "Zoom out",
      zoomReset: "Reset zoom",
      imageAlt: "Media {current} of {total}",
      error: {
        loadFailed: "Failed to load media",
        retry: "Retry",
      },
    },
  },
  studio: {
    publish: {
      media: {
        imageTitle: "Image set",
        videoTitle: "Video set",
        imageHint: "Upload {min}-{max} images. The first item becomes the cover.",
        videoHint: "Upload {min}-{max} videos.",
        select: "Select media",
        addMore: "Add more media",
        empty: "Drop or select media to start arranging",
        listLabel: "Selected media",
        previewAlt: "Preview of {name}",
        cover: "Cover",
        remove: "Remove {name}",
        moveUp: "Move {name} up",
        moveDown: "Move {name} down",
        uploading: "Uploading...",
        uploadingProgress: "Uploading {progress}%",
        uploadPending: "Wait for all media uploads to finish",
        uploadFailed: "Upload failed. Remove the failed item and try again.",
        countError: "Upload {min}-{max} media files",
        maxCount: "You can upload up to {max} media files",
        wrongType: "Choose a file matching the content type",
        invalidType: "Invalid media type",
        imageReadFailed: "Could not read the image dimensions",
        videoReadFailed: "Could not read the video file",
        posterUnavailable: "Could not generate a video poster",
        posterRequired: "Video cover generation failed.",
        loadingLimits: "Loading upload limits...",
        limitsUnavailable: "Upload limits are unavailable. Try again later.",
      },
    },
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

// 把 lib/api 裸 fetch 的 auth 端点（/auth/csrf、/auth/refresh）钉在本进程内，
// 防止单测挂载 AuthProvider 时打真实网络（#381 后 AuthContext 走 refreshSession
// 裸 fetch 管线）。返回恢复函数。
export function installAuthFetchStub(options?: { refreshStatus?: number }) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const path = String(input);
    if (path.endsWith("/auth/csrf")) {
      return new Response(JSON.stringify({ csrf_token: "test-csrf" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    if (path.endsWith("/auth/refresh")) {
      return new Response(
        JSON.stringify({ code: "INVALID_TOKEN", message: "not logged in" }),
        { status: options?.refreshStatus ?? 401, headers: { "Content-Type": "application/json" } },
      );
    }
    return originalFetch(input, init);
  }) as typeof fetch;
  return () => {
    globalThis.fetch = originalFetch;
  };
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
