# OmniCraft Community Original Fanwork Source Linkage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增强原创/二创联动，支持二创基于二创的 `source_fanwork_id` 来源链，并在详情页克制展示灵感来源、相关二创和衍生作品。

**Architecture:** 扩展 `content_items` 增加 `source_fanwork_id`，将发布校验从单一 `source_original_id` 规则升级为 fanwork 三选一来源规则；`related-fanworks` 对 original 查二创，对 fanwork 查衍生作品；前端发布表单支持 IP、原创来源或二创来源三选一，详情页新增轻量来源归因和横向作品行。

**Tech Stack:** Go/Gin/GORM/PostgreSQL, Next.js App Router, next-intl, React Testing Library, Playwright.

---

## Cross-Plan Coordination

- Execution source: this is part of the 2026-06-30 community feature plan family, derived from `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`. It is not a historical `task.json` task and not a 2026-05-30 Beta roadmap checkbox; executing it requires an explicit user request naming this plan or the community feature family.
- Shared-file integration and migration order for the six community plans is: messages-notifications (`057`) -> browse-history (no migration) -> collections (`058`) -> content-series (`059`) -> source-linkage (`060`) -> collaboration-invites (`061`).
- `frontend/app/(protected)/messages/page.tsx`, `frontend/components/social/ChatWindow.tsx`, and `frontend/components/social/ConversationList.tsx` must land in messages-notifications before collaboration-invites extends typed invite cards.
- `frontend/components/content/ContentDetail.tsx` changes from collections and content-series must already be present; this plan adds source attribution and related/derivative rows after them.
- `frontend/components/studio/PublishForm.tsx` source fields in this plan must land before collaboration-invites adds the collaborator picker.
- `backend/config/config.go` and `backend/config.yaml` changes from browse-history and collaboration-invites must be implemented serially and rebased before verification.
- Before any UI code, grep `design/ui-spec.md` for the exact `## Page:` / `## Component:` sections named by this plan and follow those sections as the visual authority. As of 2026-06-30, `/studio/publish/fanwork`, `SourceContentPicker`, `RelatedFanworks`, `SourceAttribution`, `ContentDetail`, and `ContentCard` are present; do not rewrite `design/ui-spec.md` unless an implementation-time check proves a required section is absent or stale.
- Authority alignment note: `AGENTS.md`, `CLAUDE.md`, and `architecture.md` must describe the same fanwork three-source model as `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`. Task 0 verifies that alignment first; only update those co-authoritative docs if the implementation branch still contains stale single-source prose.
- Expected-result convention: any "Run and confirm red" step expects FAIL for the behavior under test; any "Verify green" / "Run ... tests" step expects PASS. If the observed result differs, stop and update the plan before proceeding.
- Frontend focused test convention: current `frontend/package.json` defines `npm run test` as a fixed suite, so focused TS/TSX tests in this plan use `node --import tsx --test <file>` directly. Do not write `npm run test -- <file>` unless the package script is changed first.
- Before implementation, run `git status --short`, reserve exact files, and stage only exact touched files. Do not use directory-level staging such as `git add backend`, `git add frontend`, `git add design`, `git add screenshots`, or `git add docs/superpowers/plans`.
- Staging note: the sample `git add` command at the end must be reduced to files actually changed in that implementation. Omit `design/ui-spec.md` when it was only read/verified; omit generated docs such as `architecture.md` unless `doc-validator` changed them during this task.

---

## File Structure

### Backend

- Read/modify if stale: `AGENTS.md`, `CLAUDE.md`, `architecture.md` - 先验证来源规则和发布/详情 API 文档是否已与本计划的 fanwork 三选一来源模型一致；仅当分支仍有旧单来源规则时再更新。
- Create: `backend/migrations/060_add_source_fanwork_id.sql`.
- Modify: `backend/internal/model/content.go`
- Modify: `backend/internal/repository/content_repo.go`
- Modify: `backend/internal/service/content_service.go`
- Modify: `backend/internal/handler/content.go`
- Modify: `backend/internal/pkg/response/safe_error.go` only if central error mapping needs new codes.
- Modify: `backend/internal/model/content_migration_test.go`
- Modify: `backend/internal/service/content_source_test.go`
- Modify: `backend/internal/handler/content_publish_route_test.go`

### Frontend

