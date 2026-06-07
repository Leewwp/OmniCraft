import fs from "node:fs";
import path from "node:path";
import assert from "node:assert/strict";
import { test } from "node:test";

const registerSource = fs.readFileSync(
  path.join(process.cwd(), "app", "(public)", "register", "page.tsx"),
  "utf8",
);

test("register page does not send a register request when aliyun_v2 captcha token is empty", () => {
  const guardIndex = registerSource.indexOf('config.captcha.provider === "aliyun_v2" && !captchaToken');
  const postIndex = registerSource.indexOf('api.post<RegisterResponse>("/api/v1/auth/register"');

  assert.notEqual(guardIndex, -1, "register page must guard empty aliyun_v2 captcha tokens locally");
  assert.notEqual(postIndex, -1, "register page must still submit to the register endpoint after validation");
  assert.ok(guardIndex < postIndex, "the empty-token guard must run before api.post");
});

test("register page exposes stable captcha and submit button ids", () => {
  assert.match(registerSource, /buttonId=\{REGISTER_SUBMIT_BUTTON_ID\}/);
  assert.match(registerSource, /id=\{REGISTER_SUBMIT_BUTTON_ID\}/);
  assert.match(registerSource, /const REGISTER_SUBMIT_BUTTON_ID = "register-submit-button"/);
  assert.match(registerSource, /containerId=\{REGISTER_CAPTCHA_CONTAINER_ID\}/);
  assert.match(registerSource, /const REGISTER_CAPTCHA_CONTAINER_ID = "register-captcha-container"/);
});
