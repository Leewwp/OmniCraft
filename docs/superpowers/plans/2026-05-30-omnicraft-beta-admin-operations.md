# OmniCraft Admin Operations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give operators a complete, auditable Beta control surface for reports, feedback, queues and existing moderation actions.

**Architecture:** Add an append-only audit module and wrap sensitive admin mutations before building UI. Reuse existing reports and queue APIs, add feedback admin APIs, and keep queue inspection read-only during Beta. Admin configuration remains an allowlisted masked view.

**Tech Stack:** Go/Gin/GORM/PostgreSQL, Next.js App Router, `next-intl`, MCP Playwright.

---

## File Structure

### Backend

- Create: `backend/migrations/052_admin_audit_logs.sql`.
- Create: `backend/internal/model/admin_audit.go`.
- Create: `backend/internal/repository/admin_audit_repo.go`.
- Create: `backend/internal/service/admin_audit_service.go`.
- Create: `backend/internal/handler/admin_audit.go`.
- Modify: `backend/internal/handler/admin.go`, `backend/internal/handler/category.go`, `backend/internal/handler/judge.go`, `backend/internal/handler/routes.go`.
- Modify: `backend/internal/container/container.go`.
- Add focused tests in `backend/internal/service` and `backend/internal/handler`.

### Frontend

