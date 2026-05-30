# OmniCraft Beta Baseline Report — 2026-05-30

## 1. Git State

| Item | Value |
|------|-------|
| Branch | `main` (tracking `origin/main`) |
| HEAD SHA | `85fb33fbef579fc56053f280d0d641a8897d4885` |
| Dirty files | None (clean working tree) |

## 2. Task Status (IDs 156–168)

All 13 tasks in the 156–168 range have `passes: true`. Terminal output contained mojibake for CJK titles; the `passes` column was unambiguous.

## 3. Quality Gate Results

### Backend (Go)

| Command | Result | Notes |
|---------|--------|-------|
| `go build ./...` | **PASS** | Zero errors |
| `go vet ./...` | **PASS** | Zero warnings |
| `go test ./...` | **FAIL** | 2 test failures (pre-existing) |

**Pre-existing test failures:**

1. `internal/handler/routes_security_test.go:15` — `TestAgentRoutesUseAgentRateLimit`: agent routes must include group-level AgentRateLimit
2. `internal/service/review_remediation_test.go:70` — `TestApprovedAppealRestoresContentStatus`: comment appeal should not use content updates, got `map[string]interface{}{"status":"published"}`

### Frontend (Next.js)

| Command | Result | Notes |
|---------|--------|-------|
| `npm run lint` (tsc --noEmit) | **FAIL** | 5 TS errors in `app/(protected)/rehab/page.tsx` (syntax error at line 160) |
| `npm run build` | **FAIL** | Same `rehab/page.tsx` parsing error blocks Turbopack |

**Pre-existing frontend failure:**

- `app/(protected)/rehab/page.tsx:160` — JSX syntax error: `Expected '</', got '{'`. The file has a malformed JSX block around the `CourseContent` conditional rendering.

### Tauri Client

| Command | Result | Notes |
|---------|--------|-------|
| `npm run build` (vite) | **PASS** | 30 modules, built in 769ms |
| `cargo test --manifest-path src-tauri/Cargo.toml` | **FAIL** | `AGENT_HMAC_SECRET` env var not defined at compile time (requires `env!` macro) |

### Infrastructure

| Command | Result | Notes |
|---------|--------|-------|
| `docker compose config` | **PASS** | All 6 services render correctly (postgres, pgbouncer, redis, backend, frontend, nginx) |

## 4. External Dependencies Not Configured

| Dependency | Status | Impact |
|------------|--------|--------|
| Alibaba Cloud OSS credentials | Not configured (`OSS_ACCESS_KEY_ID` / `OSS_ACCESS_KEY_SECRET` empty) | File upload/download endpoints non-functional |
| Alibaba Cloud Green (content safety) | Not configured (`GREEN_ACCESS_KEY_ID` / `GREEN_ACCESS_KEY_SECRET` empty) | AI content review falls back to pass-through |
| SMTP server | Not configured | Password reset and email verification emails cannot be sent |
| CAPTCHA provider | Not configured | Registration/login CAPTCHA not enforced |
| `AGENT_HMAC_SECRET` for Tauri | Not set in build environment | `cargo test` / `cargo build` for Tauri client fails |
| Production HTTPS / domain | Not configured | Cookie `Secure` flag and CORS production origins not testable |

## 5. Follow-Up Task IDs

- **F-02**: Fail-closed interaction eligibility (depends on this baseline)
- **F-03**: Move browser refresh sessions to HttpOnly cookies
- **F-04**: Add minimal public runtime config
- **F-05**: Make Chinese keyword search reliable
- **F-06**: Route every download through authorization
- **Pre-existing fix needed**: `rehab/page.tsx` JSX syntax error (blocks frontend build)
- **Pre-existing fix needed**: `routes_security_test.go` and `review_remediation_test.go` test failures
