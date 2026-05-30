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
- Modify: `backend/internal/handler/admin.go`, `backend/internal/handler/routes.go`.
- Modify: `backend/internal/container/container.go`.
- Add focused tests in `backend/internal/service` and `backend/internal/handler`.

### Frontend

- Modify: `frontend/app/admin/layout.tsx`.
- Modify: `frontend/app/admin/page.tsx`.
- Create: `frontend/app/admin/dashboard/page.tsx`.
- Create: `frontend/app/admin/reports/page.tsx`.
- Create: `frontend/app/admin/feedback/page.tsx`.
- Create: `frontend/app/admin/queue/page.tsx`.
- Create: `frontend/app/admin/audit-logs/page.tsx`.
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

- [ ] **Step 1: Add migration**

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

- [ ] **Step 2: Write failing allowlist tests**

Cover stripping `token`, `cookie`, `password`, `api_key`, `secret`, `grant`, local paths and request headers from metadata.

- [ ] **Step 3: Implement service**

```go
func (s *AdminAuditService) Record(ctx context.Context, entry RecordAdminAuditInput) error
func (s *AdminAuditService) List(ctx context.Context, filter AdminAuditFilter) ([]model.AdminAuditLog, int64, error)
```

Rules:

- Append only. Do not expose update/delete APIs.
- Store trace ID from request context.
- Filter metadata using a per-action allowlist.
- Log internal record failure with `slog`; decide per mutation whether audit failure blocks the action. For security-sensitive changes, fail closed.

- [ ] **Step 4: Run tests**

```powershell
cd backend
go test ./internal/service -run TestAdminAudit -v
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 5: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta A-01: add append-only admin audit logs - completed"
```

## Task A-02: Audit Existing Sensitive Admin Mutations

**Files:**
- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/admin_audit_test.go`

- [ ] **Step 1: Write failing handler tests**

Verify audit rows are created for:

- IP approve/reject.
- Content ban/restore.
- User ban/unban.
- Appeal resolution.
- Report resolution.
- Runtime config patch.
- Category create/update/delete/reorder.
- LLM config create/update/delete/activate/test.
- Future feedback resolution.

- [ ] **Step 2: Add a handler helper**

```go
func (h *AdminHandler) auditOrFail(c *gin.Context, action, targetType, targetID string, metadata map[string]any) bool
```

The helper writes a sanitized entry and returns a safe 500 response if required audit persistence fails.

- [ ] **Step 3: Keep admin config masked**

Confirm `GET /api/v1/admin/config` exposes booleans or masks for credentials. Do not return complete secrets and do not make frontend rendering depend on them.

- [ ] **Step 4: Run checks**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 5: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta A-02: audit sensitive admin mutations - completed"
```

## Task A-03: Add Admin Dashboard And Reports Pages

**Files:**
- Modify: `frontend/app/admin/page.tsx`
- Create: `frontend/app/admin/dashboard/page.tsx`
- Create: `frontend/app/admin/reports/page.tsx`
- Create: `frontend/components/admin/AdminMetricCard.tsx`
- Create: `frontend/components/admin/AdminFilterBar.tsx`
- Modify: `frontend/app/admin/layout.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Inspect visual authority**

```powershell
rg -n "## Page: /admin|## Component:" design/ui-spec.md
```

Reuse existing admin page density and no-shadow design. New admin routes without dedicated specs follow existing `/admin/contents` patterns.

- [ ] **Step 2: Redirect admin index**

Make `/admin` redirect to `/admin/dashboard`.

- [ ] **Step 3: Implement dashboard**

Use existing stats APIs first. Show cards for pending reports, pending content reviews, pending appeals, queue health and open feedback once available. Missing optional metrics render an explicit unavailable state, not fake numbers.

- [ ] **Step 4: Implement reports page**

Use:

```text
GET   /api/v1/admin/reports
GET   /api/v1/admin/reports/stats
PATCH /api/v1/admin/reports/:id
```

Support status/type filters, report detail, resolve action and empty states.

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
- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/admin_feedback_test.go`
- Create: `frontend/app/admin/feedback/page.tsx`
- Create: `frontend/app/admin/queue/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Add feedback admin API**

Mount:

```text
GET   /api/v1/admin/feedback
GET   /api/v1/admin/feedback/:id
PATCH /api/v1/admin/feedback/:id
```

Allow status, category and assignment filters. PATCH can assign, reply or close. Audit every PATCH.

- [ ] **Step 2: Implement feedback page**

Render ticket filters, detail, sanitized diagnostic summary, screenshot preview, reply, assignment and close action.

- [ ] **Step 3: Implement read-only queue page**

Use existing:

```text
GET /api/v1/admin/queue/stats
GET /api/v1/admin/queue/dlq
```

Render health and DLQ entries. Do not add replay controls in Beta.

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

Use MCP Playwright to resolve feedback and inspect queue state. Save screenshots under `screenshots/beta-a04-*`.

- [ ] **Step 5: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta A-04: add admin feedback and queue pages - completed"
```

## Task A-05: Add Audit Page And Complete Navigation

**Files:**
- Create: `backend/internal/handler/admin_audit.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/admin_audit_list_test.go`
- Create: `frontend/app/admin/audit-logs/page.tsx`
- Modify: `frontend/app/admin/layout.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Add list API**

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
