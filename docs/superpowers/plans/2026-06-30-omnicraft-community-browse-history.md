# OmniCraft Community Browse History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将浏览足迹增强为配置驱动的 7 天保留、可筛选分页、可批量删除、可展示失效内容占位的完整体验。

**Architecture:** 后端在查询层和清理任务层同时执行保留期策略；API 在一个 Beta 兼容窗口内同时返回 `items` 和旧 `history` 字段；前端 `/history` 改为筛选芯片、日期范围、批量管理和灰色失效占位卡片。定时清理使用 `time.AfterFunc` 自循环，不改造全局 scheduler 框架。

**Tech Stack:** Go/Gin/GORM/PostgreSQL, Next.js App Router, next-intl, React Testing Library, Playwright.

---

## Cross-Plan Coordination

- Shared-file integration and migration order for the six community plans is: messages-notifications (`057`) -> browse-history (no migration) -> collections (`058`) -> content-series (`059`) -> source-linkage (`060`) -> collaboration-invites (`061`).
- `frontend/app/(protected)/messages/page.tsx`, `frontend/components/social/ChatWindow.tsx`, and `frontend/components/social/ConversationList.tsx` must land in messages-notifications before collaboration-invites extends typed invite cards.
- `frontend/components/content/ContentDetail.tsx` changes must land collections before content-series before source-linkage.
- `frontend/components/studio/PublishForm.tsx` changes must land source-linkage before collaboration-invites; this plan must not edit `PublishForm.tsx`.
- `backend/config/config.go` and `backend/config.yaml` changes from this plan and collaboration-invites must be implemented serially and rebased before verification.
- Before any UI code, grep `design/ui-spec.md` for the exact `## Page:` / `## Component:` sections named by this plan and follow those sections as the visual authority. As of 2026-06-30, `/history`, `ContentCard`, `SkeletonCard`, `ConfirmModal`, `EmptyState`, `LoadingSpinner`, and `Toast` are present; do not rewrite `design/ui-spec.md` unless an implementation-time check proves a required section is absent or stale.
- Expected-result convention: any "Run and confirm red" step expects FAIL for the behavior under test; any "Verify green" / "Run ... tests" step expects PASS. If the observed result differs, stop and update the plan before proceeding.
- Before implementation, run `git status --short`, reserve exact files, and stage only exact touched files. Do not use directory-level staging such as `git add backend`, `git add frontend`, `git add design`, `git add screenshots`, or `git add docs/superpowers/plans`.
- Staging note: the sample `git add` command at the end must be reduced to files actually changed in that implementation. Omit `design/ui-spec.md` when it was only read/verified; omit generated docs such as `architecture.md` unless `doc-validator` changed them during this task.

---

## File Structure

### Backend

- Modify: `backend/config/config.go` - 新增 `BrowseHistoryConfig` 和 `Config.BrowseHistory`，不加入 `SaveOverride`。
- Modify: `backend/config.yaml` - 新增 `browse_history.retention_days` 和 `cleanup_time`。
- Modify: `backend/internal/handler/browse_history.go` - 解析筛选参数、返回兼容 DTO、支持 `{ ids }` 批量删除。
- Modify: `backend/internal/repository/browse_history_repo.go` - 条件查询、保留期过滤、失效内容 DTO、批量删除、过期删除。
- Create: `backend/internal/pkg/scheduler/browse_history_cleanup.go` - Asia/Shanghai 每日清理任务。
- Modify: `backend/cmd/server/main.go` - 启动清理任务。
- Create: `backend/internal/handler/browse_history_test.go`
- Create: `backend/internal/repository/browse_history_repo_test.go`
- Create: `backend/internal/pkg/scheduler/browse_history_cleanup_test.go`

### Frontend

