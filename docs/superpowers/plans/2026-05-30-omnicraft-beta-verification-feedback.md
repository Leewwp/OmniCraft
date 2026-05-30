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
- Create focused tests beside new packages.

### Frontend

- Modify: `frontend/app/(public)/register/page.tsx`.
- Modify: `frontend/app/(public)/forgot-password/page.tsx`.
- Modify: `frontend/app/(public)/reset-password/page.tsx`.
- Modify: `frontend/app/(public)/verify-email/page.tsx`.
- Modify: `frontend/app/(protected)/settings/page.tsx`.
- Create: `frontend/app/(public)/help/page.tsx`.
- Create: `frontend/app/(public)/privacy/page.tsx`.
- Create: `frontend/app/(public)/terms/page.tsx`.
- Create: `frontend/app/(public)/client/page.tsx`.
- Create: `frontend/app/(public)/feedback/page.tsx`.
- Create: `frontend/app/(protected)/feedback/page.tsx`.
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

- [ ] **Step 1: Add schema**

```sql
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS accepted_terms_version VARCHAR(32),
  ADD COLUMN IF NOT EXISTS accepted_terms_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS accepted_privacy_version VARCHAR(32),
  ADD COLUMN IF NOT EXISTS accepted_privacy_at TIMESTAMPTZ;
```

Token records remain in Redis only. Store hashes, never raw tokens:

```text
verify:email:<sha256(token)>
reset:password:<sha256(token)>
verify:resend:<user_id>
captcha:login-failures:<normalized-email>
```

- [ ] **Step 2: Add provider interfaces and fakes**

```go
type MailSender interface {
    SendVerification(ctx context.Context, to, link string) error
    SendPasswordReset(ctx context.Context, to, link string) error
}

type CaptchaVerifier interface {
    Verify(ctx context.Context, token, remoteIP string) error
}
```

Use no-op providers only in explicit development mode. Production configuration without a real provider must return a startup/configuration error.

- [ ] **Step 3: Add verification service tests**

Cover hashed storage, single use, correct TTL separation, resend cooldown and provider failure.

- [ ] **Step 4: Implement `VerificationService`**

Expose:

```go
func (s *VerificationService) SendVerification(ctx context.Context, user *model.User) error
func (s *VerificationService) VerifyEmail(ctx context.Context, rawToken string) error
func (s *VerificationService) SendPasswordReset(ctx context.Context, email string) error
func (s *VerificationService) ResetPassword(ctx context.Context, rawToken, newPassword string) error
```

- [ ] **Step 5: Run tests**

```powershell
cd backend
go test ./internal/service -run TestVerification -v
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 6: Commit**

```powershell
git add backend .env.example docs/superpowers/plans progress.txt
git commit -m "Beta V-01: add verification providers and token lifecycle - completed"
```

## Task V-02: Wire Registration, Resend And Reset Flows

**Files:**
- Modify: `backend/internal/container/container.go`
- Modify: `backend/internal/service/auth_service.go`
- Modify: `backend/internal/handler/auth.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/handler/auth_verification_test.go`

- [ ] **Step 1: Write failing handler tests**

Cover:

- Registration creates an unverified user, records accepted terms, sends verification mail and does not return tokens.
- Forgot-password response is identical for known and unknown emails.
- Production responses never contain raw tokens.
- Resend requires captcha and respects cooldown.
- Reset token can be used exactly once.
- Password minimum length matches configured policy.

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

Response:

```json
{"user":{"id":1,"email":"user@example.com"},"verification_required":true}
```

Do not create an interactive browser session before verification.

- [ ] **Step 3: Add resend route and captcha checks**

Mount:

```text
POST /api/v1/auth/resend-verification
```

Require `email` and `captcha_token`, return a uniform response, and enforce cooldown.

- [ ] **Step 4: Add login captcha threshold**

After configured failed-login threshold, require captcha. Keep normal login friction low before threshold.

- [ ] **Step 5: Run checks**

```powershell
cd backend
go test ./...
go vet ./...
go build ./...
```

- [ ] **Step 6: Stop for production credentials if needed**

If real SMTP or captcha credentials are absent, record the blocker in `progress.txt`. Unit tests may pass with fakes, but do not mark production verification complete.

- [ ] **Step 7: Commit**

```powershell
git add backend docs/superpowers/plans progress.txt
git commit -m "Beta V-02: close email verification and reset flows - completed"
```

## Task V-03: Add Verification UI And Terms Acceptance

**Files:**
- Create: `frontend/components/verification/CaptchaWidget.tsx`
- Modify: `frontend/app/(public)/register/page.tsx`
- Modify: `frontend/app/(public)/forgot-password/page.tsx`
- Modify: `frontend/app/(public)/reset-password/page.tsx`
- Modify: `frontend/app/(public)/verify-email/page.tsx`
- Modify: `frontend/app/(protected)/settings/page.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Read UI specs**

