package service

// T39（FIX-03）：closed 词表统一 + 撤权激活 + 准确率奖励接线。
// ①撤权判断与结案横幅匹配 closed_ 前缀（裸 closed 恒假=死代码）；
// ②撤权 outcome 阈值读 cfg.Judge.PassThreshold 与闭案口径一致；
// ③闭案提交后多数派在册判官 AwardJudgeAccuracy(+1)（幂等）；
// ④防误伤守卫：窗口内取回票数 >10 才生效（回归；ErrorRateWindow=15 档）。

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

func setupT39JudgeTest(t *testing.T, passThreshold float64) (*JudgeService, *gorm.DB) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.JudgeCase{}, &model.JudgeVote{},
		&model.JudgeReasonVote{}, &model.JudgeQualification{}, &model.ReputationLog{},
		&model.OutboxEvent{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })
	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{Judge: config.JudgeConfig{MinVotesRequired: 2, PassThreshold: passThreshold, ErrorRateWindow: 15}}
	notifSvc := NewNotificationService(repository.NewNotificationRepository(db))

	svc := NewJudgeService(repository.NewJudgeRepository(db), NewReputationService(db), cfg)
	svc.SetNotificationService(notifSvc)
	svc.SetContentOutcomeWriter(db, rdb, repository.NewOutboxRepository(db))
	return svc, db
}

func seedT39JudgeUser(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		ID:           id,
		Email:        "t39-" + string(rune('a'+id%26)) + "@example.test",
		Username:     "t39_user_" + intToStr(int(id)),
		PasswordHash: "hash",
		Role:         "user",
		Reputation:   10,
	}).Error)
	require.NoError(t, db.Create(&model.JudgeQualification{UserID: id, ContentType: "article", IsActive: true}).Error)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func TestT39RevokeTriggersOnClosedPrefixHistory(t *testing.T) {
	svc, db := setupT39JudgeTest(t, 0.6)
	// 被撤权判官：11 票历史，其中 6 票与多数派相左（closed_approve 案投 reject）。
	seedT39JudgeUser(t, db, 3901)
	for i := 1; i <= 6; i++ {
		require.NoError(t, db.Create(&model.JudgeCase{ID: int64(3900 + i), TargetType: "content", TargetID: int64(i), Status: "closed_approve", VoteApprove: 15, VoteReject: 5}).Error)
		require.NoError(t, db.Create(&model.JudgeVote{CaseID: int64(3900 + i), JudgeID: 3901, Vote: "reject"}).Error)
	}
	for i := 1; i <= 5; i++ {
		require.NoError(t, db.Create(&model.JudgeCase{ID: int64(3910 + i), TargetType: "content", TargetID: int64(100 + i), Status: "open"}).Error)
		require.NoError(t, db.Create(&model.JudgeVote{CaseID: int64(3910 + i), JudgeID: 3901, Vote: "approve"}).Error)
	}

	require.NoError(t, svc.checkAndRevokeIfNeeded(3901))

	var quals int64
	require.NoError(t, db.Model(&model.JudgeQualification{}).Where("user_id = ? AND is_active = ?", 3901, true).Count(&quals).Error)
	require.Zero(t, quals, "closed_ 前缀终态应参与撤权统计：错误率 6/11 > 0.5 → 撤权")

	var penalty int64
	require.NoError(t, db.Model(&model.ReputationLog{}).Where("user_id = ? AND reason = ?", 3901, "judge_error_rate").Count(&penalty).Error)
	require.EqualValues(t, 1, penalty, "撤权伴随 -1 信誉分")
}

func TestT39RevokeGuardBelowWindow(t *testing.T) {
	svc, db := setupT39JudgeTest(t, 0.6)
	seedT39JudgeUser(t, db, 3902)
	// 仅 10 票全错：窗口守卫（>10 才生效）不得撤权。
	for i := 1; i <= 10; i++ {
		require.NoError(t, db.Create(&model.JudgeCase{ID: int64(3950 + i), TargetType: "content", TargetID: int64(200 + i), Status: "closed_approve", VoteApprove: 15, VoteReject: 5}).Error)
		require.NoError(t, db.Create(&model.JudgeVote{CaseID: int64(3950 + i), JudgeID: 3902, Vote: "reject"}).Error)
	}

	require.NoError(t, svc.checkAndRevokeIfNeeded(3902))

	var quals int64
	require.NoError(t, db.Model(&model.JudgeQualification{}).Where("user_id = ? AND is_active = ?", 3902, true).Count(&quals).Error)
	require.EqualValues(t, 1, quals, "窗口内票数未超过 10 不得撤权（防误伤守卫回归）")
}

