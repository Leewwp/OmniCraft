# OmniCraft OSS Upload And Download Hardening Implementation Plan

> ✅ **完成状态**: 本计划全部步骤已于 2026-06-09 执行完毕。执行记录见 `docs/superpowers/progress/2026-06-09-security-hardening-execution.md`。以下步骤仅保留原始计划结构作历史参考。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent forged upload references, reduce file-abuse risk, keep OSS private, and ensure downloads always pass through backend authorization.

**Architecture:** Treat presigned upload URLs as short-lived server-issued grants, not as durable authorization. Store upload grants in Redis, bind each grant to user, purpose, file type, declared size, MIME type, and OSS key, then consume grants during publish or feedback submission. Keep download authorization centralized in `DownloadContent` and align lifecycle documentation with the real `uploads/{user_id}/...` key layout.

**Tech Stack:** Go/Gin handlers, Redis, Aliyun OSS SDK, content service, feedback service, Next.js download UI, Go tests.

---

## Scope And Mode

This plan does not change the Tauri desktop security chain and does not enable desktop deploy.

Do not update `task.json` or Beta roadmap checkboxes unless this plan is converted into a tracked task.

## Current Repo Facts

- `backend/internal/service/oss_service.go` already creates content upload keys as `uploads/{user_id}/{file_type}/{yyyy}/{mm}/{dd}/...` and validates declared type, MIME, size, duration, and sheet-music extension before signing.
- `backend/internal/service/content_service.go` currently accepts publish attachments with raw `oss_key` through `AttachmentInput`; there is no server-side proof that the caller received that key from the presign endpoint or that the object was uploaded as declared.
- `backend/internal/service/feedback_service.go` already uses `grant_id` for feedback attachments. This plan must preserve that behavior and add explicit purpose isolation rather than replacing it blindly.
- `backend/internal/handler/content.go` constructs its own `ContentService` and `OSSService` in `NewContentHandler(db, cfg, rdb)`. Any content upload grant wiring must reach this handler path, not only `container.ServiceContainer.ContentService`.
- `DownloadContent` already returns a signed `download_url` JSON response and checks auth, content status, visibility, `allow_copy`, attachment ownership, and download guards. This plan extends tests and keeps that backend endpoint as the only download CTA.

## File Structure

- Modify: `backend/internal/service/oss_service.go`
  - Owns OSS key creation, upload type validation, and object metadata helpers.
- Modify: `backend/internal/pkg/aliyun/oss.go`
  - Owns low-level OSS adapter methods such as signed URL and object metadata.
- Create: `backend/internal/service/upload_grant_service.go`
  - Owns Redis-backed one-time upload grants.
- Create: `backend/internal/service/upload_grant_service_test.go`
  - Owns grant issue/consume/ownership tests.
- Modify: `backend/internal/handler/content.go`
  - Returns `grant_id` from content presign and validates grants during publish.
- Do not modify: `backend/internal/container/container.go`
  - Content upload grants are wired in `NewContentHandler(db, cfg, rdb)` for this plan. Container-level sharing can be designed later if another handler needs the same service.
- Modify: `backend/internal/service/content_service.go`
  - Validates attachments and prevents raw forged OSS keys.
- Modify: `backend/internal/model/content.go`
  - Adds `grant_id` to publish attachment input while keeping `oss_key` as a response/internal field during this transition.
- Modify: `backend/internal/handler/content_download_test.go`
  - Extends download authorization tests.
- Modify: `backend/internal/service/feedback_service.go`
  - Keeps feedback screenshot grants separate from content grants and validates purpose.
- Modify: `frontend/components/content/FileUploader.tsx`
  - Sends `grant_id` with uploaded attachments.
- Modify: `frontend/lib/content.ts`
  - Normalizes attachment fields without exposing raw public OSS URLs as download links.
- Modify: `frontend/components/content/DownloadButton.tsx`
  - Confirms download still uses backend endpoint.
- Modify: `docs/oss-lifecycle.md`
  - Aligns lifecycle prefix documentation with real code.

