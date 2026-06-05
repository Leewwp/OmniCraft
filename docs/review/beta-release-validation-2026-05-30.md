# OmniCraft Web Beta Release Validation Report

**Date:** 2026-06-01
**Validator:** Automated agent session
**Base SHA:** 071fd5ec4c85270d37169d6e4740da3cdb8890e8
**Decision:** ✅ PASS — Web Beta release gate satisfied

---

## 1. Prerequisite Task Status

All Web Beta roadmap tasks verified checked `[x]`:

| Task | Title | Status |
|------|-------|--------|
| F-01 | Database schema alignment | ✅ |
| F-02 | Config-driven feature flags | ✅ |
| F-03 | Unified error envelope | ✅ |
| F-04 | Captcha integration (Alibaba Cloud CAPTCHA 2.0) | ✅ |
| F-05 | Chinese search (ILIKE + pg_trgm fallback) | ✅ |
| F-06 | Download API with signed URL | ✅ |
| V-01 | Email verification flow | ✅ |
| V-02 | Password reset flow | ✅ |
| V-03 | Captcha on registration | ✅ |
| V-04 | Terms & privacy acceptance | ✅ |
| V-05 | Admin verification management | ✅ |
| V-06 | Verification rate limiting | ✅ |
| A-01 | Admin user management | ✅ |
| A-02 | Admin content management | ✅ |
| A-03 | Admin audit logging | ✅ |
| A-04 | Admin config with secret masking | ✅ |
| A-05 | Admin feedback management | ✅ |
| G-01 | Agent feature gate | ✅ |
| G-02 | Agent search visibility | ✅ |
| G-03 | Agent chat context & safety | ✅ |
| G-04 | Usage guide panel | ✅ |
| G-05 | Publish assistance & compliance | ✅ |
| D-01 | Remove deploy script endpoint | ✅ |

Desktop tasks D-02 through D-05 remain unchecked; `desktop_deploy_enabled=false` confirmed.

---

## 2. Migration Validation

### 2.1 Empty Database Path

Created disposable `beta_validation` database and applied all 52 migrations in lexical order.

**Result:** All 52 migrations applied successfully.

**Fix applied:** Migrations 041 and 042 used `CREATE EXTENSION IF NOT EXISTS pg_jieba` which fails when the extension is unavailable. Rewrote both to wrap extension creation in `DO $$ EXCEPTION WHEN OTHERS` blocks and use dynamic SQL for the backfill `UPDATE` statements, selecting between `jiebacfg` and `simple` based on `pg_extension` existence.

### 2.2 Upgrade Path

Created `beta_upgrade` database, applied pre-Beta migrations (001-048), then Beta migrations (049-052).

**Result:** Both paths succeed. Verified:
- `pgvector` extension available
- `pg_trgm` indexes created (047)
- `content_search_idx` GIN index (041)
- `users.accepted_terms_version` / `accepted_terms_at` columns (050)
- `feedback_tickets`, `feedback_replies`, `feedback_attachments` tables (051)
- `admin_audit_logs` table (052)

---

## 3. Engineering Gates

| Command | Result |
|---------|--------|
| `go build ./...` | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./...` | ✅ PASS (all packages) |
| `npm run build` | ✅ PASS |
| `npm run lint` | ✅ PASS |

---

## 4. Secret-Leak Scan

Ran pattern scan across `backend/`, `frontend/`, `tauri-client/`:

| Pattern | Findings | Assessment |
|---------|----------|------------|
| `err.Error()` | 16 matches | All in `switch` matching, slog logging, `SanitizeError` filter, or test code. No raw errors returned to clients. ✅ |
| `refresh_token` | 0 in frontend | No refresh tokens in frontend storage or API responses. ✅ |
| `AGENT_HMAC_SECRET` | Only in config.go, .env.example, docs | Not in client code. ✅ |
| `private_key` / `access_key_secret` | Config structs with `json:"-"` tags, test fixtures, docs | Not serialized in API responses. ✅ |
| `console.error` | 3 in frontend error handler | Logs error context, does not display raw errors to users. ✅ |

**Result:** No secret leaks detected. `.env` in `.gitignore`.

---

## 5. API Authorization Matrix

| Endpoint | Expected | Actual | Result |
|----------|----------|--------|--------|
| `GET /api/v1/contents/search?q=test` | 200 | 200 | ✅ |
| `GET /api/v1/admin/users` (anon) | 401 | 401 | ✅ |
| `GET /api/v1/feedback/me` (anon) | 401 | 401 | ✅ |
| `GET /api/v1/notifications` (anon) | 401 | 401 | ✅ |
| `GET /api/v1/admin/config` (anon) | 401 | 401 | ✅ |
| `GET /api/v1/admin/audit-logs` (anon) | 401 | 401 | ✅ |
| `GET /api/v1/categories` | 200 | 200 | ✅ |
| `GET /api/v1/config/public` | 200 | 200 | ✅ |
| `GET /api/v1/contents/1/download` (anon) | 401 | 401 | ✅ |
| `GET /api/v1/rehab/courses` (anon) | 401 | 401 | ✅ |
| `GET /api/v1/agent/script/1` | 404 | 404 | ✅ Removed |
| `POST /api/v1/deploy-grants` | 503 | 503 | ✅ FEATURE_DISABLED |
| `POST /api/v1/payments/create` | 503 | 503 | ✅ FEATURE_DISABLED |

Authenticated tests (Bearer token):

| Endpoint | Result |
|----------|--------|
| `GET /api/v1/auth/me` | ✅ 200 |
| `GET /api/v1/notifications` | ✅ 200 |
| `GET /api/v1/rehab/courses` | ✅ 200 |
| `GET /api/v1/users/me/contents` | ✅ 200 |
| `GET /api/v1/reputation-logs/me` | ✅ 200 |
| `GET /api/v1/users/me/history` | ✅ 200 |
| `GET /api/v1/appeals/me` | ✅ 200 |
| `GET /api/v1/feedback/me` | ✅ 200 |

---

## 6. Verification & Feedback API Tests

- Registration creates user and sends verification email (smtp.mode=logger in dev). ✅
- Login returns JWT access token. ✅
- CSRF token consistency: cookie and body tokens match via Gin context. ✅
- Feedback ticket submission with category/title/description. ✅
- Feedback/me returns user's own tickets. ✅
- Admin feedback routes require admin role. ✅

---

## 7. Browser Tests

Playwright headless tests on `http://localhost:3001`:

