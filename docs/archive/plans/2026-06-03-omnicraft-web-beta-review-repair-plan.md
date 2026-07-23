# OmniCraft Web Beta Review Repairs Implementation Plan

> **归档说明**：全部 7 个 Task 已于 2026-06-03 ~ 06-07 执行完毕并经最终集成复验（progress.txt 有逐 Task 记录），checkbox 当时未回填。2026-07-23 文档瘦身归档，文件不再维护。


> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair the verified Web Beta review defects so the public Web Beta can be revalidated without known security, auth, feedback, search/download, Agent, or Admin audit blockers.

**Architecture:** This plan treats the review findings as a Beta-mode repair queue, not as new historical `task.json` work. Repairs are grouped by subsystem so each batch can be implemented, tested, reviewed, and committed independently while preserving the existing Next.js + Go/Gin/GORM + PostgreSQL + Redis architecture. Security fixes are first, cross-stack functional regressions second, and the larger Admin audit transaction refactor is isolated because it touches many mutation paths.

**Tech Stack:** Go 1.22+, Gin, GORM, PostgreSQL 16, Redis 7, Next.js App Router, TypeScript strict mode, Tailwind CSS, next-intl, MCP Playwright.

---

## Scope And Source Of Truth

Use AGENTS.md **Mode A: 双轨 Beta 计划集**. Do not modify `task.json`.

Primary review inputs:

- `docs/review/web-beta-review-summary.md`
- `docs/review/web-beta-review-00-release-evidence.md`
- `docs/review/web-beta-review-01-authz-runtime.md`
- `docs/review/web-beta-review-02-session-config-desktop-off.md`
- `docs/review/web-beta-review-03-verification.md`
- `docs/review/web-beta-review-04-feedback-public-pages.md`
- `docs/review/web-beta-review-05-search-download.md`
- `docs/review/web-beta-review-06-admin-audit.md`
- `docs/review/web-beta-review-07-admin-console.md`
- `docs/review/web-beta-review-08-agent-entrypoints.md`
- `docs/review/web-beta-review-09-cross-stack-e2e.md`

Verified corrections to the review summary:

- Anonymous feedback is broken, but the most likely root cause is not `NULL user_id`. `OptionalAuth` sets `userID=0`; `FeedbackHandler` treats key existence as authenticated and writes `user_id=0`, which violates the foreign key.
- Browser screenshots are not wholly missing. The original R-01 screenshot paths may be incomplete, but `screenshots/review-web-beta/*` contains later review screenshots.
- The anonymous AI search toggle finding in Review 08 is internally inconsistent. Re-test before changing UI.
- `ContentVisibilityWhere` string interpolation is a project rule violation and maintainability risk; do not overstate it as direct SQL injection because `viewerID` is an int64 from auth context.

## File Map

### Auth, Session, Captcha, Release Safety

- `backend/internal/middleware/auth.go`
  - Fail closed when Redis blacklist state cannot be checked for protected requests.
  - Keep `OptionalAuth` anonymous degradation safe.
- `backend/internal/middleware/auth_test.go`
  - Add Redis-error blacklist tests for `AuthRequired`.
- `backend/internal/middleware/cors.go`
  - Add localhost CORS origins only outside release mode.
- `backend/internal/middleware/cors_test.go`
  - Add release/debug origin allowlist tests.
- `backend/internal/middleware/interaction.go`
  - Fail closed for publish freeze checks when Redis is unavailable for publish-only routes.
- `backend/internal/middleware/interaction_test.go`
  - Add publish-freeze Redis unavailable test.
- `backend/internal/handler/auth.go`
  - Inject and verify captcha in register, forgot-password, resend, and login threshold paths.
  - Remove refresh-token JSON body fallback from refresh/logout.
  - Enforce resend cooldown response.
  - Return access token + refresh cookie after successful reset if the existing spec still requires auto-login.
- `backend/internal/handler/auth_test.go`
  - Add handler tests for captcha, cooldown, cookie-only refresh/logout, and reset auto-login.
- `backend/internal/container/container.go`
  - Pass captcha verifier into `AuthHandler`.
- `backend/internal/pkg/response/safe_error.go`
  - Default unknown errors to safe fallback.
- `backend/internal/pkg/response/safe_error_test.go`
  - Add unknown internal error does-not-leak test.
- `backend/cmd/server/main.go`
  - Call `cfg.ValidateRelease()` before connecting external services.
- `backend/config/config.go`
  - Keep `ValidateRelease()` as the policy source; add tests if no config test file exists.
- `backend/config/config_test.go`
  - Add release-mode rejection tests for `captcha.provider=bypass` and `smtp.mode=logger`.

### Verification Runtime Cache And Token Atomicity

- `backend/internal/service/verification_service.go`
  - Invalidate runtime status cache after email verification.
  - Make verification/reset token consumption atomic.
  - Handle `rdb == nil` explicitly for token flows.
- `backend/internal/service/verification_service_test.go`
  - Add cache invalidation and double-submit token tests.

### Feedback And Reports

- `backend/internal/handler/feedback.go`
  - Treat `userID=0` as anonymous.
  - Return stable 400/500 responses without raw errors.
- `backend/internal/service/feedback_service.go`
  - Consume screenshot upload grants.
  - Bind attachment OSS keys to grants.
  - Return actual upload URL only after OSS integration exists, or provide a backend upload endpoint; do not leave frontend PUT to an unmounted path.
  - Send notifications/emails for Admin public replies and close/reopen where required.
