import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const PLACEHOLDER_RE = /\{([a-zA-Z0-9_]+)(?:,[^}]*)?\}/g;

export function flattenLeaves(value, prefix = "", result = new Map()) {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    for (const [key, child] of Object.entries(value)) {
      flattenLeaves(child, prefix ? `${prefix}.${key}` : key, result);
    }
  } else {
    result.set(prefix, value);
  }
  return result;
}

export function placeholders(value) {
  return [...String(value ?? "").matchAll(PLACEHOLDER_RE)]
    .map((match) => match[1])
    .sort();
}

export function validateI18nParity(reference, candidate, { namespaces = [] } = {}) {
  const referenceLeaves = flattenLeaves(reference);
  const candidateLeaves = flattenLeaves(candidate);
  const errors = [];

  for (const key of referenceLeaves.keys()) {
    if (!candidateLeaves.has(key)) errors.push(`missing key: ${key}`);
  }
  for (const key of candidateLeaves.keys()) {
    if (!referenceLeaves.has(key)) errors.push(`extra key: ${key}`);
  }
  for (const [key, value] of referenceLeaves) {
    if (!candidateLeaves.has(key)) continue;
    if (typeof value !== "string" || typeof candidateLeaves.get(key) !== "string") {
      errors.push(`non-string leaf: ${key}`);
      continue;
    }
    if (!value.trim() || !String(candidateLeaves.get(key)).trim()) {
      errors.push(`empty leaf: ${key}`);
    }
    if (JSON.stringify(placeholders(value)) !== JSON.stringify(placeholders(candidateLeaves.get(key)))) {
      errors.push(`placeholder mismatch: ${key}`);
    }
  }
  for (const namespace of namespaces) {
    const prefix = `${namespace}.`;
    if (![...referenceLeaves.keys()].some((key) => key.startsWith(prefix))) {
      errors.push(`missing documented namespace: ${namespace}`);
    }
  }
  return errors;
}

export async function validateMessageCatalogs({ referencePath, candidatePath, namespaces = [] }) {
  const [referenceText, candidateText] = await Promise.all([
    readFile(referencePath, "utf8"),
    readFile(candidatePath, "utf8"),
  ]);
  return validateI18nParity(JSON.parse(referenceText), JSON.parse(candidateText), { namespaces });
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const [referencePath, candidatePath] = process.argv.slice(2);
  if (!referencePath || !candidatePath) {
    console.error("Usage: node check-i18n-parity.mjs <reference.json> <candidate.json>");
    process.exit(2);
  }
  const errors = await validateMessageCatalogs({
    referencePath,
    candidatePath,
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
  });
  if (errors.length) {
    console.error(errors.join("\n"));
    process.exit(1);
  }
  console.log("i18n parity OK");
}
