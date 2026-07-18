import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";

const missingSeriesID = 9_007_199_254_740_000;
const realApiBase = "http://127.0.0.1:8080/api/v1";
const realSeriesEndpoint = `http://127.0.0.1:8080/api/v1/series/${missingSeriesID}`;

function seedRealSeriesFixture() {
  const dsn = process.env.OMNICRAFT_TEST_DB_DSN ?? "";
  const database = dsn.match(/(?:^|\s)dbname=([^\s]+)/)?.[1];
  const composeRoot = process.env.OMNICRAFT_TEST_COMPOSE_ROOT;
  if (!database || !composeRoot) throw new Error("CrossStack fixture requires OMNICRAFT_TEST_DB_DSN and OMNICRAFT_TEST_COMPOSE_ROOT");
  const sql = `
    INSERT INTO users (id, email, password_hash, username, email_verified_at)
    VALUES (9001, 'series-cross-stack@example.test', '$2b$04$cJeEvDtMz4Ax2FagQjVL2.e1HpENkguyfl1BAIbbMfJmI4PYLVsk2', 'series-cross-stack', NOW())
    ON CONFLICT (id) DO NOTHING;
    INSERT INTO content_items (id, title, author_id, zone, category, content_type, status, is_public)
    VALUES
      (9101, 'Cross-stack first chapter', 9001, 'original', 'literature', 'article', 'published', TRUE),
      (9102, 'Cross-stack second chapter', 9001, 'original', 'literature', 'article', 'published', TRUE),
      (9103, 'Cross-stack candidate chapter', 9001, 'original', 'literature', 'article', 'published', TRUE)
    ON CONFLICT (id) DO NOTHING;
    INSERT INTO content_series (id, title, description, owner_id, zone)
    VALUES
      (9201, 'Cross-stack series', 'Real backend fixture', 9001, 'original'),
      (9202, 'Cross-stack companion series', 'Second real membership', 9001, 'original')
    ON CONFLICT (id) DO NOTHING;
    INSERT INTO content_series_items (series_id, content_item_id, sort_order)
    VALUES (9201, 9101, 0), (9201, 9102, 1), (9202, 9101, 0)
    ON CONFLICT (series_id, content_item_id) DO NOTHING;
  `;
  execFileSync("docker", ["compose", "-f", `${composeRoot}/docker-compose.yml`, "--project-directory", composeRoot, "exec", "-T", "postgres", "psql", "-v", "ON_ERROR_STOP=1", "-U", "omnicraft", "-d", database, "-c", sql], { stdio: "pipe" });
}

test.beforeAll(() => {
  seedRealSeriesFixture();
});

test("public series page renders the real backend not-found response without route mocks", async ({ page }) => {
  const responsePromise = page.waitForResponse((response) => response.url() === realSeriesEndpoint);

  await page.goto(`/series/${missingSeriesID}`);

  const response = await responsePromise;
  expect(response.status()).toBe(404);
  const unavailable = page.getByText("Series not found or unavailable", { exact: true });
  await expect(unavailable).toBeVisible();
  await expect(unavailable.locator("xpath=ancestor::main[1]").getByRole("link")).toHaveCount(0);
});

test("public series page renders a real backend series and preserves item order", async ({ page }) => {
  const responsePromise = page.waitForResponse((response) => response.url() === "http://127.0.0.1:8080/api/v1/series/9201");

  await page.goto("/series/9201");

  const response = await responsePromise;
  expect(response.status()).toBe(200);
  await expect(page.getByRole("heading", { name: "Cross-stack series" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Cross-stack first chapter" })).toHaveAttribute("href", "/original/9101");
  await expect(page.getByRole("link", { name: "Cross-stack second chapter" })).toHaveAttribute("href", "/original/9102");
});

test("real content detail renders multiple SeriesNav memberships and first/last boundaries", async ({ page }) => {
  await page.goto("/original/9101");

  await expect(page.getByText("Cross-stack series")).toBeVisible();
  await expect(page.getByRole("tab", { name: "Cross-stack companion series" })).toBeVisible();
  await page.getByRole("tab", { name: "Cross-stack series", exact: true }).click();
  await expect(page.getByRole("button", { name: /Previous chapter unavailable; this is the first chapter/i })).toBeDisabled();
  await expect(page.getByRole("link", { name: /Next chapter: Cross-stack second chapter/i })).toHaveAttribute("href", "/original/9102");

  await page.goto("/original/9102");
  await expect(page.getByRole("button", { name: /Next chapter unavailable; this is the last chapter/i })).toBeDisabled();
});

test("studio series completes real create, add, reorder, remove, and delete flow", async ({ page }) => {
  const csrfResponse = await page.request.get(`${realApiBase}/auth/csrf`);
  expect(csrfResponse.ok()).toBeTruthy();
  const csrf = await csrfResponse.json() as { csrf_token: string };
  const login = await page.request.post(`${realApiBase}/auth/login`, {
    data: { email: "series-cross-stack@example.test", password: "cross-stack-password" },
    headers: { "X-CSRF-Token": csrf.csrf_token },
  });
  if (!login.ok()) throw new Error(`login failed: ${login.status()} ${await login.text()}`);

  await page.goto("/studio/series");
  await expect(page.getByRole("heading", { name: "Content series" })).toBeVisible();
  await page.getByRole("button", { name: "Create series" }).click();
  const createForm = page.locator("form");
  await createForm.getByLabel("Series title").fill("Cross-stack mutable series");
  await createForm.getByLabel("Series description").fill("Real mutation flow");
  await createForm.getByRole("button", { name: "Create", exact: true }).click();
  await expect(page.getByRole("button", { name: /Cross-stack mutable series/ })).toBeVisible();

  await page.getByLabel("Search content to add").fill("Cross-stack first chapter");
  await page.getByText("Cross-stack first chapter", { exact: true }).locator("..") .getByRole("button", { name: "Add" }).click();
  await expect(page.getByRole("list", { name: "Series content ordering list" }).getByText("Cross-stack first chapter", { exact: true })).toBeVisible();

  await page.getByLabel("Search content to add").fill("Cross-stack second chapter");
  await page.getByText("Cross-stack second chapter", { exact: true }).locator("..") .getByRole("button", { name: "Add" }).click();
  const ordering = page.getByRole("list", { name: "Series content ordering list" });
  await expect(ordering.getByText("Cross-stack second chapter", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Move Cross-stack second chapter up" }).click();
  await expect(ordering.locator("li").first()).toContainText("Cross-stack second chapter");
  await page.getByRole("button", { name: "Remove Cross-stack first chapter from series" }).click();
  await expect(ordering.getByText("Cross-stack first chapter", { exact: true })).toHaveCount(0);

  await page.getByRole("button", { name: "Delete series" }).click();
  await page.getByRole("button", { name: "Delete series" }).last().click();
  await expect(page.getByRole("button", { name: /Cross-stack mutable series/ })).toHaveCount(0);
});