- Modify: `frontend/app/(protected)/history/page.tsx` - 增加筛选、日期、批量选择、失效占位和错误保留数据行为。
- Modify: `frontend/lib/api.ts` - 增加支持 DELETE body 的 helper，保留现有 `api.delete(path)` 兼容。
- Read: `design/ui-spec.md` - 对照 `/history`、`ContentCard`、`SkeletonCard`、`ConfirmModal`、`EmptyState`、`LoadingSpinner`、`Toast`；当前计划不应主动修改 UI spec。
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`
- Create: `frontend/tests/history-page.test.tsx`
- Create: `frontend/e2e/history.spec.ts`

---

## Task 1: Add Config Mapping Tests

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Test: add to an existing config test file or create `backend/config/config_test.go`

- [ ] **Step 1: Write failing config test**

Add a test that loads `backend/config.yaml` and asserts:

```go
if cfg.BrowseHistory.RetentionDays != 7 { t.Fatalf(...) }
if cfg.BrowseHistory.CleanupTime != "03:00" { t.Fatalf(...) }
```

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./config -run TestBrowseHistoryConfig -v
```

Expected: compile failure or zero-value failure because `BrowseHistoryConfig` does not exist yet.

- [ ] **Step 3: Implement config structs**

Add:

```go
type BrowseHistoryConfig struct {
    RetentionDays int    `mapstructure:"retention_days"`
    CleanupTime   string `mapstructure:"cleanup_time"`
}
```

Mount it on `Config`:

```go
BrowseHistory BrowseHistoryConfig `mapstructure:"browse_history"`
```

Do not add this config to `SaveOverride`, admin config DTOs, or public config.

- [ ] **Step 4: Add YAML defaults**

Add:

```yaml
browse_history:
  retention_days: 7
  cleanup_time: "03:00"
```

- [ ] **Step 5: Verify green**

Run:

```powershell
cd backend
go test ./config -run TestBrowseHistoryConfig -v
```

---

## Task 2: Implement Repository Query And Delete Semantics

**Files:**
- Modify: `backend/internal/repository/browse_history_repo.go`
- Test: `backend/internal/repository/browse_history_repo_test.go`

- [ ] **Step 1: Write failing repository tests**

Cover:

```go
func TestBrowseHistoryListAppliesRetentionWindow(t *testing.T) {}
func TestBrowseHistoryListFiltersByContentTypeAndDateRange(t *testing.T) {}
func TestBrowseHistoryListReturnsNullContentForUnavailableContent(t *testing.T) {}
func TestBrowseHistoryListCountMatchesFilteredRows(t *testing.T) {}
func TestBrowseHistoryDeleteByIDsScopesToCurrentUserAndLimit(t *testing.T) {}
func TestBrowseHistoryDeleteExpiredUsesRetentionDays(t *testing.T) {}
```

Unavailable content means `content_items.status != 'published'` or `content_items.deleted_at IS NOT NULL`.

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/repository -run TestBrowseHistory -v
```

- [ ] **Step 3: Add query options and DTO**

Define focused repository types:

```go
type BrowseHistoryListOptions struct {
    UserID        int64
    ContentType   string
    StartDate     *time.Time
    EndDate       *time.Time
    RetentionDays int
    Page          int
    PageSize      int
}

type BrowseHistoryItemDTO struct {
    ID          int64              `json:"id"`
    Content     *model.ContentItem  `json:"content"`
    ContentItem *model.ContentItem  `json:"content_item"`
    ViewedAt    time.Time          `json:"viewed_at"`
}
```

`ContentItem` is a temporary compatibility alias for the current frontend's `history[*].content_item` shape. Keep it equal to `Content`, including `null` for unavailable content, during the Beta compatibility window.

- [ ] **Step 4: Implement filtered list**

Rules:

- always `WHERE browse_history.user_id = ?`
- always `WHERE viewed_at >= now - retentionDays`
- when `content_type` is present, the query joins `content_items`
- when date range query params are present, the handler parses them and passes concrete times to repo
- response item has `Content = nil` for unpublished or soft-deleted content
- response item also has `ContentItem = Content` for legacy `content_item` consumers
- `total` counts only retained, filtered history rows

- [ ] **Step 5: Implement delete methods**

Add:

```go
func (r *BrowseHistoryRepository) DeleteByUserAndIDs(userID int64, ids []int64) error
func (r *BrowseHistoryRepository) DeleteExpired(retentionDays int) (int64, error)
```

`DeleteByUserAndIDs` must include `WHERE user_id = ? AND id IN ?`.

- [ ] **Step 6: Verify repository green**

Run:

```powershell
cd backend
go test ./internal/repository -run TestBrowseHistory -v
```

---

## Task 3: Implement Handler Contract

**Files:**
- Modify: `backend/internal/handler/browse_history.go`
- Test: `backend/internal/handler/browse_history_test.go`

- [ ] **Step 1: Add failing handler tests**

Cover:

- `GET /api/v1/users/me/history` returns `items`, `history`, `total`, `page`, `page_size`
- `items` and `history` are identical arrays during compatibility window
- every returned item includes both `content` and legacy `content_item`; both are `null` for unavailable content
- response includes `retention_days` from `config.yaml > browse_history.retention_days`
- `limit` aliases `page_size`
- `page_size` wins when both `limit` and `page_size` exist
- invalid `content_type` returns `400 INVALID_CONTENT_TYPE`
- invalid date returns `400 INVALID_DATE`
- `DELETE {"ids":[...]}` deletes selected rows only
- `DELETE {"ids":[]}` clears all rows
- `DELETE` with no body clears all rows
- more than 100 IDs returns `400 TOO_MANY_IDS`

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/handler -run TestBrowseHistory -v
```

