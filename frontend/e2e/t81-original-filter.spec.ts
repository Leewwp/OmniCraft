import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

const PROXY_LOG = "/tmp/omnicraft-t81-proxy.log";
const SCREENSHOTS = path.join(process.cwd(), "..", "screenshots");

interface ProxyEntry {
  ts: string;
  method: string;
  url: string;
  status: number;
  body?: { contents?: Array<{ category: string }> } | null;
}

async function proxyEntriesSince(marker: number): Promise<ProxyEntry[]> {
  const raw = await readFile(PROXY_LOG, "utf8");
  const lines = raw.trim().split("\n");
  return lines
    .slice(marker)
    .filter((l) => l.trim())
    .map((l) => JSON.parse(l) as ProxyEntry);
}

async function waitForContentsEntry(
  marker: number,
  predicate: (e: ProxyEntry) => boolean,
): Promise<ProxyEntry> {
  let deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const entries = await proxyEntriesSince(marker);
    const hit = entries.find(predicate);
    if (hit) return hit;
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error("no matching /contents proxy entry within 15s");
}

test("#81 original zone category filter narrows the stream and never mixes recommended with filters", async ({ page }) => {
  const raw = await readFile(PROXY_LOG, "utf8");
  const marker = raw.trim() ? raw.trim().split("\n").length : 0;

  await page.goto("/original");
  await expect(page).toHaveURL(/\/original$/);
  await expect(page.getByRole("button", { name: "Recommended", pressed: true })).toBeVisible();
  await expect(page.locator("article.group").first()).toBeVisible({ timeout: 15_000 });
  const initialCardCount = await page.locator("article.group").count();
  expect(initialCardCount).toBeGreaterThanOrEqual(5);
  await page.screenshot({ path: path.join(SCREENSHOTS, "t81-original-before-tab-click.png"), fullPage: true });

  const recommendedEntry = await waitForContentsEntry(marker, (e) => e.url.includes("zone=original") && e.url.includes("sort=recommended"));
  expect(recommendedEntry.status).toBe(200);
  const recommendedCategories = new Set((recommendedEntry.body?.contents ?? []).map((c) => c.category));
  expect(recommendedCategories.size).toBeGreaterThan(1);

  await page.getByRole("button", { name: "Film & TV" }).click();
  await expect(page).toHaveURL(/category=film_tv/);
  await expect(page.getByRole("button", { name: "Film & TV", pressed: true })).toBeVisible();

  const filteredEntry = await waitForContentsEntry(marker, (e) => e.url.includes("category=film_tv"));
  expect(filteredEntry.url).toContain("sort=hot");
  expect(filteredEntry.url).not.toContain("sort=recommended");
  expect(filteredEntry.status).toBe(200);
  const categories = new Set((filteredEntry.body?.contents ?? []).map((c) => c.category));
  expect(categories).toEqual(new Set(["film_tv"]));
  expect(filteredEntry.body?.contents?.length ?? 0).toBeGreaterThan(0);

  await page.screenshot({ path: path.join(SCREENSHOTS, "t81-original-after-tab-click.png"), fullPage: true });

  const staleDeepLinkMarker = await new Promise<number>((resolve) => {
    readFile(PROXY_LOG, "utf8").then((r) => resolve(r.trim() ? r.trim().split("\n").length : 0));
  });
  await page.goto("/original?category=film_tv&sort=recommended");
  await expect(page).toHaveURL(/category=film_tv&sort=recommended/);
  const staleEntry = await waitForContentsEntry(staleDeepLinkMarker, (e) => e.url.includes("category=film_tv") && e.url.includes("sort=recommended"));
  expect(staleEntry.status).toBe(200);
  const staleCategories = new Set((staleEntry.body?.contents ?? []).map((c) => c.category));
  expect(staleCategories).toEqual(new Set(["film_tv"]));
  await page.screenshot({ path: path.join(SCREENSHOTS, "t81-original-stale-deeplink.png"), fullPage: true });
});
