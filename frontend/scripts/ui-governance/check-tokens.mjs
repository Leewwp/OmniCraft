import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const REQUIRED_TABLES = ["颜色", "标签颜色", "间距", "圆角", "字体", "微阴影"];
const REQUIRED_TOKENS = [
  "--accent-hover",
  "--border-destructive",
  "--radius-full",
  "--elevation-1",
  "--elevation-2",
  "--elevation-3",
];

function cleanCell(value) {
  return value.trim().replace(/^`|`$/g, "");
}

function splitRow(line) {
  return line
    .trim()
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map(cleanCell);
}

function isDividerRow(line) {
  return splitRow(line).every((cell) => /^:?-{3,}:?$/.test(cell));
}

export function parseMarkdownTables(markdown) {
  const lines = markdown.split(/\r?\n/);
  const tables = new Map();
  let heading = "";

  for (let index = 0; index < lines.length; index += 1) {
    const headingMatch = lines[index].match(/^###\s+(.+?)\s*$/);
    if (headingMatch) {
      heading = headingMatch[1];
      continue;
    }
    if (!heading || !lines[index].trim().startsWith("|")) continue;

    const header = splitRow(lines[index]);
    const divider = lines[index + 1];
    if (!divider?.trim().startsWith("|") || !isDividerRow(divider)) continue;

    const rows = [];
    index += 2;
    while (index < lines.length && lines[index].trim().startsWith("|")) {
      const cells = splitRow(lines[index]);
      rows.push(Object.fromEntries(header.map((name, cellIndex) => [name, cells[cellIndex] ?? ""])));
      index += 1;
    }
    index -= 1;
    tables.set(heading, rows);
  }

  return tables;
}

function normalizeValue(value) {
  return value.trim().replace(/\s+/g, " ").toLowerCase();
}

function parseDeclarations(css) {
  const declarations = new Map();
  const pattern = /(--[a-z0-9-]+)\s*:\s*([^;{}]+);/gi;
  for (const match of css.matchAll(pattern)) {
    const values = declarations.get(match[1]) ?? [];
    values.push(normalizeValue(match[2]));
    declarations.set(match[1], values);
  }
  return declarations;
}

function parseScopedDeclarations(css, selector) {
  const declarations = new Map();
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const blockPattern = new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`, "g");
  for (const block of css.matchAll(blockPattern)) {
    for (const [token, values] of parseDeclarations(block[1])) {
      declarations.set(token, values.at(-1));
    }
  }
  return declarations;
}

export function findUndefinedCustomPropertyReferences(css) {
  const declarations = new Set(parseDeclarations(css).keys());
  const references = new Set(
    [...css.matchAll(/var\(\s*(--[a-z0-9-]+)/gi)].map((match) => match[1]),
  );
  return [...references].filter((token) => !declarations.has(token)).sort();
}

function tokenRows(tables) {
  const rows = [...tables.values()].flat();
  const tokenRows = rows.filter((row) => row.Token?.startsWith("--"));
  const tagRows = rows.flatMap((row) => {
    if (!row.Tag) return [];
    return [
      {
        Token: `--tag-${row.Tag}-bg`,
        Light: row["Light BG"],
        Dark: row["Dark BG"],
      },
      {
        Token: `--tag-${row.Tag}-fg`,
        Light: row["Light FG"],
        Dark: row["Dark FG"],
      },
    ];
  });
  return [...tokenRows, ...tagRows];
}

export function validateTokenContract(markdown, css) {
  const errors = [];
  const tables = parseMarkdownTables(markdown);
  const declarations = parseDeclarations(css);
  const light = parseScopedDeclarations(css, ":root");
  const dark = parseScopedDeclarations(css, ".dark");

  for (const heading of REQUIRED_TABLES) {
    if (!tables.has(heading)) errors.push(`Missing authoritative table: ${heading}`);
  }

  const rows = tokenRows(tables);
  const documentedTokens = new Set(rows.map((row) => row.Token));
  for (const required of REQUIRED_TOKENS) {
    if (!documentedTokens.has(required)) errors.push(`Missing authoritative token: ${required}`);
  }

  for (const row of rows) {
    const { Token: token } = row;
    if (row.Light && row.Dark) {
      if (light.get(token) !== normalizeValue(row.Light)) {
        errors.push(`${token} light mismatch: expected ${row.Light}`);
      }
      if (dark.get(token) !== normalizeValue(row.Dark)) {
        errors.push(`${token} dark mismatch: expected ${row.Dark}`);
      }
      continue;
    }
    if (row["值"] && !(declarations.get(token) ?? []).includes(normalizeValue(row["值"]))) {
      errors.push(`${token} mismatch: expected ${row["值"]}`);
    }
  }

  for (const token of findUndefinedCustomPropertyReferences(css)) {
    errors.push(`Undefined custom-property reference: ${token}`);
  }

  return errors;
}

async function main() {
  const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
  const frontendRoot = path.resolve(scriptDirectory, "../..");
  const repositoryRoot = path.resolve(frontendRoot, "..");
  const [markdown, css] = await Promise.all([
    readFile(path.join(repositoryRoot, "design/design-system.md"), "utf8"),
    readFile(path.join(frontendRoot, "app/globals.css"), "utf8"),
  ]);
  const errors = validateTokenContract(markdown, css);
  if (errors.length > 0) {
    console.error(errors.map((error) => `- ${error}`).join("\n"));
    process.exitCode = 1;
    return;
  }
  console.log("UI token governance check passed.");
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
