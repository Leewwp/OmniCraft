import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { cleanup, createFakeCaptcha, fireEvent, installDom, renderWithIntl, typeInto, waitFor } from "./runtime-test-helpers";

test.afterEach(() => {
  cleanup();
});

test("failed OSS screenshot upload does not retain a screenshot grant for feedback submission", async () => {
  installDom();

  const feedbackModule = await import("@/components/feedback/FeedbackForm");
  const apiModule = await import("@/lib/api");
  const FeedbackFormInner = feedbackModule.FeedbackFormInner;
  const { FakeCaptcha } = createFakeCaptcha();

  const originalPost = apiModule.api.post;
  const postCalls: Array<{ path: string; body: any }> = [];
  apiModule.api.post = (async (path: string, body: any) => {
    postCalls.push({ path, body });
    if (path === "/api/v1/feedback/attachments/presign") {
      return {
        grant_id: "grant-1",
        oss_key: "feedback/screenshot-1.png",
        upload_url: "https://uploads.example/screenshot-1.png",
      };
    }
    if (path === "/api/v1/feedback") {
      return { id: 42 };
    }
    throw new Error(`unexpected api.post path ${path}`);
  }) as typeof apiModule.api.post;

  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url === "https://uploads.example/screenshot-1.png") {
      assert.equal(init?.method, "PUT");
      return new Response("upload failed", { status: 500 });
    }
    return new Response("not-found", { status: 404 });
  }) as typeof fetch;

  try {
    const view = renderWithIntl(<FeedbackFormInner user={null} CaptchaComponent={FakeCaptcha} />);

    await typeInto(view.getByRole("combobox") as HTMLSelectElement, "other");
    const textboxes = view.getAllByRole("textbox");
    await typeInto(textboxes[0] as HTMLInputElement, "Upload regression");
    await typeInto(textboxes[1] as HTMLTextAreaElement, "upload failure should not keep grant");
    await typeInto(textboxes[2] as HTMLInputElement, "review@example.com");
    fireEvent.click(view.getByRole("button", { name: "solve captcha" }));

    const fileInput = view.container.querySelector("#feedback-screenshot") as HTMLInputElement | null;
    assert.ok(fileInput, "feedback screenshot input should exist");
    const screenshot = new File(["img"], "failure.png", { type: "image/png" });
    fireEvent.change(fileInput!, { target: { files: [screenshot] } });

    await waitFor(() => assert.match(view.container.textContent ?? "", /Screenshot upload failed/));
    assert.equal(postCalls[0]?.path, "/api/v1/feedback/attachments/presign");
    assert.equal(view.queryByText("failure.png"), null);

    fireEvent.click(view.getByRole("button", { name: "solve captcha" }));
    await waitFor(() => assert.equal(view.getByRole("button", { name: "Submit feedback" }).hasAttribute("disabled"), false));

    const form = view.container.querySelector("form");
    assert.ok(form, "feedback form should exist");
    fireEvent.submit(form);

    await waitFor(() => assert.equal(postCalls.length, 2));
    assert.equal(postCalls[1]?.path, "/api/v1/feedback");
    assert.deepEqual(postCalls[1]?.body.attachment_grants, []);
  } finally {
    apiModule.api.post = originalPost;
    globalThis.fetch = originalFetch;
  }
});
