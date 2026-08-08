package service

import (
	"errors"
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
// dimensions come from the poster grant, never from the client.
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
	posterGrant := issueGrant("image", "uploads/42/image/poster.jpg", "image/jpeg")

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
	if want := "https://cdn.example.test/uploads/42/image/poster.jpg"; content.CoverImageURL != want {
		t.Fatalf("cover URL = %q, want %q (verified poster)", content.CoverImageURL, want)
	}
	if content.CoverWidth == nil || *content.CoverWidth != 640 || content.CoverHeight == nil || *content.CoverHeight != 360 {
		t.Fatalf("cover dims = (%v,%v), want poster (640,360)", content.CoverWidth, content.CoverHeight)
	}
	if verifier.calls != 2 {
		t.Fatalf("verify calls = %d, want 2 (media item + poster)", verifier.calls)
	}
	if _, err := grants.Consume(ctx, posterGrant.ID, 42, "content"); err == nil {
		t.Fatal("poster grant must be consumed by a successful publish")
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

// TestPublishLegacyVideoWithoutPosterStillSucceeds: legacy clients that do not
// send a poster grant keep the pre-contract behavior (server-side snapshot
// fallback); the publish itself must not fail and the legacy single-video set
// stays readable.
func TestPublishLegacyVideoWithoutPosterStillSucceeds(t *testing.T) {
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

	content, err := svc.PublishContent(PublishContentInput{
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
	if err != nil {
		t.Fatalf("legacy video publish must not fail: %v", err)
	}
	rows, err := svc.contentRepo.GetAttachments(content.ID)
	if err != nil {
		t.Fatalf("get attachments: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("legacy video attachments = %d, want 1", len(rows))
	}
}

// TestPersistentObjectURL covers the stable unsigned public URL derivation:
// CDN domain preferred, bucket-qualified endpoint fallback, empty inputs and a
// nil service yield "".
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
		{name: "endpoint fallback https", svc: &OSSService{cfg: cfg("", "https://oss-cn-shanghai.aliyuncs.com", "omnicraft")}, key: "uploads/1.png", want: "https://omnicraft.oss-cn-shanghai.aliyuncs.com/uploads/1.png"},
		{name: "endpoint fallback http", svc: &OSSService{cfg: cfg("", "http://oss-cn-shanghai.aliyuncs.com", "omnicraft")}, key: "uploads/1.png", want: "https://omnicraft.oss-cn-shanghai.aliyuncs.com/uploads/1.png"},
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
