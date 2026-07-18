# OmniCraft Community Content Series Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加公开内容系列能力，让创作者把原创或二创内容组织成有序系列，并在内容详情页展示上一章/下一章导航。

**Architecture:** 新增 `content_series` 与 `content_series_items` 多对多模型；后端只允许系列 owner 管理，添加内容时允许 owner 自己发布的内容或 owner 已确认贡献的内容；前端新增公开 `/series/[id]` 页面、Studio 系列管理页和 `SeriesNav`。首版只做公开系列，不引入私有系列权限分支。

**Tech Stack:** Go/Gin/GORM/PostgreSQL, Next.js App Router, next-intl, React Testing Library, Playwright.

---

## Cross-Plan Coordination

- Execution source: this is part of the 2026-06-30 community feature plan family, derived from `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`. It is not a historical `task.json` task and not a 2026-05-30 Beta roadmap checkbox; executing it requires an explicit user request naming this plan or the community feature family.
- Shared-file integration and migration order for the six community plans is: messages-notifications (`057`) -> browse-history (no migration) -> collections (`058`) -> content-series (`059`) -> source-linkage (`060`) -> collaboration-invites (`061`).
- `frontend/app/(protected)/messages/page.tsx`, `frontend/components/social/ChatWindow.tsx`, and `frontend/components/social/ConversationList.tsx` must land in messages-notifications before collaboration-invites extends typed invite cards.
- `frontend/components/content/ContentDetail.tsx` changes from collections must already be present; this plan then adds `SeriesNav`; source-linkage adds attribution/related rows after this plan.
- `frontend/components/studio/PublishForm.tsx` changes must land source-linkage before collaboration-invites; this plan must not edit `PublishForm.tsx`.
- `backend/config/config.go` and `backend/config.yaml` changes from browse-history and collaboration-invites must be implemented serially and rebased before verification.
- Before any UI code, grep `design/ui-spec.md` for the exact `## Page:` / `## Component:` sections named by this plan and follow those sections as the visual authority. As of 2026-06-30, `/series/[id]`, `/studio/series`, `SeriesNav`, `ContentDetail`, `StudioSidebar`, `ContentCard`, `ConfirmModal`, and `Toast` are present; do not rewrite `design/ui-spec.md` unless an implementation-time check proves a required section is absent or stale.
- Expected-result convention: any "Run and confirm red" step expects FAIL for the behavior under test; any "Verify green" / "Run ... tests" step expects PASS. If the observed result differs, stop and update the plan before proceeding.
- Frontend focused test convention: current `frontend/package.json` defines `npm run test` as a fixed suite, so focused TS/TSX tests in this plan use `node --import tsx --test <file>` directly. Do not write `npm run test -- <file>` unless the package script is changed first.
- Before implementation, run `git status --short`, reserve exact files, and stage only exact touched files. Do not use directory-level staging such as `git add backend`, `git add frontend`, `git add design`, `git add screenshots`, or `git add docs/superpowers/plans`.
- Staging note: the sample `git add` command at the end must be reduced to files actually changed in that implementation. Omit `design/ui-spec.md` when it was only read/verified; omit generated docs such as `architecture.md` unless `doc-validator` changed them during this task.

---

## File Structure

### Backend

- Create: `backend/migrations/059_create_content_series.sql` or next available migration number at implementation time.
- Create: `backend/internal/model/series.go`
- Create: `backend/internal/repository/series_repo.go`
- Create: `backend/internal/service/series_service.go`
- Create: `backend/internal/handler/series.go`
- Create: `backend/internal/model/series_migration_test.go`
- Create: `backend/internal/service/series_service_test.go`
- Create: `backend/internal/handler/series_test.go`
- Modify: the current sole route owner: `backend/internal/handler/routes.go`, or `backend/internal/router/routes.go` after hardening Task 3. Never recreate the deleted handler route owner.
- Modify: `backend/internal/handler/content.go` - 内容详情返回紧凑 `series_memberships`。
- Modify: `backend/internal/repository/content_repo.go` only if needed for content detail series lookup.

### Frontend

