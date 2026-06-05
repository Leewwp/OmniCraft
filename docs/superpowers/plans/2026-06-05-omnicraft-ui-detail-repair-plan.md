# OmniCraft UI Detail Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair the UI detail issues found in the frontend audit: broken mobile layouts, misleading controls, missing feedback states, inconsistent form/select/switch behavior, and search/publish interactions that appear functional but do not complete the intended action.

**Architecture:** Keep the existing Next.js app structure and Tailwind token system. Fix user-visible breakages first, then introduce small reusable UI primitives so future pages stop hand-rolling inputs, selects, switches, errors, and empty states. Avoid broad visual redesign; align implementation with `design/ui-spec.md` and the existing flat 1px border OmniCraft vocabulary.

**Tech Stack:** Next.js 16, React 19, TypeScript strict mode, Tailwind CSS 4, next-intl, @base-ui/react, lucide-react, Playwright.

---

## Scope And Ground Rules

- This plan is for frontend UI repair only. Do not update Beta roadmap checkboxes or `task.json` from this plan unless a later task explicitly adopts it as a tracked implementation task.
- Preserve existing i18n rules: all new UI strings must go through `frontend/messages/zh.json` and `frontend/messages/en.json`.
- Preserve existing API behavior unless a UI currently drops required data. The known dropped data is `source_original_id` in the fanwork publish form, and the existing fanwork IP text field sends `ip_name` even though the backend publish contract accepts `ip_id`.
- Do not introduce decorative redesigns. The target style remains flat, compact, border-based, and tool-like.
- Do not replace browser semantics with fake controls unless the replacement is fully keyboard and screen-reader accessible.
- Before each implementation task, read `design/ui-spec.md` and use `rg`/`Select-String` to find any matching `## Component:` or `## Page:` sections for the files being changed. If a matching section exists, it is the visual authority for that task.
- Use exact staging only. Commit once per task.

## File Structure

Create or modify the following files over the course of the repair.

### Test Harness

- Create: `frontend/playwright.config.ts`  
  Local Playwright test config for responsive UI and interaction regressions.
- Create: `frontend/e2e/helpers/mock-public-apis.ts`  
  Shared API mocks for UI tests that do not require the Go backend.
- Create: `frontend/e2e/ui-layout.spec.ts`  
  Home/search mobile layout tests and smoke checks for console errors under mocked API conditions.
- Create: `frontend/e2e/search-filter-contract.spec.ts`  
  Search filter request contract tests.
- Modify: `frontend/package.json`  
  Add `test:e2e` script and Playwright dependency.
- Modify: `frontend/package-lock.json`  
  Lockfile update after adding Playwright.

### UI Primitives

- Create: `frontend/components/ui/field.tsx`  
  `Field`, `FieldLabel`, `FieldHint`, `FieldError` helpers with `aria-describedby` conventions.
- Modify: `frontend/components/ui/textarea.tsx`  
  Align focus, invalid, disabled, placeholder, and height behavior with `Input`.
- Create: `frontend/components/ui/select.tsx`  
  Canonical styled native select wrapper for simple single-choice selects.
- Create: `frontend/components/ui/checkbox.tsx`  
  Canonical checkbox wrapper for auth/settings forms.
- Create: `frontend/components/ui/switch.tsx`  
  Canonical switch with keyboard support, visible focus, `aria-checked`, and label integration.
- Create: `frontend/components/ui/empty-state.tsx`  
  Shared empty/error state component for lists and API failures.

### Search And Layout

- Inspect only: `frontend/components/layout/Sidebar.tsx`  
  Confirm it already accepts `className`; do not edit it unless the caller className cannot hide it.
- Modify: `frontend/components/home/HomePageClient.tsx`  
  Hide sidebar on mobile, make main content full-width, and add useful empty/error states.
- Modify: `frontend/app/(public)/search/page.tsx`  
  Fix mobile stack layout, search drawer semantics, filter config normalization, and keyword search endpoint.
- Modify: `frontend/components/layout/FacetedSearchSidebar.tsx`  
  Emit normalized search filter fields and use canonical selects/buttons.
- Create: `frontend/lib/search-filters.ts`  
  Normalize snake_case and camelCase filter data and build `/api/v1/contents/search` query params.

### Auth, Captcha, And Error Feedback

- Modify: `frontend/app/(public)/login/page.tsx`
- Modify: `frontend/app/(public)/register/page.tsx`
- Modify: `frontend/app/(public)/forgot-password/page.tsx`
- Modify: `frontend/app/(public)/reset-password/page.tsx`
- Modify: `frontend/app/(public)/verify-email/pending/page.tsx`
- Modify: `frontend/components/verification/CaptchaWidget.tsx`
- Create: `frontend/lib/user-facing-error.ts`

### Publish And Admin Controls

- Modify: `frontend/components/studio/PublishForm.tsx`
- Create: `frontend/components/studio/IPPicker.tsx`
- Create: `frontend/components/studio/SourceOriginalPicker.tsx`
- Modify: `frontend/app/(protected)/admin/categories/page.tsx`
- Modify: `frontend/components/admin/AdminFilterBar.tsx`
- Modify: `frontend/app/(protected)/admin/audit-logs/page.tsx`
- Modify: `frontend/app/(protected)/admin/feedback/page.tsx`

### Visual Drift Cleanup

- Modify: `frontend/components/home/prototype-landing.tsx`
- Modify: `frontend/components/content/ContentCard.tsx`
- Modify: `frontend/components/content/MasonryGrid.tsx`
- Modify: `frontend/components/content/FileUploader.tsx`
- Modify: `frontend/components/social/NotificationDropdown.tsx`
- Modify: `frontend/components/content/VersionHistory.tsx`
- Modify: `frontend/app/(protected)/history/page.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`

---

## Task 1: Add UI Regression Test Harness

**Files:**
- Create: `frontend/playwright.config.ts`
- Create: `frontend/e2e/helpers/mock-public-apis.ts`
- Create: `frontend/e2e/ui-layout.spec.ts`
- Create: `frontend/e2e/search-filter-contract.spec.ts`
- Modify: `frontend/package.json`
- Modify: `frontend/package-lock.json`

- [ ] **Step 1: Add Playwright dependency and scripts**

Run:

```bash
cd frontend
npm install -D @playwright/test
npx playwright install chromium
```

Modify `frontend/package.json` scripts:

```json
{
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "lint": "tsc --noEmit",
    "test:e2e": "playwright test"
  }
}
```

- [ ] **Step 2: Create Playwright config**

Create `frontend/playwright.config.ts`:

