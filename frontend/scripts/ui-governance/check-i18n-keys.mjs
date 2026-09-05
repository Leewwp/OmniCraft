import ts from "typescript";

// #380/F-A006: static i18n key-existence gate. Scans production sources for
// next-intl translation references and verifies every statically resolvable
// key exists in both the zh and en catalogs. The zh=en parity counter (check
// -i18n-parity) cannot catch a reference pointing at a key missing from BOTH
// catalogs; #308 and F-A006 are that blind spot's third strike, one of which
// rendered a raw key name to users on the reset-password validation path.
//
// Deliberate limitations (same strict-scan posture as checkNativeDialogs):
// - Only `useTranslations` / `getTranslations` bindings (incl. `await` and
//   `{ namespace: "..." }` object form) feed the variable→namespace map; an
//   alias import or a shadowed local named like a mapped variable escapes.
// - Dynamic keys (template literals with substitution, identifiers, chained
//   expressions) are reported as unresolved and skipped, not errors — the
//   `settings.reputation.reasons.${reason}` pattern is enumerated server-side
//   and cannot be resolved here without duplicating that enum.
// - Member calls are scanned for t.rich/t.raw/t.markup; other members (e.g.
//   t.has) are ignored.

const TRANSLATOR_FACTORIES = new Set(["useTranslations", "getTranslations"]);
const TRANSLATOR_MEMBERS = new Set(["rich", "raw", "markup"]);

function literalText(node) {
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
    return node.text;
  }
  return null;
}

function namespaceFromArgument(arg) {
  if (!arg) return "";
  const direct = literalText(arg);
  if (direct !== null) return direct;
  if (ts.isObjectLiteralExpression(arg)) {
    for (const property of arg.properties) {
      if (!ts.isPropertyAssignment(property)) continue;
      const name = property.name;
      const isNamespace =
        (ts.isIdentifier(name) && name.text === "namespace") ||
        (ts.isStringLiteral(name) && name.text === "namespace");
      if (isNamespace) return literalText(property.initializer);
    }
  }
  return null;
}

// Extracts the call expression behind `await` so
// `const t = await getTranslations("auth")` binds like the sync form.
function unwrapAwait(node) {
  return node && ts.isAwaitExpression(node) ? node.expression : node;
}

function collectNamespaceBindings(sourceFile) {
  const bindings = new Map();
  function visit(node) {
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name)) {
      const initializer = unwrapAwait(node.initializer);
      if (initializer && ts.isCallExpression(initializer)) {
        const callee = initializer.expression;
        const isFactory =
          ts.isIdentifier(callee) && TRANSLATOR_FACTORIES.has(callee.text);
        if (isFactory) {
          const namespace = namespaceFromArgument(initializer.arguments[0]);
          if (namespace !== null) bindings.set(node.name.text, namespace);
        }
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return bindings;
}

// Returns { resolved: [{ filePath, line, key, allowNonLeaf }], unresolved: [{ filePath, line }] }.
export function collectI18nKeyReferences(sourceByPath) {
  const resolved = [];
  const unresolved = [];
  for (const [filePath, source] of Object.entries(sourceByPath)) {
    const scriptKind = filePath.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
    const sourceFile = ts.createSourceFile(filePath, source, ts.ScriptTarget.Latest, true, scriptKind);
    const bindings = collectNamespaceBindings(sourceFile);

    function visit(node) {
      if (ts.isCallExpression(node)) {
        // TS AST names the call target `expression` (not ESTree's `callee`).
        const callee = node.expression;
        let namespace = null;
        let allowNonLeaf = false;
        if (ts.isIdentifier(callee) && bindings.has(callee.text)) {
          namespace = bindings.get(callee.text);
        } else if (
          ts.isPropertyAccessExpression(callee) &&
          ts.isIdentifier(callee.expression) &&
          bindings.has(callee.expression.text) &&
          TRANSLATOR_MEMBERS.has(callee.name.text)
        ) {
          namespace = bindings.get(callee.expression.text);
          // t.raw() legitimately returns a nested object, so its target need
          // not be a leaf; plain t() must resolve to a message.
          allowNonLeaf = callee.name.text === "raw";
        }
        if (namespace !== null) {
          const argument = node.arguments[0];
          const key = argument ? literalText(argument) : null;
          const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart());
          if (key !== null) {
            const fullKey = namespace ? `${namespace}.${key}` : key;
            resolved.push({ filePath, line: line + 1, key: fullKey, allowNonLeaf });
          } else {
            unresolved.push({ filePath, line: line + 1 });
          }
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(sourceFile);
  }
  return { resolved, unresolved };
}

// Both catalogs must carry every referenced key; reports which side is
// missing so a zh-only or en-only addition cannot sneak through. Plain t()
// references must land on a message leaf; t.raw() may target an intermediate
// namespace object, so those are checked against the full path set.
export function validateI18nKeyReferences(sourceByPath, zhCatalog, enCatalog) {
  const zh = flattenPaths(zhCatalog);
  const en = flattenPaths(enCatalog);
  const { resolved } = collectI18nKeyReferences(sourceByPath);
  const errors = [];
  for (const { filePath, line, key, allowNonLeaf } of resolved) {
    const sides = [];
    if (allowNonLeaf) {
      if (!zh.allPaths.has(key)) sides.push("zh");
      if (!en.allPaths.has(key)) sides.push("en");
    } else {
      if (!zh.leaves.has(key)) sides.push("zh");
      if (!en.leaves.has(key)) sides.push("en");
    }
    if (sides.length > 0) {
      errors.push(`${filePath}:${line} missing i18n key ${key} (${sides.join("+")})`);
    }
  }
  return errors;
}

function flattenPaths(value) {
  const leaves = new Set();
  const allPaths = new Set();
  function visit(node, prefix) {
    for (const [key, child] of Object.entries(node)) {
      const path = prefix ? `${prefix}.${key}` : key;
      allPaths.add(path);
      if (child && typeof child === "object" && !Array.isArray(child)) {
        visit(child, path);
      } else {
        leaves.add(path);
      }
    }
  }
  visit(value, "");
  return { leaves, allPaths };
}
