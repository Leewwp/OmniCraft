package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"omnicraft/backend/config"
)

// TestPublishContentRejectsInvalidMediaSets covers the authoritative media set
// contract that runs BEFORE any upload grant is consumed and before any row is
// written: quantity, purity and positive dimensions.
func TestPublishContentRejectsInvalidMediaSets(t *testing.T) {
	svc, _, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()

	image := func() AttachmentInput {
		return AttachmentInput{
			FileType: "image",
			OSSKey:   "uploads/42/image/forged.png",
			MimeType: "image/png",
			Width:    intPtr(100),
			Height:   intPtr(100),
		}
	}
	video := func() AttachmentInput {
		return AttachmentInput{
			FileType:    "video",
			OSSKey:      "uploads/42/video/forged.mp4",
			MimeType:    "video/mp4",
			DurationSec: intPtr(10),
			Width:       intPtr(1280),
			Height:      intPtr(720),
		}
	}

	tests := []struct {
		name        string
		contentType string
		attachments []AttachmentInput
		extra       func(*PublishContentInput)
	}{
		{
			name:        "image content with a single attachment is too few",
			contentType: "image",
			attachments: []AttachmentInput{image()},
		},
		{
			name:        "image content with ten attachments is too many",
			contentType: "image",
			attachments: []AttachmentInput{image(), image(), image(), image(), image(), image(), image(), image(), image(), image()},
		},
		{
			name:        "video content with four attachments is too many",
			contentType: "video",
			attachments: []AttachmentInput{video(), video(), video(), video()},
		},
		{
			name:        "image content cannot carry video attachments",
			contentType: "image",
			attachments: []AttachmentInput{image(), video()},
		},
		{
			name:        "video content cannot carry image attachments",
			contentType: "video",
			attachments: []AttachmentInput{video(), image()},
		},
		{
			name:        "zero width is rejected",
			contentType: "image",
			attachments: []AttachmentInput{{FileType: "image", OSSKey: "uploads/42/image/w.png", MimeType: "image/png", Width: intPtr(0), Height: intPtr(100)}, image()},
		},
		{
			name:        "negative height is rejected",
			contentType: "video",
			attachments: []AttachmentInput{{FileType: "video", OSSKey: "uploads/42/video/h.mp4", MimeType: "video/mp4", Width: intPtr(1280), Height: intPtr(-1)}},
		},
		{
			name:        "missing width on video is rejected",
			contentType: "video",
			attachments: []AttachmentInput{{FileType: "video", OSSKey: "uploads/42/video/w.mp4", MimeType: "video/mp4", Height: intPtr(720)}},
		},
		{
			name:        "media content cannot accept an arbitrary client cover_image_url",
			contentType: "image",
			attachments: []AttachmentInput{image(), image()},
			extra: func(input *PublishContentInput) {
				input.CoverImageURL = "https://evil.example.com/cover.png"
			},
		},
		{
			name:        "image content cannot carry a poster grant",
			contentType: "image",
			attachments: []AttachmentInput{image(), image()},
			extra: func(input *PublishContentInput) {
				input.PosterGrantID = "unexpected"
			},
		},
		{
			name:        "non-media content cannot carry poster fields",
			contentType: "article",
			extra: func(input *PublishContentInput) {
				input.PosterGrantID = "unexpected"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := mediaGalleryPublishInput(tt.contentType, tt.attachments...)
			if tt.extra != nil {
				tt.extra(&input)
			}
			_, err := svc.PublishContent(input, 42)
			if !errors.Is(err, ErrMediaSetInvalid) {
				t.Fatalf("publish err = %v, want ErrMediaSetInvalid", err)
			}
		})
	}
}

// TestPublishContentMediaSetValidationHappensBeforeGrantConsumption proves the
// contract runs before any upload grant is consumed and before any write: the
// grant stays intact after a rejected publish.
func TestPublishContentMediaSetValidationHappensBeforeGrantConsumption(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	grant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/file.png",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}

	// Invalid media set (single image) referencing a valid grant: the grant
	// must NOT be consumed because validation precedes grant consumption.
	_, err = svc.PublishContent(mediaGalleryPublishInput("image",
		AttachmentInput{GrantID: grant.ID, FileType: "image", OSSKey: "forged", MimeType: "image/png", Width: intPtr(100), Height: intPtr(100)},
	), 42)
	if !errors.Is(err, ErrMediaSetInvalid) {
		t.Fatalf("publish err = %v, want ErrMediaSetInvalid", err)
	}
	if _, err := grants.Consume(ctx, grant.ID, 42, "content"); err != nil {
		t.Fatalf("grant must remain consumable after a rejected publish: %v", err)
	}
}