```ts
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: true,
  webServer: {
    command: "npm run dev -- --hostname 127.0.0.1 --port 3000",
    url: "http://127.0.0.1:3000",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  use: {
    baseURL: "http://127.0.0.1:3000",
    trace: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile-chrome", use: { ...devices["Pixel 5"] } },
  ],
});
```

- [ ] **Step 3: Write failing mobile layout tests**

Create `frontend/e2e/helpers/mock-public-apis.ts`:

```ts
import type { Page } from "@playwright/test";

export async function mockPublicApis(page: Page) {
  await page.route("**/api/v1/config/public", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        features: { web_agent_enabled: false, desktop_deploy_enabled: false },
        captcha: { provider: "bypass" },
        legal: { current_terms_version: "test", current_privacy_version: "test" },
      }),
    }),
  );
  await page.route("**/api/v1/auth/refresh", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ code: "UNAUTHORIZED", message: "unauthorized" }),
    }),
  );
  await page.route("**/api/v1/ips/stats/category_counts", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ category_counts: {} }) }),
  );
  await page.route("**/api/v1/ips?**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ips: [] }) }),
  );
  await page.route("**/api/v1/contents/search?**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [], total: 0 }) }),
  );
  await page.route("**/api/v1/contents?**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ contents: [], total: 0 }) }),
  );
  await page.route("**/api/v1/stats/summary", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ summary: { users: 0, ips: 0, contents: 0 } }) }),
  );
  await page.route("**/api/v1/tags/faceted?**", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ tags: [] }) }),
  );
}
```

Create `frontend/e2e/ui-layout.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { mockPublicApis } from "./helpers/mock-public-apis";

test("home uses full-width content and hides desktop sidebar on mobile", async ({ page }) => {
  await mockPublicApis(page);
  await page.setViewportSize({ width: 375, height: 844 });
  await page.goto("/");

  await expect(page.getByRole("button", { name: /收起侧边栏|Collapse sidebar/ })).toBeHidden();
  const main = page.locator("[data-testid='home-main-content']");
  await expect(main).toBeVisible();
  const box = await main.boundingBox();
  expect(box?.width).toBeGreaterThan(330);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  expect(overflow).toBe(false);
});

test("search stacks controls vertically and opens an accessible mobile filter dialog", async ({ page }) => {
  await mockPublicApis(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/search");

  const results = page.locator("[data-testid='search-results-panel']");
  await expect(results).toBeVisible();
  const box = await results.boundingBox();
  expect(box?.width).toBeGreaterThan(340);

  await page.getByRole("button", { name: /打开高级筛选|Open advanced filters/ }).click();
  await expect(page.getByRole("dialog", { name: /高级筛选|Advanced filters/ })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog", { name: /高级筛选|Advanced filters/ })).toBeHidden();
});
```

- [ ] **Step 4: Write failing search contract test**

Create `frontend/e2e/search-filter-contract.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { mockPublicApis } from "./helpers/mock-public-apis";

test("search sends selected filters in backend query parameter names", async ({ page }) => {
  await mockPublicApis(page);
  const contentRequests: string[] = [];
  await page.route("**/api/v1/contents/search?**", (route) => {
    contentRequests.push(route.request().url());
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [], total: 0 }),
    });
  });

  await page.goto("/search");
  await page.getByRole("button", { name: /高级筛选|Advanced filter/ }).click();
  await page.getByRole("button", { name: /图片|Image/ }).click();
  await page.getByLabel(/时间范围|Time range/).selectOption("week");
  await page.getByPlaceholder(/关键词|keyword/i).fill("测试");
  await page.getByRole("button", { name: /搜索|Search/ }).click();

  await expect.poll(() => contentRequests.at(-1) ?? "").toContain("content_type=image");
  expect(contentRequests.at(-1)).toContain("/api/v1/contents/search?");
  expect(contentRequests.at(-1)).toContain("time_range=week");
});
```

- [ ] **Step 5: Run tests to verify they fail against current UI**

Run:

```bash
cd frontend
npm run test:e2e -- --project=mobile-chrome frontend/e2e/ui-layout.spec.ts
npm run test:e2e -- --project=chromium frontend/e2e/search-filter-contract.spec.ts
```

Expected: FAIL because home mobile sidebar remains visible, search mobile result panel is too narrow, filter dialog lacks accessible semantics, and search filter fields are mismatched.

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/playwright.config.ts frontend/e2e/helpers/mock-public-apis.ts frontend/e2e/ui-layout.spec.ts frontend/e2e/search-filter-contract.spec.ts
git commit -m "test: add UI detail regression coverage"
```

---

## Task 2: Fix Home Mobile Layout And Empty/Error Feedback

**Files:**
- Modify: `frontend/components/home/HomePageClient.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Confirm failing home mobile test**

Run:

```bash
cd frontend
npm run test:e2e -- --project=mobile-chrome frontend/e2e/ui-layout.spec.ts -g "home uses full-width"
```

Expected: FAIL.

- [ ] **Step 2: Hide sidebar below `md` from the caller**

Modify `frontend/components/home/HomePageClient.tsx` around the root layout:

```tsx
return (
  <div className="mx-auto flex w-full max-w-[1440px] min-h-[calc(100vh-52px)]">
    <Sidebar
      className="hidden md:block"
      sections={sidebarSections}
      trending={{ title: t("home.trendingIpsThisWeek"), entries: trendingEntries }}
    />

    <div data-testid="home-main-content" className="min-w-0 flex-1">
      {/* existing main content */}
    </div>
  </div>
);
```

Do not make `Sidebar` globally hidden on mobile because other callers may want a drawer later. The caller owns the responsive behavior.

- [ ] **Step 3: Make mobile content header resilient**

In `HomePageClient.tsx`, change narrow horizontal stats to wrap:

```tsx
<div className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
  {/* stats */}
</div>
```

Ensure the content toolbar does not reserve horizontal space for an empty category strip:

```tsx
<div className="sticky top-[52px] z-40 bg-background px-4 py-2.5 md:px-6">
  <div className="flex min-w-0 items-center gap-2">
    <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto" style={{ scrollbarWidth: "none" }}>
      {/* type buttons */}
    </div>
    <div className="shrink-0">
      {/* sort select */}
    </div>
  </div>
</div>
```

- [ ] **Step 4: Add user-visible API fallback states**

Add local state:

```tsx
const [contentError, setContentError] = useState(false);
const [ipError, setIpError] = useState(false);
```

When IP/content fetches fail, set the corresponding error flag instead of silently swallowing:

```tsx
.catch(() => {
  setIpError(true);
});
```

Render compact messages:

```tsx
{ipError && (
  <div className="rounded-md border border-border bg-card px-3 py-2 text-xs text-muted-foreground">
    {t("home.ipLoadFailed")}
  </div>
)}

{contentError ? (
  <div className="rounded-md border border-border bg-card p-8 text-center text-sm text-muted-foreground">
    {t("home.contentLoadFailed")}
  </div>
) : (
  <MasonryGrid items={contents} emptyText={t("home.noOriginalContent")} />
)}
```

