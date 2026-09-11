import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import {
  findUndefinedCustomPropertyReferences,
  parseMarkdownTables,
  validateTokenContract,
} from "./check-tokens.mjs";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(scriptDirectory, "../..");
const repositoryRoot = path.resolve(frontendRoot, "..");
const designSystemPath = path.join(repositoryRoot, "design/design-system.md");
const globalsPath = path.join(frontendRoot, "app/globals.css");

async function readAuthority() {
  const [markdown, css] = await Promise.all([
    readFile(designSystemPath, "utf8"),
    readFile(globalsPath, "utf8"),
  ]);
  return { markdown, css };
}

test("approved U-01 tables are emitted exactly by globals.css", async () => {
  const { markdown, css } = await readAuthority();
  const tables = parseMarkdownTables(markdown);

  assert.deepEqual(
    tables.get("间距")?.map(({ 用途, 值 }) => [用途, 值]),
    [
      ["卡片内边距", "12px"],
      ["区块内边距", "16px"],
      ["详情卡内边距", "20px / 24px"],
      ["页面 gutter", "16px (mobile) / 24px (desktop)"],
      ["网格 gap", "16px"],
      ["区块间距", "24px / 32px"],
    ],
  );

  assert.deepEqual(validateTokenContract(markdown, css), []);

  const brokenTagCss = css.replace("--tag-blue-bg: #eef2ff;", "--tag-blue-bg: #ffffff;");
  assert.match(
    validateTokenContract(markdown, brokenTagCss).join("\n"),
    /--tag-blue-bg light mismatch/,
  );
});

test("the approved radius, Chinese sans fallbacks, and elevation scale stay explicit", async () => {
  const { markdown, css } = await readAuthority();
  const tables = parseMarkdownTables(markdown);

  assert.deepEqual(
    tables.get("圆角")?.map(({ Token, 值 }) => [Token, 值]),
    [
      ["--radius-sm", "3px"],
      ["--radius-md", "8px"],
      ["--radius-lg", "8px"],
      ["--radius-xl", "12px"],
      ["--radius-full", "9999px"],
    ],
  );
  assert.match(css, /--font-sans:[^;]*'PingFang SC'[^;]*'Microsoft YaHei'/);
  assert.deepEqual(
    tables.get("微阴影")?.map(({ Token }) => Token),
    ["--elevation-1", "--elevation-2", "--elevation-3"],
  );
  assert.match(css, /--shadow-sm:\s*var\(--elevation-1\)/);
  assert.match(css, /--shadow-md:\s*var\(--elevation-3\)/);
  assert.match(css, /--shadow-lg:\s*var\(--elevation-3\)/);
  assert.match(css, /@media\s*\(prefers-reduced-motion:\s*reduce\)/);
  assert.match(css, /\[class\*="animate-pulse"\][\s\S]*animation:\s*none\s*!important/);
  assert.match(css, /\[class\*="hover:scale-"\]:hover[\s\S]*--tw-scale-x:\s*1\s*!important/);
  assert.match(css, /\[class\*="hover:translate-x-"\]:hover[\s\S]*--tw-translate-x:\s*0\s*!important/);
  assert.match(css, /\[class\*="hover:translate-y-"\]:hover[\s\S]*--tw-translate-y:\s*0\s*!important/);
});

test("undefined custom-property references fail governance validation", () => {
  const css = ":root { --defined: #fff; --consumer: var(--missing); }";
  assert.deepEqual(findUndefinedCustomPropertyReferences(css), ["--missing"]);
});
