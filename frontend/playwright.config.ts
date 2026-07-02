import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: true,
  webServer: [
    {
      command: "npm run dev -- --hostname 127.0.0.1 --port 3000",
      url: "http://127.0.0.1:3000",
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command: "cd ../backend && go run cmd/server/main.go",
      url: "http://127.0.0.1:8080/healthz",
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
  ],
  use: {
    baseURL: "http://127.0.0.1:3000",
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "desktop",
      use: { ...devices["Desktop Chrome"] },
      testIgnore: [
        "**/contract-smoke.spec.ts",
        "**/*.mock.spec.ts",
        "**/*.integration.spec.ts",
      ],
    },
    {
      name: "mobile-chrome",
      use: { ...devices["Pixel 5"] },
      testIgnore: [
        "**/contract-smoke.spec.ts",
        "**/*.mock.spec.ts",
        "**/*.integration.spec.ts",
      ],
    },
    {
      name: "mocked",
      use: { ...devices["Desktop Chrome"] },
      testMatch: ["**/contract-smoke.spec.ts", "**/*.mock.spec.ts"],
      workers: 1,
    },
    {
      name: "cross-stack",
      use: { ...devices["Desktop Chrome"] },
      testMatch: ["**/*.integration.spec.ts"],
      workers: 1,
    },
  ],
});
