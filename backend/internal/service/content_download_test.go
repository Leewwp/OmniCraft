package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// SP-16 #451: the download orchestration previously lived inline in the REST
// handler; it moves into ContentService.RequestDownload so the MCP tool
// omnicraft_request_download reuses byte-identical semantics (visibility,
// AllowCopy, archive gate, metering) instead of duplicating them.

type fakeDownloadSigner struct {
	calls []string
}

func (f *fakeDownloadSigner) GeneratePresignDownloadURL(ctx context.Context, ossKey string, ttl time.Duration) (string, error) {
	f.calls = append(f.calls, ossKey)
	return "https://signed.example.com/" + ossKey + "?ttl=" + ttl.String(), nil
}

func newDownloadTestService(t *testing.T) (*ContentService, *fakeDownloadSigner, *redis.Client, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentAttachment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	repo := repository.NewContentRepository(db)
	svc := NewContentServiceWithDeps(repo, nil, rdb)
	signer := &fakeDownloadSigner{}
	svc = svc.WithDownloadSigner(signer)
	return svc, signer, rdb, db
}

func seedDownloadFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now()
	if err := db.Create(&model.User{ID: 501, Email: "dl-author@example.com", Username: "dl_author", PasswordHash: "x", Reputation: 10, Role: "user", EmailVerifiedAt: &now}).Error; err != nil {
		t.Fatalf("seed author: %v", err)
	}
	viewer := model.User{ID: 502, Email: "dl-viewer@example.com", Username: "dl_viewer", PasswordHash: "x", Reputation: 10, Role: "user", EmailVerifiedAt: &now}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatalf("seed viewer: %v", err)
	}
}

func seedDownloadContent(t *testing.T, db *gorm.DB, id int64, status string, isPublic bool, allowCopy bool, attachments int) {
	t.Helper()
	c := model.ContentItem{
		ID: id, Title: "dl fixture", AuthorID: 501, Zone: "original", Category: "game",
		ContentType: "sheet_music", Status: status, IsPublic: isPublic, AllowCopy: allowCopy,
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("seed content %d: %v", id, err)
	}
	for i := 0; i < attachments; i++ {
		// Two attachments both claim primary on purpose: the picker must
		// refuse to guess without an explicit attachment_id.
		primary := true
		size := int64(1234)
		att := model.ContentAttachment{
			ContentItemID: id, FileType: "file", MimeType: "application/zip",
			OSSKey: "uploads/sp16dl/" + string(rune('a'+i)), FileSize: &size, ScanStatus: "not_required",
			IsPrimary: &primary,
		}
		if err := db.Create(&att).Error; err != nil {
			t.Fatalf("seed attachment: %v", err)
		}
	}
}

