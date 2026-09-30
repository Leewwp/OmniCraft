import test from "node:test";
import assert from "node:assert/strict";

import { usageGuidePanelTarget } from "@/components/agent/UsageGuidePanel";

/* #723 站内使用指导面板适用范围（判定优先级：feature gate / published 由
 * 调用方叠加；此处钉品类 × 附件判定合同）。 */

test("direct target types always show the panel", () => {
  for (const type of ["mod", "template", "3d_print", "sheet_music"]) {
    assert.equal(usageGuidePanelTarget(type, undefined), true, `${type} shows with no attachments`);
    assert.equal(usageGuidePanelTarget(type, []), true);
  }
});

test("article never shows the panel, even with anomalous document attachments", () => {
  assert.equal(usageGuidePanelTarget("article", undefined), false);
  assert.equal(usageGuidePanelTarget("article", [{ file_type: "document" }]), false, "historical anomaly data stays hidden");
});

test("non-target types show only with a document-family attachment", () => {
  assert.equal(usageGuidePanelTarget("video", [{ file_type: "document" }]), true);
  assert.equal(usageGuidePanelTarget("other", [{ file_type: "document" }]), true);
  assert.equal(usageGuidePanelTarget("video", [{ file_type: "image" }]), false);
  assert.equal(usageGuidePanelTarget("prompt", [{ file_type: "text" }]), false, "text-family readme files do not trigger");
  assert.equal(usageGuidePanelTarget("other", undefined), false);
});

test("mixed attachments satisfy on any document row", () => {
  assert.equal(
    usageGuidePanelTarget("video", [{ file_type: "text" }, { file_type: "document" }, { file_type: "model3d" }]),
    true,
  );
});
