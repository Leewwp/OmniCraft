package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
)

var (
	ErrReviewTargetNotFound = errors.New("review target not found")
)

type SubmitReviewInput struct {
	TargetType  string
	TargetID    int64
	ContentType string
	Title       string
	Description string
	AuthorID    int64
	Attachments []AttachmentInput
}

type AICallbackInput struct {
	TargetType  string                 `json:"target_type" binding:"required"`
	TargetID    int64                  `json:"target_id" binding:"required"`
	Result      string                 `json:"result" binding:"required"`
	RawResponse map[string]interface{} `json:"raw_response"`
}

type ReviewService struct {
	db       *gorm.DB
	rdb      *redis.Client
	cfg      *config.Config
	reputSvc *ReputationService
	green    *aliyun.GreenClient
}

func NewReviewService(db *gorm.DB, rdb *redis.Client, cfg *config.Config, reputSvc *ReputationService) *ReviewService {
	var greenClient *aliyun.GreenClient
	if cfg != nil {
		greenClient = aliyun.NewGreenClient(cfg.Green.AccessKeyID, cfg.Green.AccessKeySecret, cfg.Green.Region)
	}
	return &ReviewService{
		db:       db,
		rdb:      rdb,
		cfg:      cfg,
		reputSvc: reputSvc,
		green:    greenClient,
	}
}

func (s *ReviewService) SubmitForAIReview(ctx context.Context, in SubmitReviewInput) error {
	if s == nil || s.db == nil {
		return errors.New("review service not initialized")
	}
	if s.green == nil {
		return aliyun.ErrGreenNotConfigured
	}

	result := "pass"
	raw := map[string]interface{}{"source": "green"}

	textResult, err := s.green.TextModeration(ctx, strings.TrimSpace(in.Title+"\n"+in.Description))
	if err != nil {
		return err
	}
	result = mergeReviewResult(result, textResult.Result)
	for k, v := range textResult.RawResponse {
		raw[k] = v
	}

	for _, a := range in.Attachments {
		mime := strings.ToLower(strings.TrimSpace(a.MimeType))
		ossKey := strings.TrimSpace(a.OSSKey)
		if ossKey == "" {
			continue
		}

		scanURL := s.resolveScanObjectURL(ossKey)
		if strings.HasPrefix(mime, "image/") {
			imgRes, scanErr := s.green.ImageModeration(ctx, scanURL)
			if scanErr != nil {
				return scanErr
			}
			result = mergeReviewResult(result, imgRes.Result)
		}
		if strings.HasPrefix(mime, "video/") || strings.EqualFold(a.FileType, "video") {
			videoRes, scanErr := s.green.VideoAsyncScan(ctx, scanURL, s.cfg.Green.CallbackURL)
			if scanErr != nil {
				return scanErr
			}
			result = mergeReviewResult(result, videoRes.Result)
			if videoRes.TaskID != "" {
				raw["video_task_id"] = videoRes.TaskID
			}
		}
	}

	if err := s.recordAIReview(in.TargetType, in.TargetID, result, raw); err != nil {
		return err
	}

	return s.ProcessAICallback(ctx, AICallbackInput{
		TargetType:  in.TargetType,
		TargetID:    in.TargetID,
		Result:      result,
		RawResponse: raw,
	})
}

func (s *ReviewService) ProcessAICallback(ctx context.Context, in AICallbackInput) error {
	if s == nil || s.db == nil {
		return errors.New("review service not initialized")
	}

	result := normalizeReviewResult(in.Result)
	if err := s.recordAIReview(in.TargetType, in.TargetID, result, in.RawResponse); err != nil {
		return err
	}

	if strings.EqualFold(in.TargetType, "ip") {
		return s.processIPReviewResult(in.TargetID, result)
	}
	if !strings.EqualFold(in.TargetType, "content") {
		return nil
	}

	var content model.ContentItem
	if err := s.db.First(&content, in.TargetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReviewTargetNotFound
		}
		return err
	}

	switch result {
	case "block", "violation":
		if content.Status != "banned" {
			if err := s.db.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("status", "banned").Error; err != nil {
				return err
			}
			relatedID := content.ID
			if s.reputSvc != nil {
				if err := s.reputSvc.AddReputation(content.AuthorID, -3, "ai_violation", &relatedID); err != nil {
					return err
				}
			}
			if err := s.applyRepeatViolationPenalty(ctx, content.AuthorID); err != nil {
				return err
			}
		}
	case "review":
		if err := s.db.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("status", "under_review").Error; err != nil {
			return err
		}
		if err := s.ensureJudgeCase(content); err != nil {
			return err
		}
	default:
		if err := s.db.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("status", "published").Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *ReviewService) processIPReviewResult(ipID int64, result string) error {
	var ip model.IP
	if err := s.db.First(&ip, ipID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReviewTargetNotFound
		}
		return err
	}
	if result != "block" && result != "violation" {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.IP{}).Where("id = ?", ipID).Update("status", "banned").Error; err != nil {
			return err
		}
		return tx.Model(&model.ContentItem{}).Where("ip_id = ?", ipID).Update("status", "banned").Error
	})
}

