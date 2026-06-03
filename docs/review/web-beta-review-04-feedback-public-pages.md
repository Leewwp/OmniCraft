# Review: V-04 ~ V-06 — Feedback, Public Pages, Footer

**Reviewer**: Automated code review agent
**Date**: 2026-06-02
**Branch**: `main` (ahead 30)
**Commit**: `15dc57fe362a382f3559165ca9767354c38a2317`

---

## 1. Checklist Results

| # | Check | Verdict | Evidence |
|---|-------|---------|----------|
| 1 | Anonymous feedback requires captcha + contact_email; logged-in users can only read own tickets | ✅ PASS | `feedback_service.go:75-87` enforces captcha + contact_email for anonymous; `GetTicketForUser` checks `ticket.UserID == userID` |
| 2 | `/feedback/me` registered before `/feedback/:id` to avoid route mismatch | ✅ PASS | `routes.go:242-244` — `feedback.GET("/me", ...)` on line 242, `feedback.GET("/:id", ...)` on line 243 |
| 3 | Diagnostic summary strict allowlist; no token/cookie/path/header/stack trace persisted | ✅ PASS | `feedback_service.go:17-22` — `allowedDiagnosticKeys` has only `app_version`, `platform`, `route`, `error_code`; `filterDiagnostics()` strips all others |
| 4 | Screenshot uses feedback-specific presign, Redis grant, feedback-staging prefix; does not reuse content upload key | ⚠️ PARTIAL | See Finding V-05-1 |
| 5 | MIME, size, key ownership and grant single-consumption validation | ⚠️ PARTIAL | See Finding V-05-2 |
| 6 | Internal note never returned to user | ✅ PASS | `feedback_repo.go:23-24` — `FindTicketByID` preloads replies with `Where("is_internal_note = false")`; `FindTicketByIDForAdmin` loads all |
| 7 | Admin reply/close notification and anonymous email behavior observable; failures not silently swallowed | ⚠️ PARTIAL | See Finding V-05-3 |
| 8 | Footer reaches /help, /privacy, /terms, /feedback, /client | ✅ PASS | Browser test confirms all 5 links present |
| 9 | /client shows unavailable state when download_enabled=false; no fabricated download info | ✅ PASS | Browser test confirms no download links; page shows amber "unavailable" card |
| 10 | Legal version, effective date and content from approved input | ⚠️ BLOCKED | See Finding V-04-1 |

---

## 2. Findings

### V-04-1: Legal page content not verified against approved input [BLOCKED]

**Files**: `privacy/page.tsx`, `terms/page.tsx`

The plan states: *"Before implementing `/privacy` or `/terms`, require approved legal copy, effective dates and the exact configured version identifiers from the roadmap's maintainer-decision table. Do not draft legal text."*

Both pages render content entirely from `next-intl` translation keys (e.g., `t("privacy.dataProcessingContent")`, `t("terms.acceptanceContent")`). The actual legal text content is in `messages/zh.json` and `messages/en.json`, which I did not review for legal accuracy. Neither page displays a version number or effective date from the backend config.

The `PublicLegalDTO` exposes `current_terms_version` and `current_privacy_version` from the backend, but the frontend `/terms` and `/privacy` pages do not read or display these values.

**Status**: BLOCKED — Cannot confirm legal text is from approved input. Version/effective date display is missing.

---

### V-05-1: Screenshot upload grant is NOT consumed on ticket submission [MEDIUM]

**Files**: `feedback_service.go:106-115`, `feedback_handler.go:20-75`

The plan specifies: *"Ticket submission consumes that upload grant, verifies the exact OSS key, image MIME and configured image size, then writes `feedback_attachments`."*

Current implementation:
1. `PresignUpload` creates a Redis grant key `feedback:upload_grant:<grantID>` with the OSS key as value
2. `SubmitTicket` receives `attachment_oss_keys` and looks up attachments by OSS key
3. **The Redis grant key is never deleted (consumed)** — it expires via TTL only
4. The `grantID` is not passed in the ticket submission, so there is no way to verify the grant was issued for this specific submission
5. The OSS key ownership is not verified against the grant — any user who knows an OSS key could attach it to their ticket

**Impact**: A grant could be reused across multiple submissions. The grant-to-submission binding is not enforced.

**Severity**: Medium — the plan requires single-consumption; current code allows reuse within TTL window.

---

### V-05-2: MIME/size validation only at presign time, not at ticket submission time [LOW]

**File**: `feedback_service.go:138-146`

MIME and size validation happens in `PresignUpload` but NOT in `SubmitTicket`. If a user uploads a file that passes presign validation, they could potentially replace the OSS object with a different file (different MIME/size) before submitting the ticket, and the ticket submission would not re-validate.

**Severity**: Low — requires OSS write access exploitation; the presign URL is short-lived.

---

### V-05-3: Admin reply/close does NOT send notifications or emails [HIGH]

**Files**: `admin_feedback.go:75-210`, `feedback_service.go:216-292`

