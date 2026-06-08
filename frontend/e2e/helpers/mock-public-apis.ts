import type { Page } from "@playwright/test";

export async function mockPublicApis(page: Page) {
  await page.route("**/api/v1/config/public", (route) =>
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
        captcha: { provider: "bypass" },
        legal: {
          current_terms_version: "test",
          current_privacy_version: "test",
        },
      }),
    }),
  );

  await page.route("**/api/v1/auth/csrf", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ csrf_token: "test-csrf-token" }),
    }),
  );

  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ code: "UNAUTHORIZED", message: "unauthorized" }),
    }),
  );

  await page.route("**/api/v1/ips/stats/category_counts", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ category_counts: {} }),
    }),
  );

  await page.route("**/api/v1/ips?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ips: [] }),
    }),
  );

  await page.route("**/api/v1/contents/search?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [], total: 0 }),
    }),
  );

  await page.route("**/api/v1/contents?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ contents: [], total: 0 }),
    }),
  );

  await page.route("**/api/v1/stats/summary", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ summary: { users: 0, ips: 0, contents: 0 } }),
    }),
  );

  await page.route("**/api/v1/tags/faceted?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tags: [] }),
    }),
  );
}
