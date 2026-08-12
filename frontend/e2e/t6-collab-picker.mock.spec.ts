import { expect, test, type Page } from "@playwright/test";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

const CREATOR = {
  id: 5,
  email: "creator@example.com",
  username: "creator",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "zh",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-01-01T00:00:00Z",
  created_at: "2026-01-01T00:00:00Z",
};

const USER_RESULTS = [
  { id: 1, username: "Luminary", avatar_url: "https://cdn.example.test/1.png", reputation: 10, role: "creator" },
  { id: 2, username: "Lumina", avatar_url: "", reputation: 8, role: "user" },
  { id: 3, username: "Lumos", avatar_url: "", reputation: 6, role: "user" },
  { id: 4, username: "Luna", avatar_url: "", reputation: 5, role: "user" },
  { id: 5, username: "Lucien", avatar_url: "", reputation: 4, role: "user" },
];

function removeButton(page: Page, username: string) {
  return page.getByRole("button", { name: `移除 ${username}`, exact: true });
}

async function mockCollabPickerSession(page: Page) {
  await mockPublicApis(page);
  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", path: "/", domain: "127.0.0.1" }]);

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
      body: JSON.stringify({ user: CREATOR }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
    }),
  );

  await mockApiRoute(page, "**/api/v1/users/search?**", async (route) => {
    const url = new URL(route.request().url());
    const q = url.searchParams.get("q") ?? "";
    if (q.startsWith("err")) {
      await route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ code: "DB_ERROR", message: "user search failed" }),
      });
      return;
    }
    const users = q === "zoo" ? [] : USER_RESULTS;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ users, total: users.length }),
    });
  });
}

test("t6 collab picker: search, select chips, duplicate disabled, cap, remove, error, and empty states", async ({ page }) => {
  await mockCollabPickerSession(page);

  await page.goto("/studio/collab-picker-demo");
  await expect(page.getByRole("combobox", { name: "联合创作者" })).toBeVisible();

  const input = page.getByRole("combobox");
  await input.fill("lum");
  await expect(page.getByRole("option", { name: "Luminary", exact: true })).toBeVisible();
  await expect(page.getByRole("option", { name: "Lumina", exact: true })).toBeVisible();

  await page.getByRole("option", { name: "Luminary", exact: true }).click();
  await expect(removeButton(page, "Luminary")).toBeVisible();

  await input.fill("lum");
  const duplicate = page.getByRole("option").filter({ hasText: "已选择" });
  await expect(duplicate).toBeVisible();
  await expect(duplicate).toHaveAttribute("aria-disabled", "true");
  await expect(duplicate).toContainText("已选择");

  await page.getByRole("option", { name: "Lumina", exact: true }).click();
  await expect(removeButton(page, "Lumina")).toBeVisible();

  await page.screenshot({ path: `${SCREENSHOTS}/community-collab-picker-desktop.png`, fullPage: true });

  for (const username of ["Lumos", "Luna", "Lucien"]) {
    await input.fill(username);
    await page.getByRole("option", { name: username, exact: true }).click();
  }
  await expect(page.getByText("最多可选择 5 位联合创作者")).toBeVisible();
  await expect(page.getByRole("combobox")).toBeDisabled();

  await removeButton(page, "Lumina").click();
  await expect(page.getByRole("combobox")).toBeEnabled();

  await input.fill("err");
  await expect(page.getByText("搜索失败，请重试。")).toBeVisible();
  await page.getByRole("button", { name: "重试", exact: true }).click();

  await input.fill("zoo");
  await expect(page.getByText("未找到匹配用户。")).toBeVisible();
});

test.describe("mobile", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });

  test("t6 collab picker mobile: chips wrap and removal targets stay at least 44px tall", async ({ page }) => {
    await mockCollabPickerSession(page);

    await page.goto("/studio/collab-picker-demo");
    const input = page.getByRole("combobox");

    for (const username of ["Luminary", "Lumina", "Lumos"]) {
      await input.fill(username);
      await page.getByRole("option", { name: username, exact: true }).click();
    }

    for (const username of ["Luminary", "Lumina", "Lumos"]) {
      const remove = removeButton(page, username);
      await expect(remove).toBeVisible();
      const box = await remove.boundingBox();
      expect(box, `remove button for ${username} should have a bounding box`).not.toBeNull();
      expect(box!.height, `remove button for ${username} should be at least 44px tall on coarse pointers`).toBeGreaterThanOrEqual(44);
    }

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasHorizontalOverflow).toBe(false);

    await page.screenshot({ path: `${SCREENSHOTS}/community-collab-picker-mobile.png`, fullPage: true });
  });
});
