package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
)

func TestPublishContentRequiresUploadGrant(t *testing.T) {
	svc, _, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()

	_, err := svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		FileType: "image",
		OSSKey:   "uploads/42/image/forged.png",
		MimeType: "image/png",
	}), 42)

	if err != ErrUploadGrantInvalid {
		t.Fatalf("missing grant err = %v, want ErrUploadGrantInvalid", err)
	}
}

func TestPublishContentRejectsWrongUserAndPurposeGrants(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := context.Background()

	otherUserGrant, err := grants.Issue(ctx, UploadGrant{
		UserID:   7,
		Purpose:  "content",
		OSSKey:   "uploads/7/image/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue other user grant: %v", err)
	}
	_, err = svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		GrantID:  otherUserGrant.ID,
		FileType: "image",
		MimeType: "image/png",
	}), 42)
	if err != ErrUploadGrantInvalid {
		t.Fatalf("wrong user err = %v, want ErrUploadGrantInvalid", err)
	}

	feedbackGrant, err := grants.Issue(ctx, UploadGrant{
		UserID:   42,
		Purpose:  "feedback",
		OSSKey:   "feedback/42/image/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue feedback grant: %v", err)
	}
	_, err = svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		GrantID:  feedbackGrant.ID,
		FileType: "image",
		MimeType: "image/png",
	}), 42)
	if err != ErrUploadGrantInvalid {
		t.Fatalf("wrong purpose err = %v, want ErrUploadGrantInvalid", err)
	}
}

func TestPublishContentRejectsFeedbackGrantNamespace(t *testing.T) {
	svc, _, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := context.Background()

	feedbackSvc := NewFeedbackService(nil, nil, svc.rdb, nil, 300, fakeFeedbackOSSSigner{})
	feedbackGrant, err := feedbackSvc.PresignUpload(ctx, PresignUploadInput{
		UserID:    ptrInt64(42),
		FileName:  "shot.png",
		MimeType:  "image/png",
		SizeBytes: 512,
	})
	if err != nil {
		t.Fatalf("feedback presign: %v", err)
	}

	_, err = svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		GrantID:  feedbackGrant.GrantID,
		FileType: "image",
		MimeType: "image/png",
	}), 42)
	if err != ErrUploadGrantInvalid {
		t.Fatalf("feedback namespace err = %v, want ErrUploadGrantInvalid", err)
	}
}

func TestPublishContentConsumesUploadGrantOnce(t *testing.T) {
	svc, grants, verifier, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := context.Background()

	grant, err := grants.Issue(ctx, UploadGrant{
		UserID:   42,
		Purpose:  "content",
		OSSKey:   "uploads/42/image/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}

	input := baseGrantPublishInput(AttachmentInput{
		GrantID:  grant.ID,
		FileType: "image",
		OSSKey:   "uploads/42/image/forged.png",
		MimeType: "image/png",
	})
	content, err := svc.PublishContent(input, 42)
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if content == nil || content.ID == 0 {
		t.Fatalf("content not created: %#v", content)
	}
	if verifier.calls != 1 {
		t.Fatalf("verifier calls = %d, want 1", verifier.calls)
	}
	if verifier.lastGrant.OSSKey != grant.OSSKey {
		t.Fatalf("verified OSSKey = %q, want %q", verifier.lastGrant.OSSKey, grant.OSSKey)
	}

	_, err = svc.PublishContent(input, 42)
	if err != ErrUploadGrantInvalid {
		t.Fatalf("reuse grant err = %v, want ErrUploadGrantInvalid", err)
	}
}

func TestPublishContentRejectsUploadedObjectMismatch(t *testing.T) {
	svc, grants, verifier, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := context.Background()
	verifier.err = &UploadValidationError{Message: "uploaded file size does not match grant"}

	grant, err := grants.Issue(ctx, UploadGrant{
		UserID:   42,
		Purpose:  "content",
		OSSKey:   "uploads/42/image/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}

	_, err = svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		GrantID:  grant.ID,
		FileType: "image",
		MimeType: "image/png",
	}), 42)
	if !errors.Is(err, ErrUploadGrantInvalid) {
		t.Fatalf("publish err = %v, want ErrUploadGrantInvalid", err)
	}
	if verifier.calls != 1 {
		t.Fatalf("verifier calls = %d, want 1", verifier.calls)
	}
	verifier.err = nil
	_, err = svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		GrantID:  grant.ID,
		FileType: "image",
		MimeType: "image/png",
	}), 42)
	if err != nil {
		t.Fatalf("retry after uploaded object mismatch should reuse restored grant: %v", err)
	}
}

