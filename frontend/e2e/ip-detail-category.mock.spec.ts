import { expect, test, type Page } from "@playwright/test";
import { createServer, type Server } from "node:http";

const IP = {
  id: 1,
  name: "星尘计划",
  description: "mock ip for category in-place switching",
  category: "anime",
  tags: [],
  follower_count: 3,
  is_following: false,
};

function contentItem(id: number, title: string, contentType: string) {
  return {
    id,
    title,
    content_type: contentType,
    zone: "fanwork",
    status: "published",
    author: { id: 2, username: "alice", avatar_url: "" },
    created_at: "2026-08-01T00:00:00Z",
    view_count: id * 10,
    like_count: 1,
  };
}

// The IP detail page fetches on the server for SSR, which page.route cannot
// intercept. The mocked webServer points both the browser and Next SSR at
// http://127.0.0.1:18080, so a local stub serves both sides.
let stub: Server;

test.beforeAll(async () => {
  stub = createServer((req, res) => {
    const url = new URL(req.url ?? "/", "http://127.0.0.1:18080");
    const send = (payload: unknown, status = 200) => {
      res.writeHead(status, {
        "Content-Type": "application/json",
        "Access-Control-Allow-Origin": "*",
        "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
        "Access-Control-Allow-Headers": "*",
        "Access-Control-Max-Age": "86400",
      });
      res.end(JSON.stringify(payload));
    };

    // The page origin is :3001 while this stub is :18080, so browser-side
    // fetches (and their CSRF preflights) need permissive CORS headers.
    if (req.method === "OPTIONS") {
      return send({}, 204);
    }

    if (url.pathname === "/api/v1/ips/1") {
      return send({ ip: IP });
    }
    if (url.pathname === "/api/v1/ips/1/discussions") {
      return send({ discussions: [], total: 0 });
    }
    if (url.pathname === "/api/v1/config/public") {
      return send({
        features: {
          web_agent_enabled: false,
          desktop_deploy_enabled: false,
          creator_support_enabled: false,
          payment_enabled: false,
        },
        captcha: { provider: "bypass", prefix: "", scene_id: "", region: "cn" },
        client: { download_enabled: false, download_url: "", latest_version: "" },
        legal: { current_terms_version: "test", current_privacy_version: "test" },
      });
    }
    if (url.pathname === "/api/v1/contents") {
      const contentType = url.searchParams.get("content_type") || "all";
      if (contentType === "image") {
        return send({ contents: [contentItem(101, "星尘图片精选集", "image")], total: 1 });
      }
      if (contentType === "video") {
        return send({ contents: [contentItem(103, "星尘视频日志", "video")], total: 1 });
      }
      return send({
        contents: [contentItem(100, "星尘全部内容样板", "image"), contentItem(102, "星尘视频日志", "video")],
        total: 2,
      });
    }
    return send({ code: "NOT_FOUND", message: "stub" }, 404);
  });
  if (process.env.U03_REUSE_STUB !== "1") { await new Promise<void>((resolve) => stub.listen(18080, "127.0.0.1", resolve)); }
});

test.afterAll(async () => {
  await new Promise<void>((resolve) => stub.close(() => resolve()));
});

async function prepare(page: Page) {
  await page.context().addCookies([{ name: "NEXT_LOCALE", value: "zh", url: "http://127.0.0.1:3001" }]);
  await page.setViewportSize({ width: 1440, height: 900 });
}

test("ip detail category pills switch in place, sync the URL, and survive refresh", async ({ page }) => {
  await prepare(page);

  await page.goto("/ip/1");
  const pills = page.getByRole("navigation", { name: "内容类目" });
  await expect(pills.getByRole("button", { name: "全部", pressed: true })).toBeVisible();
  await expect(page.getByText("星尘全部内容样板")).toBeVisible();

  // Scroll down so scroll preservation is observable, then switch in place.
  await page.evaluate(() => window.scrollTo(0, 400));
  const scrollBefore = await page.evaluate(() => window.scrollY);

  await pills.getByRole("button", { name: "图片" }).click();
  await expect(page).toHaveURL(/\/ip\/1\?category=image$/);
  await expect(pills.getByRole("button", { name: "图片", pressed: true })).toBeVisible();
  await expect(page.getByText("星尘图片精选集")).toBeVisible();
  await expect(page.getByText("星尘全部内容样板")).toHaveCount(0);

  // router.replace must not scroll and must not push a history entry.
  expect(await page.evaluate(() => window.scrollY)).toBe(scrollBefore);

  // Refresh restores the category from the URL query (SSR honors it).
  await page.reload();
  await expect(pills.getByRole("button", { name: "图片", pressed: true })).toBeVisible();
  await expect(page.getByText("星尘图片精选集")).toBeVisible();
});

test("legacy /ip/[id]/[category] route redirects to the query form", async ({ page }) => {
  await prepare(page);

  await page.goto("/ip/1/image");
  await expect(page).toHaveURL(/\/ip\/1\?category=image$/);

  await page.goto("/ip/1/video?sort=newest");
  await expect(page).toHaveURL(/\/ip\/1\?category=video&sort=newest$/);
});