- `backend/internal/repository/feedback_repo.go`
  - Add transaction variants if needed for attachment grant consumption and ticket creation.
- `backend/internal/handler/feedback_test.go`
  - Add anonymous feedback regression for no-auth request.
- `backend/internal/service/feedback_service_test.go`
  - Add upload grant single-use and admin-notification tests.
- `backend/internal/handler/admin_feedback.go`
  - Wire notification/email-visible failure behavior if implemented at handler level.
  - Keep audit handling aligned with the Admin audit transaction task.
- `backend/internal/repository/search_repo.go`
  - Stop updating a nonexistent `action_taken` column unless migration adds it.
- `backend/migrations/055_web_beta_review_repairs.sql`
  - Add `reports.action_taken` if keeping the API contract.
  - Fix `feedback_tickets.status` check to include `reopened`, or change service/UI to avoid `reopened`.
  - Add any corrective migration for previously rewritten 041/042 only if target environments may have run the old migration.

### Search, Visibility, Download

- `backend/internal/repository/content_visibility.go`
  - Add IP ban exclusion to shared content visibility.
  - Replace raw `fmt.Sprintf` where clause with clause + args helper.
- `backend/internal/repository/search_repo.go`
  - Use parameterized visibility clause args in raw SQL paths.
  - Reuse shared visibility logic for suggestions where practical.
- `backend/internal/repository/search_repo_test.go`
  - Add DB-backed Chinese substring and visibility tests using disposable DB/test fixtures.
- `backend/internal/handler/content.go`
  - Apply author deleted/banned and IP banned checks in download path.
- `backend/internal/handler/content_download_test.go`
  - Add download rejection tests for author banned/deleted and IP banned content.
- `frontend/components/content/SheetMusicViewer.tsx`
  - Replace direct PDF download `<a href={ossUrl}>` with `DownloadButton`.
- `frontend/components/content/SheetMusicViewer.test.tsx`
  - Add component test if the project already has frontend component test infrastructure; otherwise cover by Playwright in the verification batch.

### Agent Entrypoints And Publish Assist

- `backend/internal/service/agent_service.go`
  - Validate `UploadAssist` LLM output before returning.
  - Apply shared visibility semantics in `NLSearch`.
- `backend/internal/handler/agent.go`
  - Add `WebAgentEnabled` checks for conversation list/detail endpoints.
- `backend/internal/service/agent_service_test.go`
  - Add upload-assist validation tests and NLSearch visibility tests.
- `frontend/components/publish/PublishForm.tsx`
  - Require warning acknowledgement before applying compliance-affecting suggestions.
  - Block upload-assist apply and submit when compliance result is `violation`.
  - Validate suggested title/description lengths before applying.
- `frontend/messages/zh.json`
- `frontend/messages/en.json`
  - Add any new UI text through next-intl keys.

### Admin Audit Integrity

- `backend/internal/service/admin_audit_service.go`
  - Keep `RecordTx` as the only success-audit path for local DB sensitive mutations.
  - Add best-effort failed-attempt recording helper if useful.
- `backend/internal/repository/admin_audit_repo.go`
  - Keep `CreateTx`.
- `backend/internal/handler/admin.go`
  - Refactor sensitive local DB mutations to wrap domain write + success audit in one `db.Transaction`.
  - Add sanitized failed-attempt audit rows for validation/domain failures where the admin identity is known.
- `backend/internal/handler/admin_feedback.go`
  - Same transaction approach for feedback patch/reply.
- `backend/internal/handler/category.go`
  - Use `middleware.GetUserID(c)`.
  - Add `TraceID`.
  - Use transaction + `RecordTx`.
- `backend/internal/handler/judge.go`
  - Use `middleware.GetUserID(c)`.
  - Add `TraceID`.
  - Use transaction + `RecordTx`.
- Repository/service files touched by admin handlers
  - Add `Tx` variants only where required. Prefer small wrappers over broad repository rewrites.
- `backend/internal/handler/admin_audit_transaction_test.go`
  - Add rollback-on-audit-failure tests for one representative endpoint per mutation pattern.

## Success Criteria

- All P0 findings are fixed and covered by tests.
- Cross-stack E2E failures from Review 09 are repaired:
  - anonymous feedback works;
  - resend cooldown returns 429 or the agreed stable rate-limit response;
  - report resolution works and writes audit rows.
- Auth/session/captcha flows obey cookie-only refresh, release-mode config rejection, and captcha requirements.
- Admin sensitive mutations cannot commit without a success audit row.
- Shared visibility rules exclude unpublished, soft-deleted, deleted-author, banned-author, banned-IP, and unauthorized private content.
- All user download CTAs go through `GET /api/v1/contents/:id/download?attachment_id=...`.
- Repository-wide gates pass:
  - backend: `go test ./...`
  - backend: `go vet ./...`
  - backend: `go build ./...`
  - frontend: `npm run lint`
  - frontend: `npm run build`
  - Playwright browser/API evidence for repaired UI and E2E flows.

## Execution Rules

- Use one repair batch per branch/commit. Suggested branch prefix: `codex/beta/repair-<batch-name>`.
- Do not update Beta roadmap checkboxes until the corresponding repair batch has tests, review, and verification.
- Do not modify `task.json`.
- Use TDD for each behavior change: failing test, minimal implementation, passing test, refactor.
- Use exact staging only.
- If production-only infrastructure is missing, record it as blocked evidence; do not fake real SMTP/CAPTCHA/OSS success.

