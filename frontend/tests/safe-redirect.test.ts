import assert from "node:assert/strict";
import test from "node:test";

import { safeRedirectPath } from "@/lib/safe-redirect";

/* SP-25 FR-05（中-11）：登录 redirect 白名单校验。 */

test("safeRedirectPath only allows in-site paths", () => {
  // 外域形态全部回退 "/"
  assert.equal(safeRedirectPath("//evil.com"), "/");
  assert.equal(safeRedirectPath("///evil.com"), "/");
  assert.equal(safeRedirectPath("https://evil.com"), "/");
  assert.equal(safeRedirectPath("http://evil.com/phish"), "/");
  assert.equal(safeRedirectPath("https://app.leeppp.online.evil.im"), "/");
  assert.equal(safeRedirectPath("\\/evil.com"), "/");
  assert.equal(safeRedirectPath("javascript:alert(1)"), "/");
  assert.equal(safeRedirectPath("evil.com"), "/");
  // 合法站内路径原样放行
  assert.equal(safeRedirectPath("/studio/overview"), "/studio/overview");
  assert.equal(safeRedirectPath("/"), "/");
  assert.equal(safeRedirectPath("/content/123?x=1#frag"), "/content/123?x=1#frag");
  // 空值回退
  assert.equal(safeRedirectPath(""), "/");
  assert.equal(safeRedirectPath(null), "/");
  assert.equal(safeRedirectPath(undefined), "/");
});
