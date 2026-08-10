import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const ONE_PIXEL_PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
  "base64",
);

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
}

test("image media composer preserves dimensions and reordered sort order in the publish payload", async ({ page }) => {
  await mockCreatorSession(page);

  let grantNumber = 0;
  const contentCreates: Array<Record<string, unknown>> = [];

  await mockApiRoute(page, "**/api/v1/contents/oss-token", (route) => {
    grantNumber += 1;
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        upload_url: `http://127.0.0.1:18080/mock-upload/${grantNumber}`,
        oss_key: `uploads/creator/media-${grantNumber}.png`,
        grant_id: `grant-${grantNumber}`,
        expires_in: 300,
      }),
    });
  });
  await page.route("http://127.0.0.1:18080/mock-upload/**", (route) =>
    route.fulfill({
      status: 200,
      headers: {
        "access-control-allow-origin": "*",
        "access-control-allow-methods": "PUT, OPTIONS",
      },
      body: "",
    }),
  );
  await mockApiRoute(page, "**/api/v1/contents", async (route) => {
    if (route.request().method() !== "POST") return route.fallback();
    contentCreates.push(route.request().postDataJSON() as Record<string, unknown>);
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ id: 123 }),
    });
  });

  await page.goto("/studio/publish/original");
  await page.getByRole("button", { name: /Image/i }).click();
  await page.getByPlaceholder("Enter work title").fill("Media composer contract");
  await page.getByRole("button", { name: "Film & TV" }).click();

  await page.locator('input[type="file"]').setInputFiles([
    { name: "one.png", mimeType: "image/png", buffer: ONE_PIXEL_PNG },
    { name: "two.png", mimeType: "image/png", buffer: ONE_PIXEL_PNG },
  ]);
  await expect(page.getByAltText("Preview of one.png")).toBeVisible();
  await expect(page.getByAltText("Preview of two.png")).toBeVisible();
  await page.getByRole("button", { name: "Move one.png down" }).click();

  await page.screenshot({ path: "../screenshots/omnicraft-media-uploader-desktop.png", fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: "../screenshots/omnicraft-media-uploader-mobile.png", fullPage: true });

  await page.getByRole("button", { name: /^Publish$/i }).click();
  await expect.poll(() => contentCreates.length).toBe(1);

  expect(contentCreates[0]).toMatchObject({
    content_type: "image",
    attachments: [
      { file_name: "two.png", width: 1, height: 1, sort_order: 0 },
      { file_name: "one.png", width: 1, height: 1, sort_order: 1 },
    ],
  });
  expect(contentCreates[0]).not.toHaveProperty("cover_oss_key");
});
