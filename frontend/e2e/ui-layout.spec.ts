import { expect, test } from "@playwright/test";
import { mockApiRoute } from "./helpers/mock-api-guard";
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

test("search hides raw API failure details and keeps retry visible", async ({ page }) => {
  await mockPublicApis(page);
  await mockApiRoute(page, "**/api/v1/contents/search?**", (route) =>
    route.fulfill({
      status: 500,
      contentType: "application/json",
      body: JSON.stringify({
        code: "DB_ERROR",
        message: "secret database stack trace",
      }),
    }),
  );
  await page.goto("/search");

  await page.getByPlaceholder(/Search by keyword|输入关键词搜索/i).fill("atlas");
  await page.getByRole("button", { name: /^(Search|搜索)$/i }).click();

  await expect(page.getByText(/Failed to load|加载失败/)).toBeVisible();
  await expect(page.getByRole("button", { name: /Retry|重试/i })).toBeVisible();
  await expect(page.getByText("secret database stack trace")).toHaveCount(0);
  await expect(page.getByText("DB_ERROR")).toHaveCount(0);
});

test("register exposes independent keyboard-accessible password reveal controls", async ({ page }) => {
  await mockPublicApis(page);
  await page.goto("/register");

  const passwordInput = page.locator("#password");
  const confirmPasswordInput = page.locator("#confirmPassword");
  const passwordReveal = page.getByRole("button", {
    name: /^(Show password|显示密码)$/i,
  });
  const confirmPasswordReveal = page.getByRole("button", {
    name: /Confirm password|确认密码/i,
  });
  const revealButtons = page.locator("button[aria-pressed]");

  await expect(passwordReveal).toHaveAttribute("aria-pressed", "false");
  await expect(confirmPasswordReveal).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByRole("button", {
    name: /Show password|显示密码|Hide password|隐藏密码/i,
  })).toHaveCount(1);
  await expect(revealButtons).toHaveCount(2);

  await passwordReveal.click();
  await expect(passwordInput).toHaveAttribute("type", "text");
  await expect(confirmPasswordInput).toHaveAttribute("type", "password");

  await confirmPasswordReveal.click();
  await expect(passwordInput).toHaveAttribute("type", "text");
  await expect(confirmPasswordInput).toHaveAttribute("type", "text");

  await page.getByRole("button", { name: /^(Hide password|隐藏密码)$/i }).click();
  await expect(passwordInput).toHaveAttribute("type", "password");
  await expect(confirmPasswordInput).toHaveAttribute("type", "text");

  await passwordInput.focus();
  await page.keyboard.press("Tab");
  await expect(revealButtons.nth(0)).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(confirmPasswordInput).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(revealButtons.nth(1)).toBeFocused();
});