---

## Task 1: Auth, Captcha, Release Guard, And Cookie-Only Session

**Files:**

- Modify: `backend/internal/middleware/auth.go`
- Modify: `backend/internal/middleware/cors.go`
- Modify: `backend/internal/middleware/interaction.go`
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/internal/container/container.go`
- Modify: `backend/internal/pkg/response/safe_error.go`
- Modify: `backend/cmd/server/main.go`
- Test: `backend/internal/middleware/auth_test.go`
- Test: `backend/internal/middleware/cors_test.go`
- Test: `backend/internal/middleware/interaction_test.go`
- Test: `backend/internal/handler/auth_test.go`
- Test: `backend/internal/pkg/response/safe_error_test.go`
- Test: `backend/config/config_test.go`

### TDD Steps

- [ ] **Step 1: Add failing test for Redis blacklist fail-closed**

Add a test in `backend/internal/middleware/auth_test.go` that uses a Redis client pointed at an unavailable address or a failing Redis mock, sends a valid access token to an `AuthRequired` route, and expects:

```go
assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
assert.Contains(t, rec.Body.String(), "AUTH_STATUS_UNAVAILABLE")
```

Expected initial result: FAIL because `AuthRequired` ignores Redis errors from `rdb.Get()`.

- [ ] **Step 2: Implement blacklist fail-closed**

In `AuthRequired`, after `rdb.Get(...).Result()`:

```go
if redisErr == nil && val == "1" {
    c.JSON(401, gin.H{"code": "UNAUTHORIZED", "message": "token has been revoked"})
    c.Abort()
    return
}
if redisErr != nil && redisErr != redis.Nil {
    c.JSON(503, gin.H{"code": "AUTH_STATUS_UNAVAILABLE", "message": "account status is temporarily unavailable"})
    c.Abort()
    return
}
```

Keep `OptionalAuth` anonymous-degrading on Redis errors; do not fail public read routes.

- [ ] **Step 3: Add failing tests for auth captcha**

In `backend/internal/handler/auth_test.go`, create a fake `captcha.CaptchaVerifier` that records calls and can return an error. Cover:

- `Register` requires and verifies captcha.
- `ForgotPassword` requires and verifies captcha.
- `ResendVerification` requires and verifies captcha.
- Login after threshold requires captcha.

Expected initial result: FAIL because `AuthHandler` has no captcha verifier.

- [ ] **Step 4: Inject captcha verifier into AuthHandler**

Modify `AuthHandler`:

```go
type AuthHandler struct {
    authService *service.AuthService
    verificationService *service.VerificationService
    userRepo *repository.UserRepository
    captchaVerifier captcha.CaptchaVerifier
    rdb *redis.Client
    cfg *config.Config
}
```

Update constructor and `container.NewContainer()` so the existing `captchaVerifier` is passed to auth handler construction. Remove `_ = captchaVerifier`.

- [ ] **Step 5: Verify captcha in auth handlers**

Add helper:

```go
func (h *AuthHandler) verifyCaptcha(c *gin.Context, token string) bool {
    if token == "" {
        c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_REQUIRED", "message": "captcha verification required"})
        return false
    }
    if h.captchaVerifier != nil {
        if err := h.captchaVerifier.Verify(c.Request.Context(), token, ""); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"code": "CAPTCHA_FAILED", "message": "captcha verification failed"})
            return false
        }
    }
    return true
}
```

Call it in register, forgot-password, resend. For login threshold, use Redis key `captcha:login-failures:<sha256(normalized-email)>`.

- [ ] **Step 6: Implement login captcha threshold**

On invalid credentials:

- increment failure counter;
- set TTL, for example `verification.login_captcha_threshold` cooldown window or a new documented window if already present;
- once failures `>= LoginCaptchaThreshold`, require captcha on subsequent login attempts;
- clear counter on successful login.

Use SHA-256 of normalized email in Redis key names. Do not store raw email in Redis key names.

- [ ] **Step 7: Add failing test for release validation startup**

Add `backend/config/config_test.go`:

```go
func TestValidateReleaseRejectsBypassCaptchaAndLoggerSMTP(t *testing.T) {
    cfg := &Config{}
    cfg.Server.Mode = "release"
    cfg.Captcha.Provider = "bypass"
    cfg.SMTP.Mode = "logger"
    require.Error(t, cfg.ValidateRelease())
}
```

Add a `main`-adjacent test only if the project has an existing pattern for testing startup; otherwise code review plus config test is acceptable.

- [ ] **Step 8: Call `ValidateRelease()` at startup**

In `backend/cmd/server/main.go`, immediately after `cfg := config.Load()`:

```go
if err := cfg.ValidateRelease(); err != nil {
    slog.Error("invalid release configuration", "error", err)
    os.Exit(1)
}
```

- [ ] **Step 9: Add and fix CORS release-mode test**

Test `middleware.CORS` with `cfg.Server.Mode = "release"` and `AllowedOrigins = []string{"https://app.leeppp.online"}`:

- Origin `https://app.leeppp.online` gets `Access-Control-Allow-Origin`.
- Origin `http://localhost:3000` does not.

Implementation: wrap localhost injection in `if cfg.Server.Mode != "release"`.

- [ ] **Step 10: Add and fix cookie-only refresh/logout tests**

Tests:

