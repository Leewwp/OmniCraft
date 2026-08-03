import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";

import { placeholders, validateI18nParity } from "./check-i18n-parity.mjs";

test("i18n parity rejects missing, extra, empty, and placeholder-mismatched leaves", () => {
  const errors = validateI18nParity(
    { messages: { count: "{count} items", title: "Title" } },
    { messages: { count: "{total} 项", extra: "Extra", title: "   " } },
    { namespaces: ["messages"] },
  );
  assert.deepEqual(errors, [
    "extra key: messages.extra",
    "placeholder mismatch: messages.count",
    "empty leaf: messages.title",
  ]);
});

test("placeholder extraction is order-stable and supports formatter options", () => {
  assert.deepEqual(placeholders("{name} · {count, plural, one {# item} other {# items}}"), ["count", "name"]);
});

test("repository message catalogs have exact parity across documented namespaces", async () => {
  const [en, zh] = await Promise.all([
    readFile(path.join(import.meta.dirname, "../../messages/en.json"), "utf8").then(JSON.parse),
    readFile(path.join(import.meta.dirname, "../../messages/zh.json"), "utf8").then(JSON.parse),
  ]);
  assert.deepEqual(validateI18nParity(en, zh, {
    namespaces: [
      "messages.tabs",
      "messages.notifications",
      "messages.conversations",
      "messages.chat",
      "messages.broadcast",
      "messages.empty",
      "messages.error",
      "messages.a11y",
    ],
  }), []);
});