## Success Criteria

- A user cannot publish an attachment by guessing or reusing another user's `oss_key`.
- Upload grants are one-time, short-lived, purpose-bound, and user-bound.
- Content upload grants and feedback upload grants are not interchangeable.
- Published content download still requires auth, reputation, visibility, `status=published`, and `allow_copy=true`.
- Public DTOs do not expose long-lived public OSS download URLs as direct download CTAs.
- `go test ./internal/service -run "UploadGrant|OSS|Publish" -count=1` passes.
- `go test ./internal/handler -run "ContentDownload|OSS|Feedback" -count=1` passes.
- Full backend and frontend checks pass.

## Task 1: Add Upload Grant Service Tests

**Files:**
- Create: `backend/internal/service/upload_grant_service_test.go`
- Create: `backend/internal/service/upload_grant_service.go`

- [ ] **Step 1: Write failing tests**

Create `backend/internal/service/upload_grant_service_test.go`:

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newUploadGrantTestService(t *testing.T) (*UploadGrantService, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewUploadGrantService(rdb, 5*time.Minute), func() {
		_ = rdb.Close()
		mr.Close()
	}
}

func TestUploadGrantConsumeRequiresSameUserAndPurpose(t *testing.T) {
	svc, cleanup := newUploadGrantTestService(t)
	defer cleanup()
	ctx := context.Background()

	grant, err := svc.Issue(ctx, UploadGrant{
		UserID: 42,
		Purpose: "content",
		OSSKey: "uploads/42/image/2026/06/08/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 123,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := svc.Consume(ctx, grant.ID, 7, "content"); err != ErrUploadGrantInvalid {
		t.Fatalf("wrong user err = %v, want ErrUploadGrantInvalid", err)
	}
	if _, err := svc.Consume(ctx, grant.ID, 42, "feedback"); err != ErrUploadGrantInvalid {
		t.Fatalf("wrong purpose err = %v, want ErrUploadGrantInvalid", err)
	}
	consumed, err := svc.Consume(ctx, grant.ID, 42, "content")
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if consumed.OSSKey != grant.OSSKey {
		t.Fatalf("OSSKey = %q, want %q", consumed.OSSKey, grant.OSSKey)
	}
	if _, err := svc.Consume(ctx, grant.ID, 42, "content"); err != ErrUploadGrantInvalid {
		t.Fatalf("second consume err = %v, want ErrUploadGrantInvalid", err)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```powershell
cd backend
go test ./internal/service -run UploadGrant -count=1
```

Expected: FAIL because `UploadGrantService` does not exist.

## Task 2: Implement Redis-Backed Upload Grants

**Files:**
- Create: `backend/internal/service/upload_grant_service.go`

- [ ] **Step 1: Add implementation**

Create:

```go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrUploadGrantInvalid = errors.New("upload grant invalid or expired")
var ErrUploadGrantUnavailable = errors.New("upload grant store unavailable")

type UploadGrant struct {
	ID       string `json:"id"`
	UserID   int64  `json:"user_id"`
	Purpose  string `json:"purpose"`
	OSSKey   string `json:"oss_key"`
	FileType string `json:"file_type"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
}

type UploadGrantService struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewUploadGrantService(rdb *redis.Client, ttl time.Duration) *UploadGrantService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &UploadGrantService{rdb: rdb, ttl: ttl}
}

func (s *UploadGrantService) Issue(ctx context.Context, grant UploadGrant) (*UploadGrant, error) {
	if s == nil || s.rdb == nil {
		return nil, ErrUploadGrantUnavailable
	}
	grant.ID = randomGrantID()
	raw, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	if err := s.rdb.Set(ctx, uploadGrantKey(grant.ID), raw, s.ttl).Err(); err != nil {
		return nil, err
	}
	return &grant, nil
}

var consumeUploadGrantScript = redis.NewScript(`
local raw = redis.call("GET", KEYS[1])
if not raw then
  return nil
end
redis.call("DEL", KEYS[1])
return raw
`)

func (s *UploadGrantService) Consume(ctx context.Context, id string, userID int64, purpose string) (*UploadGrant, error) {
	if s == nil || s.rdb == nil {
		return nil, ErrUploadGrantUnavailable
	}
	raw, err := consumeUploadGrantScript.Run(ctx, s.rdb, []string{uploadGrantKey(id)}).Text()
	if err == redis.Nil {
		return nil, ErrUploadGrantInvalid
	}
	if err != nil {
		return nil, err
	}
	var grant UploadGrant
	if err := json.Unmarshal([]byte(raw), &grant); err != nil {
		return nil, ErrUploadGrantInvalid
	}
	if grant.UserID != userID || grant.Purpose != purpose {
		return nil, ErrUploadGrantInvalid
	}
	return &grant, nil
}

func uploadGrantKey(id string) string {
	return fmt.Sprintf("upload:grant:%s", id)
}

func randomGrantID() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}
```

- [ ] **Step 2: Run tests**

Run:

```powershell
cd backend
go test ./internal/service -run UploadGrant -count=1
```

Expected: PASS.

## Task 3: Return Content Upload Grant IDs

**Files:**
- Modify: `backend/internal/service/oss_service.go`
- Modify: `backend/internal/handler/content.go`
- Create: `backend/internal/handler/content_upload_grant_test.go`

- [ ] **Step 1: Extend presign response**

In `PresignUploadResponse`, add:

```go
GrantID string `json:"grant_id"`
```

- [ ] **Step 2: Add grant service to content handler**

In `ContentHandler`, add:

```go
uploadGrants *service.UploadGrantService
```

In `NewContentHandler(db, cfg, rdb)`, initialize it directly:

```go
grantTTL := time.Duration(cfg.Feedback.UploadGrantTTLSec) * time.Second
if grantTTL <= 0 {
	grantTTL = 5 * time.Minute
}
uploadGrants := service.NewUploadGrantService(rdb, grantTTL)
```

Set `uploadGrants: uploadGrants` in the returned `ContentHandler`.

Do not add a shared `UploadGrantService` field to `container.ServiceContainer` in this task.

- [ ] **Step 3: Issue grant after signed URL generation**

In `GenerateOSSToken`, after `resp` is generated:

```go
grant, err := h.uploadGrants.Issue(c.Request.Context(), service.UploadGrant{
	UserID: userID,
	Purpose: "content",
	OSSKey: resp.OSSKey,
	FileType: req.FileType,
	MimeType: req.MimeType,
	FileSize: req.FileSize,
})
if err != nil {
	response.SafeErrorResponse(c, http.StatusServiceUnavailable, "UPLOAD_GRANT_UNAVAILABLE", err)
	return
}
resp.GrantID = grant.ID
```

If Redis is unavailable, return `503 UPLOAD_GRANT_UNAVAILABLE`; do not issue untracked upload URLs in production.

- [ ] **Step 4: Add focused handler test**

Create `backend/internal/handler/content_upload_grant_test.go`. The test must call `GenerateOSSToken` through a Gin router with an authenticated user context and assert:

- Status is `200`.
- JSON includes non-empty `grant_id`.
- JSON includes `oss_key` beginning with `uploads/42/image/`.
- Redis contains a key named `upload:grant:<grant_id>`.

Use miniredis for Redis and a test `config.Config` with fake non-empty OSS endpoint/key/bucket values. `GeneratePresignUploadURL` signs a URL locally and must not call the network; do not add real Aliyun credentials to tests.

- [ ] **Step 5: Run tests**

Run:

```powershell
cd backend
go test ./internal/handler -run "OSS|UploadGrant|Content" -count=1
```

Expected: PASS for new focused tests.

## Task 4: Require Grants During Content Publish

**Files:**
- Modify: `backend/internal/service/content_service.go`
- Modify: `backend/internal/model/content.go`
- Create: `backend/internal/service/content_upload_grant_test.go`

- [ ] **Step 1: Inspect publish attachment DTO**

Run:

```powershell
rg -n "PublishContentInput|AttachmentInput|OSSKey|oss_key" backend/internal/service backend/internal/model backend/internal/handler
```

- [ ] **Step 2: Add `GrantID` to attachment input**

In `backend/internal/service/content_service.go`, update `AttachmentInput`:

```go
GrantID string `json:"grant_id"`
```

Keep `OSSKey string `json:"oss_key"` in the struct for read/transition compatibility, but do not trust caller-provided `oss_key` during publish. After grant consumption, overwrite `a.OSSKey` with the grant's `OSSKey` before creating `model.ContentAttachment`.

