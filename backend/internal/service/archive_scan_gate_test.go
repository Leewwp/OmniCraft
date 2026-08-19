package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/archivezip"
	"omnicraft/backend/internal/repository"
)

func TestArchiveScanGateAttachmentStateMatrix(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.ContentAttachment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	statuses := []string{
		model.ScanStatusPending,
		model.ScanStatusScanning,
		model.ScanStatusFailed,
		model.ScanStatusBlocked,
		model.ScanStatusManualReview,
		model.ScanStatusLegacyUnscanned,
	}
	for _, status := range statuses {
		attachment := model.ContentAttachment{
			ContentItemID: 1,
			FileType:      "mod",
			OSSKey:        "uploads/mod.zip",
			ScanStatus:    status,
			ScanRequired:  true,
		}
		if err := db.Create(&attachment).Error; err != nil {
			t.Fatalf("create %s attachment: %v", status, err)
		}
		gate := NewArchiveScanGate(db, true)
		if err := gate.RequireAttachmentClean(context.Background(), attachment.ID); !errors.Is(err, ErrArchiveNotClean) {
			t.Fatalf("status %s error = %v, want ErrArchiveNotClean", status, err)
		}
	}

	clean := model.ContentAttachment{
		ContentItemID: 1,
		FileType:      "mod",
		OSSKey:        "uploads/mod-clean.zip",
		ScanStatus:    model.ScanStatusClean,
		ScanRequired:  true,
	}
	if err := db.Create(&clean).Error; err != nil {
		t.Fatalf("create clean attachment: %v", err)
	}
	if err := NewArchiveScanGate(db, true).RequireAttachmentClean(context.Background(), clean.ID); err != nil {
		t.Fatalf("clean attachment error = %v, want nil", err)
	}

	notRequired := model.ContentAttachment{
		ContentItemID: 1,
		FileType:      "image",
		OSSKey:        "uploads/cover.png",
		ScanStatus:    model.ScanStatusNotRequired,
	}
	if err := db.Create(&notRequired).Error; err != nil {
		t.Fatalf("create not-required attachment: %v", err)
	}
	if err := NewArchiveScanGate(db, true).RequireAttachmentClean(context.Background(), notRequired.ID); err != nil {
		t.Fatalf("not-required attachment error = %v, want nil", err)
	}
	quarantinedNonArchive := model.ContentAttachment{
		ContentItemID: 1,
		FileType:      "image",
		OSSKey:        "quarantine/archive-scan/1/1/1",
		ScanStatus:    model.ScanStatusNotRequired,
	}
	if err := db.Create(&quarantinedNonArchive).Error; err != nil {
		t.Fatalf("create quarantined non-archive attachment: %v", err)
	}
	if err := NewArchiveScanGate(db, true).RequireAttachmentClean(context.Background(), quarantinedNonArchive.ID); !errors.Is(err, ErrArchiveNotClean) {
		t.Fatalf("quarantined non-archive error = %v, want ErrArchiveNotClean", err)
	}
	if err := NewArchiveScanGate(db, false).RequireAttachmentClean(context.Background(), quarantinedNonArchive.ID); !errors.Is(err, ErrArchiveNotClean) {
		t.Fatalf("disabled gate quarantined error = %v, want ErrArchiveNotClean", err)
	}
	modNotRequired := model.ContentAttachment{
		ContentItemID: 1,
		FileType:      "mod",
		OSSKey:        "uploads/legacy-mod.zip",
		ScanStatus:    model.ScanStatusNotRequired,
	}
	if err := db.Create(&modNotRequired).Error; err != nil {
		t.Fatalf("create not-required mod attachment: %v", err)
	}
	if err := NewArchiveScanGate(db, true).RequireAttachmentClean(context.Background(), modNotRequired.ID); !errors.Is(err, ErrArchiveNotClean) {
		t.Fatalf("not-required mod error = %v, want ErrArchiveNotClean", err)
	}
	foreignPrefix := model.ContentAttachment{
		ContentItemID: 1,
		FileType:      "mod",
		OSSKey:        "staging/mod-clean.zip",
		ScanStatus:    model.ScanStatusClean,
		ScanRequired:  true,
	}
	if err := db.Create(&foreignPrefix).Error; err != nil {
		t.Fatalf("create foreign-prefix attachment: %v", err)
	}
	if err := NewArchiveScanGate(db, true).RequireAttachmentClean(context.Background(), foreignPrefix.ID); !errors.Is(err, ErrArchiveNotClean) {
		t.Fatalf("foreign-prefix error = %v, want ErrArchiveNotClean", err)
	}
	if err := NewArchiveScanGate(db, false).RequireAttachmentClean(context.Background(), attachmentIDOrFail(t, db, 1)); err != nil {
		t.Fatalf("disabled gate error = %v, want nil", err)
	}
}