- Create: `frontend/components/content/RelatedFanworks.tsx`
- Create: `frontend/components/content/SourceAttribution.tsx`
- Create: `frontend/components/studio/SourceContentPicker.tsx`.
- Modify: `frontend/lib/content.ts`
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/app/(protected)/studio/publish/fanwork/page.tsx`
- Modify: `frontend/app/(public)/content/[contentId]/page.tsx`
- Modify: `frontend/app/(public)/original/[contentId]/page.tsx`
- Modify: `frontend/app/(public)/original/[contentId]/fanworks/page.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/content/ContentSidebar.tsx` only if existing sidebar source display must stay aligned.
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`
- Read before UI code: `design/ui-spec.md` sections for `/studio/publish/fanwork`, `SourceContentPicker`, `RelatedFanworks`, `SourceAttribution`, `ContentDetail`, and `ContentCard`.
- Test: `frontend/e2e/studio-publish-fanwork.spec.ts`

---

## Task 0: Align Authoritative Source-Linkage Documentation

**Files:**
- Read/modify if stale: `AGENTS.md`
- Read/modify if stale: `CLAUDE.md`
- Read/modify if stale: `architecture.md`
- Read: `docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md`

- [ ] **Step 1: Confirm conflict scope**

Read the current "原创/二创来源联动规则" sections in `AGENTS.md` and `CLAUDE.md`, and the publish/content-detail/related-fanworks sections in `architecture.md`. Confirm whether they already describe the fanwork three-source model. If they still describe a single-source `source_original_id` model, continue with Steps 2-3; if already aligned, record that Task 0 is verification-only and do not edit these files.

- [ ] **Step 2: Update agent business rules**

If stale prose remains, in both `AGENTS.md` and `CLAUDE.md`, replace the old single-source bullets with the community design model:

- `zone='original'` rejects both `source_original_id` and `source_fanwork_id`
- `zone='fanwork'` requires at least one of `ip_id`, `source_original_id`, or `source_fanwork_id`
- `source_original_id` and `source_fanwork_id` are mutually exclusive
- `source_original_id` must point to `zone='original' AND status='published'`
- `source_fanwork_id` must point to `zone='fanwork' AND status='published'`
- legacy fanworks without content sources remain valid historical rows

- [ ] **Step 3: Update architecture API/schema prose**

If stale prose remains, update `architecture.md` in hand-written prose sections (outside any `<!-- AUTO-GENERATED START -->` / `<!-- AUTO-GENERATED END -->` blocks managed by `doc-validator`) so that content detail and publish examples mention `source_fanwork_id`, `source_fanwork`, fanwork-source related rows, and the same exact error codes used in this plan. If `architecture.md` does not use explicit auto-generated markers, treat sections whose headings match route/schema table patterns (e.g. "## API Routes", "## Database Schema") as likely auto-generated and avoid hand-editing their structured content. Do not hand-edit auto-generated schema/route sections; if later code changes require generated updates, Task 8 runs `doc-validator`.

- [ ] **Step 4: Verify no stale source rule remains**

Run:

```powershell
rg -n "单来源模型|最多绑定一个原创|只有 `zone='fanwork'` 允许填写 `source_original_id`|/publish\\?zone=fanwork&source_original_id" AGENTS.md CLAUDE.md architecture.md
```

Expected: no stale old-model prose remains except in explicitly labeled historical notes.

---

## Task 1: Add Source Fanwork Migration And Model

**Files:**
- Create: `backend/migrations/060_add_source_fanwork_id.sql`
- Modify: `backend/internal/model/content.go`
- Test: `backend/internal/model/content_migration_test.go`

- [ ] **Step 1: Re-check migration number**

Run:

```powershell
Get-ChildItem backend\migrations | Sort-Object Name | Select-Object -Last 10 -ExpandProperty Name
```

Expected number is `060_add_source_fanwork_id.sql` because this plan lands before collaboration-invites. If `060_` is already occupied by an unrelated migration at implementation time, stop and update all six community plans plus the source spec migration table before continuing.

- [ ] **Step 2: Write failing migration test**

Add a Postgres-backed test asserting:

- `content_items.source_fanwork_id` exists
- FK references `content_items(id)` with `ON DELETE SET NULL`
- index `idx_content_items_source_fanwork` exists
- index is partial where `source_fanwork_id IS NOT NULL`

- [ ] **Step 3: Implement migration**

DDL:

```sql
ALTER TABLE content_items
  ADD COLUMN IF NOT EXISTS source_fanwork_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_content_items_source_fanwork
  ON content_items (source_fanwork_id, status, created_at DESC)
  WHERE source_fanwork_id IS NOT NULL;
```

