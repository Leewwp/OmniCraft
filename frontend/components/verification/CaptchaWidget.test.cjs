const fs = require("fs");
const path = require("path");
const assert = require("assert");

const source = fs.readFileSync(path.join(__dirname, "CaptchaWidget.tsx"), "utf8");

assert.match(
  source,
  /initAliyunCaptcha/,
  "CaptchaWidget must initialize Alibaba Cloud CAPTCHA 2.0 with initAliyunCaptcha",
);
assert.match(
  source,
  /captchaVerifyParam/,
  "CaptchaWidget must pass captchaVerifyParam to onToken",
);
assert.doesNotMatch(
  source,
  /sessionId[\s\S]*sig[\s\S]*ncsig|ncsig[\s\S]*sessionId/,
  "CaptchaWidget must not rebuild legacy sessionId|sig|token|ncsig tokens",
);
