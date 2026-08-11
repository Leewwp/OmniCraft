import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

async function mockCreatorSession(page: Page) {
  await mockPublicApis(page);

  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-creator-token" } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: {
          id: 5,
          email: "creator@example.com",
          username: "creator",
          avatar_url: "",
          bio: "",
          reputation: 10,
          preferred_locale: "en",
          role: "user",
          is_banned: false,
          email_verified_at: "2026-01-01T00:00:00Z",
          created_at: "2026-01-01T00:00:00Z",
        },
      }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/ips?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ips: [{ id: 42, name: "Star Rail" }] }),
    }),
  );
}

const MIXED_ITEMS = [
  { id: 77, title: "Original Lightcone", zone: "original", status: "published" },
  { id: 78, title: "Stellar Memories", zone: "original", status: "published" },
  { id: 79, title: "Banned Draft", zone: "original", status: "banned" },
  { id: 80, title: "Under Review Draft", zone: "original", status: "pending" },
];

test("t5 source picker: search, filter, select, clear, error, and empty states with screenshots", async ({ page }) => {
  await mockCreatorSession(page);

  await mockApiRoute(page, "**/api/v1/contents/search?**", async (route) => {
    const url = new URL(route.request().url());
    const q = url.searchParams.get("q") ?? "";
    if (q.startsWith("err")) {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ code: "DB_ERROR", message: "search failed" }),
      });
      return;
    }
    const items = q === "zoo" ? [] : MIXED_ITEMS;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items, total: items.length }),
    });
  });

  await page.goto("/studio/publish/fanwork");
  await page.getByRole("button", { name: /Article/i }).click();

  await page.getByPlaceholder("Enter work title").fill("Source picker screenshot demo");
  await page.getByPlaceholder("Search and select IP...").fill("Star");
  await page.getByRole("button", { name: "Star Rail" }).click();
  await page.screenshot({ path: "../screenshots/t5-publish-source-picker-default.png", fullPage: true });

  const sourceInput = page.getByPlaceholder("Search original content title...");
  await sourceInput.fill("light");
  await expect(page.getByRole("option", { name: /Original Lightcone/ })).toBeVisible();
  await expect(page.getByRole("option", { name: /Stellar Memories/ })).toBeVisible();
  await expect(page.getByRole("option", { name: /Banned Draft/ })).not.toBeVisible();
  await expect(page.getByRole("option", { name: /Under Review Draft/ })).not.toBeVisible();
  await page.screenshot({ path: "../screenshots/t5-publish-source-picker-results.png", fullPage: true });

  await page.getByRole("option", { name: /Original Lightcone/ }).click();
  await expect(page.getByRole("button", { name: "Clear selected source" })).toBeVisible();
  await expect(page.getByText("Selected source")).toBeVisible();
  await page.screenshot({ path: "../screenshots/t5-publish-source-picker-selected.png", fullPage: true });

  await page.getByRole("button", { name: "Clear selected source" }).click();
  await expect(page.getByRole("button", { name: "Clear selected source" })).not.toBeVisible();
  await page.screenshot({ path: "../screenshots/t5-publish-source-picker-cleared.png", fullPage: true });

  await sourceInput.fill("err");
  await expect(page.getByText("Search failed. Please try again.")).toBeVisible();
  await page.screenshot({ path: "../screenshots/t5-publish-source-picker-error.png", fullPage: true });

  await page.getByRole("button", { name: "Retry" }).click();
  await sourceInput.fill("zoo");
  await expect(page.getByText("No matching content found.")).toBeVisible();
  await page.screenshot({ path: "../screenshots/t5-publish-source-picker-empty.png", fullPage: true });
});
