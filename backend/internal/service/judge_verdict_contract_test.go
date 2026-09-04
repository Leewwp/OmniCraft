package service

// T38（FIX-36b）：verdict 契约 + 理由投票守卫。
// ①GetVerdictDetail 返回 votes:[{judge_name,upvotes,downvotes}]（join users +
//   reason votes 聚合；裸 votes 致前端 NaN）；
// ②VoteReason 加判官资格校验（无任一有效资格 403）+ 禁自赞（owner==caller 409）；
// ③重复投票幂等/切换（UNIQUE(reason_owner_vote_id, voter_id)）。

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

func setupT38VerdictTest(t *testing.T) (*JudgeService, *gorm.DB) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.JudgeCase{}, &model.JudgeVote{},
		&model.JudgeReasonVote{}, &model.JudgeQualification{},
	))

	cfg := &config.Config{Judge: config.JudgeConfig{MinVotesRequired: 2, PassThreshold: 0.6}}
	svc := NewJudgeService(repository.NewJudgeRepository(db), NewReputationService(db), cfg)
	return svc, db
}

func seedT38Judge(t *testing.T, db *gorm.DB, id int64, username string) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		ID:           id,
		Email:        username + "@example.test",
		Username:     username,
		PasswordHash: "hash",
		Role:         "user",
		Reputation:   10,
	}).Error)
}

func TestT38VerdictVotesContract(t *testing.T) {
	svc, db := setupT38VerdictTest(t)
	seedT38Judge(t, db, 3801, "t38_judge_a")
	seedT38Judge(t, db, 3802, "t38_judge_b")
	seedT38Judge(t, db, 3803, "t38_voter_c")
	seedT38Judge(t, db, 3804, "t38_voter_d")

	require.NoError(t, db.Create(&model.JudgeCase{ID: 3801, TargetType: "content", TargetID: 1, Status: "open", MinVotes: 20}).Error)
	voteA := model.JudgeVote{CaseID: 3801, JudgeID: 3801, Vote: "approve", Reason: "内容确实违规"}
	voteB := model.JudgeVote{CaseID: 3801, JudgeID: 3802, Vote: "reject"}
	require.NoError(t, db.Create(&voteA).Error)
	require.NoError(t, db.Create(&voteB).Error)

	// 理由 A：2 赞 1 踩；理由 B：无理由无投票。
	require.NoError(t, db.Create(&model.JudgeReasonVote{ReasonOwnerVoteID: voteA.ID, VoterID: 3802, VoteType: "up"}).Error)
	require.NoError(t, db.Create(&model.JudgeReasonVote{ReasonOwnerVoteID: voteA.ID, VoterID: 3803, VoteType: "up"}).Error)
	require.NoError(t, db.Create(&model.JudgeReasonVote{ReasonOwnerVoteID: voteA.ID, VoterID: 3804, VoteType: "down"}).Error)

	_, votes, err := svc.GetVerdictDetail(3801)
	require.NoError(t, err)
	require.Len(t, votes, 2)

	byJudge := map[int64]struct {
		name      string
		reason    *string
		upvotes   int64
		downvotes int64
	}{}
	for _, v := range votes {
		byJudge[v.JudgeID] = struct {
			name      string
			reason    *string
			upvotes   int64
			downvotes int64
		}{v.JudgeName, v.Reason, v.Upvotes, v.Downvotes}
	}
	require.Equal(t, "t38_judge_a", byJudge[3801].name, "必须 join users 返回昵称")
	require.NotNil(t, byJudge[3801].reason)
	require.Equal(t, "内容确实违规", *byJudge[3801].reason)
	require.EqualValues(t, 2, byJudge[3801].upvotes, "赞数聚合")
	require.EqualValues(t, 1, byJudge[3801].downvotes, "踩数聚合")
	require.Equal(t, "t38_judge_b", byJudge[3802].name)
	require.EqualValues(t, 0, byJudge[3802].upvotes)
}

func TestT38VoteReasonSelfVoteRejected(t *testing.T) {
	svc, db := setupT38VerdictTest(t)
	seedT38Judge(t, db, 3811, "t38_owner")
	require.NoError(t, db.Create(&model.JudgeCase{ID: 3811, TargetType: "content", TargetID: 1, Status: "open", MinVotes: 20}).Error)
	vote := model.JudgeVote{CaseID: 3811, JudgeID: 3811, Vote: "approve", Reason: "my own reason"}
	require.NoError(t, db.Create(&vote).Error)

	err := svc.VoteReason(vote.ID, 3811, "up")
	require.ErrorIs(t, err, ErrReasonSelfVote)
}

func TestT38VoteReasonRequiresQualification(t *testing.T) {
	svc, db := setupT38VerdictTest(t)
	seedT38Judge(t, db, 3821, "t38_owner")
	seedT38Judge(t, db, 3822, "t38_voter")
	require.NoError(t, db.Create(&model.JudgeCase{ID: 3821, TargetType: "content", TargetID: 1, Status: "open", MinVotes: 20}).Error)
	vote := model.JudgeVote{CaseID: 3821, JudgeID: 3821, Vote: "approve", Reason: "a reason"}
	require.NoError(t, db.Create(&vote).Error)

	// 无任一有效判官资格 → 403。
	err := svc.VoteReason(vote.ID, 3822, "up")
	require.ErrorIs(t, err, ErrJudgeQualificationRequired)

	// 持有效资格 → 成功。
	require.NoError(t, db.Create(&model.JudgeQualification{UserID: 3822, ContentType: "article", IsActive: true}).Error)
	require.NoError(t, svc.VoteReason(vote.ID, 3822, "up"))
}

func TestT38VoteReasonUpsertIdempotentAndSwitch(t *testing.T) {
	svc, db := setupT38VerdictTest(t)
	seedT38Judge(t, db, 3831, "t38_owner")
	seedT38Judge(t, db, 3832, "t38_voter")
	require.NoError(t, db.Create(&model.JudgeQualification{UserID: 3832, ContentType: "article", IsActive: true}).Error)
	require.NoError(t, db.Create(&model.JudgeCase{ID: 3831, TargetType: "content", TargetID: 1, Status: "open", MinVotes: 20}).Error)
	vote := model.JudgeVote{CaseID: 3831, JudgeID: 3831, Vote: "approve", Reason: "a reason"}
	require.NoError(t, db.Create(&vote).Error)

	require.NoError(t, svc.VoteReason(vote.ID, 3832, "up"))
	require.NoError(t, svc.VoteReason(vote.ID, 3832, "up"), "同方向重复投票幂等")

	var count int64
	require.NoError(t, db.Model(&model.JudgeReasonVote{}).Where("reason_owner_vote_id = ? AND voter_id = ?", vote.ID, 3832).Count(&count).Error)
	require.EqualValues(t, 1, count, "UNIQUE 维度只允许一条")

	require.NoError(t, svc.VoteReason(vote.ID, 3832, "down"), "反方向为切换")
	var rv model.JudgeReasonVote
	require.NoError(t, db.Where("reason_owner_vote_id = ? AND voter_id = ?", vote.ID, 3832).First(&rv).Error)
	require.Equal(t, "down", rv.VoteType)
}

func TestT38VoteReasonTargetVoteNotFound(t *testing.T) {
	svc, db := setupT38VerdictTest(t)
	seedT38Judge(t, db, 3841, "t38_voter")
	require.NoError(t, db.Create(&model.JudgeQualification{UserID: 3841, ContentType: "article", IsActive: true}).Error)

	err := svc.VoteReason(99999, 3841, "up")
	require.ErrorIs(t, err, ErrReasonVoteNotFound)
}
