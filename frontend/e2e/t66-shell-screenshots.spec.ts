import { expect, test } from "@playwright/test";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SHOT_DIR = "../screenshots";

test.describe("#66 screenshot evidence", () => {
  test.use({ locale: "zh-CN" });
  test.describe.configure({ timeout: 120_000 });

  test.beforeEach(async ({ page }) => {
    await mockPublicApis(page);
    await page.goto("/", { waitUntil: "load" });
  });

  async function routeTo(page: import("@playwright/test").Page, path: string) {
    await page.goto(path, { waitUntil: "load" });
    await expect(page.getByRole("link", { name: /万象工坊|OmniCraft/i }).first()).toBeVisible({ timeout: 20000 });
  }

  test("desktop light shell screenshots", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("theme", "light"));
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.evaluate(() => document.documentElement.classList.remove("dark"));
    for (const [path, name] of [
      ["/", "t66-home-desktop-light"],
      ["/original", "t66-original-desktop-light"],
      ["/ips", "t66-ips-desktop-light"],
      ["/recommend", "t66-recommend-desktop-light"],
    ] as const) {
      await routeTo(page, path);
      await page.screenshot({ path: `${SHOT_DIR}/${name}.png`, fullPage: false });
    }
  });

  test("desktop dark shell screenshot (sidebar surface)", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("theme", "dark"));
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/original", { waitUntil: "load" });
    await page.waitForFunction(() => document.documentElement.classList.contains("dark"));
    await expect(page.getByRole("link", { name: /万象工坊|OmniCraft/i }).first()).toBeVisible({ timeout: 20000 });
    await page.screenshot({ path: `${SHOT_DIR}/t66-original-desktop-dark.png`, fullPage: false });
  });

  test("mobile shell screenshots incl. brand menu routing", async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("theme", "light"));
    await page.setViewportSize({ width: 375, height: 844 });
    await page.evaluate(() => document.documentElement.classList.remove("dark"));
    await routeTo(page, "/original");
    await page.screenshot({ path: `${SHOT_DIR}/t66-original-mobile.png`, fullPage: false });

    await page.getByRole("button", { name: /打开菜单/i }).click();
    const drawer = page.getByRole("dialog");
    await drawer.getByRole("link", { name: /万象工坊|OmniCraft/i }).click();
    await page.waitForURL(/\/recommend$/);
    await page.screenshot({ path: `${SHOT_DIR}/t66-mobile-menu-brand-to-recommend.png`, fullPage: false });
  });
});