- Move: `frontend/app/admin/` to `frontend/app/(protected)/admin/` before adding new Admin pages. The URL remains `/admin/*`; the route-group move makes the existing shared protected layout authoritative before the Admin role layout runs.
- Modify: `frontend/app/(protected)/admin/layout.tsx`.
- Modify: `frontend/app/(protected)/admin/page.tsx`.
- Create: `frontend/app/(protected)/admin/dashboard/page.tsx`.
- Create: `frontend/app/(protected)/admin/reports/page.tsx`.
- Create: `frontend/app/(protected)/admin/feedback/page.tsx`.
- Create: `frontend/app/(protected)/admin/queue/page.tsx`.
- Create: `frontend/app/(protected)/admin/audit-logs/page.tsx`.
- Create: `frontend/components/admin/AdminMetricCard.tsx`.
- Create: `frontend/components/admin/AdminFilterBar.tsx`.
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`.

## Task A-01: Add Append-Only Admin Audit Logs

**Files:**
- Create: `backend/migrations/052_admin_audit_logs.sql`
- Create: `backend/internal/model/admin_audit.go`
- Create: `backend/internal/repository/admin_audit_repo.go`
- Create: `backend/internal/service/admin_audit_service.go`
- Create: `backend/internal/service/admin_audit_service_test.go`
- Modify: `backend/internal/container/container.go`

- [x] **Step 1: Add migration**

```sql
CREATE TABLE admin_audit_logs (
  id BIGSERIAL PRIMARY KEY,
  admin_user_id BIGINT NOT NULL REFERENCES users(id),
  action VARCHAR(96) NOT NULL,
  target_type VARCHAR(48) NOT NULL,
  target_id VARCHAR(96),
  trace_id VARCHAR(96),
  metadata JSONB NOT NULL DEFAULT '{}',
  result VARCHAR(24) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_admin_audit_logs_created_at ON admin_audit_logs(created_at DESC);
CREATE INDEX idx_admin_audit_logs_action ON admin_audit_logs(action);
```

- [x] **Step 2: Write failing allowlist tests**

Cover stripping `token`, `cookie`, `password`, `api_key`, `secret`, `grant`, local paths and request headers from metadata.

- [x] **Step 3: Implement service**

```go
func (s *AdminAuditService) Record(ctx context.Context, entry RecordAdminAuditInput) error
func (s *AdminAuditService) RecordTx(ctx context.Context, tx *gorm.DB, entry RecordAdminAuditInput) error
func (s *AdminAuditService) List(ctx context.Context, filter AdminAuditFilter) ([]model.AdminAuditLog, int64, error)
```

Rules:

- Append only. Do not expose update/delete APIs.
- Store trace ID from request context.
- Filter metadata using a per-action allowlist. The initial allowlist is defined in `admin_audit_service.go`:

```go
var auditMetadataAllowlist = map[string][]string{
    "content_ban":     {"content_id", "reason", "author_id"},
    "content_restore": {"content_id", "reason"},
    "user_ban":        {"target_user_id", "reason"},
    "user_unban":      {"target_user_id", "reason"},
    "ip_approve":      {"ip_id", "decision"},
    "ip_reject":       {"ip_id", "decision"},
    "appeal_resolve":  {"appeal_id", "decision", "reason"},
    "report_resolve":  {"report_id", "decision", "reason"},
    "config_patch":    {"field", "old_value_masked", "new_value_masked"},
    "category_create": {"name", "slug", "display_order"},
    "category_update": {"category_id", "name", "slug", "display_order"},
    "category_delete": {"category_id", "name"},
    "category_reorder": {"order"},
    "llm_config_create":      {"provider", "model"},
    "llm_config_update":      {"config_id", "provider", "model"},
    "llm_config_delete":      {"config_id", "provider", "model"},
    "llm_config_activate":    {"config_id"},
    "llm_config_test":        {"config_id"},
    "judge_question_create":  {"question_id", "content_type"},
    "feedback_reply":         {"ticket_id", "is_internal_note"},
    "feedback_close":         {"ticket_id", "reason"},
    "feedback_reopen":        {"ticket_id"},
    "feedback_priority":      {"ticket_id", "priority"},
    "feedback_assign":        {"ticket_id", "assignee_admin_id"},
}
```

Any metadata key not in the action's allowlist is silently stripped before persistence. Never store raw `token`, `cookie`, `password`, `api_key`, `secret`, `grant`, local paths or request headers.
- Log internal record failure with `slog`.
- For every local-database Beta-sensitive mutation listed in A-02, execute the domain mutation and success audit row in one database transaction. If the audit insert fails, roll back and return a safe `500 AUDIT_WRITE_FAILED`. External side effects (OSS file operations, notification dispatch, email sending) must be executed **after** `tx.Commit()` succeeds, never inside the transaction. If an external side effect fails after commit, log the failure but do not roll back the committed audit row.
- For external checks such as LLM connection tests, record sanitized success/failure after the check. If audit persistence is unavailable, return `500 AUDIT_WRITE_FAILED` and do not report the external operation as an audited success.
- Record rejected or failed attempts as best-effort `result=failed` audit rows with a sanitized reason code. Never include raw provider or database errors in metadata.

- [x] **Step 4: Run tests**

```powershell
cd backend
go test ./internal/service -run TestAdminAudit -v
go test ./...
go vet ./...
go build ./...
```

- [x] **Step 5: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta A-01: add append-only admin audit logs - completed"
```

## Task A-02: Audit Existing Sensitive Admin Mutations

**Files:**
- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/category.go`
- Modify: `backend/internal/handler/judge.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/repository/category_repo.go`
- Modify: `backend/internal/service/category_service.go`
- Create: `backend/internal/handler/admin_audit_test.go`

- [x] **Step 1: Write failing handler tests**

Verify audit rows are created for:

- IP approve/reject.
- Content ban/restore.
- User ban/unban.
- Appeal resolution.
- Report resolution.
- Runtime config patch.
- Category create/update/delete/reorder.
- LLM config create/update/delete/activate/test.
- Judge question creation.
- Future feedback resolution.

- [x] **Step 2: Add a handler helper**

```go
func (h *AdminHandler) auditOrFail(c *gin.Context, action, targetType, targetID string, metadata map[string]any) bool
```

The helper writes a sanitized entry. Mutation handlers must use `RecordTx` inside a transaction-aware service method so a successful sensitive mutation cannot be committed without its success audit row. Do not bolt on a non-transactional audit call after the handler has already committed the domain write.

Wire one `AdminAuditService` from `ServiceContainer` into `AdminHandler`, `CategoryHandler` and `JudgeHandler`. Add transaction-aware repository/service entrypoints where existing handlers delegate writes, especially category mutations. Refactor `category_service.go` to accept an optional `*gorm.DB` transaction parameter so the audit service can write the audit row within the same transaction as the category mutation. If the transaction parameter is nil, the service creates its own `db` session as before. Do not instantiate an unrelated audit service per handler.

- [x] **Step 3: Keep admin config masked**

Confirm `GET /api/v1/admin/config` exposes booleans or masks for credentials. Do not return complete secrets and do not make frontend rendering depend on them.

- [x] **Step 4: Run checks**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

- [x] **Step 5: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta A-02: audit sensitive admin mutations - completed"
```

## Task A-03: Add Admin Dashboard And Reports Pages

**Execution prerequisite:** Complete A-04 first. The dashboard consumes the Admin feedback API added by A-04; dependencies take precedence over numeric task order.

**Files:**
- Modify: `frontend/app/(protected)/admin/page.tsx`
- Create: `frontend/app/(protected)/admin/dashboard/page.tsx`
- Create: `frontend/app/(protected)/admin/reports/page.tsx`
- Create: `frontend/components/admin/AdminMetricCard.tsx`
- Create: `frontend/components/admin/AdminFilterBar.tsx`
- Modify: `frontend/app/(protected)/admin/layout.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Inspect visual authority**

```powershell
rg -n "## Page: /admin|## Component:" design/ui-spec.md
```

Reuse existing admin page density and no-shadow design. New admin routes without dedicated specs follow existing `/admin/contents` patterns.

- [ ] **Step 2: Confirm protected admin routing and redirect admin index**

`A-04` moves the existing Admin route subtree into `frontend/app/(protected)/admin/`, preserving the `/admin/*` URLs. Confirm the moved routes still use the nested Admin layout role check after the shared protected layout handles unauthenticated and banned users. Make `/admin` redirect to `/admin/dashboard`.

- [ ] **Step 3: Implement dashboard**

Use existing stats APIs first. The expected API sources for dashboard metrics are:

| Metric | API Source |
|---|---|
| Pending reports count | `GET /api/v1/admin/reports/stats` |
| Queue depth / DLQ health | `GET /api/v1/admin/queue/stats` |
| Open feedback count | `GET /api/v1/admin/feedback?status=open` (returns total count in response metadata) |
| Pending appeals | `GET /api/v1/admin/appeals?status=pending` (existing endpoint) |
| Recent sensitive admin operations | `GET /api/v1/admin/audit-logs` (added in A-05, may be unavailable during A-03) |

A-04 owns the Admin feedback listing API consumed by the open-feedback metric. If another required API is missing, add it in A-03 rather than silently omitting the metric. Missing optional metrics render an explicit unavailable state, not fake numbers.

- [ ] **Step 4: Implement reports page**

Use:

```text
GET   /api/v1/admin/reports
GET   /api/v1/admin/reports/stats
PATCH /api/v1/admin/reports/:id
```

Support status/type filters, report detail, resolve action and empty states. Report detail must show target, reporter, evidence/reason, current state and resolution explanation. PATCH requires an explicit resolution status and action/explanation; do not permit an empty resolve action.

- [ ] **Step 5: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright as admin to visit dashboard and resolve a report. Verify normal users cannot access the pages. Save screenshots under `screenshots/beta-a03-*`.

- [ ] **Step 6: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta A-03: add admin dashboard and reports pages - completed"
```

## Task A-04: Add Admin Feedback And Queue Pages

**Files:**
- Move: `frontend/app/admin/` to `frontend/app/(protected)/admin/`
- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/repository/feedback_repo.go`
- Modify: `backend/internal/service/feedback_service.go`
- Create: `backend/internal/handler/admin_feedback_test.go`
- Create: `frontend/app/(protected)/admin/feedback/page.tsx`
- Create: `frontend/app/(protected)/admin/queue/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Move the existing Admin subtree below the protected route group**

Move `frontend/app/admin/*` to `frontend/app/(protected)/admin/*` without changing public URLs. Keep the nested Admin layout role check in `frontend/app/(protected)/admin/layout.tsx`, but rely on the parent `(protected)` layout for the shared login redirect and banned-account EmptyState.

- [ ] **Step 2: Add feedback admin API**

Before editing Admin UI, run:

```powershell
rg -n "## Page: /admin|## Component: AdminFilterBar|## Component: AdminMetricCard" design/ui-spec.md
```

New Admin pages without dedicated specs follow existing `/admin/contents` density and `design/design-system.md`.

Mount:

```text
GET   /api/v1/admin/feedback
GET   /api/v1/admin/feedback/:id
PATCH /api/v1/admin/feedback/:id
POST  /api/v1/admin/feedback/:id/replies
```

Allow status, category, priority and assignment filters. PATCH changes assignment, priority or status. POST creates a user-visible reply or `is_internal_note=true` note. Audit every PATCH and POST. Closing a ticket sets `resolved_at`; reopening clears it. User-visible reply/close triggers the V-05 notification/mail rules.

- [ ] **Step 3: Implement feedback page**

Render ticket filters, detail, sanitized diagnostic summary, screenshot preview, reply, internal note, priority, assignment and close action. Internal notes must never appear in user-facing ticket detail.

- [ ] **Step 4: Implement read-only queue page**

Use existing:

```text
GET /api/v1/admin/queue/stats
GET /api/v1/admin/queue/dlq
```

Render topic, queue depth, lag, failure count, recent DLQ entries and trace ID. Do not add replay controls in Beta.

- [ ] **Step 5: Run checks and browser-test**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright to resolve feedback and inspect queue state. Save screenshots under `screenshots/beta-a04-*`.

- [ ] **Step 6: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta A-04: add admin feedback and queue pages - completed"
```

## Task A-05: Add Audit Page And Complete Navigation

**Files:**
- Create: `backend/internal/handler/admin_audit.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/admin_audit_list_test.go`
- Create: `frontend/app/(protected)/admin/audit-logs/page.tsx`
- Modify: `frontend/app/(protected)/admin/layout.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Add list API**

Before editing Admin navigation UI, run:

```powershell
rg -n "## Page: /admin|## Component: Sidebar|## Component: AdminFilterBar" design/ui-spec.md
```

Mount:

```text
GET /api/v1/admin/audit-logs?action=&admin_user_id=&from=&to=&page=&page_size=
```

Return sanitized metadata only.

- [ ] **Step 2: Add page and navigation**

Admin sidebar must reach dashboard, reports, feedback, queue and audit logs in addition to existing pages. Add mobile navigation entries.

- [ ] **Step 3: Verify access boundaries**

Admin routes require runtime admin role from `AuthRequired` plus `AdminRequired`; do not rely on JWT claims or frontend redirects.

- [ ] **Step 4: Run checks and browser-test**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright to navigate every admin sidebar entry and inspect one audit record. Save `screenshots/beta-a05-admin-nav.png`.

- [ ] **Step 5: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta A-05: complete admin navigation and audit page - completed"
```
