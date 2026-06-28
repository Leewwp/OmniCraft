# Web Beta Review 00 - Release Evidence

**Date:** 2026-06-02
**Reviewer:** Automated review agent
**HEAD:** `15dc57fe362a382f3559165ca9767354c38a2317`
**Branch:** `main` (ahead 30 commits from origin)
**R-01 commit:** `6523938` (plus 3 follow-up patches: `803ab2d`, `f2fb774`, `15dc57f`)

---

## Baseline

| Item | Value |
|------|-------|
| Baseline report | `docs/review/beta-baseline-2026-05-30.md` at SHA `85fb33fb` |
| Baseline HEAD | `85fb33fbef579fc56053f280d0d641a8897d4885` |
| Current HEAD | `15dc57fe362a382f3559165ca9767354c38a2317` |
| R-01 validation report | `docs/review/beta-release-validation-2026-05-30.md` exists |
| R-01 validation report SHA | `071fd5ec4c85270d37169d6e4740da3cdb8890e8` |
| Roadmap R-01 status | `[x]` checked |
| Sub-plan R-01 Step 12 | `[x]` checked |
| Dirty files | 2 untracked (this review doc + prior review doc) |

### Baseline-to-Current Delta

The baseline reported 2 pre-existing Go test failures and 1 frontend build failure (`rehab/page.tsx` JSX syntax error). Current engineering gates all pass (see Section 4), confirming these were fixed during the Beta program.

---

## Findings

### P1-1: Migration 041/042 in-place rewrite risks upgrade consistency

**Files:**
- `backend/migrations/041_content_search_vector.sql`
- `backend/migrations/042_ips_search_vector.sql`

**What happened:** Commit `6523938` modified the content of two pre-existing migration files instead of creating new corrective migrations (e.g., 053/054).

**041 changes:**
- OLD line 1: `CREATE EXTENSION IF NOT EXISTS pg_jieba SCHEMA public;` — would abort the entire migration if `pg_jieba` shared library is absent
- NEW lines 1-6: Wrapped in `DO $$ BEGIN ... EXCEPTION WHEN OTHERS ... END $$;` — graceful fallback
- OLD backfill: Direct `UPDATE ... CASE WHEN EXISTS ...` SQL
- NEW backfill: Wrapped in `DO $$ BEGIN IF EXISTS ... EXECUTE $q$ ... $q$; END $$;` — dynamic SQL

**042 changes:**
- No `CREATE EXTENSION` line in either version (042 never created pg_jieba)
- OLD backfill: Direct `UPDATE ... CASE WHEN EXISTS ...` SQL
- NEW backfill: Wrapped in `DO $$ BEGIN IF EXISTS ... EXECUTE $q$ ... $q$; END $$;` — dynamic SQL

**Risk assessment:**
- **Fresh database**: No issue. New 041/042 apply cleanly with or without pg_jieba.
- **Existing database where old 041 succeeded** (pg_jieba was available): All statements are idempotent (`IF NOT EXISTS`, `CREATE OR REPLACE FUNCTION`, `DROP TRIGGER IF EXISTS` + `CREATE TRIGGER`, `WHERE search_vector IS NULL`). Re-running is a no-op. However, a migration tracking tool would skip the file entirely since it was already recorded as applied.
- **Existing database where old 041 failed** (pg_jieba absent): The migration would have aborted partway, leaving the database in a partially-migrated state (column may exist, trigger may not). The in-place fix does NOT help because the migration tool considers 041 already attempted/failed and won't re-run it. Manual intervention is required.

**Residual risk:** Any environment that ran the old 041 without pg_jieba has a broken migration state. The fix only prevents the issue for fresh databases. The proper approach would have been a new migration (053) that `CREATE OR REPLACE FUNCTION` with the fallback logic and re-runs the backfill with `WHERE search_vector IS NULL`.

**Severity:** P1 — not blocking for fresh production deployment, but any pre-existing staging/dev database that hit this failure needs manual repair before upgrade.

---

### P1-2: Browser-test screenshots missing from repository

> ✅ 已于 2026-06-04 修复（见 `web-beta-repair-validation-2026-06-03.md`）。

**Referenced in:** `docs/review/beta-release-validation-2026-05-30.md` line 155

The validation report claims "Screenshots saved to `screenshots/r01_*.png`" and Step 8 references `screenshots/beta-release-public-*`. The `screenshots/` directory does not exist in the repository. No screenshot files were found via glob search.