- `Refresh` without refresh cookie but with JSON `refresh_token` returns 401.
- `Logout` ignores JSON body refresh token and only clears cookie/access-token revocation paths.

Implementation: remove `ShouldBindJSON` fallback from `Refresh` and `Logout`.

- [ ] **Step 11: Add and fix SafeError fallback test**

Test:

```go
msg := response.SafeErrorMsg(errors.New("dial tcp 10.0.0.1:5432: i/o timeout"), "safe")
require.Equal(t, "safe", msg)
```

Implementation: replace final `return err.Error()` with `return fallback`.

- [ ] **Step 12: Add and fix publish-freeze Redis unavailable test**

For routes using `RequireNoPublishFreeze`, when `rdb == nil`, expect `503 AUTH_STATUS_UNAVAILABLE`.

Implementation may require changing `isPublishFrozen` to return `(bool, error)` so the middleware can distinguish frozen from unavailable.

- [ ] **Step 13: Run focused backend tests**

Run from `backend/`:

```powershell
go test ./internal/middleware ./internal/handler ./internal/pkg/response ./config -v
```

Expected: PASS.

- [ ] **Step 14: Run backend gates**

Run from `backend/`:

```powershell
go test ./...
go vet ./...
go build ./...
```

Expected: all PASS.

- [ ] **Step 15: Commit Task 1**

```powershell
git add backend/internal/middleware/auth.go backend/internal/middleware/auth_test.go backend/internal/middleware/cors.go backend/internal/middleware/cors_test.go backend/internal/middleware/interaction.go backend/internal/middleware/interaction_test.go backend/internal/handler/auth.go backend/internal/handler/auth_test.go backend/internal/container/container.go backend/internal/pkg/response/safe_error.go backend/internal/pkg/response/safe_error_test.go backend/config/config.go backend/config/config_test.go backend/cmd/server/main.go
git commit -m "Beta repair: harden auth session and release gates"
```

---

## Task 2: Verification Cache, Token Atomicity, Resend Cooldown, And Reset Auto-Login

**Files:**

- Modify: `backend/internal/service/verification_service.go`
- Modify: `backend/internal/handler/auth.go`
- Test: `backend/internal/service/verification_service_test.go`
- Test: `backend/internal/handler/auth_test.go`

### TDD Steps

- [ ] **Step 1: Add failing test for email verification cache invalidation**

Use a Redis-backed or fake cache test:

- seed runtime status for user with `EmailVerifiedAt == nil`;
- call `VerifyEmail`;
- assert runtime status cache key is invalidated or the next status resolution sees verified email.

Expected initial result: FAIL because `VerifyEmail` updates DB but does not invalidate cache.

- [ ] **Step 2: Invalidate status cache after verification**

After successful `email_verified_at` update:

```go
middleware.InvalidateUserStatusCache(s.rdb, userID)
```

If importing middleware into service would create an undesirable dependency, move invalidation helper into `service` package and update middleware to call that helper.

- [ ] **Step 3: Add failing double-submit token tests**

For email verification and password reset:

- create token;
- call verification/reset concurrently twice;
- assert exactly one succeeds and the second returns `ErrInvalidToken`.

Expected initial result: flaky/failing because current flow is GET then DEL.

- [ ] **Step 4: Make token consumption atomic**

Use Redis Lua or `WATCH` transaction. Preferred Lua script:

```lua
local digestValue = redis.call("GET", KEYS[1])
if not digestValue then return {0, ""} end
local storedDigest = redis.call("GET", KEYS[2])
if storedDigest ~= ARGV[1] then return {0, ""} end
redis.call("DEL", KEYS[1])
redis.call("DEL", KEYS[2])
return {1, digestValue}
```

Apply to both `verify:email:*` and `reset:password:*`.

- [ ] **Step 5: Add failing resend cooldown handler test**

Call `ResendVerification` twice immediately for same unverified user.

Expected:

```go
assert.Equal(t, http.StatusTooManyRequests, second.Code)
```

Expected initial result: FAIL because handler discards `ErrResendCooldown`.

- [ ] **Step 6: Handle `ErrResendCooldown` in handler**

In `ResendVerification`, replace `_ = h.verificationService.SendVerification(...)` with error handling:

```go
if err := h.verificationService.SendVerification(...); err != nil {
    if errors.Is(err, service.ErrResendCooldown) {
        c.JSON(http.StatusTooManyRequests, gin.H{"code": "RESEND_COOLDOWN", "message": "please wait before requesting another verification email"})
        return
    }
    ...
}
```

Keep unknown-email and already-verified responses uniform.

- [ ] **Step 7: Decide and test reset auto-login contract**

The Beta plan and UI spec require reset success to establish a session. Add failing handler test:

- call `ResetPassword`;
- expect `Set-Cookie: refresh_token` or `__Host-refresh_token`;
- expect JSON includes `tokens.access_token`;
- expect no `refresh_token` in JSON.

Implementation options:

- Return user ID from `VerificationService.ResetPassword`, then call an AuthService session creation helper.
- Or add a service method on AuthService to issue tokens for a known user after password reset.

Do not return refresh token in JSON.

- [ ] **Step 8: Run tests**

Run from `backend/`:

```powershell
go test ./internal/service -run "TestVerification|TestResetPassword" -v
go test ./internal/handler -run "TestAuth|TestResend|TestReset" -v
go test ./...
go vet ./...
go build ./...
```

Expected: all PASS.

- [ ] **Step 9: Commit Task 2**

