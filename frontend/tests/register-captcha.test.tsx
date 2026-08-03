import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import {
  createFakeCaptcha,
  cleanup,
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

test("register blocks aliyun_v2 submission when captcha token is empty", async () => {
  installDom();

  const pageModule = await import("@/components/auth/RegisterPageContent");
  const apiModule = await import("@/lib/api");
  const publicConfigModule = await import("@/lib/public-config");
  const RegisterPageContent = pageModule.RegisterPageContent;
  publicConfigModule.clearPublicConfigCache();

  const postCalls: Array<{ path: string; body: unknown }> = [];
  const originalPost = apiModule.api.post;
  apiModule.api.post = (async (path: string, body: unknown) => {
    postCalls.push({ path, body });
    throw new Error("register request should not be sent");
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
          captcha: {
            provider: "aliyun_v2",
            prefix: "review-prefix",
            scene_id: "review-scene",
            region: "cn",
          },
          client: {
            download_enabled: false,
            download_url: "",
            latest_version: "",
          },
          legal: {
            current_terms_version: "",
            current_privacy_version: "",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    }
    return new Response("not-found", { status: 404 });
  }) as typeof fetch;

  try {
    const { FakeCaptcha } = createFakeCaptcha();
    const router = { push: () => undefined, replace: () => undefined };

    const view = renderWithIntl(
      <RegisterPageContent user={null} router={router} CaptchaComponent={FakeCaptcha} />,
    );

    await typeInto(view.getByLabelText("Username") as HTMLInputElement, "review-user");
    await typeInto(view.getByLabelText("Email") as HTMLInputElement, "review@example.com");
    await typeInto(view.getByLabelText("Password") as HTMLInputElement, "Password123");
    await typeInto(view.getByLabelText("Confirm password") as HTMLInputElement, "Password123");

    const checkboxes = view.getAllByRole("checkbox");
    await toggleCheckbox(checkboxes[0] as HTMLInputElement, true);
    await toggleCheckbox(checkboxes[1] as HTMLInputElement, true);

    fireEvent.click(view.getByRole("button", { name: "Create account" }));

    await waitFor(() => {
      assert.equal(postCalls.length, 0);
      assert.match(view.container.textContent ?? "", /Captcha required/);
    });
  } finally {
    apiModule.api.post = originalPost;
    globalThis.fetch = originalFetch;
  }
});

test("register renders stable captcha container and button ids in the DOM", async () => {
  installDom();

  const pageModule = await import("@/components/auth/RegisterPageContent");
  const RegisterPageContent = pageModule.RegisterPageContent;
  const publicConfigModule = await import("@/lib/public-config");
  publicConfigModule.clearPublicConfigCache();
  const CaptchaEcho = ({
    containerId,
    buttonId,
  }: {
    containerId?: string;
    buttonId?: string;
  }) => <div id={containerId} data-button-id={buttonId} />;

  const view = renderWithIntl(
    <RegisterPageContent
      user={null}
      router={{ push: () => undefined, replace: () => undefined }}
      CaptchaComponent={CaptchaEcho}
    />,
  );

  assert.ok(view.container.querySelector("#register-captcha-container"));
  assert.equal(
    view.container.querySelector("#register-captcha-container")?.getAttribute("data-button-id"),
    "register-captcha-button",
  );
  assert.ok(view.container.querySelector("#register-submit-button"));
});