- [ ] **Step 4: Add model fields**

In `ContentItem`:

```go
SourceFanworkID *int64       `gorm:"index" json:"source_fanwork_id,omitempty"`
SourceFanwork   *ContentItem `gorm:"foreignKey:SourceFanworkID" json:"source_fanwork,omitempty"`
```

- [ ] **Step 5: Verify**

Run:

```powershell
cd backend
go test ./internal/model -run TestContentSourceMigration -v
```

---

## Task 2: Replace Source Validation With Explicit Error Codes

**Files:**
- Modify: `backend/internal/service/content_service.go`
- Modify: `backend/internal/service/content_source_test.go`

- [ ] **Step 1: Write failing service validation tests**

Table cases:

- `zone=original` with `source_original_id` -> `SOURCE_NOT_ALLOWED_FOR_ORIGINAL`
- `zone=original` with `source_fanwork_id` -> `SOURCE_NOT_ALLOWED_FOR_ORIGINAL`
- `zone=fanwork` with no `ip_id`, `source_original_id`, `source_fanwork_id` -> `FANWORK_SOURCE_REQUIRED`
- `zone=fanwork` with both source IDs -> `MULTIPLE_SOURCE_CONFLICT`
- source original not original/published -> `SOURCE_ORIGINAL_UNAVAILABLE`
- source fanwork not fanwork/published -> `SOURCE_FANWORK_UNAVAILABLE`
- IP-only fanwork succeeds
- original-source fanwork succeeds
- fanwork-source fanwork succeeds

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/service -run TestValidateSource -v
```

- [ ] **Step 3: Add input field**

In `PublishContentInput`:

```go
SourceFanworkID *int64 `json:"source_fanwork_id"`
```

- [ ] **Step 4: Implement validation**

Create a single validation helper that receives zone, `ipID`, source original, and source fanwork. Return typed errors that handler can map to the exact codes.

Do not enforce this rule for existing historical rows; only new publish/update payloads pass through this validation.

- [ ] **Step 5: Persist source fanwork**

Set `ContentItem.SourceFanworkID` during creation.

- [ ] **Step 6: Verify service tests**

Run:

```powershell
cd backend
go test ./internal/service -run TestValidateSource -v
```

---

## Task 3: Update Publish And Content Detail Contract

**Files:**
- Modify: `backend/internal/handler/content.go`
- Modify: `backend/internal/handler/content_publish_route_test.go`
- Modify: `frontend/lib/content.ts`

- [ ] **Step 1: Add failing route tests**

Extend `content_publish_route_test.go` to assert:

- fanwork can publish with only `source_fanwork_id`
- fanwork without IP or source fails
- original with either source fails
- both source IDs fail
- `GET /contents/:id` returns `source_fanwork: { id, title, zone: "fanwork" }` when visible
- `GET /contents/:id` still returns `source_original: { id, title, zone: "original" }` for original-source fanworks

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestCreateContentRoute -v
```

- [ ] **Step 3: Map exact error codes**

Handler mapping:

- `SOURCE_NOT_ALLOWED_FOR_ORIGINAL` -> 400
- `FANWORK_SOURCE_REQUIRED` -> 400
- `MULTIPLE_SOURCE_CONFLICT` -> 400
- `SOURCE_ORIGINAL_UNAVAILABLE` -> 400
- `SOURCE_FANWORK_UNAVAILABLE` -> 400

- [ ] **Step 4: Add source fanwork summary**

When content has `SourceFanworkID`, fetch source if visible and return:

```json
"source_fanwork": {"id": 123, "title": "...", "zone": "fanwork"}
```

Also include `zone: "original"` in `source_original` summaries returned from touched content detail paths. If source is unavailable, include `source_fanwork_id` but omit summary so frontend can render "内容已下架".

- [ ] **Step 5: Update frontend normalizer**

Add to `frontend/lib/content.ts`:

- `source_fanwork_id`
- `source_fanwork`
- `source_original`

Keep current rule: objects without valid `id/title/zone` must not become clickable cards. `SourceAttribution` may render unavailable-source text from `source_*_id` alone, but it must not create a link without a summary containing all three fields.

- [ ] **Step 6: Verify route tests**

Run:

```powershell
cd backend
go test ./internal/handler -run TestCreateContentRoute -v
```

---

## Task 4: Enhance Related Fanworks API