func TestArchiveScanGateContentPublishRequiresEveryArchiveClean(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.ContentAttachment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	content := model.ContentItem{Title: "mod", AuthorID: 1, Zone: "fanwork", ContentType: "mod", Status: "pending"}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	attachments := []model.ContentAttachment{
		{ContentItemID: content.ID, FileType: "mod", OSSKey: "uploads/a.zip", ScanStatus: model.ScanStatusClean, ScanRequired: true},
		{ContentItemID: content.ID, FileType: "mod", OSSKey: "uploads/b.zip", ScanStatus: model.ScanStatusPending, ScanRequired: true},
	}
	if err := db.Create(&attachments).Error; err != nil {
		t.Fatalf("create attachments: %v", err)
	}
	gate := NewArchiveScanGate(db, true)
	if err := gate.RequireContentCleanTx(context.Background(), db, content.ID); !errors.Is(err, ErrArchiveNotClean) {
		t.Fatalf("mixed content error = %v, want ErrArchiveNotClean", err)
	}
	if err := db.Model(&model.ContentAttachment{}).Where("id = ?", attachments[1].ID).Update("scan_status", model.ScanStatusClean).Error; err != nil {
		t.Fatalf("clean second attachment: %v", err)
	}
	if err := gate.RequireContentCleanTx(context.Background(), db, content.ID); err != nil {
		t.Fatalf("all-clean content error = %v, want nil", err)
	}
	quarantined := model.ContentAttachment{
		ContentItemID: content.ID,
		FileType:      "image",
		OSSKey:        "quarantine/archive-scan/1/1/1",
		ScanStatus:    model.ScanStatusNotRequired,
	}
	if err := db.Create(&quarantined).Error; err != nil {
		t.Fatalf("create quarantined attachment: %v", err)
	}
	if err := NewArchiveScanGate(db, false).RequireContentCleanTx(context.Background(), db, content.ID); !errors.Is(err, ErrArchiveNotClean) {
		t.Fatalf("disabled content gate error = %v, want ErrArchiveNotClean", err)
	}
}

func TestArchiveScanCleanRequiresLatestReviewPass(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.ContentAttachment{}, &model.AIReviewRecord{}); err != nil {
		t.Fatalf("migrate review fixture: %v", err)
	}
	content := model.ContentItem{Title: "reviewed mod", AuthorID: 1, Zone: "original", ContentType: "mod", Status: "under_review"}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	attachment := model.ContentAttachment{ContentItemID: content.ID, FileType: "mod", OSSKey: "uploads/reviewed.zip", ScanStatus: model.ScanStatusClean, ScanRequired: true}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	now := time.Now()
	if err := db.Create(&model.AIReviewRecord{TargetType: "content", TargetID: content.ID, Provider: "test", Result: "pass", ScannedAt: now}).Error; err != nil {
		t.Fatalf("create pass review: %v", err)
	}
	if err := db.Create(&model.AIReviewRecord{TargetType: "content", TargetID: content.ID, Provider: "test", Result: "review", ScannedAt: now.Add(time.Second)}).Error; err != nil {
		t.Fatalf("create newer review: %v", err)
	}
	svc := NewReviewService(db, nil, &config.Config{}, nil)
	svc.SetArchiveScanGate(NewArchiveScanGate(db, true))
	if err := svc.ArchiveScanClean(context.Background(), attachment.ID); err != nil {
		t.Fatalf("ArchiveScanClean() error = %v", err)
	}
	var current model.ContentItem
	if err := db.First(&current, content.ID).Error; err != nil {
		t.Fatalf("load content: %v", err)
	}
	if current.Status != "under_review" {
		t.Fatalf("content status = %q, want under_review after newer review result", current.Status)
	}
}

func TestPublishModCreatesPendingArchiveJobBeforeReviewCanPublish(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	db := svc.contentRepo.DB()
	if err := db.AutoMigrate(&model.ArchiveScanJob{}); err != nil {
		t.Fatalf("migrate archive jobs: %v", err)
	}
	svc.SetArchiveScanRepository(repository.NewArchiveScanRepository(db, repository.ArchiveScanRetryPolicy{}), true)
	svc.WithArchiveScanConfig(&config.ArchiveScanConfig{})
	svc.SetArchiveValidator(archiveGateValidator{})

	grant, err := grants.Issue(context.Background(), UploadGrant{
		UserID:   42,
		Purpose:  "content",
		OSSKey:   "uploads/42/mod/package.zip",
		FileType: "mod",
		MimeType: "application/zip",
		FileSize: 512,
	})
	if err != nil {
		t.Fatalf("issue mod grant: %v", err)
	}
	content, err := svc.PublishContent(PublishContentInput{
		Title:       "scanned mod",
		Zone:        "original",
		Category:    "game",
		ContentType: "mod",
		IsPublic:    true,
		AllowCopy:   true,
		Attachments: []AttachmentInput{{GrantID: grant.ID, FileType: "mod", MimeType: "application/zip"}},
	}, 42)
	if err != nil {
		t.Fatalf("publish mod: %v", err)
	}
	var attachment model.ContentAttachment
	if err := db.Where("content_item_id = ?", content.ID).First(&attachment).Error; err != nil {
		t.Fatalf("load mod attachment: %v", err)
	}
	if attachment.ScanStatus != model.ScanStatusPending || !attachment.ScanRequired {
		t.Fatalf("attachment scan state = %#v, want pending/required", attachment)
	}
	var job model.ArchiveScanJob
	if err := db.Where("attachment_id = ?", attachment.ID).First(&job).Error; err != nil {
		t.Fatalf("load pending archive job: %v", err)
	}
	if job.Status != model.ScanStatusPending || job.ScanVersion != 1 {
		t.Fatalf("archive job = %#v, want pending version 1", job)
	}
}