- Create: `frontend/app/(public)/series/[id]/page.tsx`
- Create: `frontend/app/(protected)/studio/series/page.tsx`
- Create: `frontend/components/content/SeriesNav.tsx`
- Create: `frontend/e2e/content-series.spec.ts`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/lib/content.ts`
- Modify: `frontend/components/studio/StudioSidebar.tsx`
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`
- Read before UI code: `design/ui-spec.md` sections for `/series/[id]`, `/studio/series`, `SeriesNav`, `ContentDetail`, and `StudioSidebar`.

---

## Task 1: Add Migration And Models

**Files:**
- Create: `backend/migrations/059_create_content_series.sql`
- Create: `backend/internal/model/series.go`
- Test: `backend/internal/model/series_migration_test.go`

- [x] **Step 1: Re-check migration number**

Run:

```powershell
Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 10 -ExpandProperty Name
```

If previous community migrations have not landed, choose the next available number rather than blindly using `059`.

- [x] **Step 2: Write failing migration test**

Assert:

- `content_series` exists
- `content_series_items` exists
- `content_series.zone` has `original/fanwork` check
- `content_series.owner_id` references `users(id)`
- `content_series_items.series_id` cascades on delete
- `content_series_items.content_item_id` references `content_items(id)`
- `content_series.cover_content_id` references `content_items(id)` with `ON DELETE SET NULL`
- unique `(series_id, content_item_id)` exists
- indexes exist on owner, series, and content lookup

- [x] **Step 3: Implement migration**

DDL must include:

```sql
CREATE TABLE content_series (...);
CREATE TABLE content_series_items (...);
CREATE INDEX idx_content_series_owner ON content_series(owner_id);
CREATE INDEX idx_series_items_series ON content_series_items(series_id);
CREATE INDEX idx_series_items_content ON content_series_items(content_item_id);
```

`content_series.cover_content_id` is a nullable FK to `content_items(id) ON DELETE SET NULL`. `content_series` must include `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` and `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`. `content_series_items` must include an `added_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` field. The migration file must also include a `-- ROLLBACK:` comment block for local-test rollback guidance; do not auto-drop shared data.

- [x] **Step 4: Add model structs**

Create:

```go
type ContentSeries struct {
    ID int64
    Title string
    Description string
    CoverContentID *int64
    OwnerID int64
    Zone string
}

type ContentSeriesItem struct {
    ID int64
    SeriesID int64
    ContentItemID int64
    SortOrder int
}
```

- [x] **Step 5: Verify migration**

Run:

```powershell
cd backend
go test ./internal/model -run TestContentSeriesMigration -v
```

---

## Task 2: Implement Series Repository And Service Rules

**Files:**
- Create: `backend/internal/repository/series_repo.go`
- Create: `backend/internal/service/series_service.go`
- Test: `backend/internal/service/series_service_test.go`

- [x] **Step 1: Write failing service tests**

Cover:

```go
func TestSeriesCreateAndListOwned(t *testing.T) {}
func TestSeriesAddOwnerAuthoredContent(t *testing.T) {}
func TestSeriesAddContributorContent(t *testing.T) {}
func TestSeriesRejectsUnrelatedContent(t *testing.T) {}
func TestSeriesRejectsZoneMismatch(t *testing.T) {}
func TestSeriesRejectsDeletedBannedOrAuthorDeletedContent(t *testing.T) {}
func TestSeriesRejectsDuplicateItem(t *testing.T) {}
func TestSeriesAddItemAppendsAfterMaxSortOrder(t *testing.T) {}
func TestSeriesAddItemConcurrentAppendKeepsStableUniqueSortOrder(t *testing.T) {}
func TestSeriesRejectsCoverNotInSeries(t *testing.T) {}
func TestSeriesCoverFallsBackWhenCoverContentDeleted(t *testing.T) {}
func TestSeriesReorderIsTransactional(t *testing.T) {}
func TestSeriesReorderRejectsMissingOrForeignItems(t *testing.T) {}
```