- [ ] **Step 3: Parse and validate query parameters**

Accepted `content_type` values:

```text
image, article, video, audio, template, sheet_music, mod, prompt, other
```

Clamp pagination:

- default `page=1`
- default `page_size=20`
- max `page_size=100`

- [ ] **Step 4: Implement compatible response**

Return:

```json
{
  "items": [],
  "history": [],
  "total": 0,
  "page": 1,
  "page_size": 20,
  "retention_days": 7
}
```

Do not remove `history` or per-item `content_item` in this implementation. `retention_days` must come from config, not a hardcoded handler constant.

- [ ] **Step 5: Implement DELETE body handling**

Use `ShouldBindJSON` carefully so empty/no body does not become a 400. Treat no body and `ids: []` as clear-all. Treat `ids` length greater than 100 as a validation error.

- [ ] **Step 6: Verify handler green**

Run:

```powershell
cd backend
go test ./internal/handler -run TestBrowseHistory -v
```

---

## Task 4: Add Daily Cleanup Scheduler

**Files:**
- Create: `backend/internal/pkg/scheduler/browse_history_cleanup.go`
- Modify: `backend/cmd/server/main.go`
- Test: `backend/internal/pkg/scheduler/browse_history_cleanup_test.go`

- [ ] **Step 1: Add failing scheduler tests**

Cover:

```go
func TestBrowseHistoryCleanupNextRunUsesAsiaShanghai(t *testing.T) {}
func TestBrowseHistoryCleanupSchedulesTomorrowWhenTodayTimePassed(t *testing.T) {}
func TestBrowseHistoryCleanupDeletesExpiredRowsAndReschedules(t *testing.T) {}
func TestBrowseHistoryCleanupInvalidConfigFallsBackAndLogs(t *testing.T) {}
```

- [ ] **Step 2: Run and confirm red**

Run:

```powershell
cd backend
go test ./internal/pkg/scheduler -run TestBrowseHistoryCleanup -v
```

- [ ] **Step 3: Implement scheduler**

Use the spec's option B:

- load `Asia/Shanghai`
- parse `cleanup_time` as `HH:MM`
- compute next run from current time in that location
- call `time.AfterFunc(delay, func(){ cleanup(); scheduleNext() })`
- include `Stop()` to stop the timer

The cleanup function calls repository `DeleteExpired(retentionDays)`.

- [ ] **Step 4: Wire in main**

In `backend/cmd/server/main.go`, after existing scheduler starts:

```go
scheduler.NewBrowseHistoryCleanup(db, &cfg.BrowseHistory).Start()
```

- [ ] **Step 5: Verify scheduler green**

Run:

```powershell
cd backend
go test ./internal/pkg/scheduler -run TestBrowseHistoryCleanup -v
```

---

## Task 5: Refactor History Page UI

**Files:**
- Modify: `frontend/app/(protected)/history/page.tsx`
- Modify: `frontend/lib/api.ts`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `frontend/tests/history-page.test.tsx`

- [ ] **Step 1: Confirm UI spec**

Run:

```powershell
rg -n "## Page: /history" design/ui-spec.md
```

Expected: `/history` exists and covers filter chips, date range, batch delete, unavailable item, loading, empty, error, a11y, i18n, and screenshot checkpoints. If any of those states are missing in a future branch, stop and repair UI spec in an explicitly scoped docs/design step before changing UI code.

