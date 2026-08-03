import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import {
  cleanup,
  createFakeCaptcha,
  fireEvent,
  installDom,
  renderWithIntl,
  toggleCheckbox,
  typeInto,
  waitFor,
} from "./runtime-test-helpers";

test.afterEach(() => {
  cleanup();
});

test("register remounts captcha after a failed submit consumes a one-time ticket", async () => {
  installDom();

  const pageModule = await import("@/components/auth/RegisterPageContent");
  const apiModule = await import("@/lib/api");
  const publicConfigModule = await import("@/lib/public-config");
  const RegisterPageContent = pageModule.RegisterPageContent;
  const { FakeCaptcha, getMountCount } = createFakeCaptcha();
  publicConfigModule.clearPublicConfigCache();

  const originalPost = apiModule.api.post;
  apiModule.api.post = (async () => {
    const err = new apiModule.ApiRequestError("USER_EXISTS", "Email taken", 409);
    throw err;
  }) as typeof apiModule.api.post;

  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    if (url.includes("/api/v1/config/public")) {
      return new Response(
        JSON.stringify({
          features: {
            web_agent_enabled: false,
            payment_enabled: false,
            creator_support_enabled: false,
            desktop_deploy_enabled: false,
          },
          captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
          client: { download_enabled: false, download_url: "", latest_version: "" },
          legal: { current_terms_version: "", current_privacy_version: "" },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }
    return new Response("not-found", { status: 404 });
  }) as typeof fetch;

  try {
    const view = renderWithIntl(
      <RegisterPageContent user={null} router={{ push: () => undefined, replace: () => undefined }} CaptchaComponent={FakeCaptcha} />,
    );

    await typeInto(view.getByLabelText("Username") as HTMLInputElement, "review-user");
    await typeInto(view.getByLabelText("Email") as HTMLInputElement, "review@example.com");
    await typeInto(view.getByLabelText("Password") as HTMLInputElement, "Password123");
    await typeInto(view.getByLabelText("Confirm password") as HTMLInputElement, "Password123");
    const checkboxes = view.getAllByRole("checkbox");
    await toggleCheckbox(checkboxes[0] as HTMLInputElement, true);
    await toggleCheckbox(checkboxes[1] as HTMLInputElement, true);
    fireEvent.click(view.getByRole("button", { name: "solve captcha" }));
    assert.equal(getMountCount(), 1);
    await new Promise((resolve) => setTimeout(resolve, 0));

    fireEvent.click(view.getByRole("button", { name: "Create account" }));

    await waitFor(() => assert.equal(getMountCount(), 2));
  } finally {
    apiModule.api.post = originalPost;
    globalThis.fetch = originalFetch;
  }
});

test("forgot-password remounts captcha after a failed submission consumes a one-time ticket", async () => {
  installDom();

  const pageModule = await import("@/components/auth/ForgotPasswordContent");
  const apiModule = await import("@/lib/api");
  const ForgotPasswordContent = pageModule.ForgotPasswordContent;
  const { FakeCaptcha, getMountCount } = createFakeCaptcha();

  const originalPost = apiModule.api.post;
  apiModule.api.post = (async () => {
    throw new apiModule.ApiRequestError("RATE_LIMITED", "Too many requests", 429);
  }) as typeof apiModule.api.post;

  try {
    const view = renderWithIntl(<ForgotPasswordContent CaptchaComponent={FakeCaptcha} />);
    await typeInto(view.getByLabelText("Email") as HTMLInputElement, "review@example.com");
    fireEvent.click(view.getByRole("button", { name: "solve captcha" }));
    assert.equal(getMountCount(), 1);
    await new Promise((resolve) => setTimeout(resolve, 0));

    fireEvent.click(view.getByRole("button", { name: "Send reset link" }));

    await waitFor(() => assert.equal(getMountCount(), 2));
  } finally {
    apiModule.api.post = originalPost;
  }
});

test("settings resend verification remounts captcha after resend attempt", async () => {
  installDom();

  const settingsModule = await import("@/components/settings/VerificationReminderCard");
  const apiModule = await import("@/lib/api");
  const VerificationReminderCard = settingsModule.VerificationReminderCard;
  const { FakeCaptcha, getMountCount } = createFakeCaptcha();

  const originalPost = apiModule.api.post;
  apiModule.api.post = (async () => {
    return {};
  }) as typeof apiModule.api.post;

  try {
    const view = renderWithIntl(
      <VerificationReminderCard email="review@example.com" CaptchaComponent={FakeCaptcha} />,
    );
    fireEvent.click(view.getByRole("button", { name: "solve captcha" }));
    assert.equal(getMountCount(), 1);

    fireEvent.click(view.getByRole("button", { name: "Resend verification" }));

    await waitFor(() => assert.equal(getMountCount(), 2));
  } finally {
    apiModule.api.post = originalPost;
  }
});

test("anonymous feedback remounts captcha after a failed ticket submission", async () => {
  installDom();

  const feedbackModule = await import("@/components/feedback/FeedbackForm");
  const apiModule = await import("@/lib/api");
  const FeedbackFormInner = feedbackModule.FeedbackFormInner;
  const { FakeCaptcha, getMountCount } = createFakeCaptcha();

  const originalPost = apiModule.api.post;
  apiModule.api.post = (async (_path: string) => {
    throw new apiModule.ApiRequestError("INTERNAL_ERROR", "Submit failed", 500);
  }) as typeof apiModule.api.post;

  try {
    const view = renderWithIntl(
      <FeedbackFormInner user={null} CaptchaComponent={FakeCaptcha} />,
    );

    const textboxes = view.getAllByRole("textbox");
    await typeInto(textboxes[0] as HTMLInputElement, "Feedback title");
    await typeInto(textboxes[1] as HTMLTextAreaElement, "Feedback description");
    await typeInto(textboxes[2] as HTMLInputElement, "review@example.com");
    await typeInto(view.getByRole("combobox") as HTMLSelectElement, "other");
    fireEvent.click(view.getByRole("button", { name: "solve captcha" }));
    assert.equal(getMountCount(), 1);
    await waitFor(() => assert.equal(view.getByRole("button", { name: "Submit feedback" }).hasAttribute("disabled"), false));

    const form = view.container.querySelector("form");
    assert.ok(form, "feedback form should exist");
    fireEvent.submit(form);

    await waitFor(() => assert.equal(getMountCount(), 2));
  } finally {
    apiModule.api.post = originalPost;
  }
});
