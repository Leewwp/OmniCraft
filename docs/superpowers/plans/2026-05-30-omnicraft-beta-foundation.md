# OmniCraft Beta Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the shared security and reliability baseline required by both Beta tracks.

**Architecture:** Convert authentication from JWT-trusting fallback behavior to a fail-closed runtime status check, migrate browser sessions to server-managed cookies, expose a strict public-config allowlist, add Chinese-search fallback, and remove direct OSS download paths. Preserve existing handlers where possible and add focused tests around trust boundaries.

**Tech Stack:** Go/Gin/GORM/PostgreSQL/Redis, Next.js App Router, `next-intl`, MCP Playwright.

---

## File Structure

### Backend

- Create: `backend/internal/service/runtime_status.go` - fail-closed authenticated status resolution.
- Create: `backend/internal/middleware/interaction.go` - common verified-email, ban and reputation eligibility middleware.
- Create: `backend/internal/repository/content_visibility.go` - one reusable content-visibility scope for keyword and Agent search.
- Modify: `backend/internal/handler/routes.go` - mount shared guards and public config route.
- Modify: `backend/internal/handler/auth.go` - cookie session responses.
- Modify: `backend/internal/service/auth_service.go` - strict refresh-token Redis behavior.
- Create: `backend/internal/handler/public_config.go` - minimal public runtime config response.
- Modify: `backend/internal/model/config_public.go` - split public DTO from admin DTO.
- Modify: `backend/config/config.go` and `backend/config.yaml` - desktop feature flag and public metadata.
- Modify: `backend/internal/repository/search_repo.go` and `backend/internal/service/agent_service.go` - trigram fallback and shared visibility filters.
- Modify: `backend/internal/service/oss_service.go` - short download TTL from config.
- Modify: `backend/internal/handler/content.go` - explicit download authorization response.
- Create/Modify tests beside touched Go packages.

### Frontend

