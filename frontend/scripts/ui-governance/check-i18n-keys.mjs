import { maskedRegions, lineAt } from "./source-scan.mjs";

// #380/F-A006: static i18n key-existence gate. Scans production sources for
// next-intl translation references and verifies every statically resolvable
// key exists in both the zh and en catalogs. The zh=en parity counter (check
// -i18n-parity) cannot catch a reference pointing at a key missing from BOTH
// catalogs; #308 and F-A006 are that blind spot's third strike, one of which
// rendered a raw key name to users on the reset-password validation path.
//
// #967：解析层不再依赖 typescript 包的 JS AST（TS 7 原生重写版无 JS
// API），改用掩码区间过滤（source-scan.mjs）+ 两遍静态正则：在原始源
// 码上扫描，命中起点落在注释/字符串内容掩码区内的丢弃（字符串里的
// 't("x")' 文案不误报），先收集 translator 绑定再扫描绑定名调用。语义
// 与原 AST 版逐点对齐（304 文件全量等价性对照见 #967 PR 描述），刻意
// 限制姿势不变：
// - Only `useTranslations` / `getTranslations` bindings (incl. `await` and
//   `{ namespace: "..." }` object form) feed the variable→namespace map; an
//   alias import or a shadowed local named like a mapped variable escapes.
// - Dynamic keys (template literals with substitution, identifiers, chained
//   expressions) are reported as unresolved and skipped, not errors — the
//   `settings.reputation.reasons.${reason}` pattern is enumerated server-side
//   and cannot be resolved here without duplicating that enum.
// - Member calls are scanned for t.rich/t.raw/t.markup; other members (e.g.
//   t.has) are ignored.

// `const t = useTranslations("ns")` / `const t = await getTranslations(...)`
// / `const t = getTranslations({ namespace: "ns" })`，含无参形式（绑定到
// 根命名空间 ""）。非字面量命名空间实参（三元/标识符）不绑定——与 AST
// 版 namespaceFromArgument 返回 null 时不入表一致。
const BINDING_PATTERN =
  /(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:await\s+)?(useTranslations|getTranslations)\s*\(\s*(?:"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)'|\{\s*namespace\s*:\s*(?:"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)')\s*\}|(?=[),]))/g;

// 绑定名调用四形态（与 AST 版调用点语义逐点对齐）：
// `t(` / `t?.(` / `t.(rich|raw|markup)(` / `t?.(rich|raw|markup)(`。
// 后瞻断言（?<![\w$.]）排除成员接收者（foo.t(...) 不视为绑定名调用，
// 且 t.has(...) 这类非枚举成员在任一分支都不匹配——AST 版同语义）。
const CALL_PATTERN =
  /(?<![\w$.])([A-Za-z_$][\w$]*)(?:\?\.\s*(rich|raw|markup)\s*\(|\.\s*(rich|raw|markup)\s*\(|\?\.\s*\(|\s*\()/g;

function unescapeLiteral(raw) {
  // StringLiteral.text 的最小等价：本仓 key 均为 ASCII 点分标识符，
  // 仅处理常见转义即可与 AST 全量等价（对照见 #967 PR）。
  return raw.replace(/\\(.)/g, (_, ch) => {
    if (ch === "n") return "\n";
    if (ch === "t") return "\t";
    return ch;
  });
}

// 调用点实参形态 → { key } | { dynamic: true }（与 AST 版 literalText
// 语义一致：字符串/无插值模板字面量为 key，其余（含缺参）一律 dynamic）。
function firstArgumentInfo(scanned, argStart) {
  const rest = scanned.slice(argStart);
  const single = /^\s*'((?:[^'\\]|\\.)*)'/.exec(rest);
  if (single) return { key: unescapeLiteral(single[1]) };
  const double = /^\s*"((?:[^"\\]|\\.)*)"/.exec(rest);
  if (double) return { key: unescapeLiteral(double[1]) };
  const template = /^\s*`((?:[^`\\]|\\.)*)`/.exec(rest);
  if (template && !template[1].includes("${")) {
    return { key: unescapeLiteral(template[1]) };
  }
  return { dynamic: true };
}

// Returns { resolved: [{ filePath, line, key, allowNonLeaf }], unresolved: [{ filePath, line }] }.
export function collectI18nKeyReferences(sourceByPath) {
  const resolved = [];
  const unresolved = [];
  for (const [filePath, source] of Object.entries(sourceByPath)) {
    const masked = maskedRegions(source);
    const inMask = (index) =>
      masked.some(([start, end]) => index >= start && index < end);
    // 两遍静态正则替代 TS AST 遍历：绑定收集与调用扫描都先经掩码过滤
    // （注释示例与字符串文案里的 t()/绑定不参与），文档序扫描，绑定名
    // （含可选链）与 rich/raw/markup 成员调用按 AST 版语义归入
    // resolved / unresolved。
    const bindings = new Map();
    for (const match of source.matchAll(BINDING_PATTERN)) {
      if (inMask(match.index)) continue;
      const name = match[1];
      const namespace = match[3] ?? match[4] ?? match[5] ?? match[6];
      // 后出现者覆盖先出现者——与 AST 版整文件单表、末次绑定生效一致。
      bindings.set(name, namespace ?? "");
    }

    for (const match of source.matchAll(CALL_PATTERN)) {
      if (inMask(match.index)) continue;
      const name = match[1];
      const member = match[2] ?? match[3] ?? null;
      if (!bindings.has(name)) continue;
      const namespace = bindings.get(name);
      const info = firstArgumentInfo(source, match.index + match[0].length);
      const line = lineAt(source, match.index);
      if (info.key !== undefined) {
        const fullKey = namespace ? `${namespace}.${info.key}` : info.key;
        resolved.push({ filePath, line, key: fullKey, allowNonLeaf: member === "raw" });
      } else {
        unresolved.push({ filePath, line });
      }
    }
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
