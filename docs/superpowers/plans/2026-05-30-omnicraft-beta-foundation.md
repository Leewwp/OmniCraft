# OmniCraft Beta Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the shared security and reliability baseline required by both Beta tracks.

**Architecture:** Convert authentication from JWT-trusting fallback behavior to a fail-closed runtime status check, migrate browser sessions to server-managed cookies, expose a strict public-config allowlist, add Chinese-search fallback, and remove direct OSS download paths. Preserve existing handlers where possible and add focused tests around trust boundaries.

**Tech Stack:** Go/Gin/GORM/PostgreSQL/Redis, Next.js App Router, `next-intl`, MCP Playwright.

---

## File Structure

### Backend

- Modify: `backend/internal/middleware/auth.go` - fail-closed authenticated status resolution.
- Create: `backend/internal/middleware/interaction.go` - common verified-email, ban and reputation eligibility middleware.
- Modify: `backend/internal/handler/routes.go` - mount shared guards and public config route.
- Modify: `backend/internal/handler/auth.go` - cookie session responses.
- Modify: `backend/internal/service/auth_service.go` - strict refresh-token Redis behavior.
- Create: `backend/internal/handler/public_config.go` - minimal public runtime config response.
- Modify: `backend/internal/model/config_public.go` - split public DTO from admin DTO.
- Modify: `backend/config/config.go` and `backend/config.yaml` - desktop feature flag and public metadata.
- Modify: `backend/internal/repository/search_repo.go` - trigram fallback and shared visibility filters.
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

- [ ] **Step 1: Record current git state**

Run:

```powershell
git status --short --branch
git rev-parse HEAD
node -e "const fs=require('fs'); const j=JSON.parse(fs.readFileSync('task.json','utf8')); console.log(j.tasks.filter(t=>t.id>=156&&t.id<=168).map(t=>`${t.id}\t${t.passes}\t${t.title}`).join('\n'))"
```

Expected: SHA, dirty files, and Tasks 156-168 are captured without interpreting terminal mojibake as file corruption.

- [ ] **Step 2: Run the historical quality gates**

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

- [ ] **Step 3: Validate local infrastructure configuration**

Run:

```powershell
cd ..\
docker compose config
```

Expected: compose configuration renders successfully. Record unavailable external services as blockers, not as passing checks.

- [ ] **Step 4: Write the baseline report**

Document: SHA, dirty files, command results, external dependencies not configured, and concrete follow-up task IDs.

- [ ] **Step 5: Commit**

```powershell
git add docs/review/beta-baseline-2026-05-30.md docs/superpowers/plans progress.txt
git commit -m "Beta F-01: capture public beta baseline - completed"
```

## Task F-02: Fail Closed And Centralize Interaction Eligibility

