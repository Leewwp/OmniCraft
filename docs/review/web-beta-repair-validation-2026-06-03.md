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
- Local migration `backend/migrations/053_web_beta_review_repairs.sql` was applied before validation. It is idempotent and confirmed `reports.action_taken` and `feedback_tickets_status_check` exist.

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
Get-Content backend\migrations\053_web_beta_review_repairs.sql | docker exec -i omnicraft-postgres psql -U omnicraft -d omnicraft
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
- Existing local PostgreSQL schema was updated with migration 053.
- Search suggestions no longer query `tags.status`.
- Captcha/session, anonymous feedback, admin report audit, search, disabled deploy/payments, and browser rendering evidence all pass in the repaired branch.

## 2026-06-05 Evidence Addendum

Additional validation on `codex/beta/repair-validation-blockers`:

- Added an explicit `features.creator_support_enabled: false` default in `backend/config.yaml`.
- Added `TestDefaultConfigDeclaresCreatorSupportDisabled` to guard against silently relying on the Go zero value for this release flag.
- Restored the `docs/review/web-beta-review-*`, review repair plan, and `e2e/` evidence files that had been dropped from the aggregate repair branch.
- Confirmed local Docker already had healthy shared `omnicraft-postgres` and `omnicraft-redis` containers. `docker compose up -d postgres redis` from this worktree could not create duplicate fixed-name containers, so existing healthy services were used instead.
- Confirmed migration repair schema exists in the local database: `reports.action_taken` and `feedback_tickets_status_check`.
- Confirmed migrations `041_content_search_vector.sql` and `042_ips_search_vector.sql` include pg_jieba fallback behavior, and `053_web_beta_review_repairs.sql` is idempotent.

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

All listed commands passed on 2026-06-05.
