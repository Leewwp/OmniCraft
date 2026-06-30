# OmniCraft Community Collections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将旧二元收藏升级为按 zone 分区的收藏集文件夹系统，同时保持旧 `/favorites` 接口和推荐画像兼容。

**Architecture:** 新增 `collections` 与 `collection_items` 表，迁移旧 `favorites` 到每个用户每个 zone 的默认收藏集；回滚窗口内保留旧 `favorites` 表并让旧接口双写默认收藏集；推荐系统读取 `collection_items` 与历史 `favorites` 的去重并集。公开收藏集详情使用 `(public)` route group，私有权限由后端兜底。

**Tech Stack:** Go/Gin/GORM/PostgreSQL, Next.js App Router, next-intl, React Testing Library, Playwright.

---

## Cross-Plan Coordination

- Shared-file integration and migration order for the six community plans is: messages-notifications (`057`) -> browse-history (no migration) -> collections (`058`) -> content-series (`059`) -> source-linkage (`060`) -> collaboration-invites (`061`).
- `frontend/app/(protected)/messages/page.tsx`, `frontend/components/social/ChatWindow.tsx`, and `frontend/components/social/ConversationList.tsx` must land in messages-notifications before collaboration-invites extends typed invite cards.
- `frontend/components/content/ContentDetail.tsx` changes from this plan must land before content-series and source-linkage add their own detail-page sections.
- `frontend/components/studio/PublishForm.tsx` changes must land source-linkage before collaboration-invites; this plan must not edit `PublishForm.tsx`.
- `backend/config/config.go` and `backend/config.yaml` changes from browse-history and collaboration-invites must be implemented serially and rebased before verification.
- Before any UI code, grep `design/ui-spec.md` for the exact `## Page:` / `## Component:` sections named by this plan and follow those sections as the visual authority. As of 2026-06-30, `/collections/[id]`, `/user/[userId]/collections`, `CollectionPicker`, `CollectionInfoCard`, `ContentTypeFilter`, `CollectionCard`, `MasonryGrid`, `ContentCard`, `ConfirmModal`, and `Toast` are present; do not rewrite `design/ui-spec.md` unless an implementation-time check proves a required section is absent or stale.
- Authority note: `architecture.md` still lists `collections/[id]` under `(protected)`, but the community design spec and current UI spec require `frontend/app/(public)/collections/[id]/page.tsx` so public collections are browseable when logged out. Treat this as known documentation drift and implement the public route unless architecture is updated before execution.
- Expected-result convention: any "Run and confirm red" step expects FAIL for the behavior under test; any "Verify green" / "Run ... tests" step expects PASS. If the observed result differs, stop and update the plan before proceeding.
- Before implementation, run `git status --short`, reserve exact files, and stage only exact touched files. Do not use directory-level staging such as `git add backend`, `git add frontend`, `git add design`, `git add screenshots`, or `git add docs/superpowers/plans`.
- Staging note: the sample `git add` command at the end must be reduced to files actually changed in that implementation. Omit `design/ui-spec.md` when it was only read/verified; omit generated docs such as `architecture.md` unless `doc-validator` changed them during this task.

---

## File Structure

### Backend

- Create: `backend/migrations/058_create_collections.sql` - 建表、默认集约束、旧 `favorites` 回填。
- Create: `backend/internal/model/collection.go`
- Create: `backend/internal/repository/collection_repo.go`
- Create: `backend/internal/service/collection_service.go`
- Create: `backend/internal/handler/collection.go`
- Create: `backend/internal/repository/collection_repo_test.go`
- Create: `backend/internal/service/collection_service_test.go`
- Create: `backend/internal/handler/collection_test.go`
- Modify: `backend/internal/handler/routes.go` - 注册 `/collections` 路由。
- Modify: `backend/internal/repository/social_repo.go`, `backend/internal/service/social_service.go`, `backend/internal/handler/social.go` - 旧 favorites 接口双写兼容。
- Modify: `backend/internal/service/recommendation_service.go` - 推荐画像改读去重并集。
- Modify: `backend/internal/handler/content.go` - 内容详情收藏状态兼容新默认收藏集。
- Extend: `GET /api/v1/collections` - 支持 `owner_id` 可选查询参数；未传时列出当前用户收藏集，传入时由后端按 viewer 身份过滤公开/私有收藏集，供 `/user/[userId]/collections` 使用。