- Modify: `frontend/lib/api.ts` - cookie-based refresh and blob/download helper.
- Modify: `frontend/lib/auth.ts` - remove refresh-token persistence.
- Modify: `frontend/contexts/AuthContext.tsx` - cookie refresh contract.
- Modify: `frontend/lib/useSSE.ts` - read the in-memory access token for authenticated streams.
- Modify: `frontend/proxy.ts` - treat browser cookie as a routing hint only.
- Create: `frontend/lib/public-config.ts` - public config fetch/cache helper.
- Create: `frontend/components/content/DownloadButton.tsx` - authorized download CTA.
- Modify: `frontend/components/content/ContentDetail.tsx`.
- Modify: `frontend/components/content/SheetMusicViewer.tsx`.
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`.

## Task F-01: Capture A Reproducible Baseline

**Files:**
- Read: `task.json`
- Read: `progress.txt`
- Read: `docs/review/*.md`
- Create: `docs/review/beta-baseline-2026-05-30.md`

- [x] **Step 1: Record current git state**

Run:

```powershell
git status --short --branch
git rev-parse HEAD
node -e "const fs=require('fs'); const j=JSON.parse(fs.readFileSync('task.json','utf8')); console.log(j.tasks.filter(t=>t.id>=156&&t.id<=168).map(t=>`${t.id}\t${t.passes}\t${t.title}`).join('\n'))"
```

Expected: SHA, dirty files, and Tasks 156-168 are captured without interpreting terminal mojibake as file corruption.

- [x] **Step 2: Run the historical quality gates**

Run:

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
cd ..\tauri-client
npm run build
cargo test --manifest-path src-tauri/Cargo.toml
```

Expected: record PASS/FAIL for every command. Do not repair unrelated failures in this task.

- [x] **Step 3: Validate local infrastructure configuration**

Run:

```powershell
cd ..\
docker compose config
```

Expected: compose configuration renders successfully. Record unavailable external services as blockers, not as passing checks.

- [x] **Step 4: Write the baseline report**

Document: SHA, dirty files, command results, external dependencies not configured, and concrete follow-up task IDs.

- [ ] **Step 5: Commit**

```powershell
git add docs/review/beta-baseline-2026-05-30.md docs/superpowers/plans progress.txt
git commit -m "Beta F-01: capture public beta baseline - completed"
```

## Task F-02: Fail Closed And Centralize Interaction Eligibility

**Files:**
- Create: `backend/internal/service/runtime_status.go`
- Modify: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/interaction.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/service/reputation_service.go`
- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/user.go`
- Create: `backend/internal/middleware/auth_test.go`
- Create: `backend/internal/middleware/interaction_test.go`

- [x] **Step 1: Write failing middleware tests**

Cover:

```go
func TestAuthRequiredRejectsWhenRedisAndDBCannotConfirmStatus(t *testing.T) {}
func TestAuthRequiredUsesFreshDBRoleInsteadOfJWTClaim(t *testing.T) {}
func TestAuthRequiredRejectsMissingOrSoftDeletedUser(t *testing.T) {}
func TestOptionalAuthDowngradesToAnonymousWhenStatusCannotBeConfirmed(t *testing.T) {}
func TestInteractionRequiredRejectsUnverifiedEmail(t *testing.T) {}
func TestInteractionRequiredRejectsLowReputation(t *testing.T) {}
func TestInteractionRequiredRejectsPublishFreezeWhenPolicyRequiresIt(t *testing.T) {}
```

Expected HTTP `503` error envelope for unknown runtime status:

```json
{"code":"AUTH_STATUS_UNAVAILABLE","message":"account status is temporarily unavailable"}
```

- [x] **Step 2: Run the tests and verify failure**

Run:

```powershell
cd backend
go test ./internal/middleware -run "TestAuthRequired|TestInteractionRequired" -v
```

Expected: `TestAuthRequiredRejectsWhenRedisAndDBCannotConfirmStatus` fails because current code falls back to JWT claims.

- [x] **Step 3: Implement strict runtime status resolution**

Use an explicit resolver in `backend/internal/service/runtime_status.go`:

```go
type RuntimeUserStatus struct {
    ID              int64
    Role            string
    IsBanned        bool
    EmailVerifiedAt *time.Time
    Reputation      int
}

func ResolveRuntimeUserStatus(ctx context.Context, db *gorm.DB, rdb *redis.Client, userID int64) (*RuntimeUserStatus, error)
```

The resolver belongs in `service` (not `middleware`) to avoid coupling the middleware package to GORM/Redis directly. The middleware calls the service-layer resolver.

Rules:

- Redis cache may accelerate status reads.
- A cache miss must query PostgreSQL.
- Redis failure plus PostgreSQL failure returns `AUTH_STATUS_UNAVAILABLE`.
- Protected requests must also fail closed when access-token revocation state cannot be checked in Redis. `OptionalAuth` downgrades to anonymous in that case.
- Missing, soft-deleted and banned users never become authenticated.
- Role comes from runtime status, never stale JWT claims.
- Use `cfg.Cache.UserStatusTTL`, not a hardcoded five minutes.
- `OptionalAuth` downgrades to anonymous when status cannot be confirmed; it must not retain claim-derived permissions.
- Invalidate or refresh cached runtime status when email verification, reputation, ban state or role changes. The cache invalidation responsibility is centralized through `RuntimeStatusCache.Invalidate(userID)` in `runtime_status.go`. The following handlers must call it after their respective state changes:
  - `verification_service.go` (email verification)
  - `reputation_service.go` (every reputation mutation)
  - `admin.go` (user ban/unban and any future role changes)
  - `user.go` (account soft delete)
- Do not use `reputation == 0` as a cache-miss sentinel. Zero is a valid score; cache lookups must distinguish missing keys from stored zero values.
- If `cfg.Reputation.MinScoreForInteraction <= 0`, the interaction guard must fail safe with `503 CONFIG_ERROR` rather than falling back to a hardcoded threshold. A zero or negative value indicates a configuration error that must be fixed, not silently tolerated.

- [x] **Step 4: Add a shared interaction guard**

Create middleware configurable by required capabilities:

```go
type InteractionPolicy struct {
    RequireVerifiedEmail bool
    RequireReputation    bool
    RequireNoPublishFreeze bool
}
```

Use `cfg.Reputation.MinScoreForInteraction`; do not pass a hardcoded `3` from `routes.go`. Mount verified-email and reputation checks on publish, edit/delete content, comments, reactions, reports, favorites, PR submission/accept/reject/merge, judge exams/votes/reason interactions, follows, private messages, downloads, Agent endpoints and future deploy-grant issue. Apply `RequireNoPublishFreeze` to publish only. Keep content-specific checks such as ownership, `allow_copy`, content status and attachment ownership inside the relevant handler/service.

The complete list of policy groups for table-driven route tests is:

| Policy Group | Endpoints | Required Capabilities |
|---|---|---|
| `publish` | POST /contents | verified email, reputation, no publish freeze |
| `edit_delete` | PATCH /contents/:id, DELETE /contents/:id | verified email, reputation |
| `comments` | POST /social/comments, PATCH /social/comments/:id, DELETE /social/comments/:id, POST /discussions/:id/comments | verified email, reputation |
| `reactions` | POST /social/reactions | verified email, reputation |
| `favorites` | POST /favorites, DELETE /favorites/:contentId | verified email, reputation |
| `reports` | POST /contents/:id/report, POST /social/comments/:id/report | verified email, reputation |
| `pull_requests` | POST /pr, POST /pr/:id/accept, POST /pr/:id/reject, POST /pr/:id/merge | verified email, reputation |
| `judge` | POST /judge/exam/submit, POST /judge/vote, POST /judge/reasons/:id/vote | verified email, reputation |
| `follows` | POST /users/:id/follow, DELETE /users/:id/follow, POST /ips/:id/follow, DELETE /ips/:id/follow | verified email, reputation |
| `messages` | POST /messages, GET /messages | verified email, reputation |
| `downloads` | GET /contents/:id/download | verified email, reputation |
| `agent` | all /agent endpoints, including POST /agent/chat/stream and POST /agent/search | verified email, reputation |
| `deploy_grant` | POST /deploy-grants | verified email, reputation |

Add table-driven route tests that exercise at least one endpoint from every policy group listed above. A middleware unit test alone is insufficient because the current defect includes incomplete route mounting.

- [x] **Step 5: Run focused and backend-wide tests**

```powershell
go test ./internal/middleware ./internal/handler ./internal/service -v
go test ./...
go vet ./...
go build ./...
```

- [x] **Step 6: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta F-02: enforce fail-closed interaction eligibility - completed"
```

## Task F-03: Move Browser Refresh Sessions To HttpOnly Cookies

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/internal/service/auth_service.go`
- Modify: `backend/internal/middleware/csrf.go`
- Modify: `backend/internal/middleware/cors.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `frontend/lib/api.ts`
- Modify: `frontend/lib/auth.ts`
- Modify: `frontend/contexts/AuthContext.tsx`
- Modify: `frontend/lib/useSSE.ts`
- Modify: `frontend/app/(public)/register/page.tsx`
- Modify: `frontend/proxy.ts`
- Create: `backend/internal/handler/auth_cookie_test.go`

- [ ] **Step 1: Write failing cookie contract tests**

Cover:

```go
func TestLoginSetsHttpOnlyRefreshCookie(t *testing.T) {}
func TestRefreshReadsCookieAndRotatesRefreshCookie(t *testing.T) {}
func TestLogoutClearsRefreshCookie(t *testing.T) {}
func TestRefreshFailsClosedWhenRedisUnavailable(t *testing.T) {}
func TestRefreshRejectsMissingOrInvalidCSRFToken(t *testing.T) {}
func TestCredentialedCORSAllowsConfiguredProductionOriginOnly(t *testing.T) {}
```

Expected refresh cookie attributes in production: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, no `Domain`, configured TTL. Cookie name is selected automatically based on `config.yaml > server.mode`: when `mode == "release"` (the existing Gin production-mode convention), use `__Host-refresh_token` (requires HTTPS); otherwise use `refresh_token`. The same logic applies to the CSRF cookie name (`__Host-csrf` vs `csrf-token`). Do not introduce a second mode string such as `production`; derive both cookie names from one helper that reads the existing server mode.

- [ ] **Step 2: Run tests and verify failure**

```powershell
cd backend
go test ./internal/handler -run "TestLoginSetsHttpOnly|TestRefreshReadsCookie|TestLogoutClears|TestRefreshFailsClosed" -v
```

- [ ] **Step 3: Add cookie helpers and strict refresh behavior**

Add helpers in `auth.go`:

```go
func setRefreshCookie(c *gin.Context, cfg *config.Config, token string)
func clearRefreshCookie(c *gin.Context, cfg *config.Config)
```

Rules:

- Login returns a short-lived access token in JSON and sets refresh token as HttpOnly cookie.
- Refresh reads only the cookie, rotates it, and returns a new access token.
- Logout revokes access and refresh credentials and clears the cookie.
- Redis unavailable during refresh must reject the refresh request.
- Never expose refresh tokens in JSON, URLs, logs or browser storage.
- Refresh and logout use the existing double-submit CSRF middleware: the readable `csrf-token`/`__Host-csrf` cookie must match `X-CSRF-Token`. Do not introduce a second CSRF scheme.
- Add `GET /api/v1/auth/csrf` as an idempotent bootstrap endpoint. It returns the CSRF token in the response body and ensures the readable CSRF cookie exists. The frontend API helper keeps the returned value in memory and sends it as `X-CSRF-Token` before the first unsafe request, including a fresh browser session before login or registration.
- Credentialed CORS allows explicit configured production origins only. Localhost variants remain development-only and wildcard origins are rejected.

- [ ] **Step 4: Remove frontend refresh-token storage**

Before editing the registration state, run:

```powershell
rg -n "## Page: /register" design/ui-spec.md
```

This step checks whether the registration page already has a UI spec, so the implementer understands which existing visual patterns to preserve. The actual verification UI changes belong to V-03; this task only removes refresh-token persistence and adjusts the post-registration redirect.

Delete `REFRESH_TOKEN`, `getRefreshToken()` and every `localStorage.setItem("refresh_token", ...)`. Replace access-token `localStorage` and JavaScript-written cookies with an in-memory module variable. On page reload, call refresh using the HttpOnly cookie to obtain a new short-lived access token. `frontend/proxy.ts` may inspect only the presence of the HttpOnly refresh cookie as a routing hint when Web and API are served from the same host; it must not decode or trust stale access claims, including for `/admin`. When production uses a separate API hostname, remove cookie-dependent proxy routing and let `AuthProvider` plus protected layouts perform the refresh/redirect decision.

**Deployment architecture note:** Production Web and API are confirmed to use separate HTTPS subdomains under `leeppp.online`. This is a same-site, cross-origin deployment. Before F-03 is marked complete, the maintainer must provide the exact Web and API hostnames. Keep the refresh and CSRF cookies host-only on the API hostname, keep `SameSite=Lax`, set `Secure`, allow only the exact Web origin through credentialed CORS, and enforce strict `Origin` validation for unsafe requests. The bootstrap response-body token allows the frontend to send `X-CSRF-Token` without reading an API-host cookie directly. Because `frontend/proxy.ts` cannot inspect an API-host cookie, remove cookie-dependent proxy routing and let `AuthProvider` plus protected layouts perform the refresh/redirect decision. Do not guess the final subdomains or silently loosen cookie policy.

Update refresh calls:

```ts
await fetch(`${API_URL}/api/v1/auth/refresh`, {
  method: "POST",
  credentials: "include",
});
```

- [ ] **Step 5: Run backend and frontend checks**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
rg -n "refresh_token|REFRESH_TOKEN|getRefreshToken" .
```

Expected: browser code contains no refresh-token persistence.

- [ ] **Step 6: Browser-test login, refresh and logout**

Use MCP Playwright. Confirm refresh token is HttpOnly, reload can refresh the access session, and logout invalidates the session. Save screenshots under `screenshots/beta-f03-*`.

- [ ] **Step 7: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta F-03: move refresh sessions to HttpOnly cookies - completed"
```

## Task F-04: Add Minimal Public Runtime Config

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `backend/internal/model/config_public.go`
- Create: `backend/internal/handler/public_config.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/public_config_test.go`
- Create: `frontend/lib/public-config.ts`

- [ ] **Step 1: Write a failing allowlist test**

Assert `GET /api/v1/config/public` contains only:

```json
{
  "features": {
    "web_agent_enabled": false,
    "payment_enabled": false,
    "creator_support_enabled": false,
    "desktop_deploy_enabled": false
  },
  "captcha": {"provider": "", "site_key": ""},
  "client": {"download_enabled": false, "download_url": "", "latest_version": ""}
}
```

Assert response text does not contain `secret`, `access_key`, `api_key`, DSN, Redis password, HMAC, private keys, OSS CDN fields, internal TTLs or rate-limit internals.

- [ ] **Step 2: Implement DTO and public route**

Do not reuse the broad admin `model.PublicConfig`; create a dedicated DTO with explicit fields only. The backend source remains `cfg.Agent.WebAgentEnabled`, but the public DTO intentionally flattens it to `features.web_agent_enabled`. Add `features.desktop_deploy_enabled`, `captcha.provider`, `captcha.site_key`, `client.download_enabled`, `client.download_url`, and `client.latest_version` as explicit source config fields rather than overloading unrelated config structs. Keep secret captcha fields out of the public DTO.

F-04 owns the initial public-safe captcha/client config shape so the endpoint compiles before V-01. V-01 extends the same captcha config with secret/provider runtime validation; it must not add a second competing struct.

- [ ] **Step 3: Add frontend fetch helper**

Expose:

```ts
export async function getPublicConfig(): Promise<PublicRuntimeConfig>
```

Use a stable fallback with all optional capabilities disabled.

- [ ] **Step 4: Run tests**

```powershell
cd backend
go test ./internal/handler -run TestPublicConfig -v
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

- [ ] **Step 5: Commit**

```powershell
git add backend frontend docs/superpowers/plans progress.txt
git commit -m "Beta F-04: expose minimal public runtime config - completed"
```

## Task F-05: Make Chinese Keyword Search Reliable

**Files:**
- Create: `backend/migrations/049_search_trigram_fallback.sql`
- Create: `backend/testdata/search_seed.sql`
- Create: `backend/internal/repository/content_visibility.go`
- Modify: `backend/internal/repository/search_repo.go`
- Modify: `backend/internal/service/search_service.go`
- Modify: `backend/internal/service/agent_service.go`
- Create: `backend/internal/repository/search_repo_test.go`

- [ ] **Step 1: Write failing repository tests**

Seed published, hidden, deleted and banned-IP content with titles such as `春日穿搭指南` and tags such as `桌面改造`.

Create `backend/testdata/search_seed.sql` with the same deterministic cases for R-01 browser verification. The script must include the minimum required users, IPs, content rows and tags so it can run against a migrated disposable database without manual setup.

Cover:

```go
func TestSearchContentsMatchesChineseSubstring(t *testing.T) {}
func TestSearchContentsExcludesNonPublishedAndDeletedContent(t *testing.T) {}
func TestAgentAndKeywordSearchShareVisibilityRules(t *testing.T) {}
func TestSearchContentsCountsOnlyMatchedRows(t *testing.T) {}
func TestSearchSuggestionsDoNotLeakHiddenContentTitles(t *testing.T) {}
```

- [ ] **Step 2: Run tests and verify failure**

```powershell
cd backend
go test ./internal/repository -run TestSearchContents -v
```

- [ ] **Step 3: Add trigram-backed fallback**

Migration `047_pg_trgm_indexes.sql` already enables `pg_trgm` and creates `idx_content_items_title_trgm`. Add only the missing content-tag index:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_content_tags_tag_trgm
  ON content_tags USING gin (tag gin_trgm_ops);
```

Repository behavior:

- Keep `tsvector` ranking where it matches.
- Add parameterized title/tag substring matching with `ILIKE ?` and a bound value such as `"%"+query+"%"`; do not concatenate user input into SQL.
- Apply one reusable visibility scope to keyword search, suggestions and Agent result hydration: published and non-deleted content only, non-deleted and non-banned author, no banned IP, and normal `is_public`/author/follower visibility for the current viewer.
- Fix `total` count: the current implementation counts rows before applying the text query filter. After the fix, `total` must count only rows that match the full-text or trigram query AND the visibility filters. The count query and the row query must use the same WHERE clause; the only difference is that the count query omits `LIMIT`/`OFFSET`.
- Fix viewer context lookup to use `middleware.UserIDKey`, not a mismatched string literal.

- [ ] **Step 4: Verify query plan**

```powershell
psql $env:DB_DSN -c "EXPLAIN ANALYZE SELECT id FROM content_items WHERE title ILIKE '%穿搭%';"
psql $env:DB_DSN -c "EXPLAIN ANALYZE SELECT content_item_id FROM content_tags WHERE tag ILIKE '%桌面%';"
```

Expected: title and content-tag trigram indexes participate once dataset size is sufficient for the planner.

- [ ] **Step 5: Run backend checks**

```powershell
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 6: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta F-05: add reliable Chinese keyword search - completed"
```

## Task F-06: Route Every Download Through Authorization

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `backend/internal/service/oss_service.go`
- Modify: `backend/internal/handler/content.go`
- Create: `backend/internal/handler/content_download_test.go`
- Create: `frontend/components/content/DownloadButton.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/components/content/SheetMusicViewer.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Write failing API tests**

Cover published success plus unauthenticated, unverified-email, banned-user, low-reputation, non-published, banned-IP, `allow_copy=false`, missing attachment, attachment-from-another-content and OSS failure paths.

- [ ] **Step 2: Add attachment-specific authorized download**

Use:

```text
GET /api/v1/contents/:id/download?attachment_id=:attachmentId
```

Return:

```json
{"download_url":"https://...","expires_in":300}
```

Do not redirect. Generate a short-lived signed URL using configured TTL and increment the count asynchronously only after signing succeeds.

Rules:

- Add `oss.download_url_ttl_sec` with a default of `300` to `config.yaml` and `config.go`; do not leave the current one-hour hardcoded TTL. This field does not currently exist in the codebase and must be created.
- When `attachment_id` is supplied, verify that it belongs to the requested content.
- When `attachment_id` is omitted, select the unique attachment where `content_attachments.is_primary = true`. If no attachment has `is_primary = true`, or if multiple attachments have `is_primary = true`, return a validation error (`400 AMBIGUOUS_ATTACHMENT`) rather than selecting an arbitrary one. The caller must specify `attachment_id` explicitly in ambiguous cases.
- Preserve asynchronous per-content download counting after URL signing succeeds.

- [ ] **Step 3: Implement `DownloadButton`**

Before editing UI, run:

```powershell
rg -n "## Component: ContentDetail|## Component: SheetMusicViewer|## Component: DownloadButton" design/ui-spec.md
```

If `DownloadButton` has no dedicated spec, follow `ContentDetail`, `SheetMusicViewer` and `design/design-system.md`.

The component requests the API URL, then navigates the browser to `download_url`. It renders retry and `/feedback` entrypoints for signing failures. Replace direct download links in `ContentDetail.tsx` and download-only branches of `SheetMusicViewer.tsx`. Preview fetches may remain short-lived signed preview URLs only if the backend explicitly provides them as preview fields.

- [ ] **Step 4: Confirm direct OSS download links are removed**

```powershell
cd frontend
rg -n "href=\\{.*oss_url|download.*oss_url" components app
```

Expected: no user download CTA uses `oss_url`.

- [ ] **Step 5: Run checks and browser-test**

```powershell
cd ..\backend
go test ./...
go vet ./...
go build ./...
cd ..\frontend
npm run lint
npm run build
```

Use MCP Playwright to download an allowed attachment and verify a denied path. Save `screenshots/beta-f06-download.png`.

- [ ] **Step 6: Commit**

```powershell
git add backend frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta F-06: unify authorized content downloads - completed"
```
