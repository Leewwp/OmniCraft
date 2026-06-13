import { expect, test } from "@playwright/test";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";

let backendServer: ReturnType<typeof createServer> | null = null;

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, {
    "content-type": "application/json",
    "access-control-allow-origin": "http://127.0.0.1:3000",
    "access-control-allow-credentials": "true",
    "access-control-allow-headers": "Content-Type, X-CSRF-Token, Authorization",
    "access-control-allow-methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
  });
  res.end(JSON.stringify(body));
}

function handleApi(req: IncomingMessage, res: ServerResponse) {
  const url = new URL(req.url ?? "/", "http://127.0.0.1:8080");
  const pathname = url.pathname;

  if (req.method === "OPTIONS") {
    res.writeHead(204, {
      "access-control-allow-origin": "http://127.0.0.1:3000",
      "access-control-allow-credentials": "true",
      "access-control-allow-headers": "Content-Type, X-CSRF-Token, Authorization",
      "access-control-allow-methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
    });
    res.end();
    return;
  }

  if (pathname === "/api/v1/config/public") {
    json(res, 200, {
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
    return;
  }

  if (pathname === "/api/v1/auth/csrf") {
    json(res, 200, { csrf_token: "contract-csrf-token" });
    return;
  }

  if (pathname === "/api/v1/auth/refresh" || pathname === "/api/v1/auth/me" || pathname === "/api/v1/notifications/unread-count") {
    json(res, 401, { code: "UNAUTHORIZED", message: "unauthorized" });
    return;
  }

  if (pathname === "/api/v1/ips/stats/category_counts") {
    json(res, 200, { category_counts: {} });
    return;
  }

  if (pathname === "/api/v1/ips") {
    json(res, 200, { ips: [{ id: 42, name: "Star Rail", slug: "star-rail", status: "approved" }] });
    return;
  }

  if (pathname === "/api/v1/contents") {
    json(res, 200, {
      contents: [
        {
          id: 101,
          title: "Contract smoke content",
          zone: "fanwork",
          content_type: "image",
          like_count: 5,
          view_count: 20,
          author: { id: 7, username: "contract-author" },
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });
    return;
  }

  if (pathname === "/api/v1/contents/search") {
    const contentType = url.searchParams.get("content_type");
    const timeRange = url.searchParams.get("time_range");
    const q = url.searchParams.get("q");
    const items = contentType === "image" && timeRange === "week" && q === "layout repair"
      ? [{ id: 202, title: "Layout repair image", content_type: "image", zone: "fanwork", author: { id: 8, username: "search-author" } }]
      : [];
    json(res, 200, { items, total: items.length, page: 1, page_size: 20, total_pages: 1, time_range: timeRange ?? "all" });
    return;
  }

  if (pathname === "/api/v1/stats/summary") {
    json(res, 200, { summary: { users: 1, ips: 1, contents: 1 } });
    return;
  }

  if (pathname === "/api/v1/tags/faceted") {
    json(res, 200, { tags: [] });
    return;
  }

  json(res, 404, { code: "NOT_FOUND", message: `unexpected contract smoke endpoint: ${pathname}` });
}

test.describe("real HTTP contract smoke", () => {
  test.beforeAll(async () => {
    backendServer = createServer(handleApi);
    await new Promise<void>((resolve, reject) => {
      backendServer?.once("error", reject);
      backendServer?.listen(8080, () => resolve());
    });
  });

  test.afterAll(async () => {
    await new Promise<void>((resolve, reject) => {
      if (!backendServer) {
        resolve();
        return;
      }
      backendServer.close((error) => {
        if (error) {
          reject(error);
          return;
        }
        resolve();
      });
    });
    backendServer = null;
  });

  test("search page loads through real HTTP calls and preserves backend filter contract", async ({ page }) => {
    await page.goto("/search");
    await page.getByRole("button", { name: /Advanced filter|Advanced filters/i }).click();
    const filterDialog = page.getByRole("dialog", { name: /Advanced filters/i });
    const hasFilterDialog = await filterDialog.isVisible().catch(() => false);
    const filterScope = hasFilterDialog ? filterDialog : page;
    await filterScope.getByRole("button", { name: /Image/i }).click();
    await filterScope.getByLabel(/Time range/i).selectOption("week");
    if (hasFilterDialog) {
      await page.keyboard.press("Escape");
      await expect(filterDialog).toBeHidden();
    }
    await page.getByPlaceholder(/keyword/i).fill("layout repair");
    await page.getByRole("button", { name: /^Search$/i }).click();

    await expect(page.getByText("Layout repair image")).toBeVisible();
  });

  test("protected publish route redirects unauthenticated users to login without mocked routes", async ({ page }) => {
    await page.goto("/studio/publish/fanwork");
    await page.waitForURL(/\/login\?redirect=%2Fstudio%2Fpublish%2Ffanwork/);
    await expect(page).toHaveURL(/\/login\?redirect=%2Fstudio%2Fpublish%2Ffanwork/);
  });
});
