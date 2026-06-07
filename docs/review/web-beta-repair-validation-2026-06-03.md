# Web Beta Repair Validation - 2026-06-03

## Baseline

- Validation date: 2026-06-04
- Branch: `codex/beta/repair-validation-blockers`
- Repair base includes Task 1 through Task 6 repair commits plus validation blocker fix:
  - `5cda58e` Beta repair: harden auth session and release gates
  - `bd3e1c8` Beta repair: fix verification token lifecycle
  - `4fb78c6` Beta repair: restore feedback and report workflows
  - `84f4c00` Beta repair: unify content visibility and downloads
  - `508138a` Beta repair: validate agent suggestions
  - `dbbd55f` Beta repair: make admin audit mutations transactional
  - `e9705a7` Beta repair: fix validation blocker search suggestions
- Local dependencies:
  - `omnicraft-postgres` healthy on `localhost:5432`
  - `omnicraft-redis` healthy on `localhost:6379`
- Local migration `backend/migrations/055_web_beta_review_repairs.sql` was applied before validation. It is idempotent and confirmed `reports.action_taken` and `feedback_tickets_status_check` exist.

## Repairs Covered

- Auth/session/captcha:
  - Register, forgot-password, and resend-verification require captcha.
  - Refresh ignores JSON body refresh tokens and uses refresh cookie only.
  - Register -> pending -> local email verification -> login -> refresh -> logout path works in local bypass captcha mode.
- Verification lifecycle:
  - Covered by focused and full backend gates from the repair branch.
- Feedback and admin report:
  - Anonymous feedback submission works with `category=web_bug`, `description`, `contact_email`, and local bypass captcha token.
  - Admin report resolve updates report state and writes an admin audit row.
- Search/download:
  - Search suggestions return 200 and no longer query nonexistent `tags.status`.
  - Content search returns 200.
  - Download rejects unauthenticated access.
- Agent/release flags:
  - Removed desktop script endpoint remains 404.
  - Deploy grants return 503 while desktop deploy is disabled.
  - Payments return 503 while payments are disabled.
  - Public config confirms `desktop_deploy_enabled=false` and `client.download_enabled=false`.
  - `web_agent_enabled=false` conversation endpoint behavior is covered by focused handler test.
- Browser evidence:
  - Register, login, feedback, admin reports, search, and content detail pages rendered under Playwright.

## Commands Run