- [x] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/service -run TestSeries -v
```

- [x] **Step 3: Implement repository methods**

Required methods:

```go
CreateSeries(ctx, series)
ListSeriesByOwner(ctx, ownerID, zone)
GetSeriesDetail(ctx, seriesID)
UpdateSeries(ctx, seriesID, ownerID, patch)
DeleteSeries(ctx, seriesID, ownerID)
AddItem(ctx, seriesID, ownerID, contentID)
RemoveItem(ctx, seriesID, ownerID, itemID)
ReorderItems(ctx, seriesID, ownerID, itemIDs)
ListMembershipsForContent(ctx, contentID)
```

`AddItem` must append by setting `sort_order = COALESCE(MAX(sort_order), -1) + 1` within the target series. Do not rely on the database default `0` except for the first item. The append path must lock the parent `content_series` row in the same transaction before reading `MAX(sort_order)`, so concurrent appends to an empty or non-empty series cannot produce duplicate or unstable ordering.

- [x] **Step 4: Implement ownership and addable-content rule**

Adding content is allowed only when:

```text
content_items.author_id = series.owner_id
OR EXISTS content_contributors WHERE content_item_id = content.id AND user_id = series.owner_id
```

Do not let a contributor manage someone else's series.

The target content must not be soft-deleted and must have a series-eligible status. The current content state machine creates submissions as `pending` (there is no separate `draft`/`pending_review` status). Owner management may add `pending` or `published` content so a series can be prepared before release; `under_review`, `banned`, and `author_deleted` content are not addable. Public series detail and previous/next navigation return only `published` non-deleted content. Tests must prove a pending item can be managed in Studio without leaking through the public detail response.

When adding, calculate the append `sort_order` in the same transaction as the insert. First lock the target parent row:

```sql
SELECT id FROM content_series WHERE id = ? FOR UPDATE
```

Then compute `MAX(sort_order)` for that series and insert the new item. Tests must simulate concurrent append requests and prove the resulting `sort_order` values are unique and contiguous.

Locking the parent `content_series` row is the serialization point for appends. After that lock is acquired, reading `MAX(sort_order)` from `content_series_items` in the same transaction is sufficient for PostgreSQL consistency; do not add table-wide locks on `content_series_items`.

- [x] **Step 5: Implement reorder transaction**

Use `db.Transaction`. Lock series items for the target `series_id` before update:

```sql
SELECT id FROM content_series_items WHERE series_id = ? FOR UPDATE
```

Reject the request if `item_ids` is missing any existing item or contains an item from another series.

During reorder, concurrent `AddItem` or `RemoveItem` operations for the same series are expected to wait behind the transaction lock and then retry/observe the updated order. This is acceptable; do not return a spurious conflict solely because the lock is waiting.

- [x] **Step 6: Verify service**

Run:

```powershell
cd backend
go test ./internal/service -run TestSeries -v
```

---

## Task 3: Add Series API Routes

**Files:**
- Create: `backend/internal/handler/series.go`
- Modify: current sole route owner (`backend/internal/handler/routes.go` or `backend/internal/router/routes.go`)
- Test: `backend/internal/handler/series_test.go`

- [x] **Step 1: Add failing route tests**

Routes:

```text
POST   /api/v1/series
GET    /api/v1/series
GET    /api/v1/series/:id
PUT    /api/v1/series/:id
DELETE /api/v1/series/:id
POST   /api/v1/series/:id/items
DELETE /api/v1/series/:id/items/:itemId
PUT    /api/v1/series/:id/items/reorder
```

- [x] **Step 2: Assert response shapes**

`GET /api/v1/series/:id` returns:

```json
{
  "series": {
    "id": 1,
    "title": "...",
    "description": "...",
    "zone": "original",
    "owner": {"id": 1, "username": "..."},
    "cover": "https://...",
    "item_count": 12
  },
  "items": [
    {"id": 10, "sort_order": 0, "content": {"id": 100, "title": "..."}}
  ]
}
```

The backend resolves `cover` in this order:

1. `cover_content_id` first image when valid
2. smallest `sort_order` content cover
3. `null`

Public detail filters items through the shared content-visibility rule. `item_count` is the count of visible published items for anonymous/non-owner viewers; the authenticated owner Studio endpoint may return a separate management count. Do not leak pending titles through the public response.

- [x] **Step 3: Implement handler and error mapping**

Exact errors:

- `SERIES_NOT_FOUND` -> 404
- `NOT_SERIES_OWNER` -> 403
- `CONTENT_NOT_OWNED_OR_CONTRIBUTED` -> 400
- `ZONE_MISMATCH` -> 400
- `DUPLICATE_SERIES_ITEM` -> 409
- `COVER_NOT_IN_SERIES` -> 400

- [x] **Step 4: Register routes**

Use auth-if-present middleware for public detail:

```go
v1.GET("/series/:id", optAuth, seriesHandler.GetSeries)
```

Use auth and interaction guard for mutations.

- [x] **Step 5: Verify handler**

Run:

```powershell
cd backend
go test ./internal/handler -run TestSeries -v
```

---

## Task 4: Add Series Memberships To Content Detail

**Files:**
- Modify: `backend/internal/handler/content.go`
- Modify: `backend/internal/repository/series_repo.go` — add `ListMembershipsForContent(ctx, contentID)` here (natural home for series queries; covered by existing `series_repo` tests in Task 2). If the query must instead live in `content_repo.go`, add `backend/internal/repository/content_repo_test.go` with `TestContentRepoListSeriesMemberships`.
- Modify: `frontend/lib/content.ts`
- Test: `backend/internal/handler/series_test.go`

- [x] **Step 1: Add failing content-detail test**

Create a content item that belongs to two series. Assert `GET /api/v1/contents/:id` returns:

```json
"series_memberships": [
  {
    "series_id": 1,
    "series_title": "...",
    "current_index": 3,
    "total": 12,
    "previous": {"id": 101, "title": "..."},
    "next": {"id": 103, "title": "..."}
  }
]
```

Return all compact memberships for the current content, ordered by series update time then series ID. The UI renders the first three as tabs and places remaining memberships in an overflow menu; the backend must not truncate to three. Because the full list is returned, `memberships.length` is the single total and no redundant `series_memberships_total` field is added.
`current_index` is 1-based for display (`第 3 / 共 12`), while `content_series_items.sort_order` remains zero-based and backend-owned. Do not make the frontend add 1, or the first/last disabled logic will drift.

- [x] **Step 2: Implement backend membership lookup**

For each series containing the content, compute previous/next by `sort_order`. Return only published, non-deleted content entries for public navigation. If the current content is not publicly visible, do not expose memberships through the public content-detail path.

- [x] **Step 3: Extend frontend normalizer**

In `frontend/lib/content.ts`, add `series_memberships` with snake_case/PascalCase tolerance. Invalid previous/next objects missing `id` or `title` must become `undefined`.

- [x] **Step 4: Verify backend**

Run:

```powershell
cd backend
go test ./internal/handler -run TestContentDetailSeriesMemberships -v
```

---

## Task 5: Add UI Spec And SeriesNav Component

**Files:**
- Read: `design/ui-spec.md`
- Create: `frontend/components/content/SeriesNav.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: add to `frontend/tests/series-nav.test.tsx`

