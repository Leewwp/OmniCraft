package service

import (
	"omnicraft/backend/internal/model"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
)

type ReputationService struct {
	db    *gorm.DB
	rdb   *redis.Client
	cfg   *config.Config
	cache *RuntimeStatusCache
}

func NewReputationService(db *gorm.DB) *ReputationService {
	return &ReputationService{db: db}
}

func NewReputationServiceWithCache(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *ReputationService {
	return &ReputationService{
		db:    db,
		rdb:   rdb,
		cfg:   cfg,
		cache: NewRuntimeStatusCache(rdb, cfg),
	}
}

func (s *ReputationService) SetCache(cache *RuntimeStatusCache) {
	s.cache = cache
}

func (s *ReputationService) AddReputation(userID int64, delta int, reason string, relatedID *int64) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
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
	})
	if err == nil && s.cache != nil {
		s.cache.Invalidate(userID)
	}
	return err
}

func (s *ReputationService) GetLogs(userID int64, page, pageSize int) ([]model.ReputationLog, int64, error) {
	var logs []model.ReputationLog
	var total int64

	q := s.db.Model(&model.ReputationLog{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// score returns the configured score value, falling back to fallback when the
// service has no config reference (cfg is nil) or the configured value is zero.
func (s *ReputationService) score(fallback int, getCfg func() int) int {
	if s.cfg == nil {
		return fallback
	}
	if v := getCfg(); v != 0 {
		return v
	}
	return fallback
}

func (s *ReputationService) AwardQualityContent(userID int64, contentID int64, likeCount int, threshold int) error {
	if likeCount < threshold {
		return nil
	}
	var count int64
	s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'quality_content' AND related_id = ?", userID, contentID).
		Count(&count)
	if count > 0 {
		return nil
	}
	return s.AddReputation(userID, s.score(3, func() int { return s.cfg.Reputation.ScoreQualityContent }), "quality_content", &contentID)
}

func (s *ReputationService) AwardPRMerged(userID int64, prID int64) error {
	return s.AddReputation(userID, s.score(3, func() int { return s.cfg.Reputation.ScorePRMerged }), "pr_merged", &prID)
}

func (s *ReputationService) AwardQualityComment(userID int64, commentID int64, likeCount int, threshold int) error {
	if likeCount < threshold {
		return nil
	}
	var count int64
	s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'quality_comment' AND related_id = ?", userID, commentID).
		Count(&count)
	if count > 0 {
		return nil
	}
	return s.AddReputation(userID, s.score(2, func() int { return s.cfg.Reputation.ScoreQualityComment }), "quality_comment", &commentID)
}

func (s *ReputationService) AwardTagRecognized(userID int64, tagSuggestionID int64) error {
	return s.AddReputation(userID, s.score(1, func() int { return s.cfg.Reputation.ScoreTagRecognized }), "tag_recognized", &tagSuggestionID)
}

func (s *ReputationService) AwardJudgeAccuracy(userID int64, caseID int64) error {
	// T39（FIX-03）：同案幂等（闭案并发触发/重复调用只记一次）。
	var count int64
	s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'judge_accuracy' AND related_id = ?", userID, caseID).
		Count(&count)
	if count > 0 {
		return nil
	}
	return s.AddReputation(userID, s.score(1, func() int { return s.cfg.Reputation.ScoreJudgeAccuracy }), "judge_accuracy", &caseID)
}

func (s *ReputationService) AwardRehabCourse(userID int64, courseID int64) error {
	var count int64
	s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'rehab_course' AND related_id = ?", userID, courseID).
		Count(&count)
	if count > 0 {
		return nil
	}
	return s.AddReputation(userID, s.score(1, func() int { return s.cfg.Reputation.ScoreRehabCourse }), "rehab_course", &courseID)
}

func (s *ReputationService) PenalizeMaliciousContent(userID int64, contentID int64) error {
	return s.AddReputation(userID, s.score(-3, func() int { return s.cfg.Reputation.ScoreMaliciousContent }), "malicious_content", &contentID)
}

func (s *ReputationService) PenalizeMaliciousPR(userID int64, prID int64) error {
	return s.AddReputation(userID, s.score(-3, func() int { return s.cfg.Reputation.ScoreMaliciousPR }), "malicious_pr", &prID)
}

func (s *ReputationService) PenalizeMaliciousComment(userID int64, commentID int64) error {
	return s.AddReputation(userID, s.score(-2, func() int { return s.cfg.Reputation.ScoreMaliciousComment }), "malicious_comment", &commentID)
}

func (s *ReputationService) PenalizeMaliciousReport(userID int64, reportID int64) error {
	return s.AddReputation(userID, s.score(-2, func() int { return s.cfg.Reputation.ScoreMaliciousReport }), "malicious_report", &reportID)
}

func (s *ReputationService) PenalizeMaliciousTagReport(userID int64, tagID int64) error {
	return s.AddReputation(userID, s.score(-1, func() int { return s.cfg.Reputation.ScoreMaliciousTagReport }), "malicious_tag_report", &tagID)
}

func (s *ReputationService) AwardValidReport(reporterID int64, contentID int64) error {
	var count int64
	s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'valid_report' AND related_id = ?", reporterID, contentID).
		Count(&count)
	if count > 0 {
		return nil
	}
	return s.AddReputation(reporterID, s.score(1, func() int { return s.cfg.Reputation.ScoreValidReport }), "valid_report", &contentID)
}

func (s *ReputationService) PenalizeJudgeError(userID int64, caseID int64) error {
	return s.AddReputation(userID, s.score(-1, func() int { return s.cfg.Reputation.ScoreJudgeError }), "judge_error", &caseID)
}

func (s *ReputationService) CancelJudgeErrorPenalty(userID int64, caseID int64) error {
	return s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'judge_error' AND related_id = ?", userID, caseID).
		Delete(nil).Error
}