**Files:**
- Modify: `backend/internal/repository/content_repo.go`
- Modify: `backend/internal/handler/content.go`
- Test: `backend/internal/handler/content_publish_route_test.go` or new focused test

- [ ] **Step 1: Add failing API tests**

Cover:

- original content returns rows where `source_original_id = :id`
- fanwork content returns rows where `source_fanwork_id = :id`
- non-published and deleted children are excluded
- `content_type` filter works
- `sort=hot` default orders by hot score / popularity
- `sort=new` orders by newest
- `limit` aliases `page_size`
- `page_size` wins over `limit`

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestListRelatedFanworks -v
```

- [ ] **Step 3: Add repository filter**

In `ListContentsFilter`, add:

```go
SourceFanworkID *int64
```

Apply `WHERE source_fanwork_id = ?` when present.

- [ ] **Step 4: Modify handler logic**

For `GET /api/v1/contents/:id/related-fanworks`:

- if source content `zone == original`, filter by `SourceOriginalID`
- if source content `zone == fanwork`, filter by `SourceFanworkID`
- otherwise return empty list or validation error only if impossible by model

Do not keep old `NOT_ORIGINAL` behavior for fanwork sources.

- [ ] **Step 5: Verify API tests**

Run:

```powershell
cd backend
go test ./internal/handler -run TestListRelatedFanworks -v
```

---

## Task 5: Confirm Publish UI Spec And Source Picker API

**Files:**
- Read: `design/ui-spec.md`
- Create: `frontend/components/studio/SourceContentPicker.tsx`
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/app/(protected)/studio/publish/fanwork/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: add focused tests under `frontend/tests/studio-publish-fanwork.test.tsx`
- Test: `frontend/e2e/studio-publish-fanwork.spec.ts`

- [ ] **Step 1: Confirm UI spec before UI code**

Run:

```powershell
rg -n "## Page: /studio/publish/fanwork|## Component: SourceContentPicker|## Component: CollabUserPicker" design/ui-spec.md
```

The publish page spec must define loading, empty, validation, error, disabled, a11y, i18n namespace, query prefill, and screenshot checkpoints. It must state that `SourceContentPicker` owns inspiration source selection and `CollabUserPicker` is reserved for the later collaboration-invites plan.

- [ ] **Step 2: Add failing source picker tests**

Assert:

- `sourceKind="original"` searches `GET /api/v1/contents/search?zone=original&q=<query>&limit=8`
- `sourceKind="fanwork"` searches `GET /api/v1/contents/search?zone=fanwork&q=<query>&limit=8`
- result rows require `id`, `title`, and matching `zone`
- selecting a result emits the selected content summary
- clearing selection emits `undefined`
- loading, empty, and error states render localized text

- [ ] **Step 3: Implement `SourceContentPicker`**

Create:

```ts
interface SourceContentPickerProps {
  sourceKind: "original" | "fanwork";
  selected?: { id: number; title: string; zone: "original" | "fanwork" };
  onSelect: (content?: { id: number; title: string; zone: "original" | "fanwork" }) => void;
}
```

Do not create a fanwork-specific picker file. Existing original-source picker exports must be migrated to use `SourceContentPicker` or kept only as compatibility wrappers that internally render `SourceContentPicker`.

- [ ] **Step 4: Run focused picker tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/studio-publish-fanwork.test.tsx
```

Expected: picker tests PASS before editing `PublishForm.tsx`.

---

## Task 6: Update Fanwork Publish Flow

**Files:**
- Modify: `frontend/components/studio/PublishForm.tsx`
- Modify: `frontend/app/(protected)/studio/publish/fanwork/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/studio-publish-fanwork.test.tsx`
- Test: `frontend/e2e/studio-publish-fanwork.spec.ts`

- [ ] **Step 1: Add failing publish tests**

Cover:

- publish fanwork with IP only
- publish fanwork with source original only
- publish fanwork with source fanwork only
- submit with no IP/source is blocked before API call
- selecting source original clears source fanwork
- selecting source fanwork clears source original
- query prefill `?source_original_id=...` loads source summary
- query prefill `?source_fanwork_id=...` loads source summary
- query prefill with both IDs keeps `source_original_id`, clears `source_fanwork_id`, and shows the localized warning specified by `design/ui-spec.md`
- invalid query prefill id shows a localized non-blocking warning and leaves the picker empty

- [ ] **Step 2: Change validation copy**

