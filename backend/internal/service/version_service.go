package service

import (
	"errors"
	"fmt"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/diffengine"
	"omnicraft/backend/internal/repository"
)

var ErrVersionNotFound = errors.New("version not found")

type VersionService struct {
	versionRepo *repository.VersionRepository
	contentRepo *repository.ContentRepository
}

func NewVersionService(vRepo *repository.VersionRepository, cRepo *repository.ContentRepository) *VersionService {
	return &VersionService{versionRepo: vRepo, contentRepo: cRepo}
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

func (s *VersionService) ListVersions(contentID int64) ([]model.ContentVersion, error) {
	return s.versionRepo.ListByContent(contentID)
}