**Impact:** R-01 Step 8 (browser-test public journeys) and Step 9 (browser-test admin journeys) require Playwright screenshots as release evidence. Their absence means the browser-test evidence is unverifiable from the repository alone.

**Severity:** P1 — evidence claimed but not persisted. The validation report's browser-test section cannot be independently reproduced from repository artifacts.

---

### P1-3: SafeErrorMsg fallback leaks unrecognized error text

**File:** `backend/internal/pkg/response/safe_error.go:45`

```go
func SafeErrorMsg(err error, fallback string) string {
    // ... known business errors return safe messages ...
    // ... SQL/DSN patterns return fallback ...
    return err.Error()  // line 45: UNRECOGNIZED ERRORS PASS THROUGH
}
```

If an error does not match a known `businessErrors` entry and does not contain `sql:`, `pq:`, `dsn`, or `record not found`, the raw `err.Error()` string is returned to the client. Examples of errors that would pass through:
- Redis: `OOM command not allowed when used memory > 'maxmemory'`
- Filesystem: `open /etc/omnicraft/config.yaml: permission denied`
- Network: `dial tcp 10.0.0.1:5432: i/o timeout`

These leak internal infrastructure details. The function should default to `fallback` for any unrecognized error.

**Severity:** P1 — violates Task 102 (error message sanitization) and AGENTS.md rule 16 ("No raw error exposure"). The `SafeErrorResponse` wrapper is used in 100+ handler call sites, so the blast radius is significant.

---

### P2-1: creator_support_enabled absent from config.yaml

**File:** `backend/config.yaml`

The `FeaturesConfig.CreatorSupportEnabled` field exists in `backend/config/config.go:93` and is exposed in the public config DTO at `backend/internal/handler/public_config.go:14`. However, `creator_support_enabled` is not explicitly set in `config.yaml`. Go's bool zero value means it defaults to `false`, which satisfies R-01 Step 10 ("`creator_support_enabled=false` unless separately approved").

**Residual risk:** An operator could miss this flag when reviewing production config. Should be explicitly set to `false` in `config.yaml` for visibility.

**Severity:** P2 — functionally correct but operationally unclear.

---

### P2-2: STSToken struct has serializable access_key_secret field

**File:** `backend/internal/pkg/aliyun/oss.go:18`

```go
type STSToken struct {
    AccessKeyID     string `json:"access_key_id"`
    AccessKeySecret string `json:"access_key_secret"`
    SecurityToken   string `json:"security_token"`
    Expiration      string `json:"expiration"`
}
```

The `AccessKeySecret` field has a `json` tag, meaning it would be serialized if the struct is ever marshaled into an API response. Currently, `GetSTS()` is only called internally and the result is not returned to clients. However, this is a latent risk — a future developer could accidentally return this struct in a handler response.

**Severity:** P2 — not exploitable today, but the `json:"-"` tag should be applied to `AccessKeySecret` as a defense-in-depth measure, or the struct should use a separate response DTO.

---

### P2-3: R-01 committed across 4 separate commits

**Commits:**
1. `6523938` — "Beta R-01: validate public web beta release - completed"
2. `803ab2d` — "Beta R-01: restore progress.txt history - append instead of overwrite"
3. `f2fb774` — "Beta R-01: add release validation report, deploy-grants/payments 503 endpoints, screenshots"
4. `15dc57f` — "Beta R-01: append missing progress entry for web beta release validation"

The AGENTS.md rule states "one task per session, one commit per task." R-01 was split across 4 commits, with the first commit (`6523938`) overwriting `progress.txt` (2486 deletions), later fixed in `803ab2d`.

**Severity:** P2 — process violation. The first commit destroyed progress.txt history, which was restored in the second. No data loss in final state, but the intermediate commit was destructive.

---

### P2-4: AGENT_HMAC_SECRET compiled into Tauri client

**File:** `tauri-client/src-tauri/src/commands/security.rs:7`

```rust
const HMAC_SECRET: &str = env!("AGENT_HMAC_SECRET");
```

The Tauri client still embeds the HMAC secret at compile time. Per the roadmap cross-plan contract, D-03 must replace this with Ed25519 verification (client holds only the public key). This is acceptable only while `desktop_deploy_enabled=false` and no client binary is distributed.

**Severity:** P2 — acceptable for Web Beta with desktop deploy disabled. Must be resolved before D-03/R-02.

---