func TestT39OutcomeThresholdReadsConfig(t *testing.T) {
	svc, db := setupT39JudgeTest(t, 0.8)
	seedT39JudgeUser(t, db, 3903)
	// 闭案口径 80% 下 approve 票 75% 判 reject 案（closed_reject）；
	// 撤权阈值若硬编码 0.6 会把该案判为 approve（口径漂移）。
	for i := 1; i <= 11; i++ {
		require.NoError(t, db.Create(&model.JudgeCase{ID: int64(3970 + i), TargetType: "content", TargetID: int64(300 + i), Status: "closed_reject", VoteApprove: 15, VoteReject: 5}).Error)
		require.NoError(t, db.Create(&model.JudgeVote{CaseID: int64(3970 + i), JudgeID: 3903, Vote: "approve"}).Error)
	}

	require.NoError(t, svc.checkAndRevokeIfNeeded(3903))

	var quals int64
	require.NoError(t, db.Model(&model.JudgeQualification{}).Where("user_id = ? AND is_active = ?", 3903, true).Count(&quals).Error)
	require.Zero(t, quals, "阈值读配置后：11/11 与 60% 口径的多数派相左 → 撤权激活")
}

func TestT39MajorityJudgesAwardedOnClose(t *testing.T) {
	svc, db := setupT39JudgeTest(t, 0.6)
	// 三判官：A/B 多数派（approve），C 少数派（reject）。min_votes=2，B 投后闭案。
	seedT39JudgeUser(t, db, 3911)
	seedT39JudgeUser(t, db, 3912)
	seedT39JudgeUser(t, db, 3913)
	// 少数派 C 投票后立即被撤权也不影响已记票；此处保持全员在册。
	require.NoError(t, db.Create(&model.JudgeCase{ID: 3910, TargetType: "content", TargetID: 400, Status: "open", MinVotes: 2}).Error)

	require.NoError(t, svc.SubmitVote(SubmitVoteInput{CaseID: 3910, Vote: "approve", Reason: "违规"}, 3911))
	require.NoError(t, svc.SubmitVote(SubmitVoteInput{CaseID: 3910, Vote: "approve"}, 3912))
	require.NoError(t, svc.SubmitVote(SubmitVoteInput{CaseID: 3910, Vote: "reject"}, 3913))

	var status string
	require.NoError(t, db.Model(&model.JudgeCase{}).Where("id = ?", 3910).Pluck("status", &status).Error)
	require.Equal(t, "closed_approve", status)

	var awardedMajority int64
	require.NoError(t, db.Model(&model.ReputationLog{}).
		Where("user_id IN ? AND reason = ? AND related_id = ?", []int64{3911, 3912}, "judge_accuracy", int64(3910)).
		Count(&awardedMajority).Error)
	require.EqualValues(t, 2, awardedMajority, "多数派在册判官各 +1 准确率奖励")

	var awardedMinority int64
	require.NoError(t, db.Model(&model.ReputationLog{}).
		Where("user_id = ? AND reason = ?", int64(3913), "judge_accuracy").
		Count(&awardedMinority).Error)
	require.Zero(t, awardedMinority, "少数派不奖励")
}

func TestT39AwardJudgeAccuracyIdempotent(t *testing.T) {
	svc, db := setupT39JudgeTest(t, 0.6)
	seedT39JudgeUser(t, db, 3921)

	require.NoError(t, svc.reputSvc.AwardJudgeAccuracy(3921, 3925))
	require.NoError(t, svc.reputSvc.AwardJudgeAccuracy(3921, 3925), "同案重复奖励幂等")

	var logs int64
	require.NoError(t, db.Model(&model.ReputationLog{}).Where("user_id = ? AND reason = ? AND related_id = ?", 3921, "judge_accuracy", int64(3925)).Count(&logs).Error)
	require.EqualValues(t, 1, logs)
}
