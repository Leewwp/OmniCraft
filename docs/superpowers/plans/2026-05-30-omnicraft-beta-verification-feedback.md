# OmniCraft Verification And Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the account-verification, help, legal and user-feedback loops required for a public Web Beta.

**Architecture:** Introduce narrow `verification` and `feedback` modules. External SMTP and captcha providers sit behind interfaces so unit tests use fakes and production verification stops with an explicit blocker until real credentials are supplied. UI pages remain thin clients of the new APIs and all new strings use `next-intl`.

**Tech Stack:** Go/Gin/GORM/PostgreSQL/Redis, SMTP-compatible mail provider, captcha provider, Next.js App Router, MCP Playwright.

---

## File Structure

### Backend

- Create: `backend/migrations/050_verification_and_terms.sql`.
- Create: `backend/migrations/051_feedback.sql`.
- Modify: `backend/config/config.go`, `backend/config.yaml`, `.env.example`.
- Create: `backend/internal/pkg/mail/sender.go`, `backend/internal/pkg/mail/smtp.go`.
- Create: `backend/internal/pkg/captcha/verifier.go`.
- Create: `backend/internal/service/verification_service.go`.
- Modify: `backend/internal/service/auth_service.go`.
- Modify: `backend/internal/handler/auth.go`, `backend/internal/handler/routes.go`.
- Create: `backend/internal/model/feedback.go`.
- Create: `backend/internal/repository/feedback_repo.go`.
- Create: `backend/internal/service/feedback_service.go`.
- Create: `backend/internal/handler/feedback.go`.
- Modify: `backend/internal/service/notification_service.go`.
- Create focused tests beside new packages.

### Frontend

- Modify: `frontend/app/(public)/register/page.tsx`.
- Modify: `frontend/app/(public)/forgot-password/page.tsx`.
- Modify: `frontend/app/(public)/reset-password/page.tsx`.
- Modify: `frontend/app/(public)/verify-email/page.tsx`.
- Create: `frontend/app/(public)/verify-email/pending/page.tsx`.
- Modify: `frontend/app/(protected)/settings/page.tsx`.
- Create: `frontend/app/(public)/help/page.tsx`.
- Create: `frontend/app/(public)/privacy/page.tsx`.
- Create: `frontend/app/(public)/terms/page.tsx`.
- Create: `frontend/app/(public)/client/page.tsx`.
- Create: `frontend/app/(public)/feedback/page.tsx`.
- Create: `frontend/app/(protected)/feedback/mine/page.tsx`.
- Create: `frontend/app/(protected)/feedback/[feedbackId]/page.tsx`.
- Create: `frontend/components/verification/CaptchaWidget.tsx`.
- Create: `frontend/components/feedback/FeedbackForm.tsx`.
- Modify: `frontend/components/layout/Footer.tsx`.
- Modify: `frontend/messages/zh.json`, `frontend/messages/en.json`.

## Task V-01: Add Verification Schema And External Adapters

**Files:**
- Create: `backend/migrations/050_verification_and_terms.sql`
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `.env.example`
- Create: `backend/internal/pkg/mail/sender.go`
- Create: `backend/internal/pkg/mail/smtp.go`
- Create: `backend/internal/pkg/captcha/verifier.go`
- Create: `backend/internal/service/verification_service.go`
- Create: `backend/internal/service/verification_service_test.go`

- [x] **Step 1: Add schema**

```sql
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS accepted_terms_version VARCHAR(32),
  ADD COLUMN IF NOT EXISTS accepted_terms_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS accepted_privacy_version VARCHAR(32),
  ADD COLUMN IF NOT EXISTS accepted_privacy_at TIMESTAMPTZ;
```

Token records remain in Redis only. Store hashes, never raw tokens:

```text
verify:email:<sha256(token)>   → stores user_id
verify:email:user:<user_id>    → stores active token SHA-256 digest
reset:password:<sha256(token)> → stores user_id
reset:password:user:<user_id>  → stores active token SHA-256 digest
verify:resend:<user_id>        → stores last resend timestamp
captcha:login-failures:<sha256(normalized-email)> → stores failure count
```