### Frontend

- Create: `frontend/lib/collections.ts`
- Create: `frontend/components/content/CollectionPicker.tsx`
- Create: `frontend/components/content/CollectionCard.tsx`
- Create: `frontend/components/content/CollectionInfoCard.tsx`
- Create: `frontend/components/content/ContentTypeFilter.tsx`
- Create: `frontend/app/(public)/collections/[id]/page.tsx`
- Create: `frontend/app/(public)/user/[userId]/collections/page.tsx`
- Create: `frontend/tests/collection-picker.test.tsx`
- Create: `frontend/tests/collection-detail.test.tsx`
- Create: `frontend/tests/user-collections-page.test.tsx`
- Create: `frontend/e2e/collections.spec.ts`
- Modify: `frontend/app/(protected)/studio/favorites/page.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/content/ContentDetailClient.tsx`
- Read: `design/ui-spec.md` - 对照 `/collections/[id]`、`/user/[userId]/collections`、`CollectionPicker`、`CollectionInfoCard`、`ContentTypeFilter`、`CollectionCard`；当前计划不应主动修改 UI spec。
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`

---

## Task 1: Add Migration And Models

**Files:**
- Create: `backend/migrations/058_create_collections.sql`
- Create: `backend/internal/model/collection.go`
- Test: `backend/internal/model/content_migration_test.go` or `backend/internal/model/collection_migration_test.go`

- [ ] **Step 1: Re-check migration number**

Run:

```powershell
Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 10 -ExpandProperty Name
```

If `058_` already exists, use the next available number and update implementation references.

- [ ] **Step 2: Write failing migration test**

Assert after applying the migration:

- `collections` table exists
- `collection_items` table exists
- `idx_collections_one_default_per_zone` exists and is partial on `is_default = TRUE`
- `UNIQUE (collection_id, content_item_id)` exists
- `collections.zone` only allows `original` or `fanwork`

- [ ] **Step 3: Create migration**

Migration must:

1. Create `collections`.
2. Create `collection_items`.
3. Create two default collections per user, one for `original`, one for `fanwork`.
   - Use deterministic titles until users rename them: `默认原创收藏` for `original`, `默认二创收藏` for `fanwork`.
   - Set default collection `sort_order = 0`; user-created collections append after defaults by `max(sort_order)+1`.
4. Backfill old `favorites` by joining `content_items.zone`.
5. Keep `favorites` table intact.
6. Use `ON CONFLICT DO NOTHING` for idempotent replay.

- [ ] **Step 4: Add models**

Create:

```go
type Collection struct {
    ID int64
    UserID int64
    Title string
    Description string
    Zone string
    IsDefault bool
    IsPublic bool
    SortOrder int
}

type CollectionItem struct {
    ID int64
    CollectionID int64
    ContentItemID int64
    Note string
    AddedAt time.Time
}
```

- [ ] **Step 5: Verify migration tests**

Run:

```powershell
cd backend
go test ./internal/model -run TestCollectionMigration -v
```

---

## Task 2: Implement Repository Rules

**Files:**
- Create: `backend/internal/repository/collection_repo.go`
- Test: `backend/internal/repository/collection_repo_test.go`

- [ ] **Step 1: Write failing repository tests**

Cover:

```go
func TestCollectionRepoCreateAndListByOwnerAndZone(t *testing.T) {}
func TestCollectionRepoGetPublicVisibleToAnonymous(t *testing.T) {}
func TestCollectionRepoGetPrivateVisibleOnlyToOwner(t *testing.T) {}
func TestCollectionRepoAddItemRejectsZoneMismatch(t *testing.T) {}
func TestCollectionRepoAddItemRejectsUnpublishedOrDeletedContent(t *testing.T) {}
func TestCollectionRepoAddItemRejectsDuplicate(t *testing.T) {}
func TestCollectionRepoDeleteRejectsDefaultCollection(t *testing.T) {}
func TestCollectionRepoDetailFiltersUnavailableContent(t *testing.T) {}
func TestCollectionRepoListCollectionsMarksContainsItem(t *testing.T) {}
```

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/repository -run TestCollectionRepo -v
```