- [x] **Step 1: Confirm UI spec before component code**

Run:

```powershell
rg -n "## Component: SeriesNav|## Page: /series/\\[id\\]|## Page: /studio/series" design/ui-spec.md
```

Expected: all three sections exist. If a future branch lacks one, stop and repair UI spec in an explicitly scoped docs/design step before coding. `SeriesNav` must be visually restrained and positioned below body content and above comments.

- [x] **Step 2: Write failing component tests**

Cover:

- single series renders title and chapter position
- first item disables previous with "已是第一章"
- last item disables next with "已是最后一章"
- multiple series renders up to three tabs
- when membership count is greater than three, render the first three tabs plus a compact `更多(N)` menu listing every remaining series
- catalog link points to `/series/:id`

- [x] **Step 3: Implement `SeriesNav`**

Props:

```ts
interface SeriesNavProps {
  memberships: SeriesMembership[];
}
```

Use `Link` for valid previous/next/catalog targets. Use icon buttons or compact text links, not oversized cards. If there are more than three memberships, keep layout stable by showing the first three tabs and a keyboard-accessible `更多(N)` menu; each overflow item links to its own `/series/:id`. `N` equals `memberships.length - 3`.

- [x] **Step 4: Insert in `ContentDetail.tsx`**

Place `SeriesNav` after main body/attachments and before comments. If `ReactionBar` placement conflicts with UI spec, follow `design/ui-spec.md`.

