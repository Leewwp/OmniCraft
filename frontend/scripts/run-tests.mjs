import { readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";

const excludedDirectories = new Set([
  ".next",
  "coverage",
  "e2e",
  "node_modules",
  "playwright-report",
  "test-results",
]);
const supportedTestFile = /\.test\.(?:cjs|mjs|js|jsx|ts|tsx)$/;
const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));

function normalizePath(filePath) {
  return filePath.replaceAll("\\", "/");
}

export async function discoverTestFiles(root) {
  const discovered = [];

  async function visit(directory) {
    const entries = await readdir(directory, { withFileTypes: true });

    for (const entry of entries) {
      const entryPath = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        if (!excludedDirectories.has(entry.name)) {
          await visit(entryPath);
        }
        continue;
      }

      if (entry.isFile() && supportedTestFile.test(entry.name)) {
        discovered.push(normalizePath(path.resolve(entryPath)));
      }
    }
  }

  await visit(path.resolve(root));
  return discovered.sort();
}

export function groupTestFiles(files) {
  const grouped = { native: [], typed: [] };

  for (const file of files) {
    if (/\.test\.tsx?$/.test(file)) {
      grouped.typed.push(file);
    } else {
      grouped.native.push(file);
    }
  }

  return grouped;
}

export function runTestFiles(grouped, spawn = spawnSync) {
  const commands = [
    [grouped.native, ["--test"]],
    [grouped.typed, ["--import", "tsx", "--test", "--test-concurrency=1"]],
  ];

  for (const [files, args] of commands) {
    if (files.length === 0) {
      continue;
    }

    const result = spawn(process.execPath, [...args, ...files], {
      shell: false,
      stdio: "inherit",
    });
    if (result.error) {
      throw result.error;
    }
    if (result.status !== 0) {
      return result.status ?? 1;
    }
  }

  return 0;
}

export async function runMode(mode, options = {}) {
  if (mode !== "unit") {
    throw new Error(`Unknown test mode: ${mode}`);
  }

  const root = options.root ?? path.resolve(scriptDirectory, "..");
  const files = await discoverTestFiles(root);
  return runTestFiles(groupTestFiles(files), options.spawn);
}

async function main() {
  try {
    const status = await runMode(process.argv[2]);
    if (status !== 0) {
      process.exitCode = status;
    }
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}
