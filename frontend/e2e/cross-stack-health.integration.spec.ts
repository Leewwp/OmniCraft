import { expect, test } from "@playwright/test";

const realApiBase = "http://127.0.0.1:8080";
const publicConfigEndpoint = `${realApiBase}/api/v1/config/public`;

test("browser-loaded frontend consumes the actual public backend configuration response", async ({ page }) => {
  const publicConfigResponse = page.waitForResponse((response) => (
    response.url() === publicConfigEndpoint
    && response.request().resourceType() === "fetch"
    && response.request().frame() === page.mainFrame()
  ));

  await page.goto("/");
  await expect(page.locator("body")).toBeVisible();

  const response = await publicConfigResponse;
  expect(response.status()).toBeGreaterThanOrEqual(200);
  expect(response.status()).toBeLessThan(300);
  await expect(response.json()).resolves.toEqual(expect.objectContaining({
    features: expect.any(Object),
    captcha: expect.any(Object),
  }));
});