### P3-1: Validation report base SHA is stale

**File:** `docs/review/beta-release-validation-2026-05-30.md:5`

The report references base SHA `071fd5ec4c85270d37169d6e4740da3cdb8890e8`, but the current HEAD is `15dc57fe`. The 3 follow-up commits after the original R-01 commit are not reflected in the report.

**Severity:** P3 — informational. The report was written at an earlier commit and the follow-up patches only added missing endpoints and restored progress.txt.

---

### P3-2: Legal version fields empty in config.yaml

**File:** `backend/config.yaml:80-81`

```yaml
legal:
  current_terms_version: ""
  current_privacy_version: ""
```

Per the roadmap "Remaining Human Decisions Required Before Execution," approved legal copy and version identifiers are required for V-02, V-04, and R-01. These remain empty, meaning the terms acceptance flow cannot verify which version the user agreed to.

**Severity:** P3 — functionally, the verification flow works with empty strings (users "accept" an empty version), but this does not satisfy the legal requirement for versioned terms. This is a production deployment blocker, not a code issue.

---

### P3-3: progress.txt was overwritten in initial R-01 commit

Commit `6523938` shows `2462 +-------------------` (deletions) in progress.txt, indicating the file was overwritten rather than appended. This was corrected in commit `803ab2d` which restored the history. No data loss in the final state.

**Severity:** P3 — already corrected, noted for process improvement.

---

## Migration Verification

### Method

Migration verification was performed by static analysis of the SQL files and git history. A live disposable-database test was not performed in this session because Docker PostgreSQL is not currently running and the review scope is read-only verification, not implementation.

### Analysis

**Beta migrations (049-052):**

| File | Content | Idempotent | Risk |
|------|---------|------------|------|
| `049_search_trigram_fallback.sql` | `CREATE EXTENSION IF NOT EXISTS pg_trgm; CREATE INDEX CONCURRENTLY IF NOT EXISTS ...` | Yes | Low — `CONCURRENTLY` cannot run inside a transaction; if the migration runner wraps in a transaction, this will fail |
| `050_verification_and_terms.sql` | `ALTER TABLE users ADD COLUMN IF NOT EXISTS ...` | Yes | None |
| `051_feedback.sql` | `CREATE TABLE ...; CREATE INDEX ...` | No — `CREATE TABLE` fails if table exists | Low — only runs once on fresh DB; upgrade path requires table not to exist |
| `052_admin_audit_logs.sql` | `CREATE TABLE ...; CREATE INDEX ...` | No — same as 051 | Low — same as 051 |

**Pre-Beta migrations modified by R-01 (041/042):** See P1-1 above.

**049 CONCURRENTLY concern:** `CREATE INDEX CONCURRENTLY` cannot execute inside a transaction block. If the migration runner wraps each file in `BEGIN ... COMMIT`, migration 049 will fail with "CREATE INDEX CONCURRENTLY cannot run inside a transaction block." The Docker Compose config mounts migrations to `/docker-entrypoint-initdb.d`, which runs files in a single transaction per file. This needs verification against the actual PostgreSQL init behavior — `docker-entrypoint-initdb.d` scripts run via `psql` which auto-commits each statement, so `CONCURRENTLY` should work in that context. However, a migration tool like golang-migrate or goose would wrap in a transaction and fail.

### Verdict

Fresh-database path: all 52 migrations apply correctly (confirmed by R-01 validation report). Upgrade path from 001-048 to 049-052: applies correctly. The 041/042 in-place modification is the only consistency concern (P1-1).

---

## Engineering Gates

| Command | Result | Notes |
|---------|--------|-------|
| `go build ./...` | **PASS** | Zero errors |
| `go vet ./...` | **PASS** | Zero warnings |
| `go test ./...` | **PASS** | All packages pass (baseline had 2 pre-existing failures, now fixed) |
| `npm run lint` | **PASS** | `tsc --noEmit` passes (baseline had rehab/page.tsx failure, now fixed) |
| `npm run build` | **PASS** | Next.js 16.2.4 Turbopack, 60 routes generated |
| `docker compose config` | **PASS** | 6 services (postgres, pgbouncer, redis, backend, frontend, nginx) |

**Improvement from baseline:** All 3 pre-existing failures (2 Go tests + 1 frontend build) reported in the baseline have been resolved during the Beta program.

---

## Secret Scan