Backend gates:

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
go test ./internal/handler -run TestAgentConversationEndpointsRespectFeatureFlag -v
```

Frontend gates:

```powershell
cd frontend
npm run build
npm run lint
```

Note: the first `npm run lint` attempt ran before `.next/types` existed and failed with missing generated Next type files. `npm run build` generated `.next/types`, and the fresh rerun of `npm run lint` passed.

Local validation setup:

```powershell
Get-Content backend\migrations\055_web_beta_review_repairs.sql | docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft
cd backend && go run cmd/seed_admin/main.go
cd backend && go run cmd/server/main.go
cd frontend && npm run dev
python task7_validation_runner.py
```

The temporary runner was removed after generating evidence.

## Browser/API Evidence

Structured evidence:

- `screenshots/review-web-beta/repair-validation/task7-validation-results.json`

API results recorded in the JSON evidence:

| Check | Result |
| --- | --- |
| Captcha required on register | PASS, 400 `CAPTCHA_REQUIRED` |
| Captcha required on forgot password | PASS, 400 `CAPTCHA_REQUIRED` |
| Captcha required on resend verification | PASS, 400 `CAPTCHA_REQUIRED` |
| Refresh ignores JSON body token | PASS, 401 `INVALID_TOKEN` |
| Register creates pending verification account | PASS, 202 |
| Login after local verification | PASS, 200 |
| Refresh uses cookie session | PASS, 200 |
| Logout succeeds | PASS, 200 |
| Anonymous feedback submit | PASS, 201 |
| Admin login | PASS, 200 |
| Admin report resolve creates audit row | PASS, 200 and audit count 1 |
| Search suggestions returns 200 | PASS, 200 |
| Content search returns 200 | PASS, 200 |
| Download rejects unauthenticated | PASS, 401 `UNAUTHORIZED` |
| Agent script endpoint remains removed | PASS, 404 |
| Desktop deploy grants disabled | PASS, 503 `FEATURE_DISABLED` |
| Payments disabled | PASS, 503 `FEATURE_DISABLED` |
| Desktop/client download flags disabled | PASS, public config flags false |

Screenshots:

- `screenshots/review-web-beta/repair-validation/01-auth-register.png`
- `screenshots/review-web-beta/repair-validation/02-auth-login-refresh.png`
- `screenshots/review-web-beta/repair-validation/03-feedback-anonymous.png`
- `screenshots/review-web-beta/repair-validation/04-admin-report-resolve.png`
- `screenshots/review-web-beta/repair-validation/05-search-visibility.png`
- `screenshots/review-web-beta/repair-validation/06-download-authorized-cta.png`

## Remaining Production-Only Blockers

These are not local code failures, but still block production Web Beta deployment:

- SMTP real provider and credentials.
- Alibaba Cloud CAPTCHA 2.0 credentials.
- Alibaba Cloud OSS bucket and credentials.
- Production PostgreSQL DSN with SSL.
- Production Redis auth/TLS configuration if required by deployment.
- HTTPS certificate and deployed API origin.
- `security.allowed_origins` set to the production app origin.
- Strong `JWT_SECRET`.
- Approved legal terms/privacy version IDs and approved page copy.
- Alibaba Cloud Green/content safety credentials.
- Desktop Ed25519 keys only if desktop deployment tasks resume later.

## Remaining Code Risks

- Task 7 did not exhaustively replay every admin feedback reply/internal-note UI path in a browser. Backend gates and API-level admin report/feedback repairs passed, and browser screenshots covered the admin report surface.
- Download authorization was validated for unauthenticated rejection in this runtime pass. Author-banned, IP-banned, low-reputation, and unverified download paths remain covered by backend handler tests from Task 4 rather than fully recreated through browser state setup in this pass.
- `web_agent_enabled=false` was verified through focused handler test because the running local config keeps `web_agent_enabled=true`; restarting with a temporary override would duplicate the existing unit coverage and disrupt the runtime evidence server.

## Verdict

PASS for local Web Beta repair revalidation.

The known Task 7 blockers from the failed validation pass have been resolved:

- Missing Task 1/2 repair commits were incorporated into the validation branch.
- Existing local PostgreSQL schema was updated with migration 055.
- Search suggestions no longer query `tags.status`.
- Captcha/session, anonymous feedback, admin report audit, search, disabled deploy/payments, and browser rendering evidence all pass in the repaired branch.

## 2026-06-05 Evidence Addendum

Additional validation on `codex/beta/repair-validation-blockers`:

- Added an explicit `features.creator_support_enabled: false` default in `backend/config.yaml`.
- Added `TestDefaultConfigDeclaresCreatorSupportDisabled` to guard against silently relying on the Go zero value for this release flag.
- Added `TestValidateReleaseRejectsDefaultJWTSecret`; release mode now rejects the default `dev-secret-change-in-production` JWT secret.
- Added `TestSTSTokenJSONDoesNotExposeAccessKeySecret`; `STSToken.AccessKeySecret` now uses `json:"-"`.
- Restored the `docs/review/web-beta-review-*`, review repair plan, and `e2e/` evidence files that had been dropped from the aggregate repair branch.
- Confirmed local Docker already had healthy shared `omnicraft-postgres` and `omnicraft-redis` containers. `docker compose up -d postgres redis` from this worktree could not create duplicate fixed-name containers, so existing healthy services were used instead.
- Confirmed migration repair schema exists in the local database: `reports.action_taken` and `feedback_tickets_status_check`.
- Confirmed migrations `041_content_search_vector.sql` and `042_ips_search_vector.sql` include pg_jieba fallback behavior, and `055_web_beta_review_repairs.sql` is idempotent.
- Replayed migrations against disposable local PostgreSQL databases:
  - Empty database: all `backend/migrations/*.sql` in lexical order passed.
  - Upgrade database: `001` through `048`, then `049` and later migrations in lexical order passed.

Commands rerun:

```powershell
cd backend
go test ./config -run TestDefaultConfigDeclaresCreatorSupportDisabled -v
go test ./config -run "TestValidateReleaseRejectsDefaultJWTSecret|TestValidateReleaseRejectsBypassCaptchaAndLoggerSMTP" -v
go test ./internal/pkg/aliyun -run TestSTSTokenJSONDoesNotExposeAccessKeySecret -v
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run build
npm run lint
```

All listed commands passed on 2026-06-05. The release secret scan was rerun; remaining matches are reviewed development placeholders, tests, redaction logic, ordinary refresh-cookie identifiers, frontend error logging, or deferred Tauri HMAC scope while desktop deployment remains disabled.

## 2026-06-07 Final Integration Addendum

Branch: `codex/beta-integration`

Review findings and status:

- Important: Task 6 success audit writes were transactional, but invalid admin report resolution did not write a sanitized failed-attempt audit row. Fixed in `2eb6b02` with `TestAdminResolveReportInvalidStatusRecordsFailedAudit`.
- Important: Task 4 sheet-music detail page could omit the authorized download CTA when the attachment had `oss_key` but no preview `oss_url`. Fixed in `4759a12` with `frontend/tests/sheet-music-viewer-download.test.mjs`.
- Important: Rebase reconciliation produced duplicate `053_*.sql` migration numbering and no `055_*.sql`. Fixed during final integration by renaming the repair migration to `backend/migrations/055_web_beta_review_repairs.sql` and updating references.

Task 1-7 coverage check:

| Repair task | Integration status | Evidence |
| --- | --- | --- |
| Task 1 Auth/Captcha/Release Guard/Cookie Session | Covered | Auth/session/captcha tests and release guard tests pass in full backend gate. |
| Task 2 Verification Cache/Token Atomicity | Covered | Verification/reset focused tests remain covered by full backend gate. |
| Task 3 Feedback/Upload Grants/Notifications/Report Resolution | Covered | Feedback service/handler tests, migration 055 replay, and browser `/feedback` render pass. |
| Task 4 Search Visibility/Download Authorization/Sheet Music CTA | Covered | Search/download handler/repository tests pass; browser search links to `/original/86`; sheet music detail renders `Download Sheet Music` through `DownloadButton`. |
| Task 5 Agent Output Validation/Feature Flags | Covered | Agent service/handler tests pass; disabled Web Agent behavior covered by handler tests. |
| Task 6 Admin Audit Transaction Integrity | Covered | Admin audit transaction tests pass, including failed-attempt audit self-check. |
| Task 7 Evidence Revalidation/Browser Regression | Covered | Fresh 2026-06-07 backend, frontend, migration, and browser checks recorded below. |

Fresh commands rerun from `C:\tmp\omnicraft-worktrees\beta-integration` on 2026-06-07:

```powershell
cd backend
$env:GOCACHE='C:\tmp\omnicraft-go-cache'; go test ./...
$env:GOCACHE='C:\tmp\omnicraft-go-cache'; go vet ./...
$env:GOCACHE='C:\tmp\omnicraft-go-cache'; go build ./...

cd ..\frontend
npm test
npm run lint
npm run build
```

Result: all listed commands passed.

Migration replay:

- Empty database replay passed on `omnicraft_verify_empty_20260607_055`, applying 55 migration files in lexical order.
- Upgrade replay passed on `omnicraft_verify_upgrade_20260607_055`, applying `001` through `048`, then `049` through `055`.
- Idempotence reapply passed on:
  - `041_content_search_vector.sql`
  - `042_ips_search_vector.sql`
  - `053_content_items_soft_delete.sql`
  - `054_users_soft_delete.sql`
  - `055_web_beta_review_repairs.sql`
- `pg_jieba` is unavailable in the local PostgreSQL image; `041` falls back to `simple` config as designed.

Browser regression:

- `/feedback`: form, category control, and submit button render.
- `/search`: submitting keyword `validation` through the page input returns a result linking to `/original/86`.
- `/original/86`: renders exactly one `Download Sheet Music` button, with no direct OSS links and no `omnicraft://` links.

Screenshots:

- `screenshots/final-integration-20260607-055/feedback-page.png`
- `screenshots/final-integration-20260607-055/search-validation.png`
- `screenshots/final-integration-20260607-055/sheet-music-detail-download-cta.png`

Feature flags:

- `features.desktop_deploy_enabled=false`
- `client.download_enabled=false`
- `features.payment_enabled=false`
- `features.creator_support_enabled=false`

Remaining external blockers:

- Production SMTP provider and credentials.
- Alibaba Cloud CAPTCHA 2.0 credentials.
- Alibaba Cloud OSS bucket and credentials.
- Alibaba Cloud Green/content-safety credentials.
- Production PostgreSQL/Redis configuration, including SSL/auth/TLS requirements.
- HTTPS certificate, production API/app origins, and `security.allowed_origins`.
- Strong production `JWT_SECRET`.
- Approved legal terms/privacy version IDs and approved legal copy.
- Desktop Ed25519 keys remain relevant only if the desktop deployment track resumes; desktop deploy and client download stay disabled.