- [ ] **Step 5: Add translations**

Add to `frontend/messages/zh.json`:

```json
{
  "home": {
    "ipLoadFailed": "IP 内容暂时加载失败，请稍后重试",
    "contentLoadFailed": "内容暂时加载失败，请稍后重试"
  }
}
```

Add to `frontend/messages/en.json`:

```json
{
  "home": {
    "ipLoadFailed": "IP content could not be loaded. Please try again later.",
    "contentLoadFailed": "Content could not be loaded. Please try again later."
  }
}
```

Merge these keys into the existing `home` objects rather than replacing them.

- [ ] **Step 6: Verify**

Run:

```bash
cd frontend
npm run test:e2e -- --project=mobile-chrome frontend/e2e/ui-layout.spec.ts -g "home uses full-width"
npm run lint
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/components/home/HomePageClient.tsx frontend/messages/zh.json frontend/messages/en.json
git commit -m "fix: repair home mobile layout"
```

---

## Task 3: Fix Search Mobile Layout And Drawer Semantics

**Files:**
- Modify: `frontend/app/(public)/search/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Confirm failing search mobile test**

Run:

```bash
cd frontend
npm run test:e2e -- --project=mobile-chrome frontend/e2e/ui-layout.spec.ts -g "search stacks"
```

Expected: FAIL.

- [ ] **Step 2: Change search layout from always-row to mobile-column**

Modify the container around filter/results:

```tsx
<div className="flex flex-col gap-4 md:flex-row">
  <div className="md:hidden">
    <Button
      variant="outline"
      size="sm"
      onClick={() => setFilterDrawerOpen(true)}
      className="w-full"
      aria-label={t("search.filter.openAdvancedFilter")}
    >
      <SlidersHorizontal className="mr-1.5 h-4 w-4" />
      {t("search.filter.advancedFilter")}
    </Button>
  </div>

  <aside className="hidden w-[260px] shrink-0 md:block">
    <FacetedSearchSidebar onFilterChange={handleFilterChange} />
  </aside>

  <main data-testid="search-results-panel" className="min-w-0 flex-1 space-y-4">
    {/* existing result content */}
  </main>
</div>
```

- [ ] **Step 3: Make mobile filter drawer a real dialog**

Replace the mobile drawer wrapper:

```tsx
{filterDrawerOpen && (
  <div className="fixed inset-0 z-50 md:hidden">
    <button
      type="button"
      aria-label={t("common.close")}
      className="absolute inset-0 bg-black/40"
      onClick={closeFilterDrawer}
    />
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="mobile-filter-title"
      className="absolute bottom-0 left-0 right-0 max-h-[85vh] overflow-y-auto rounded-t-lg border-t border-border bg-background p-4"
      onKeyDown={(e) => {
        if (e.key === "Escape") closeFilterDrawer();
      }}
    >
      <div className="mb-3 flex items-center justify-between">
        <span id="mobile-filter-title" className="text-sm font-semibold">
          {t("search.filter.advancedFilter")}
        </span>
        <button
          type="button"
          ref={closeFilterButtonRef}
          onClick={closeFilterDrawer}
          className="rounded-md p-1 hover:bg-muted focus:outline-none focus:ring-2 focus:ring-ring"
          aria-label={t("common.close")}
        >
          <X className="h-4 w-4" />
        </button>
      </div>
      <FacetedSearchSidebar onFilterChange={handleFilterChange} />
    </div>
  </div>
)}
```

Add focus restore behavior in the same step:

```tsx
const openFilterButtonRef = useRef<HTMLButtonElement | null>(null);
const closeFilterButtonRef = useRef<HTMLButtonElement | null>(null);

useEffect(() => {
  if (filterDrawerOpen) {
    closeFilterButtonRef.current?.focus();
  }
}, [filterDrawerOpen]);

function closeFilterDrawer() {
  setFilterDrawerOpen(false);
  requestAnimationFrame(() => openFilterButtonRef.current?.focus());
}
```

Attach `ref={openFilterButtonRef}` to the mobile filter opener. Use `closeFilterDrawer()` for overlay click, close button click, and Escape. Do not add a new focus-trap dependency in this task; this repair pass requires explicit focus entry, Escape close, and focus restoration.

- [ ] **Step 4: Add translations**

Add to `frontend/messages/zh.json`:

```json
{
  "search": {
    "filter": {
      "openAdvancedFilter": "打开高级筛选"
    }
  }
}
```

Add to `frontend/messages/en.json`:

```json
{
  "search": {
    "filter": {
      "openAdvancedFilter": "Open advanced filters"
    }
  }
}
```

`common.close` already exists in the current locale files. Reuse it rather than adding a duplicate key.

- [ ] **Step 5: Verify**

Run:

```bash
cd frontend
npm run test:e2e -- --project=mobile-chrome frontend/e2e/ui-layout.spec.ts -g "search stacks"
npm run lint
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add "frontend/app/(public)/search/page.tsx" frontend/messages/zh.json frontend/messages/en.json
git commit -m "fix: repair search mobile drawer layout"
```

---

## Task 4: Normalize Search Filter Contract

**Files:**
- Create: `frontend/lib/search-filters.ts`
- Modify: `frontend/app/(public)/search/page.tsx`
- Modify: `frontend/components/layout/FacetedSearchSidebar.tsx`
- Modify: `frontend/e2e/search-filter-contract.spec.ts`

- [ ] **Step 1: Confirm failing contract test**

Run:

```bash
cd frontend
npm run test:e2e -- --project=chromium frontend/e2e/search-filter-contract.spec.ts
```

Expected: FAIL because selected filters do not reliably reach backend query params and keyword search currently calls `/api/v1/contents` instead of the backend full-text route `/api/v1/contents/search`.

- [ ] **Step 2: Create filter normalization helper**

Create `frontend/lib/search-filters.ts`:

```ts
export interface SearchFilterConfig {
  category?: string;
  selectedTags?: string[];
  selected_tags?: string[];
  contentTypes?: string[];
  content_types?: string[];
  timeRange?: string;
  time_range?: string;
  sort?: string;
}

export interface NormalizedSearchFilterConfig {
  category?: string;
  selectedTags: string[];
  contentTypes: string[];
  timeRange?: string;
  sort?: string;
}

function compactList(value: string[] | undefined): string[] {
  return Array.isArray(value) ? value.map((item) => item.trim()).filter(Boolean) : [];
}

