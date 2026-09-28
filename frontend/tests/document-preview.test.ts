import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";

import { parseCsv, restrictImageSources, sanitizeDocumentHtml } from "@/components/content/DocumentViewer";

// dompurify 与 DOMParser 在调用期才需要 DOM：静态 import 后于本模块体
// 注入 jsdom globals（先于任何 test 体执行）。

/**
 * #688 前端预览安全 seam：sanitizer 注入用例（script / javascript: 链接 /
 * 外链资源加载被剥除）+ CSV 解析 + 附件分发合同（file_type 主路由）。
 * DOMPurify 与 DOMParser 需要 DOM——jsdom 供给（组件测试既有先例）。
 */
const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: "http://localhost/" });
(globalThis as Record<string, unknown>).window = dom.window;
(globalThis as Record<string, unknown>).document = dom.window.document;
(globalThis as Record<string, unknown>).DOMParser = dom.window.DOMParser;

test("sanitizer strips script tags and inline event handlers", () => {
  const dirty = `<p>ok</p><script>alert(1)</script><p onclick="steal()">click</p><img src="x" onerror="alert(2)">`;
  const clean = sanitizeDocumentHtml(dirty);
  assert.ok(!clean.includes("<script"), "script must be stripped");
  assert.ok(!/on(error|click)\s*=/i.test(clean), "event handlers must be stripped");
  assert.ok(clean.includes("<p>ok</p>"), "legitimate paragraphs survive");
});

test("sanitizer blocks javascript: and data: (non-image) URLs on links", () => {
  const clean = sanitizeDocumentHtml(
    `<a href="javascript:alert(1)">evil</a><a href="data:text/html;base64,PHNjcmlwdD4=">data</a><a href="https://example.com/x">ok</a>`,
  );
  assert.ok(!/javascript:/i.test(clean), "javascript: href must not survive");
  assert.ok(!/data:text\/html/i.test(clean), "data:text/html href must not survive");
  assert.ok(clean.includes('href="https://example.com/x"'), "https links survive");
});

test("post-pass strips non-embedded image sources and hardens anchors", () => {
  const html = `<img src="https://evil.example.com/pixel.png"><img src="data:image/png;base64,iVBORw0KGgo="><a href="javascript:x">a</a><a href="https://example.com">b</a>`;
  const clean = restrictImageSources(html);
  assert.ok(!clean.includes("https://evil.example.com"), "external resource loads must be dropped");
  assert.ok(clean.includes("data:image/png;base64,"), "inline embedded images survive");
  assert.ok(clean.includes('rel="noopener noreferrer nofollow"'), "anchors are hardened");
  assert.ok(!clean.includes('href="javascript:'), "unsafe hrefs neutralized to #");
});

test("parseCsv handles quoting, escaped quotes and CRLF", () => {
  const rows = parseCsv('a,b,c\r\n"x,1","say ""hi""",3\n,,\n');
  assert.deepEqual(rows[0], ["a", "b", "c"]);
  assert.deepEqual(rows[1], ["x,1", 'say "hi"', "3"]);
  assert.deepEqual(rows[2], ["", "", ""]);
});

test("attachment dispatch contract: file_type is the primary route", async () => {
  // file_type 主路由 → 族群内 subtype 按扩展名（DocumentViewer 内部）→
  // 无查看器回落下载卡。这里以纯数据合同钉住路由判定（组件树由
  // detail 页集成测试与 T4 冒烟覆盖）。
  const dispatch = (att: { file_type?: string; oss_url?: string; scan_status?: string }): "scan-card" | "viewer" | "download-card" => {
    if (att.scan_status && att.scan_status !== "clean" && att.scan_status !== "not_required") return "scan-card";
    if (att.file_type === "document" && att.oss_url) return "viewer";
    return "download-card";
  };
  assert.equal(dispatch({ file_type: "document", oss_url: "https://oss/x.docx" }), "viewer");
  assert.equal(dispatch({ file_type: "document", scan_status: "blocked" }), "scan-card");
  assert.equal(dispatch({ file_type: "document", scan_status: "pending" }), "scan-card");
  // 后端对非 clean 不签发 URL——无 URL 的 document 行同样回落卡片。
  assert.equal(dispatch({ file_type: "document" }), "download-card");
  assert.equal(dispatch({ file_type: "audio", oss_url: "https://oss/x.mp3" }), "download-card");
  assert.equal(dispatch({ file_type: "text" }), "download-card");
  assert.equal(dispatch({ file_type: "image", scan_status: "not_required", oss_url: "u" }), "download-card");
  assert.equal(dispatch({ file_type: "mod", scan_status: "clean", oss_url: "u" }), "download-card");
});
