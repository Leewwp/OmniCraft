# OmniCraft Web Beta Review Summary

> 📌 **权威来源**：本文档是 production-only blockers 的权威清单。其他审查报告中的 blockers 表格均为当时快照，以本文档为准。

**Generated**: 2026-06-03
**Source HEAD**: `15dc57fe362a382f3559165ca9767354c38a2317`
**Branch**: `main` (ahead 30 commits from origin)

> **状态说明（2026-06-28）**：以下 P0/P1/P2/P3 findings 的修复状态因时间跨度较大，未做逐项回填。各 finding 的实际修复状态请交叉对比 `web-beta-repair-validation-2026-06-03.md` 和后续 review 报告。

---

## Release Recommendation

**GO-WITH-BLOCKERS**

The Web Beta codebase is architecturally sound and passes all engineering gates (`go build`, `go vet`, `go test`, `npm run lint`, `npm run build`). 14 of 17 cross-stack E2E journeys pass. However, 3 P0 findings and multiple P1 findings must be resolved before production deployment. The production infrastructure (SMTP, CAPTCHA, OSS, HTTPS, etc.) is entirely unconfigured and is a separate deployment blocker not related to code quality.

---

## P0 Findings

### P0-01: AuthRequired fails open on Redis token-revocation check

- **Source**: Report 01 (P0-01)
- **Category**: Code defect — security
- **File**: `backend/internal/middleware/auth.go:54-62`
- **Description**: When Redis is available but returns an error for the blacklist check (timeout, failover), the middleware treats the error as "token not revoked" and continues. This violates the F-02 fail-closed contract: a revoked token (post-logout or post-ban) is accepted if Redis is temporarily unreachable.
- **Fix**: When `rdb != nil` and `redisErr` is not `redis.Nil`, return `503 AUTH_STATUS_UNAVAILABLE`.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: F-02 revalidation
- **Affected files**: `auth.go`, `auth_test.go`

### P0-02: Captcha is NOT verified on registration, forgot-password, or resend

- **Source**: Report 03 (V-02-1)
- **Category**: Code defect — security
- **File**: `backend/internal/handler/auth.go:58-98, 211-224, 273-298`; `backend/internal/container.go:125`
- **Description**: The `CaptchaVerifier` is created but assigned to `_` (discarded) and never injected into the auth handler. `Register`, `ForgotPassword`, and `ResendVerification` accept `captcha_token` in the request body but never verify it. Anyone can submit unlimited requests without captcha.
- **Fix**: Inject `CaptchaVerifier` into `AuthHandler`; call `verifier.Verify(captchaToken)` in all three handlers.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: V-02 revalidation
- **Affected files**: `container.go`, `auth.go`, `auth_test.go`

### P0-03: ValidateRelease() is defined but never called at startup

- **Source**: Report 03 (V-02-3)
- **Category**: Code defect — security
- **File**: `backend/config/config.go:402-417`; `backend/cmd/server/main.go`
- **Description**: `ValidateRelease()` correctly checks that `captcha.provider != "bypass"` and `smtp.mode != "logger"` in release mode, but is never called in `main.go`. A release deployment with bypass captcha or logger SMTP would start without error.
- **Fix**: Call `cfg.ValidateRelease()` in `main.go` before starting the server when `cfg.Server.Mode == "release"`.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: V-02 revalidation
- **Affected files**: `cmd/server/main.go`

---

## P1 Findings

### P1-01: SafeErrorMsg fallback leaks unrecognized error text

- **Source**: Report 00 (P1-3), Report 01 (P1-02)
- **Category**: Code defect — security
- **File**: `backend/internal/pkg/response/safe_error.go:45`
- **Description**: If an error does not match a known `businessErrors` entry and does not contain SQL/pq/dsn/record-not-found substrings, `err.Error()` is returned to the client. This can leak Redis OOM, file paths, connection strings, etc. Violates Task 102 and AGENTS.md rule 16.
- **Fix**: Change line 45 from `return err.Error()` to `return fallback`.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: Task 102 revalidation
- **Affected files**: `safe_error.go`, `safe_error_test.go`

### P1-02: Login captcha threshold logic is NOT implemented

- **Source**: Report 03 (V-02-2)
- **Category**: Code defect — security
- **File**: `backend/internal/handler/auth.go:100-130`; `backend/config/config.go:191`
- **Description**: `LoginCaptchaThreshold` config exists (default 3) but the `Login` handler has zero logic for incrementing failure counters, checking counters, or clearing on success.
- **Fix**: Implement Redis-backed failure counter with `captcha:login-failures:<sha256(email)>` key pattern per V-02 plan.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: V-02 revalidation
- **Affected files**: `auth.go`, `auth_test.go`

### P1-03: CORS unconditionally adds localhost variants even in release mode

