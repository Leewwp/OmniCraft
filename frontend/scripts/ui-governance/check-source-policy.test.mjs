import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { PRODUCTION_ROOTS } from "./run-all.mjs";
import {
  checkRawErrorPolicy,
  measureSourcePolicy,
  RAW_RENDER_PATTERNS,
} from "./check-source-policy.mjs";

test("raw-error gate flags every audited rendering pattern", () => {
  const sources = {
    "components/ok.tsx": "const message = getUserFacingErrorKey(err);",
    "components/bad1.tsx": "toast('error', error.message || 'fallback');",
    "components/bad2.tsx": "const message = err instanceof ApiRequestError ? err.message : null;",
    "components/bad3.tsx": "setError(`${err.code}: ${err.message}`);",
    "components/bad4.tsx": "toast('error', err.message);",
  };
  assert.deepEqual(checkRawErrorPolicy(sources), [
    { filePath: "components/bad1.tsx", pattern: "error-message-union" },
    { filePath: "components/bad2.tsx", pattern: "apirequesterror-instanceof" },
    { filePath: "components/bad3.tsx", pattern: "code-message-interpolation" },
    { filePath: "components/bad4.tsx", pattern: "toast-error-message" },
  ]);
});

test("raw-error gate ignores console logging and safe mapping helpers", () => {
  const sources = {
    "lib/logger.ts": "console.error('request failed', err.message);",
    "lib/mapper.ts": "const key = getUserFacingErrorKey(err, fallbackKey);",
  };
  assert.deepEqual(checkRawErrorPolicy(sources), []);
});

test("measured source-policy report counts hard-coded text, aria labels, colors, and touch targets", () => {
  const sources = {
    "app/page.tsx": [
      '<button aria-label="保存" className="h-6 w-8 h-10">Save</button>',
      '<div style={{ color: "#4f46e5" }}>删除</div>',
    ].join("\n"),
    "components/dynamic.tsx":
      '<button aria-label={t("common.close")} className="h-12">{t("common.save")}</button>',
  };
  const report = measureSourcePolicy(sources);
  assert.equal(report.hardCodedText.count, 2);
  assert.equal(report.hardCodedText.files, 1);
  assert.equal(report.ariaLabels.count, 1);
  assert.equal(report.colors.count, 1);
  assert.equal(report.touchTargets.count, 3);
  assert.equal(report.touchTargets.files, 1);
});

test("repository raw-error policy passes on the current baseline", async () => {
  const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
  const frontendRoot = path.resolve(scriptDirectory, "../..");
  const sources = {};
  for (const filePath of await listProductionSources(frontendRoot)) {
    sources[filePath] = await readFile(path.join(frontendRoot, filePath), "utf8");
  }
  assert.deepEqual(checkRawErrorPolicy(sources), []);
  assert.equal(RAW_RENDER_PATTERNS.length, 4);
});

async function listProductionSources(frontendRoot) {
  const files = [];
  async function visit(directory) {
    const entries = await readdir(path.join(frontendRoot, directory), { withFileTypes: true });
    for (const entry of entries) {
      const relativePath = `${directory}/${entry.name}`;
      if (entry.isDirectory()) {
        await visit(relativePath);
      } else if (
        /\.(ts|tsx)$/.test(entry.name) &&
        !/\.(?:test|spec)\.(ts|tsx|js|mjs|cjs)$/.test(entry.name) &&
        !/\.d\.ts$/.test(entry.name)
      ) {
        files.push(relativePath);
      }
    }
  }
  for (const root of PRODUCTION_ROOTS) {
    await visit(root);
  }
  return files.sort();
}
