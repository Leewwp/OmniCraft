package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type JudgeRepository struct {
	db *gorm.DB
}

func NewJudgeRepository(db *gorm.DB) *JudgeRepository {
	return &JudgeRepository{db: db}
}

func (r *JudgeRepository) GetRandomQuestions(contentType string, n int) ([]model.JudgeQuestion, error) {
	var questions []model.JudgeQuestion
	err := r.db.Where("content_type = ? AND is_active = TRUE", contentType).
		Order("RANDOM()").Limit(n).Find(&questions).Error
	return questions, err
}

func (r *JudgeRepository) CountQuestions(contentType string) (int64, error) {
	var count int64
	err := r.db.Model(&model.JudgeQuestion{}).
		Where("content_type = ? AND is_active = TRUE", contentType).Count(&count).Error
	return count, err
}

func (r *JudgeRepository) CreateExamRecord(record *model.JudgeExamRecord) error {
	return r.db.Create(record).Error
}

func (r *JudgeRepository) CheckQualification(userID int64, contentType string) (bool, error) {
	var count int64
	err := r.db.Model(&model.JudgeQualification{}).
		Where("user_id = ? AND content_type = ? AND is_active = TRUE", userID, contentType).
		Count(&count).Error
	return count > 0, err
}

func (r *JudgeRepository) CreateQualification(userID uint, contentType string) error {
	q := model.JudgeQualification{
		UserID:      userID,
		ContentType: contentType,
		IsActive:    true,
	}
	return r.db.FirstOrCreate(&q, model.JudgeQualification{UserID: userID, ContentType: contentType}).Error
}

func (r *JudgeRepository) CreateQuestion(q *model.JudgeQuestion) error {
	return r.db.Create(q).Error
}

func (r *JudgeRepository) CreateCase(c *model.JudgeCase) error {
	return r.db.Create(c).Error
}

func (r *JudgeRepository) FindCase(id int64) (*model.JudgeCase, error) {
	var c model.JudgeCase
	err := r.db.First(&c, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *JudgeRepository) ListOpenCases(qualifiedTypes []string, page, pageSize int) ([]model.JudgeCase, int64, error) {
	var cases []model.JudgeCase
	var total int64
	q := r.db.Model(&model.JudgeCase{}).Where("status = ? AND target_type IN ?", "open", qualifiedTypes)
	q.Count(&total)
	offset := (page - 1) * pageSize
	err := q.Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&cases).Error
	return cases, total, err
}

func (r *JudgeRepository) CreateVote(v *model.JudgeVote) error {
	return r.db.Create(v).Error
}

func (r *JudgeRepository) HasVoted(caseID int64, judgeID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.JudgeVote{}).Where("case_id = ? AND judge_id = ?", caseID, judgeID).Count(&count).Error
	return count > 0, err
}

func (r *JudgeRepository) GetVoteStats(caseID int64) (approve, reject int64, err error) {
	r.db.Model(&model.JudgeVote{}).Where("case_id = ? AND vote = ?", caseID, "approve").Count(&approve)
	r.db.Model(&model.JudgeVote{}).Where("case_id = ? AND vote = ?", caseID, "reject").Count(&reject)
	return
}

func (r *JudgeRepository) CloseCase(id int64, status string, approve, reject int) error {
	return r.db.Model(&model.JudgeCase{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"vote_approve": approve,
		"vote_reject":  reject,
		"closed_at":    gorm.Expr("NOW()"),
	}).Error
}

func (r *JudgeRepository) ListVotesForCase(caseID int64) ([]model.JudgeVote, error) {
	var votes []model.JudgeVote
	err := r.db.Where("case_id = ?", caseID).Find(&votes).Error
	return votes, err
}

func (r *JudgeRepository) CreateReasonVote(rv *model.JudgeReasonVote) error {
	return r.db.Create(rv).Error
}

func (r *JudgeRepository) GetUserQualifications(userID int64) ([]model.JudgeQualification, error) {
	var quals []model.JudgeQualification
	err := r.db.Where("user_id = ? AND is_active = TRUE", userID).Find(&quals).Error
	return quals, err
}

func (r *JudgeRepository) GetRecentVoteHistory(judgeID int64, limit int) ([]model.JudgeVote, error) {
	var votes []model.JudgeVote
	err := r.db.Where("judge_id = ?", judgeID).Order("created_at DESC").Limit(limit).Find(&votes).Error
	return votes, err
}

func (r *JudgeRepository) RevokeQualifications(userID int64) error {
	return r.db.Model(&model.JudgeQualification{}).Where("user_id = ?", userID).Update("is_active", false).Error
}