- [ ] **Step 3: Add content service dependency**

In `ContentService`, add:

```go
uploadGrants *UploadGrantService
```

Add a builder:

```go
func (s *ContentService) WithUploadGrantService(grants *UploadGrantService) *ContentService {
	s.uploadGrants = grants
	return s
}
```

Wire it in `NewContentHandler`.

- [ ] **Step 4: Validate and consume grant per attachment**

In `PublishContent`, before creating `ContentAttachment` rows:

```go
if a.GrantID == "" {
	return nil, ErrUploadGrantInvalid
}
grant, err := s.uploadGrants.Consume(ctx, a.GrantID, authorID, "content")
if err != nil {
	return nil, err
}
if grant.FileType != a.FileType {
	return nil, ErrUploadGrantInvalid
}
a.OSSKey = grant.OSSKey
a.FileSize = &grant.FileSize
a.MimeType = grant.MimeType
```

Do not change every existing caller of `PublishContent` in one sweep. Add a context-aware method and keep the old method as a wrapper:

```go
func (s *ContentService) PublishContent(input PublishContentInput, authorID int64) (*model.ContentItem, error) {
	return s.PublishContentWithContext(context.Background(), input, authorID)
}

func (s *ContentService) PublishContentWithContext(ctx context.Context, input PublishContentInput, authorID int64) (*model.ContentItem, error) {
	// move the existing PublishContent body here
}
```