func TestPublishModRejectsInvalidArchiveBeforeScanJob(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	db := svc.contentRepo.DB()
	if err := db.AutoMigrate(&model.ArchiveScanJob{}); err != nil {
		t.Fatalf("migrate archive jobs: %v", err)
	}
	svc.SetArchiveScanRepository(repository.NewArchiveScanRepository(db, repository.ArchiveScanRetryPolicy{}), true)
	svc.WithArchiveScanConfig(&config.ArchiveScanConfig{})
	svc.SetArchiveValidator(archiveGateValidator{err: archivezip.ErrPathInvalid})
	grant, err := grants.Issue(context.Background(), UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/mod/invalid.zip",
		FileType: "mod", MimeType: "application/zip", FileSize: 512,
	})
	if err != nil {
		t.Fatalf("issue mod grant: %v", err)
	}
	_, err = svc.PublishContent(PublishContentInput{
		Title: "invalid mod", Zone: "original", Category: "game", ContentType: "mod",
		IsPublic: true, AllowCopy: true,
		Attachments: []AttachmentInput{{GrantID: grant.ID, FileType: "mod", MimeType: "application/zip"}},
	}, 42)
	if !errors.Is(err, archivezip.ErrPathInvalid) {
		t.Fatalf("publish error = %v, want ARCHIVE_PATH_INVALID", err)
	}
	var jobs int64
	if err := db.Model(&model.ArchiveScanJob{}).Count(&jobs).Error; err != nil {
		t.Fatalf("count scan jobs: %v", err)
	}
	if jobs != 0 {
		t.Fatalf("scan jobs = %d, want 0 after structural rejection", jobs)
	}
}

func TestPublishModRequiresArchiveAttachmentWhenScanningEnabled(t *testing.T) {
	svc, _, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	svc.SetArchiveScanRepository(repository.NewArchiveScanRepository(svc.contentRepo.DB(), repository.ArchiveScanRetryPolicy{}), true)
	_, err := svc.PublishContent(PublishContentInput{Title: "missing mod", Zone: "original", Category: "game", ContentType: "mod"}, 42)
	if !errors.Is(err, ErrArchiveAttachmentRequired) {
		t.Fatalf("publish error = %v, want ErrArchiveAttachmentRequired", err)
	}
}

func TestPublishModFailsClosedWhenArchiveRepositoryIsMissing(t *testing.T) {
	svc, grants, _, cleanup := newContentGrantPublishService(t)
	defer cleanup()
	svc.archiveScanEnabled = true
	svc.archiveScanRepo = nil
	svc.WithArchiveScanConfig(&config.ArchiveScanConfig{})
	svc.SetArchiveValidator(archiveGateValidator{})
	grant, err := grants.Issue(context.Background(), UploadGrant{
		UserID: 42, Purpose: "content", OSSKey: "uploads/42/mod/missing-repo.zip",
		FileType: "mod", MimeType: "application/zip", FileSize: 512,
	})
	if err != nil {
		t.Fatalf("issue mod grant: %v", err)
	}
	_, err = svc.PublishContent(PublishContentInput{
		Title: "missing scan repository", Zone: "original", Category: "game", ContentType: "mod",
		IsPublic: true, AllowCopy: true,
		Attachments: []AttachmentInput{{GrantID: grant.ID, FileType: "mod", MimeType: "application/zip"}},
	}, 42)
	if !errors.Is(err, ErrArchiveScanUnavailable) {
		t.Fatalf("publish error = %v, want ErrArchiveScanUnavailable", err)
	}
}

type archiveGateValidator struct{ err error }

func (v archiveGateValidator) ValidateArchiveStructure(context.Context, string, int64, archivezip.Quota) error {
	return v.err
}

func attachmentIDOrFail(t *testing.T, db *gorm.DB, id int64) int64 {
	t.Helper()
	var attachment model.ContentAttachment
	if err := db.First(&attachment, id).Error; err != nil {
		t.Fatalf("load attachment %d: %v", id, err)
	}
	return attachment.ID
}