func TestPublishContentRejectsMediaOrderThatDoesNotStartAtZero(t *testing.T) {
	svc, _, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()

	_, err := svc.PublishContent(PublishContentInput{
		Title:       "gapped order",
		Zone:        "original",
		ContentType: "image",
		Attachments: []AttachmentInput{
			{FileType: "image", SortOrder: intPtr(1), Width: intPtr(100), Height: intPtr(100)},
			{FileType: "image", SortOrder: intPtr(2), Width: intPtr(100), Height: intPtr(100)},
		},
	}, 42)
	if !errors.Is(err, ErrMediaSetInvalid) {
		t.Fatalf("publish err = %v, want ErrMediaSetInvalid", err)
	}
}

func mediaGalleryPublishInput(contentType string, attachments ...AttachmentInput) PublishContentInput {
	return PublishContentInput{
		Title:       "media publish",
		Zone:        "original",
		Category:    "game",
		ContentType: contentType,
		IsPublic:    true,
		AllowCopy:   true,
		Attachments: attachments,
	}
}

// TestPublishImageGalleryDerivesCoverFromFirstItem proves the media set happy
// path: a valid image gallery consumes its grants, persists attachments in
// stable sort_order (submission order must not matter), marks only the
// sort_order=0 entry as the derived cover and writes cover_width/cover_height
// from that first item.
func TestPublishImageGalleryDerivesCoverFromFirstItem(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}

	issueGrant := func(key string, size int) UploadGrant {
		grant, err := grants.Issue(ctx, UploadGrant{
			UserID: 42, Purpose: "content", OSSKey: key, FileType: "image",
			MimeType: "image/png", FileSize: int64(size),
		})
		if err != nil {
			t.Fatalf("issue grant: %v", err)
		}
		return *grant
	}
	first := issueGrant("uploads/42/image/first.png", 111)
	second := issueGrant("uploads/42/image/second.png", 222)

	// Deliberately out-of-order submission: sort_order=0 wins regardless of
	// the order the client lists the attachments.
	content, err := svc.PublishContent(PublishContentInput{
		Title:       "gallery",
		Zone:        "original",
		Category:    "game",
		ContentType: "image",
		IsPublic:    true,
		AllowCopy:   true,
		Attachments: []AttachmentInput{
			{GrantID: second.ID, FileType: "image", OSSKey: "forged", SortOrder: intPtr(1), Width: intPtr(300), Height: intPtr(400)},
			{GrantID: first.ID, FileType: "image", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(100), Height: intPtr(200)},
		},
	}, 42)
	if err != nil {
		t.Fatalf("publish image gallery: %v", err)
	}
	if want := "https://cdn.example.test/uploads/42/image/first.png"; content.CoverImageURL != want {
		t.Fatalf("cover URL = %q, want %q (sort_order=0 item)", content.CoverImageURL, want)
	}
	if content.CoverWidth == nil || *content.CoverWidth != 100 || content.CoverHeight == nil || *content.CoverHeight != 200 {
		t.Fatalf("cover dims = (%v,%v), want (100,200)", content.CoverWidth, content.CoverHeight)
	}

	rows, err := svc.contentRepo.GetAttachments(content.ID)
	if err != nil {
		t.Fatalf("get attachments: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("attachments = %d, want 2", len(rows))
	}
	if rows[0].OSSKey != first.OSSKey || rows[1].OSSKey != second.OSSKey {
		t.Fatalf("persisted order = [%s, %s], want sort_order 0 first", rows[0].OSSKey, rows[1].OSSKey)
	}
	if rows[0].IsPrimary == nil || !*rows[0].IsPrimary || rows[1].IsPrimary == nil || *rows[1].IsPrimary {
		t.Fatalf("is_primary must mark only the derived cover, got (%v,%v)", rows[0].IsPrimary, rows[1].IsPrimary)
	}
	for _, grant := range []UploadGrant{first, second} {
		if _, err := grants.Consume(ctx, grant.ID, 42, "content"); err == nil {
			t.Fatalf("grant %s must be consumed by a successful publish", grant.ID)
		}
	}
}