The per-user keys enable bidirectional lookup and single-active-token enforcement. On verification or reset: hash the incoming token, read `verify:email:<digest>` or `reset:password:<digest>` to obtain `user_id`, then read the matching per-user key and compare the stored active digest with a constant-time comparison. Consume both keys atomically with a Redis Lua script or equivalent transaction so concurrent requests cannot use the same token twice. When issuing a replacement token, atomically read and delete the previous digest key before writing the new digest key and per-user key. Overwriting only the per-user key is insufficient because it leaves stale digest keys behind.

Email normalization uses `strings.ToLower(strings.TrimSpace(email))`. For non-ASCII local parts (extremely rare), Beta preserves them as-is without Unicode NFC/NFD normalization. Redis rate-limit keys use SHA-256 of the normalized address rather than the raw email so Redis key names do not expose PII.

- [x] **Step 2: Add provider interfaces and fakes**

```go
type MailSender interface {
    SendVerification(ctx context.Context, to, link string) error
    SendPasswordReset(ctx context.Context, to, link string) error
}

type CaptchaVerifier interface {
    Verify(ctx context.Context, token, remoteIP string) error
}
```

Use fakes in unit tests. Use no-op providers only in explicit local development mode. A Beta release configuration without real providers must return a startup/configuration error. Extend the F-04 captcha config rather than defining a duplicate struct. Alibaba Cloud CAPTCHA 2.0 is the confirmed Beta provider; keep the `CaptchaVerifier` interface provider-agnostic and isolate Alibaba Cloud SDK details inside the adapter. Add these exact config fields: `web.public_base_url`, `smtp.mode`, `smtp.host`, `smtp.port`, `smtp.user`, `smtp.password`, `smtp.from_address`, `captcha.provider`, `captcha.prefix`, `captcha.scene_id`, `captcha.region`, `captcha.access_key_id`, `captcha.access_key_secret`, `verification.email_ttl_sec`, `verification.reset_ttl_sec`, `verification.resend_cooldown_sec`, `verification.login_captcha_threshold`, and `verification.password_min_length`. Support `CAPTCHA_ACCESS_KEY_ID` and `CAPTCHA_ACCESS_KEY_SECRET` environment overrides. Public config exposes only `provider`, `prefix`, `scene_id` and `region`. `smtp.mode=logger` and `captcha.provider=bypass` are local-development-only values and must be rejected when `server.mode == "release"`; release mode requires `captcha.provider=aliyun_v2`.

- [x] **Step 3: Add verification service tests**

Cover hashed storage, single use, previous-link invalidation, correct TTL separation, resend cooldown and provider failure.

- [x] **Step 4: Implement `VerificationService`**

Expose:

```go
func (s *VerificationService) SendVerification(ctx context.Context, user *model.User) error
func (s *VerificationService) VerifyEmail(ctx context.Context, rawToken string) error
func (s *VerificationService) SendPasswordReset(ctx context.Context, email string) error
func (s *VerificationService) ResetPassword(ctx context.Context, rawToken, newPassword string) error
```

- [x] **Step 5: Run tests**

```powershell
cd backend
go test ./internal/service -run TestVerification -v
go test ./...
go vet ./...
go build ./...
```

- [x] **Step 6: Commit**

```powershell
git add backend .env.example docs/superpowers/plans progress.txt
git commit -m "Beta V-01: add verification providers and token lifecycle - completed"
```

