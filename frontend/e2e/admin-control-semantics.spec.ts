import { expect, test, type Page } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

async function mockAdminSession(page: Page) {
  await mockPublicApis(page);
  const categoryCreates: unknown[] = [];

  await mockApiRoute(page, "**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ tokens: { access_token: "test-admin-token" } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        user: {
          id: 1,
          email: "admin@example.com",
          username: "admin",
          avatar_url: "",
          bio: "",
          reputation: 10,
          preferred_locale: "en",
          role: "admin",
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

  const categoriesPayload = {
    categories: [
      {
        id: 10,
        zone: "fanwork",
        level: "category",
        parent_id: null,
        name_i18n: { zh: "游戏", en: "Games" },
        slug: "games",
        sort_order: 1,
        is_active: true,
      },
      {
        id: 20,
        zone: "original",
        level: "category",
        parent_id: null,
        name_i18n: { zh: "原创根分类", en: "Original Root" },
        slug: "original-root",
        sort_order: 1,
        is_active: true,
      },
    ],
  };

  await mockApiRoute(page, "**/api/v1/categories", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(categoriesPayload),
    }),
  );

  await mockApiRoute(page, "**/api/v1/categories?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(categoriesPayload),
    }),
  );

  await mockApiRoute(page, "**/api/v1/admin/audit-logs?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [], total: 0, page: 1, page_size: 20 }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/admin/feedback?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        items: [
          {
            id: 77,
            user_id: 2,
            contact_email: "user@example.com",
            category: "web_bug",
            title: "Broken control",
            description: "The select looks native.",
            diagnostic_summary: {},
            status: "open",
            priority: "normal",
            assignee_admin_id: null,
            replies: [],
            attachments: [],
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
            resolved_at: null,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/admin/feedback/77", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        id: 77,
        user_id: 2,
        contact_email: "user@example.com",
        category: "web_bug",
        title: "Broken control",
        description: "The select looks native.",
        diagnostic_summary: {},
        status: "open",
        priority: "normal",
        assignee_admin_id: null,
        replies: [],
        attachments: [],
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z",
        resolved_at: null,
      }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/admin/reports?**", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ reports: [], total: 0, page: 1, page_size: 20 }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/admin/categories", async (route) => {
    categoryCreates.push(route.request().postDataJSON());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ id: 30 }),
    });
  });

  return { categoryCreates };
}

// Mocked UI semantics coverage only. Real admin/category semantics are verified in Go tests.
test("mocked search advanced selects use the canonical select primitive", async ({ page }) => {
  await mockPublicApis(page);
  await page.goto("/search");
  await page.getByRole("button", { name: /Advanced filter|Advanced filters/i }).click();

  await expect(page.getByLabel(/Time range/i)).toHaveAttribute("data-slot", "select");
  await expect(page.getByLabel(/Sort by/i)).toHaveAttribute("data-slot", "select");
});

test("mocked admin filters and category numeric-like fields use deliberate controls", async ({ page }) => {
  const { categoryCreates } = await mockAdminSession(page);

  await page.goto("/admin/audit-logs");
  await expect(page.getByLabel(/All Actions/i)).toHaveAttribute("data-slot", "select");

  await page.goto("/admin/reports");
  await expect(page.getByLabel(/Report status/i)).toHaveAttribute("data-slot", "select");
  await expect(page.getByLabel(/Report type/i)).toHaveAttribute("data-slot", "select");

  await page.goto("/admin/feedback");
  await expect(page.getByLabel(/All Statuses/i)).toHaveAttribute("data-slot", "select");
  await expect(page.getByLabel(/All Categories/i)).toHaveAttribute("data-slot", "select");
  await page.getByRole("button", { name: /Broken control/i }).click();
  await expect(page.getByLabel(/^Status$|^状态$/i)).toHaveAttribute("data-slot", "select");
  await expect(page.getByLabel(/Priority/i)).toHaveAttribute("data-slot", "select");

  await page.goto("/admin/categories");
  await page.getByRole("button", { name: /New Category/i }).click();

  await expect(page.locator("input[type='number']")).toHaveCount(0);
  await page.getByLabel(/Zone|分区/i).selectOption("original");
  await expect(page.getByLabel(/Parent ID/i)).toHaveAttribute("data-slot", "select");
  await expect(page.getByLabel(/Parent ID/i)).toContainText(/Original Root|原创根分类/i);
  await page.getByLabel(/Parent ID/i).selectOption("20");
  await page.getByLabel(/Zone|鍒嗗尯/i).selectOption("fanwork");
  await expect(page.getByLabel(/Sort/i)).toHaveAttribute("type", "text");
  await expect(page.getByLabel(/Sort/i)).toHaveAttribute("inputmode", "numeric");
  await page.getByRole("textbox", { name: "Hottest" }).fill("Fanwork child");
  await page.getByRole("textbox", { name: "Recommended", exact: true }).fill("Fanwork child");
  await page.getByRole("textbox", { name: "recommended", exact: true }).fill("fanwork-child");
  await page.getByRole("button", { name: /^Create$/i }).click();
  await expect.poll(() => categoryCreates.length).toBe(1);
  expect(categoryCreates[0]).toMatchObject({ zone: "fanwork", parent_id: null });
  await page.getByRole("button", { name: /New Category/i }).click();

  await page.getByRole("button", { name: /Cancel|取消/i }).click();
  await page.getByRole("button", { name: /Edit|编辑/i }).first().click();
  await page.getByLabel(/Sort/i).fill("45x6");
  await expect(page.getByLabel(/Sort/i)).toHaveValue("456");
});
