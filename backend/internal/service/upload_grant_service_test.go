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
		UserID:   42,
		Purpose:  "content",
		OSSKey:   "uploads/42/image/2026/06/08/file.png",
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