// TestPublishVideoWithPosterGrantDerivesCoverFromPoster proves the controlled
// poster happy path: the poster arrives as an image upload grant owned by the
// publisher, is consumed and verified, and the persistent cover URL and
// dimensions come from the uploaded object (server-derived), never from the
// client's PosterWidth/PosterHeight fields.
func TestPublishVideoWithPosterGrantDerivesCoverFromPoster(t *testing.T) {
	svc, grants, verifier, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}

	issueGrant := func(fileType, key, mime string) UploadGrant {
		grant, err := grants.Issue(ctx, UploadGrant{
			UserID: 42, Purpose: "content", OSSKey: key, FileType: fileType,
			MimeType: mime, FileSize: 123,
		})
		if err != nil {
			t.Fatalf("issue grant: %v", err)
		}
		return *grant
	}
	videoGrant := issueGrant("video", "uploads/42/video/clip.mp4", "video/mp4")
	posterGrant := issueGrant("image", "uploads/42/image/poster.png", "image/png")

	content, err := svc.PublishContent(PublishContentInput{
		Title:         "video",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: posterGrant.ID,
		PosterWidth:   intPtr(640),
		PosterHeight:  intPtr(360),
		Attachments: []AttachmentInput{
			{GrantID: videoGrant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if err != nil {
		t.Fatalf("publish video with poster: %v", err)
	}
	if want := "https://cdn.example.test/uploads/42/image/poster.png"; content.CoverImageURL != want {
		t.Fatalf("cover URL = %q, want %q (verified poster)", content.CoverImageURL, want)
	}
	// Dimensions must be derived from the object (1920x1080 for .png), not
	// echoed from the client's 640x360 claim.
	if content.CoverWidth == nil || *content.CoverWidth != 1920 || content.CoverHeight == nil || *content.CoverHeight != 1080 {
		t.Fatalf("cover dims = (%v,%v), want object-derived (1920,1080)", content.CoverWidth, content.CoverHeight)
	}
	if verifier.calls != 2 {
		t.Fatalf("verify calls = %d, want 2 (media item + poster)", verifier.calls)
	}
	if _, err := grants.Consume(ctx, posterGrant.ID, 42, "content"); err == nil {
		t.Fatal("poster grant must be consumed by a successful publish")
	}
}

// TestPublishVideoWithoutClientPosterDimensionsSucceeds proves the video
// poster contract does not require client-submitted poster_width/poster_height:
// the server consumes and verifies the poster grant and derives the cover
// dimensions from the uploaded object itself.
func TestPublishVideoWithoutClientPosterDimensionsSucceeds(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}

	posterGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/poster.png",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue poster grant: %v", err)
	}
	videoGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/clip.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue video grant: %v", err)
	}

	content, err := svc.PublishContent(PublishContentInput{
		Title:         "video",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: posterGrant.ID,
		Attachments: []AttachmentInput{
			{GrantID: videoGrant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if err != nil {
		t.Fatalf("publish video without client poster dimensions: %v", err)
	}
	if want := "https://cdn.example.test/uploads/42/image/poster.png"; content.CoverImageURL != want {
		t.Fatalf("cover URL = %q, want %q", content.CoverImageURL, want)
	}
	if content.CoverWidth == nil || *content.CoverWidth != 1920 || content.CoverHeight == nil || *content.CoverHeight != 1080 {
		t.Fatalf("cover dims = (%v,%v), want object-derived (1920,1080)", content.CoverWidth, content.CoverHeight)
	}
}

// TestPublishVideoIgnoresWrongClientPosterDimensions proves client-submitted
// poster_width/poster_height are never trusted: even absurd values are ignored
// and the persisted cover dimensions come from the object header.
func TestPublishVideoIgnoresWrongClientPosterDimensions(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}

	posterGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/poster.png",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue poster grant: %v", err)
	}
	videoGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/clip.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue video grant: %v", err)
	}

	content, err := svc.PublishContent(PublishContentInput{
		Title:         "video",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: posterGrant.ID,
		PosterWidth:   intPtr(-5),
		PosterHeight:  intPtr(999999),
		Attachments: []AttachmentInput{
			{GrantID: videoGrant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if err != nil {
		t.Fatalf("publish video with wrong client poster dimensions must still succeed: %v", err)
	}
	if content.CoverWidth == nil || *content.CoverWidth != 1920 || content.CoverHeight == nil || *content.CoverHeight != 1080 {
		t.Fatalf("cover dims = (%v,%v), want object-derived (1920,1080)", content.CoverWidth, content.CoverHeight)
	}
}

// TestPublishImageIgnoresClientPosterDimensions proves client poster width/height
// are ignored for image content too: the cover always comes from the media set
// first item, never from the compatibility poster fields.
func TestPublishImageIgnoresClientPosterDimensions(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}

	g1, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/first.png",
		FileType: "image", MimeType: "image/png", FileSize: 111,
	})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}
	g2, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/second.png",
		FileType: "image", MimeType: "image/png", FileSize: 222,
	})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}

	content, err := svc.PublishContent(PublishContentInput{
		Title:        "gallery",
		Zone:         "original",
		Category:     "game",
		ContentType:  "image",
		IsPublic:     true,
		AllowCopy:    true,
		PosterWidth:  intPtr(999),
		PosterHeight: intPtr(999),
		Attachments: []AttachmentInput{
			{GrantID: g1.ID, FileType: "image", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(100), Height: intPtr(200)},
			{GrantID: g2.ID, FileType: "image", OSSKey: "forged", SortOrder: intPtr(1), Width: intPtr(300), Height: intPtr(400)},
		},
	}, 42)
	if err != nil {
		t.Fatalf("publish image with client poster dimensions must succeed: %v", err)
	}
	if content.CoverWidth == nil || *content.CoverWidth != 100 || content.CoverHeight == nil || *content.CoverHeight != 200 {
		t.Fatalf("cover dims = (%v,%v), want media-set first item (100,200)", content.CoverWidth, content.CoverHeight)
	}
}

