import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";

const source = readFileSync(new URL("../components/feedback/FeedbackForm.tsx", import.meta.url), "utf8");

test("feedback screenshot upload checks OSS PUT before storing grant", () => {
  const fetchIndex = source.indexOf("fetch(presignRes.upload_url");
  const okCheckIndex = source.indexOf("if (!uploadRes.ok)");
  const setGrantIndex = source.indexOf("setScreenshotGrant({ grant_id: presignRes.grant_id");

  assert.notEqual(fetchIndex, -1, "FeedbackForm should PUT directly to the presigned upload_url");
  assert.notEqual(okCheckIndex, -1, "FeedbackForm should reject failed OSS PUT responses");
  assert.notEqual(setGrantIndex, -1, "FeedbackForm should store grant after a successful upload");
  assert.ok(fetchIndex < okCheckIndex, "upload response should be checked after PUT");
  assert.ok(okCheckIndex < setGrantIndex, "grant must not be stored before response.ok is checked");
});
