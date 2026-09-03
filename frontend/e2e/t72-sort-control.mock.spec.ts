import { expect, test, type Page } from "@playwright/test";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SHOT_DIR = path.join(process.cwd(), "..", "screenshots");

test.use({ locale: "zh-CN" });

async function openSort(page: Page, route: string) {
  await page.goto(route);
  const trigger = page.getByRole("combobox", { name: "排序方式" });
  await expect(trigger).toBeVisible();
  await trigger.click();
  const listbox = page.getByRole("listbox");
  await expect(listbox).toBeVisible();
  return { trigger, listbox };
}

test.describe("shared sort control across zones (#72)", () => {
  test("the fanwork zone, original zone and IP library reuse the same combobox/listbox control", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    for (const route of ["/", "/original", "/ips"]) {
      await page.goto(route);
      const trigger = page.getByRole("combobox", { name: "排序方式" });
      await expect(trigger).toBeVisible();
      await expect(trigger).toHaveAttribute("aria-haspopup", "listbox");
      await expect(trigger).toHaveAttribute("aria-expanded", "false");
      await trigger.click();
      const listbox = page.getByRole("listbox");
      await expect(listbox).toBeVisible();
      const options = listbox.getByRole("option");
      expect(await options.count()).toBeGreaterThan(0);
      await page.keyboard.press("Escape");
      await expect(page.getByRole("listbox")).toBeHidden();
    }
  });

  test("existing sort values stay compatible: deep links reflect in the trigger and URL semantics hold", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto("/original?sort=most_views");
    await expect(page.getByRole("combobox", { name: "排序方式" })).toHaveText(/最多点击/);
    await expect(page).toHaveURL(/sort=most_views/);

    await page.goto("/original?category=film_tv&sort=hot");
    await expect(page.getByRole("combobox", { name: "排序方式" })).toHaveText(/最热门/);

    await page.goto("/original");
    const trigger = page.getByRole("combobox", { name: "排序方式" });
    await trigger.click();
    await page.getByRole("option", { name: "最新发布" }).click();
    await expect(page).toHaveURL(/sort=newest/);
    await expect(page.getByRole("combobox", { name: "排序方式" })).toHaveText(/最新发布/);
  });

  test("IP library requests carry the chosen sort param", async ({ page }) => {
    await mockPublicApis(page);
    const captured: string[] = [];
    await mockApiRoute(page, "**/api/v1/ips?**", (route) => {
      captured.push(route.request().url());
      route.fallback();
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/ips");
    await expect(page.getByRole("combobox", { name: "IP 排序方式" })).toHaveText(/最热门/);

    await page.getByRole("combobox", { name: "IP 排序方式" }).click();
    await page.getByRole("option", { name: "名称" }).click();
    await expect(page.getByRole("combobox", { name: "IP 排序方式" })).toHaveText(/名称/);
    await expect.poll(() => captured.some((u) => u.includes("sort=name"))).toBe(true);
  });

  test("fanwork zone requests carry the chosen sort param", async ({ page }) => {
    await mockPublicApis(page);
    const captured: string[] = [];
    await mockApiRoute(page, "**/api/v1/contents?**", (route) => {
      if (route.request().url().includes("zone=fanwork")) {
        captured.push(route.request().url());
      }
      route.fallback();
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/");
    const trigger = page.getByRole("combobox", { name: "排序方式" });
    await expect(trigger).toHaveText(/最热门/);

    await trigger.click();
    await page.getByRole("option", { name: "最多点击" }).click();
    await expect(trigger).toHaveText(/最多点击/);
    await expect.poll(() => captured.some((u) => u.includes("sort=most_views"))).toBe(true);
  });

  test("keyboard flow: Tab into trigger, arrows/Home/End navigate, Enter commits, Escape and outside click close with focus return", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/original");
    const trigger = page.getByRole("combobox", { name: "排序方式" });

    await trigger.focus();
    await expect(trigger).toBeFocused();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("listbox")).toBeVisible();
    await expect(page.getByRole("option", { name: "推荐" })).toBeFocused();

    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("option", { name: "最热门" })).toBeFocused();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("option", { name: "最新发布" })).toBeFocused();
    await page.keyboard.press("Home");
    await expect(page.getByRole("option", { name: "推荐" })).toBeFocused();
    await page.keyboard.press("End");
    await expect(page.getByRole("option", { name: "最多点击" })).toBeFocused();

    await page.keyboard.press("Escape");
    await expect(page.getByRole("listbox")).toBeHidden();
    await expect(trigger).toBeFocused();
    expect(new URL(page.url()).searchParams.get("sort")).toBeNull();

    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("listbox")).toBeVisible();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("option", { name: "最热门" })).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(/sort=hot/);
    await expect(page.getByRole("listbox")).toBeHidden();
    await expect(trigger).toHaveText(/最热门/);

    // Space commits on a fresh page to avoid remount races around router.push.
    await page.goto("/original");
    const freshTrigger = page.getByRole("combobox", { name: "排序方式" });
    await freshTrigger.focus();
    await expect(freshTrigger).toBeFocused();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("listbox")).toBeVisible();
    await expect(page.getByRole("option", { name: "推荐" })).toBeFocused();
    await page.keyboard.press("ArrowDown");
    await expect(page.getByRole("option", { name: "最热门" })).toBeFocused();
    await page.keyboard.press(" ");
    await expect(page).toHaveURL(/sort=hot/);
    await expect(page.getByRole("listbox")).toBeHidden();
  });

  test("outside click closes the popup and restores trigger focus", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/original");
    const trigger = page.getByRole("combobox", { name: "排序方式" });
    await trigger.click();
    await expect(page.getByRole("listbox")).toBeVisible();
    await page.getByRole("heading", { level: 1 }).click();
    await expect(page.getByRole("listbox")).toBeHidden();
    await expect(trigger).toBeFocused();
  });

  test("sticky toolbar stacking: popup floats above toolbar content and never covers the trigger", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await openSort(page, "/original");

    await expect
      .poll(() =>
        page.evaluate(() => {
          const listbox = document.querySelector('[role="listbox"]');
          const popup = listbox?.parentElement;
          const combo = listbox instanceof HTMLElement
            ? document.querySelector(`[role="combobox"][aria-controls="${listbox.id}"]`)
            : null;
          if (!(popup instanceof HTMLElement) || !(combo instanceof HTMLElement)) return null;
          const p = popup.getBoundingClientRect();
          const c = combo.getBoundingClientRect();
          return {
            popupZ: getComputedStyle(popup).zIndex,
            toolbarZ: getComputedStyle(combo.closest("div[class*='sticky']") ?? combo.parentElement!).zIndex,
            popupBottom: p.bottom,
            viewportH: window.innerHeight,
            popupTop: p.top,
            triggerBottom: c.bottom,
          };
        }),
      )
      .toMatchObject({ popupZ: "50" });
    const settled = await page.evaluate(() => {
      const listbox = document.querySelector('[role="listbox"]');
      const popup = listbox?.parentElement;
      const combo = listbox instanceof HTMLElement
        ? document.querySelector(`[role="combobox"][aria-controls="${listbox.id}"]`)
        : null;
      if (!(popup instanceof HTMLElement) || !(combo instanceof HTMLElement)) return null;
      const p = popup.getBoundingClientRect();
      const c = combo.getBoundingClientRect();
      return {
        popupZ: getComputedStyle(popup).zIndex,
        toolbarZ: getComputedStyle(combo.closest("div[class*='sticky']") ?? combo.parentElement!).zIndex,
        popupTop: p.top,
        popupBottom: p.bottom,
        triggerTop: c.top,
        triggerBottom: c.bottom,
        viewportH: window.innerHeight,
      };
    });
    expect(settled).not.toBeNull();
    expect(Number(settled!.toolbarZ)).toBeLessThan(50);
    expect(settled!.popupBottom).toBeLessThanOrEqual(settled!.viewportH);
    expect(settled!.popupTop).toBeGreaterThanOrEqual(settled!.triggerBottom);

    // Anti-regression (T21 header search also exposes role="combobox"): with multiple
    // comboboxes in DOM, the probe must resolve the sort trigger via the open listbox's
    // aria-controls owner — never the first combobox in document order.
    const anchoring = await page.evaluate(() => {
      const listbox = document.querySelector('[role="listbox"]');
      const combo = listbox instanceof HTMLElement
        ? document.querySelector(`[role="combobox"][aria-controls="${listbox.id}"]`)
        : null;
      return {
        comboboxCount: document.querySelectorAll('[role="combobox"]').length,
        label: combo?.getAttribute("aria-label") ?? null,
        hasPopup: combo?.getAttribute("aria-haspopup") ?? null,
      };
    });
    expect(anchoring.comboboxCount).toBeGreaterThan(1);
    expect(anchoring.label).toContain("排序方式");
    expect(anchoring.hasPopup).toBe("listbox");

    await page.screenshot({ path: `${SHOT_DIR}/web-t72-sort-desktop.png` });
    await page.keyboard.press("Escape");
  });

  test("narrow viewport: popup stays inside the viewport without horizontal overflow", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 320, height: 700 });
    const { listbox } = await openSort(page, "/ips");

    const geometry = await page.evaluate(() => {
      const listbox = document.querySelector('[role="listbox"]');
      const popup = listbox?.parentElement;
      const combo = listbox instanceof HTMLElement
        ? document.querySelector(`[role="combobox"][aria-controls="${listbox.id}"]`)
        : null;
      if (!(popup instanceof HTMLElement) || !(combo instanceof HTMLElement)) return null;
      const p = popup.getBoundingClientRect();
      const c = combo.getBoundingClientRect();
      return {
        popupLeft: p.left,
        popupRight: p.right,
        viewportW: window.innerWidth,
        overlap: !(p.right <= c.left || p.left >= c.right || p.bottom <= c.top || p.top >= c.bottom),
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
      };
    });

    expect(geometry).not.toBeNull();
    expect(geometry!.popupRight).toBeLessThanOrEqual(geometry!.viewportW);
    expect(geometry!.popupLeft).toBeGreaterThanOrEqual(0);
    expect(geometry!.overlap).toBe(false);
    expect(geometry!.scrollWidth).toBeLessThanOrEqual(geometry!.clientWidth);
    await page.screenshot({ path: `${SHOT_DIR}/web-t72-sort-mobile-ips-edge.png` });
    await page.keyboard.press("Escape");
  });

  test("viewport bottom collision: popup flips above the trigger instead of covering it", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 375, height: 240 });
    await openSort(page, "/original");

    await expect
      .poll(() =>
        page.evaluate(() => {
          const listbox = document.querySelector('[role="listbox"]');
          const popup = listbox?.parentElement;
          const combo = listbox instanceof HTMLElement
            ? document.querySelector(`[role="combobox"][aria-controls="${listbox.id}"]`)
            : null;
          if (!(popup instanceof HTMLElement) || !(combo instanceof HTMLElement)) return null;
          const p = popup.getBoundingClientRect();
          const c = combo.getBoundingClientRect();
          const overlap = !(p.right <= c.left || p.left >= c.right || p.bottom <= c.top || p.top >= c.bottom);
          return { overlap, fits: p.top >= 0 && p.bottom <= window.innerHeight };
        }),
      )
      .toMatchObject({ overlap: false, fits: true });

    const geometry = await page.evaluate(() => {
      const listbox = document.querySelector('[role="listbox"]');
      const popup = listbox?.parentElement;
      const combo = listbox instanceof HTMLElement
        ? document.querySelector(`[role="combobox"][aria-controls="${listbox.id}"]`)
        : null;
      if (!(popup instanceof HTMLElement) || !(combo instanceof HTMLElement)) return null;
      const p = popup.getBoundingClientRect();
      const c = combo.getBoundingClientRect();
      return {
        popupTop: p.top,
        popupBottom: p.bottom,
        triggerTop: c.top,
        triggerBottom: c.bottom,
        viewportH: window.innerHeight,
        overlap: !(p.right <= c.left || p.left >= c.right || p.bottom <= c.top || p.top >= c.bottom),
      };
    });

    expect(geometry).not.toBeNull();
    expect(geometry!.overlap).toBe(false);
    expect(geometry!.popupTop).toBeGreaterThanOrEqual(0);
    expect(geometry!.popupBottom).toBeLessThanOrEqual(geometry!.viewportH);
    await page.screenshot({ path: `${SHOT_DIR}/web-t72-sort-flip-above.png` });
    await page.keyboard.press("Escape");
  });

  test("mobile screenshot evidence", async ({ page }) => {
    await mockPublicApis(page);
    await page.setViewportSize({ width: 375, height: 844 });
    const { trigger } = await openSort(page, "/original");
    await expect(trigger).toBeVisible();
    await page.screenshot({ path: `${SHOT_DIR}/web-t72-sort-mobile.png` });
    await page.keyboard.press("Escape");
    await expect(page.getByRole("listbox")).toBeHidden();
  });
});
