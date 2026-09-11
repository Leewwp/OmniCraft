package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
) // newContentCoverReviewService wires a ContentService whose ReviewService
// carries the injectable fakeGreenScanner seam, so the publish chain can be
// tested end to end: poster-grant cover derivation -> reviewInput assembly ->
// Green scan calls -> status transition.
func newContentCoverReviewService(t *testing.T) (*ContentService, *UploadGrantService, *fakeGreenScanner, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if err := db.AutoMigrate(
		&model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{},
		&model.AIReviewRecord{}, &model.ReputationLog{}, &model.User{}, &model.JudgeCase{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Mirror migration 068's unique index so the OnConflict idempotency clause
	// works on the in-memory sqlite schema.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_review_records_provider_task
		ON ai_review_records (provider, provider_task_id)`).Error; err != nil {
		t.Fatalf("create ai_review_records unique index: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	previousRedisClient := redisclient.Client
	redisclient.Client = rdb

	cfg := &config.Config{
		OSS: config.OSSConfig{Domain: "https://cdn.example.test"},
		Green: config.GreenConfig{
			Seed:        "seed_test_value",
			CallbackURL: "https://api.leeppp.online/api/v1/internal/ai-callback",
		},
		Reputation: config.ReputationConfig{
			RepeatViolationWindowDays: 7, RepeatViolationThreshold: 1, RepeatViolationExtraPenalty: -1,
		},
		Judge: config.JudgeConfig{MinVotesRequired: 3},
	}

	reviewSvc := NewReviewService(db, rdb, cfg, NewReputationService(db))
	green := &fakeGreenScanner{}
	reviewSvc.green = green

	grants := NewUploadGrantService(rdb, 5*time.Minute)
	svc := NewContentServiceWithDeps(repository.NewContentRepository(db), reviewSvc, rdb).
		WithUploadGrantService(grants).
		WithUploadedObjectVerifier(&fakeUploadedObjectVerifier{}).
		WithImageDimensionsResolver(&fakeImageDimensionsResolver{})
	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}

	return svc, grants, green, func() {
		redisclient.Client = previousRedisClient
		_ = rdb.Close()
		mr.Close()
	}
}

func contentCoverDBStatus(t *testing.T, svc *ContentService, contentID int64) string {
	t.Helper()
	var status string
	require.NoError(t, svc.contentRepo.DB().Raw("SELECT status FROM content_items WHERE id = ?", contentID).Scan(&status).Error)
	return status
}

// B2: a video publish whose derived poster cover is scanned by Green and
// blocked must not publish: the content row ends banned, the cover URL handed
// to Green is the server-derived poster URL, and the video scan carries the
// seed/dataId contract parameters.
func TestPublishVideoCoverBlockRejectsPublish(t *testing.T) {
	svc, grants, green, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	ctx := context.Background()

	poster, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/poster.png",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	require.NoError(t, err)
	video, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/v.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 456,
	})
	require.NoError(t, err)

	green.imageResult = &aliyun.GreenScanResult{Result: "block", Reason: "violent_content"}

	content, err := svc.PublishContent(PublishContentInput{
		Title:         "video cover review",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: poster.ID,
		Attachments: []AttachmentInput{
			{GrantID: video.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0),
				Width: intPtr(1280), Height: intPtr(720), DurationSec: intPtr(10)},
		},
	}, 42)
	require.NoError(t, err, "block is applied as a status transition, not a publish error")

	require.Equal(t, "banned", contentCoverDBStatus(t, svc, content.ID), "cover block must reject publication")
	require.Equal(t, []string{"https://cdn.example.test/uploads/42/image/poster.png"}, green.imageCalls,
		"the derived poster URL must be the only image scanned")
	require.Len(t, green.videoCalls, 1)
	require.Equal(t, "seed_test_value", green.videoCalls[0].Seed)
	require.Equal(t, fmt.Sprintf("content:%d", content.ID), green.videoCalls[0].DataID)
	require.Equal(t, "https://cdn.example.test/uploads/42/video/v.mp4", green.videoCalls[0].VideoURL)
}

// Decision 6: a cover that is not a platform OSS object fails the publish
// with an explicit error and is never sent to Green.
func TestPublishContentCoverNotPlatformObjectRejectsPublish(t *testing.T) {
	svc, _, green, cleanup := newContentCoverReviewService(t)
	defer cleanup()

	_, err := svc.PublishContent(PublishContentInput{
		Title:         "article cover review",
		Zone:          "original",
		Category:      "game",
		ContentType:   "article",
		IsPublic:      true,
		AllowCopy:     true,
		CoverImageURL: "https://evil.example.com/cover.png",
	}, 42)
	require.ErrorIs(t, err, ErrCoverNotPlatformOSSObject)
	require.Empty(t, green.imageCalls, "non-platform cover URLs must never reach Green")
}

func TestPublishContentWithoutCoverSkipsImageScan(t *testing.T) {
	svc, _, green, cleanup := newContentCoverReviewService(t)
	defer cleanup()

	content, err := svc.PublishContent(PublishContentInput{
		Title:       "plain article",
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		IsPublic:    true,
		AllowCopy:   true,
	}, 42)
	require.NoError(t, err)
	require.Empty(t, green.imageCalls, "no cover means no image scan")
	require.Equal(t, "published", contentCoverDBStatus(t, svc, content.ID), "pass keeps publishing")
}

func TestPublishVideoCoverPassPublishesContent(t *testing.T) {
	svc, grants, green, cleanup := newContentCoverReviewService(t)
	defer cleanup()
	ctx := context.Background()

	poster, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/poster.png",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	require.NoError(t, err)
	video, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/v.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 456,
	})
	require.NoError(t, err)

	content, err := svc.PublishContent(PublishContentInput{
		Title:         "video cover review",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: poster.ID,
		Attachments: []AttachmentInput{
			{GrantID: video.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0),
				Width: intPtr(1280), Height: intPtr(720), DurationSec: intPtr(10)},
		},
	}, 42)
	require.NoError(t, err)
	require.Equal(t, "published", contentCoverDBStatus(t, svc, content.ID))
	require.Equal(t, []string{"https://cdn.example.test/uploads/42/image/poster.png"}, green.imageCalls)
	require.Len(t, green.videoCalls, 1)
}