func TestPublishContentPreservesVerifierInfrastructureErrors(t *testing.T) {
	svc, grants, verifier, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := context.Background()
	verifier.err = errors.New("oss metadata lookup failed")

	grant, err := grants.Issue(ctx, UploadGrant{
		UserID:   42,
		Purpose:  "content",
		OSSKey:   "uploads/42/image/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}

	_, err = svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		GrantID:  grant.ID,
		FileType: "image",
		MimeType: "image/png",
	}), 42)
	if err == nil {
		t.Fatal("publish err = nil, want verifier infrastructure error")
	}
	if errors.Is(err, ErrUploadGrantInvalid) {
		t.Fatalf("publish err = %v, should not map verifier infrastructure failure to ErrUploadGrantInvalid", err)
	}
	if verifier.calls != 1 {
		t.Fatalf("verifier calls = %d, want 1", verifier.calls)
	}

	verifier.err = nil
	_, err = svc.PublishContent(baseGrantPublishInput(AttachmentInput{
		GrantID:  grant.ID,
		FileType: "image",
		MimeType: "image/png",
	}), 42)
	if err != nil {
		t.Fatalf("retry after verifier infrastructure failure should reuse restored grant: %v", err)
	}
}

func TestUploadGrantIssueFailsClosedWhenEntropyUnavailable(t *testing.T) {
	_, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()

	previousReader := uploadGrantEntropyReader
	uploadGrantEntropyReader = failingReader{}
	t.Cleanup(func() { uploadGrantEntropyReader = previousReader })

	_, err := grants.Issue(context.Background(), UploadGrant{
		UserID:   42,
		Purpose:  "content",
		OSSKey:   "uploads/42/image/file.png",
		FileType: "image",
		MimeType: "image/png",
		FileSize: 123,
	})
	if !errors.Is(err, ErrUploadGrantUnavailable) {
		t.Fatalf("Issue err = %v, want ErrUploadGrantUnavailable", err)
	}
}

func newContentGrantPublishService(t *testing.T) (*ContentService, *UploadGrantService, *fakeUploadedObjectVerifier, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	// Single connection so publish-transaction writes are visible to later
	// reads (in-memory sqlite databases are per-connection). Follows the
	// browse_history/admin_notification_broadcast test precedent.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	previousRedisClient := redisclient.Client
	redisclient.Client = rdb
	grants := NewUploadGrantService(rdb, 5*time.Minute)
	verifier := &fakeUploadedObjectVerifier{}
	svc := NewContentServiceWithDeps(repository.NewContentRepository(db), nil, rdb).
		WithUploadGrantService(grants).
		WithUploadedObjectVerifier(verifier).
		WithImageDimensionsResolver(&fakeImageDimensionsResolver{})
	return svc, grants, verifier, func() {
		redisclient.Client = previousRedisClient
		_ = rdb.Close()
		mr.Close()
	}
}

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

type fakeUploadedObjectVerifier struct {
	calls     int
	lastGrant UploadGrant
	err       error
}

func (f *fakeUploadedObjectVerifier) VerifyUploadedObject(ctx context.Context, grant UploadGrant) error {
	_ = ctx
	f.calls++
	f.lastGrant = grant
	return f.err
}

// fakeImageDimensionsResolver derives trusted dimensions from the object key
// itself, so tests prove the server, not the client, owns cover dimensions.
type fakeImageDimensionsResolver struct {
	err error
}

func (f *fakeImageDimensionsResolver) ResolveImageDimensions(ctx context.Context, ossKey string) (int, int, error) {
	_ = ctx
	if f.err != nil {
		return 0, 0, f.err
	}
	switch {
	case len(ossKey) >= 3 && ossKey[len(ossKey)-3:] == "png":
		return 1920, 1080, nil
	case len(ossKey) >= 4 && ossKey[len(ossKey)-4:] == ".jpg":
		return 1280, 720, nil
	}
	return 0, 0, errors.New("unsupported image")
}

func baseGrantPublishInput(attachment AttachmentInput) PublishContentInput {
	return PublishContentInput{
		Title:    "grant publish",
		Zone:     "original",
		Category: "game",
		// Non-media content type keeps the grant semantics of these tests
		// independent from the media set gallery contract (image/video
		// content now requires a full valid media set).
		ContentType: "article",
		IsPublic:    true,
		AllowCopy:   true,
		Attachments: []AttachmentInput{attachment},
	}
}