## Task V-02: Wire Registration, Resend And Reset Flows

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `backend/internal/model/config_public.go`
- Modify: `backend/internal/handler/public_config_test.go`
- Modify: `backend/internal/container/container.go`
- Modify: `backend/internal/service/auth_service.go`
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/auth_verification_test.go`
- Modify: `frontend/lib/public-config.ts`

- [ ] **Step 1: Write failing handler tests**

Cover:

- Registration creates an unverified user, records accepted terms, sends verification mail and does not return tokens.
- Forgot-password response is identical for known and unknown emails.
- Production responses never contain raw tokens.
- Registration, forgot-password and resend require captcha.
- Registration, forgot-password and resend apply limits by both remote IP and normalized email.
- Resend requires captcha, invalidates the previous verification link and respects cooldown.
- Reset token can be used exactly once.
- Password minimum length matches configured policy.
- Login requires captcha only after the configured failed-login threshold and clears the failure counter after success.

- [ ] **Step 2: Update registration contract**

Input:

```json
{
  "email":"user@example.com",
  "username":"user",
  "password":"...",
  "captcha_token":"...",
  "accepted_terms_version":"2026-05-30",
  "accepted_privacy_version":"2026-05-30"
}
```

The `accepted_terms_version` and `accepted_privacy_version` values must match the current versions defined in `config.yaml > legal.current_terms_version` and `legal.current_privacy_version`. Extend the F-04 public config DTO and its allowlist test with only these two legal version strings. The backend validates that the submitted versions equal the current configured versions; if they do not match, registration is rejected with `400 TERMS_VERSION_MISMATCH` or `400 PRIVACY_VERSION_MISMATCH`. The frontend reads these version strings from the public config endpoint and includes them in the registration payload. Do not hardcode version strings in the frontend.

Response:

```json
{"user":{"id":1,"email":"user@example.com"},"verification_required":true}
```

Return HTTP `202`. Do not create an interactive browser session before verification. Do not place the full email address in a redirect URL, browser history or server logs.

- [ ] **Step 3: Add resend route and captcha checks**

Mount:

```text
POST /api/v1/auth/resend-verification
```

Require `email` and `captcha_token`, return a uniform response, and enforce cooldown.

Forgot-password also accepts `email` and `captcha_token`, returns the same response for known and unknown emails, and never exposes a token. Reset-password accepts only `token` and `new_password`; after a successful reset it MUST establish the cookie/access-token session required by the existing `design/ui-spec.md` auto-login state and MUST NOT return a refresh token in JSON.

- [ ] **Step 4: Add login captcha threshold**

After configured failed-login threshold, require captcha. Keep normal login friction low before threshold.

- [ ] **Step 5: Run checks**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 6: Stop when Beta credentials are unavailable**

If real SMTP or captcha credentials are absent at the Beta verification point, record the blocker in `progress.txt`. Unit tests and local development may use fakes, but do not mark the Beta verification task complete and do not commit it as completed until real credentials are provisioned. `server.mode == "release"` must reject logger/bypass providers during startup.

- [ ] **Step 7: Commit**

```powershell
git add backend frontend docs/superpowers/plans progress.txt
git commit -m "Beta V-02: close email verification and reset flows - completed"
```

## Task V-03: Add Verification UI And Terms Acceptance

**Files:**
- Create: `frontend/components/verification/CaptchaWidget.tsx`
- Modify: `frontend/app/(public)/register/page.tsx`
- Modify: `frontend/app/(public)/forgot-password/page.tsx`
- Modify: `frontend/app/(public)/reset-password/page.tsx`
- Modify: `frontend/app/(public)/verify-email/page.tsx`
- Create: `frontend/app/(public)/verify-email/pending/page.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Read UI specs**

```powershell
rg -n "## Page: /register|## Page: /forgot-password|## Page: /reset-password|## Page: /settings" design/ui-spec.md
rg -n "## Component: CaptchaWidget|## Component: EmptyState|## Component: ConfirmModal" design/ui-spec.md
```

Apply existing visual rules. New pages without a spec must follow `design/design-system.md`.

- [ ] **Step 2: Update registration**

Remove token saving after registration. Add terms and privacy acceptance checkboxes linked to `/terms` and `/privacy`, captcha widget, and redirect to `/verify-email/pending` after the HTTP `202` response. Do not put the full email in the query string. The pending page may display a masked address only while the previous registration state is still available in memory; after reload it asks the user to re-enter the email before resending.

- [ ] **Step 3: Update verification page**

Create `/verify-email/pending` with masked email, resend cooldown, captcha-protected resend, change-email/back-to-register action and back-to-login action. Update `/verify-email?token=...` to read `token` from the URL query string and verify automatically. Do not ask the user to paste a token.

- [ ] **Step 4: Update reset page and settings**

Add captcha to forgot-password and use the configured `verification.resend_cooldown_sec` countdown. Use the query-string reset token, configured password policy, password-strength indicator and success auto-login contract from `design/ui-spec.md`. Settings must preserve the existing account-deletion section, display verified/unverified state, provide a captcha-protected resend button for unverified users and link to `/terms` and `/privacy`.

