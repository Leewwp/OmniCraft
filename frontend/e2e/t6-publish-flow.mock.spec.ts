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

const ORIGINAL_ITEMS = [
  { id: 77, title: "Original Lightcone", zone: "original", status: "published" },
];
const FANWORK_ITEMS = [
  { id: 88, title: "Fanwork Piece", zone: "fanwork", status: "published" },
];

async function mockSourceSearch(page: Page) {
  await mockApiRoute(page, "**/api/v1/contents/search?**", async (route) => {
    const url = new URL(route.request().url());
    const q = url.searchParams.get("q") ?? "";
    const zone = url.searchParams.get("zone") ?? "";
    const items = q === "zoo" ? [] : zone === "fanwork" ? FANWORK_ITEMS : ORIGINAL_ITEMS;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items, total: items.length }),
    });
  });
}

function publishedContentDetail(id: number, title: string, zone: "original" | "fanwork") {
  return {
    content: { id, title, zone, status: "published", author: { id: 1, username: "tester" } },
    attachments: [],
    tags: [],
    series_memberships: [],
  };
}

async function mockContentDetail(page: Page) {
  await mockApiRoute(page, "**/api/v1/contents/*", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith("/search")) {
      await route.fallback();
      return;
    }
    const id = url.pathname.split("/").pop();
    if (id === "77") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(publishedContentDetail(77, "Original Lightcone", "original")),
      });
      return;
    }
    if (id === "88") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(publishedContentDetail(88, "Fanwork Piece", "fanwork")),
      });
      return;
    }
    await route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ code: "CONTENT_NOT_FOUND", message: "not found" }),
    });
  });
}

async function mockCreateContent(page: Page, contentCreates: unknown[]) {
  await mockApiRoute(page, "**/api/v1/contents", async (route) => {
    if (route.request().method() !== "POST") {
      await route.fallback();
      return;
    }
    contentCreates.push(route.request().postDataJSON());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ id: 123 }),
    });
  });
}

async function openFanworkForm(page: Page, query = "") {
  await page.goto(`/studio/publish/fanwork${query}`);
  await page.getByRole("button", { name: /Article/i }).click();
}