- [x] **Step 5: Run tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/series-nav.test.tsx
```

---

## Task 6: Confirm Public Series UI Spec And API Helper

**Files:**
- Read: `design/ui-spec.md`
- Modify: `frontend/lib/content.ts`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: add to `frontend/tests/series-nav.test.tsx` or create `frontend/tests/series-api.test.tsx`

- [x] **Step 1: Confirm UI spec before page code**

Run:

```powershell
rg -n "## Page: /series/\\[id\\]|## Component: SeriesNav" design/ui-spec.md
```

Both sections must exist before page implementation starts.

- [x] **Step 2: Add failing API-helper/normalizer tests**

Assert:

- `frontend/lib/content.ts` normalizes `series_memberships` from snake_case and PascalCase inputs.
- invalid previous/next entries missing `id` or `title` normalize to `undefined`.
- the series detail helper, if added, calls `GET /api/v1/series/:id`.

- [x] **Step 3: Implement minimal helper/normalizer work**

Keep helper code focused: do not build UI in this task. Add i18n keys under `series.detail.*` and `series.nav.*`.

- [x] **Step 4: Run focused tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/series-nav.test.tsx
```

Expected: PASS.

---

## Task 7: Add Public Series Detail Page

**Files:**
- Create: `frontend/app/(public)/series/[id]/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/e2e/content-series.spec.ts`

- [x] **Step 1: Add failing page/e2e assertions**

Assert logged-out access, title/owner/item count rendering, ordered item links, empty state, and not-found state.

- [x] **Step 2: Implement public route**

Route must live at:

```text
frontend/app/(public)/series/[id]/page.tsx
```

- [x] **Step 3: Render backend detail**

Fetch `GET /api/v1/series/:id`. Render cover, title, owner, item count, and ordered list by `sort_order`.

- [x] **Step 4: Handle empty and not found states**

- no items: show empty series state
- 404: show not found / EmptyState
- network error: show localized load failure

- [x] **Step 5: Add Playwright assertions**

Check logged-out access, ordered items, item links, and not-found state.

---

## Task 8: Write Studio Series Management Frontend Tests

**Files:**
- Create: `frontend/tests/studio-series-page.test.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [x] **Step 1: Confirm UI spec before page code**

Run:

```powershell
rg -n "## Page: /studio/series" design/ui-spec.md
```

The section must specify list, detail, create, edit, item add/remove, up/down reorder, loading, empty, error, a11y, i18n, and screenshot checkpoints.

- [x] **Step 2: Add failing list/create tests**

Assert:

- list fetch calls `GET /api/v1/series`
- create submits `POST /api/v1/series`

- [x] **Step 3: Add failing edit/delete tests**

Assert:

- edit submits `PUT /api/v1/series/:id`
- delete submits `DELETE /api/v1/series/:id`

- [x] **Step 4: Add failing item-management tests**

Assert:

- add item submits `POST /api/v1/series/:id/items`
- remove item submits `DELETE /api/v1/series/:id/items/:itemId`
- reorder buttons submit `PUT /api/v1/series/:id/items/reorder` with full `item_ids`
- drag/drop affordances are not rendered in this implementation

- [x] **Step 5: Add i18n keys**

Add namespaces `series.studio.*`, `series.studio.form.*`, `series.studio.items.*`, `series.studio.toast.*`, and `series.studio.a11y.*`.

- [x] **Step 6: Run focused tests and confirm red**

Run:

```powershell
cd frontend
node --import tsx --test tests/studio-series-page.test.tsx
```

Expected: FAIL until the page is implemented.

---

## Task 9: Add Studio Series Management UI

**Files:**
- Create: `frontend/app/(protected)/studio/series/page.tsx`
- Modify: `frontend/components/studio/StudioSidebar.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/studio-series-page.test.tsx`
- Test: `frontend/e2e/content-series.spec.ts`

- [x] **Step 1: Add sidebar entry**

Add "内容系列管理" under creator/studio content management. Use a stable icon from `lucide-react`; do not use a text-only pill if an icon button pattern exists.

- [x] **Step 2: Implement list and create**

The page must allow:

- list my series
- create series with title, description, zone

- [x] **Step 3: Implement edit and delete**

The page must allow:

- edit title/description/cover
- delete series

- [x] **Step 4: Implement add/remove item management**

Allow:

- search/select owner-authored or contributor content
- add item
- remove item

- [x] **Step 5: Implement reorder buttons**

Allow:

- reorder with up/down buttons
- drag handles are out-of-scope for this implementation; do not render drag affordances or include drag/drop code

Every reorder operation calls:

```text
PUT /api/v1/series/:id/items/reorder
{"item_ids":[...]}
```

- [x] **Step 6: Run focused studio tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/studio-series-page.test.tsx
```

