package repository

import (
	"time"

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

func (r *JudgeRepository) CreateQualification(userID int64, contentType string) error {
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

func (r *JudgeRepository) ListOpenCases(qualifiedTypes []string, judgeID int64, page, pageSize int) ([]model.JudgeCase, int64, error) {
	var cases []model.JudgeCase
	var total int64
	// T40（FIX-36d）：排除本人已投案件——投票后不再出现在队列。
	q := r.db.Model(&model.JudgeCase{}).
		Where("status = ? AND target_type IN ?", "open", qualifiedTypes).
		Where("NOT EXISTS (SELECT 1 FROM judge_votes v WHERE v.case_id = judge_cases.id AND v.judge_id = ?)", judgeID)
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

// GetVoteByID 按 ID 取投票（T38 理由投票守卫需 owner 判定）。
func (r *JudgeRepository) GetVoteByID(id int64) (*model.JudgeVote, error) {
	var vote model.JudgeVote
	if err := r.db.First(&vote, id).Error; err != nil {
		return nil, err
	}
	return &vote, nil
}

// UpsertReasonVote（T38）：同方向重复投票幂等，反方向切换（UNIQUE 维度单行）。
func (r *JudgeRepository) UpsertReasonVote(voteID, voterID int64, voteType string) error {
	var existing model.JudgeReasonVote
	err := r.db.Where("reason_owner_vote_id = ? AND voter_id = ?", voteID, voterID).First(&existing).Error
	if err == nil {
		if existing.VoteType == voteType {
			return nil
		}
		existing.VoteType = voteType
		return r.db.Save(&existing).Error
	}
	return r.db.Create(&model.JudgeReasonVote{
		ReasonOwnerVoteID: voteID,
		VoterID:           voterID,
		VoteType:          voteType,
	}).Error
}

// VerdictVoteItem（T38/FIX-36b）：verdict 投票条目对外契约——join users 取
// 昵称 + reason votes 聚合赞/踩（裸 JudgeVote 缺 judge_name/upvotes/downvotes，
// 前端渲染 NaN）。定义在 repository 层避免与 service 循环依赖。
type VerdictVoteItem struct {
	ID        int64     `json:"id"`
	JudgeID   int64     `json:"judge_id"`
	JudgeName string    `json:"judge_name"`
	Vote      string    `json:"vote"`
	Reason    *string   `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
	Upvotes   int64     `json:"upvotes"`
	Downvotes int64     `json:"downvotes"`
}

// ListVerdictVotes（T38/FIX-36b）：verdict 投票契约——join users 取昵称，
// LEFT JOIN reason votes 聚合赞/踩计数。
func (r *JudgeRepository) ListVerdictVotes(caseID int64) ([]VerdictVoteItem, error) {
	var rows []VerdictVoteItem
	err := r.db.Raw(`
		SELECT v.id, v.judge_id, u.username AS judge_name, v.vote, v.reason, v.created_at,
		       COALESCE(SUM(CASE WHEN rv.vote_type = 'up' THEN 1 ELSE 0 END), 0)   AS upvotes,
		       COALESCE(SUM(CASE WHEN rv.vote_type = 'down' THEN 1 ELSE 0 END), 0) AS downvotes
		FROM judge_votes v
		JOIN users u ON u.id = v.judge_id
		LEFT JOIN judge_reason_votes rv ON rv.reason_owner_vote_id = v.id
		WHERE v.case_id = ?
		GROUP BY v.id, v.judge_id, u.username, v.vote, v.reason, v.created_at
		ORDER BY v.created_at ASC
	`, caseID).Scan(&rows).Error
	return rows, err
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

func (r *JudgeRepository) GetDistinctCaseTypes() ([]string, error) {
	var types []string
	err := r.db.Model(&model.JudgeCase{}).
		Where("status IN (?, ?)", "closed_approve", "closed_reject").
		Distinct("target_type").Pluck("target_type", &types).Error
	return types, err
}

func (r *JudgeRepository) ListClosedCasesForQuestioning(contentType string, limit int) ([]model.JudgeCase, error) {
	var cases []model.JudgeCase
	err := r.db.Where(
		"target_type = ? AND status IN (?, ?) AND id NOT IN (SELECT source_case_id FROM judge_questions WHERE source_case_id IS NOT NULL)",
		contentType, "closed_approve", "closed_reject",
	).Order("(vote_approve + vote_reject) DESC").Limit(limit).Find(&cases).Error
	return cases, err
}
