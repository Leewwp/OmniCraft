import type { Page, Route } from "@playwright/test";

type GuardState = {
  installed: boolean;
  allowedPatterns: string[];
};

const guardStates = new WeakMap<Page, GuardState>();

function getState(page: Page): GuardState {
  let state = guardStates.get(page);
  if (!state) {
    state = { installed: false, allowedPatterns: [] };
    guardStates.set(page, state);
  }
  return state;
}

function escapeRegex(value: string): string {
  return value.replace(/[|\\{}()[\]^$+?.*]/g, "\\$&");
}

function globToRegex(pattern: string): RegExp {
  const escaped = escapeRegex(pattern)
    .replace(/\\\*\\\*/g, ".*")
    .replace(/\\\*/g, "[^?]*")
    .replace(/\\\?/g, ".");
  return new RegExp(`^${escaped}$`);
}

export async function installMockedApiGuard(page: Page): Promise<void> {
  const state = getState(page);
  if (state.installed) {
    return;
  }

  await page.route("**/api/v1/**", async (route: Route) => {
    const url = route.request().url();
    const allowed = getState(page).allowedPatterns.some((pattern) => globToRegex(pattern).test(url));
    if (allowed) {
      await route.fallback();
      return;
    }

    await route.fulfill({
      status: 599,
      contentType: "application/json",
      body: JSON.stringify({
        code: "UNEXPECTED_TEST_API_CALL",
        message: `unexpected mocked API call: ${url}`,
      }),
    });
  });

  state.installed = true;
}

export async function mockApiRoute(page: Page, pattern: string, handler: Parameters<Page["route"]>[1]): Promise<void> {
  const state = getState(page);
  if (!state.allowedPatterns.includes(pattern)) {
    state.allowedPatterns.push(pattern);
  }
  await page.route(pattern, handler);
}
