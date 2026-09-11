import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
const HERE = path.dirname(fileURLToPath(import.meta.url));
const result = spawnSync("npx", ["playwright", "test", "--config=playwright.mocked.config.ts", "e2e/t72-sort-control.mock.spec.ts"], { cwd: HERE, stdio: "inherit" });
process.exit(result.status ?? 1);