Update only `ContentHandler.CreateContent` to call `PublishContentWithContext(c.Request.Context(), input, callerID)`.

- [ ] **Step 5: Add tests**

Test cases:

- Missing `grant_id` fails.
- Grant issued for another user fails.
- Feedback-purpose grant fails.
- First publish with a valid grant succeeds.
- Reusing the same grant in a second publish fails.

- [ ] **Step 6: Run service tests**

Run:

```powershell
cd backend
go test ./internal/service -run "UploadGrant|Publish" -count=1
```

Expected: PASS.

## Task 5: Verify OSS Object Metadata Before Finalizing Attachments

**Files:**
- Modify: `backend/internal/pkg/aliyun/oss.go`
- Modify: `backend/internal/service/oss_service.go`
- Modify: `backend/internal/service/oss_service_test.go`

- [ ] **Step 1: Add low-level metadata method**

In `aliyun.OSSClient`, add:

```go
type ObjectMeta struct {
	ContentLength int64
	ContentType   string
}

func (c *OSSClient) GetObjectMeta(ossKey string) (*ObjectMeta, error) {
	props, err := c.bucket.GetObjectDetailedMeta(ossKey)
	if err != nil {
		return nil, err
	}
	length, _ := strconv.ParseInt(props.Get("Content-Length"), 10, 64)
	return &ObjectMeta{
		ContentLength: length,
		ContentType: props.Get("Content-Type"),
	}, nil
}
```

Reuse existing `strconv` import if available; otherwise add it.

- [ ] **Step 2: Add service wrapper**

In `OSSService`:

```go
func (s *OSSService) VerifyUploadedObject(ctx context.Context, grant UploadGrant) error {
	_ = ctx
	meta, err := s.client.GetObjectMeta(grant.OSSKey)
	if err != nil {
		return err
	}
	if meta.ContentLength != grant.FileSize {
		return &UploadValidationError{Message: "uploaded file size does not match grant"}
	}
	if !strings.EqualFold(strings.TrimSpace(meta.ContentType), strings.TrimSpace(grant.MimeType)) {
		return &UploadValidationError{Message: "uploaded content type does not match grant"}
	}
	return nil
}
```

