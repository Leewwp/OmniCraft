package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// newInitialVersionPublishService builds a minimal publish stack over sqlite.
// With wireVersions it mirrors the production container assembly so the
// FIX-42 contract (publish creates the initial v1 snapshot) is exercised
// end to end at the service layer.
func newInitialVersionPublishService(t *testing.T, wireVersions bool) (*ContentService, *VersionService, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	// Single connection so publish-transaction writes are visible to later
	// reads (in-memory sqlite databases are per-connection).
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{}, &model.ContentVersion{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewContentServiceWithDeps(repository.NewContentRepository(db), nil, nil)
	var versionSvc *VersionService
	if wireVersions {
		versionSvc = NewVersionService(repository.NewVersionRepository(db), repository.NewContentRepository(db))
		svc.SetVersionService(versionSvc)
	}
	return svc, versionSvc, func() {}
}

func baseInitialVersionInput() PublishContentInput {
	return PublishContentInput{
		Title:       "versioned publish",
		Description: "v1 body snapshot",
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		IsPublic:    true,
	}
}

func TestPublishContentCreatesInitialVersion(t *testing.T) {
	svc, versionSvc, cleanup := newInitialVersionPublishService(t, true)
	defer cleanup()

	content, err := svc.PublishContent(baseInitialVersionInput(), 42)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	versions, err := versionSvc.ListVersions(content.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("versions = %d, want exactly 1 initial version after publish", len(versions))
	}
	v := versions[0]
	if v.ContentItemID != content.ID {
		t.Fatalf("version content id = %d, want %d", v.ContentItemID, content.ID)
	}
	if v.AuthorID != 42 {
		t.Fatalf("version author id = %d, want 42", v.AuthorID)
	}
	if v.VersionNumber != 1 {
		t.Fatalf("version number = %d, want 1", v.VersionNumber)
	}
	if v.StorageType != "full" {
		t.Fatalf("storage type = %q, want full", v.StorageType)
	}
	if v.StorageKey != baseInitialVersionInput().Description {
		t.Fatalf("storage key = %q, want full description body", v.StorageKey)
	}
	if v.Status != "active" {
		t.Fatalf("status = %q, want active", v.Status)
	}
	if !v.IsLatest {
		t.Fatal("initial version must be marked latest")
	}
}

func TestPublishContentWithoutVersionServiceStillSucceeds(t *testing.T) {
	svc, _, cleanup := newInitialVersionPublishService(t, false)
	defer cleanup()

	// Local constructor call sites (handler/admin) build ContentService
	// without a version service; publishing must keep working unchanged.
	content, err := svc.PublishContent(baseInitialVersionInput(), 42)
	if err != nil {
		t.Fatalf("publish without version service: %v", err)
	}
	if content == nil || content.ID == 0 {
		t.Fatalf("content not created: %#v", content)
	}
}
