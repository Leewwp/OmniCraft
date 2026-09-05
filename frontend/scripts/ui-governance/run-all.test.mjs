import assert from "node:assert/strict";
import test from "node:test";

import {
  checkNativeDialogs,
  checkUndefinedTokenReferences,
  definedCustomProperties,
  runAllChecks,
} from "./run-all.mjs";

test("native-dialog check flags confirm/prompt/alert through window, globalThis, and bare scope", () => {
  const sources = {
    "app/ok.tsx": "const ok = <ConfirmModal open={true} />;",
    "components/bad.tsx": [
      "export function Del() {",
      "  if (window.confirm('del?')) remove();",
      "}",
      "const name = prompt('name');",
      "alert('boom');",
      "globalThis.alert('x');",
      "window['confirm']('again');",
    ].join("\n"),
  };
  assert.deepEqual(checkNativeDialogs(sources), [
    { filePath: "components/bad.tsx", line: 2, kind: "confirm" },
    { filePath: "components/bad.tsx", line: 4, kind: "prompt" },
    { filePath: "components/bad.tsx", line: 5, kind: "alert" },
    { filePath: "components/bad.tsx", line: 6, kind: "alert" },
    { filePath: "components/bad.tsx", line: 7, kind: "confirm" },
  ]);
});

test("native-dialog check tolerates string literals and member methods named like dialogs", () => {
  const sources = {
    "components/ok.tsx": [
      "const hint = 'window.confirm is forbidden';",
      "export const dialog = { confirm: () => true };",
      "export function Use() { return dialog.confirm(); }",
    ].join("\n"),
  };
  assert.deepEqual(checkNativeDialogs(sources), []);
});

test("definedCustomProperties collects @theme and :root declarations", () => {
  const css =
    "@theme inline { --font-sans: Arial; --color-primary: var(--primary); } :root { --primary: #fff; }";
  assert.deepEqual([...definedCustomProperties(css)].sort(), [
    "--color-primary",
    "--font-sans",
    "--primary",
  ]);
});

test("undefined-token check flags unknown var() references and honors runtime prefixes", () => {
  const css = ":root { --defined: #fff; --tag-blue-bg: #eef2ff; }";
  const sources = {
    'components/a.tsx': "style={{ color: 'var(--defined)' }} className=\"bg-[var(--tag-blue-bg)]\"",
    'components/b.tsx': "style={{ color: 'var(--accent-hover)' }}",
    'components/c.tsx': "style={{ transform: 'var(--tw-scale-x)' }}",
  };
  assert.deepEqual(checkUndefinedTokenReferences(sources, css), [
    { filePath: "components/b.tsx", token: "--accent-hover" },
  ]);
});

test("repository-wide UI governance gate passes on the current baseline", async () => {
  const result = await runAllChecks();
  const failed = result.checks.filter((check) => check.errors.length > 0);
  assert.deepEqual(failed.map((check) => check.name), []);
  assert.deepEqual(
    result.checks.map((check) => check.name),
    ["tokens", "i18n parity", "i18n key references", "native dialogs", "undefined tokens", "source policy"],
  );
  assert.deepEqual(Object.keys(result.measure).sort(), [
    "ariaLabels",
    "colors",
    "hardCodedText",
    "touchTargets",
  ]);
});