- [ ] **Step 3: Implement repository methods**

Required methods:

```go
CreateCollection(ctx, collection)
ListCollections(ctx, ownerID, zone, containsContentItemID)
GetCollectionForViewer(ctx, collectionID, viewerID)
UpdateCollection(ctx, collectionID, ownerID, patch)
DeleteCollection(ctx, collectionID, ownerID)
AddItem(ctx, collectionID, ownerID, contentID, note)
RemoveItem(ctx, collectionID, ownerID, itemID)
UpdateItemNote(ctx, collectionID, ownerID, itemID, note)
ListItems(ctx, collectionID, page, pageSize, contentType)
EnsureDefaultCollection(ctx, userID, zone)
```

When `containsContentItemID` is non-nil, collection list summaries must include:

```go
ContainsItem bool   `json:"contains_item"`
ItemID       *int64 `json:"item_id,omitempty"`
```

`ItemID` is the `collection_items.id` row to remove/update if the content is already in that collection.

- [ ] **Step 4: Encode exact errors**

Map repository/service errors to these codes later:

- `COLLECTION_NOT_FOUND`
- `ZONE_MISMATCH`
- `DUPLICATE_COLLECTION_ITEM`
- `DEFAULT_COLLECTION_PROTECTED`
- `ZONE_IMMUTABLE`
- `INVALID_CONTENT`

- [ ] **Step 5: Verify green**

Run:

```powershell
cd backend
go test ./internal/repository -run TestCollectionRepo -v
```

---

## Task 3: Implement Service Compatibility With Favorites

**Files:**
- Create: `backend/internal/service/collection_service.go`
- Modify: `backend/internal/repository/social_repo.go`
- Modify: `backend/internal/service/social_service.go`
- Modify: `backend/internal/handler/social.go`
- Test: `backend/internal/service/collection_service_test.go`

- [ ] **Step 1: Add failing service tests**

Cover:

