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
      // SSR 服务端 fetch 的 API 来源同样钉死到本套件 18080 stub：
      // process.env 已存在的值优先于 .env.local（@next/env 不覆盖既有进程变量），
      // 工位本地 env 文件无法再把 server-fetch 打向其他会话的后端（L 批 T1 假败根因）。
      INTERNAL_API_URL: "http://127.0.0.1:18080",
      NEXT_DIST_DIR: ".next-playwright-mocked",
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