```powershell
git add backend/internal/service/verification_service.go backend/internal/service/verification_service_test.go backend/internal/handler/auth.go backend/internal/handler/auth_test.go
git commit -m "Beta repair: fix verification token lifecycle"
```

---

## Task 3: Feedback, Upload Grants, Notifications, And Report Resolution

**Files:**

- Modify: `backend/internal/handler/feedback.go`
- Modify: `backend/internal/service/feedback_service.go`
- Modify: `backend/internal/repository/feedback_repo.go`
- Modify: `backend/internal/handler/admin_feedback.go`
- Modify: `backend/internal/repository/search_repo.go`
- Create: `backend/migrations/055_web_beta_review_repairs.sql`
- Test: `backend/internal/handler/feedback_test.go`
- Test: `backend/internal/service/feedback_service_test.go`
- Test: `backend/internal/handler/admin_feedback_test.go`
- Test: `backend/internal/repository/search_repo_test.go`

### TDD Steps

- [ ] **Step 1: Add failing anonymous feedback test**

Mount `/api/v1/feedback` with `OptionalAuth`. Send request without `Authorization`.

Expected:

```go
assert.Equal(t, http.StatusCreated, rec.Code)
assert.Contains(t, rec.Body.String(), `"user_id":`)
assert.NotContains(t, rec.Body.String(), `"user_id":0`)
```

Expected initial result: FAIL with 500 or foreign key failure.

- [ ] **Step 2: Fix anonymous user detection**

In `FeedbackHandler.SubmitTicket` and `PresignUpload`, replace:

```go
if exists {
    id := userID.(int64)
    uid = &id
}
```

with:

```go
if exists {
    if id, ok := userID.(int64); ok && id > 0 {
        uid = &id
    }
}
```

- [ ] **Step 3: Add failing report resolution schema test**

Test `UpdateReportStatus(id, "resolved", "upheld")` against a migrated disposable DB.

Expected initial result: FAIL because `reports.action_taken` does not exist.

- [ ] **Step 4: Add migration for report action**

Create `backend/migrations/055_web_beta_review_repairs.sql`:

```sql
ALTER TABLE reports
  ADD COLUMN IF NOT EXISTS action_taken TEXT;
```

If report resolution needs `resolved_at` / `resolved_by`, add those in the same migration only if handlers will write them now. Avoid unused columns.

- [ ] **Step 5: Fix feedback status schema mismatch**

Current migration allows `open`, `in_progress`, `resolved`, `closed`; service allows `reopened`. Choose one:

Preferred: update migration check to include `reopened`:

```sql
ALTER TABLE feedback_tickets
  DROP CONSTRAINT IF EXISTS feedback_tickets_status_check;

ALTER TABLE feedback_tickets
  ADD CONSTRAINT feedback_tickets_status_check
  CHECK (status IN ('open', 'in_progress', 'resolved', 'closed', 'reopened'));
```

Add a test for `PatchTicket(... Status: "reopened")`.

- [ ] **Step 6: Add failing upload grant single-use test**

Flow:

- call `PresignUpload`;
- submit ticket with the returned grant/key;
- submit another ticket with same grant/key;
- expect second submit rejects.

Expected initial result: FAIL because grant is not consumed and grant ID is not bound to submission.

- [ ] **Step 7: Bind attachment submissions to grants**

Change request DTO to include attachment grants:

```go
type FeedbackAttachmentGrantInput struct {
    GrantID string `json:"grant_id"`
    OSSKey string `json:"oss_key"`
}
```

For each attachment:

- read Redis `feedback:upload_grant:<grantID>`;
- compare exact OSS key;
- delete key atomically before attaching;
- reject mismatch/reuse.

Use Lua or Redis transaction for consume-and-compare.

- [ ] **Step 8: Fix screenshot upload URL contract**

Choose the smallest real implementation:

Option A, if OSS signing exists:

- inject OSS service into FeedbackService;
- return `upload_url`, `grant_id`, `oss_key`;
- frontend PUTs to `upload_url`.

Option B, if OSS is still unavailable for local Beta:

- add a backend `PUT /api/v1/feedback/attachments/staging/:grant_id` endpoint that stores locally only in dev;
- mark production OSS as blocked.

Do not leave frontend uploading to `http://localhost:8080/feedback-staging/...` without a route.

- [ ] **Step 9: Add failing admin reply/close notification tests**

For logged-in ticket:

- admin public reply creates notification for ticket owner;
- admin close creates notification.

For anonymous ticket with `contact_email`:

- mail sender is called;
- mail failure returns a visible retryable admin response, not silent success.

- [ ] **Step 10: Inject notification and mail dependencies**

Update `FeedbackService` constructor to receive:

```go
notificationSvc *service.NotificationService
mailSender mail.MailSender
```

or define a narrow interface to avoid import cycles.

Send notifications/emails only for public replies and close/reopen actions, not internal notes.

- [ ] **Step 11: Run tests**

Run from `backend/`:

```powershell
go test ./internal/handler -run "TestFeedback|TestAdminFeedback|TestReport" -v
go test ./internal/service -run TestFeedback -v
go test ./internal/repository -run TestReport -v
go test ./...
go vet ./...
go build ./...
```

Expected: all PASS.

- [ ] **Step 12: Commit Task 3**