- old `Favorite(userID, contentID)` creates/keeps old `favorites` row
- old `Favorite` also inserts into the correct default collection by content `zone`
- old `Unfavorite` removes old `favorites` row and the default collection item only
- old `Unfavorite` does not remove the same content from user-created collections
- collection add clears recommendation cache as old favorite did

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/service -run "TestCollectionService|TestFavoriteCompatibility" -v
```

- [ ] **Step 3: Add collection service**

The collection service owns zone validation, default collection creation, and visible-content filtering. It should call the existing content repository rather than duplicating content visibility rules.

- [ ] **Step 4: Adapt legacy favorites**

Keep old endpoints:

- `POST /api/v1/favorites`
- `DELETE /api/v1/favorites/:contentId`
- `GET /api/v1/users/:id/favorites`

During rollback window, writes are double-write. Reads may continue from old `favorites` for compatibility, but recommendation must read the de-duped union.

- [ ] **Step 5: Verify service tests**

Run:

```powershell
cd backend
go test ./internal/service -run "TestCollectionService|TestFavoriteCompatibility" -v
```

---

## Task 4: Add Collection API Routes

**Files:**
- Create: `backend/internal/handler/collection.go`
- Modify: `backend/internal/handler/routes.go`
- Test: `backend/internal/handler/collection_test.go`

- [ ] **Step 1: Add failing handler tests**

Test all routes:

```text
POST   /api/v1/collections
GET    /api/v1/collections
GET    /api/v1/collections/:id
PUT    /api/v1/collections/:id
DELETE /api/v1/collections/:id
POST   /api/v1/collections/:id/items
DELETE /api/v1/collections/:id/items/:itemId
PUT    /api/v1/collections/:id/items/:itemId
```

- [ ] **Step 2: Assert permission behavior**

Tests must verify:

- public detail can be accessed without auth
- private detail returns `404 COLLECTION_NOT_FOUND` for non-owner
- mutations require auth and owner
- creation requires reputation via existing route guard
- `GET /api/v1/collections?zone=fanwork&content_item_id=123` returns same-zone collection summaries with `contains_item` and `item_id`
- `GET /api/v1/collections?owner_id=:userId` returns public collections for anonymous/non-owner viewers and all collections for the owner
- `GET /api/v1/collections/:id?page=1&page_size=20&content_type=article` filters collection contents by `content_type`
- blank title, title longer than 200 characters, invalid zone, and client attempts to set `is_default` all return validation errors

- [ ] **Step 3: Implement handler**

Request bodies:

```json
{"title":"神作收藏","description":"...","zone":"original","is_public":true}
{"content_item_id":567,"note":"第三章特别精彩"}
```

Detail response includes `collection`, `items`, `total`, `page`, `page_size`.

List response shape:

```json
{
  "items": [
    {
      "id": 1,
      "title": "默认原创收藏",
      "zone": "original",
      "is_default": true,
      "is_public": false,
      "sort_order": 0,
      "item_count": 3,
      "contains_item": true,
      "item_id": 99
    }
  ],
  "total": 1
}
```

`content_item_id` may be omitted. The response shape is fixed:

- when `content_item_id` is present, every list item includes `contains_item`; items that already contain the content also include `item_id`, and items that do not contain it omit `item_id`
- when `content_item_id` is absent, every list item includes `contains_item: false` and omits `item_id`

List query semantics:

- No `owner_id`: authenticated user's own collections only; optional `zone` and `content_item_id` are supported for `CollectionPicker`.
- With `owner_id`: public endpoint behavior for `/user/[userId]/collections`; anonymous/non-owner viewers receive only `is_public=true` collections, owner receives both public and private collections.
- When `content_item_id` is supplied, the handler must first verify the content exists and use its `zone` to keep the collection list same-zone even if the query omits `zone`.

Validation rules:

- `title` is trimmed and must be 1-200 characters.
- `description` defaults to `""`.
- `zone` must be `original` or `fanwork`.
- `is_default` and `user_id` are server-controlled and must not be accepted from create/update payloads.
- `PUT /api/v1/collections/:id` may update only `title`, `description`, `is_public`, and `sort_order`; any `zone` field returns `400 ZONE_IMMUTABLE`.

Handler tests must assert both shapes.

- [ ] **Step 4: Register routes**

Public detail uses auth-if-present middleware:

```go
v1.GET("/collections/:id", optAuth, collectionHandler.GetCollection)
```

Mutations use auth and interaction guard.

- [ ] **Step 5: Verify handler tests**

Run:

```powershell
cd backend
go test ./internal/handler -run TestCollection -v
```

---

## Task 5: Update Recommendation Compatibility

**Files:**
- Modify: `backend/internal/service/recommendation_service.go`
- Test: `backend/internal/service/recommendation_service_test.go`

- [ ] **Step 1: Add failing recommendation tests**

Cover:

- user with only `collection_items` has non-zero interaction profile
- item present in both `favorites` and `collection_items` is counted once
- content type/category weights remain consistent with old favorites behavior

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/service -run "TestRecommendation.*Favorite|TestRecommendation.*Collection" -v
```

- [ ] **Step 3: Implement de-duped union query**

Replace direct `favorites` reads with a query equivalent to:

```sql
SELECT DISTINCT content_item_id FROM (
  SELECT content_item_id FROM favorites WHERE user_id = ?
  UNION
  SELECT ci.content_item_id
  FROM collection_items ci
  JOIN collections c ON c.id = ci.collection_id
  WHERE c.user_id = ?
) x
```

- [ ] **Step 4: Verify recommendation tests**

Run:

```powershell
cd backend
go test ./internal/service -run "TestRecommendation.*Favorite|TestRecommendation.*Collection" -v
```

---

## Task 6: Confirm UI Spec And Add Collection API Helper

**Files:**
- Create: `frontend/lib/collections.ts`
- Read: `design/ui-spec.md`
- Test: `frontend/tests/collection-picker.test.tsx`

- [ ] **Step 1: Confirm UI spec before UI code**

Run:

```powershell
rg -n "## Page: /collections/\\[id\\]|## Page: /user/\\[userId\\]/collections|## Component: CollectionPicker|## Component: CollectionInfoCard|## Component: ContentTypeFilter|## Component: CollectionCard" design/ui-spec.md
```

Expected: all sections are present. If a future branch lacks one, stop and repair UI spec in an explicitly scoped docs/design step before UI code. Implementation must use `CollectionPicker` as the canonical component name. The legacy add-to-collection modal export is deprecated and may only remain as a compatibility wrapper or internal implementation detail; new imports must reference `CollectionPicker`.

- [ ] **Step 2: Add failing API-helper tests**

Assert `frontend/lib/collections.ts` functions call:

- `GET /api/v1/collections?zone=...&content_item_id=...`
- `POST /api/v1/collections`
- `GET /api/v1/collections/:id?page=...&page_size=...&content_type=...`
- `POST /api/v1/collections/:id/items`
- `DELETE /api/v1/collections/:id/items/:itemId`
- `PUT /api/v1/collections/:id`
- `DELETE /api/v1/collections/:id`

- [ ] **Step 3: Implement API helper**

Create typed functions in `frontend/lib/collections.ts`:

```ts
listCollections(params?: { zone?: string; contentItemId?: number })
createCollection(input)
getCollection(id, params)
addCollectionItem(collectionId, contentItemId, note?)
removeCollectionItem(collectionId, itemId)
updateCollection(id, patch)
deleteCollection(id)
```

Normalize list summaries so `contains_item` is always boolean and `item_id` is present only when the backend returns a numeric collection item id.

- [ ] **Step 4: Run focused helper tests**

Run:

```powershell
cd frontend
npm run test -- tests/collection-picker.test.tsx
```

Expected: helper tests PASS.

---

## Task 7: Build Collection Picker And Studio Favorites UI

**Files:**
- Create: `frontend/components/content/CollectionPicker.tsx`
- Create: `frontend/components/content/CollectionCard.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/content/ContentDetailClient.tsx`
- Modify: `frontend/app/(protected)/studio/favorites/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/collection-picker.test.tsx`

- [ ] **Step 1: Add failing component tests**

Assert:

- picker lists only collections with matching `zone`
- picker calls list API with current `content_item_id`
- "+ 新建" creates collection inline and selects it
- duplicate item shows "已添加" state
- already-added state comes from `contains_item`, not from attempting a duplicate add
- 10+ collections display search field
- content detail opens picker instead of direct favorite toggle
- no new code imports the legacy add-to-collection modal export

- [ ] **Step 2: Implement `CollectionPicker`**

`CollectionPicker` must:

- render stable modal dimensions
- list same-zone collections
- call `listCollections({ zone, contentItemId })` so the backend supplies `contains_item` / `item_id`
- show already-added state without submitting a duplicate request
- use `item_id` from the list response when removing an already-added item
- allow inline create
- update selected state without a full page reload
- use i18n for every visible string

- [ ] **Step 3: Update content detail entry**

`ContentDetail` / `ContentDetailClient` must open `CollectionPicker` from the existing favorite/add-to-collection action. Keep legacy `/favorites` compatibility behavior in backend; do not keep a direct one-click favorite toggle as the primary new UI.

- [ ] **Step 4: Update `/studio/favorites`**

`/studio/favorites` must show original and fanwork zones separately, use `CollectionCard`, provide create/edit/delete collection actions, and protect default collections from deletion in UI as well as backend.

- [ ] **Step 5: Add i18n keys**

Add all visible strings under `collections.picker.*`, `collections.card.*`, and `studioFavorites.*` in both `frontend/messages/zh.json` and `frontend/messages/en.json`.

