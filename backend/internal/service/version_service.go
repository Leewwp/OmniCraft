package service

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/diffengine"
	"omnicraft/backend/internal/repository"
)

var (
	ErrVersionNotFound  = errors.New("version not found")
	ErrVersionForbidden = errors.New("only content participants can read this version")
)

type VersionService struct {
	versionRepo *repository.VersionRepository
	contentRepo *repository.ContentRepository
}

func NewVersionService(vRepo *repository.VersionRepository, cRepo *repository.ContentRepository) *VersionService {
	return &VersionService{versionRepo: vRepo, contentRepo: cRepo}
}

// WithDB binds version reads to an existing GORM connection. Projection work
// holds a PostgreSQL advisory-lock session for its whole operation, so version
// reconstruction must use that same physical connection when one is supplied.
func (s *VersionService) WithDB(db *gorm.DB) *VersionService {
	return NewVersionService(repository.NewVersionRepository(db), repository.NewContentRepository(db))
}

func (s *VersionService) CreateInitialVersion(contentID int64, authorID int64, fullText string) (*model.ContentVersion, error) {
	v := &model.ContentVersion{
		ContentItemID: contentID,
		AuthorID:      authorID,
		VersionNumber: 1,
		StorageType:   "full",
		StorageKey:    fullText,
		DiffSummary:   "initial version",
		Status:        "active",
		IsLatest:      true,
	}

	if err := s.versionRepo.CreateVersion(v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VersionService) CreateNewVersion(contentID int64, authorID int64, baseVersionID int64, newText string) (*model.ContentVersion, error) {
	base, err := s.versionRepo.FindByID(baseVersionID)
	if err != nil || base == nil {
		return nil, ErrVersionNotFound
	}

	versions, err := s.versionRepo.ListByContent(contentID)
	if err != nil {
		return nil, err
	}
	nextNum := len(versions) + 1

	patch := diffengine.ComputePatch("", newText)
	summary := fmt.Sprintf("v%d: %s", nextNum, diffengine.DiffSummary("", newText))

	v := &model.ContentVersion{
		ContentItemID:   contentID,
		ParentVersionID: &baseVersionID,
		AuthorID:        authorID,
		VersionNumber:   nextNum,
		StorageType:     "diff",
		StorageKey:      patch,
		DiffSummary:     summary,
		Status:          "active",
		IsLatest:        true,
	}

	if err := s.versionRepo.CreateVersion(v); err != nil {
		return nil, err
	}

	if err := s.versionRepo.SetLatest(contentID, v.ID); err != nil {
		return nil, err
	}

	return v, nil
}

func (s *VersionService) GetVersionContent(versionID int64) (string, error) {
	v, err := s.versionRepo.FindByID(versionID)
	if err != nil || v == nil {
		return "", ErrVersionNotFound
	}

	// proposed snapshots are self-contained full bodies living outside the
	// published lineage (GetVersionChain skips them), so they resolve
	// directly instead of walking the chain (FIX-21).
	if v.Status == "proposed" {
		return v.StorageKey, nil
	}

	if v.StorageType == "full" {
		return v.StorageKey, nil
	}

	chain, err := s.versionRepo.GetVersionChain(v.ContentItemID)
	if err != nil {
		return "", err
	}

	content := ""
	for _, cv := range chain {
		if cv.StorageType == "full" {
			content = cv.StorageKey
		} else if cv.StorageType == "diff" && cv.StorageKey != "" {
			result, err := diffengine.ApplyPatch(content, cv.StorageKey)
			if err != nil {
				return "", err
			}
			content = result
		}
		if cv.ID == v.ID {
			break
		}
	}

	return content, nil
}

// LoadLatestPublishedContent returns the latest active version number and its
// reconstructed body. Projection consumers must use this seam instead of
// treating a diff patch as a full document.
func (s *VersionService) LoadLatestPublishedContent(ctx context.Context, contentID int64) (int, string, error) {
	if err := ctx.Err(); err != nil {
		return 0, "", err
	}
	version, err := s.versionRepo.GetLatest(contentID)
	if err != nil {
		return 0, "", err
	}
	if version == nil || version.Status != "active" {
		return 0, "", ErrVersionNotFound
	}
	content, err := s.GetVersionContent(version.ID)
	if err != nil {
		return 0, "", err
	}
	if err := ctx.Err(); err != nil {
		return 0, "", err
	}
	return version.VersionNumber, content, nil
}

func (s *VersionService) ListVersions(contentID int64) ([]model.ContentVersion, error) {
	return s.versionRepo.ListByContent(contentID)
}

func (s *VersionService) ListVersionsPaged(contentID int64, page, pageSize int) ([]model.ContentVersion, int64, error) {
	return s.versionRepo.ListByContentPaged(contentID, page, pageSize)
}

// ListVersionsPagedForViewer hides proposed versions from everyone but the
// content author: readers only ever see the active lineage (FIX-21①).
func (s *VersionService) ListVersionsPagedForViewer(contentID int64, page, pageSize int, viewerID int64) ([]model.ContentVersion, int64, error) {
	content, err := s.contentRepo.FindByID(contentID)
	if err != nil || content == nil {
		return nil, 0, ErrContentNotFound
	}
	return s.versionRepo.ListByContentPagedForViewer(contentID, page, pageSize, viewerID == content.AuthorID)
}

// GetVersionForViewer enforces participant-only reads on the version detail
// endpoint: content author, the proposed version's submitter or admin.
// Without this gate any optAuth reader could pull the full text of versions
// belonging to banned content (F-056, FIX-21④).
func (s *VersionService) GetVersionForViewer(versionID int64, viewerID int64, isAdmin bool) (string, error) {
	v, err := s.versionRepo.FindByID(versionID)
	if err != nil || v == nil {
		return "", ErrVersionNotFound
	}
	content, err := s.contentRepo.FindByID(v.ContentItemID)
	if err != nil || content == nil {
		return "", ErrVersionNotFound
	}
	if !isAdmin && viewerID != content.AuthorID && !(v.Status == "proposed" && viewerID == v.AuthorID) {
		return "", ErrVersionForbidden
	}
	return s.GetVersionContent(versionID)
}
