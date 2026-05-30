package service

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type ReputationService struct {
	db *gorm.DB
}

func NewReputationService(db *gorm.DB) *ReputationService {
	return &ReputationService{db: db}
}

func (s *ReputationService) AddReputation(userID int64, delta int, reason string, relatedID *int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
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
	return s.AddReputation(userID, 3, "quality_content", &contentID)
}

func (s *ReputationService) AwardPRMerged(userID int64, prID int64) error {
	return s.AddReputation(userID, 3, "pr_merged", &prID)
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
	return s.AddReputation(userID, 2, "quality_comment", &commentID)
}

func (s *ReputationService) AwardTagRecognized(userID int64, tagSuggestionID int64) error {
	return s.AddReputation(userID, 1, "tag_recognized", &tagSuggestionID)
}

func (s *ReputationService) AwardJudgeAccuracy(userID int64, caseID int64) error {
	return s.AddReputation(userID, 1, "judge_accuracy", &caseID)
}

func (s *ReputationService) AwardRehabCourse(userID int64, courseID int64) error {
	var count int64
	s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'rehab_course' AND related_id = ?", userID, courseID).
		Count(&count)
	if count > 0 {
		return nil
	}
	return s.AddReputation(userID, 1, "rehab_course", &courseID)
}

func (s *ReputationService) PenalizeMaliciousContent(userID int64, contentID int64) error {
	return s.AddReputation(userID, -3, "malicious_content", &contentID)
}

func (s *ReputationService) PenalizeMaliciousPR(userID int64, prID int64) error {
	return s.AddReputation(userID, -3, "malicious_pr", &prID)
}

func (s *ReputationService) PenalizeMaliciousComment(userID int64, commentID int64) error {
	return s.AddReputation(userID, -2, "malicious_comment", &commentID)
}

func (s *ReputationService) PenalizeMaliciousReport(userID int64, reportID int64) error {
	return s.AddReputation(userID, -2, "malicious_report", &reportID)
}

func (s *ReputationService) PenalizeMaliciousTagReport(userID int64, tagID int64) error {
	return s.AddReputation(userID, -1, "malicious_tag_report", &tagID)
}

func (s *ReputationService) AwardValidReport(reporterID int64, contentID int64) error {
	var count int64
	s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'valid_report' AND related_id = ?", reporterID, contentID).
		Count(&count)
	if count > 0 {
		return nil
	}
	return s.AddReputation(reporterID, 1, "valid_report", &contentID)
}

func (s *ReputationService) CancelJudgeErrorPenalty(userID int64, caseID int64) error {
	return s.db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = 'judge_error' AND related_id = ?", userID, caseID).
		Delete(nil).Error
}
