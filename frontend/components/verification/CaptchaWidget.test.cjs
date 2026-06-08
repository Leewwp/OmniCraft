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
  /api\.post<CaptchaVerifyResponse>\("\/api\/v1\/captcha\/verify"/,
  "CaptchaWidget must send captchaVerifyParam to the backend captcha verify endpoint",
);
assert.match(
  source,
  /captcha_verify_param:\s*captchaVerifyParam/,
  "CaptchaWidget must submit the raw Aliyun captchaVerifyParam to the backend",
);
assert.match(
  source,
  /onToken\(verifyResult\.captcha_token\)/,
  "CaptchaWidget must pass the server-issued captcha_token to onToken",
);
assert.doesNotMatch(
  source,
  /onToken\(captchaVerifyParam\)/,
  "CaptchaWidget must not treat the Aliyun callback param as the application token",
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
assert.match(
  source,
  /autoRefresh:\s*false/,
  "CaptchaWidget must disable Aliyun autoRefresh after successful verification",
);
