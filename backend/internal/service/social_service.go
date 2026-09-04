package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrCommentNotFound       = errors.New("comment not found")
	ErrDiscussionNotFound    = errors.New("discussion not found")
	ErrLowReputation         = errors.New("reputation too low to post")
	ErrCommentForbidden      = errors.New("not comment author")
	ErrAlreadyReported       = errors.New("already reported this target")
	ErrTextBlocked           = errors.New("text rejected by content moderation")
	ErrModerationUnavailable = errors.New("content moderation unavailable")
)

const defaultMinScoreForInteraction = 3

// TextReviewer is the minimal moderation dependency SocialService needs to
// gate user-generated text before it is persisted.
type TextReviewer interface {
	ReviewText(ctx context.Context, text string) (string, error)
}

type SocialService struct {
	socialRepo       *repository.SocialRepository
	contentRepo      *repository.ContentRepository
	userRepo         *repository.UserRepository
	cfg              *config.Config
	rdb              *redis.Client
	notifSvc         *NotificationService
	reviewSvc        TextReviewer
	judgeCaseEnsurer JudgeCaseEnsurer
}

func NewSocialService(sRepo *repository.SocialRepository, cRepo *repository.ContentRepository, uRepo *repository.UserRepository, cfg *config.Config, reviewSvc TextReviewer) *SocialService {
	return &SocialService{
		socialRepo:  sRepo,
		contentRepo: cRepo,
		userRepo:    uRepo,
		cfg:         cfg,
		reviewSvc:   reviewSvc,
	}
}

func NewSocialServiceWithRedis(sRepo *repository.SocialRepository, cRepo *repository.ContentRepository, uRepo *repository.UserRepository, cfg *config.Config, rdb *redis.Client, reviewSvc TextReviewer) *SocialService {
	return &SocialService{
		socialRepo:  sRepo,
		contentRepo: cRepo,
		userRepo:    uRepo,
		cfg:         cfg,
		rdb:         rdb,
		reviewSvc:   reviewSvc,
	}
}

func (s *SocialService) SetNotificationService(ns *NotificationService) {
	s.notifSvc = ns
}

// JudgeCaseEnsurer is the shared seam for creating a crowd-judge case for a
// content that entered manual review (FIX-11): the AI review path and the
// report auto-hide path funnel through the same idempotent creation.
type JudgeCaseEnsurer interface {
	EnsureContentJudgeCase(tx *gorm.DB, content model.ContentItem) error
}

// SetJudgeCaseEnsurer wires the judge-case creation seam used by the report
// auto-hide path (FIX-11). Unwired (nil) the auto-hide still hides the
// content but skips case creation.
func (s *SocialService) SetJudgeCaseEnsurer(ensurer JudgeCaseEnsurer) {
	s.judgeCaseEnsurer = ensurer
}

type PostCommentInput struct {
	ContentItemID *int64 `json:"content_item_id"`
	DiscussionID  *int64 `json:"discussion_id"`
	ParentID      *int64 `json:"parent_id"`
	Body          string `json:"body" binding:"required,min=1,max=5000"`
}

func (s *SocialService) PostComment(ctx context.Context, input PostCommentInput, authorID int64) (*model.Comment, error) {
	if err := s.ensureCanInteract(authorID); err != nil {
		return nil, err
	}
	if err := s.moderateText(ctx, "comment", input.Body); err != nil {
		return nil, err
	}

	comment := &model.Comment{
		ContentItemID: input.ContentItemID,
		DiscussionID:  input.DiscussionID,
		ParentID:      input.ParentID,
		AuthorID:      authorID,
		Body:          input.Body,
		Status:        "published",
	}

	if err := s.socialRepo.CreateComment(comment); err != nil {
		return nil, err
	}

	// T12（FIX-18）前提②定夺：对讨论的评论统一在此维护 reply_count 与
	// last_active_at（latest_reply 排序驱动）。讨论路由与 /social/comments
	// 两个入口经同一分支，一条评论只递增一次——防双计；handler 不再手动
	// IncrementReplyCount。递增失败不回滚已创建的评论，仅记录日志。
	if input.DiscussionID != nil {
		if err := s.socialRepo.IncrementDiscussionReplyCount(*input.DiscussionID); err != nil {
			slog.Warn("failed to increment discussion reply_count", "discussion_id", *input.DiscussionID, "error", err)
		}
	}

	if s.notifSvc != nil {
		if input.ContentItemID != nil {
			content, err := s.contentRepo.FindByID(*input.ContentItemID)
			if err == nil && content != nil && content.AuthorID != authorID {
				s.notifSvc.Notify(content.AuthorID, "reply", "comment", "新评论", input.Body, "content", content.ID, authorID)
			}
		} else if input.DiscussionID != nil {
			disc, err := s.socialRepo.FindDiscussion(*input.DiscussionID)
			if err == nil && disc != nil && disc.AuthorID != authorID {
				s.notifSvc.Notify(disc.AuthorID, "reply", "comment", "新回复", input.Body, "discussion", disc.ID, authorID)
			}
		}
	}

	return comment, nil
}

