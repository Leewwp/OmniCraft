import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { uploadFamilyHints, type PublicConfig } from "@/lib/public-config";
import { FileUploader } from "@/components/content/FileUploader";
import { cleanup, fireEvent, render } from "./runtime-test-helpers";

/* #726 发布页可上传类型清单：
 * 1. uploadFamilyHints 投影合同（注册表行 → 逐族扩展名/上限/必传标记；
 *    required_any_of = "至少命中其中一族"语义）；
 * 2. FALLBACK 兜底含 model3d 族与 3d_print 行（投影缺失不静默丢提示）；
 * 3. FileUploader 分组提示渲染（含必传标记；不传 familyHints 维持旧行为）。
 */

const baseConfig = {
  features: {} as PublicConfig["features"],
  captcha: {} as PublicConfig["captcha"],
  client: {} as PublicConfig["client"],
  legal: {} as PublicConfig["legal"],
  upload: {} as PublicConfig["upload"],
  collaboration: {} as PublicConfig["collaboration"],
  oss_domain: "",
};

const projectedConfig = {
  ...baseConfig,
  content_types: [
    { key: "3d_print", zones: ["original", "fanwork"], form: "file", upload_file_types: ["model3d", "text"], judge_eligible: true, attachment_policy: { required_any_of: ["model3d"] } },
    { key: "template", zones: ["original"], form: "file", upload_file_types: ["text", "document", "model3d"], judge_eligible: true },
  ],
  upload_file_types: [
    { key: "model3d", extensions: [".stl", ".obj", ".3mf", ".gcode", ".ply", ".mtl"], max_mb: 50 },
    { key: "text", extensions: null, max_mb: 10 },
    { key: "document", extensions: [".docx", ".xlsx", ".csv"], max_mb: 20 },
  ],
} as unknown as PublicConfig;

test("uploadFamilyHints: registry projection with per-family limits and required flag", () => {
  const hints = uploadFamilyHints(projectedConfig, "3d_print");
  assert.deepEqual(hints, [
    { key: "model3d", extensions: [".stl", ".obj", ".3mf", ".gcode", ".ply", ".mtl"], maxMB: 50, required: true },
    { key: "text", extensions: null, maxMB: 10, required: false },
  ], "order follows the registry row; required marks only the required_any_of family");

  const template = uploadFamilyHints(projectedConfig, "template");
  assert.equal(template.length, 3);
  assert.ok(template.every((hint) => !hint.required), "no policy on template → nothing marked required");
});

test("uploadFamilyHints: fallback entries carry model3d and the 3d_print policy", () => {
  const hints = uploadFamilyHints(null, "3d_print");
  assert.equal(hints[0].key, "model3d", "model3d never silently missing on the fallback path");
  assert.deepEqual(hints[0].extensions, [".stl", ".obj", ".3mf", ".gcode", ".ply", ".mtl"]);
  assert.equal(hints[0].maxMB, 50);
  assert.equal(hints[0].required, true, "fallback 3d_print keeps required_any_of=[model3d]");
  assert.equal(hints[1].key, "text");

  const template = uploadFamilyHints(null, "template");
  assert.deepEqual(template.map((hint) => hint.key), ["text", "document", "model3d"], "fallback template row includes model3d");
});

test("uploadFamilyHints: unknown content type yields no hints", () => {
  assert.deepEqual(uploadFamilyHints(projectedConfig, "does-not-exist"), []);
});

function renderWithIntl(ui: React.ReactElement) {
  return render(<IntlProvider locale="en" messages={enMessages as never}>{ui}</IntlProvider>);
}

test("FileUploader: grouped hints render per family with required/optional marks", () => {
  const { getByTestId, container } = renderWithIntl(
    <FileUploader familyHints={uploadFamilyHints(projectedConfig, "3d_print")} />,
  );
  const list = getByTestId("upload-family-hints");
  const modelLine = getByTestId("upload-family-hint-model3d").textContent ?? "";
  assert.ok(modelLine.includes(".stl"), "extension whitelist is visible");
  assert.ok(modelLine.includes(".mtl"));
  assert.ok(modelLine.includes("50"), "per-family limit is visible");
  assert.ok(modelLine.includes("At least one required"), "required family is marked");
  const textLine = getByTestId("upload-family-hint-text").textContent ?? "";
  assert.ok(textLine.includes("Text files and PDF"), "no wildcard shown for extension-less families");
  assert.ok(textLine.includes("Optional"));
  assert.ok(!textLine.includes("*"), "client_accept wildcard never leaks as copy");
  assert.equal(container.textContent?.includes("Limit:"), false, "single-limit line replaced when hints exist");
});

test("FileUploader: without familyHints the legacy single-limit line remains", () => {
  const { container, queryByTestId } = renderWithIntl(<FileUploader maxMB={20} />);
  assert.equal(queryByTestId("upload-family-hints"), null);
  assert.ok(container.textContent?.includes("Limit: 20MB"), "legacy hint unaffected for other call sites");
});

test("FileUploader: hint button still opens the file dialog flow (smoke via click)", () => {
  const { getByRole } = renderWithIntl(
    <FileUploader familyHints={uploadFamilyHints(null, "3d_print")} />,
  );
  const button = getByRole("button");
  assert.ok(button.textContent?.includes("Select"));
  fireEvent.click(button);
});

test.afterEach(() => cleanup());
