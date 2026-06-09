import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";

const uploaderSource = readFileSync(
  new URL("../components/content/FileUploader.tsx", import.meta.url),
  "utf8",
);
const publishFormSource = readFileSync(
  new URL("../components/studio/PublishForm.tsx", import.meta.url),
  "utf8",
);

test("content uploader preserves backend upload grant id with uploaded assets", () => {
  assert.match(uploaderSource, /grant_id: string;/, "OSS upload token must include grant_id");
  assert.match(uploaderSource, /grantId: string;/, "UploadedAsset must expose grantId");
  assert.match(uploaderSource, /grantId: token\.grant_id/, "FileUploader must store token.grant_id");
});

test("publish form submits attachment grant ids instead of trusting raw OSS keys alone", () => {
  assert.match(
    publishFormSource,
    /grant_id: f\.grantId/,
    "PublishForm attachments must include grant_id for backend publish validation",
  );
});