| Page | Result |
|------|--------|
| Homepage `/` | ✅ |
| Login `/login` | ✅ |
| Register `/register` | ✅ |
| Search `/search?q=test` | ✅ |
| Feedback `/feedback` | ✅ |
| Terms `/terms` | ✅ |
| Privacy `/privacy` | ✅ |
| Help `/help` | ✅ |

Screenshots saved to `screenshots/r01_*.png`.

---

## 8. Production Configuration

| Check | Result |
|-------|--------|
| CORS `allowed_origins` has explicit origins, no wildcard | ✅ |
| `desktop_deploy_enabled: false` | ✅ |
| `payment_enabled: false` | ✅ |
| `shutdown_timeout: 15` (configurable) | ✅ |
| `report_auto_hide_rate` present in config | ✅ |
| `comment_fold_threshold` present in config | ✅ |
| `download_url_ttl_sec: 300` | ✅ |
| Public config does not expose secrets | ✅ |

---

## 9. Bugs Found and Fixed During Validation

| Bug | File | Fix |
|-----|------|-----|
| pg_jieba extension unavailable crashes migration | `041_content_search_vector.sql`, `042_ips_search_vector.sql` | Wrap in `DO $$ EXCEPTION`, use dynamic SQL for backfill |
| Comment appeal incorrectly restores content status | `appeal_helpers.go` | Remove `comment` branch from `appealTargetUpdates` |
| Search parameter binding has extra tsQuery | `search_repo.go` | Remove duplicate tsQuery from `dataArgs` prefix |
| CSRF cookie and body token mismatch | `csrf.go` | Store token in Gin context, `GetCSRFToken` reads from context first |
| Feedback handler uses wrong context key | `feedback.go` | `"user_id"` → `middleware.UserIDKey` |
| Missing `shutdown_timeout` config | `config.go`, `config.yaml`, `main.go` | Add `ShutdownTimeout` field, read from config in graceful shutdown |
| Missing `deploy-grants` route returning 503 | `routes.go` | Add inline handler returning `503 FEATURE_DISABLED` |
| Missing `payments` route returning 503 | `routes.go` | Add catch-all handler returning `503 FEATURE_DISABLED` |
| CSRF blocks feature-disabled endpoints | `csrf.go` | Add `deploy-grants` and `payments` to `isInternalPath` skip list |

---

## 10. Residual Risks

1. **pg_jieba unavailable**: Chinese full-text search uses `simple` config instead of `jiebacfg`, reducing Chinese tokenization quality. Mitigation: ILIKE + pg_trgm fallback from F-05 provides acceptable Chinese search.
2. **SMTP in development**: `smtp.mode=logger` in dev; production requires real SMTP credentials.
3. **Captcha bypass**: `captcha.provider=bypass` allowed in debug mode; production requires `aliyun_v2`.
4. **Desktop deploy disabled**: D-02 through D-05 not implemented; `desktop_deploy_enabled=false` until R-02 passes.
5. **Payment disabled**: `payment_enabled=false`; all payment endpoints return 503.

---

## 11. Release Decision

**APPROVED for Web Beta release.**

All prerequisite tasks complete, all engineering gates pass, no secret leaks, authorization matrix correct, feature flags properly disable desktop deploy and payment, and residual risks are documented and mitigated.

---

## 12. 2026-06-05 Repair Validation Addendum

The 2026-06-03 review summary superseded the original local R-01 decision with `GO-WITH-BLOCKERS`.
The blocker repair branch `codex/beta/repair-validation-blockers` now includes the code repairs and evidence in
`docs/review/web-beta-repair-validation-2026-06-03.md`.

Additional 2026-06-05 checks:

- `features.desktop_deploy_enabled=false`, `client.download_enabled=false`, and `features.payment_enabled=false` remain unchanged.
- `features.creator_support_enabled=false` is now explicitly declared in `backend/config.yaml`.
- The aggregate repair branch restores the Web Beta review reports, repair plan, and E2E evidence files instead of deleting them.
- `reports.action_taken` and `feedback_tickets_status_check` are present in the local validation database after migration 053.

Commands rerun:

```powershell
cd backend
go test ./config -run TestDefaultConfigDeclaresCreatorSupportDisabled -v
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run build
npm run lint
```

Result: all listed commands passed.

Production deployment remains blocked until real SMTP, Alibaba CAPTCHA/OSS/Green credentials, production PostgreSQL/Redis,
HTTPS certificate, production allowed origins, strong JWT secret, and approved legal version inputs are provided.
Desktop one-click deployment, Tauri client distribution, and payment remain disabled.