```powershell
git add backend/internal/handler/feedback.go backend/internal/handler/feedback_test.go backend/internal/service/feedback_service.go backend/internal/service/feedback_service_test.go backend/internal/repository/feedback_repo.go backend/internal/handler/admin_feedback.go backend/internal/handler/admin_feedback_test.go backend/internal/repository/search_repo.go backend/internal/repository/search_repo_test.go backend/migrations/055_web_beta_review_repairs.sql
git commit -m "Beta repair: restore feedback and report workflows"
```

---

## Task 4: Search Visibility, Download Authorization, And Sheet Music CTA

**Files:**

- Modify: `backend/internal/repository/content_visibility.go`
- Modify: `backend/internal/repository/search_repo.go`
- Modify: `backend/internal/handler/content.go`
- Modify: `frontend/components/content/SheetMusicViewer.tsx`
- Test: `backend/internal/repository/search_repo_test.go`
- Test: `backend/internal/handler/content_download_test.go`

### TDD Steps

- [ ] **Step 1: Add failing visibility tests**

DB-backed tests should seed:

- published public normal content;
- soft-deleted content;
- content by banned user;
- content by deleted user;
- content linked to banned IP;
- private content by another user;
- private content by viewer.

Assert keyword search, suggestion search, and raw visibility helper exclude the hidden cases.

Expected initial result: FAIL for banned-IP cases.

- [ ] **Step 2: Add IP ban exclusion to shared visibility**

In `ApplyContentVisibilityScope`:

```go
Where("content_items.ip_id IS NULL OR content_items.ip_id NOT IN (SELECT id FROM ips WHERE status = ?)", "banned")
```

Ensure `ContentItem` actually uses `ip_id`; if the column has a different name, adapt to the model.

- [ ] **Step 3: Replace `ContentVisibilityWhere` with parameterized helper**

Replace:

```go
func ContentVisibilityWhere(viewerID int64) string
```

with:

```go
func ContentVisibilitySQL(viewerID int64) (string, []interface{})
```

Return SQL with `?` placeholders and args including viewerID and banned status.

Update `searchContentsWithQuery` to combine args in correct order.

- [ ] **Step 4: Add failing download visibility tests**

For `DownloadContent`, assert 403 when:

- content author is banned;
- content author is soft-deleted;
- content IP is banned.

Expected initial result: FAIL because handler checks only caller/content/allow_copy/attachment.

- [ ] **Step 5: Apply visibility checks in download path**

Preferred implementation:

- fetch content through a repository method that applies shared visibility scope for the caller;
- or explicitly preload/check author and IP status before signing.

Return stable errors:

```go
c.JSON(http.StatusForbidden, gin.H{"code": "CONTENT_UNAVAILABLE", "message": "content is unavailable"})
```

- [ ] **Step 6: Replace direct PDF download link**

Change `PDFViewer` signature:

```tsx
function PDFViewer({ ossUrl, contentId, attachmentId }: { ossUrl: string; contentId: number; attachmentId: number })
```

Replace:

```tsx
<a href={ossUrl}>...</a>
```

with:

```tsx
<DownloadButton contentId={contentId} attachmentId={attachmentId} contentType="sheet_music" size="sm" />
```

Call:

```tsx
<PDFViewer ossUrl={renderable.oss_url} contentId={contentId} attachmentId={renderable.id} />
```

Keep `<embed src={ossUrl}>` only as preview. If preview URLs are long-lived raw OSS URLs, schedule a separate preview authorization task.

- [ ] **Step 7: Run tests and frontend gates**

Run:

```powershell
cd backend
go test ./internal/repository -run "TestSearch|TestContentVisibility" -v
go test ./internal/handler -run TestDownload -v
go test ./...
go vet ./...
go build ./...
```

Then:

```powershell
cd frontend
npm run lint
npm run build
```

Expected: all PASS.

- [ ] **Step 8: Commit Task 4**

```powershell
git add backend/internal/repository/content_visibility.go backend/internal/repository/search_repo.go backend/internal/repository/search_repo_test.go backend/internal/handler/content.go backend/internal/handler/content_download_test.go frontend/components/content/SheetMusicViewer.tsx
git commit -m "Beta repair: unify content visibility and downloads"
```

---

## Task 5: Agent Output Validation And Feature-Flag Consistency

**Files:**

- Modify: `backend/internal/service/agent_service.go`
- Modify: `backend/internal/handler/agent.go`
- Modify: `frontend/components/publish/PublishForm.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`
- Test: `backend/internal/service/agent_service_test.go`
- Test: `backend/internal/handler/agent_test.go`

### TDD Steps

- [ ] **Step 1: Add failing UploadAssist validation tests**

Use a fake LLM provider returning:

- 100 tags;
- tag longer than allowed;
- invalid category;
- very long suggested title/description;
- unknown fields.

Expected:

- output tags are capped;
- invalid category is blank or replaced with allowed fallback;
- strings are truncated or rejected according to chosen contract;
- unknown fields are ignored.

Expected initial result: FAIL because backend returns LLM output as-is.

- [ ] **Step 2: Implement backend suggestion sanitizer**

Add helper in `agent_service.go`:

```go
func sanitizeUploadAssistResult(result UploadAssistResult, zoneOrContentType string) UploadAssistResult
```

Rules:

- max tags: 10;
- max tag length: use existing platform tag limit if defined, otherwise 32 and document it;
- title max: match publish form/backend content title max;
- description max: match backend content description max;
- category must be in allowed category list from config or shared constants.

- [ ] **Step 3: Add failing NLSearch visibility test**