- [ ] **Step 5: Run frontend checks**

```powershell
cd frontend
npm run lint
npm run build
```

- [ ] **Step 6: Browser-test**

Use MCP Playwright to verify register, verify-email, resend, forgot-password, reset-password and settings states. Save screenshots under `screenshots/beta-v03-*`.

- [ ] **Step 7: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta V-03: add account verification UI - completed"
```

## Task V-04: Add Help, Legal And Client Information Pages

**Files:**
- Create: `frontend/app/(public)/help/page.tsx`
- Create: `frontend/app/(public)/privacy/page.tsx`
- Create: `frontend/app/(public)/terms/page.tsx`
- Create: `frontend/app/(public)/client/page.tsx`
- Modify: `frontend/components/layout/Footer.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Define page content**

Before editing UI, run:

```powershell
rg -n "## Component: Footer|## Page: /help|## Page: /privacy|## Page: /terms|## Page: /client" design/ui-spec.md
```

For pages without dedicated specs, follow `design/design-system.md`.

Before implementing `/privacy` or `/terms`, require approved legal copy, effective dates and the exact configured version identifiers from the roadmap's maintainer-decision table. Do not draft legal text. Web Beta is confirmed not to expose a client download. Render `/client` as an unavailable/information state with `client.download_enabled=false`; do not invent supported platforms, installation instructions, a download URL or a version. Approved client distribution facts are deferred until the desktop client is opened later.

Use versioned static content:

- `/help`: registration verification, publish, downloads, favorites, reports, reputation, Agent and client FAQ.
- `/privacy`: version and effective date plus data processing, cookies, uploads, logs, feedback diagnostics and deletion.
- `/terms`: version and effective date matching backend config.
- `/client`: supported platforms, installation, permission explanation, download link, manual fallback, common errors, feedback link, and an explicit notice that one-click deploy is unavailable unless secure desktop deploy is enabled.

- [ ] **Step 2: Implement pages and Footer routes**

Replace the existing placeholder Footer `/` links with exact routes: `/help`, `/privacy`, `/terms`, `/feedback`, and `/client`. Preserve any existing non-placeholder route unless it conflicts with the UI spec.

- [ ] **Step 3: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright to click Footer links and save `screenshots/beta-v04-footer.png`.

- [ ] **Step 4: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta V-04: add help legal and client pages - completed"
```

## Task V-05: Add Feedback Model And API

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Create: `backend/migrations/051_feedback.sql`
- Create: `backend/internal/model/feedback.go`
- Create: `backend/internal/repository/feedback_repo.go`
- Create: `backend/internal/service/feedback_service.go`
- Create: `backend/internal/handler/feedback.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/service/notification_service.go`
- Create: `backend/internal/service/feedback_service_test.go`
- Create: `backend/internal/handler/feedback_test.go`

- [ ] **Step 1: Add migration**

```sql
CREATE TABLE feedback_tickets (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT REFERENCES users(id),
  contact_email VARCHAR(255),
  category VARCHAR(32) NOT NULL,
  title VARCHAR(160) NOT NULL,
  description TEXT NOT NULL,
  diagnostic_summary JSONB NOT NULL DEFAULT '{}',
  status VARCHAR(24) NOT NULL DEFAULT 'open',
  priority VARCHAR(24) NOT NULL DEFAULT 'normal',
  assignee_admin_id BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at TIMESTAMPTZ
);

CREATE TABLE feedback_replies (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL REFERENCES feedback_tickets(id),
  author_user_id BIGINT REFERENCES users(id),
  author_admin_id BIGINT REFERENCES users(id),
  body TEXT NOT NULL,
  is_internal_note BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (author_user_id IS NOT NULL OR author_admin_id IS NOT NULL),
  CHECK (NOT (author_user_id IS NOT NULL AND author_admin_id IS NOT NULL))
);

