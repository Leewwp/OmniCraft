package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

// run-1 审计 #7/#14：SubmitVote 此前既不校验判官资格（零资格账号可投票
// 决定内容发布/封禁，与 VoteReason 的资格闸不一致），也不拒绝已结案案例
//（迟到投票可再触发关闭改写已记录裁决）；CloseCase 的 UPDATE 不带
// status='open' 守卫，并发/重复关闭会互相覆盖。

func setupJudgeVoteGuardStack(t *testing.T) (*JudgeService, *gorm.DB) {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.JudgeCase{}, &model.JudgeVote{},
		&model.JudgeQualification{}, &model.ReputationLog{}, &model.OutboxEvent{},
	))

	// MinVotes=5：单票不触发结案，聚焦守卫本身。
	cfg := &config.Config{Judge: config.JudgeConfig{MinVotesRequired: 5, PassThreshold: 0.6}}
	judgeSvc := NewJudgeService(repository.NewJudgeRepository(db), NewReputationService(db), cfg)
	judgeSvc.SetContentOutcomeWriter(db, nil, repository.NewOutboxRepository(db))
	return judgeSvc, db
}

func seedJudgeVoteGuardCase(t *testing.T, db *gorm.DB, status string) (caseID int64, judgeID int64) {
	t.Helper()
	author := seedReviewUser(t, db)
	judgeID = seedReviewUser(t, db)
	content := model.ContentItem{
		Title: "guard content", AuthorID: author, Zone: "original",
		Category: "game", ContentType: "article", Status: "under_review", IsPublic: true,
	}
	require.NoError(t, db.Create(&content).Error)
	judgeCase := model.JudgeCase{
		TargetType: "article", TargetID: content.ID, Status: status, MinVotes: 5,
	}
	require.NoError(t, db.Create(&judgeCase).Error)
	return judgeCase.ID, judgeID
}

// 审计 #7：无 judge_qualifications 行的用户投票 → ErrJudgeQualificationRequired
// （403 JUDGE_QUALIFICATION_REQUIRED）；持匹配类型资格行的用户投票成功。
func TestSubmitVoteRequiresQualification(t *testing.T) {
	judgeSvc, db := setupJudgeVoteGuardStack(t)
	caseID, judgeID := seedJudgeVoteGuardCase(t, db, "open")

	err := judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judgeID)
	require.ErrorIs(t, err, ErrJudgeQualificationRequired, "unqualified users must not vote on judge cases")

	var voteCount int64
	require.NoError(t, db.Model(&model.JudgeVote{}).Where("case_id = ?", caseID).Count(&voteCount).Error)
	require.Equal(t, int64(0), voteCount, "the rejected vote must not persist")

	require.NoError(t, db.Create(&model.JudgeQualification{
		UserID: judgeID, ContentType: "article", IsActive: true,
	}).Error)
	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judgeID))
	require.NoError(t, db.Model(&model.JudgeVote{}).Where("case_id = ?", caseID).Count(&voteCount).Error)
	require.Equal(t, int64(1), voteCount, "a qualified judge's vote persists")
}

// 审计 #7 补充：资格类型不匹配（案例 article、资格 skin）同样拒绝。
func TestSubmitVoteRejectsMismatchedQualificationType(t *testing.T) {
	judgeSvc, db := setupJudgeVoteGuardStack(t)
	caseID, judgeID := seedJudgeVoteGuardCase(t, db, "open")

	require.NoError(t, db.Create(&model.JudgeQualification{
		UserID: judgeID, ContentType: "skin", IsActive: true,
	}).Error)
	err := judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judgeID)
	require.ErrorIs(t, err, ErrJudgeQualificationRequired, "qualifications of another content type must not admit the vote")
}

// 审计 #14：结案（closed_*）案例拒绝新投票——迟到投票不得改写已记录裁决。
func TestSubmitVoteRejectsClosedCase(t *testing.T) {
	judgeSvc, db := setupJudgeVoteGuardStack(t)
	for _, status := range []string{"closed_approve", "closed_reject"} {
		caseID, judgeID := seedJudgeVoteGuardCase(t, db, status)
		require.NoError(t, db.Create(&model.JudgeQualification{
			UserID: judgeID, ContentType: "article", IsActive: true,
		}).Error)

		err := judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "reject"}, judgeID)
		require.ErrorIs(t, err, ErrCaseClosed, "closed case (%s) must reject new votes", status)

		var voteCount int64
		require.NoError(t, db.Model(&model.JudgeVote{}).Where("case_id = ?", caseID).Count(&voteCount).Error)
		require.Equal(t, int64(0), voteCount, "vote on a closed case must not persist")
	}
}

// 审计 #14：CloseCase guarded claim——只有 status='open' 的行可被关闭；
// 第二次关闭（含不同结论）不得生效，首次裁决不被覆盖（并发关闭仅一次生效）。
func TestCloseCaseGuardedClaimIsIdempotent(t *testing.T) {
	_, db := setupJudgeVoteGuardStack(t)
	caseID, _ := seedJudgeVoteGuardCase(t, db, "open")
	repo := repository.NewJudgeRepository(db)

	require.NoError(t, repo.CloseCase(caseID, "closed_approve", 3, 2))

	err := repo.CloseCase(caseID, "closed_reject", 2, 3)
	require.ErrorIs(t, err, repository.ErrCaseAlreadyClosed, "the second close must lose the guarded claim")

	var judgeCase model.JudgeCase
	require.NoError(t, db.First(&judgeCase, caseID).Error)
	require.Equal(t, "closed_approve", judgeCase.Status, "the concurrent conflicting close must not overwrite the recorded verdict")
	require.Equal(t, 3, judgeCase.VoteApprove)
	require.Equal(t, 2, judgeCase.VoteReject)
}