Expected: PASS.

- [x] **Step 7: Add Playwright studio flow**

Create a series, add two contents, move second up, verify reorder API payload, remove item, delete series.

---

## Task 10: Verification And Documentation Sync

**Files:**
- Modify if generated: `architecture.md`
- Screenshot outputs listed in Step 4.

- [x] **Step 1: Run backend gates**

Run:

```powershell
cd backend
go test ./internal/model -run TestContentSeriesMigration -v
go test ./internal/service -run TestSeries -v
go test ./internal/handler -run "TestSeries|TestContentDetailSeries" -v
go test ./...
go vet ./...
go build ./...
```

- [x] **Step 2: Run frontend gates**

Run:

```powershell
cd frontend
npm run test
npm run lint
npm run build
npx playwright test e2e/content-series.spec.ts
```

- [x] **Step 3: Run doc-validator**

Because this plan changes migrations and routes:

```powershell
cd tools/doc-validator
go run . --fix
```

- [x] **Step 4: Browser verification**

1. Open content in one series; verify `SeriesNav`.
2. Open first and last item; verify disabled previous/next states.
3. Open content in multiple series; verify tabs.
4. Open `/series/:id` logged out; verify list and links.
5. Open `/studio/series`; create, add, reorder, remove, delete.
6. Save screenshots:
   - `screenshots/community-content-series-nav-desktop.png`
   - `screenshots/community-content-series-nav-mobile.png`
   - `screenshots/community-content-series-detail-desktop.png`
   - `screenshots/community-content-series-detail-mobile.png`
   - `screenshots/community-content-series-studio-desktop.png`
   - `screenshots/community-content-series-studio-mobile.png`

- [x] **Step 5: Commit when implementing**

```powershell
$routeOwner = if (Test-Path backend/internal/router/routes.go) { 'backend/internal/router/routes.go' } else { 'backend/internal/handler/routes.go' }
git add -- backend/migrations/059_create_content_series.sql backend/internal/model/series.go backend/internal/model/series_migration_test.go backend/internal/repository/series_repo.go backend/internal/service/series_service.go backend/internal/service/series_service_test.go backend/internal/handler/series.go backend/internal/handler/series_test.go backend/internal/handler/content.go $routeOwner "frontend/app/(public)/series/[id]/page.tsx" "frontend/app/(protected)/studio/series/page.tsx" frontend/components/content/SeriesNav.tsx frontend/components/content/ContentDetail.tsx frontend/lib/content.ts frontend/components/studio/StudioSidebar.tsx frontend/messages/zh.json frontend/messages/en.json frontend/tests/series-nav.test.tsx frontend/tests/studio-series-page.test.tsx frontend/e2e/content-series.spec.ts screenshots/community-content-series-nav-desktop.png screenshots/community-content-series-nav-mobile.png screenshots/community-content-series-detail-desktop.png screenshots/community-content-series-detail-mobile.png screenshots/community-content-series-studio-desktop.png screenshots/community-content-series-studio-mobile.png docs/superpowers/plans/2026-06-30-omnicraft-community-content-series.md progress.txt
# Also add architecture.md if doc-validator --fix modified it during this task.
git commit -m "Community 4: content series"
```

---

## Plan Self-Check

- [x] Plan explicitly says first release is public series only.
- [x] Ownership rule distinguishes owner management from contributor addability.
- [x] New series items append after the current maximum `sort_order`.
- [x] Reorder step requires a transaction, row lock, and full item-set validation.
- [x] Cover fallback order is backend-defined.
- [x] Content detail response includes enough data for previous/next without extra client guessing.
- [x] Public series and navigation never leak pending, under-review, banned, author-deleted, or soft-deleted content.
- [x] Cover foreign key uses `ON DELETE SET NULL` and cover fallback remains valid.
- [x] Membership response is not truncated before the UI builds its `更多(N)` overflow menu.
- [x] UI spec sections are verified before UI code, and no UI details are invented outside `design/ui-spec.md`.
- [x] Studio management uses tested up/down reorder buttons; drag handles are explicitly out-of-scope.
- [x] Studio management task is split into list/create, edit/delete, item add/remove, and reorder verification.
- [x] Browser verification covers first, last, multiple-series, public detail, and studio management.
- [x] `doc-validator` is required because routes and migrations change.