export function normalizeSearchFilters(config: SearchFilterConfig): NormalizedSearchFilterConfig {
  return {
    category: config.category?.trim() || undefined,
    selectedTags: compactList(config.selectedTags ?? config.selected_tags),
    contentTypes: compactList(config.contentTypes ?? config.content_types),
    timeRange: config.timeRange ?? config.time_range ?? undefined,
    sort: config.sort?.trim() || undefined,
  };
}

export function buildContentSearchParams(query: string, config: SearchFilterConfig): URLSearchParams {
  const normalized = normalizeSearchFilters(config);
  const params = new URLSearchParams();
  params.set("q", query);
  if (normalized.category) params.set("category", normalized.category);
  if (normalized.selectedTags.length > 0) params.set("tags", normalized.selectedTags.join(","));
  if (normalized.contentTypes.length > 0) params.set("content_type", normalized.contentTypes.join(","));
  if (normalized.timeRange) params.set("time_range", normalized.timeRange);
  if (normalized.sort) params.set("sort", normalized.sort);
  return params;
}

export function buildContentSearchPath(query: string, config: SearchFilterConfig): string {
  const params = buildContentSearchParams(query, config);
  return `/api/v1/contents/search?${params.toString()}`;
}
```

- [ ] **Step 3: Use helper in search page**

Modify `frontend/app/(public)/search/page.tsx`:

```tsx
import {
  buildContentSearchPath,
  normalizeSearchFilters,
  type SearchFilterConfig,
} from "@/lib/search-filters";
import { normalizeContentList } from "@/lib/content";
```

Replace local `FilterConfig` with the imported `SearchFilterConfig`.

In `doSearch`:

```tsx
const path = buildContentSearchPath(q, filter);
const data = await api.get<{ items?: unknown[]; contents?: unknown[]; total?: number }>(path);
setResults(normalizeContentList(data.items ?? data.contents ?? []));
```

Remove the old request to `/api/v1/contents?${params.toString()}` from `doSearch`. The backend search route returns `items`; older list endpoints return `contents`, so normalize both shapes.

In `handleFilterChange`:

```tsx
function handleFilterChange(config: SearchFilterConfig) {
  setFilterConfig(config);
  if (query) void doSearch(query, config);
}
```

For active filter chips:

```tsx
const normalizedFilters = normalizeSearchFilters(filterConfig);
```

Use `normalizedFilters.selectedTags`, `normalizedFilters.contentTypes`, and `normalizedFilters.timeRange` when rendering chips.

- [ ] **Step 4: Emit camelCase from sidebar while keeping saved search compatibility**

Modify `frontend/components/layout/FacetedSearchSidebar.tsx`:

```tsx
import type { SearchFilterConfig } from "@/lib/search-filters";

export interface FilterConfig extends SearchFilterConfig {}
```

Change `buildConfig`:

```tsx
const buildConfig = useCallback((): FilterConfig => ({
  category: selectedCategory || undefined,
  selectedTags: selectedTags.length > 0 ? [...selectedTags] : undefined,
  contentTypes: contentTypes.length > 0 ? [...contentTypes] : undefined,
  timeRange: timeRange || undefined,
  sort: sort || undefined,
}), [selectedCategory, selectedTags, contentTypes, timeRange, sort]);
```

Keep `handleApplySavedSearch` accepting old snake_case:

```tsx
const normalized = normalizeSearchFilters(search.config);
if (normalized.category) setSelectedCategory(normalized.category);
setSelectedTags(normalized.selectedTags);
setContentTypes(normalized.contentTypes);
setTimeRange(normalized.timeRange ?? "");
setSort(normalized.sort ?? "");
```

- [ ] **Step 5: Verify**

Run:

```bash
cd frontend
npm run test:e2e -- --project=chromium frontend/e2e/search-filter-contract.spec.ts
npm run lint
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/lib/search-filters.ts "frontend/app/(public)/search/page.tsx" frontend/components/layout/FacetedSearchSidebar.tsx frontend/e2e/search-filter-contract.spec.ts
git commit -m "fix: normalize search filter contract"
```

---

## Task 5: Introduce Canonical Form Controls

**Files:**
- Create: `frontend/components/ui/field.tsx`
- Modify: `frontend/components/ui/textarea.tsx`
- Create: `frontend/components/ui/select.tsx`
- Create: `frontend/components/ui/checkbox.tsx`
- Create: `frontend/components/ui/switch.tsx`
- Create: `frontend/components/ui/empty-state.tsx`

- [ ] **Step 1: Create field helpers**

Create `frontend/components/ui/field.tsx`:

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";

function Field({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("flex flex-col gap-1.5", className)} {...props} />;
}

function FieldLabel({ className, ...props }: React.ComponentProps<typeof Label>) {
  return <Label className={cn("text-sm font-medium", className)} {...props} />;
}

function FieldHint({ className, ...props }: React.ComponentProps<"p">) {
  return <p className={cn("text-xs text-muted-foreground", className)} {...props} />;
}

function FieldError({ className, ...props }: React.ComponentProps<"p">) {
  return <p className={cn("text-xs text-destructive", className)} role="alert" {...props} />;
}

export { Field, FieldLabel, FieldHint, FieldError };
```

- [ ] **Step 2: Align textarea**

Modify `frontend/components/ui/textarea.tsx` to match `Input` state rules:

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "min-h-20 w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-sm outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:bg-input/30 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
```

- [ ] **Step 3: Add styled native select**

Create `frontend/components/ui/select.tsx`:

```tsx
import * as React from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

function Select({ className, children, ...props }: React.ComponentProps<"select">) {
  return (
    <span className="relative block w-full">
      <select
        data-slot="select"
        className={cn(
          "h-8 w-full appearance-none rounded-lg border border-input bg-background px-2.5 py-1 pr-8 text-sm outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:bg-input/30",
          className,
        )}
        {...props}
      >
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
    </span>
  );
}

export { Select };
```

- [ ] **Step 4: Add checkbox**

Create `frontend/components/ui/checkbox.tsx`:

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

type CheckboxProps = Omit<React.ComponentProps<"input">, "type">;

function Checkbox({ className, ...props }: CheckboxProps) {
  return (
    <input
      type="checkbox"
      data-slot="checkbox"
      className={cn(
        "h-4 w-4 rounded border border-input accent-primary outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}

export { Checkbox };
```

- [ ] **Step 5: Add switch**

Create `frontend/components/ui/switch.tsx`:

```tsx
import * as React from "react";
import { cn } from "@/lib/utils";

interface SwitchProps extends Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "onChange"> {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}

function Switch({ checked, onCheckedChange, className, disabled, ...props }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      data-slot="switch"
      className={cn(
        "relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border border-transparent outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
        checked ? "bg-primary" : "bg-muted-foreground/25",
        className,
      )}
      onClick={() => onCheckedChange(!checked)}
      {...props}
    >
      <span
        className={cn(
          "inline-block h-4 w-4 rounded-full bg-white transition-transform",
          checked ? "translate-x-6" : "translate-x-1",
        )}
      />
    </button>
  );
}

export { Switch };
```

