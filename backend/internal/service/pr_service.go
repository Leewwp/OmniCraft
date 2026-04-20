package service

import (
	"errors"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrPRNotFound  = errors.New("pr not found")
	ErrPRBlocked   = errors.New("submitter is blocked by content author")
	ErrPRConflict  = errors.New("base version is no longer latest, conflict detected")
	ErrPRForbidden = errors.New("only content author can manage this pr")
)

type PRService struct {
	prRepo      *repository.PRRepository
	versionRepo *repository.VersionRepository
	contentRepo *repository.ContentRepository
}

func NewPRService(prRepo *repository.PRRepository, vRepo *repository.VersionRepository, cRepo *repository.ContentRepository) *PRService {
	return &PRService{prRepo: prRepo, versionRepo: vRepo, contentRepo: cRepo}
}

type SubmitPRInput struct {
	ContentItemID int64  `json:"content_item_id" binding:"required"`
	BaseVersionID int64  `json:"base_version_id" binding:"required"`
	Message       string `json:"message"`
	NewText       string `json:"new_text"`
}

func (s *PRService) SubmitPR(input SubmitPRInput, submitterID int64) (*model.PullRequest, error) {
	content, err := s.contentRepo.FindByID(input.ContentItemID)
	if err != nil || content == nil {
		return nil, ErrContentNotFound
	}

	blocked, err := s.prRepo.IsBlocked(content.AuthorID, submitterID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrPRBlocked
	}

	latest, err := s.versionRepo.GetLatest(input.ContentItemID)
	if err != nil {
		return nil, err
	}
	if latest != nil && latest.ID != input.BaseVersionID {
		return nil, ErrPRConflict
	}

	pr := &model.PullRequest{
		ContentItemID: input.ContentItemID,
		SubmitterID:   submitterID,
		BaseVersionID: input.BaseVersionID,
		Status:        "open",
		Message:       input.Message,
	}

	if err := s.prRepo.CreatePR(pr); err != nil {
		return nil, err
	}

	return pr, nil
}

func (s *PRService) GetPR(id int64) (*model.PullRequest, error) {
	pr, err := s.prRepo.FindByID(id)
	if err != nil || pr == nil {
		return nil, ErrPRNotFound
	}
	return pr, nil
}

func (s *PRService) ListPRs(contentID int64, status string) ([]model.PullRequest, error) {
	return s.prRepo.ListByContent(contentID, status)
}

func (s *PRService) AcceptPR(prID int64, callerID int64) error {
	pr, err := s.prRepo.FindByID(prID)
	if err != nil || pr == nil {
		return ErrPRNotFound
	}

	content, err := s.contentRepo.FindByID(pr.ContentItemID)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != callerID {
		return ErrPRForbidden
	}

	now := time.Now()
	if err := s.prRepo.UpdateStatus(prID, "accepted", map[string]interface{}{"resolved_at": now}); err != nil {
		return err
	}

	_ = s.prRepo.UpsertContributor(pr.ContentItemID, pr.SubmitterID)
	return nil
}

func (s *PRService) RejectPR(prID int64, callerID int64, reason string) error {
	pr, err := s.prRepo.FindByID(prID)
	if err != nil || pr == nil {
		return ErrPRNotFound
	}

	content, err := s.contentRepo.FindByID(pr.ContentItemID)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != callerID {
		return ErrPRForbidden
	}

	now := time.Now()
	return s.prRepo.UpdateStatus(prID, "rejected", map[string]interface{}{
		"reject_reason": reason,
		"resolved_at":   now,
	})
}

func (s *PRService) BlockContributor(authorID int64, blockedID int64) error {
	return s.prRepo.BlockUser(authorID, blockedID)
}

func (s *PRService) UnblockContributor(authorID int64, blockedID int64) error {
	return s.prRepo.UnblockUser(authorID, blockedID)
}

func (s *PRService) ListContributors(contentID int64, callerID int64) (interface{}, error) {
	content, err := s.contentRepo.FindByID(contentID)
	if err != nil || content == nil {
		return nil, ErrContentNotFound
	}
	if content.AuthorID != callerID {
		return nil, ErrPRForbidden
	}
	return s.prRepo.ListContributors(contentID)
}
