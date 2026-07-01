import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

async function mockMessagesSession(page: Page) {
  await mockPublicApis(page);

  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-user-token" } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: {
          id: 1,
          email: "alice@example.test",
          username: "alice",
          avatar_url: "",
          bio: "",
          reputation: 10,
          preferred_locale: "en",
          role: "user",
          is_banned: false,
          email_verified_at: "2026-06-30T00:00:00Z",
          created_at: "2026-06-30T00:00:00Z",
        },
      }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ unread_counts: { total: 1, reply: 0, like: 0, system: 0, pr: 0, follow: 0, broadcast: 1 } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/notifications", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        notifications: [
          {
            id: 77,
            type: "system",
            channel: "broadcast",
            title: "Maintenance window",
            body: "**Downtime** from 02:00.",
            is_read: false,
            created_at: "2026-06-30T12:05:00Z",
          },
        ],
      }),
    }),
  );
}

test("message center renders broadcast notifications with title and Markdown", async ({ page }) => {
  await mockMessagesSession(page);

  await page.goto("/messages");

  await expect(page.getByText("Maintenance window")).toBeVisible();
  await expect(page.getByText("Broadcast").first()).toBeVisible();
  await expect(page.locator("strong", { hasText: "Downtime" })).toBeVisible();
});
