import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";

const source = readFileSync(
  new URL("../components/content/SheetMusicViewer.tsx", import.meta.url),
  "utf8",
);

test("sheet music attachments without preview URLs still render authorized download CTAs", () => {
  assert.match(
    source,
    /const previewable = attachments\.find\(\(a\) => \{[\s\S]*Boolean\(a\.oss_url\)[\s\S]*type === "pdf"/,
    "preview candidates should require oss_url so oss_key-only attachments remain downloadable",
  );
  assert.match(
    source,
    /const downloadOnlyAttachments = attachments\.filter\(\(a\) => a !== previewable\)/,
    "non-previewable attachments should be kept in a download-only list",
  );
  assert.match(
    source,
    /downloadOnlyAttachments\.map\(\(att\) => \([\s\S]*<AttachmentRow/,
    "download-only sheet music attachments should render AttachmentRow with DownloadButton",
  );
});

test("sheet music preview attachments keep an authorized download CTA", () => {
  assert.match(
    source,
    /function PreviewDownloadButton/,
    "previewable MIDI or MusicXML attachments should have a separate authorized download CTA",
  );
  assert.doesNotMatch(
    source,
    /<a\s+href=\{ossUrl\}/,
    "sheet music downloads must not use direct OSS anchors",
  );
});