CREATE TABLE feedback_attachments (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL REFERENCES feedback_tickets(id),
  oss_key TEXT NOT NULL,
  file_type VARCHAR(32) NOT NULL,
  mime_type VARCHAR(100) NOT NULL,
  size_bytes BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Add CHECK constraints for the frozen categories (`web_bug`, `desktop_deploy`, `content_or_community`, `account_or_security`, `agent_quality`, `feature_request`, `other`) and statuses (`open`, `in_progress`, `resolved`, `closed`).

- [ ] **Step 2: Write failing API tests**

Cover logged-in and anonymous submissions, captcha requirement for anonymous users, feedback-specific screenshot presign and issued-key consumption, MIME/size validation, diagnostic allowlist stripping, user ownership checks, hidden internal notes and notification/mail behavior.

- [ ] **Step 3: Implement service rules**

Diagnostic allowlist:

```go
var allowedDiagnosticKeys = map[string]struct{}{
    "app_version": {}, "platform": {}, "route": {}, "error_code": {},
}
```

Never persist token, cookie, local path, request headers or raw stack traces.

Screenshots use:

```text
POST /api/v1/feedback/attachments/presign
```

Use optional auth. Anonymous callers must provide captcha. Issue an opaque upload grant in Redis (TTL from `feedback.upload_grant_ttl_sec`, default `300`) and a short-lived upload URL under `feedback-staging/<opaque-id>/...`. Ticket submission consumes that upload grant, verifies the exact OSS key, image MIME and configured image size, then writes `feedback_attachments`. Do not reuse `uploads/<user_id>/...` content keys without a feedback purpose grant.

On Admin reply or close, reuse `NotificationService` for logged-in users. Send email for anonymous tickets with `contact_email`; production mail failure must be visible to the Admin as a retryable delivery failure, not silently discarded.

Require `contact_email` for anonymous submissions. Logged-in users may omit it and use their account email for optional delivery.
Only Admin callers may create `is_internal_note=true` replies. User-facing detail responses must always filter internal notes.

- [ ] **Step 4: Mount routes**

```text
POST /api/v1/feedback
POST /api/v1/feedback/attachments/presign
GET  /api/v1/feedback/me
GET  /api/v1/feedback/:id
```

Use optional auth for submit and required auth for user list/detail.
Register `/feedback/me` before `/feedback/:id` and return only owned tickets plus non-internal replies to user-facing callers.

- [ ] **Step 5: Run checks**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 6: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta V-05: add feedback ticket API - completed"
```

## Task V-06: Add Feedback Pages And Entry Points

**Files:**
- Create: `frontend/components/feedback/FeedbackForm.tsx`
- Create: `frontend/app/(public)/feedback/page.tsx`
- Create: `frontend/app/(protected)/feedback/mine/page.tsx`
- Create: `frontend/app/(protected)/feedback/[feedbackId]/page.tsx`
- Modify: `frontend/components/layout/Footer.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Implement shared feedback form**

Before editing UI, run:

```powershell
rg -n "## Component: FeedbackForm|## Page: /feedback" design/ui-spec.md
```

If no dedicated spec exists, follow `design/design-system.md`.

Fields: category, title, description, contact email, optional screenshot, captcha for anonymous users, explicit diagnostic-summary opt-in and privacy hint. Upload screenshot through `/api/v1/feedback/attachments/presign`, not `/api/v1/contents/oss-token`.

- [ ] **Step 2: Implement user list and detail pages**

Authenticated users can view `/feedback/mine`, open `/feedback/[feedbackId]`, inspect current status and see non-internal reply history. Keep anonymous submissions intentionally non-trackable in Beta. **Beta follow-up note:** After Beta, allow anonymous users to view their ticket status via a high-entropy token link received at submission time. The server must store only the token hash. This is recorded in the roadmap's "Non-Blocking Beta Follow-Ups" section.

- [ ] **Step 3: Add Footer and client entrypoints**

Footer must link to `/feedback`. The desktop client plan adds its own feedback CTA.

- [ ] **Step 4: Run checks and browser-test**

```powershell
cd frontend
npm run lint
npm run build
```

Use MCP Playwright to submit anonymous and authenticated feedback, inspect user list/detail, and save screenshots under `screenshots/beta-v06-*`.

- [ ] **Step 5: Commit**

```powershell
git add frontend screenshots docs/superpowers/plans progress.txt
git commit -m "Beta V-06: add feedback pages and entrypoints - completed"
```