Seed vector search results pointing to content that is soft-deleted, author-deleted, author-banned, IP-banned, private-other-user.

Expected initial result: FAIL because `NLSearch` manually checks only status, author banned, and public/viewer.

- [ ] **Step 4: Reuse shared visibility in NLSearch**

Preferred: after vector IDs are known, query `content_items` through GORM with `ApplyContentVisibilityScope(..., viewerID).Where("id IN ?", contentIDs)` and preserve vector score order in memory.

- [ ] **Step 5: Add feature flag tests for conversation endpoints**

When `cfg.Agent.WebAgentEnabled=false`, assert:

- `GET /agent/conversations` returns 503 `FEATURE_DISABLED`;
- `GET /agent/conversations/:id/messages` returns 503.

Expected initial result: FAIL if handlers skip flag check.

- [ ] **Step 6: Add feature checks in `agent.go`**

At top of `ListConversations` and `GetConversationMessages`:

```go
if !h.cfg.Agent.WebAgentEnabled {
    c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "web agent is disabled"})
    return
}
```

- [ ] **Step 7: Add frontend compliance behavior**

In `PublishForm.tsx`:

- if compliance risk is `warning`, require explicit acknowledgement before applying warning-associated suggestions;
- if risk is `violation`, disable upload-assist apply and final submit;
- validate suggested title/description length before applying.

Add next-intl keys for any new labels/errors.

- [ ] **Step 8: Run tests and gates**

Run:

```powershell
cd backend
go test ./internal/service -run TestAgent -v
go test ./internal/handler -run TestAgent -v
go test ./...
go vet ./...
go build ./...
```

Then:

```powershell
cd frontend
npm run lint
npm run build
```

Expected: all PASS.

- [ ] **Step 9: Commit Task 5**

```powershell
git add backend/internal/service/agent_service.go backend/internal/service/agent_service_test.go backend/internal/handler/agent.go backend/internal/handler/agent_test.go frontend/components/publish/PublishForm.tsx frontend/messages/zh.json frontend/messages/en.json
git commit -m "Beta repair: validate agent suggestions"
```

---

## Task 6: Admin Audit Transaction Integrity

**Files:**

- Modify: `backend/internal/handler/admin.go`
- Modify: `backend/internal/handler/admin_feedback.go`
- Modify: `backend/internal/handler/category.go`
- Modify: `backend/internal/handler/judge.go`
- Modify as needed: repositories/services used by these handlers
- Test: `backend/internal/handler/admin_audit_transaction_test.go`
- Test: `backend/internal/service/admin_audit_service_test.go`

### Design

Current unsafe pattern:

```go
domainMutation()
h.auditSvc.Record(ctx, entry)
```

Required pattern:

```go
err := h.db.Transaction(func(tx *gorm.DB) error {
    if err := domainMutationTx(tx); err != nil {
        return err
    }
    if err := h.auditSvc.RecordTx(ctx, tx, entry); err != nil {
        return err
    }
    return nil
})
```

External side effects, such as notifications or provider tests, must run after commit.

### TDD Steps

- [ ] **Step 1: Add rollback-on-audit-failure test for representative content mutation**

For `BanContent`:

- seed content `status=published`;
- inject audit service/repo that fails on create;
- call handler;
- assert response is `500 AUDIT_WRITE_FAILED`;
- assert content status is still `published`.

Expected initial result: FAIL because mutation commits before audit.

- [ ] **Step 2: Add rollback-on-audit-failure tests for each mutation pattern**

At minimum cover:

- direct repository update: content ban/restore;
- service update: IP approve/reject;
- user update: ban/unban;
- existing transaction path: appeal resolve;
- config/LLM local DB mutation where applicable;
- admin feedback patch/reply;
- category mutation;
- judge question create.

- [ ] **Step 3: Add failed-attempt audit tests**

For validation failure after admin identity is known:

- send invalid status/body;
- assert domain mutation does not happen;
- assert best-effort audit row with `Result: "failed"` and sanitized reason code.

Do not include raw DB/provider error strings in metadata.

- [ ] **Step 4: Add Tx variants narrowly**

For each repository/service method needed by admin handlers, add a tx-aware variant:

```go
func (r *ContentRepository) UpdateContentTx(tx *gorm.DB, id int64, updates map[string]interface{}) error
```

Prefer private helper:

```go
func (r *ContentRepository) updateContent(db *gorm.DB, id int64, updates map[string]interface{}) error
```

so existing public non-admin callers are unchanged.

- [ ] **Step 5: Refactor `admin.go` handlers**

Replace `auditOrFail` success usage for local DB mutations with a transaction helper.

Add helper:

```go
func (h *AdminHandler) withAuditTx(c *gin.Context, entry service.RecordAdminAuditInput, mutate func(tx *gorm.DB) error) bool
```

Inside helper:

- start `h.db.Transaction`;
- call mutate;
- call `h.auditSvc.RecordTx`;
- return stable `AUDIT_WRITE_FAILED` on audit failures;
- return stable domain error responses from caller where needed.

- [ ] **Step 6: Refactor `admin_feedback.go`**

Ensure `PatchFeedback` and `ReplyFeedback` cannot commit without success audit row.

If notification/email from Task 3 exists, send it after transaction commit.

- [ ] **Step 7: Refactor `category.go` and `judge.go`**

Replace `c.GetInt64("user_id")` with:

```go
adminID := middleware.GetUserID(c)
```

Add:

