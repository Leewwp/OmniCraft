import assert from "node:assert/strict";
import test from "node:test";

import {
  FALLBACK_CONTENT_TYPE_ENTRIES,
  clientAcceptForContentType,
  contentTypeEntries,
  contentTypeMap,
  deriveUploadFamilyForExtension,
  formForContentType,
  isFilePrimaryContentType,
  uploadFileTypeEntries,
  uploadFileTypeMap,
  zoneContentKeys,
  type PublicConfig,
} from "@/lib/public-config";

const emptyConfig = {
  features: {} as PublicConfig["features"],
  captcha: {} as PublicConfig["captcha"],
  client: {} as PublicConfig["client"],
  legal: {} as PublicConfig["legal"],
  upload: {} as PublicConfig["upload"],
  collaboration: {} as PublicConfig["collaboration"],
  oss_domain: "",
} satisfies PublicConfig;

/** #687 读取层：兜底注册表与访问器语义 */
test("contentTypeEntries falls back to the shipped baseline when the projection is absent", () => {
  assert.equal(contentTypeEntries(null).length, 9);
  assert.equal(contentTypeEntries(undefined).length, 9);
  assert.equal(contentTypeEntries(emptyConfig).length, 9);
  assert.equal(FALLBACK_CONTENT_TYPE_ENTRIES.length, 9);
});

test("projected registry wins over the fallback", () => {
  const config = {
    ...emptyConfig,
    content_types: [
      { key: "image", zones: ["original"], form: "media", upload_file_types: ["image"], judge_eligible: true },
      { key: "3d_print", zones: ["original", "fanwork"], form: "file", upload_file_types: ["model3d", "text"], judge_eligible: true },
    ],
  } satisfies PublicConfig;
  const keys = contentTypeEntries(config).map((entry) => entry.key);
  assert.deepEqual(keys, ["image", "3d_print"]);
  assert.equal(zoneContentKeys(config, "fanwork").length, 1);
  assert.equal(zoneContentKeys(config, "fanwork")[0], "3d_print");
});

test("form axis reproduces the legacy FILE_PRIMARY semantics", () => {
  // 文件主形态 = form ∈ {file, media}，与旧 FILE_PRIMARY_TYPES 六类一致。
  for (const key of ["image", "video", "audio", "sheet_music", "mod", "template"]) {
    assert.equal(isFilePrimaryContentType(null, key), true, `${key} stays file-primary`);
  }
  for (const key of ["article", "prompt", "other"]) {
    assert.equal(isFilePrimaryContentType(null, key), false, `${key} stays text-primary`);
  }
  // 未知类型回退 text 形态（不崩、与旧 includes 判定等价）。
  assert.equal(formForContentType(null, "unknown_type"), "text");
  assert.equal(isFilePrimaryContentType(null, "unknown_type"), false);
});

test("zoneContentKeys fallback order equals the legacy IP hub filter list", () => {
  assert.deepEqual(zoneContentKeys(null, "fanwork"), [
    "image",
    "article",
    "video",
    "audio",
    "mod",
    "prompt",
    "sheet_music",
    "other",
  ]);
  assert.deepEqual(zoneContentKeys(null, "original"), [
    "image",
    "article",
    "video",
    "audio",
    "template",
    "sheet_music",
    "other",
  ]);
});

test("clientAccept: unrestricted family yields *, explicit-only yields the sorted union, none yields undefined", () => {
  assert.equal(clientAcceptForContentType(null, "mod"), "*");
  assert.equal(clientAcceptForContentType(null, "template"), "*");
  assert.equal(clientAcceptForContentType(null, "sheet_music"), ".mid,.midi,.mscx,.mscz,.mxl,.pdf,.xml");
  assert.equal(clientAcceptForContentType(null, "article"), undefined);
});

test("deriveUploadFamilyForExtension implements the v2.1 derivation contract", () => {
  // .pdf 双属由品类上下文消解：sheet_music 品类 = 显式命中 sheet_music 族；
  // template 品类 = 无显式命中走唯一 unrestricted text 族兜底。
  assert.equal(deriveUploadFamilyForExtension(null, "sheet_music", "song.pdf"), "sheet_music");
  assert.equal(deriveUploadFamilyForExtension(null, "template", "manual.pdf"), "text");
  assert.equal(deriveUploadFamilyForExtension(null, "template", "notes.doc"), "text");
  assert.equal(deriveUploadFamilyForExtension(null, "mod", "pack.zip"), "mod");
  // 无兜底族群 / 文本形态品类 / 未知品类 → null（不支持的格式）。
  assert.equal(deriveUploadFamilyForExtension(null, "sheet_music", "song.docx"), null);
  assert.equal(deriveUploadFamilyForExtension(null, "article", "x.pdf"), null);
  assert.equal(deriveUploadFamilyForExtension(null, "nope", "x.pdf"), null);
  // 大小写与无扩展名容错。
  assert.equal(deriveUploadFamilyForExtension(null, "sheet_music", "Song.PDF"), "sheet_music");
  assert.equal(deriveUploadFamilyForExtension(null, "template", "noext"), null);
});

test("uploadFileTypeMap exposes extension policy (null = unrestricted) and caps", () => {
  const families = uploadFileTypeMap(null);
  assert.equal(Object.keys(families).length, 8);
  assert.equal(families["sheet_music"].extensions === null, false, "sheet_music is explicit");
  assert.equal(families["document"].extensions === null, false, "#688 document is explicit");
  assert.deepEqual(families["document"].extensions, [".docx", ".xlsx", ".csv"]);
  assert.equal(families["audio"].max_mb, 50);
  assert.equal(families["mod"].extensions, null, "mod is unrestricted (MIME-driven)");
  assert.equal(families["mod"].max_mb, 500);
  assert.equal(families["avatar"].max_mb, 20, "avatar reuses the image budget");
  assert.equal(uploadFileTypeEntries(null).length, 8);
});

test("contentTypeMap lookup is keyed by type key", () => {
  const map = contentTypeMap(null);
  assert.equal(map["mod"].judge_eligible, false);
  assert.equal(map["mod"].zones.length, 1);
  assert.equal(map["article"].form, "text");
  assert.equal(map["not_a_type"], undefined);
});
