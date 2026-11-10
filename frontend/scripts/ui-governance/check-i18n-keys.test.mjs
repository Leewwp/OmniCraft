import assert from "node:assert/strict";
import test from "node:test";

import {
  collectI18nKeyReferences,
  validateI18nKeyReferences,
} from "./check-i18n-keys.mjs";

const CATALOG = {
  auth: {
    login: "登录",
    errorPasswordMismatch: "两次输入的密码不一致",
  },
  common: { view: "查看" },
  settings: {
    reputation: { reasons: { judge_accuracy: "众裁准确率奖励" } },
  },
};

test("flags a reference whose key is missing from both catalogs (F-A006 shape)", () => {
  const sources = {
    "app/(public)/reset-password/page.tsx": [
      'const t = useTranslations();',
      't("auth.passwordMismatch");',
    ].join("\n"),
  };
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), [
    "app/(public)/reset-password/page.tsx:2 missing i18n key auth.passwordMismatch (zh+en)",
  ]);
});

test("passes when the referenced key exists in both catalogs", () => {
  const sources = {
    "app/page.tsx": [
      'const t = useTranslations();',
      't("auth.errorPasswordMismatch");',
      't("auth.login");',
      't("common.view");',
    ].join("\n"),
  };
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), []);
});

test("names the single offending side for a zh-only key", () => {
  const zh = { common: { save: "保存" } };
  const en = { common: {} };
  const sources = { "components/x.tsx": 'const t = useTranslations();\nt("common.save");' };
  assert.deepEqual(validateI18nKeyReferences(sources, zh, en), [
    "components/x.tsx:2 missing i18n key common.save (en)",
  ]);
});

test("namespace-scoped translator binds and prefixes keys", () => {
  const sources = {
    "components/y.tsx": [
      'const t = useTranslations("settings.reputation.reasons");',
      't("judge_accuracy");',
      't("missing_reason");',
    ].join("\n"),
  };
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), [
    "components/y.tsx:3 missing i18n key settings.reputation.reasons.missing_reason (zh+en)",
  ]);
});

test("getTranslations with object namespace option and await is bound", () => {
  const sources = {
    "app/z/page.tsx": [
      'export async function Head() {',
      '  const t = await getTranslations({ namespace: "auth" });',
      '  return t("login");',
      '}',
    ].join("\n"),
  };
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), []);
});

test("dynamic keys and unmapped calls are skipped, not errors", () => {
  const sources = {
    "components/dyn.tsx": [
      'const t = useTranslations("settings.reputation.reasons");',
      't(`${prefix}.judge_accuracy`);',
      'const locale = pickLocale();',
      'locale("common.view");',
    ].join("\n"),
  };
  const { resolved, unresolved } = collectI18nKeyReferences(sources);
  assert.equal(resolved.length, 0);
  assert.equal(unresolved.length, 1);
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), []);
});

test("t.rich resolves against the bound namespace; non-translator members are ignored", () => {
  const sources = {
    "components/rich.tsx": [
      'const t = useTranslations("auth");',
      't.rich("login", { b: (chunks) => chunks });',
      't.has("login");',
    ].join("\n"),
  };
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), []);
});

test("plain t() must hit a leaf; t.raw() may target a nested namespace", () => {
  const sources = {
    "components/leaf.tsx": [
      'const t = useTranslations();',
      't("settings");',
      't.raw("settings.reputation");',
    ].join("\n"),
  };
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), [
    "components/leaf.tsx:2 missing i18n key settings (zh+en)",
  ]);
});

test("unbound variable named like a translator is not treated as one", () => {
  const sources = {
    "components/shadow.tsx": 'render(t("auth.login"));',
  };
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), []);
});

test("#967 scanner traps: comments, strings, receivers, optional chains, templates", () => {
  const sources = {
    "components/traps.tsx": [
      '// const t = useTranslations("auth"); t("auth.missing");',
      'const t = useTranslations("auth");',
      'const note = \'t("auth.missing")\';',
      't?.("login");',
      't?.raw("settings.reputation");',
      't.has("login");',
      'obj.t("missing");',
      't(`login`);',
      't.rich(count);',
    ].join("\n"),
  };
  const { resolved, unresolved } = collectI18nKeyReferences(sources);
  assert.deepEqual(
    resolved.map(({ line, key, allowNonLeaf }) => ({ line, key, allowNonLeaf })),
    [
      { line: 4, key: "auth.login", allowNonLeaf: false },
      { line: 5, key: "auth.settings.reputation", allowNonLeaf: true },
      { line: 8, key: "auth.login", allowNonLeaf: false },
    ],
  );
  assert.deepEqual(unresolved, [{ filePath: "components/traps.tsx", line: 9 }]);
  // 注释/字符串里的假引用与非枚举成员不产出错误；真实命中按目录校验
  // （auth.settings.reputation 是命名空间前缀路径，目录中不存在 → 报缺失）。
  assert.deepEqual(validateI18nKeyReferences(sources, CATALOG, CATALOG), [
    "components/traps.tsx:5 missing i18n key auth.settings.reputation (zh+en)",
  ]);
});