func TestRequestDownloadHappyPath(t *testing.T) {
	svc, signer, rdb, db := newDownloadTestService(t)
	seedDownloadFixture(t, db)
	seedDownloadContent(t, db, 601, "published", true, true, 1)

	res, err := svc.RequestDownload(context.Background(), 502, 601, "")
	if err != nil {
		t.Fatalf("RequestDownload: %v", err)
	}
	if res.URL == "" || res.ExpiresIn <= 0 {
		t.Errorf("url/expires_in not populated: %+v", res)
	}
	if res.Attachment == nil || res.Attachment.FileSize == nil || *res.Attachment.FileSize != 1234 {
		t.Errorf("attachment metadata missing: %+v", res.Attachment)
	}
	if len(signer.calls) != 1 {
		t.Errorf("presign called %d times, want 1", len(signer.calls))
	}
	// The count is recorded asynchronously (GoSafe); poll briefly.
	var score float64
	deadline := time.Now().Add(2 * time.Second)
	for {
		var err error
		score, err = rdb.ZScore(context.Background(), "rank:download:counts", "601").Result()
		if err == nil && score == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("download count not recorded via redis fallback: score=%v err=%v", score, err)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRequestDownloadAuthAndVisibility(t *testing.T) {
	svc, _, _, db := newDownloadTestService(t)
	seedDownloadFixture(t, db)

	if _, err := svc.RequestDownload(context.Background(), 0, 601, ""); !errors.Is(err, ErrDownloadUnauthorized) {
		t.Errorf("anonymous caller: got %v, want ErrDownloadUnauthorized", err)
	}

	seedDownloadContent(t, db, 601, "published", true, true, 1)
	if _, err := svc.RequestDownload(context.Background(), 502, 999, ""); !errors.Is(err, ErrContentNotFound) {
		t.Errorf("missing content: got %v, want ErrContentNotFound", err)
	}

	seedDownloadContent(t, db, 602, "pending", true, true, 1)
	if _, err := svc.RequestDownload(context.Background(), 502, 602, ""); !errors.Is(err, ErrDownloadNotPublished) {
		t.Errorf("pending content: got %v, want ErrDownloadNotPublished", err)
	}

	seedDownloadContent(t, db, 603, "published", false, true, 1)
	if _, err := svc.RequestDownload(context.Background(), 502, 603, ""); !errors.Is(err, ErrDownloadUnavailable) {
		t.Errorf("private content: got %v, want ErrDownloadUnavailable", err)
	}

	seedDownloadContent(t, db, 604, "published", true, false, 1)
	if _, err := svc.RequestDownload(context.Background(), 502, 604, ""); !errors.Is(err, ErrDownloadNotAllowed) {
		t.Errorf("allow_copy=false: got %v, want ErrDownloadNotAllowed", err)
	}
}

func TestRequestDownloadAttachmentSelection(t *testing.T) {
	svc, _, _, db := newDownloadTestService(t)
	seedDownloadFixture(t, db)

	seedDownloadContent(t, db, 611, "published", true, true, 0)
	if _, err := svc.RequestDownload(context.Background(), 502, 611, ""); !errors.Is(err, ErrNoAttachments) {
		t.Errorf("no attachments: got %v, want ErrNoAttachments", err)
	}

	seedDownloadContent(t, db, 612, "published", true, true, 2)
	if _, err := svc.RequestDownload(context.Background(), 502, 612, ""); !errors.Is(err, ErrAmbiguousAttachment) {
		t.Errorf("two non-unique primaries without attachment_id: got %v, want ErrAmbiguousAttachment", err)
	}
	if _, err := svc.RequestDownload(context.Background(), 502, 612, "not-a-number"); !errors.Is(err, ErrInvalidAttachmentID) {
		t.Errorf("non-numeric attachment_id: got %v, want ErrInvalidAttachmentID", err)
	}

	var att model.ContentAttachment
	if err := db.Where("content_item_id = ?", 612).Order("id ASC").First(&att).Error; err != nil {
		t.Fatalf("load attachment: %v", err)
	}
	res, err := svc.RequestDownload(context.Background(), 502, 612, strconv.FormatInt(att.ID, 10))
	if err != nil {
		t.Fatalf("explicit attachment_id: %v", err)
	}
	if res.Attachment == nil || res.Attachment.ID != att.ID {
		t.Errorf("explicit attachment not honored: %+v", res.Attachment)
	}

	other := model.ContentAttachment{ContentItemID: 611, FileType: "file", OSSKey: "uploads/sp16dl/other"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("seed foreign attachment: %v", err)
	}
	if _, err := svc.RequestDownload(context.Background(), 502, 612, strconv.FormatInt(other.ID, 10)); !errors.Is(err, ErrAttachmentMismatch) {
		t.Errorf("foreign attachment_id: got %v, want ErrAttachmentMismatch", err)
	}
}

func TestDownloadURLTTLPolicy(t *testing.T) {
	archive := model.ContentAttachment{FileType: "mod", ScanRequired: true}
	if got := downloadURLTTL(300, true, 120, archive); got != 120*time.Second {
		t.Fatalf("archive ttl = %s, want 2m", got)
	}
	if got := downloadURLTTL(300, true, 900, archive); got != 300*time.Second {
		t.Fatalf("capped archive ttl = %s, want 5m", got)
	}
	nonArchive := model.ContentAttachment{FileType: "image"}
	if got := downloadURLTTL(300, true, 120, nonArchive); got != 300*time.Second {
		t.Fatalf("non-archive ttl = %s, want OSS ttl", got)
	}
	if got := downloadURLTTL(0, false, 0, nonArchive); got != 300*time.Second {
		t.Fatalf("default ttl = %s, want 5m fallback", got)
	}
}

func TestRequestDownloadArchiveGate(t *testing.T) {
	svc, _, _, db := newDownloadTestService(t)
	seedDownloadFixture(t, db)
	seedDownloadContent(t, db, 621, "published", true, true, 1)

	svc.archiveScanEnabled = true
	var att model.ContentAttachment
	if err := db.Where("content_item_id = ?", 621).First(&att).Error; err != nil {
		t.Fatalf("load attachment: %v", err)
	}
	if err := db.Model(&att).Update("scan_status", "infected").Error; err != nil {
		t.Fatalf("mark infected: %v", err)
	}
	if _, err := svc.RequestDownload(context.Background(), 502, 621, ""); !errors.Is(err, ErrArchiveNotClean) {
		t.Errorf("infected archive served: got %v, want ErrArchiveNotClean", err)
	}
}