// TestPublishFailureCleanupLogUsesRealGrantKeys proves the rollback cleanup log
// is built from the consumed grants (real server-side OSS keys) and never from
// client-submitted keys, and that the consumed poster grant's key is included.
func TestPublishFailureCleanupLogUsesRealGrantKeys(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}
	svc.imageDimensions = &fakeImageDimensionsResolver{err: errors.New("not an image")}

	posterGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/real-poster.png",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue poster grant: %v", err)
	}
	videoGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/real-clip.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue video grant: %v", err)
	}

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	_, err = svc.PublishContent(PublishContentInput{
		Title:         "video",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: posterGrant.ID,
		Attachments: []AttachmentInput{
			{GrantID: videoGrant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if err == nil {
		t.Fatal("publish must fail when poster dimensions cannot be derived")
	}

	output := logs.String()
	if !strings.Contains(output, posterGrant.OSSKey) {
		t.Fatalf("cleanup log must contain the real consumed poster grant OSS key %q; log=%q", posterGrant.OSSKey, output)
	}
	if strings.Contains(output, `"forged"`) {
		t.Fatalf("cleanup log must not contain client-submitted keys; log=%q", output)
	}

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode cleanup log: %v; output=%q", err, logs.String())
	}
	msg, _ := entry["msg"].(string)
	if !strings.Contains(msg, "manual cleanup") {
		t.Fatalf("unexpected log message %q, want manual cleanup entry", msg)
	}
}

// failOnVideoVerifier rejects only video grants so the poster grant is fully
// consumed and verified before the media attachment fails.
type failOnVideoVerifier struct{}

func (failOnVideoVerifier) VerifyUploadedObject(_ context.Context, grant UploadGrant) error {
	if grant.FileType == "video" {
		return errors.New("object gone")
	}
	return nil
}

// TestPublishFailureCleanupLogIncludesConsumedAttachmentKeys: when a media
// attachment grant is consumed before the failure, its real key must also be in
// the cleanup log alongside the poster key.
func TestPublishFailureCleanupLogIncludesConsumedAttachmentKeys(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}
	svc.uploadedObjectVerifier = failOnVideoVerifier{}

	posterGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/real-poster.png",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue poster grant: %v", err)
	}
	videoGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/real-clip.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue video grant: %v", err)
	}

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	_, err = svc.PublishContent(PublishContentInput{
		Title:         "video",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: posterGrant.ID,
		Attachments: []AttachmentInput{
			{GrantID: videoGrant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if err == nil {
		t.Fatal("publish must fail when video attachment verification fails")
	}

	output := logs.String()
	if !strings.Contains(output, posterGrant.OSSKey) {
		t.Fatalf("cleanup log must contain the real consumed poster grant OSS key %q; log=%q", posterGrant.OSSKey, output)
	}
	if !strings.Contains(output, videoGrant.OSSKey) {
		t.Fatalf("cleanup log must contain the real consumed attachment grant OSS key %q; log=%q", videoGrant.OSSKey, output)
	}
	if strings.Contains(output, `"forged"`) {
		t.Fatalf("cleanup log must not contain client-submitted keys; log=%q", output)
	}
}

// TestPublishVideoPosterDerivationFailureRejectsPublish: when the poster
// object's dimensions cannot be derived, the publish must fail (the poster is
// not trustworthy) and the poster grant must be restored.
func TestPublishVideoPosterDerivationFailureRejectsPublish(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	svc.ossSvc = &OSSService{cfg: &config.Config{OSS: config.OSSConfig{Domain: "https://cdn.example.test"}}}
	svc.imageDimensions = &fakeImageDimensionsResolver{err: errors.New("not an image")}

	posterGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/poster.bin",
		FileType: "image", MimeType: "image/png", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue poster grant: %v", err)
	}
	videoGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/clip.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue video grant: %v", err)
	}

	_, err = svc.PublishContent(PublishContentInput{
		Title:         "video",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: posterGrant.ID,
		PosterWidth:   intPtr(640),
		PosterHeight:  intPtr(360),
		Attachments: []AttachmentInput{
			{GrantID: videoGrant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if err == nil {
		t.Fatal("publish must fail when poster dimensions cannot be derived")
	}
	if !errors.Is(err, ErrMediaSetInvalid) {
		t.Fatalf("error = %v, want ErrMediaSetInvalid wrapper", err)
	}
	if _, err := grants.Consume(ctx, posterGrant.ID, 42, "content"); err != nil {
		t.Fatal("poster grant must be restored (re-consumable) when publish fails")
	}
}

// TestPublishVideoPosterGrantMustBeImage: a video poster grant carrying a
// non-image object is rejected, and the consumed grant is restored so the
// publisher can retry.
func TestPublishVideoPosterGrantMustBeImage(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	issueGrant := func(fileType, key string) UploadGrant {
		grant, err := grants.Issue(ctx, UploadGrant{
			UserID: 42, Purpose: "content", OSSKey: key, FileType: fileType,
			MimeType: "video/mp4", FileSize: 123,
		})
		if err != nil {
			t.Fatalf("issue grant: %v", err)
		}
		return *grant
	}
	videoGrant := issueGrant("video", "uploads/42/video/clip.mp4")
	posterGrant := issueGrant("video", "uploads/42/video/not-an-image.mp4")

	_, err := svc.PublishContent(PublishContentInput{
		Title:         "video",
		Zone:          "original",
		Category:      "game",
		ContentType:   "video",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: posterGrant.ID,
		PosterWidth:   intPtr(640),
		PosterHeight:  intPtr(360),
		Attachments: []AttachmentInput{
			{GrantID: videoGrant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if !errors.Is(err, ErrMediaSetInvalid) {
		t.Fatalf("publish err = %v, want ErrMediaSetInvalid", err)
	}
	if _, err := grants.Consume(ctx, posterGrant.ID, 42, "content"); err != nil {
		t.Fatalf("poster grant must be restored after a rejected publish: %v", err)
	}
}

func TestPublishVideoRejectsPosterGrantWithMismatchedMime(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	videoGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/clip.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue video grant: %v", err)
	}
	posterGrant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/image/not-image.bin",
		FileType: "image", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue poster grant: %v", err)
	}

	_, err = svc.PublishContent(PublishContentInput{
		Title: "video", Zone: "original", Category: "game", ContentType: "video",
		IsPublic: true, AllowCopy: true, PosterGrantID: posterGrant.ID,
		PosterWidth: intPtr(640), PosterHeight: intPtr(360),
		Attachments: []AttachmentInput{{
			GrantID: videoGrant.ID, FileType: "video", SortOrder: intPtr(0),
			Width: intPtr(1280), Height: intPtr(720),
		}},
	}, 42)
	if !errors.Is(err, ErrMediaSetInvalid) {
		t.Fatalf("publish err = %v, want ErrMediaSetInvalid", err)
	}
	if _, err := grants.Consume(ctx, posterGrant.ID, 42, "content"); err != nil {
		t.Fatalf("poster grant must be restored after a rejected publish: %v", err)
	}
}

// TestPublishImageRejectsPosterGrant: poster grants are video-only; image
// content carrying one is rejected before any grant is consumed.
func TestPublishImageRejectsPosterGrant(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	issueGrant := func(key string) UploadGrant {
		grant, err := grants.Issue(ctx, UploadGrant{
			UserID: 42, Purpose: "content", OSSKey: key, FileType: "image",
			MimeType: "image/png", FileSize: 123,
		})
		if err != nil {
			t.Fatalf("issue grant: %v", err)
		}
		return *grant
	}
	g1 := issueGrant("uploads/42/image/a.png")
	g2 := issueGrant("uploads/42/image/b.png")
	poster := issueGrant("uploads/42/image/poster.png")

	_, err := svc.PublishContent(PublishContentInput{
		Title:         "gallery",
		Zone:          "original",
		Category:      "game",
		ContentType:   "image",
		IsPublic:      true,
		AllowCopy:     true,
		PosterGrantID: poster.ID,
		PosterWidth:   intPtr(640),
		PosterHeight:  intPtr(360),
		Attachments: []AttachmentInput{
			{GrantID: g1.ID, FileType: "image", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(100), Height: intPtr(200)},
			{GrantID: g2.ID, FileType: "image", OSSKey: "forged", SortOrder: intPtr(1), Width: intPtr(300), Height: intPtr(400)},
		},
	}, 42)
	if !errors.Is(err, ErrMediaSetInvalid) {
		t.Fatalf("publish err = %v, want ErrMediaSetInvalid", err)
	}
	if _, err := grants.Consume(ctx, poster.ID, 42, "content"); err != nil {
		t.Fatalf("poster grant must remain unconsumed after a rejected publish: %v", err)
	}
}

// New video publishes require a verified poster grant. Historical rows remain
// readable through the existing compatibility ordering path.
func TestPublishVideoWithoutPosterIsRejected(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	ctx := t.Context()

	grant, err := grants.Issue(ctx, UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/video/legacy.mp4",
		FileType: "video", MimeType: "video/mp4", FileSize: 123,
	})
	if err != nil {
		t.Fatalf("issue grant: %v", err)
	}

	_, err = svc.PublishContent(PublishContentInput{
		Title:       "legacy-video",
		Zone:        "original",
		Category:    "game",
		ContentType: "video",
		IsPublic:    true,
		AllowCopy:   true,
		Attachments: []AttachmentInput{
			{GrantID: grant.ID, FileType: "video", OSSKey: "forged", SortOrder: intPtr(0), Width: intPtr(1280), Height: intPtr(720)},
		},
	}, 42)
	if !errors.Is(err, ErrMediaSetInvalid) {
		t.Fatalf("publish err = %v, want ErrMediaSetInvalid", err)
	}
}

// TestPersistentObjectURL covers stable delivery-domain derivation. A private
// bucket endpoint must never be exposed as an unsigned fallback URL.
func TestPersistentObjectURL(t *testing.T) {
	cfg := func(domain, endpoint, bucket string) *config.Config {
		return &config.Config{OSS: config.OSSConfig{Domain: domain, Endpoint: endpoint, BucketName: bucket}}
	}
	tests := []struct {
		name string
		svc  *OSSService
		key  string
		want string
	}{
		{name: "cdn domain preferred", svc: &OSSService{cfg: cfg("https://cdn.example.test", "", "")}, key: "uploads/1.png", want: "https://cdn.example.test/uploads/1.png"},
		{name: "cdn domain trailing slash trimmed", svc: &OSSService{cfg: cfg("https://cdn.example.test/", "", "")}, key: "uploads/1.png", want: "https://cdn.example.test/uploads/1.png"},
		{name: "private bucket has no unsigned fallback", svc: &OSSService{cfg: cfg("", "https://oss-cn-shanghai.aliyuncs.com", "omnicraft")}, key: "uploads/1.png", want: ""},
		{name: "empty key", svc: &OSSService{cfg: cfg("https://cdn.example.test", "", "")}, key: " ", want: ""},
		{name: "empty key trims", svc: &OSSService{cfg: cfg("https://cdn.example.test", "", "")}, key: "  uploads/1.png  ", want: "https://cdn.example.test/uploads/1.png"},
		{name: "empty config", svc: &OSSService{}, key: "uploads/1.png", want: ""},
		{name: "nil service", svc: nil, key: "uploads/1.png", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.svc.PersistentObjectURL(tt.key); got != tt.want {
				t.Fatalf("PersistentObjectURL(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func intPtr(v int) *int {
	return &v
}