- [ ] **Step 6: Add empty state**

Create `frontend/components/ui/empty-state.tsx`:

```tsx
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
  icon?: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn("rounded-md border border-border bg-card p-8 text-center", className)}>
      {Icon && <Icon className="mx-auto h-8 w-8 text-muted-foreground" />}
      <p className="mt-3 text-sm font-medium text-foreground">{title}</p>
      {description && <p className="mt-1 text-xs text-muted-foreground">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export { EmptyState };
```

- [ ] **Step 7: Verify typecheck**

Run:

```bash
cd frontend
npm run lint
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add frontend/components/ui/field.tsx frontend/components/ui/textarea.tsx frontend/components/ui/select.tsx frontend/components/ui/checkbox.tsx frontend/components/ui/switch.tsx frontend/components/ui/empty-state.tsx
git commit -m "feat: add canonical UI form primitives"
```

---

## Task 6: Repair Auth Forms, Password Toggles, Captcha Feedback, And Safe Errors

**Files:**
- Create: `frontend/lib/user-facing-error.ts`
- Modify: `frontend/components/verification/CaptchaWidget.tsx`
- Modify: `frontend/app/(public)/login/page.tsx`
- Modify: `frontend/app/(public)/register/page.tsx`
- Modify: `frontend/app/(public)/forgot-password/page.tsx`
- Modify: `frontend/app/(public)/reset-password/page.tsx`
- Modify: `frontend/app/(public)/verify-email/pending/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Create user-facing error mapper**

Create `frontend/lib/user-facing-error.ts`:

```ts
import { ApiRequestError } from "@/lib/api";

export function getUserFacingErrorKey(error: unknown, fallbackKey = "common.operationFailed"): string {
  if (!(error instanceof ApiRequestError)) return fallbackKey;

  switch (error.code) {
    case "INVALID_CREDENTIALS":
      return "auth.errorInvalidCredentials";
    case "USER_BANNED":
      return "auth.errorBanned";
    case "USER_EXISTS":
      return "auth.errorEmailTaken";
    case "USERNAME_TAKEN":
      return "auth.errorUsernameTaken";
    case "TERMS_VERSION_MISMATCH":
      return "auth.errorTermsVersionMismatch";
    case "PRIVACY_VERSION_MISMATCH":
      return "auth.errorPrivacyVersionMismatch";
    case "TOKEN_EXPIRED":
    case "UNAUTHORIZED":
      return "auth.errorSessionExpired";
    case "RATE_LIMITED":
      return "common.rateLimited";
    default:
      return fallbackKey;
  }
}
```

- [ ] **Step 2: Improve captcha component state**

Modify `frontend/components/verification/CaptchaWidget.tsx`:

```tsx
import { useEffect, useRef, useCallback, useState } from "react";

// inside component
const [status, setStatus] = useState<"loading" | "ready" | "bypass" | "error">("loading");

// when bypass:
setStatus("bypass");
onToken("bypass");
return;

// when SDK is ready:
setStatus("ready");

// on SDK missing/fail:
setStatus("error");
onError?.(t("auth.captchaFailed"));
```

Render:

```tsx
return (
  <div className="space-y-1">
    <div ref={containerRef} className="captcha-widget" />
    {status === "loading" && <p className="text-xs text-muted-foreground">{t("auth.captchaLoading")}</p>}
    {status === "error" && <p className="text-xs text-destructive" role="alert">{t("auth.captchaFailed")}</p>}
  </div>
);
```

Do not show text for `bypass`; local/dev bypass should stay quiet.

- [ ] **Step 3: Make password toggles keyboard-accessible**

In login/register pages, remove `tabIndex={-1}` from password reveal buttons.

Add labels:

```tsx
<button
  type="button"
  aria-label={showPassword ? t("auth.hidePassword") : t("auth.showPassword")}
  aria-pressed={showPassword}
  className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
  onClick={() => setShowPassword(!showPassword)}
>
  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
</button>
```

- [ ] **Step 4: Connect field errors to inputs**

Use `Field`, `FieldLabel`, `FieldError` in auth forms.

Example for register email:

```tsx
<Field>
  <FieldLabel htmlFor="email">{t("auth.email")}</FieldLabel>
  <Input
    id="email"
    type="email"
    placeholder="you@example.com"
    autoComplete="email"
    value={email}
    onChange={(e) => setEmail(e.target.value)}
    required
    disabled={isLoading}
    aria-invalid={Boolean(errors.email)}
    aria-describedby={errors.email ? "email-error" : undefined}
  />
  {errors.email && <FieldError id="email-error">{errors.email}</FieldError>}
</Field>
```

- [ ] **Step 5: Stop rendering raw backend messages in auth forms**

Replace `err.message || ...` fallback with mapped i18n keys:

```tsx
const key = getUserFacingErrorKey(err, "auth.errorLoginFailed");
setError(t(key));
```

For field-specific codes, continue assigning field errors but use mapped translations only.

- [ ] **Step 6: Add translations**

Add missing keys to both locale files:

```json
{
  "auth": {
    "showPassword": "显示密码",
    "hidePassword": "隐藏密码",
    "captchaLoading": "验证码加载中...",
    "errorSessionExpired": "登录状态已过期，请重新登录"
  },
  "common": {
    "rateLimited": "操作过于频繁，请稍后重试"
  }
}
```

Use English equivalents in `en.json`.

`auth.captchaFailed` already exists in both current locale files. Reuse the existing key and update its wording only if necessary. Do not add duplicate keys to the same object.

- [ ] **Step 7: Verify**

Run:

```bash
cd frontend
npm run lint
npm run test:e2e -- --project=chromium frontend/e2e/ui-layout.spec.ts
```

Manual MCP Playwright checks:

- `/login`: Tab reaches email, password, password reveal, checkbox, submit, links.
- `/register`: Tab reaches both password reveal control and terms/privacy checkboxes.
- Backend unavailable: user sees generic network/operation failure, not raw backend internals.

- [ ] **Step 8: Commit**

```bash
git add frontend/lib/user-facing-error.ts frontend/components/verification/CaptchaWidget.tsx "frontend/app/(public)/login/page.tsx" "frontend/app/(public)/register/page.tsx" "frontend/app/(public)/forgot-password/page.tsx" "frontend/app/(public)/reset-password/page.tsx" "frontend/app/(public)/verify-email/pending/page.tsx" frontend/messages/zh.json frontend/messages/en.json
git commit -m "fix: repair auth form feedback and accessibility"
```

---

## Task 7: Replace Misleading Native Controls In Search And Admin Screens

**Files:**
- Modify: `frontend/components/layout/FacetedSearchSidebar.tsx`
- Modify: `frontend/components/admin/AdminFilterBar.tsx`
- Modify: `frontend/app/(protected)/admin/audit-logs/page.tsx`
- Modify: `frontend/app/(protected)/admin/feedback/page.tsx`
- Modify: `frontend/app/(protected)/admin/categories/page.tsx`

- [ ] **Step 1: Replace simple selects with canonical Select**

In each file, import:

```tsx
import { Select } from "@/components/ui/select";
```

Replace native:

```tsx
<select className="...">
```

with:

```tsx
<Select aria-label={f.allLabel} value={f.value} onChange={(e) => f.onChange(e.target.value)}>
```

In `AdminFilterBar`, use each filter's existing `allLabel` as the accessible label unless the caller provides a more specific label. In page-level admin filters, use the existing page translation key already rendered next to the control. Preserve all existing options and values.

- [ ] **Step 2: Remove number steppers from ID-like fields**

In `frontend/app/(protected)/admin/categories/page.tsx`, change `parent_id` from:

```tsx
<input type="number" ... />
```

to a real select:

```tsx
<Select
  value={createValues.parent_id}
  onChange={(e) => setCreateValues((v) => ({ ...v, parent_id: e.target.value }))}
  aria-label={t("admin.categories.parentId")}
