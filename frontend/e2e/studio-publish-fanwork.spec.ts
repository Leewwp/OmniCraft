import { expect, test, type Page } from "@playwright/test";
import { mockPublicApis } from "./helpers/mock-public-apis";

async function mockCreatorSession(page: Page) {
  await mockPublicApis(page);

  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-creator-token" } }),
    }),
  );

  await page.route("**/api/v1/auth/me", (route) =>
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

  await page.route("**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
    }),
  );
}

test("fanwork publish selects IP and source original IDs before submitting", async ({ page }) => {
  await mockCreatorSession(page);

  const sourceSearchUrls: string[] = [];
  const legacyContentSearchUrls: string[] = [];
  const contentCreates: unknown[] = [];

  await page.route("**/api/v1/ips?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ips: [{ id: 42, name: "Star Rail" }] }),
    }),
  );

  await page.route("**/api/v1/contents/search?**", (route) => {
    sourceSearchUrls.push(route.request().url());
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [{ id: 77, title: "Original Lightcone" }], total: 1 }),
    });
  });

  await page.route("**/api/v1/contents?**", (route) => {
    if (route.request().method() === "GET") {
      legacyContentSearchUrls.push(route.request().url());
    }
    return route.fallback();
  });

  await page.route("**/api/v1/contents", async (route) => {
    contentCreates.push(route.request().postDataJSON());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ id: 123 }),
    });
  });

  await page.goto("/studio/publish/fanwork");
  await page.getByRole("button", { name: /Article/i }).click();

  await page.getByPlaceholder("Enter work title").fill("Fanwork payload repair");
  await page.getByPlaceholder("Search and select IP...").fill("Star");
  await page.getByRole("button", { name: "Star Rail" }).click();

  await page.getByPlaceholder("Search original content title...").fill("Original");
  await page.getByRole("button", { name: "Original Lightcone" }).click();

  await page.getByPlaceholder("Enter body content (Markdown)...").fill("A repaired fanwork submission.");
  await page.getByRole("button", { name: /^Publish$/i }).click();

  await expect.poll(() => contentCreates.length).toBe(1);
  expect(contentCreates[0]).toMatchObject({
    zone: "fanwork",
    content_type: "article",
    ip_id: 42,
    source_original_id: 77,
  });
  expect(contentCreates[0]).not.toHaveProperty("ip_name");
  expect(sourceSearchUrls.some((url) => url.includes("/api/v1/contents/search?"))).toBe(true);
  expect(legacyContentSearchUrls).toHaveLength(0);
});

test("fanwork pickers preserve edited query after clearing a selected option", async ({ page }) => {
  await mockCreatorSession(page);

  await page.route("**/api/v1/ips?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ips: [{ id: 42, name: "Star Rail" }] }),
    }),
  );

  await page.route("**/api/v1/contents/search?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [{ id: 77, title: "Original Lightcone" }], total: 1 }),
    }),
  );

  await page.goto("/studio/publish/fanwork");
  await page.getByRole("button", { name: /Article/i }).click();

  const ipInput = page.getByPlaceholder("Search and select IP...");
  const sourceInput = page.getByPlaceholder("Search original content title...");

  await ipInput.fill("Star");
  await page.getByRole("button", { name: "Star Rail" }).click();
  await ipInput.fill("Star Rail edited");
  await expect(ipInput).toHaveValue("Star Rail edited");

  await sourceInput.fill("Original");
  await page.getByRole("button", { name: "Original Lightcone" }).click();
  await sourceInput.fill("Original Lightcone edited");
  await expect(sourceInput).toHaveValue("Original Lightcone edited");
});
