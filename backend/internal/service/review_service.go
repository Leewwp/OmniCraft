package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/rediskeys"
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
	TargetType     string                 `json:"target_type" binding:"required"`
	TargetID       int64                  `json:"target_id" binding:"required"`
	Result         string                 `json:"result" binding:"required"`
	RawResponse    map[string]interface{} `json:"raw_response"`
	ProviderTaskID string                 `json:"provider_task_id"`
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

	if err := s.ProcessAICallback(ctx, AICallbackInput{
		TargetType:  in.TargetType,
		TargetID:    in.TargetID,
		Result:      result,
		RawResponse: raw,
	}); err != nil {
		return err
	}
	return nil
}

func (s *ReviewService) ProcessAICallback(ctx context.Context, in AICallbackInput) error {
	if s == nil || s.db == nil {
		return errors.New("review service not initialized")
	}

	result := normalizeReviewResult(in.Result)
	providerTaskID := strings.TrimSpace(in.ProviderTaskID)

	// Idempotent short-circuit: a callback for an already-recorded provider
	// task id returns success (HTTP 200 semantics) without re-recording,
	// re-penalizing or re-freezing. Alibaba Cloud retries callback delivery,
	// so duplicate callbacks are the normal case, not an error.
	if providerTaskID != "" {
		var count int64
		if err := s.db.WithContext(ctx).Model(&model.AIReviewRecord{}).
			Where("provider = ? AND provider_task_id = ?", "aliyun", providerTaskID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
	}

	// The dedup re-check and the record insert share one transaction with the
	// status transition, so two concurrent duplicate callbacks cannot both
	// pass the check; the unique index converts the losing insert into a
	// no-op. The record is written before the repeat-violation count so the
	// current violation is visible inside the window.
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		recorded, err := s.recordAIReview(ctx, tx, in.TargetType, in.TargetID, result, in.RawResponse, providerTaskID)
		if err != nil {
			return err
		}
		if !recorded {
			return nil
		}

		if strings.EqualFold(in.TargetType, "ip") {
			return s.processIPReviewResult(tx, in.TargetID, result)
		}
		if !strings.EqualFold(in.TargetType, "content") {
			return nil
		}
		return s.applyContentReviewResult(ctx, tx, in.TargetID, result)
	})
}

// applyContentReviewResult transitions a content item according to the
// normalized AI result. banned is the terminal state of the AI channel:
// pass and review results use a conditional update that can never overwrite
// a banned row, so a late async result cannot resurrect blocked content.
// The block branch stays idempotent: an already-banned row is not
// re-penalized.
func (s *ReviewService) applyContentReviewResult(ctx context.Context, tx *gorm.DB, contentID int64, result string) error {
	var content model.ContentItem
	if err := tx.First(&content, contentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReviewTargetNotFound
		}
		return err
	}

	switch result {
	case "block":
		if content.Status == "banned" {
			return nil
		}
		if err := tx.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("status", "banned").Error; err != nil {
			return err
		}
		relatedID := content.ID
		if s.reputSvc != nil {
			if err := addReputationTx(tx, content.AuthorID, -3, "ai_violation", &relatedID); err != nil {
				return err
			}
		}
		return s.applyRepeatViolationPenalty(ctx, tx, content.AuthorID)
	case "review":
		res := tx.Model(&model.ContentItem{}).
			Where("id = ? AND status <> ?", content.ID, "banned").
			Update("status", "under_review")
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected == 0 {
			return nil
		}
		return s.ensureJudgeCase(tx, content)
	default:
		res := tx.Model(&model.ContentItem{}).
			Where("id = ? AND status <> ?", content.ID, "banned").
			Update("status", "published")
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected == 0 {
			return nil
		}
	}
	return nil
}

func (s *ReviewService) processIPReviewResult(tx *gorm.DB, ipID int64, result string) error {
	var ip model.IP
	if err := tx.First(&ip, ipID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReviewTargetNotFound
		}
		return err
	}
	if result != "block" && result != "violation" {
		return nil
	}
	if err := tx.Model(&model.IP{}).Where("id = ?", ipID).Update("status", "banned").Error; err != nil {
		return err
	}
	return tx.Model(&model.ContentItem{}).Where("ip_id = ?", ipID).Update("status", "banned").Error
}

func (s *ReviewService) applyRepeatViolationPenalty(ctx context.Context, tx *gorm.DB, authorID int64) error {
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
	err := tx.WithContext(ctx).
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
		if err := addReputationTx(tx, authorID, extraPenalty, "repeat_violation", nil); err != nil {
			return err
		}
	}

	if s.rdb != nil {
		freezeKey := rediskeys.PublishFreezeKey(authorID)
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

func (s *ReviewService) ensureJudgeCase(tx *gorm.DB, content model.ContentItem) error {
	var count int64
	if err := tx.Model(&model.JudgeCase{}).
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
	return tx.Create(&judgeCase).Error
}

// recordAIReview persists one ai_review_records row inside tx. It returns
// false when the row was skipped because the same (provider,
// provider_task_id) already exists (a concurrent duplicate callback). Rows
// without a provider task id are stored as NULL so they never conflict in the
// unique index: synchronous submissions always record, and the initial
// submission record and the async result keep separate idempotency keys.
func (s *ReviewService) recordAIReview(ctx context.Context, tx *gorm.DB, targetType string, targetID int64, result string, raw map[string]interface{}, providerTaskID string) (bool, error) {
	rawBytes, _ := json.Marshal(raw)
	record := model.AIReviewRecord{
		TargetType:  strings.TrimSpace(targetType),
		TargetID:    targetID,
		Provider:    "aliyun",
		Result:      result,
		RawResponse: rawBytes,
	}
	if providerTaskID != "" {
		record.ProviderTaskID = &providerTaskID
	}
	stmt := tx.WithContext(ctx).Model(&model.AIReviewRecord{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "provider_task_id"}},
			DoNothing: true,
		}).
		Create(&record)
	if err := stmt.Error; err != nil {
		return false, err
	}
	return stmt.RowsAffected > 0, nil
}

// addReputationTx applies a reputation log entry and the score delta inside
// the caller's transaction, so the review record, the penalty and the status
// transition are atomic: a failed penalty rolls the record back too, and a
// duplicate callback can never charge the penalty twice.
func addReputationTx(tx *gorm.DB, userID int64, delta int, reason string, relatedID *int64) error {
	log := model.ReputationLog{
		UserID:    userID,
		Delta:     delta,
		Reason:    reason,
		RelatedID: relatedID,
	}
	if err := tx.Create(&log).Error; err != nil {
		return err
	}
	return tx.Model(&model.User{}).
		Where("id = ?", userID).
		UpdateColumn("reputation", gorm.Expr("reputation + ?", delta)).Error
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