- [ ] **Step 3: Add a testable verifier interface**

In `backend/internal/service/content_service.go`, add this interface near `ContentService`:

```go
type UploadedObjectVerifier interface {
	VerifyUploadedObject(ctx context.Context, grant UploadGrant) error
}
```

Add this field to `ContentService`:

```go
uploadedObjectVerifier UploadedObjectVerifier
```

Add a setter:

```go
func (s *ContentService) WithUploadedObjectVerifier(verifier UploadedObjectVerifier) *ContentService {
	s.uploadedObjectVerifier = verifier
	return s
}
```

In `NewContentHandler`, wire the real OSS service as the verifier:

```go
contentSvc.WithUploadedObjectVerifier(ossSvc)
```

- [ ] **Step 4: Integrate before DB attachment creation**

Call `VerifyUploadedObject` after consuming each content grant and before inserting attachments. In unit tests, use a fake `UploadedObjectVerifier`; do not call real Aliyun OSS.

- [ ] **Step 5: Run tests**

Run:

```powershell
cd backend
go test ./internal/service -run "OSS|UploadGrant|Publish" -count=1
```

Expected: PASS.

## Task 6: Keep Feedback Grants Purpose-Isolated

**Files:**
- Modify: `backend/internal/service/feedback_service.go`
- Modify: `backend/internal/handler/feedback.go`
- Modify: `backend/internal/service/feedback_service_test.go`
- Modify: `backend/internal/handler/feedback_test.go`

- [ ] **Step 1: Inspect current feedback grant code**

Run:

```powershell
rg -n "feedback:upload_grant|UploadGrant|PresignUpload|AttachmentOSSKeys|Consume" backend/internal/service/feedback_service.go backend/internal/handler/feedback.go
```

- [ ] **Step 2: Preserve feedback-specific grants**

Feedback already has a grant flow (`PresignUpload`, `consumeUploadGrant`, `FeedbackAttachmentGrantInput`) using Redis keys named `feedback:upload_grant:<grant_id>`. Keep that implementation. Do not migrate feedback to the shared `UploadGrantService` in this plan.

The purpose isolation requirement is satisfied by separate Redis key namespaces:

- Content publish grants use `upload:grant:<grant_id>` and are consumed by `UploadGrantService.Consume(..., "content")`.
- Feedback grants use `feedback:upload_grant:<grant_id>` and are consumed only by `FeedbackService.consumeUploadGrant`.

Keep anonymous feedback CAPTCHA verification unchanged.

- [ ] **Step 3: Add tests**

Test cases:

- Content grant cannot be used as feedback attachment.
- Feedback grant cannot be used as content attachment.
- Anonymous feedback presign still requires captcha.
- Feedback upload URL uses a feedback-specific OSS prefix, not the content publish prefix.

- [ ] **Step 4: Run tests**

Run:

```powershell
cd backend
go test ./internal/service ./internal/handler -run "Feedback|UploadGrant" -count=1
```

Expected: PASS.

## Task 7: Harden Download Authorization And Client Usage

**Files:**
- Modify: `backend/internal/handler/content.go`
- Modify: `backend/internal/handler/content_download_test.go`
- Modify: `frontend/components/content/DownloadButton.tsx`
- Modify: `frontend/components/content/ContentDetail.tsx`
- Modify: `frontend/lib/content.ts`

- [ ] **Step 1: Extend backend download tests**

Add tests confirming:

- Anonymous users get 401.
- Low-reputation users are blocked by `downloadsGuard`.
- Banned/deleted authors make content unavailable.
- Private content is downloadable only by the author.
- `attachment_id` must belong to the content.
- `allow_copy=false` blocks download.
- Response includes `download_url` and `expires_in`, not an HTTP redirect.

- [ ] **Step 2: Run download tests and classify coverage**

