import { expect, test } from "@playwright/test";
import { mockPublicApis } from "./helpers/mock-public-apis";

test("home uses full-width content and hides desktop sidebar on mobile", async ({ page }) => {
  await mockPublicApis(page);
  await page.setViewportSize({ width: 375, height: 844 });
  await page.goto("/");

  await expect(page.getByRole("button", { name: /Collapse sidebar|收起侧边栏/i })).toBeHidden();

  const main = page.locator("[data-testid='home-main-content']");
  await expect(main).toBeVisible();
  const box = await main.boundingBox();
  expect(box?.width).toBeGreaterThan(330);

  const hasHorizontalOverflow = await page.evaluate(
    () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
  );
  expect(hasHorizontalOverflow).toBe(false);
});

test("search stacks controls vertically and opens an accessible mobile filter dialog", async ({ page }) => {
  await mockPublicApis(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/search");

  const results = page.locator("[data-testid='search-results-panel']");
  await expect(results).toBeVisible();
  const box = await results.boundingBox();
  expect(box?.width).toBeGreaterThan(340);

  await page.getByRole("button", { name: /Open advanced filters|Advanced filters|打开高级筛选|高级筛选/i }).click();
  const dialog = page.getByRole("dialog", { name: /Advanced filters|高级筛选/i });
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
});
