package service

import (
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/aliyun"
)

// TestGeneratePresignUploadURLRestrictsImageMIMEToParserSupported proves the
// content upload contract (media gallery items and video posters) accepts
// exactly the MIME types the imageinfo parser can decode (PNG/JPEG/WebP).
// Accepting any image/* would let an upload succeed only to fail at publish
// time when dimensions cannot be derived.
func TestGeneratePresignUploadURLRestrictsImageMIMEToParserSupported(t *testing.T) {
	svc := newTestOSSService(t)

	accepted := []string{"image/png", "image/jpeg", "image/webp", "IMAGE/PNG", " image/jpeg "}
	for _, mime := range accepted {
		if _, err := svc.GeneratePresignUploadURL(t.Context(), PresignUploadRequest{
			FileName: "poster.png", FileType: "image", MimeType: mime, FileSize: 1024,
		}, 42); err != nil {
			t.Fatalf("GeneratePresignUploadURL(image, %q) = %v, want nil", mime, err)
		}
	}

	rejected := []string{"image/gif", "image/tiff", "image/svg+xml", "image/avif", "image/bmp", "image/heic"}
	for _, mime := range rejected {
		_, err := svc.GeneratePresignUploadURL(t.Context(), PresignUploadRequest{
			FileName: "poster.gif", FileType: "image", MimeType: mime, FileSize: 1024,
		}, 42)
		if err == nil {
			t.Fatalf("GeneratePresignUploadURL(image, %q) = nil, want UploadValidationError", mime)
		}
		if _, ok := err.(*UploadValidationError); !ok {
			t.Fatalf("GeneratePresignUploadURL(image, %q) err = %T, want *UploadValidationError", mime, err)
		}
	}
}

// TestGeneratePresignUploadURLKeepsOtherTypesOnTheirOwnContract proves video
// and text uploads keep their existing MIME contracts and are not caught by
// the parser-supported image restriction.
func TestGeneratePresignUploadURLKeepsOtherTypesOnTheirOwnContract(t *testing.T) {
	svc := newTestOSSService(t)

	if _, err := svc.GeneratePresignUploadURL(t.Context(), PresignUploadRequest{
		FileName: "clip.mp4", FileType: "video", MimeType: "video/mp4", FileSize: 1024,
	}, 42); err != nil {
		t.Fatalf("video/mp4 presign = %v, want nil", err)
	}
	if _, err := svc.GeneratePresignUploadURL(t.Context(), PresignUploadRequest{
		FileName: "notes.pdf", FileType: "text", MimeType: "application/pdf", FileSize: 1024,
	}, 42); err != nil {
		t.Fatalf("application/pdf text presign = %v, want nil", err)
	}
}

// newTestOSSService builds an OSSService with a locally-constructed client:
// NewOSSClient only assembles the SDK objects (no network I/O), so presign
// validation logic is fully exercisable without real OSS credentials.
func newTestOSSService(t *testing.T) *OSSService {
	t.Helper()
	client, err := aliyun.NewOSSClient("https://oss-cn-hangzhou.aliyuncs.com", "test-access-key", "test-access-secret", "test-bucket")
	if err != nil {
		t.Fatalf("build test OSS client: %v", err)
	}
	return &OSSService{cfg: &config.Config{
		Limits: config.LimitsConfig{ImageMaxMB: 20, VideoMaxMB: 300, VideoMaxSec: 180, TextMaxMB: 10, ModMaxMB: 500, SheetMusicMaxMB: 50},
		Upload: config.UploadConfig{SheetMusicExtensions: []string{".pdf"}},
	}, client: client}
}
