import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import ts from "typescript";
import { fileURLToPath, pathToFileURL } from "node:url";

import { I18N_NAMESPACES, validateI18nParity } from "./check-i18n-parity.mjs";
import { checkRawErrorPolicy, measureSourcePolicy } from "./check-source-policy.mjs";
import { validateTokenContract } from "./check-tokens.mjs";

// Production source set governed by the UI gates. Root-level dev files such as
// proxy.ts (Next dev-time proxy) and types/ declarations are intentionally
// excluded: they never render user-facing UI.
export const PRODUCTION_ROOTS = ["app", "components", "contexts", "lib", "i18n"];

// Runtime CSS custom properties injected by frameworks (Tailwind v4, Base UI)
// that are not declared in app/globals.css source.
const RUNTIME_TOKEN_PREFIXES = ["--tw-", "--base-ui-"];

const NATIVE_DIALOG_NAMES = new Set(["confirm", "prompt", "alert"]);
const GLOBAL_HOLDERS = new Set(["window", "globalThis"]);

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

// Detects direct native-dialog call sites. Deliberate limitations: an alias
// capture (`const c = window.confirm; c()`) or a shadowed local function named
// `confirm`/`prompt`/`alert` would escape or falsely trip the gate; both are
// documented as intentional exceptions of this strict call-site scan.
export function checkNativeDialogs(sourceByPath) {
  const findings = [];
  for (const [filePath, source] of Object.entries(sourceByPath)) {
    const scriptKind = filePath.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
    // getStart() requires parent pointers on this TypeScript version, so
    // setParentNodes stays enabled even though the traversal itself only
    // reads child nodes.
    const sourceFile = ts.createSourceFile(filePath, source, ts.ScriptTarget.Latest, true, scriptKind);

    function visit(node) {
      if (ts.isCallExpression(node)) {
        const kind = dialogKind(node.expression);
        if (kind) {
          const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart());
          findings.push({ filePath, line: line + 1, kind });
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(sourceFile);
  }
  return findings;
}

function dialogKind(callee) {
  if (ts.isIdentifier(callee) && NATIVE_DIALOG_NAMES.has(callee.text)) {
    return callee.text;
  }
  if (
    ts.isPropertyAccessExpression(callee) &&
    ts.isIdentifier(callee.expression) &&
    GLOBAL_HOLDERS.has(callee.expression.text) &&
    NATIVE_DIALOG_NAMES.has(callee.name.text)
  ) {
    return callee.name.text;
  }
  if (
    ts.isElementAccessExpression(callee) &&
    ts.isIdentifier(callee.expression) &&
    GLOBAL_HOLDERS.has(callee.expression.text) &&
    ts.isStringLiteral(callee.argumentExpression) &&
    NATIVE_DIALOG_NAMES.has(callee.argumentExpression.text)
  ) {
    return callee.argumentExpression.text;
  }
  return null;
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
