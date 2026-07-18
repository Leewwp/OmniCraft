import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

async function source(relativePath) {
  return readFile(path.join(frontendRoot, relativePath), "utf8");
}

test("mocked contracts start only Next and use a dedicated mock API port", async () => {
  const [packageJson, config, contractSmoke] = await Promise.all([
    source("package.json"),
    source("playwright.mocked.config.ts"),
    source("e2e/contract-smoke.spec.ts"),
  ]);
  const scripts = JSON.parse(packageJson).scripts;

  assert.equal(scripts["test:contracts"], "playwright test --config=playwright.mocked.config.ts");
  assert.match(config, /command:\s*["']npm run dev -- --hostname 127\.0\.0\.1 --port 3001["']/);
  assert.match(config, /url:\s*["']http:\/\/127\.0\.0\.1:3001["']/);
  assert.match(config, /reuseExistingServer:\s*false/);
  assert.match(config, /NEXT_PUBLIC_API_URL:\s*["']http:\/\/127\.0\.0\.1:18080["']/);
  assert.match(config, /testMatch:\s*\[[^\]]*contract-smoke\.spec\.ts[^\]]*\]/s);
  assert.doesNotMatch(config, /go run|backend|127\.0\.0\.1:8080(?!\d)|postgres|redis/i);
  assert.match(contractSmoke, /127\.0\.0\.1:18080/);
  assert.match(contractSmoke, /access-control-allow-origin["']:\s*["']http:\/\/127\.0\.0\.1:3001["']/);
  assert.doesNotMatch(contractSmoke, /127\.0\.0\.1:3000/);
  assert.doesNotMatch(contractSmoke, /127\.0\.0\.1:8080/);
});

test("cross-stack tests have no Playwright-managed servers and cannot select zero tests silently", async () => {
  const [packageJson, config, integrationSpec] = await Promise.all([
    source("package.json"),
    source("playwright.cross-stack.config.ts"),
    source("e2e/cross-stack-health.integration.spec.ts"),
  ]);
  const scripts = JSON.parse(packageJson).scripts;

  assert.equal(scripts["test:cross-stack"], "playwright test --config=playwright.cross-stack.config.ts");
  assert.equal(scripts["test:e2e"], "npm run test:cross-stack");
  assert.match(config, /testMatch:\s*\[[^\]]*\.integration\.spec\.ts[^\]]*\]/s);
  assert.match(config, /baseURL:\s*["']http:\/\/127\.0\.0\.1:3000["']/);
  assert.doesNotMatch(config, /webServer\s*:/);
  assert.doesNotMatch(config, /projects\s*:/);
  assert.doesNotMatch(config, /18080/);
  assert.doesNotMatch(scripts["test:cross-stack"], /pass-with-no-tests/);
  assert.match(integrationSpec, /const realApiBase = ["']http:\/\/127\.0\.0\.1:8080["']/);
  assert.match(integrationSpec, /page\.waitForResponse/);
  assert.match(integrationSpec, /api\/v1\/config\/public/);
  assert.doesNotMatch(integrationSpec, /page\.request|page\.route|createServer|18080/);
});

test("the legacy default configuration delegates to cross-stack configuration", async () => {
  const config = await source("playwright.config.ts");

  assert.match(config, /playwright\.cross-stack\.config/);
  assert.doesNotMatch(config, /go run|contract-smoke\.spec\.ts/);
});