The plan specifies: *"On Admin reply or close, reuse `NotificationService` for logged-in users. Send email for anonymous tickets with `contact_email`; production mail failure must be visible to the Admin as a retryable delivery failure, not silently discarded."*

Current implementation:
1. `PatchTicket` (close/reopen) — **no notification or email sent**
2. `AdminReply` — **no notification or email sent**
3. The `FeedbackService` does not have a reference to `NotificationService` or `MailSender`
4. The `AdminFeedbackHandler` records audit logs but does not trigger any user-facing notification

**Severity**: HIGH — the plan explicitly requires notification/email on admin actions.

---

### V-05-4: PresignUpload does not return an actual upload URL [MEDIUM]

**Files**: `feedback_service.go:128-169`, `FeedbackForm.tsx:71-75`

The `PresignUpload` method returns `grantID` and `ossKey`, but does NOT generate an OSS presigned upload URL. The frontend `FeedbackForm.tsx` attempts to upload to:

```typescript
await fetch(`${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/${presignRes.oss_key}`, {
  method: "PUT", body: file, headers: { "Content-Type": file.type },
});
```

This constructs a URL like `http://localhost:8080/feedback-staging/<id>/<filename>` which would hit the Go backend, but there is no route handler for `PUT /feedback-staging/*`. The presign endpoint should return an actual OSS presigned URL (like `/api/v1/contents/oss-token` does for content uploads).

**Severity**: Medium — screenshot upload will fail in practice because there is no PUT endpoint for the staging path.

---

### V-06-1: /client page shows feature descriptions that could be considered fabricated [LOW]

**File**: `client/page.tsx:27-39`

The plan states: *"do not invent supported platforms, installation instructions, a download URL or a version."*

The page correctly shows an "unavailable" banner and does not show download links or version info. However, it does show a "Features" section with items like `client.feature1`, `client.feature2`, `client.feature3` which describe client capabilities. While these are generic (from i18n keys), they could be interpreted as advertising features that don't exist yet.

**Severity**: Low — the unavailable banner is prominent, but the features section is borderline.

---

### V-06-2: FeedbackForm sends `navigator.userAgent` as `platform` diagnostic [LOW]

**File**: `FeedbackForm.tsx:113`

```typescript
diagnosticSummary.platform = navigator.userAgent;
```

The `allowedDiagnosticKeys` allowlist includes `platform`, and the plan's intent is to collect platform info. However, `navigator.userAgent` is a full UA string (not a clean platform identifier) and could contain version/OS details beyond what's needed. The plan's diagnostic allowlist is correctly enforced server-side, so this is more of a data quality issue than a security issue.

**Severity**: Low — server-side allowlist correctly strips any extra keys.

---

## 3. Build & Test Results

| Check | Result |
|-------|--------|
| `go test ./...` | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go build ./...` | ✅ PASS |
| `npm run lint` (frontend) | ✅ PASS |
| `npm run build` (frontend) | ✅ PASS |

---

## 4. Browser Test Results

| Page | Result | Screenshot |
|------|--------|------------|
| Home (footer) | ✅ Footer renders with 5 links | `04-feedback-public-pages/01-home-footer.png` |
| /help | ✅ FAQ sections render with expand/collapse | `04-feedback-public-pages/02-help.png` |
| /privacy | ✅ Privacy sections render | `04-feedback-public-pages/03-privacy.png` |
| /terms | ✅ Terms sections render | `04-feedback-public-pages/04-terms.png` |
| /client | ✅ Unavailable banner shown; no download links | `04-feedback-public-pages/05-client.png` |
| /feedback (anonymous) | ✅ Form with captcha + contact email | `04-feedback-public-pages/06-feedback-public.png` |
| /feedback/mine (unauthenticated) | ✅ Redirects to `/login?redirect=%2Ffeedback%2Fmine` | `04-feedback-public-pages/07-feedback-mine-redirect.png` |

Footer links verified: `/help` ✅, `/privacy` ✅, `/terms` ✅, `/feedback` ✅, `/client` ✅

---

## 5. Production-Only Verification Items

1. **Real OSS presigned URL generation** — screenshot upload presign needs actual Alibaba Cloud OSS integration
2. **Admin notification/email delivery** — requires real SMTP for anonymous ticket email
3. **Legal text approval** — requires maintainer confirmation of approved legal copy

---

## 6. Summary

| Category | Count |
|----------|-------|
| HIGH findings | 1 |
| MEDIUM findings | 2 |
| LOW findings | 3 |
| BLOCKED | 1 |
| PASS | 6 |

**Overall Assessment**: V-04 through V-06 have solid foundations — the feedback schema, diagnostic allowlist, internal note filtering, and route ordering are all correct. The most critical issues are:
1. Admin reply/close does not send notifications or emails (V-05-3)
2. Screenshot upload grant is not consumed on submission (V-05-1)
3. PresignUpload does not return an actual OSS upload URL (V-05-4)
4. Legal pages do not display version/effective date from config (V-04-1)

The public pages (help, privacy, terms, client) and Footer are well-implemented and pass all browser tests.
