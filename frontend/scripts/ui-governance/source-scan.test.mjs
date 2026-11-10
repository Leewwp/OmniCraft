import assert from "node:assert/strict";
import test from "node:test";

import { maskedRegions, lineAt } from "./source-scan.mjs";

// #967：掩码状态机回归——三类曾真实咬人的形态（模板插值、正则内反引号、
// JSX `/>` 斜杠）全部锁定：插值内的 t()/绑定必须落在掩码外。
// 注意：字符串**内容**一律掩码（掩码只用于命中起点过滤，字面量从原始
// 源码读取），断言因此只看调用形态不看实参内容。

function maskedText(source) {
  const masked = maskedRegions(source);
  const chars = source.split("");
  for (const [start, end] of masked) {
    for (let i = start; i < end; i++) {
      if (chars[i] !== "\n") chars[i] = " ";
    }
  }
  return chars.join("");
}

test("template expression contents are code, not mask", () => {
  const source = "const s = `prefix ${t('key')} and ${`nested ${u('k2')}`} tail`;";
  const visible = maskedText(source);
  assert.ok(visible.includes("t("), "插值内的调用是代码，不掩码");
  assert.ok(visible.includes("u("), "嵌套模板插值内的调用是代码");
  assert.ok(!visible.includes("prefix"), "模板字面 chunk 被掩码");
  assert.ok(!visible.includes("tail"), "模板尾部 chunk 被掩码");
});

test("regex literals containing backticks and slashes do not open templates", () => {
  const source = [
    "function strip(md) {",
    "  return md",
    '    .replace(/```[\\s\\S]*?```/g, " ")',
    '    .replace(/[#>*_~`|-]+/g, " ")',
    '    .replace(/\\s+/g, " ");',
    "}",
  ].join("\n");
  const visible = maskedText(source);
  assert.ok(visible.includes('" ")'), "正则后的字符串按代码处理（未被模板态吞掉）");
  // 正则字面量内容本身被掩码（含反引号），不污染后续状态。
  assert.ok(!visible.includes("[\\s\\S]*?"), "正则内容被掩码");
});

test("JSX self-close and text slashes stay division-position", () => {
  const source = [
    "<div>",
    '  <EmptyState title={t("a.empty")} />',
    '  <p>{t("a.x")} / {t("a.y")}</p>',
    "</div>",
  ].join("\n");
  const visible = maskedText(source);
  assert.equal((visible.match(/\bt\(/g) || []).length, 3, "三处调用全在掩码外");
  assert.ok(visible.includes("EmptyState title="), "自闭合标签斜杠不吞标签");
});

test("comments and string contents are masked; apostrophes are not string openers", () => {
  const source = [
    "// don't call t('fake.key') here",
    "const note = 't(\"fake.key\")';",
    "const label = pick(t) ? \"ok\" : \"no\";",
    "t2('real');",
  ].join("\n");
  const visible = maskedText(source);
  assert.ok(!visible.includes("t('fake"), "行注释整体掩码（含其中的调用形态）");
  assert.ok(visible.includes("pick(t)"), "注释后的代码不掩码");
  assert.ok(visible.includes("t2("), "字符串后的代码不掩码");
});

test("lineAt counts 1-based lines", () => {
  const source = "a\nbb\nccc";
  assert.equal(lineAt(source, 0), 1);
  assert.equal(lineAt(source, 1), 1);
  assert.equal(lineAt(source, 2), 2);
  assert.equal(lineAt(source, 5), 3);
});