```go
TraceID: c.GetString("trace_id")
```

Use transaction + `RecordTx`.

- [ ] **Step 8: Preserve external side-effect ordering**

For operations that call external services:

- perform DB mutation + audit in transaction;
- commit;
- then send notification/email/provider call;
- if side effect fails, return/report according to task-specific requirements, but do not roll back committed domain state unless the spec explicitly requires it.

- [ ] **Step 9: Run admin tests**

Run from `backend/`:

```powershell
go test ./internal/handler -run "TestAdminAudit|TestAdmin|TestCategory|TestJudge|TestFeedback" -v
go test ./internal/service -run TestFilterMetadata -v
go test ./...
go vet ./...
go build ./...
```

Expected: all PASS.

- [ ] **Step 10: Commit Task 6**

```powershell
git add backend/internal/handler/admin.go backend/internal/handler/admin_feedback.go backend/internal/handler/category.go backend/internal/handler/judge.go backend/internal/handler/admin_audit_transaction_test.go backend/internal/service/admin_audit_service.go backend/internal/service/admin_audit_service_test.go
git add <exact repository/service tx variant files>
git commit -m "Beta repair: make admin audit mutations transactional"
```

---

## Task 7: Evidence Revalidation And Browser Regression

**Files:**

- Create: `docs/review/web-beta-repair-validation-2026-06-03.md`
- Create/update screenshots under: `screenshots/review-web-beta/repair-validation/`
- Do not update roadmap checkboxes until maintainers decide how these repairs map to Beta tracking.

### Steps

- [ ] **Step 1: Start local dependencies**

```powershell
docker compose up -d postgres redis
```

Expected: postgres and redis are healthy.

- [ ] **Step 2: Start backend and frontend**

Backend:

```powershell
cd backend
go run cmd/server/main.go
```

Frontend:

```powershell
cd frontend
npm run dev
```

- [ ] **Step 3: Run full backend gates**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

Expected: all PASS.

- [ ] **Step 4: Run full frontend gates**

```powershell
cd frontend
npm run lint
npm run build
```

Expected: all PASS.

- [ ] **Step 5: Re-run E2E journeys from Review 09**

Use MCP Playwright/API requests. Revalidate at least:

- Register -> pending -> verify -> login -> refresh -> logout.
- Captcha-required auth endpoints in local bypass mode.
- Resend cooldown.
- Forgot/reset password, including auto-login if retained.
- Anonymous feedback submit.
- Logged-in feedback list/detail.
- Admin reply/internal note/close; user detail hides internal note.
- Report resolve creates audit record.
- Search excludes hidden content and handles Chinese substring.
- Download rejects unauthenticated, unverified, low reputation, banned, author-banned, IP-banned.
- Sheet music PDF download CTA calls authorized endpoint.
- Agent disabled hides UI and backend returns 503.
- Admin pages access boundaries.
- `/api/v1/agent/script/:id` remains 404.
- `desktop_deploy_enabled=false` and `client.download_enabled=false`.

- [ ] **Step 6: Save screenshots**

Save screenshots to:

```text
screenshots/review-web-beta/repair-validation/
```

Use filenames that map to journeys, for example:

- `01-auth-register.png`
- `02-auth-login-refresh.png`
- `03-feedback-anonymous.png`
- `04-admin-report-resolve.png`
- `05-search-visibility.png`
- `06-download-authorized-cta.png`

- [ ] **Step 7: Write validation report**

Create `docs/review/web-beta-repair-validation-2026-06-03.md` with:

```markdown
# Web Beta Repair Validation - 2026-06-03
## Baseline
## Repairs Covered
## Commands Run
## Browser/API Evidence
## Remaining Production-Only Blockers
## Remaining Code Risks
## Verdict
```

- [ ] **Step 8: Commit Task 7**

```powershell
git add docs/review/web-beta-repair-validation-2026-06-03.md screenshots/review-web-beta/repair-validation
git commit -m "Beta repair: add repair validation evidence"
```

---

## Suggested Repair Order

1. Task 1: Auth/session/release safety.
2. Task 2: Verification lifecycle.
3. Task 3: Feedback/report functional breakages.
4. Task 4: Search/download visibility.
5. Task 5: Agent validation.
6. Task 6: Admin audit transaction integrity.
7. Task 7: Full repair validation evidence.

Task 6 is intentionally later because it is larger and has a broader blast radius. If production audit compliance is the release gate, move Task 6 before Task 5.

## Review Requirements

After each task:

1. Run the task-specific tests.
2. Run affected repository gates.
3. Request two reviews:
   - specification compliance review against the original Beta plan and verified review finding;
   - code quality review for regressions, security, and tests.
4. Fix review findings before starting the next task.

## Manual Production Configuration Required After Code Repairs

These are not code defects, but they block production deployment:

- SMTP real provider.
- Alibaba Cloud CAPTCHA 2.0 credentials.
- Alibaba Cloud OSS bucket and credentials.
- Production PostgreSQL DSN with SSL.
- Production Redis with auth and TLS enabled where the managed Redis provider supports it.
- HTTPS certificate and Nginx/API deployment for `api.leeppp.online`.
- `security.allowed_origins` set to `https://app.leeppp.online`.
- Strong `JWT_SECRET`.
- Approved legal terms/privacy version IDs and approved page copy.
- Alibaba Cloud Green/content safety credentials.
- Desktop Ed25519 keys only if D-02 through D-05/R-02 are resumed later.
