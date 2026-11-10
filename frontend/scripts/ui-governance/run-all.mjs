import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { I18N_NAMESPACES, validateI18nParity } from "./check-i18n-parity.mjs";
import { validateI18nKeyReferences } from "./check-i18n-keys.mjs";
import { checkRawErrorPolicy, measureSourcePolicy } from "./check-source-policy.mjs";
import { validateTokenContract } from "./check-tokens.mjs";
import { maskedRegions, lineAt } from "./source-scan.mjs";

// Production source set governed by the UI gates. Root-level dev files such as
// proxy.ts (Next dev-time proxy) and types/ declarations are intentionally
// excluded: they never render user-facing UI.
export const PRODUCTION_ROOTS = ["app", "components", "contexts", "lib", "i18n"];

// Runtime CSS custom properties injected by frameworks (Tailwind v4, Base UI)
// that are not declared in app/globals.css source.
const RUNTIME_TOKEN_PREFIXES = ["--tw-", "--base-ui-"];

export async function discoverProductionSources(frontendRoot) {
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

// 三形态（与 AST 版 dialogKind 逐点对齐）：裸标识符 `confirm(`（后瞻排除
// 成员接收者）、`window|globalThis.(?.)confirm(`、`window|globalThis["confirm"](`。
const NATIVE_DIALOG_BARE =
  /(?<![\w$.])(confirm|prompt|alert)\s*\(/g;
const NATIVE_DIALOG_MEMBER =
  /(?<![\w$])(window|globalThis)\s*\??\.\s*(confirm|prompt|alert)\s*\(/g;
const NATIVE_DIALOG_BRACKET =
  /(?<![\w$])(window|globalThis)\s*\[\s*["'](confirm|prompt|alert)["']\s*\]\s*\(/g;

export function checkNativeDialogs(sourceByPath) {
  const findings = [];
  for (const [filePath, source] of Object.entries(sourceByPath)) {
    // 原始源码上扫描，命中起点落在注释/字符串掩码区内的丢弃（文案与
    // 注释里的 "confirm(" 不误报；`window['confirm'](` 起点在掩码外保留）。
    const masked = maskedRegions(source);
    const inMask = (index) =>
      masked.some(([start, end]) => index >= start && index < end);
    const hits = [];
    for (const match of source.matchAll(NATIVE_DIALOG_BARE)) {
      if (!inMask(match.index)) hits.push({ index: match.index, kind: match[1] });
    }
    for (const match of source.matchAll(NATIVE_DIALOG_MEMBER)) {
      if (!inMask(match.index)) hits.push({ index: match.index, kind: match[2] });
    }
    for (const match of source.matchAll(NATIVE_DIALOG_BRACKET)) {
      if (!inMask(match.index)) hits.push({ index: match.index, kind: match[2] });
    }
    // 文档序输出，与 AST 版遍历顺序一致。
    hits.sort((a, b) => a.index - b.index);
    for (const { index, kind } of hits) {
      findings.push({ filePath, line: lineAt(source, index), kind });
    }
  }
  return findings;
}

export function definedCustomProperties(css) {
  const names = new Set();
  for (const match of css.matchAll(/--[a-zA-Z0-9-]+\s*:/g)) {
    names.add(match[0].slice(0, -1).trim());
  }
  return names;
}

export function checkUndefinedTokenReferences(sourceByPath, css) {
  const defined = definedCustomProperties(css);
  const findings = [];
  for (const [filePath, source] of Object.entries(sourceByPath)) {
    for (const match of source.matchAll(/var\(\s*(--[a-zA-Z0-9-]+)/g)) {
      const token = match[1];
      if (defined.has(token)) continue;
      if (RUNTIME_TOKEN_PREFIXES.some((prefix) => token.startsWith(prefix))) continue;
      findings.push({ filePath, token });
    }
  }
  return findings;
}

export async function runAllChecks(frontendRoot = defaultFrontendRoot()) {
  const repositoryRoot = path.resolve(frontendRoot, "..");
  const [designSystem, globalsCss, enJson, zhJson] = await Promise.all([
    readFile(path.join(repositoryRoot, "design/design-system.md"), "utf8"),
    readFile(path.join(frontendRoot, "app/globals.css"), "utf8"),
    readFile(path.join(frontendRoot, "messages/en.json"), "utf8"),
    readFile(path.join(frontendRoot, "messages/zh.json"), "utf8"),
  ]);

  const sources = {};
  for (const relativePath of await discoverProductionSources(frontendRoot)) {
    sources[relativePath] = await readFile(path.join(frontendRoot, relativePath), "utf8");
  }

  const checks = [
    {
      name: "tokens",
      errors: validateTokenContract(designSystem, globalsCss),
    },
    {
      name: "i18n parity",
      errors: validateI18nParity(JSON.parse(enJson), JSON.parse(zhJson), {
        namespaces: I18N_NAMESPACES,
      }),
    },
    {
      // #380/F-A006: referenced-key existence — catches a key missing from
      // BOTH catalogs (the parity counter is blind to that shape).
      name: "i18n key references",
      errors: validateI18nKeyReferences(sources, JSON.parse(zhJson), JSON.parse(enJson)),
    },
    {
      name: "native dialogs",
      errors: checkNativeDialogs(sources).map(
        ({ filePath, line, kind }) => `${filePath}:${line} ${kind}()`,
      ),
    },
    {
      name: "undefined tokens",
      errors: checkUndefinedTokenReferences(sources, globalsCss).map(
        ({ filePath, token }) => `${filePath} references ${token}`,
      ),
    },
    {
      name: "source policy",
      errors: checkRawErrorPolicy(sources).map(
        ({ filePath, pattern }) => `${filePath} matches ${pattern}`,
      ),
    },
  ];

  return {
    checks,
    measure: measureSourcePolicy(sources),
  };
}

function defaultFrontendRoot() {
  return path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
}

async function main() {
  const { checks, measure } = await runAllChecks();

  let failed = false;
  for (const check of checks) {
    if (check.errors.length === 0) {
      console.log(`[ok] ${check.name}`);
    } else {
      failed = true;
      console.error(`[fail] ${check.name}`);
      for (const error of check.errors) {
        console.error(`  - ${error}`);
      }
    }
  }

  console.log(`[measure] hard-coded JSX text: ${measure.hardCodedText.count} in ${measure.hardCodedText.files} file(s)`);
  console.log(`[measure] hard-coded aria-label literals: ${measure.ariaLabels.count} in ${measure.ariaLabels.files} file(s)`);
  console.log(`[measure] color literals: ${measure.colors.count} in ${measure.colors.files} file(s)`);
  console.log(`[measure] compact explicit sizes below 44px: ${measure.touchTargets.count} in ${measure.touchTargets.files} file(s)`);
  console.log("[measure] reports are informational; gate enforcement is pending exception encoding");

  if (failed) {
    console.error("UI governance checks failed.");
    process.exitCode = 1;
    return;
  }
  console.log("UI governance checks passed.");
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
