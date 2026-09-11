import { expect, test } from "@playwright/test";
import { mockPublicApis } from "./helpers/mock-public-apis";

test.describe("unified page shell (#66)", () => {
  test.use({ locale: "zh-CN" });

  test("brand entry routes to the recommendation feed on desktop and mobile", async ({ page }) => {
    await mockPublicApis(page);

    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/original");
    await page.getByRole("link", { name: /万象工坊|OmniCraft/i }).click();
    await page.waitForURL(/\/recommend$/);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

    await page.setViewportSize({ width: 375, height: 844 });
    await page.goto("/original");
    await page.getByRole("button", { name: /打开菜单/i }).click();
    const drawer = page.getByRole("dialog");
    await drawer.getByRole("link", { name: /万象工坊|OmniCraft/i }).click();
    await page.waitForURL(/\/recommend$/);
  });

  test("desktop header brand aligns with the shared 1280px page-shell grid", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    for (const route of ["/original", "/recommend"]) {
      await page.goto(route);
      const geometry = await page.evaluate(() => {
        const logo = document.querySelector("header a[href='/recommend']");
        const headerInner = document.querySelector("header > div");
        const shell = Array.from(document.querySelectorAll("main div")).find(
          (el) => el instanceof HTMLElement && getComputedStyle(el).maxWidth === "1280px",
        );
        if (!(logo instanceof HTMLElement) || !(headerInner instanceof HTMLElement) || !(shell instanceof HTMLElement)) {
          return null;
        }
        return {
          logoX: logo.getBoundingClientRect().left,
          headerInnerX: headerInner.getBoundingClientRect().left,
          shellX: shell.getBoundingClientRect().left,
          shellWidth: shell.getBoundingClientRect().width,
        };
      });
      expect(geometry).not.toBeNull();
      expect(geometry!.shellWidth).toBe(1280);
      expect(geometry!.logoX).toBeCloseTo(geometry!.shellX + 24, 0);
      expect(geometry!.headerInnerX).toBeCloseTo(geometry!.shellX, 0);
    }
  });

  test("public sidebar and adjacent page surface share one background in light and dark mode", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/original");

    for (const mode of ["light", "dark"]) {
      await page.evaluate((dark) => {
        document.documentElement.classList.toggle("dark", dark);
      }, mode === "dark");

      const colors = await page.evaluate(() => {
        const aside = document.querySelector("aside");
        if (!(aside instanceof HTMLElement)) return null;
        return {
          sidebar: getComputedStyle(aside).backgroundColor,
          surface: getComputedStyle(document.body).backgroundColor,
        };
      });
      expect(colors).not.toBeNull();
      expect(colors!.sidebar).toBe(colors!.surface);
      expect(colors!.sidebar).not.toBe("rgba(0, 0, 0, 0)");
    }
  });

  test("filter selected states use the colored pill contract with aria-pressed on /, /original and /ips", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.evaluate(() => document.documentElement.classList.remove("dark"));

    await page.goto("/");
    let active = page.getByRole("button", { name: /全部/, pressed: true }).first();
    await expect(active).toBeVisible();
    await expect(active).toHaveCSS("border-radius", "9999px");
    await expect(active).toHaveCSS("border-color", "rgb(79, 70, 229)");
    await expect(active).toHaveCSS("background-color", "rgb(238, 242, 255)");
    await expect(page.getByRole("button", { pressed: false }).first()).toBeVisible();

    await page.goto("/original");
    active = page.getByRole("button", { name: /推荐/, pressed: true });
    await expect(active).toBeVisible();
    await expect(active).toHaveAttribute("aria-pressed", "true");
    await expect(active).toHaveCSS("border-radius", "9999px");
    await expect(active).toHaveCSS("border-color", "rgb(79, 70, 229)");
    await expect(active).toHaveCSS("background-color", "rgb(238, 242, 255)");

    await page.goto("/ips");
    active = page.getByRole("button", { name: /全部/, pressed: true }).first();
    await expect(active).toBeVisible();
    await expect(active).toHaveCSS("border-radius", "9999px");
    await expect(active).toHaveCSS("border-color", "rgb(79, 70, 229)");
    await expect(active).toHaveCSS("background-color", "rgb(238, 242, 255)");

    // Keyboard interaction: filter pills are focusable and Enter triggers in-place switching.
    await page.goto("/original");
    const tabs = page.getByRole("navigation", { name: "原创区" }).getByRole("button");
    await tabs.first().focus();
    await expect(tabs.first()).toBeFocused();
    await page.keyboard.press("Tab");
    const second = tabs.nth(1);
    await expect(second).toBeFocused();
    await second.press("Enter");
    await page.waitForURL(/\/original\?category=/);
    await expect(second).toHaveAttribute("aria-pressed", "true");
  });

  test("no horizontal overflow on mobile shell surfaces", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 375, height: 844 });

    for (const route of ["/", "/original", "/ips", "/recommend"]) {
      await page.goto(route);
      const hasOverflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
      );
      expect(hasOverflow, `${route} must not overflow horizontally`).toBe(false);
    }
  });
});