```powershell
rg -n "## Page: /register|## Page: /forgot-password|## Page: /reset-password|## Page: /settings" design/ui-spec.md
```

Apply existing visual rules. New pages without a spec must follow `design/design-system.md`.

- [ ] **Step 2: Update registration**

Remove token saving after registration. Add terms checkbox linked to `/terms`, captcha widget, and a success state instructing the user to check email.

- [ ] **Step 3: Update verification page**

Read `token` from the URL query string and verify automatically. Keep a manual resend action; do not ask the user to paste a token.

- [ ] **Step 4: Update reset page and settings**

Use query-string reset token. Settings must display verified/unverified state and a resend button for unverified users.

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

Use versioned static content:

- `/help`: registration verification, publish, downloads, favorites, reports, reputation, Agent and client FAQ.
- `/privacy`: data processing, cookies, uploads, logs, feedback diagnostics and deletion.
- `/terms`: current agreement version matching backend config.
- `/client`: supported platforms, download link, manual fallback, and an explicit notice that one-click deploy is unavailable unless secure desktop deploy is enabled.

- [ ] **Step 2: Implement pages and Footer routes**

Replace the existing Footer `/` links with `/help`, `/privacy`, `/terms`, `/feedback`, and `/client` where appropriate.

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
- Create: `backend/migrations/051_feedback.sql`
- Create: `backend/internal/model/feedback.go`
- Create: `backend/internal/repository/feedback_repo.go`
- Create: `backend/internal/service/feedback_service.go`
- Create: `backend/internal/handler/feedback.go`
- Modify: `backend/internal/handler/routes.go`
- Create: `backend/internal/service/feedback_service_test.go`
- Create: `backend/internal/handler/feedback_test.go`

- [ ] **Step 1: Add migration**

```sql
CREATE TABLE feedback_tickets (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT REFERENCES users(id),
  category VARCHAR(32) NOT NULL,
  title VARCHAR(160) NOT NULL,
  description TEXT NOT NULL,
  contact TEXT,
  screenshot_oss_key TEXT,
  diagnostic_summary JSONB NOT NULL DEFAULT '{}',
  status VARCHAR(24) NOT NULL DEFAULT 'open',
  admin_reply TEXT,
  assigned_admin_id BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 2: Write failing API tests**

Cover logged-in and anonymous submissions, captcha requirement for anonymous users, screenshot key validation, diagnostic allowlist stripping, user ownership checks and admin update validation.

- [ ] **Step 3: Implement service rules**

Diagnostic allowlist:

```go
var allowedDiagnosticKeys = map[string]struct{}{
    "app_version": {}, "platform": {}, "route": {}, "error_code": {},
}
```

Never persist token, cookie, local path, request headers or raw stack traces.

- [ ] **Step 4: Mount routes**

```text
POST /api/v1/feedback
GET  /api/v1/feedback/me
GET  /api/v1/feedback/:id
```

Use optional auth for submit and required auth for user list/detail.

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
- Create: `frontend/app/(protected)/feedback/page.tsx`
- Create: `frontend/app/(protected)/feedback/[feedbackId]/page.tsx`
- Modify: `frontend/components/layout/Footer.tsx`
- Modify: `frontend/messages/zh.json`
- Modify: `frontend/messages/en.json`

- [ ] **Step 1: Implement shared feedback form**

Fields: category, title, description, contact, optional screenshot, captcha for anonymous users and privacy hint. Upload screenshot with the OSS presign path and configured image size limit.

- [ ] **Step 2: Implement user list and detail pages**

Authenticated users can view their own ticket status and admin reply. Keep anonymous submissions intentionally non-trackable in Beta.

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