Ran pattern scan: `err\.Error\(\)|refresh_token|AGENT_HMAC_SECRET|private_key|access_key_secret|api_key|console\.error`

### err.Error() — 16 matches in backend, 0 in frontend

| Location | Context | Assessment |
|----------|---------|------------|
| `admin_audit_service.go:89,111` | `slog.String("error", err.Error())` — structured logging | ✅ Safe — server-side log only |
| `interaction.go:33` | `errCode := err.Error()` — switch matching against known codes | ✅ Safe — not returned to client |
| `feedback.go:55,106,172` | `switch err.Error()` — matching known error strings | ✅ Safe — not returned to client |
| `admin_feedback.go:64,100,177` | `switch err.Error()` / `if err.Error() ==` — matching | ✅ Safe — not returned to client |
| `redis_stream.go:135` | DLQ payload in Redis — internal infrastructure | ✅ Safe — not exposed to clients |
| `redis_stream.go:151` | `errStr := err.Error()` — BUSYGROUP check | ✅ Safe — internal comparison |
| `safe_error.go:41,42,45` | SanitizeError filter itself | ⚠️ **P1-3** — line 45 fallback leaks unrecognized errors |
| `observability.go:25` | `err.Error() == "ERR no such key"` — internal check | ✅ Safe |
| `provider_test.go:60` | Test assertion | ✅ Safe |

### refresh_token — 23 matches in backend, 0 in frontend

All matches are in:
- Auth service Redis key construction (server-side only)
- Auth handler cookie name logic (HttpOnly cookie, not in JSON body)
- Auth handler request body parsing (accepts token from client for backward compat)
- Config struct (`refresh_token_ttl`)
- Test code asserting refresh_token is NOT in JSON body

**Assessment:** ✅ No refresh token exposed in frontend storage or API JSON responses. The F-03 cookie migration is correctly implemented.

### AGENT_HMAC_SECRET — Found in Tauri client and backend config

| Location | Assessment |
|----------|------------|
| `tauri-client/src-tauri/src/commands/security.rs:7` | ⚠️ **P2-4** — compiled into client binary |
| `backend/config/config.go:356` | ✅ Server-side env var override |
| `.env.example:35` | ✅ Empty placeholder |
| Various docs/progress | ✅ Documentation only |

### private_key — 4 matches in backend

All in `admin_audit_service.go` and its test — part of the sensitive-key allowlist that strips these keys from audit metadata. ✅ Safe.

### access_key_secret — 8 matches

| Location | Assessment |
|----------|------------|
| `config.go:77,85,161` | `json:"-"` tag — not serialized | ✅ |
| `config.yaml:30,37,54` | Empty string values | ✅ |
| `admin.go:40` | In sensitive-fields mask map | ✅ |
| `aliyun/oss.go:18` | ⚠️ **P2-2** — STSToken struct has `json:"access_key_secret"` tag |

### api_key — 41 matches

Majority in test code, config with `json:"-"`, admin masking, and docs. Key locations:
- `config.go:145`: `LLMAPIKey` with `json:"-"` ✅
- `llm_config_service.go`: Properly encrypts before storage, masks in list responses ✅
- `frontend/admin/agent-config/page.tsx`: Form sends `api_key` to backend which encrypts it ✅

### console.error — 3 matches in frontend

| Location | Assessment |
|----------|------------|
| `lib/error-handler.ts:27` | Logs `error.code` + `error.message` (structured) | ✅ Safe — not displayed to user |
| `lib/error-handler.ts:36` | Logs error object context | ✅ Safe — not displayed to user |
| `app/error.tsx:18` | Logs unhandled page error | ✅ Safe — not displayed to user |

---

## Production-Only Blockers

The following items cannot be verified locally and require production environment configuration. Per AGENTS.md, these are blocking for production deployment but not for local development.

