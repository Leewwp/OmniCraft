import assert from "node:assert/strict";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

import {
  discoverTestFiles,
  groupTestFiles,
  runMode,
  runTestFiles,
} from "./run-tests.mjs";

async function withFixture(files, callback) {
  const root = await mkdtemp(path.join(os.tmpdir(), "omnicraft-test-discovery-"));

  try {
    await Promise.all(
      files.map(async ([relativePath, content = ""]) => {
        const filePath = path.join(root, relativePath);
        await mkdir(path.dirname(filePath), { recursive: true });
        await writeFile(filePath, content, "utf8");
      }),
    );
    await callback(root);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
}

test("discovers every supported unit-test extension recursively in stable order", async () => {
  await withFixture(
    [
      ["z.test.tsx"],
      ["nested/f.test.ts"],
      ["nested/e.test.jsx"],
      ["nested/d.test.js"],
      ["c.test.mjs"],
      ["b.test.cjs"],
      ["a.test.txt"],
      ["ignored.spec.ts"],
    ],
    async (root) => {
      const discovered = await discoverTestFiles(root);
      const relative = discovered.map((file) =>
        path.relative(root, file).replaceAll(path.sep, "/"),
      );

      assert.deepEqual(relative, [
        "b.test.cjs",
        "c.test.mjs",
        "nested/d.test.js",
        "nested/e.test.jsx",
        "nested/f.test.ts",
        "z.test.tsx",
      ]);
      assert.ok(discovered.every((file) => !file.includes("\\")));
    },
  );
});

test("excludes dependency, build, report, coverage, and e2e directories", async () => {
  await withFixture(
    [
      ["kept.test.ts"],
      ["node_modules/dependency.test.ts"],
      [".next/generated.test.js"],
      ["coverage/instrumented.test.js"],
      ["playwright-report/report.test.mjs"],
      ["test-results/result.test.cjs"],
      ["e2e/browser.test.ts"],
    ],
    async (root) => {
      const discovered = await discoverTestFiles(root);
      assert.deepEqual(discovered, [
        path.join(root, "kept.test.ts").replaceAll(path.sep, "/"),
      ]);
    },
  );
});

test("groups native and TypeScript tests without changing their stable order", () => {
  const grouped = groupTestFiles([
    "a.test.cjs",
    "b.test.js",
    "c.test.jsx",
    "d.test.mjs",
    "e.test.ts",
    "f.test.tsx",
  ]);

  assert.deepEqual(grouped, {
    native: ["a.test.cjs", "b.test.js", "c.test.jsx", "d.test.mjs"],
    typed: ["e.test.ts", "f.test.tsx"],
  });
});

test("returns the child test command failure code", () => {
  const calls = [];
  const status = runTestFiles(
    { native: ["failing.test.mjs"], typed: [] },
    (...args) => {
      calls.push(args);
      return { status: 7 };
    },
  );

  assert.equal(status, 7);
  assert.deepEqual(calls, [
    [
      process.execPath,
      ["--test", "failing.test.mjs"],
      { shell: false, stdio: "inherit" },
    ],
  ]);
});

test("runs TypeScript tests serially to avoid resource-contention flakes", () => {
  const calls = [];
  const status = runTestFiles(
    { native: [], typed: ["component.test.tsx"] },
    (...args) => {
      calls.push(args);
      return { status: 0 };
    },
  );

  assert.equal(status, 0);
  assert.deepEqual(calls, [
    [
      process.execPath,
      ["--import", "tsx", "--test", "--test-concurrency=1", "component.test.tsx"],
      { shell: false, stdio: "inherit" },
    ],
  ]);
});

test("rejects unsupported runner modes", async () => {
  await assert.rejects(() => runMode("browser"), /unknown test mode: browser/i);
});