The UI must no longer say IP is required for all fanworks. It must say fanwork requires IP or inspiration source.

- [ ] **Step 3: Wire `SourceContentPicker` into `PublishForm`**

Use two `SourceContentPicker` instances:

- one with `sourceKind="original"` for `source_original_id`
- one with `sourceKind="fanwork"` for `source_fanwork_id`

Both must call the existing search route `GET /api/v1/contents/search` with `zone`, `q`, and `limit=8`. The current backend route supports `zone`; do not add a second content search API in this plan.

- [ ] **Step 4: Submit payload**

Payload may include only one of:

- `source_original_id`
- `source_fanwork_id`

It may include `ip_id` along with either source if user selected an IP. It must never include both source IDs.

- [ ] **Step 5: Add i18n keys**

Add visible strings under `studio.publish.fanwork.source.*`, `sourceContentPicker.*`, and `studio.publish.fanwork.validation.*`.

- [ ] **Step 6: Run Playwright focused test**

Run:

```powershell
cd frontend
npx playwright test e2e/studio-publish-fanwork.spec.ts
```

---

## Task 7: Add Source Attribution And Related Rows

**Files:**
- Read: `design/ui-spec.md`
- Create: `frontend/components/content/SourceAttribution.tsx`
- Create: `frontend/components/content/RelatedFanworks.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/app/(public)/content/[contentId]/page.tsx`
- Modify: `frontend/app/(public)/original/[contentId]/page.tsx`
- Modify: `frontend/app/(public)/original/[contentId]/fanworks/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: add to `frontend/tests/source-linkage-components.test.tsx`

- [ ] **Step 1: Confirm UI spec before component code**

Run:

```powershell
rg -n "## Component: RelatedFanworks|## Component: SourceAttribution" design/ui-spec.md
```

Expected: both sections are present. If a future branch lacks one, stop and repair UI spec in an explicitly scoped docs/design step before coding. Both components must follow restrained visual treatment.

- [ ] **Step 2: Add failing component tests**

Assert:

- `SourceAttribution` links original sources to `/original/:id`
- `SourceAttribution` links fanwork sources to `/content/:id`
- IP-only fanwork renders no attribution row
- source id without summary renders unavailable gray text with no link
- `RelatedFanworks` hides when total is zero
- `RelatedFanworks` shows "查看全部" only when total is greater than 8 and `viewAllHref` is present
- derivative row label uses "衍生作品" via i18n and never renders "三创"

- [ ] **Step 3: Implement `SourceAttribution`**

Rules:

- fanwork with `source_original` links to original detail
- fanwork with `source_fanwork` links to content detail
- fanwork with only IP and no content source renders nothing
- source ID exists but summary absent renders gray text "灵感来源：内容已下架" and is not clickable

Place it under title metadata and above body.

- [ ] **Step 4: Implement `RelatedFanworks`**

Props:

```ts
interface RelatedFanworksProps {
  sourceContentId: number;
  sourceZone: "original" | "fanwork";
  titleKey: string;
  createHref?: string;
  viewAllHref?: string;
}
```

Behavior:

- fetch related endpoint with `page=1&page_size=8`
- hide when `total=0`
- horizontal scroll row of small content cards
- "查看全部" only when `total > 8` and a valid `viewAllHref` is provided
- original create link points to `/studio/publish/fanwork?source_original_id=<id>`
- fanwork derivative create link, when enabled, points to `/studio/publish/fanwork?source_fanwork_id=<id>`
- fanwork derivative row uses label "衍生作品", not "三创"

- [ ] **Step 5: Wire detail pages**

Original detail:

- show `RelatedFanworks` below body and above comments
- keep existing `/original/[contentId]/fanworks` full page working and pass it as `viewAllHref`

Fanwork detail:

- show `SourceAttribution` near title
- show derivative `RelatedFanworks` below body and above comments; do not create a new derivative-list page in this plan, so omit `viewAllHref` unless such a route already exists
- do not add recursive source-chain UI

> **首版范围裁剪**：衍生作品"查看全部"列表页（类似 `/original/[contentId]/fanworks`）延后到后续版本。首版仅展示 `RelatedFanworks` 横向滚动卡片行（最多 8 张），不提供 `viewAllHref`。

- [ ] **Step 6: Run focused component tests**

Run:

```powershell
cd frontend
node --import tsx --test tests/source-linkage-components.test.tsx
```

---

## Task 8: Full Verification And Documentation Sync

**Files:**
- Modify if generated: `architecture.md`
- Screenshot outputs listed in Step 5.

- [ ] **Step 1: Run backend focused gates**

Run:

```powershell
cd backend
go test ./internal/model -run TestContentSourceMigration -v
go test ./internal/service -run TestValidateSource -v
go test ./internal/handler -run "TestCreateContentRoute|TestListRelatedFanworks" -v
```

- [ ] **Step 2: Run backend full gates**

Run:

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 3: Run frontend gates**

Run:

```powershell
cd frontend
npm run test
npm run lint
npm run build
npx playwright test e2e/studio-publish-fanwork.spec.ts
```

- [ ] **Step 4: Run doc-validator**

Because this plan changes migrations:

```powershell
cd tools/doc-validator
go run . --fix
```

- [ ] **Step 5: Browser verification**

1. Original detail with fanworks shows inline related row, max 8 cards, create link.
2. Original detail without fanworks hides the row.
3. Fanwork with `source_original` shows lightweight source link.
4. Fanwork with `source_fanwork` shows source link to `/content/:id`.
5. Fanwork with derivatives shows "衍生作品" row.
6. Publish fanwork with IP only, original source only, and fanwork source only.
7. Submit fanwork with no IP/source; localized validation blocks submit.
8. Save screenshots:
   - `screenshots/community-source-picker-desktop.png`
   - `screenshots/community-source-picker-mobile.png`
   - `screenshots/community-source-attribution-desktop.png`
   - `screenshots/community-source-attribution-unavailable.png`
   - `screenshots/community-related-fanworks-desktop.png`
   - `screenshots/community-derivatives-mobile.png`

- [ ] **Step 6: Commit when implementing**

```powershell
git add -- AGENTS.md CLAUDE.md architecture.md backend/migrations/060_add_source_fanwork_id.sql backend/internal/model/content.go backend/internal/model/content_migration_test.go backend/internal/repository/content_repo.go backend/internal/service/content_service.go backend/internal/service/content_source_test.go backend/internal/handler/content.go backend/internal/handler/content_publish_route_test.go backend/internal/pkg/response/safe_error.go frontend/components/content/RelatedFanworks.tsx frontend/components/content/SourceAttribution.tsx frontend/components/studio/SourceContentPicker.tsx frontend/lib/content.ts frontend/components/studio/PublishForm.tsx "frontend/app/(protected)/studio/publish/fanwork/page.tsx" "frontend/app/(public)/content/[contentId]/page.tsx" "frontend/app/(public)/original/[contentId]/page.tsx" "frontend/app/(public)/original/[contentId]/fanworks/page.tsx" frontend/components/content/ContentDetail.tsx frontend/components/content/ContentSidebar.tsx frontend/messages/zh.json frontend/messages/en.json frontend/tests/studio-publish-fanwork.test.tsx frontend/tests/source-linkage-components.test.tsx frontend/e2e/studio-publish-fanwork.spec.ts screenshots/community-source-picker-desktop.png screenshots/community-source-picker-mobile.png screenshots/community-source-attribution-desktop.png screenshots/community-source-attribution-unavailable.png screenshots/community-related-fanworks-desktop.png screenshots/community-derivatives-mobile.png docs/superpowers/plans/2026-06-30-omnicraft-community-source-linkage.md progress.txt
git commit -m "Community 5: original fanwork source linkage"
```

---

## Plan Self-Check

- [ ] All five new source validation errors are named with exact conditions.
- [ ] `AGENTS.md`, `CLAUDE.md`, and `architecture.md` no longer describe the stale single-source-only model.
- [ ] Historical fanwork rows without sources are not retroactively blocked.
- [ ] Model, migration, input DTO, repository filter, handler response, frontend normalize, publish UI, and detail UI are all included.
- [ ] Related API behavior differs correctly for original source and fanwork source.
- [ ] Source summaries include `id`, `title`, and `zone` before frontend renders clickable attribution.
- [ ] `SourceContentPicker` is canonical and no fanwork-specific picker file is created.
- [ ] Fanwork source search uses `GET /api/v1/contents/search?zone=fanwork&q=<query>&limit=8`.
- [ ] UI labels use "衍生作品" instead of "三创".
- [ ] Source attribution has unavailable-source behavior and does not render for IP-only fanwork.
- [ ] Publish UI prevents both source IDs from being submitted together.
- [ ] Browser verification covers original rows, fanwork source links, derivative rows, and publish validation.