- **Source**: Report 02 (P1-01)
- **Category**: Code defect — security
- **File**: `backend/internal/middleware/cors.go:18-28`
- **Description**: Localhost origins (`http://localhost:3000/3001`, `http://127.0.0.1:3000/3001`) are always added to the credentialed CORS allowlist regardless of server mode. An attacker on the same network could make credentialed cross-origin requests.
- **Fix**: Guard localhost variant injection with `cfg.Server.Mode != "release"`.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: F-03 revalidation
- **Affected files**: `cors.go`

### P1-04: Refresh handler falls back to reading refresh_token from JSON body

- **Source**: Report 02 (P1-02)
- **Category**: Code defect — security
- **File**: `backend/internal/handler/auth.go:158-170`
- **Description**: The F-03 plan states "Refresh reads only the cookie." The JSON body fallback allows an XSS attacker who steals a refresh token to use it without the HttpOnly cookie, undermining the cookie-only security model.
- **Fix**: Remove the JSON body fallback. Return `401 INVALID_TOKEN` when the cookie is missing.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: F-03 revalidation
- **Affected files**: `auth.go`

### P1-05: Logout handler accepts refresh_token from JSON body

- **Source**: Report 02 (P1-03)
- **Category**: Code defect — security
- **File**: `backend/internal/handler/auth.go:147-153`
- **Description**: Same issue as P1-04. The F-03 plan says "Never expose refresh tokens in JSON, URLs, logs or browser storage." Accepting refresh tokens from JSON body maintains an attack surface the cookie migration was designed to eliminate.
- **Fix**: Remove the JSON body path. Only read the refresh token from the HttpOnly cookie.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: F-03 revalidation
- **Affected files**: `auth.go`

### P1-06: verification_service.go missing cache invalidation after email verification

- **Source**: Report 01 (P1-01)
- **Category**: Code defect — functional
- **File**: `backend/internal/service/verification_service.go`
- **Description**: After email verification, the cached `RuntimeUserStatus` still has `EmailVerifiedAt == nil`. Until cache TTL expires (300s), `InteractionRequired` with `RequireVerifiedEmail` rejects the newly verified user.
- **Fix**: Add `middleware.InvalidateUserStatusCache(rdb, userID)` after the `email_verified_at` DB update.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: F-02 revalidation
- **Affected files**: `verification_service.go`

### P1-07: isPublishFrozen fails open when Redis is nil

- **Source**: Report 01 (P1-03)
- **Category**: Code defect — security
- **File**: `backend/internal/middleware/interaction.go:83-90`
- **Description**: When `rdb == nil`, a publish-frozen user bypasses the freeze check. The fail-closed principle suggests inability to check freeze status should block publishing.
- **Fix**: When `rdb == nil` and `RequireNoPublishFreeze` is true, return `503 AUTH_STATUS_UNAVAILABLE`.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: F-02 revalidation
- **Affected files**: `interaction.go`

### P1-08: Migration 041/042 in-place rewrite risks upgrade consistency

- **Source**: Report 00 (P1-1)
- **Category**: Code defect — data integrity
- **File**: `backend/migrations/041_content_search_vector.sql`, `backend/migrations/042_ips_search_vector.sql`
- **Description**: R-01 modified pre-existing migration files instead of creating new corrective migrations. Fresh databases are fine. Existing databases where old 041 failed (pg_jieba absent) have a broken migration state that the in-place fix does not resolve.
- **Fix**: Create new migration (053) that applies `CREATE OR REPLACE FUNCTION` fix and re-runs backfill with `WHERE search_vector IS NULL`. Verify target production DB never ran old 041 without pg_jieba.
- **Suggested priority**: Must fix before production (if upgrading from a prior state)
- **Suggested regression batch**: R-01 revalidation
- **Affected files**: New `053_fix_search_vector.sql`

### P1-09: Browser-test screenshots missing from repository

- **Source**: Report 00 (P1-2)
- **Category**: Test evidence gap
- **File**: `screenshots/` directory does not exist
- **Description**: R-01 validation report references `screenshots/r01_*.png` and `screenshots/beta-release-*` but no screenshots exist in the repository. Browser-test evidence is unverifiable from repository artifacts alone.
- **Fix**: Re-run Playwright browser tests and persist screenshots.
- **Suggested priority**: Should fix before production
- **Suggested regression batch**: R-01 revalidation

### P1-10: Anonymous feedback submission returns 500

- **Source**: Report 09 (BUG-09-1)
- **Category**: Code defect — functional
- **File**: `backend/internal/service/feedback_service.go`
- **Description**: GORM insert fails when `user_id` is NULL. Anonymous feedback is completely non-functional.
- **Fix**: Ensure model and GORM properly handle nullable `*int64` user_id for anonymous submissions.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: V-05 revalidation
- **Affected files**: `feedback_service.go`, `feedback_model.go`

### P1-11: Resend verification cooldown not enforced at handler level

