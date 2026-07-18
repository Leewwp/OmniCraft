import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  testMatch: ["**/contract-smoke.spec.ts", "**/*.mock.spec.ts"],
  timeout: 30_000,
  expect: { timeout: 5_000 },
  workers: 1,
  webServer: {
    command: "npm run dev -- --hostname 127.0.0.1 --port 3001",
    url: "http://127.0.0.1:3001",
    env: {
      NEXT_PUBLIC_API_URL: "http://127.0.0.1:18080",
    },
    reuseExistingServer: false,
    timeout: 120_000,
  },
  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://127.0.0.1:3001",
    trace: "retain-on-failure",
  },
});
