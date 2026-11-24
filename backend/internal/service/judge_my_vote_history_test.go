package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

// #1022：GetMyVoteHistory —— 仅本人投票、created_at 倒序、分页边界与
// 理由赞/踩聚合（LEFT JOIN judge_reason_votes）。

func setupMyVoteHistoryStack(t *testing.T) (*JudgeService, *gorm.DB) {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.JudgeCase{}, &model.JudgeVote{},
		&model.JudgeReasonVote{}, &model.JudgeQualification{},
	))
	cfg := &config.Config{Judge: config.JudgeConfig{MinVotesRequired: 5, PassThreshold: 0.6}}
	return NewJudgeService(repository.NewJudgeRepository(db), NewReputationService(db), cfg), db
}

func seedMyVoteHistoryCase(t *testing.T, db *gorm.DB, status string, age time.Duration) *model.JudgeCase {
	t.Helper()
	author := seedReviewUser(t, db)
	content := model.ContentItem{
		Title: "history case content", AuthorID: author, Zone: "original",
		Category: "game", ContentType: "article", Status: "under_review", IsPublic: true,
	}
	require.NoError(t, db.Create(&content).Error)
	judgeCase := model.JudgeCase{
		TargetType: "article", TargetID: content.ID, Status: status, MinVotes: 5,
	}
	require.NoError(t, db.Create(&judgeCase).Error)
	if status != "open" {
		closedAt := time.Now().Add(-age)
		require.NoError(t, db.Model(&model.JudgeCase{}).Where("id = ?", judgeCase.ID).
			Updates(map[string]interface{}{"closed_at": closedAt}).Error)
	}
	return &judgeCase
}

func TestGetMyVoteHistoryOrdersByCreatedAtDescAndPaginates(t *testing.T) {
	judgeSvc, db := setupMyVoteHistoryStack(t)
	judgeID := seedReviewUser(t, db)
	otherID := seedReviewUser(t, db)

	oldCase := seedMyVoteHistoryCase(t, db, "open", 2*time.Hour)
	newCase := seedMyVoteHistoryCase(t, db, "closed_reject", time.Hour)

	voteOld := model.JudgeVote{
		CaseID: oldCase.ID, JudgeID: judgeID, Vote: "reject",
		Reason: "old vote reason", CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	require.NoError(t, db.Create(&voteOld).Error)
	voteNew := model.JudgeVote{
		CaseID: newCase.ID, JudgeID: judgeID, Vote: "approve",
		Reason: "new vote reason", CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, db.Create(&voteNew).Error)
	// 他人投票不得混入本人历史。
	require.NoError(t, db.Create(&model.JudgeVote{
		CaseID: newCase.ID, JudgeID: otherID, Vote: "reject",
		CreatedAt: time.Now(),
	}).Error)
	// 本人新投票上的理由赞/踩各一票。
	require.NoError(t, db.Create(&model.JudgeReasonVote{
		ReasonOwnerVoteID: voteNew.ID, VoterID: otherID, VoteType: "up",
	}).Error)
	require.NoError(t, db.Create(&model.JudgeReasonVote{
		ReasonOwnerVoteID: voteNew.ID, VoterID: seedReviewUser(t, db), VoteType: "down",
	}).Error)

	rows, total, err := judgeSvc.GetMyVoteHistory(judgeID, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total, "total 只计本人投票")
	require.Len(t, rows, 2)
	require.Equal(t, newCase.ID, rows[0].CaseID, "created_at 倒序：新投票在前")
	require.Equal(t, "approve", rows[0].MyVote)
	require.Equal(t, "closed_reject", rows[0].CaseStatus)
	require.NotNil(t, rows[0].ClosedAt)
	require.Equal(t, int64(1), rows[0].Upvotes, "理由赞数应聚合 reason votes")
	require.Equal(t, int64(1), rows[0].Downvotes, "理由踩数应聚合 reason votes")
	require.Equal(t, oldCase.ID, rows[1].CaseID)
	require.Equal(t, "open", rows[1].CaseStatus)
	require.Nil(t, rows[1].ClosedAt)
	require.Equal(t, int64(0), rows[1].Upvotes)

	// 分页：page_size=1 → 每页一行，第二页是旧投票。
	page1, total, err := judgeSvc.GetMyVoteHistory(judgeID, 1, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, page1, 1)
	require.Equal(t, newCase.ID, page1[0].CaseID)
	page2, _, err := judgeSvc.GetMyVoteHistory(judgeID, 2, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, oldCase.ID, page2[0].CaseID)
}
