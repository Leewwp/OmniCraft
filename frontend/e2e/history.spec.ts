import path from "node:path";
import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

type HistoryItem = {
  id: number;
  content: {
    id: number;
    title: string;
    zone: "original";
    content_type: "article";
  } | null;
  content_item: HistoryItem["content"];
  viewed_at: string;
};

async function mockHistorySession(page: Page) {
  await mockPublicApis(page);
  const state = {
    items: [historyItem(1, "First history item"), historyItem(2, "Second history item")],
    deleteBodies: [] as unknown[],
  };

  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "history-user-token" } }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: {
          id: 7,
          email: "history@example.test",
          username: "history-user",
          avatar_url: "",
          bio: "",
          reputation: 10,
          preferred_locale: "en",
          role: "user",
          is_banned: false,
          email_verified_at: "2026-07-18T00:00:00Z",
          created_at: "2026-07-18T00:00:00Z",
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
  await mockApiRoute(page, "**/api/v1/users/me/history?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        items: state.items,
        history: state.items,
        total: state.items.length,
        page: 1,
        page_size: 20,
        retention_days: 7,
      }),
    }),
  );
  await mockApiRoute(page, "**/api/v1/users/me/history", async (route) => {
    const bodyText = route.request().postData();
    if (!bodyText) {
      await route.fulfill({
        status: 400,
        contentType: "application/json",
        body: JSON.stringify({ code: "CLEAR_CONFIRMATION_REQUIRED", message: "explicit clear confirmation is required" }),
      });
      return;
    }

    const body = JSON.parse(bodyText) as { ids?: number[]; clear_all?: boolean };
    state.deleteBodies.push(body);
    if (body.clear_all === true) {
      state.items = [];
    } else if (Array.isArray(body.ids) && body.ids.length > 0) {
      state.items = state.items.filter((item) => !body.ids?.includes(item.id));
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ message: "cleared" }),
    });
  });

  return state;
}

test("selected delete and confirmed clear-all use distinct explicit request modes", async ({ page }, testInfo) => {
  const state = await mockHistorySession(page);
  await page.goto("/history");

  await expect(page.getByText("First history item")).toBeVisible();
  await page.getByRole("button", { name: "Batch manage" }).click();
  await page.getByLabel("Select First history item").click();
  await page.getByRole("button", { name: "Delete selected (1)" }).click();
  await page.getByRole("button", { name: "Delete", exact: true }).click();

  await expect(page.getByText("First history item")).toHaveCount(0);
  expect(state.deleteBodies[0]).toEqual({ ids: [1] });

  await page.getByRole("button", { name: "Clear all" }).click();
  await page.getByRole("button", { name: "Cancel" }).click();
  expect(state.deleteBodies).toHaveLength(1);
  await expect(page.getByText("Second history item")).toBeVisible();

  await page.getByRole("button", { name: "Clear all" }).click();
  await page.getByRole("button", { name: "Delete", exact: true }).click();

  await expect(page.getByText("No browsing history")).toBeVisible();
  expect(state.deleteBodies[1]).toEqual({ clear_all: true });

  await expect(page.locator('[aria-live="polite"] > div')).toHaveCount(0, { timeout: 6_000 });

  const screenshotName = testInfo.project.name === "mobile-chrome"
    ? "community-browse-history-mobile.png"
    : "community-browse-history-desktop.png";
  await page.screenshot({
    path: path.join(process.cwd(), "..", "screenshots", screenshotName),
    fullPage: true,
  });
});

test("a stripped DELETE body is rejected without deleting history", async ({ page }) => {
  const state = await mockHistorySession(page);
  await page.goto("/history");
  await expect(page.getByText("First history item")).toBeVisible();

  const result = await page.evaluate(async () => {
    const response = await fetch("http://127.0.0.1:8080/api/v1/users/me/history", {
      method: "DELETE",
      credentials: "include",
    });
    return { status: response.status, body: await response.json() as { code?: string } };
  });

  expect(result.status).toBe(400);
  expect(result.body.code).toBe("CLEAR_CONFIRMATION_REQUIRED");
  expect(state.items).toHaveLength(2);
  await expect(page.getByText("First history item")).toBeVisible();
});

function historyItem(id: number, title: string): HistoryItem {
  const content = { id: id + 100, title, zone: "original" as const, content_type: "article" as const };
  return {
    id,
    content,
    content_item: content,
    viewed_at: `2026-07-${18 - id}T12:00:00Z`,
  };
}
