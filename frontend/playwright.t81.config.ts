import { defineConfig, devices } from "@playwright/test";

// Ticket #81 browser verification config: runs against the worktree's own
// frontend dev server on :3001 (port 3000 is reserved for the parallel agent).
export default defineConfig({
  testDir: "./e2e",
  testMatch: ["**/t81-original-filter.spec.ts"],
  timeout: 60_000,
  expect: { timeout: 15_000 },
  workers: 1,
  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://127.0.0.1:3001",
    trace: "retain-on-failure",
  },
});
