package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrPRNotFound         = errors.New("pr not found")
	ErrPRBlocked          = errors.New("submitter is blocked by content author")
	ErrPRConflict         = errors.New("base version is no longer latest, conflict detected")
	ErrPRForbidden        = errors.New("only content author can manage this pr")
	ErrPRInvalidState     = errors.New("pr is already resolved")
	ErrPRMergeTextMissing = errors.New("merged text required")
)

type PRService struct {
	prRepo      *repository.PRRepository
	versionRepo *repository.VersionRepository
	contentRepo *repository.ContentRepository
	notifSvc    *NotificationService
	reputSvc    *ReputationService
	outbox      repository.OutboxWriter
	rdb         *redis.Client
}

func NewPRService(prRepo *repository.PRRepository, vRepo *repository.VersionRepository, cRepo *repository.ContentRepository) *PRService {
	return &PRService{prRepo: prRepo, versionRepo: vRepo, contentRepo: cRepo}
}

func (s *PRService) SetNotificationService(ns *NotificationService) {
	s.notifSvc = ns
}

// SetMergeSupport wires the merge-time collaborators: cache invalidation
// (redis), the transactional outbox for the content.updated index event and
// the reputation service that awards +3 on merge (FIX-21). Every argument is
// optional at construction time and nil-tolerated by the merge path.
func (s *PRService) SetMergeSupport(rdb *redis.Client, outbox repository.OutboxWriter, reputSvc *ReputationService) {
	s.rdb = rdb
	s.outbox = outbox
	s.reputSvc = reputSvc
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

	// new_text lands as a proposed version so the diff right column and the
	// later merge both read the submitted body instead of dropping it
	// (FIX-21①). Proposed snapshots stay out of the published lineage: not
	// latest, excluded from GetVersionChain, hidden from readers.
	if input.NewText != "" {
		versions, err := s.versionRepo.ListByContent(input.ContentItemID)
		if err != nil {
			return nil, err
		}
		nextNum := len(versions) + 1
		parentID := input.BaseVersionID
		proposed := &model.ContentVersion{
			ContentItemID:   input.ContentItemID,
			ParentVersionID: &parentID,
			AuthorID:        submitterID,
			VersionNumber:   nextNum,
			StorageType:     "full",
			StorageKey:      input.NewText,
			DiffSummary:     fmt.Sprintf("v%d: proposed via PR", nextNum),
			Status:          "proposed",
			IsLatest:        false,
		}
		if err := s.versionRepo.CreateVersion(proposed); err != nil {
			return nil, err
		}
		pr.ProposedVersionID = &proposed.ID
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

// GetPRForViewer enforces participant-only reads on the PR detail endpoint:
// content author, PR submitter or admin (FIX-21④).
func (s *PRService) GetPRForViewer(id int64, viewerID int64, isAdmin bool) (*model.PullRequest, error) {
	pr, err := s.prRepo.FindByID(id)
	if err != nil || pr == nil {
		return nil, ErrPRNotFound
	}
	content, err := s.contentRepo.FindByID(pr.ContentItemID)
	if err != nil || content == nil {
		return nil, ErrContentNotFound
	}
	if !isAdmin && viewerID != content.AuthorID && viewerID != pr.SubmitterID {
		return nil, ErrPRForbidden
	}
	return pr, nil
}

func (s *PRService) ListPRs(contentID int64, status string) ([]model.PullRequest, error) {
	return s.prRepo.ListByContent(contentID, status)
}

func (s *PRService) ListPRsPaged(contentID int64, status string, page, pageSize int) ([]model.PullRequest, int64, error) {
	return s.prRepo.ListByContentPaged(contentID, status, page, pageSize)
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
	if pr.Status != "open" {
		return ErrPRInvalidState
	}

	now := time.Now()
	if err := s.prRepo.UpdateStatus(prID, "accepted", map[string]interface{}{"resolved_at": now}); err != nil {
		return err
	}

	if err := s.prRepo.UpsertContributor(pr.ContentItemID, pr.SubmitterID); err != nil {
		slog.Error("failed to upsert contributor on accept", "content_id", pr.ContentItemID, "submitter_id", pr.SubmitterID, "error", err)
	}

	if s.notifSvc != nil {
		// accept 只标记采纳：正文要等 merge 才应用（FIX-21③）。
		title := "PR 已采纳，待合并生效"
		if content != nil {
			title = "PR 已采纳，待合并生效：" + content.Title
		}
		s.notifSvc.Notify(pr.SubmitterID, "pr", "pr_accepted", title, "", "pr", prID, callerID)
	}
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
	if pr.Status != "open" {
		return ErrPRInvalidState
	}

	now := time.Now()
	if err := s.prRepo.UpdateStatus(prID, "rejected", map[string]interface{}{
		"reject_reason": reason,
		"resolved_at":   now,
	}); err != nil {
		return err
	}

	if s.notifSvc != nil {
		title := "PR 已被拒绝"
		if content != nil {
			title = "PR 已被拒绝：" + content.Title
		}
		s.notifSvc.Notify(pr.SubmitterID, "pr", "pr_rejected", title, reason, "pr", prID, callerID)
	}
	return nil
}

func (s *PRService) ManualMerge(prID int64, callerID int64, mergedText string) (*model.ContentVersion, error) {
	pr, err := s.prRepo.FindByID(prID)
	if err != nil || pr == nil {
		return nil, ErrPRNotFound
	}

	content, err := s.contentRepo.FindByID(pr.ContentItemID)
	if err != nil || content == nil {
		return nil, ErrContentNotFound
	}
	if content.AuthorID != callerID {
		return nil, ErrPRForbidden
	}
	// merge applies both open and accepted PRs: accept marks intent, merge
	// remains the single write point for the body (FIX-21②).
	if pr.Status != "open" && pr.Status != "accepted" {
		return nil, ErrPRInvalidState
	}

	// Without an explicit merged_text the merge applies the submitted
	// proposal — that is the whole point of persisting new_text.
	if mergedText == "" && pr.ProposedVersionID != nil {
		proposed, err := s.versionRepo.FindByID(*pr.ProposedVersionID)
		if err != nil {
			return nil, err
		}
		if proposed != nil {
			mergedText = proposed.StorageKey
		}
	}
	if mergedText == "" {
		return nil, ErrPRMergeTextMissing
	}

	versions, err := s.versionRepo.ListByContent(pr.ContentItemID)
	if err != nil {
		return nil, err
	}
	nextNum := len(versions) + 1
	parentID := pr.BaseVersionID

	v := &model.ContentVersion{
		ContentItemID:   pr.ContentItemID,
		ParentVersionID: &parentID,
		AuthorID:        callerID,
		VersionNumber:   nextNum,
		StorageType:     "full",
		StorageKey:      mergedText,
		DiffSummary:     fmt.Sprintf("v%d: manual merge from PR #%d", nextNum, prID),
		Status:          "active",
		IsLatest:        true,
	}

	// Version creation, latest flip, body sync and the content.updated index
	// event share one transaction so a failure never leaves the visible body
	// ahead of its version chain (or the RAG index stale, FIX-21②).
	now := time.Now()
	err = s.prRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(v).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ContentVersion{}).
			Where("content_item_id = ?", pr.ContentItemID).
			Update("is_latest", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ContentVersion{}).
			Where("id = ?", v.ID).
			Update("is_latest", true).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ContentItem{}).
			Where("id = ?", pr.ContentItemID).
			Update("description", mergedText).Error; err != nil {
			return err
		}
		if err := EmitContentStatusEventTx(context.Background(), tx, s.outbox, events.TopicContentUpdated, content, ""); err != nil {
			return err
		}
		if err := tx.Model(&model.PullRequest{}).
			Where("id = ?", prID).
			Updates(map[string]interface{}{"status": "merged", "resolved_at": now}).Error; err != nil {
			return err
		}
		return tx.Exec(`
			INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at)
			VALUES (?, ?, 1, NOW())
			ON CONFLICT (content_item_id, user_id)
			DO UPDATE SET pr_count = content_contributors.pr_count + 1
		`, pr.ContentItemID, pr.SubmitterID).Error
	})
	if err != nil {
		return nil, err
	}

	InvalidateContentCaches(s.rdb, pr.ContentItemID)

	// +3 for the contributor whose PR got merged (SP-03/FIX-03 wiring point).
	if s.reputSvc != nil {
		if err := s.reputSvc.AwardPRMerged(pr.SubmitterID, prID); err != nil {
			slog.Error("failed to award pr_merged reputation", "pr_id", prID, "submitter_id", pr.SubmitterID, "error", err)
		}
	}

	if s.notifSvc != nil {
		s.notifSvc.Notify(pr.SubmitterID, "pr", "pr_merged", "PR 已合并："+content.Title, "", "pr", prID, callerID)
	}
	return v, nil
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
