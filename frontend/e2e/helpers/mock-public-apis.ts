import type { Page } from "@playwright/test";
import { installMockedApiGuard, mockApiRoute } from "./mock-api-guard";

export async function mockPublicApis(page: Page) {
  await installMockedApiGuard(page);

  await mockApiRoute(page, "**/api/v1/config/public", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        features: {
          web_agent_enabled: false,
          desktop_deploy_enabled: false,
          creator_support_enabled: false,
          payment_enabled: false,
        },
        captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
        client: { download_enabled: false, download_url: "", latest_version: "" },
        legal: {
          current_terms_version: "test",
          current_privacy_version: "test",
        },
      }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/auth/csrf", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ csrf_token: "test-csrf-token" }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ code: "UNAUTHORIZED", message: "unauthorized" }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/ips/stats/category_counts", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ category_counts: {} }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/ips?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ips: [] }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/contents/search?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [], total: 0, page: 1, page_size: 20, total_pages: 0, time_range: "all" }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/contents?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ contents: [], total: 0, page: 1, page_size: 20 }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/stats/summary", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ summary: { users: 0, ips: 0, contents: 0 } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/tags/faceted?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tags: [] }),
    }),
  );
}