func (s *ReviewService) applyRepeatViolationPenalty(ctx context.Context, authorID int64) error {
	windowDays := s.cfg.Reputation.RepeatViolationWindowDays
	if windowDays <= 0 {
		windowDays = 7
	}
	threshold := s.cfg.Reputation.RepeatViolationThreshold
	if threshold <= 0 {
		threshold = 2
	}
	extraPenalty := s.cfg.Reputation.RepeatViolationExtraPenalty
	if extraPenalty == 0 {
		extraPenalty = -1
	}

	windowStart := time.Now().AddDate(0, 0, -windowDays)
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.AIReviewRecord{}).
		Joins("JOIN content_items ON ai_review_records.target_id = content_items.id").
		Distinct("ai_review_records.target_id").
		Where("ai_review_records.target_type = ?", "content").
		Where("content_items.author_id = ?", authorID).
		Where("ai_review_records.result IN ?", []string{"block", "violation"}).
		Where("ai_review_records.scanned_at >= ?", windowStart).
		Count(&count).Error
	if err != nil {
		return err
	}

	// Apply extra penalty when the threshold is first reached.
	if count < int64(threshold) {
		return nil
	}

	if s.reputSvc != nil {
		if err := s.reputSvc.AddReputation(authorID, extraPenalty, "repeat_violation", nil); err != nil {
			return err
		}
	}

	if s.rdb != nil {
		freezeKey := buildPublishFreezeKey(authorID)
		freezeTTL := 7 * 24 * time.Hour
		if s.cfg != nil && s.cfg.Cache.PublishFreezeTTL > 0 {
			freezeTTL = time.Duration(s.cfg.Cache.PublishFreezeTTL) * time.Second
		}
		if err := s.rdb.Set(ctx, freezeKey, "1", freezeTTL).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (s *ReviewService) ensureJudgeCase(content model.ContentItem) error {
	var count int64
	if err := s.db.Model(&model.JudgeCase{}).
		Where("target_id = ? AND status = ?", content.ID, "open").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	minVotes := s.cfg.Judge.MinVotesRequired
	if minVotes <= 0 {
		minVotes = 20
	}

	judgeCase := model.JudgeCase{
		TargetType: content.ContentType,
		TargetID:   content.ID,
		Status:     "open",
		MinVotes:   minVotes,
	}
	return s.db.Create(&judgeCase).Error
}

func (s *ReviewService) recordAIReview(targetType string, targetID int64, result string, raw map[string]interface{}) error {
	rawBytes, _ := json.Marshal(raw)
	record := model.AIReviewRecord{
		TargetType:  strings.TrimSpace(targetType),
		TargetID:    targetID,
		Provider:    "aliyun",
		Result:      result,
		RawResponse: rawBytes,
	}
	return s.db.Create(&record).Error
}

func (s *ReviewService) resolveScanObjectURL(ossKey string) string {
	if s.cfg != nil && strings.TrimSpace(s.cfg.OSS.Domain) != "" {
		return strings.TrimRight(strings.TrimSpace(s.cfg.OSS.Domain), "/") + "/" + strings.TrimLeft(ossKey, "/")
	}
	return ossKey
}

func mergeReviewResult(current, incoming string) string {
	priority := map[string]int{"pass": 1, "review": 2, "block": 3, "violation": 3}
	curr := normalizeReviewResult(current)
	next := normalizeReviewResult(incoming)
	if priority[next] > priority[curr] {
		return next
	}
	return curr
}

func normalizeReviewResult(result string) string {
	r := strings.ToLower(strings.TrimSpace(result))
	switch r {
	case "block", "violation":
		return "block"
	case "review", "pending":
		return "review"
	default:
		return "pass"
	}
}

func buildPublishFreezeKey(userID int64) string {
	return "publish_freeze:" + strconv.FormatInt(userID, 10)
}
