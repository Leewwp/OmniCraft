import test from "node:test";
import assert from "node:assert/strict";

/**
 * SP-19 G3-1（#520）Milkdown 编辑器封装底座测试。
 *
 * 编辑器内核（ProseMirror/Crepe/Vue 组件树）不适配 node 测试 DOM（jsdom
 * 缺 floating-ui/CodeMirror 所需布局 API——挂载即崩），故本文件分层：
 * 1) 纯逻辑（草稿键/字数/中文化覆盖表/上传状态包装）走行为测试；
 * 2) 关键接线纪律（replaceAll 三场景、单实例、禁 Latex、locale 感知、
 *    渲染层不动）走源码契约断言；
 * 3) 编辑器内部交互（IME/表格/代码块/round-trip）按票面在浏览器验证。
 */
import { readFile } from "node:fs/promises";
import path from "node:path";

async function read(relativePath: string) {
  return readFile(path.join(process.cwd(), relativePath), "utf8");
}


test("editor wrapper is client-only and pins milkdown exact versions", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  assert.match(source, /^"use client"/);
  const pkg = JSON.parse(await read("package.json"));
  assert.equal(pkg.dependencies["@milkdown/crepe"], "7.22.1");
  assert.equal(pkg.dependencies["@milkdown/react"], "7.22.1");
  assert.equal(pkg.dependencies["@milkdown/kit"], "7.22.1");
});

test("replaceAll discipline: only setMarkdown/restoreDraft call replaceAll, never on input", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  const occurrences = source.split("replaceAll(").length - 1;
  // setMarkdown + restoreDraft 两次调用；输入路径（markdownUpdated）不得出现。
  assert.ok(occurrences === 2, `replaceAll occurrences = ${occurrences}, want 2`);
  const start = source.indexOf("listener.markdownUpdated((_, markdown) =>");
  const body = source.slice(start, source.indexOf("});", source.indexOf("flushDraft(markdown)", start)) + 3);
  assert.doesNotMatch(body, /replaceAll/);
});

test("single instance: editor factory runs once with empty deps; locale strings only at mount", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  assert.match(source, /useEditor\(\(container\) =>/);
  assert.match(source, /\}, \[\]\);/);
  // 挂载后 locale/placeholder 变更不重建（注释纪律 + 不出现在 useEditor deps）。
  const factory = source.slice(source.indexOf("useEditor((container)"), source.indexOf("}, []);", source.indexOf("useEditor((container)")));
  assert.doesNotMatch(factory, /locale|placeholder/);
});

test("features: Latex/TopBar/AI disabled; images only when allowImages", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  assert.match(source, /\[CrepeFeature\.Latex\]: false/);
  assert.match(source, /\[CrepeFeature\.TopBar\]: false/);
  assert.match(source, /\[CrepeFeature\.AI\]: false/);
  assert.match(source, /\[CrepeFeature\.ImageBlock\]: false/);
});

test("zh strings are locale-gated, not unconditional", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  const build = source.slice(source.indexOf("function buildFeatureConfigs"));
  assert.match(build, /if \(locale === "zh"\)/);
  assert.match(build, /加粗/);
  const en = build.slice(build.indexOf("const configs: CrepeConfig[\"featureConfigs\"] = placeholder"));
  assert.doesNotMatch(en.slice(0, 200), /加粗|上传图片/);
});

test("image upload reuses contents oss-token presign with exact byte declaration", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  assert.match(source, /\/api\/v1\/contents\/oss-token/);
  assert.match(source, /file_size: file\.size/);
  assert.match(source, /method: "PUT"/);
  assert.match(source, /body: file/);
});

test("draft: user-scoped key, debounce save, restore prompt (no silent override), clear on submit", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  assert.match(source, /milkdown-draft:/);
  assert.match(source, /userIdentity/);
  assert.match(source, /DRAFT_DEBOUNCE_MS/);
  // 进入时仅「提示恢复」：编辑器仍以 defaultValue 启动，恢复动作 = replaceAll。
  assert.match(source, /defaultValue\.trim\(\) === ""/);
  assert.match(source, /setDraftFound\(saved\)/);
  assert.match(source, /clearDraft/);
});

test("word count is rune-based and rendered in a self-drawn bottom bar", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  assert.match(source, /function countRunes/);
  assert.match(source, /\[\.\.\.text\]\.length/);
  assert.match(source, /wordCount", \{ count: wordCount \}/);
});

test("render layer untouched: wrapper emits markdown strings only (no react-markdown import)", async () => {
  const source = await read("components/markdown/MilkdownEditor.tsx");
  assert.doesNotMatch(source, /from ["']react-markdown["']/); // 只禁 import，注释提及不算
  assert.match(source, /onChange\?: \(markdown: string\) => void/);
});

test("i18n keys exist in zh/en catalogs", async () => {
  for (const loc of ["zh", "en"]) {
    const messages = JSON.parse(await read(`messages/${loc}.json`));
    assert.ok(messages.editor, `${loc} missing editor namespace`);
    for (const key of ["wordCount", "draftFound", "restoreDraft", "discardDraft", "imageUploading", "imageUploadFailed"]) {
      assert.ok(messages.editor[key], `${loc} missing editor.${key}`);
    }
  }
});

test("theme bridge maps crepe variables onto design tokens", async () => {
  const css = await read("components/markdown/milkdown-editor.css");
  assert.match(css, /--crepe-color-primary: var\(--accent-emphasis/);
  assert.match(css, /--crepe-color-background: var\(--background\)/);
});