**Files:**
- Modify: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/interaction.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/middleware/auth_test.go`
- Create: `backend/internal/middleware/interaction_test.go`

- [ ] **Step 1: Write failing middleware tests**

Cover:

```go
func TestAuthRequiredRejectsWhenRedisAndDBCannotConfirmStatus(t *testing.T) {}
func TestAuthRequiredUsesFreshDBRoleInsteadOfJWTClaim(t *testing.T) {}
func TestOptionalAuthDowngradesToAnonymousWhenStatusCannotBeConfirmed(t *testing.T) {}
func TestInteractionRequiredRejectsUnverifiedEmail(t *testing.T) {}
func TestInteractionRequiredRejectsLowReputation(t *testing.T) {}
```

Expected error envelope for unknown runtime status:

```json
{"code":"AUTH_STATUS_UNAVAILABLE","message":"account status is temporarily unavailable"}
```

- [ ] **Step 2: Run the tests and verify failure**

Run:

```powershell
cd backend
go test ./internal/middleware -run "TestAuthRequired|TestInteractionRequired" -v
```

Expected: `TestAuthRequiredRejectsWhenRedisAndDBCannotConfirmStatus` fails because current code falls back to JWT claims.

- [ ] **Step 3: Implement strict runtime status resolution**

Use an explicit resolver:

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

Rules:

- Redis cache may accelerate status reads.
- A cache miss must query PostgreSQL.
- Redis failure plus PostgreSQL failure returns `AUTH_STATUS_UNAVAILABLE`.
- Deleted and banned users never become authenticated.
- Role comes from runtime status, never stale JWT claims.
- Use `cfg.Cache.UserStatusTTL`, not a hardcoded five minutes.
- `OptionalAuth` downgrades to anonymous when status cannot be confirmed; it must not retain claim-derived permissions.
- Invalidate or refresh cached runtime status when email verification, reputation, ban state or role changes.

- [ ] **Step 4: Add a shared interaction guard**

Create middleware configurable by required capabilities:

```go
type InteractionPolicy struct {
    RequireVerifiedEmail bool
    RequireReputation    bool
}
```

Mount it on publish, edit/delete content, comments, reactions, reports, favorites, PR submission/accept/reject/merge, judge exams/votes/reason interactions, follows, private messages, downloads, Agent endpoints and future deploy grants. Use `cfg.Reputation.MinScoreForInteraction`.

- [ ] **Step 5: Run focused and backend-wide tests**

```powershell
go test ./internal/middleware ./internal/handler ./internal/service -v
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 6: Commit**

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
```

Expected refresh cookie attributes in production: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, no `Domain`, configured TTL. Use a `__Host-` cookie name in production.

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
- Refresh and logout validate CSRF or trusted Origin because they consume cookies.

- [ ] **Step 4: Remove frontend refresh-token storage**

Delete `REFRESH_TOKEN`, `getRefreshToken()` and every `localStorage.setItem("refresh_token", ...)`. Replace access-token `localStorage` and JavaScript-written cookies with an in-memory module variable. On page reload, call refresh using the HttpOnly cookie to obtain a new short-lived access token. `frontend/proxy.ts` may inspect only the presence of the HttpOnly refresh cookie as a routing hint; it must not decode or trust stale access claims.

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
  "client": {"download_enabled": false, "download_url": "", "latest_version": ""},
  "oss": {"cdn_base": ""}
}
```

Assert response text does not contain `secret`, `access_key`, `api_key`, DSN, Redis password, HMAC or private keys.

- [ ] **Step 2: Implement DTO and public route**

Do not reuse the broad admin `model.PublicConfig`; create a dedicated DTO with explicit fields only.

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
- Modify: `backend/internal/repository/search_repo.go`
- Modify: `backend/internal/service/search_service.go`
- Create: `backend/internal/repository/search_repo_test.go`

- [ ] **Step 1: Write failing repository tests**

Seed published, hidden, deleted and banned-IP content with titles such as `春日穿搭指南` and tags such as `桌面改造`.

Cover:

```go
func TestSearchContentsMatchesChineseSubstring(t *testing.T) {}
func TestSearchContentsExcludesNonPublishedAndDeletedContent(t *testing.T) {}
func TestAgentAndKeywordSearchShareVisibilityRules(t *testing.T) {}
```

- [ ] **Step 2: Run tests and verify failure**

```powershell
cd backend
go test ./internal/repository -run TestSearchContents -v
```

- [ ] **Step 3: Add trigram-backed fallback**

Migration:

```sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_content_items_title_trgm
  ON content_items USING gin (title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_content_tags_tag_trgm
  ON content_tags USING gin (tag gin_trgm_ops);
```

Repository behavior:

- Keep `tsvector` ranking where it matches.
- Add parameterized `ILIKE '%' || ? || '%'` fallback for Chinese title and tag substring matching.
- Apply one reusable visibility scope to keyword and Agent result hydration: published content only, non-deleted author, non-banned IP, normal content visibility rules.
- Fix viewer context lookup to use `middleware.UserIDKey`, not a mismatched string literal.

- [ ] **Step 4: Verify query plan**

```powershell
psql $env:DB_DSN -c "EXPLAIN ANALYZE SELECT id FROM content_items WHERE title ILIKE '%穿搭%';"
```

Expected: trigram index participates once dataset size is sufficient for the planner.

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

Cover published success plus unauthenticated, unverified-email, banned-user, low-reputation, non-published, `allow_copy=false`, missing attachment and OSS failure paths.

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

- [ ] **Step 3: Implement `DownloadButton`**

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