- [ ] **Step 6: Run frontend tests**

Run:

```powershell
cd frontend
npm run test -- tests/collection-picker.test.tsx
```

---

## Task 8: Add Public Collection Detail Page

**Files:**
- Create: `frontend/app/(public)/collections/[id]/page.tsx`
- Create: `frontend/components/content/CollectionInfoCard.tsx`
- Create: `frontend/components/content/ContentTypeFilter.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/collection-detail.test.tsx`
- Test: `frontend/e2e/collections.spec.ts`

- [ ] **Step 1: Add failing detail tests**

Assert:

- public collection renders for logged-out users
- private collection `404 COLLECTION_NOT_FOUND` renders a non-leaking EmptyState
- `CollectionInfoCard` shows title, description, author, item count, zone, and public/private badge
- `ContentTypeFilter` updates the URL query with `content_type`
- owner controls render only for owner
- default collection delete action is disabled

- [ ] **Step 2: Implement route in public group**

Route path must be:

```text
frontend/app/(public)/collections/[id]/page.tsx
```

Do not place this page under `(protected)` because public collections are viewable by logged-out users.

- [ ] **Step 3: Implement `CollectionInfoCard`**

`CollectionInfoCard` is in scope for this plan. It renders the page summary block specified in `design/ui-spec.md`; it is not a generic card wrapper for the page section.

- [ ] **Step 4: Implement `ContentTypeFilter`**

`ContentTypeFilter` is in scope for this plan. It supports `all`, `image`, `article`, `video`, `audio`, `template`, `sheet_music`, `mod`, `prompt`, and `other`, and it updates `content_type` query params without hardcoded visible strings.

- [ ] **Step 5: Fetch backend detail**

Use `GET /api/v1/collections/:id?page=1&page_size=...`. Render only content returned by backend; do not attempt client-side private filtering.

- [ ] **Step 6: Handle error states**

- `404 COLLECTION_NOT_FOUND`: EmptyState / not found style
- `403`: EmptyState if backend returns forbidden
- no items: empty collection state

- [ ] **Step 7: Add Playwright checks**

Mock or seed:

- public collection logged out renders cards
- private collection logged out shows EmptyState
- owner sees remove controls
- non-owner does not see remove controls
- content type filter updates query and refetches

---

## Task 9: Add User Collections List Page

**Files:**
- Create: `frontend/app/(public)/user/[userId]/collections/page.tsx`
- Create or reuse: `frontend/components/content/CollectionCard.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/user-collections-page.test.tsx`
- Test: `frontend/e2e/collections.spec.ts`

- [ ] **Step 1: Add failing page tests**

Assert:

- own profile shows private and public collections
- another user's page shows public collections only
- logged-out access to another user's page shows public collections only
- empty own page shows create CTA
- empty other-user page shows read-only EmptyState
- cards link to `/collections/:id`

- [ ] **Step 2: Implement route**

Route path must be:

```text
frontend/app/(public)/user/[userId]/collections/page.tsx
```

The page uses `GET /api/v1/collections?owner_id=:userId`. Task 4 includes this handler contract; do not defer `owner_id` support to the page implementation, because the UI spec requires a public user collection list and visibility must be enforced server-side.

- [ ] **Step 3: Implement list UI**

Use `CollectionCard`, not `CollectionInfoCard`. Keep the page as a simple grid with no nested cards and i18n under `collections.userList.*`.

- [ ] **Step 4: Run focused page tests**

Run:

```powershell
cd frontend
npm run test -- tests/user-collections-page.test.tsx
```

---

## Task 10: Full Verification And Documentation Sync

**Files:**
- Modify if generated: `architecture.md`
- Screenshot outputs listed in Step 4.

- [ ] **Step 1: Run backend gates**

Run:

```powershell
cd backend
go test ./internal/repository -run TestCollection -v
go test ./internal/service -run "TestCollection|TestFavorite|TestRecommendation" -v
go test ./internal/handler -run TestCollection -v
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 2: Run frontend gates**

Run:

```powershell
cd frontend
npm run test
npm run lint
npm run build
npm run test:e2e -- collections.spec.ts
```

- [ ] **Step 3: Run doc-validator**

Because this plan changes migrations and routes:

```powershell
cd tools/doc-validator
go run . --fix
```

- [ ] **Step 4: Browser verification**

1. Logged-in user creates original and fanwork collections in `/studio/favorites`.
2. User edits title, public flag, and sort order.
3. User cannot delete default collection.
4. Content detail opens `CollectionPicker`; adding same-zone content succeeds.
5. Public `/collections/:id` works logged out.
6. Private `/collections/:id` is hidden from non-owner.
7. `/user/:userId/collections` shows public collections to logged-out users.
8. Save screenshots:
   - `screenshots/community-collections-detail-desktop.png`
   - `screenshots/community-collections-owner-mobile.png`
   - `screenshots/community-collections-picker-desktop.png`
   - `screenshots/community-collections-picker-mobile.png`
   - `screenshots/community-collections-user-list-desktop.png`

- [ ] **Step 5: Commit when implementing**

```powershell
git add -- backend/migrations/058_create_collections.sql backend/internal/model/collection.go backend/internal/model/collection_migration_test.go backend/internal/repository/collection_repo.go backend/internal/repository/collection_repo_test.go backend/internal/service/collection_service.go backend/internal/service/collection_service_test.go backend/internal/handler/collection.go backend/internal/handler/collection_test.go backend/internal/handler/routes.go backend/internal/repository/social_repo.go backend/internal/service/social_service.go backend/internal/handler/social.go backend/internal/service/recommendation_service.go backend/internal/service/recommendation_service_test.go backend/internal/handler/content.go frontend/lib/collections.ts frontend/components/content/CollectionPicker.tsx frontend/components/content/CollectionCard.tsx frontend/components/content/CollectionInfoCard.tsx frontend/components/content/ContentTypeFilter.tsx "frontend/app/(protected)/studio/favorites/page.tsx" "frontend/app/(public)/collections/[id]/page.tsx" "frontend/app/(public)/user/[userId]/collections/page.tsx" frontend/components/content/ContentDetail.tsx frontend/components/content/ContentDetailClient.tsx frontend/messages/zh.json frontend/messages/en.json frontend/tests/collection-picker.test.tsx frontend/tests/collection-detail.test.tsx frontend/tests/user-collections-page.test.tsx frontend/e2e/collections.spec.ts design/ui-spec.md screenshots/community-collections-detail-desktop.png screenshots/community-collections-owner-mobile.png screenshots/community-collections-picker-desktop.png screenshots/community-collections-picker-mobile.png screenshots/community-collections-user-list-desktop.png architecture.md docs/superpowers/plans/2026-06-30-omnicraft-community-collections.md progress.txt
git commit -m "Community 3: collections folder system"
```

---

## Plan Self-Check

- [ ] Plan preserves old `favorites` API during rollback window.
- [ ] Plan explicitly double-writes old favorite actions to default collections.
- [ ] Recommendation compatibility uses de-duped union, not only one table.
- [ ] Public collection route is under `(public)`, with backend permission enforcement.
- [ ] Default collections are protected from deletion.
- [ ] Default collections have deterministic titles and sort order.
- [ ] Collection list API can mark whether the current content is already contained in each collection.
- [ ] Collection list response shape for absent `content_item_id` is fixed as `contains_item:false` on every item with no `item_id`.
- [ ] `CollectionPicker` is canonical; the legacy add-to-collection modal export is deprecated and not imported by new code.
- [ ] `CollectionInfoCard`, `ContentTypeFilter`, and `/user/[userId]/collections` are in scope with tests and screenshots.
- [ ] Zone mismatch behavior and `ZONE_IMMUTABLE` are explicitly tested.
- [ ] Migration is idempotent and does not drop `favorites`.
- [ ] Browser verification covers public, private, owner, and non-owner states.
