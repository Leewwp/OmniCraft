import { expect, test, type Page } from "@playwright/test";
import path from "node:path";
import { mockApiRoute } from "./helpers/mock-api-guard";
import { mockPublicApis } from "./helpers/mock-public-apis";

const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

const USER = {
  id: 1,
  email: "alice@example.test",
  username: "alice",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "zh",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-06-30T00:00:00Z",
  created_at: "2026-06-30T00:00:00Z",
  accept_collab_invites: true,
};

interface MergeResult {
  accepted_ip_ids: number[];
  discarded_ip_ids: number[];
  items: Array<{ ip: { id: number; name: string }; visited_at: string }>;
}

const SERVER_IP_VISITS = (ids: Array<{ id: number; name: string }>) => ({
  items: ids.map((ip, i) => ({
    ip,
    visited_at: `2026-08-12T0${10 - i}:00:00Z`,
  })),
  limit: 6,
});

async function mockSession(page: Page, options: { signedIn?: boolean; merge?: () => MergeResult | null } = {}) {
  await mockPublicApis(page);
  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", path: "/", domain: "127.0.0.1" }]);

  if (options.signedIn) {
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
        body: JSON.stringify({ user: USER }),
      }),
    );
    await mockApiRoute(page, "**/api/v1/notifications/unread-count", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } }),
      }),
    );
    await mockApiRoute(page, "**/api/v1/users/me/ip-visits/merge", (route) => {
      if (route.request().method() !== "POST") {
        route.fallback();
        return;
      }
      const result = options.merge?.() ?? {
        accepted_ip_ids: [],
        discarded_ip_ids: [],
        items: [],
      };
      if (result === null) {
        route.fulfill({
          status: 500,
          contentType: "application/json",
          body: JSON.stringify({ code: "IP_VISIT_MERGE_FAILED", message: "merge failed" }),
        });
        return;
      }
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(result),
      });
    });
    await mockApiRoute(page, "**/api/v1/users/me/ip-visits", (route) => {
      if (route.request().method() !== "GET") {
        route.fallback();
        return;
      }
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(SERVER_IP_VISITS([{ id: 1, name: "星尘" }, { id: 2, name: "觉醒者" }])),
      });
    });
  }
}

async function seedRecentIps(page: Page, items: Array<{ id: number; name: string }>) {
  await page.addInitScript((seed) => {
    localStorage.setItem("recent_ips", JSON.stringify(seed));
  }, items);
}

function recentLinks(page: Page) {
  return page.getByText("最近访问 IP").locator("..").locator("..").getByRole("link");
}

async function localRecentIps(page: Page): Promise<string | null> {
  return page.evaluate(() => localStorage.getItem("recent_ips"));
}

test("anonymous home shows deduped and capped local recent visits", async ({ page }) => {
  await mockSession(page);
  await page.setViewportSize({ width: 1280, height: 800 });
  await seedRecentIps(page, [
    { id: 1, name: "星尘" },
    { id: 2, name: "觉醒者" },
    { id: 1, name: "星尘" },
    { id: 3, name: "月面基地" },
    { id: 4, name: "机械纪元" },
    { id: 5, name: "雾都猎人" },
    { id: 6, name: "深海回响" },
    { id: 7, name: "云端花园" },
  ]);

  await page.goto("/");

  const links = recentLinks(page);
  await expect(links).toHaveCount(6);
  await expect(links.first()).toHaveText("星尘");
  await expect(links).toHaveText([
    "星尘",
    "觉醒者",
    "月面基地",
    "机械纪元",
    "雾都猎人",
    "深海回响",
  ]);
  await page.screenshot({ path: path.join(SCREENSHOTS, "web-t73-home-anonymous-recent.png"), fullPage: false });
});

test("signed-in recent list reads account history and the merge clears acknowledged local ids", async ({ page }) => {
  const mergeCalls: string[] = [];
  await mockSession(page, {
    signedIn: true,
    merge: () => {
      mergeCalls.push("merge");
      return {
        accepted_ip_ids: [1, 2],
        discarded_ip_ids: [3],
        items: SERVER_IP_VISITS([
          { id: 2, name: "觉醒者" },
          { id: 1, name: "星尘" },
        ]).items,
      };
    },
  });
  await page.setViewportSize({ width: 1280, height: 800 });
  await seedRecentIps(page, [
    { id: 1, name: "星尘" },
    { id: 2, name: "觉醒者" },
    { id: 3, name: "已下架作品" },
  ]);

  await page.goto("/");

  await expect(page.getByText("最近访问 IP")).toBeVisible();
  await expect.poll(() => mergeCalls.length).toBeGreaterThanOrEqual(1);
  // Only server-acknowledged ids (accepted + discarded) are removed locally.
  await expect.poll(async () => localRecentIps(page)).toBe("[]");
  await expect(recentLinks(page)).toHaveCount(2);
  await page.screenshot({ path: path.join(SCREENSHOTS, "web-t73-home-signed-in-merged.png"), fullPage: false });
});

test("merge failure retains local records until a retry succeeds", async ({ page }) => {
  let mergeMode: "fail" | "ok" = "fail";
  await mockSession(page, {
    signedIn: true,
    merge: () => {
      if (mergeMode === "fail") {
        return null as unknown as MergeResult;
      }
      return {
        accepted_ip_ids: [1],
        discarded_ip_ids: [],
        items: SERVER_IP_VISITS([{ id: 1, name: "星尘" }]).items,
      };
    },
  });
  await page.setViewportSize({ width: 1280, height: 800 });
  await seedRecentIps(page, [{ id: 1, name: "星尘" }]);

  await page.goto("/");
  await expect.poll(async () => localRecentIps(page)).toBe(JSON.stringify([{ id: 1, name: "星尘" }]));

  mergeMode = "ok";
  await page.reload();
  await expect.poll(async () => localRecentIps(page)).toBe("[]");
  await page.screenshot({ path: path.join(SCREENSHOTS, "web-t73-home-merge-retry-success.png"), fullPage: false });
});

test("signed-in recent list stays visible on mobile", async ({ page }) => {
  await mockSession(page, { signedIn: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await seedRecentIps(page, [
    { id: 1, name: "星尘" },
    { id: 2, name: "觉醒者" },
  ]);

  await page.goto("/");

  await expect(recentLinks(page)).toHaveCount(2);
  await expect(recentLinks(page).first()).toBeVisible();
  await page.screenshot({ path: path.join(SCREENSHOTS, "web-t73-home-mobile.png"), fullPage: false });
});
