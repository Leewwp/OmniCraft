import { expect, test } from "@playwright/test";
import { mockPublicApis } from "./helpers/mock-public-apis";

test("search sends selected filters in backend query parameter names", async ({ page }) => {
  await mockPublicApis(page);
  const contentRequests: string[] = [];

  await page.route("**/api/v1/contents/search?**", (route) => {
    contentRequests.push(route.request().url());
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [], total: 0 }),
    });
  });

  await page.goto("/search");
  await page.getByRole("button", { name: /Advanced filter|Advanced filters/i }).click();
  await page.getByRole("button", { name: /Image/i }).click();
  await page.getByLabel(/Time range/i).selectOption("week");
  await page.getByPlaceholder(/keyword/i).fill("layout repair");
  await page.getByRole("button", { name: /^Search$/i }).click();

  await expect.poll(() => contentRequests.at(-1) ?? "").toContain("/api/v1/contents/search?");
  expect(contentRequests.at(-1)).toContain("content_type=image");
  expect(contentRequests.at(-1)).toContain("time_range=week");
});
