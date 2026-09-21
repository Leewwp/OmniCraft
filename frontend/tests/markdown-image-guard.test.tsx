import assert from "node:assert/strict";
import test from "node:test";
import React from "react";

import { isAllowedImageSrc } from "@/lib/image-guard";
import { MarkdownRenderer } from "@/components/content/MarkdownRenderer";

import { cleanup, installDom, renderWithIntl } from "./runtime-test-helpers";

test.before(() => installDom());
test.afterEach(() => cleanup());

/* SP-25 FR-07（中-1 渲染层兜底）：流式窗口内非平台图源不加载。 */

test("isAllowedImageSrc whitelists platform sources only", () => {
  assert.equal(isAllowedImageSrc("/api/img/1.png", ""), true, "relative path");
  assert.equal(isAllowedImageSrc("https://bucket.oss-cn-hangzhou.aliyuncs.com/a.png", ""), true, "aliyuncs baseline");
  assert.equal(isAllowedImageSrc("https://cdn.omnicraft.local/a.png", "cdn.omnicraft.local"), true, "configured oss domain");
  assert.equal(isAllowedImageSrc("https://cdn.omnicraft.local/a.png", ""), false, "oss domain not yet loaded");
  assert.equal(isAllowedImageSrc("https://evil.example.org/cat.png", "cdn.omnicraft.local"), false, "external host");
  assert.equal(isAllowedImageSrc("https://evil.example.org/cat.png?x=.png", ""), false, "extension-spoofed query");
  assert.equal(isAllowedImageSrc("data:image/png;base64,AAAA", ""), false, "data URI");
  assert.equal(isAllowedImageSrc(undefined, ""), false, "missing src");
  assert.equal(isAllowedImageSrc("//evil.example.org/a.png", ""), false, "protocol-relative external");
});

test("MarkdownRenderer replaces non-platform images with a placeholder", () => {
  const { container, queryByText } = renderWithIntl(
    <MarkdownRenderer content={"外链 ![cat](https://evil.example.org/cat.png) 与平台 ![ok](https://bucket.oss-cn-hangzhou.aliyuncs.com/ok.png)"} />,
  );
  const html = container.innerHTML;
  assert.equal(queryByText("Image blocked: non-platform source") !== null, true, "placeholder text must render");
  assert.equal(html.includes("evil.example.org"), false, "external src must not appear in the DOM");
  assert.equal(html.includes("bucket.oss-cn-hangzhou.aliyuncs.com/ok.png"), true, "platform image must load");
});