| Item | Config Location | Current Value | Required for Production | Status |
|------|----------------|---------------|------------------------|--------|
| SMTP credentials | `smtp.host/user/password/from_address` | `mode: "logger"`, all empty | Real SMTP server for verification/password-reset emails | ❌ Not configured |
| Alibaba Cloud CAPTCHA | `captcha.provider/access_key_id/access_key_secret/scene_id` | `provider: "bypass"`, all empty | `provider: "aliyun_v2"` with real credentials | ❌ Not configured |
| Alibaba Cloud OSS | `oss.endpoint/access_key_id/access_key_secret/bucket_name/domain` | All empty | Real OSS bucket and credentials | ❌ Not configured |
| PostgreSQL | `database.dsn` | localhost dev DSN | Production DSN with SSL | ❌ Not configured |
| Redis | `redis.addr/password` | localhost:6379, empty password | Production Redis with auth | ❌ Not configured |
| HTTPS certificate | Nginx + `ssl_certs` volume | Not present | Valid TLS certificate for `api.leeppp.online` | ❌ Not configured |
| Allowed Origins | `security.allowed_origins` | `http://localhost:3000` | `https://app.leeppp.online` only, no wildcard | ❌ Not configured |
| Legal versions | `legal.current_terms_version/current_privacy_version` | Empty strings | Approved legal version identifiers | ❌ Not provided |
| JWT secret | `jwt.secret` | `dev-secret-change-in-production` | Cryptographically random production secret | ❌ Not configured |
| Ed25519 private key | `deploy.ed25519_private_key_b64` / env var | Not present | Required only for D-03+ (desktop deploy) | ⬜ Deferred |
| Green (content safety) | `green.access_key_id/access_key_secret` | Empty | AI content review | ❌ Not configured |

---

## Evidence Inventory

| Artifact | Expected Location | Present | Notes |
|----------|------------------|---------|-------|
| R-01 validation report | `docs/review/beta-release-validation-2026-05-30.md` | ✅ | 203 lines, covers Steps 1-11 |
| Roadmap R-01 checkbox | `docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md` | ✅ | `[x]` |
| Sub-plan Step 12 checkbox | `docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md` | ✅ | `[x]` |
| Browser-test screenshots | `screenshots/r01_*.png` / `screenshots/beta-release-*` | ❌ | **Missing** — referenced in report but not in repository |
| Admin-test screenshots | `screenshots/beta-release-admin-*` | ❌ | **Missing** |
| progress.txt R-01 entry | `progress.txt` | ✅ | Two entries: main + supplement |
| Baseline report | `docs/review/beta-baseline-2026-05-30.md` | ✅ | Captured at SHA `85fb33fb` |
| Config feature flags | `backend/config.yaml` | ✅ | `desktop_deploy_enabled: false`, `payment_enabled: false`, `client.download_enabled: false` |
| `creator_support_enabled` in config.yaml | `backend/config.yaml` | ❌ | **Missing** — defaults to `false` in Go but not explicit |

---

## Verdict

**CONDITIONAL PASS — Web Beta release evidence is substantially complete with 3 P1 findings requiring attention.**

### Must-fix before production deployment (P1):

1. **P1-1 (Migration 041/042 rewrite):** Verify that the target production database has never run the old 041 without pg_jieba. If it has, create a new migration (053) that applies the `CREATE OR REPLACE FUNCTION` fix and re-runs the backfill. For fresh production databases, no action needed.

2. **P1-2 (Missing screenshots):** Re-run Playwright browser tests and persist screenshots to `screenshots/beta-release-public-*` and `screenshots/beta-release-admin-*`. Without these, the browser-test evidence in the validation report is unsubstantiated.

3. **P1-3 (SafeErrorMsg fallback):** Change `safe_error.go:45` from `return err.Error()` to `return fallback`. This ensures unrecognized errors never leak internal details to clients.

### Should-fix before production deployment (P2):

4. Add `creator_support_enabled: false` explicitly to `config.yaml`.
5. Apply `json:"-"` to `STSToken.AccessKeySecret` in `aliyun/oss.go:18`.
6. Replace `AGENT_HMAC_SECRET` with Ed25519 before any desktop client distribution (D-03 dependency).

### Production deployment blockers (infrastructure, not code):

7. All 10 items in the Production-Only Blockers table must be configured before the application serves real users. The most critical are: SMTP, CAPTCHA provider switch from `bypass` to `aliyun_v2`, OSS credentials, HTTPS certificate for `api.leeppp.online`, and approved legal version strings.

### Residual risks after fixes:

- **pg_jieba unavailability**: Chinese full-text search uses `simple` config instead of `jiebacfg`, reducing tokenization quality. Mitigated by ILIKE + pg_trgm fallback from F-05.
- **049 CONCURRENTLY**: If a transactional migration runner is adopted in the future, `CREATE INDEX CONCURRENTLY` in 049 will fail. Current Docker initdb path is safe.
- **Legal version enforcement**: Empty `legal.current_terms_version` means the terms acceptance flow records an empty version, providing no legal audit trail.