Run:

```powershell
cd backend
go test ./internal/handler -run ContentDownload -count=1
```

Expected: If any listed case is missing, tests must FAIL first and the missing assertion must be added. If all listed cases are already covered and pass, record that in the task notes and make no backend download-code changes for this task.

- [ ] **Step 3: Keep `DownloadContent` as only download path**

Ensure the frontend uses:

```ts
api.get<{ download_url: string; expires_in: number }>(
  `/api/v1/contents/${contentId}/download${query}`
)
```

No direct `<a href={attachment.oss_url}>Download</a>` should remain.

Search:

```powershell
rg -n "oss_url|download_url|href=.*oss|DownloadButton" frontend/components frontend/app frontend/lib
```

- [ ] **Step 4: Remove raw OSS URL download CTAs**

If `oss_url` is still needed for inline preview, label it preview-only in the DTO/type name or adjacent code comment and keep actual download through `DownloadButton`.

Do not expose long-lived public OSS URLs as download links. `download_url` may appear only as the short-lived value returned by `GET /api/v1/contents/:id/download` and handled inside `DownloadButton`.

- [ ] **Step 5: Run frontend tests**

Run:

```powershell
cd frontend
npm run lint
npm run test
```

Expected: PASS.

## Task 8: Align OSS Lifecycle Documentation

**Files:**
- Modify: `docs/oss-lifecycle.md`

- [ ] **Step 1: Fix prefix documentation**

Current code builds keys like:

```text
uploads/{user_id}/{file_type}/{yyyy}/{mm}/{dd}/{timestamp}_{random}.{ext}
```

Update docs from `uploads/{file_type}/...` to `uploads/{user_id}/{file_type}/...`.

- [ ] **Step 2: Add grant rules**

Add:

```markdown
## Upload Grant Rules

- The backend issues presigned PUT URLs only together with a short-lived upload grant.
- A publish request must reference the grant ID, not only a raw OSS key.
- Grants are bound to user ID, purpose, file type, MIME type, declared file size, and OSS key.
- Grants are consumed once.
- Feedback screenshot grants and content publish grants are not interchangeable.
```

- [ ] **Step 3: Add private bucket reminder**

Ensure the document states:

```markdown
The bucket must remain private. Public read ACLs are not part of the application authorization model.
```

## Task 9: Full Verification And Commit

**Files:**
- All files changed in this plan.

- [ ] **Step 1: Backend verification**

Run:

```powershell
cd backend
go test ./internal/service -run "UploadGrant|OSS|Publish|Feedback" -count=1
go test ./internal/handler -run "ContentDownload|OSS|Feedback" -count=1
go test ./...
go build ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 2: Frontend verification**

Run:

```powershell
cd frontend
npm run lint
npm run test
npm run build
```

Expected: PASS.

- [ ] **Step 3: Inspect for raw direct download links**

Run:

```powershell
rg -n "href=.*oss_url|href=.*download_url|oss_url" frontend/components frontend/app frontend/lib
```

Expected: no direct download CTA uses raw OSS URLs. `download_url` may appear only in `DownloadButton` response handling; preview-only `oss_url` usages must be documented in code comments.

- [ ] **Step 4: Commit exact files**

Run:

```powershell
git add backend/internal/service/oss_service.go backend/internal/pkg/aliyun/oss.go backend/internal/service/upload_grant_service.go backend/internal/service/upload_grant_service_test.go backend/internal/handler/content.go backend/internal/service/content_service.go backend/internal/model/content.go backend/internal/handler/content_download_test.go backend/internal/service/feedback_service.go backend/internal/handler/feedback.go backend/internal/service/feedback_service_test.go backend/internal/handler/feedback_test.go frontend/components/content/FileUploader.tsx frontend/lib/content.ts frontend/components/content/DownloadButton.tsx frontend/components/content/ContentDetail.tsx docs/oss-lifecycle.md
git commit -m "Security: harden OSS upload and download flow"
```

Only stage files that were actually changed.