func (s *SocialService) DeleteComment(commentID int64, callerID int64) error {
	c, err := s.socialRepo.FindComment(commentID)
	if err != nil || c == nil {
		return ErrCommentNotFound
	}
	if c.AuthorID != callerID {
		return ErrCommentForbidden
	}
	return s.socialRepo.DeleteComment(commentID)
}

func (s *SocialService) EditComment(ctx context.Context, commentID int64, callerID int64, newBody string) (*model.Comment, error) {
	c, err := s.socialRepo.FindComment(commentID)
	if err != nil || c == nil {
		return nil, ErrCommentNotFound
	}
	if c.AuthorID != callerID {
		return nil, ErrCommentForbidden
	}
	if err := s.moderateText(ctx, "edit_comment", newBody); err != nil {
		return nil, err
	}
	if err := s.socialRepo.EditComment(commentID, newBody); err != nil {
		return nil, err
	}
	c.Body = newBody
	return c, nil
}

func (s *SocialService) ListComments(contentID int64, parentID *int64, page, pageSize int) ([]model.Comment, int64, error) {
	return s.socialRepo.ListComments(contentID, parentID, page, pageSize)
}

type PostDiscussionInput struct {
	IPID          *int64 `json:"ip_id"`
	ContentItemID *int64 `json:"content_item_id"`
	Title         string `json:"title" binding:"required,min=1,max=500"`
	Body          string `json:"body"`
}

func (s *SocialService) PostDiscussion(ctx context.Context, input PostDiscussionInput, authorID int64) (*model.Discussion, error) {
	if err := s.ensureCanInteract(authorID); err != nil {
		return nil, err
	}
	if err := s.moderateText(ctx, "discussion", strings.TrimSpace(input.Title)+"\n"+input.Body); err != nil {
		return nil, err
	}

	d := &model.Discussion{
		IPID:          input.IPID,
		ContentItemID: input.ContentItemID,
		AuthorID:      authorID,
		Title:         input.Title,
		Body:          input.Body,
		Status:        "published",
	}
	if err := s.socialRepo.CreateDiscussion(d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *SocialService) GetDiscussion(id int64) (*model.Discussion, error) {
	d, err := s.socialRepo.FindDiscussion(id)
	if err != nil || d == nil {
		return nil, ErrDiscussionNotFound
	}
	return d, nil
}

func (s *SocialService) ListDiscussions(ipID *int64, contentID *int64, page, pageSize int) ([]model.Discussion, int64, error) {
	return s.socialRepo.ListDiscussions(ipID, contentID, page, pageSize)
}

type ReactInput struct {
	TargetType string `json:"target_type" binding:"required,oneof=content comment"`
	TargetID   int64  `json:"target_id" binding:"required"`
	Reaction   string `json:"reaction" binding:"required,oneof=like dislike"`
}

func (s *SocialService) React(input ReactInput, userID int64) (string, error) {
	if err := s.ensureCanInteract(userID); err != nil {
		return "", err
	}

	reaction := &model.Reaction{
		UserID:     userID,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		Reaction:   input.Reaction,
	}

	action, err := s.socialRepo.UpsertReaction(reaction)
	if err != nil {
		return "", err
	}

	// 点踩不通知作者（FIX-31b：减少负面轰炸），仅点赞产生通知。
	if action == "created" && input.TargetType == "content" && input.Reaction == "like" && s.notifSvc != nil {
		content, cErr := s.contentRepo.FindByID(input.TargetID)
		if cErr == nil && content != nil && content.AuthorID != userID {
			s.notifSvc.Notify(content.AuthorID, "like", "like", "新的赞", "", "content", content.ID, userID)
		}
	}

	if input.TargetType == "content" {
		likes, dislikes, _ := s.socialRepo.GetReactionCounts("content", input.TargetID)
		s.contentRepo.UpdateContent(input.TargetID, map[string]interface{}{
			"like_count":    likes,
			"dislike_count": dislikes,
		})

		if s.rdb != nil && input.Reaction == "like" {
			ctx := context.Background()
			s.rdb.ZIncrBy(ctx, "rank:hot:contents", 3, fmt.Sprintf("%d", input.TargetID))
		}
	}

	redisclient.ClearRecCache(context.Background(), userID)
	return action, nil
}

func (s *SocialService) ensureCanInteract(userID int64) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}
	if user.IsBanned {
		return errors.New("user is banned")
	}
	if user.Reputation < minScoreForInteraction(s.cfg) {
		return ErrLowReputation
	}
	return nil
}