- [ ] **Step 2: Add failing frontend tests**

Assert:

- new frontend uses `items` first
- old `{ history: [...] }` still renders
- old `{ history: [{ content_item: ... }] }` still renders during compatibility window
- retention copy reads `retention_days` from the API response or falls back to a non-numeric localized phrase when metadata is absent
- content type chip calls API with `content_type`
- date range calls API with `start_date` and `end_date`
- invalid `content: null` renders gray card and does not navigate
- selecting two cards and deleting calls DELETE with `{ ids: [...] }`
- network error shows toast and keeps last successful data

- [ ] **Step 3: Add DELETE body helper**

Because current `api.delete(path)` accepts no body, add one of:

```ts
api.deleteWithBody(path, body)
```

or a generic request helper. Do not break existing `api.delete(path)` callers.

- [ ] **Step 4: Implement page UI**

UI states:

- loading: skeleton x5
- empty: icon/illustration, no records copy, home button
- error: toast only, keep prior data if any
- loaded: grouped cards plus localized footer copy driven by API `retention_days`; do not hardcode "7 days" in component code or translation values
- batch mode: stable checkboxes and top "删除选中(N)" action

No hardcoded UI strings; add keys under `history.*`.

- [ ] **Step 5: Run frontend tests**

Run:

```powershell
cd frontend
npm run test -- tests/history-page.test.tsx
```

---

## Task 6: Verification And Documentation Sync

**Files:**
- Modify if generated: `architecture.md`
- Screenshot outputs: `screenshots/community-browse-history-*.png`

- [ ] **Step 1: Run backend gates**

Run:

```powershell
cd backend
go test ./internal/repository -run TestBrowseHistory -v
go test ./internal/handler -run TestBrowseHistory -v
go test ./internal/pkg/scheduler -run TestBrowseHistoryCleanup -v
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
```

- [ ] **Step 3: Run doc-validator**

Because this plan changes `backend/config/config.go`, run:

```powershell
cd tools/doc-validator
go run . --fix
```

- [ ] **Step 4: Browser verification**

Use Playwright:

1. Log in and open several published content pages.
2. Visit `/history`; verify cards render.
3. Filter by `article` and verify network query includes `content_type=article`.
4. Select date range and verify query includes `start_date` / `end_date`.
5. Toggle batch mode, delete selected records, verify only selected records disappear.
6. Clear all and verify empty state.
7. Seed one unpublished/deleted content history record; verify gray non-clickable placeholder.
8. Save screenshots under `screenshots/`.

- [ ] **Step 5: Commit when implementing**

```powershell
git add -- backend/config/config.go backend/config.yaml backend/internal/handler/browse_history.go backend/internal/handler/browse_history_test.go backend/internal/repository/browse_history_repo.go backend/internal/repository/browse_history_repo_test.go backend/internal/pkg/scheduler/browse_history_cleanup.go backend/internal/pkg/scheduler/browse_history_cleanup_test.go backend/cmd/server/main.go "frontend/app/(protected)/history/page.tsx" frontend/lib/api.ts frontend/messages/zh.json frontend/messages/en.json frontend/tests/history-page.test.tsx frontend/e2e/history.spec.ts design/ui-spec.md screenshots/community-browse-history-desktop.png screenshots/community-browse-history-mobile.png architecture.md docs/superpowers/plans/2026-06-30-omnicraft-community-browse-history.md progress.txt
git commit -m "Community 2: browse history enhancement"
```

---

## Plan Self-Check

- [ ] Retention period is config-driven; no SQL or frontend hardcodes the number 7 except default config and translated explanatory copy fed by config where possible.
- [ ] Query layer and scheduler layer both enforce retention.
- [ ] API response preserves old `history` and adds new `items`.
- [ ] API response preserves legacy per-item `content_item` until old history consumers are migrated.
- [ ] API response exposes config-derived `retention_days` for UI copy.
- [ ] Invalid content returns `content: null` instead of deleting the history row.
- [ ] DELETE semantics explicitly distinguish selected IDs, empty IDs, no body, and too many IDs.
- [ ] Scheduler uses `time.AfterFunc` self-rescheduling and Asia/Shanghai.
- [ ] Frontend plan includes API helper work for DELETE bodies.
- [ ] `doc-validator` is required because `config.go` changes.
