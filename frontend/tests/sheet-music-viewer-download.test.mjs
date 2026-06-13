import { readFileSync } from "node:fs";
import test from "node:test";
import assert from "node:assert/strict";

const source = readFileSync(
  new URL("../components/content/SheetMusicViewer.tsx", import.meta.url),
  "utf8",
);

test("sheet music downloads do not regress to direct OSS anchors", () => {
  assert.doesNotMatch(
    source,
    /<a\s+href=\{ossUrl\}/,
    "sheet music downloads must not use direct OSS anchors",
  );
});
