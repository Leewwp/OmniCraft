import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

async function mockAnonymousCollections(page: Page) {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/collections/9?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(collectionDetail({ is_public: true })),
    }),
  );
  await mockApiRoute(page, "**/api/v1/collections/404?**", (route) =>
    route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ code: "COLLECTION_NOT_FOUND", message: "collection not found" }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/collections?owner_id=7", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [collectionSummary({ title: "Public shelf", is_public: true })], total: 1 }),
    }),
  );
}

async function mockOwnerCollections(page: Page) {
  await mockAnonymousCollections(page);
  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "owner-token" } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: {
          id: 7,
          email: "ada@example.test",
          username: "Ada",
          avatar_url: "",
          bio: "",
          reputation: 10,
          preferred_locale: "en",
          role: "user",
          is_banned: false,
          email_verified_at: "2026-07-02T00:00:00Z",
          created_at: "2026-07-02T00:00:00Z",
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
  await mockApiRoute(page, "**/api/v1/collections/9?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(collectionDetail({ is_public: false, is_default: true })),
    }),
  );
  await mockApiRoute(page, "**/api/v1/collections?owner_id=7", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        items: [
          collectionSummary({ title: "Private research", is_public: false }),
          collectionSummary({ id: 12, title: "Public shelf", is_public: true }),
        ],
        total: 2,
      }),
    }),
  );
}

test("public collection detail renders while logged out", async ({ page }) => {
  await mockAnonymousCollections(page);

  await page.goto("/collections/9");

  await expect(page.getByText("Public shelf")).toBeVisible();
  await expect(page.getByText("Story board")).toBeVisible();
});

test("private collection detail hides title from logged-out users", async ({ page }) => {
  await mockAnonymousCollections(page);

  await page.goto("/collections/404");

  await expect(page.getByText("Collection unavailable")).toBeVisible();
  await expect(page.getByText("Private research")).toHaveCount(0);
});

test("owner detail shows disabled delete for default collection", async ({ page }) => {
  await mockOwnerCollections(page);

  await page.goto("/collections/9");

  await expect(page.getByRole("button", { name: "Edit collection" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Delete collection" })).toBeDisabled();
});

test("content type filter updates query and refetches collection detail", async ({ page }) => {
  await mockAnonymousCollections(page);

  await page.goto("/collections/9");
  await page.getByRole("tab", { name: "Video" }).click();

  await expect(page).toHaveURL(/content_type=video/);
  await expect(page.getByText("Public shelf")).toBeVisible();
});

test("user collections list is public but owner sees private collections and actions", async ({ page }) => {
  await mockOwnerCollections(page);

  await page.goto("/user/7/collections");

  await expect(page.getByText("Private research")).toBeVisible();
  await expect(page.getByText("Public shelf")).toBeVisible();
  await expect(page.getByRole("button", { name: "New collection" })).toBeVisible();
});

test("anonymous user collections list links public cards to collection detail", async ({ page }) => {
  await mockAnonymousCollections(page);

  await page.goto("/user/7/collections");
  await page.getByRole("link", { name: /Public shelf/ }).click();

  await expect(page).toHaveURL(/\/collections\/11$/);
});

function collectionSummary(overrides: Record<string, unknown> = {}) {
  return {
    id: 11,
    user_id: 7,
    title: "Public shelf",
    description: "Shared bookmarks",
    zone: "original",
    is_default: false,
    is_public: true,
    item_count: 1,
    contains_item: false,
    ...overrides,
  };
}

function collectionDetail(overrides: Record<string, unknown> = {}) {
  return {
    collection: {
      id: 9,
      user_id: 7,
      title: "Public shelf",
      description: "Shared bookmarks",
      zone: "original",
      is_public: true,
      is_default: false,
      item_count: 1,
      owner: { id: 7, username: "Ada" },
      ...overrides,
    },
    items: [
      {
        id: 101,
        content_item: {
          id: 501,
          title: "Story board",
          zone: "original",
          content_type: "article",
          cover_image_url: "",
          author: { id: 7, username: "Ada" },
        },
      },
    ],
    total: 1,
    page: 1,
    page_size: 20,
  };
}