func minScoreForInteraction(cfg *config.Config) int {
	if cfg == nil || cfg.Reputation.MinScoreForInteraction <= 0 {
		return defaultMinScoreForInteraction
	}
	return cfg.Reputation.MinScoreForInteraction
}

// moderateText runs the text moderation gate before comment/discussion text
// is persisted. Blank text is skipped without an external call. A "block"
// (or "violation") result rejects the submission. Availability policy follows
// the A4 environment semantics via RunModerationGate: in release mode any
// moderation failure is fail-closed, while in local/test mode an unconfigured
// Green client is fail-open and must be recorded via structured logs.
func (s *SocialService) moderateText(ctx context.Context, action, text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	var review func(context.Context) (string, error)
	if s.reviewSvc != nil {
		review = func(ctx context.Context) (string, error) {
			return s.reviewSvc.ReviewText(ctx, trimmed)
		}
	}
	return RunModerationGate(ctx, s.cfg, action, "content moderation", "submission",
		review, true, ErrTextBlocked, ErrModerationUnavailable)
}

func (s *SocialService) Report(targetType string, targetID int64, reporterID int64, reason, detail string) error {
	existing, err := s.socialRepo.FindReportByUserAndTarget(reporterID, targetType, targetID)
	if err == nil && existing != nil {
		return ErrAlreadyReported
	}

	report := &model.Report{
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		Detail:     detail,
		Status:     "pending",
	}
	if err := s.socialRepo.CreateReport(report); err != nil {
		return err
	}

	if targetType == "content" {
		content, err := s.contentRepo.FindByID(targetID)
		if err != nil || content == nil || content.ViewCount == 0 {
			return nil
		}
		count, _ := s.socialRepo.CountReports("content", targetID)
		threshold := s.cfg.Social.ReportAutoHideRate
		ratio := float64(count) / float64(content.ViewCount)
		if ratio >= threshold {
			// 守卫：只有 under_review/published 会被隐藏——banned 终态不可被
			// 举报打回复核（FIX-11）；真实发生隐藏才触发众裁与作者通知。
			hidden := false
			if err := s.contentRepo.Transaction(func(txRepo *repository.ContentRepository) error {
				res := txRepo.DB().Model(&model.ContentItem{}).
					Where("id = ? AND status IN ?", targetID, []string{"under_review", "published"}).
					Update("status", "under_review")
				if res.Error != nil {
					return res.Error
				}
				hidden = res.RowsAffected > 0
				if !hidden {
					return nil
				}
				// 隐藏的内容必须有众裁出口（FIX-11）：与 AI 审核转复核路径
				// 复用同一幂等建案逻辑，举报达阈值不再无限期滞留。
				if s.judgeCaseEnsurer != nil {
					return s.judgeCaseEnsurer.EnsureContentJudgeCase(txRepo.DB(), *content)
				}
				return nil
			}); err != nil {
				return err
			}
			if hidden {
				InvalidateContentCaches(s.rdb, targetID)
				if s.notifSvc != nil {
					s.notifSvc.NotifyContentStatus(content.AuthorID, content.ID, content.Title, "under_review", "举报达到阈值，内容进入众裁复核", 0)
				}
			}
		}
	}
	return nil
}