- **Source**: Report 09 (BUG-09-2), Report 03 (V-01-1 related)
- **Category**: Code defect — security
- **File**: `backend/internal/handler/auth.go` (~line 295)
- **Description**: Handler discards `ErrResendCooldown` with `_ = h.verificationService.SendVerification(...)`. Rapid resend requests both return 200 instead of 429.
- **Fix**: Check the returned error and return 429 when `ErrResendCooldown` is raised.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: V-02 revalidation
- **Affected files**: `auth.go`

### P1-12: Report resolve API broken — missing `action_taken` column

- **Source**: Report 09 (BUG-09-3)
- **Category**: Code defect — functional / schema mismatch
- **File**: `backend/internal/repository/search_repo.go:362-367`
- **Description**: Repository code tries to update `action_taken` column which does not exist in the `reports` table. Admins cannot resolve reports via the API.
- **Fix**: Add `action_taken` column to `reports` table via new migration, or update repository code to match existing schema.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: A-03 revalidation
- **Affected files**: `search_repo.go`, new migration file

### P1-13: Audit writes are NOT atomic with domain mutations

- **Source**: Report 06 (A01-1), Report 07 (Issue #1)
- **Category**: Code defect — audit integrity
- **File**: All admin handlers (`admin.go`, `admin_feedback.go`, `category.go`, `judge.go`)
- **Description**: All handlers use `Record` (separate transaction) instead of `RecordTx` (same transaction). If audit write fails, domain mutation is already committed with no audit trail. The `RecordTx` method exists but is unused. This directly violates A-01 spec.
- **Fix**: Refactor all admin mutation handlers to use `db.Transaction()` wrapping both domain write and `RecordTx`. Move external side effects after commit.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: A-01/A-02 revalidation
- **Affected files**: All admin handlers, repository methods needing tx variants

### P1-14: No failed-attempt audit rows are recorded

- **Source**: Report 06 (A01-2)
- **Category**: Code defect — audit completeness
- **File**: `backend/internal/handler/admin.go:713-731`
- **Description**: `auditOrFail` always records `result: "success"`. No code path records `result: "failed"` for validation failures, authorization failures, or domain write failures. A-01 spec requires best-effort failed audit rows.
- **Fix**: Add failed-attempt audit recording with sanitized reason codes before returning error responses.
- **Suggested priority**: Should fix before production
- **Suggested regression batch**: A-01 revalidation
- **Affected files**: `admin.go`, all admin handlers

### P1-15: Admin reply/close does NOT send notifications or emails

- **Source**: Report 04 (V-05-3)
- **Category**: Code defect — functional
- **File**: `backend/internal/service/admin_feedback.go:75-210`, `backend/internal/service/feedback_service.go:216-292`
- **Description**: `PatchTicket` (close/reopen) and `AdminReply` send no notification or email. `FeedbackService` has no reference to `NotificationService` or `MailSender`.
- **Fix**: Inject `NotificationService` and `MailSender` into `FeedbackService`; send notifications on admin actions.
- **Suggested priority**: Should fix before production
- **Suggested regression batch**: V-05 revalidation
- **Affected files**: `feedback_service.go`, `admin_feedback.go`, `container.go`

### P1-16: PDFViewer uses `<a href={ossUrl}>` as download CTA, bypassing authorization

- **Source**: Report 05 (F06-1)
- **Category**: Code defect — security
- **File**: `frontend/components/content/SheetMusicViewer.tsx:243-258`
- **Description**: The PDFViewer component renders a direct `<a href={ossUrl}>` download button that bypasses the `GET /api/v1/contents/:id/download` authorization endpoint. Violates F-06 Step 4.
- **Fix**: Replace `<a href={ossUrl}>` with `DownloadButton` component.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: F-06 revalidation
- **Affected files**: `SheetMusicViewer.tsx`

### P1-17: Agent NLSearch does not check content `deleted_at` or author `deleted_at`

- **Source**: Report 05 (F05-3)
- **Category**: Code defect — data leak
- **File**: `backend/internal/service/agent_service.go:222-232`
- **Description**: NLSearch only checks `c.Status != "published"` and `c.Author.IsBanned`, missing soft-deleted content and deleted-author content. Also missing IP ban check.
- **Fix**: Add `c.DeletedAt.IsZero()`, author `deleted_at` check, and IP ban check. Preferably reuse `ApplyContentVisibilityScope`.
- **Suggested priority**: Must fix before production
- **Suggested regression batch**: G-02 revalidation
- **Affected files**: `agent_service.go`

---

## P2 Findings

### P2-01: `ContentVisibilityWhere` uses `fmt.Sprintf` to interpolate viewerID into SQL

- **Source**: Report 05 (F05-1)
- **Category**: Code defect — SQL safety
- **File**: `backend/internal/repository/content_visibility.go:23-28`
- **Description**: `fmt.Sprintf("... content_items.author_id = %d", viewerID)` violates the "禁止 SQL 字符串拼接" rule. While viewerID is int64 from auth context (not user input), the pattern should use parameterized queries.
- **Suggested priority**: Should fix
- **Suggested regression batch**: F-05 revalidation
- **Affected files**: `content_visibility.go`, `search_repo.go`

### P2-02: `ApplyContentVisibilityScope` does not filter content with banned IP

- **Source**: Report 05 (F05-2)
- **Category**: Code defect — data leak
- **File**: `backend/internal/repository/content_visibility.go:9-21`
- **Description**: Visibility scope does not filter out content whose associated IP has `status = 'banned'`. Per AGENTS.md: "IP 被永久封禁时，关联内容全部下架."
- **Suggested priority**: Should fix
- **Suggested regression batch**: F-05 revalidation
- **Affected files**: `content_visibility.go`

### P2-03: Download handler does not check author ban/deleted or IP ban

- **Source**: Report 05 (F06-2)
- **Category**: Code defect — data leak
- **File**: `backend/internal/handler/content.go:423-528`
- **Description**: `DownloadContent` does not verify content author is not banned/deleted, nor check IP ban status. These checks exist in the shared visibility scope but are not applied in the download path.
- **Suggested priority**: Should fix
- **Suggested regression batch**: F-06 revalidation
- **Affected files**: `content.go`

### P2-04: Screenshot upload grant is NOT consumed on ticket submission

- **Source**: Report 04 (V-05-1)
- **Category**: Code defect — security
- **File**: `backend/internal/service/feedback_service.go:106-115`
- **Description**: Redis grant key is never deleted (consumed) — expires via TTL only. Grant could be reused across multiple submissions.
- **Suggested priority**: Should fix
- **Suggested regression batch**: V-05 revalidation
- **Affected files**: `feedback_service.go`

### P2-05: PresignUpload does not return an actual OSS upload URL

- **Source**: Report 04 (V-05-4)
- **Category**: Code defect — functional
- **File**: `backend/internal/service/feedback_service.go:128-169`; `frontend/components/feedback/FeedbackForm.tsx:71-75`
- **Description**: Returns `grantID` and `ossKey` but no OSS presigned URL. Frontend attempts PUT to a path with no route handler. Screenshot upload will fail in practice.
- **Suggested priority**: Should fix
- **Suggested regression batch**: V-05 revalidation
- **Affected files**: `feedback_service.go`, `FeedbackForm.tsx`

### P2-06: CSRF middleware skips deploy-grants path entirely

- **Source**: Report 02 (P2-01)
- **Category**: Code defect — deferred security
- **File**: `backend/internal/middleware/csrf.go:61-77`
- **Description**: `POST /deploy-grants` is fully exempted from CSRF. When D-02 enables deploy grants, the issue endpoint will lack CSRF protection. Not exploitable today (feature disabled).
- **Suggested priority**: Fix before D-02
- **Suggested regression batch**: D-02 validation
- **Affected files**: `csrf.go`

### P2-07: STSToken struct has serializable `access_key_secret` field

- **Source**: Report 00 (P2-2)
- **Category**: Latent security risk
- **File**: `backend/internal/pkg/aliyun/oss.go:18`
- **Description**: `AccessKeySecret` has `json:"access_key_secret"` tag. Not exploitable today (struct not returned to clients), but should use `json:"-"` as defense-in-depth.
- **Suggested priority**: Should fix
- **Suggested regression batch**: Any
- **Affected files**: `oss.go`

### P2-08: `creator_support_enabled` absent from config.yaml

- **Source**: Report 00 (P2-1)
- **Category**: Operational clarity
- **File**: `backend/config.yaml`
- **Description**: Go zero value defaults to `false` (correct), but should be explicitly set for operator visibility.
- **Suggested priority**: Should fix
- **Suggested regression batch**: Any
- **Affected files**: `config.yaml`

### P2-09: AGENT_HMAC_SECRET compiled into Tauri client

- **Source**: Report 00 (P2-4)
- **Category**: Deferred desktop scope
- **File**: `tauri-client/src-tauri/src/commands/security.rs:7`
- **Description**: Acceptable while `desktop_deploy_enabled: false`. Must be resolved before D-03/R-02.
- **Suggested priority**: Fix before D-03
- **Suggested regression batch**: D-03 validation
- **Affected files**: `security.rs`

### P2-10: Category/Judge handlers use `c.GetInt64("user_id")` instead of `middleware.GetUserID(c)`

- **Source**: Report 06 (A01-3)
- **Category**: Code defect — audit accuracy
- **File**: `backend/internal/handler/category.go:57,84,111,137`; `backend/internal/handler/judge.go:181`
- **Description**: May return 0 if context key doesn't match, resulting in `admin_user_id = 0` in audit logs.
- **Suggested priority**: Should fix
- **Suggested regression batch**: A-02 revalidation
- **Affected files**: `category.go`, `judge.go`

### P2-11: Category/Judge handlers omit TraceID in audit entries

- **Source**: Report 06 (A01-4)
- **Category**: Code defect — audit completeness
- **File**: `backend/internal/handler/category.go:56-64,84-91,111-118,137-143`; `backend/internal/handler/judge.go:179-188`
- **Description**: These handlers don't call `c.GetString("trace_id")`, resulting in empty trace_id in audit records.
- **Suggested priority**: Should fix
- **Suggested regression batch**: A-02 revalidation
- **Affected files**: `category.go`, `judge.go`

### P2-12: Backend `UploadAssist` does not validate LLM response before returning

- **Source**: Report 08 (Issue #1)
- **Category**: Code defect — input validation
- **File**: `backend/internal/service/agent_service.go:58-93`
- **Description**: No tag count limit, tag length limit, category enum check, or field length limits on LLM output. Plan explicitly requires backend validation.
- **Suggested priority**: Should fix
- **Suggested regression batch**: G-05 revalidation
- **Affected files**: `agent_service.go`

### P2-13: Warning/violation compliance levels do not enforce required acknowledgement/block semantics

- **Source**: Report 08 (Issue #2)
- **Category**: Code defect — functional
- **File**: `frontend/components/publish/PublishForm.tsx:86-106, 404-415`
- **Description**: Warning-level suggestions do not require explicit acknowledgement before applying. Violation-level does not block the apply button for upload assist — only blocks final submit.
- **Suggested priority**: Should fix
- **Suggested regression batch**: G-05 revalidation
- **Affected files**: `PublishForm.tsx`

### P2-14: `SearchSuggestions` duplicates visibility logic inline

- **Source**: Report 05 (F05-4)
- **Category**: Code quality — maintenance risk
- **File**: `backend/internal/repository/search_repo.go:36-62`
- **Description**: Hardcodes `ci.is_public = true` instead of reusing `ApplyContentVisibilityScope`. Risks divergence.
- **Suggested priority**: Should fix
- **Suggested regression batch**: F-05 revalidation
- **Affected files**: `search_repo.go`

### P2-15: Reset-password does NOT establish auto-login session

- **Source**: Report 03 (V-02-4)
- **Category**: Code defect — functional
- **File**: `backend/internal/handler/auth.go:226-250`; `frontend/app/(protected)/reset-password/page.tsx`
- **Description**: Handler returns only `{"message": "password reset successfully"}` — no access token, no cookie. Plan and UI spec both require auto-login after reset.
- **Suggested priority**: Should fix
- **Suggested regression batch**: V-02 revalidation
- **Affected files**: `auth.go`, `reset-password/page.tsx`

### P2-16: Token consumption is NOT atomic — race condition possible

- **Source**: Report 03 (V-01-1)
- **Category**: Code defect — race condition
- **File**: `backend/internal/service/verification_service.go:127-130, 217-220`
- **Description**: Read-then-delete is not atomic. Between GET and DEL, a concurrent request could also read the same keys and succeed. Plan explicitly requires Lua script or equivalent atomic transaction.
- **Suggested priority**: Should fix
- **Suggested regression batch**: V-01 revalidation
- **Affected files**: `verification_service.go`

### P2-17: `ListConversations` and `GetConversationMessages` do not check `WebAgentEnabled`

- **Source**: Report 08 (Issue #3)
- **Category**: Code defect — consistency
- **File**: `backend/internal/handler/agent.go:253-275`
- **Description**: These endpoints return data even when agent is disabled. Behind auth+agentGuard so not publicly accessible, but should check feature flag for consistency.
- **Suggested priority**: Should fix
- **Suggested regression batch**: G-01 revalidation
- **Affected files**: `agent.go`

---

## P3 Findings

### P3-01: Validation report base SHA is stale

- **Source**: Report 00 (P3-1)
- **Category**: Documentation
- **Impact**: Informational only

### P3-02: Legal version fields empty in config.yaml

- **Source**: Report 00 (P3-2), Report 04 (V-04-1)
- **Category**: Production deployment blocker (not code)
- **Description**: `legal.current_terms_version` and `legal.current_privacy_version` are empty. Terms acceptance flow records empty version. Legal pages do not display version/effective date from config. Requires approved legal copy from maintainer.

### P3-03: progress.txt was overwritten in initial R-01 commit

- **Source**: Report 00 (P3-3)
- **Category**: Process — already corrected

### P3-04: No table-driven route tests for all policy groups

- **Source**: Report 01 (P3-01)
- **Category**: Test evidence gap
- **Description**: No route-level tests verifying every policy group endpoint has the correct guard applied.

### P3-05: InteractionRequired re-resolves status from DB on every request

- **Source**: Report 01 (P3-02)
- **Category**: Performance
- **Description**: Double resolution (AuthRequired + InteractionRequired). Not a correctness issue.

### P3-06: fetchPublicConfig throws on failure instead of returning safe defaults

- **Source**: Report 02 (P3-01)
- **Category**: Code quality
- **File**: `frontend/lib/public-config.ts:37-49`

### P3-07: Masked email placed in URL query string on register redirect

- **Source**: Report 03 (V-03-1)
- **Category**: Privacy
- **File**: `frontend/app/(auth)/register/page.tsx:79-80`
- **Description**: Masked email persists in URL/browser history. Plan suggests in-memory state instead.

### P3-08: Rate-limit keys use user ID, not SHA-256 of normalized email

- **Source**: Report 03 (V-01-2)
- **Category**: Spec compliance (minor)
- **File**: `backend/internal/service/verification_service.go:60`

### P3-09: MIME/size validation only at presign time, not at ticket submission time

- **Source**: Report 04 (V-05-2)
- **Category**: Low-risk security
- **Description**: Requires OSS write access exploitation; presign URL is short-lived.

### P3-10: /client page shows feature descriptions that could be considered fabricated

- **Source**: Report 04 (V-06-1)
- **Category**: Content accuracy
- **File**: `frontend/app/client/page.tsx:27-39`

### P3-11: FeedbackForm sends `navigator.userAgent` as `platform` diagnostic

- **Source**: Report 04 (V-06-2)
- **Category**: Data quality
- **File**: `frontend/components/feedback/FeedbackForm.tsx:113`

### P3-12: Missing integration tests for search

- **Source**: Report 05 (F05-5)
- **Category**: Test evidence gap
- **Description**: Five integration tests from F-05 plan do not exist.

### P3-13: Agent visibility tests only do source-string scanning

- **Source**: Report 05 (G02-1)
- **Category**: Test evidence gap
- **Description**: No DB-backed visibility tests for Agent NL search.

### P3-14: Test naming mismatch for admin audit

- **Source**: Report 06 (A01-5)
- **Category**: Test discoverability
- **Description**: `go test -run TestAdminAudit` matches nothing; tests named `TestFilterMetadata_*`.

### P3-15: Unknown audit actions pass all non-sensitive metadata keys

- **Source**: Report 06 (A01-6)
- **Category**: Defense-in-depth
- **Description**: Actions not in allowlist have all keys kept (except sensitive patterns). Deny-by-default would be stricter.

### P3-16: PatchFeedback audit action selection is fragile

- **Source**: Report 06 (A01-7)
- **Category**: Code quality
- **Description**: Determines action based on request field presence, not actual state change.

### P3-17: Feedback page lacks assignee UI

- **Source**: Report 07 (Issue #2)
- **Category**: Feature gap
- **Description**: Backend supports `assignee_admin_id` but frontend has no dropdown.

### P3-18: Admin layout redundant redirect + EmptyState for non-admin

- **Source**: Report 07 (Issue #3)
- **Category**: Code quality
- **Description**: Both `router.replace("/")` and access-denied EmptyState render for non-admin users.

### P3-19: Backend truncates over-length chat messages rather than rejecting

- **Source**: Report 08 (Issue #4)
- **Category**: Spec deviation (pragmatic)
- **Description**: Plan says "reject" but truncation is a reasonable Beta choice.

### P3-20: Frontend does not validate suggested_title/suggested_description length

- **Source**: Report 08 (Issue #5)
- **Category**: Input validation
- **File**: `frontend/components/publish/PublishForm.tsx:89-93`

### P3-21: SearchAgentInput visible to anonymous users (non-functional toggle)

- **Source**: Report 08 (Issue #6)
- **Category**: UX clarity

### P3-22: public-config.ts caches indefinitely

- **Source**: Report 08 (Issue #7)
- **Category**: Operational
- **Description**: Admin toggling `web_agent_enabled` at runtime won't be seen until page refresh.

### P3-23: R-01 committed across 4 separate commits

- **Source**: Report 00 (P2-3)
- **Category**: Process violation
- **Description**: First commit destroyed progress.txt history (later restored).

### P3-24: Public config test does not verify Legal DTO absence of secrets

- **Source**: Report 02 (P2-03)
- **Category**: Test completeness
- **File**: `backend/internal/handler/public_config_test.go:97`

### P3-25: Feedback page filter options use hardcoded English strings

- **Source**: Report 07 (Issue #5)
- **Category**: i18n compliance
- **File**: `frontend/app/(protected)/admin/feedback/page.tsx:329-352`

---

## Production-Only Blockers

These items require external infrastructure configuration and are NOT code defects. They block production deployment but not Web Beta code approval.

| # | Item | Config Location | Current Value | Required |
|---|------|----------------|---------------|----------|
| 1 | SMTP credentials | `smtp.host/user/password/from_address` | `mode: "logger"`, all empty | Real SMTP server |
| 2 | Alibaba Cloud CAPTCHA | `captcha.provider/access_key_id/access_key_secret/scene_id` | `provider: "bypass"` | `provider: "aliyun_v2"` with real credentials |
| 3 | Alibaba Cloud OSS | `oss.endpoint/access_key_id/access_key_secret/bucket_name/domain` | All empty | Real OSS bucket and credentials |
| 4 | PostgreSQL (production) | `database.dsn` | localhost dev DSN | Production DSN with SSL |
| 5 | Redis (production) | `redis.addr/password` | localhost:6379, empty | Production Redis with auth |
| 6 | HTTPS certificate | Nginx + `ssl_certs` volume | Not present | Valid TLS cert for `api.leeppp.online` |
| 7 | Allowed Origins | `security.allowed_origins` | `http://localhost:3000` | `https://app.leeppp.online` only |
| 8 | Legal versions | `legal.current_terms_version/current_privacy_version` | Empty strings | Approved legal version identifiers |
| 9 | JWT secret | `jwt.secret` | `dev-secret-change-in-production` | Cryptographically random production secret |
| 10 | Green (content safety) | `green.access_key_id/access_key_secret` | Empty | AI content review credentials |

**Deferred (desktop scope, not Web Beta)**:

| # | Item | Notes |
|---|------|-------|
| D-1 | Ed25519 private key | Required only for D-03+; `desktop_deploy_enabled: false` |
| D-2 | AGENT_HMAC_SECRET in Tauri client | Acceptable while no client binary distributed; P2-09 |

---

## Failed Or Blocked Test Matrix

| Journey / Test | Report | Status | Reason |
|---|---|---|---|
| Anonymous feedback submission | 09 (Journey 8) | **FAIL** | 500 error — nil user_id insert (P1-10) |
| Resend verification cooldown | 09 (Journey 3) | **FAIL** | Returns 200 instead of 429 (P1-11) |
| Report resolve API | 09 (Journey 13) | **FAIL** | Missing `action_taken` column (P1-12) |
| Low-reputation download rejection | 09 (Journey 5) | **BLOCKED** | No admin API to set user reputation |
| Download success path | 09 (Journey 5) | **BLOCKED** | No OSS bucket configured |
| Real SMTP email delivery | 09 | **BLOCKED** | `smtp.mode: "logger"` |
| Real captcha enforcement | 09 | **BLOCKED** | `captcha.provider: "bypass"` |
| Agent chat/search with LLM | 09 | **BLOCKED** | `agent.llm_api_key: ""` |
| HTTPS / TLS | 09 | **BLOCKED** | Local dev uses HTTP |
| Production CORS verification | 02 | **BLOCKED** | Requires HTTPS production setup |
| Release-mode cookie attributes | 02 | **BLOCKED** | `__Host-` prefix requires HTTPS |
| Screenshot upload (feedback) | 04 (V-05-4) | **FAIL** | No PUT route for staging path (P2-05) |
| Chinese search integration test | 05 (F05-5) | **MISSING** | Test not implemented |
| Route-level policy group tests | 01 (P3-01) | **MISSING** | No automated test for guard mounting |
| Full auth→interaction chain E2E | 01 | **MISSING** | No end-to-end middleware chain test |

---

## Duplicate Findings Consolidated

| Consolidated Finding | Source Reports | Original IDs |
|---|---|---|
| SafeErrorMsg leaks err.Error() | 00, 01 | P1-3, P1-02 |
| CORS localhost in release mode | 02 | P1-01 |
| Refresh/logout JSON body fallback | 02 | P1-02, P1-03 |
| Missing cache invalidation after email verification | 01 | P1-01 |
| Audit non-transactional with domain mutations | 06, 07 | A01-1, Issue #1 |
| Content visibility missing IP ban check | 05 | F05-2 (also affects F06-2, F05-3) |
| Legal version fields empty | 00, 04 | P3-2, V-04-1 |
| Production infrastructure blockers | 00, 09 | Section duplicates consolidated into single table above |
| Resend cooldown not enforced | 03, 09 | V-01-1 (partial), BUG-09-2 |
| Anonymous feedback broken | 04 (V-05-4 presign), 09 (BUG-09-1 insert) | Two separate bugs in same feature |

---

## Suggested Repair Order

### Batch 1 — Security P0 (must complete before any production deployment)

| # | Finding | Files | Est. Scope |
|---|---------|-------|------------|
| 1 | P0-01: AuthRequired fail-open on Redis blacklist | `auth.go`, `auth_test.go` | Small |
| 2 | P0-02: Captcha not verified on auth endpoints | `container.go`, `auth.go`, `auth_test.go` | Medium |
| 3 | P0-03: ValidateRelease() never called | `cmd/server/main.go` | Small |
| 4 | P1-01: SafeErrorMsg fallback leaks err.Error() | `safe_error.go`, `safe_error_test.go` | Small |
| 5 | P1-03: CORS localhost in release mode | `cors.go` | Small |
| 6 | P1-04/P1-05: Refresh/logout JSON body fallback | `auth.go` | Small |

### Batch 2 — Functional P1 (must complete before production)

| # | Finding | Files | Est. Scope |
|---|---------|-------|------------|
| 7 | P1-02: Login captcha threshold logic missing | `auth.go`, `auth_test.go` | Medium |
| 8 | P1-06: Cache invalidation after email verification | `verification_service.go` | Small |
| 9 | P1-07: isPublishFrozen fails open | `interaction.go` | Small |
| 10 | P1-10: Anonymous feedback 500 | `feedback_service.go`, model | Small |
| 11 | P1-11: Resend cooldown not enforced | `auth.go` | Small |
| 12 | P1-12: Report resolve missing `action_taken` column | `search_repo.go`, migration | Small |
| 13 | P1-16: PDFViewer direct oss_url download | `SheetMusicViewer.tsx` | Small |
| 14 | P1-17: NLSearch missing deleted_at/IP checks | `agent_service.go` | Small |

### Batch 3 — Audit Integrity P1

| # | Finding | Files | Est. Scope |
|---|---------|-------|------------|
| 15 | P1-13: Audit writes not atomic with domain mutations | All admin handlers, repo tx variants | Large |
| 16 | P1-14: No failed-attempt audit rows | `admin.go`, all admin handlers | Medium |
| 17 | P1-15: Admin reply/close no notifications | `feedback_service.go`, `container.go` | Medium |

### Batch 4 — P2 Code Quality

| # | Finding | Files | Est. Scope |
|---|---------|-------|------------|
| 18 | P2-01: fmt.Sprintf in ContentVisibilityWhere | `content_visibility.go`, `search_repo.go` | Small |
| 19 | P2-02: Visibility scope missing IP ban | `content_visibility.go` | Small |
| 20 | P2-03: Download handler missing author/IP checks | `content.go` | Small |
| 21 | P2-04: Feedback upload grant not consumed | `feedback_service.go` | Small |
| 22 | P2-05: PresignUpload no actual OSS URL | `feedback_service.go`, `FeedbackForm.tsx` | Medium |
| 23 | P2-07: STSToken json tag | `oss.go` | Trivial |
| 24 | P2-08: creator_support_enabled explicit in config | `config.yaml` | Trivial |
| 25 | P2-10/P2-11: Category/Judge handler audit fixes | `category.go`, `judge.go` | Small |
| 26 | P2-12: UploadAssist LLM output validation | `agent_service.go` | Medium |
| 27 | P2-13: Compliance warning/violation enforcement | `PublishForm.tsx` | Medium |
| 28 | P2-14: SearchSuggestions reuse visibility scope | `search_repo.go` | Small |
| 29 | P2-15: Reset-password auto-login | `auth.go`, `reset-password/page.tsx` | Medium |
| 30 | P2-16: Token consumption atomicity | `verification_service.go` | Small |
| 31 | P2-17: ListConversations feature flag check | `agent.go` | Small |

### Batch 5 — Evidence & Test Gaps

| # | Finding | Scope |
|---|---------|-------|
| 32 | P1-08: Migration 041/042 corrective migration | New migration file |
| 33 | P1-09: Re-run and persist browser test screenshots | Playwright execution |
| 34 | P3-12: Missing search integration tests | Test files |
| 35 | P3-04: Route-level policy group tests | Test files |

---

## Reports Read

| # | Report File | Date | Verdict |
|---|-------------|------|---------|
| 00 | `docs/review/web-beta-review-00-release-evidence.md` | 2026-06-02 | CONDITIONAL PASS |
| 01 | `docs/review/web-beta-review-01-authz-runtime.md` | 2026-06-02 | CONDITIONAL PASS |
| 02 | `docs/review/web-beta-review-02-session-config-desktop-off.md` | 2026-06-02 | CONDITIONAL PASS |
| 03 | `docs/review/web-beta-review-03-verification.md` | 2026-06-02 | FAIL (3 HIGH) |
| 04 | `docs/review/web-beta-review-04-feedback-public-pages.md` | 2026-06-02 | CONDITIONAL PASS |
| 05 | `docs/review/web-beta-review-05-search-download.md` | 2026-06-02 | PARTIAL PASS |
| 06 | `docs/review/web-beta-review-06-admin-audit.md` | 2026-06-02 | FAIL (CRITICAL) |
| 07 | `docs/review/web-beta-review-07-admin-console.md` | 2026-06-02 | PASS with minor gaps |
| 08 | `docs/review/web-beta-review-08-agent-entrypoints.md` | 2026-06-02 | PASS with moderate gaps |
| 09 | `docs/review/web-beta-review-09-cross-stack-e2e.md` | 2026-06-02 | CONDITIONAL PASS (14/17) |