>
  <option value="">{t("admin.categories.parentHint")}</option>
  {categories
    .filter((cat) => cat.level === "category")
    .map((cat) => (
      <option key={cat.id} value={cat.id}>
        {(cat.name_i18n as Record<string, string>)?.zh || cat.slug}
      </option>
    ))}
</Select>
```

This plan chooses the select. Do not leave this as an implementation-time decision.

- [ ] **Step 3: Keep sort as a deliberate numeric control**

Keep direct sort editing, but remove browser steppers. Replace `sort_order` inputs with:

```tsx
<Input
  type="text"
  inputMode="numeric"
  pattern="[0-9]*"
  value={createValues.sort_order}
  onChange={(e) => setCreateValues((v) => ({ ...v, sort_order: e.target.value.replace(/\D/g, "") }))}
/>
```

Use the same `type="text" inputMode="numeric"` pattern for inline edit `sort_order`. Convert with `Number(...)` only at submit time.

Do not leave unstyled `type="number"` on admin forms.

- [ ] **Step 4: Verify**

Run:

```bash
cd frontend
npm run lint
```

Manual MCP Playwright checks:

- Admin category parent field no longer shows browser increment/decrement controls.
- Admin selects have consistent height, focus ring, and dark mode colors.
- Search sidebar time/sort selects match the design system.

- [ ] **Step 5: Commit**

```bash
git add frontend/components/layout/FacetedSearchSidebar.tsx frontend/components/admin/AdminFilterBar.tsx "frontend/app/(protected)/admin/audit-logs/page.tsx" "frontend/app/(protected)/admin/feedback/page.tsx" "frontend/app/(protected)/admin/categories/page.tsx"
git commit -m "fix: standardize select and numeric-like controls"
```

---

## Task 8: Repair Publish Form Feedback And Fanwork Source Selection

**Files:**
- Create: `frontend/components/studio/IPPicker.tsx`
- Create: `frontend/components/studio/SourceOriginalPicker.tsx`
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Add IP picker component**

Create `frontend/components/studio/IPPicker.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

interface IPOption {
  id: number;
  name: string;
}

interface IPPickerProps {
  value: IPOption | null;
  onChange: (value: IPOption | null) => void;
  placeholder: string;
  searchLabel: string;
  loadingLabel: string;
}

