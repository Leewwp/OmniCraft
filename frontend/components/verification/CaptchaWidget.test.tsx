import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { act, cleanup, installDom, renderWithIntl, waitFor } from "@/tests/runtime-test-helpers";

test.afterEach(() => {
  cleanup();
});

test("CaptchaWidget exchanges aliyun verify params for the server-issued captcha token", async () => {
  installDom();

  const widgetModule = await import("./CaptchaWidget");
  const apiModule = await import("@/lib/api");
  const publicConfigModule = await import("@/lib/public-config");
  const CaptchaWidget = widgetModule.CaptchaWidget;
  publicConfigModule.clearPublicConfigCache();

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

  const originalPost = apiModule.api.post;
  const postCalls: Array<{ path: string; body: unknown }> = [];
  apiModule.api.post = (async (path: string, body: unknown) => {
    postCalls.push({ path, body });
    return {
      captcha_result: true,
      captcha_token: "server-issued-ticket",
    };
  }) as typeof apiModule.api.post;

  let capturedOptions: any;
  window.initAliyunCaptcha = (options: any) => {
    capturedOptions = options;
  };
  const sdkScript = document.createElement("script");
  sdkScript.id = "aliyun-captcha-sdk";
  document.head.appendChild(sdkScript);

  const tokens: string[] = [];
  const view = renderWithIntl(
    <CaptchaWidget
      containerId="captcha-container"
      buttonId="captcha-button"
      onToken={(token) => tokens.push(token)}
    />,
  );

  try {
    await waitFor(() => assert.equal(typeof capturedOptions?.captchaVerifyCallback, "function"));
    assert.equal(capturedOptions.button, "#captcha-button");
    assert.equal(capturedOptions.element, "#captcha-container");
    assert.equal(capturedOptions.autoRefresh, false);

    let callbackResult: unknown;
    await act(async () => {
      callbackResult = await capturedOptions.captchaVerifyCallback("aliyun-verify-param");
    });

    assert.deepEqual(callbackResult, { captchaResult: true, bizResult: true });
    assert.deepEqual(postCalls, [
      {
        path: "/api/v1/captcha/verify",
        body: { captcha_verify_param: "aliyun-verify-param" },
      },
    ]);
    await waitFor(() => {
      assert.deepEqual(tokens, ["server-issued-ticket"]);
      assert.match(view.container.textContent ?? "", /Captcha verified/);
    });
  } finally {
    apiModule.api.post = originalPost;
    globalThis.fetch = originalFetch;
  }
});

test("CaptchaWidget only emits bypass when the public config provider is bypass", async () => {
  installDom();

  const widgetModule = await import("./CaptchaWidget");
  const publicConfigModule = await import("@/lib/public-config");
  const CaptchaWidget = widgetModule.CaptchaWidget;
  publicConfigModule.clearPublicConfigCache();

  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async () =>
    new Response(
      JSON.stringify({
        features: {
          web_agent_enabled: false,
          payment_enabled: false,
          creator_support_enabled: false,
          desktop_deploy_enabled: false,
        },
        captcha: {
          provider: "bypass",
          prefix: "",
          scene_id: "",
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
    )) as typeof fetch;

  const tokens: string[] = [];
  try {
    renderWithIntl(<CaptchaWidget onToken={(token) => tokens.push(token)} />);
    await waitFor(() => assert.deepEqual(tokens, ["bypass"]));
  } finally {
    globalThis.fetch = originalFetch;
  }
});