test("t6 publish flow: IP-only, original-only, and fanwork-only sources each submit the expected payload", async ({ page }) => {
  await mockCreatorSession(page);
  await mockSourceSearch(page);
  await mockContentDetail(page);
  const contentCreates: unknown[] = [];
  await mockCreateContent(page, contentCreates);

  // IP only
  await openFanworkForm(page);
  await page.getByPlaceholder("Enter work title").fill("IP only fanwork");
  await page.getByPlaceholder("Search and select IP...").fill("Star");
  await page.getByRole("button", { name: "Star Rail" }).click();
  await page.getByRole("button", { name: /^Publish$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(1);
  expect(contentCreates[0]).toMatchObject({ zone: "fanwork", content_type: "article", ip_id: 42 });
  expect(contentCreates[0]).not.toHaveProperty("source_original_id");
  expect(contentCreates[0]).not.toHaveProperty("source_fanwork_id");

  // Original source only
  await page.reload();
  await page.getByRole("button", { name: /Article/i }).click();
  await page.getByPlaceholder("Enter work title").fill("Original source fanwork");
  await page.getByPlaceholder("Search original content title...").fill("light");
  await page.getByRole("option", { name: /Original Lightcone/ }).click();
  await page.getByRole("button", { name: /^Publish$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(2);
  expect(contentCreates[1]).toMatchObject({ source_original_id: 77 });
  expect(contentCreates[1]).not.toHaveProperty("ip_id");
  expect(contentCreates[1]).not.toHaveProperty("source_fanwork_id");

  // Fanwork source only
  await page.reload();
  await page.getByRole("button", { name: /Article/i }).click();
  await page.getByPlaceholder("Enter work title").fill("Fanwork source fanwork");
  await page.getByPlaceholder("Search fanwork content title...").fill("fan");
  await page.getByRole("option", { name: /Fanwork Piece/ }).click();
  await page.getByRole("button", { name: /^Publish$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(3);
  expect(contentCreates[2]).toMatchObject({ source_fanwork_id: 88 });
  expect(contentCreates[2]).not.toHaveProperty("ip_id");
  expect(contentCreates[2]).not.toHaveProperty("source_original_id");
});

test("t6 publish flow: no IP/source disables submit with validation copy; sources are mutually exclusive", async ({ page }) => {
  await mockCreatorSession(page);
  await mockSourceSearch(page);
  await mockContentDetail(page);
  const contentCreates: unknown[] = [];
  await mockCreateContent(page, contentCreates);

  await openFanworkForm(page);
  await page.getByPlaceholder("Enter work title").fill("Mutual exclusion fanwork");

  await expect(page.getByText("Fanwork requires an IP or an inspiration source")).toBeVisible();
  const submit = page.getByRole("button", { name: /^Publish$/i });
  await expect(submit).toBeDisabled();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-no-source-validation.png", fullPage: true });

  // Select original, then fanwork: fanwork selection clears the original.
  await page.getByPlaceholder("Search original content title...").fill("light");
  await page.getByRole("option", { name: /Original Lightcone/ }).click();
  await expect(page.getByText("Original Lightcone")).toBeVisible();
  await expect(submit).toBeEnabled();

  await page.getByPlaceholder("Search fanwork content title...").fill("fan");
  await page.getByRole("option", { name: /Fanwork Piece/ }).click();
  await expect(page.getByText("Fanwork Piece")).toBeVisible();
  await expect(page.getByText("Original Lightcone")).not.toBeVisible();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-mutual-exclusion.png", fullPage: true });

  await page.getByRole("button", { name: /^Publish$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(1);
  expect(contentCreates[0]).toMatchObject({ source_fanwork_id: 88 });
  expect(contentCreates[0]).not.toHaveProperty("source_original_id");
});

test("t6 publish flow: query prefill loads the source summary with a screenshot", async ({ page }) => {
  await mockCreatorSession(page);
  await mockSourceSearch(page);
  await mockContentDetail(page);
  const contentCreates: unknown[] = [];
  await mockCreateContent(page, contentCreates);

  await openFanworkForm(page, "?source_original_id=77");
  await page.getByPlaceholder("Enter work title").fill("Prefilled fanwork");
  await expect(page.getByText("Original Lightcone")).toBeVisible();
  await expect(page.getByText("Selected source")).toBeVisible();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-prefill-original.png", fullPage: true });

  await page.getByRole("button", { name: /^Publish$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(1);
  expect(contentCreates[0]).toMatchObject({ source_original_id: 77 });
  expect(contentCreates[0]).not.toHaveProperty("source_fanwork_id");
});

test("t6 publish flow: both prefill IDs keep original, clear fanwork, and show the localized warning", async ({ page }) => {
  await mockCreatorSession(page);
  await mockSourceSearch(page);
  await mockContentDetail(page);
  const contentCreates: unknown[] = [];
  await mockCreateContent(page, contentCreates);

  await openFanworkForm(page, "?source_original_id=77&source_fanwork_id=88");
  await page.getByPlaceholder("Enter work title").fill("Both prefill fanwork");
  await expect(page.getByText("Original Lightcone")).toBeVisible();
  await expect(page.getByText(/Both an original source and a fanwork source were specified/)).toBeVisible();
  await expect(page.getByText("Fanwork Piece")).not.toBeVisible();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-prefill-both-warning.png", fullPage: true });

  await page.getByRole("button", { name: /^Publish$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(1);
  expect(contentCreates[0]).toMatchObject({ source_original_id: 77 });
  expect(contentCreates[0]).not.toHaveProperty("source_fanwork_id");
});

test("t6 publish flow: invalid prefill id shows a non-blocking localized warning and leaves the picker empty", async ({ page }) => {
  await mockCreatorSession(page);
  await mockSourceSearch(page);
  await mockContentDetail(page);
  const contentCreates: unknown[] = [];
  await mockCreateContent(page, contentCreates);

  await openFanworkForm(page, "?source_original_id=abc");
  await page.getByPlaceholder("Enter work title").fill("Invalid prefill fanwork");
  await expect(page.getByText("Invalid source parameter. No source was prefilled.")).toBeVisible();
  await expect(page.getByText("Selected source")).not.toBeVisible();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-prefill-invalid-warning.png", fullPage: true });

  await page.getByRole("button", { name: "Dismiss prefill notice" }).click();
  await expect(page.getByText("Invalid source parameter. No source was prefilled.")).not.toBeVisible();
});

test("t6 publish flow: ip-only, original-only, and fanwork-only forms render for screenshots", async ({ page }) => {
  await mockCreatorSession(page);
  await mockSourceSearch(page);
  await mockContentDetail(page);

  // IP-only form state
  await openFanworkForm(page);
  await page.getByPlaceholder("Enter work title").fill("IP only fanwork");
  await page.getByPlaceholder("Search and select IP...").fill("Star");
  await page.getByRole("button", { name: "Star Rail" }).click();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-ip-only.png", fullPage: true });

  // Original-source form state (query-prefilled summary)
  await openFanworkForm(page, "?source_original_id=77");
  await page.getByPlaceholder("Enter work title").fill("Original source fanwork");
  await expect(page.getByText("Original Lightcone")).toBeVisible();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-original-source.png", fullPage: true });

  // Fanwork-source form state
  await openFanworkForm(page);
  await page.getByPlaceholder("Enter work title").fill("Fanwork source fanwork");
  await page.getByPlaceholder("Search fanwork content title...").fill("fan");
  await page.getByRole("option", { name: /Fanwork Piece/ }).click();
  await page.screenshot({ path: "../screenshots/t6-publish-fanwork-fanwork-source.png", fullPage: true });
});