export function IPPicker({ value, onChange, placeholder, searchLabel, loadingLabel }: IPPickerProps) {
  const [query, setQuery] = useState(value?.name ?? "");
  const [options, setOptions] = useState<IPOption[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!query.trim()) {
      setOptions([]);
      return;
    }

    let cancelled = false;
    setLoading(true);
    api.get<{ ips?: IPOption[] }>(`/api/v1/ips?q=${encodeURIComponent(query.trim())}`)
      .then((data) => {
        if (!cancelled) setOptions(data.ips ?? []);
      })
      .catch(() => {
        if (!cancelled) setOptions([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [query]);

  return (
    <div className="space-y-2">
      <Input
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          onChange(null);
        }}
        placeholder={placeholder}
        aria-label={searchLabel}
      />
      {options.length > 0 && (
        <div className="rounded-md border border-border bg-card p-1">
          {options.map((option) => (
            <Button
              key={option.id}
              type="button"
              variant="ghost"
              size="sm"
              className="w-full justify-start"
              onClick={() => {
                onChange(option);
                setQuery(option.name);
                setOptions([]);
              }}
            >
              {option.name}
            </Button>
          ))}
        </div>
      )}
      {loading && <p className="text-xs text-muted-foreground">{loadingLabel}</p>}
    </div>
  );
}
```

Pass `loadingLabel={t("studio.publish.ipSearching")}` from `PublishForm.tsx`; add this key to both locale files in this task.

- [ ] **Step 2: Add source original picker**

Create `frontend/components/studio/SourceOriginalPicker.tsx`. Use the existing backend full-text search route `/api/v1/contents/search`, not `/api/v1/contents`, because `ListContents` does not consume a `q` query parameter.

```tsx
"use client";

import { useEffect, useState } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

interface SourceOriginalOption {
  id: number;
  title: string;
}

interface SourceOriginalPickerProps {
  value: SourceOriginalOption | null;
  onChange: (value: SourceOriginalOption | null) => void;
  placeholder: string;
  searchLabel: string;
}

export function SourceOriginalPicker({ value, onChange, placeholder, searchLabel }: SourceOriginalPickerProps) {
  const [query, setQuery] = useState(value?.title ?? "");
  const [options, setOptions] = useState<SourceOriginalOption[]>([]);

  useEffect(() => {
    if (!query.trim()) {
      setOptions([]);
      return;
    }

    let cancelled = false;
    api.get<{ items?: SourceOriginalOption[]; contents?: SourceOriginalOption[] }>(
      `/api/v1/contents/search?zone=original&q=${encodeURIComponent(query.trim())}&limit=8`,
    )
      .then((data) => {
        if (!cancelled) setOptions(data.items ?? data.contents ?? []);
      })
      .catch(() => {
        if (!cancelled) setOptions([]);
      });

    return () => {
      cancelled = true;
    };
  }, [query]);

  return (
    <div className="space-y-2">
      <Input
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          onChange(null);
        }}
        placeholder={placeholder}
        aria-label={searchLabel}
      />
      {options.length > 0 && (
        <div className="rounded-md border border-border bg-card p-1">
          {options.map((option) => (
            <Button
              key={option.id}
              type="button"
              variant="ghost"
              size="sm"
              className="w-full justify-start"
              onClick={() => {
                onChange(option);
                setQuery(option.title);
                setOptions([]);
              }}
            >
              {option.title}
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}
```

The backend search response shape is `items`, while some older endpoints return `contents`; keep the fallback to both shapes.

- [ ] **Step 3: Wire selected values into publish payload**

Modify `frontend/components/studio/PublishForm.tsx`:

```tsx
const [selectedIP, setSelectedIP] = useState<{ id: number; name: string } | null>(null);
const [sourceOriginal, setSourceOriginal] = useState<{ id: number; title: string } | null>(null);
```

Replace plain fanwork inputs with `IPPicker` and `SourceOriginalPicker`.

Validation:

```tsx
if (zone === "fanwork" && !selectedIP) {
  toast("error", t("studio.publish.ipRequired"));
  return;
}
```

Payload:

```tsx
if (zone === "fanwork" && selectedIP) {
  payload.ip_id = selectedIP.id;
}
if (zone === "fanwork" && sourceOriginal) {
  payload.source_original_id = sourceOriginal.id;
}
```

Do not keep the old `payload.ip_name = ipSearch.trim()` behavior. The current backend `PublishContentInput` accepts `ip_id`, and `ip_name` is ignored by the publish handler. Do not keep a misleading "search" field that never selects an entity.

- [ ] **Step 4: Add picker translations**

Add the missing picker feedback keys to both locale files. Merge into the existing `studio.publish` objects:

```json
{
  "ipSearching": "正在搜索 IP..."
}
```

Use `Searching IP...` in `frontend/messages/en.json`. Reuse existing placeholder and required-error keys such as `studio.publish.ipSearchPlaceholder`, `studio.publish.searchOriginalPlaceholder`, and `studio.publish.ipRequired` if they already exist.

- [ ] **Step 5: Convert switches to canonical Switch**

Replace every `role="switch"` hand-rolled button in `PublishForm.tsx` with:

```tsx
<Switch checked={allowComments} onCheckedChange={setAllowComments} aria-label={t("studio.publish.allowComment")} />
```

Use `FieldError` for submission blockers like compliance violation.

- [ ] **Step 6: Verify**

Run:

```bash
cd frontend
npm run lint
```

Manual MCP Playwright checks:

- `/studio/publish/fanwork`: IP/source fields visibly behave as searchable selectors or clearly as text fields, not fake selectors.
- Selected IP becomes `ip_id` in the request payload.
- Selected source original becomes `source_original_id` in the request payload.
- Switches are keyboard focusable and Space/Enter toggles them.
- Compliance violation blocks submit with an inline field-level reason.

- [ ] **Step 7: Commit**

```bash
git add frontend/components/studio/IPPicker.tsx frontend/components/studio/SourceOriginalPicker.tsx frontend/components/studio/PublishForm.tsx frontend/messages/zh.json frontend/messages/en.json
git commit -m "fix: repair publish form selection and switches"
```

---

## Task 9: Standardize Loading, Empty, And Error States On Core Pages

**Files:**
- Modify: `frontend/components/content/MasonryGrid.tsx`
- Modify: `frontend/components/content/FileUploader.tsx`
- Modify: `frontend/app/(public)/search/page.tsx`
- Modify: `frontend/components/social/CommentSection.tsx`
- Modify: `frontend/components/social/NotificationList.tsx`
- Modify: `frontend/components/social/DiscussionBoard.tsx`
- Modify: `frontend/app/(protected)/dashboard/contents/page.tsx`
- Modify: `frontend/app/(protected)/admin/contents/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Replace bare empty text with EmptyState**

In `MasonryGrid.tsx`:

```tsx
import { FileText } from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";

if (items.length === 0) {
  return (
    <EmptyState
      icon={FileText}
      title={emptyText || t("content.emptyContentMsg")}
      className="p-8"
    />
  );
}
```

- [ ] **Step 2: Replace page-level loading text with skeletons or compact loading rows**

For dashboard/admin list pages, avoid:

```tsx
return <div>加载中</div>;
```

Use existing `Skeleton`:

```tsx
<div className="space-y-3 rounded-md border border-border bg-card p-4">
  {Array.from({ length: 5 }).map((_, i) => (
    <Skeleton key={i} className="h-8 w-full" />
  ))}
</div>
```

- [ ] **Step 3: Use user-facing error mapper**

On core pages, replace direct `e.message` display:

```tsx
setError(e instanceof ApiRequestError ? e.message : t("common.loadFailed"));
```

with:

```tsx
setError(t(getUserFacingErrorKey(e, "common.loadFailed")));
```

Do this in the files listed for this task only. The remaining project-wide raw error cleanup can be a follow-up security task.

- [ ] **Step 4: Verify**

Run:

```bash
cd frontend
npm run lint
npm run test:e2e -- --project=chromium frontend/e2e/ui-layout.spec.ts
```

Manual MCP Playwright checks:

- Search empty state has title + hint + retry where applicable.
- Content grids no longer collapse into unstyled text.
- API failures are visible on page and not only in console.

- [ ] **Step 5: Commit**

```bash
git add frontend/components/content/MasonryGrid.tsx frontend/components/content/FileUploader.tsx "frontend/app/(public)/search/page.tsx" frontend/components/social/CommentSection.tsx frontend/components/social/NotificationList.tsx frontend/components/social/DiscussionBoard.tsx "frontend/app/(protected)/dashboard/contents/page.tsx" "frontend/app/(protected)/admin/contents/page.tsx" frontend/messages/zh.json frontend/messages/en.json
git commit -m "fix: standardize core loading and empty states"
```

---

## Task 10: Clean Visual Drift Against UI Spec

**Files:**
- Modify: `frontend/components/home/prototype-landing.tsx`
- Modify: `frontend/components/content/ContentCard.tsx`
- Modify: `frontend/components/content/FileUploader.tsx`
- Modify: `frontend/components/social/NotificationDropdown.tsx`
- Modify: `frontend/components/content/VersionHistory.tsx`
- Modify: `frontend/app/(protected)/history/page.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Modify: `frontend/components/ui/badge.tsx`

- [ ] **Step 1: Audit disallowed visual tokens**

Run:

```bash
cd frontend
rg -n "shadow-|rounded-2xl|rounded-3xl|rounded-4xl|bg-gradient|hover:-translate-y|backdrop-blur" app components -g "*.tsx"
```

Expected before changes: several matches.

- [ ] **Step 2: Preserve allowed shadows only**

Allowed:

- Modal
- Popover
- Dropdown

For normal cards/pages, remove `shadow-sm`, `shadow-lg`, `shadow-xl`, and hover shadow changes.

Example:

```tsx
<div className="rounded-lg border border-border bg-card p-6 shadow-sm">
```

becomes:

```tsx
<div className="rounded-md border border-border bg-card p-6">
```

- [ ] **Step 3: Normalize large radii**

Replace `rounded-2xl`, `rounded-3xl`, and `rounded-4xl` in non-pill UI with `rounded-md` or `rounded-lg`.

Allowed:

- `rounded-full` for avatars, pills, switches.
- `rounded-lg` for card-like repeated items when already established.

- [ ] **Step 4: Remove hover lift where it makes dense UI unstable**

Replace:

```tsx
hover:-translate-y-0.5 active:scale-[0.98]
```

with:

```tsx
hover:bg-muted/50 active:bg-muted
```

Keep image zoom inside content covers if it does not shift layout.

- [ ] **Step 5: Verify**

Run:

```bash
cd frontend
rg -n "shadow-|rounded-2xl|rounded-3xl|rounded-4xl|bg-gradient|hover:-translate-y" app components -g "*.tsx"
npm run lint
```

Expected: remaining `shadow-*` only in modal/popover/dropdown files, and remaining large radii only where intentionally pill-like.

Manual MCP Playwright checks:

- Home, search, login, register, content detail, and studio overview still feel consistent.
- No text overflow after radius/spacing changes.

- [ ] **Step 6: Commit**

```bash
git add frontend/components/home/prototype-landing.tsx frontend/components/content/ContentCard.tsx frontend/components/content/FileUploader.tsx frontend/components/social/NotificationDropdown.tsx frontend/components/content/VersionHistory.tsx "frontend/app/(protected)/history/page.tsx" "frontend/app/(protected)/settings/page.tsx" frontend/components/ui/badge.tsx
git commit -m "style: align UI details with flat design spec"
```

---

## Task 11: Final Cross-Viewport Verification

**Files:**
- Modify: `progress.txt`
- Create screenshots in: `frontend/screenshots/ui-detail-repair/`

- [ ] **Step 1: Run static verification**

Run:

```bash
cd frontend
npm run lint
npm run build
npm run test:e2e
```

Expected: all pass.

- [ ] **Step 2: Start full local stack or document blocked protected-route checks**

For protected route screenshots, use a real local backend session:

```bash
docker compose up -d postgres redis
cd backend
go run cmd/server/main.go
```

In another terminal:

```bash
cd frontend
npm run dev
```

Log in through the UI with a local test account that has creator access. For `/admin/categories`, log in as an admin user. If no local admin/test user exists, do not fake completion: record the protected route verification as blocked in `progress.txt` with the missing account/setup details.

- [ ] **Step 3: Run browser verification with MCP Playwright**

Check these routes at desktop `1280x800` and mobile `390x844`:

- `/`
- `/search`
- `/login`
- `/register`
- `/forgot-password`
- `/reset-password?token=test`
- `/studio/publish/original`
- `/studio/publish/fanwork`
- `/admin/categories`

Save screenshots:

```text
frontend/screenshots/ui-detail-repair/home-mobile.png
frontend/screenshots/ui-detail-repair/search-mobile.png
frontend/screenshots/ui-detail-repair/login-desktop.png
frontend/screenshots/ui-detail-repair/register-mobile.png
frontend/screenshots/ui-detail-repair/publish-fanwork-desktop.png
frontend/screenshots/ui-detail-repair/admin-categories-desktop.png
```

- [ ] **Step 4: Verify interaction checklist**

For each route above:

- No horizontal overflow on mobile.
- No controls overlap or squeeze below usable width.
- Inputs have visible focus.
- Password reveal buttons are keyboard reachable.
- Selects match the UI system.
- Numeric-looking ID fields do not show browser steppers.
- Loading is visible and non-blocking.
- Empty state is explicit.
- Error state is user-facing and not raw backend text.
- Dialog/drawer has accessible name and closes via Escape.
- Console has no app errors under mocked API responses.

- [ ] **Step 5: Update progress log**

Append to `progress.txt`:

```markdown
## [YYYY-MM-DD] - UI Detail Repair Plan: Verification

### What was done:
- Repaired mobile home/search layouts.
- Normalized search filter contract.
- Added canonical form controls and consistent empty/error states.
- Repaired auth form accessibility and publish form selector behavior.
- Removed visual drift from the flat 1px border design language.

### Testing:
- `npm run lint` passed.
- `npm run build` passed.
- `npm run test:e2e` passed.
- MCP Playwright screenshots saved under `frontend/screenshots/ui-detail-repair/`.

### Notes:
- Remaining raw `ApiRequestError.message` usages outside the repaired core paths should be handled by a separate security/error-copy cleanup task.
```

- [ ] **Step 6: Commit**

```bash
git add progress.txt frontend/screenshots/ui-detail-repair
git commit -m "test: verify UI detail repair"
```

---

## Recommended Execution Order

1. Task 1: Add regression test harness.
2. Task 2: Fix home mobile layout.
3. Task 3: Fix search mobile layout and drawer semantics.
4. Task 4: Normalize search filter contract.
5. Task 5: Add canonical UI primitives.
6. Task 6: Repair auth forms and captcha feedback.
7. Task 7: Replace misleading native controls in search/admin screens.
8. Task 8: Repair publish form selector behavior.
9. Task 9: Standardize loading/empty/error states.
10. Task 10: Clean visual drift.
11. Task 11: Final verification.

## Success Criteria

- Mobile home no longer shows the desktop sidebar or squeezed main content.
- Mobile search result panel is full-width and the filter drawer behaves like an accessible dialog.
- Search selected filters are included in backend request params.
- Password reveal controls are keyboard reachable.
- ID-like fields do not expose number steppers.
- Fanwork IP selection is submitted as `ip_id`.
- Fanwork source original selection is submitted as `source_original_id`.
- API failures produce visible user-facing states instead of silent blank areas.
- Core forms and selects share consistent size, focus, disabled, error, and dark-mode styles.
- Visual drift from `design/ui-spec.md` is reduced: shadows limited to allowed overlays, excessive radii removed, hover lift removed from dense tool surfaces.
- `npm run lint`, `npm run build`, and `npm run test:e2e` pass.

## Reviewer Note

The `writing-plans` skill recommends dispatching a plan-document-reviewer subagent after saving this plan. This session did not spawn a reviewer because the available multi-agent tool is restricted to cases where the user explicitly asks for subagents or delegation. If the user approves subagent review, dispatch a single reviewer with this plan path and `design/ui-spec.md` as the review context.
