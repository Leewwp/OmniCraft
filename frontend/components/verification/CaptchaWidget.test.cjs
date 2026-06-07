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
assert.match(
  source,
  /button:\s*`#\$\{buttonId\}`/,
  "CaptchaWidget must pass a stable button selector to Alibaba Cloud CAPTCHA 2.0",
);
assert.match(
  source,
  /if\s*\(\s*config\.captcha\.provider\s*===\s*"bypass"\s*\)[\s\S]*onToken\("bypass"\)/,
  "CaptchaWidget may only issue a bypass token when the public config provider is bypass",
);
assert.doesNotMatch(
  source,
  /config\.captcha\.provider\s*===\s*"aliyun_v2"[\s\S]*onToken\("bypass"\)/,
  "CaptchaWidget must not fall back to bypass for aliyun_v2",
);
assert.doesNotMatch(
  source,
  /sessionId[\s\S]*sig[\s\S]*ncsig|ncsig[\s\S]*sessionId/,
  "CaptchaWidget must not rebuild legacy sessionId|sig|token|ncsig tokens",
);
