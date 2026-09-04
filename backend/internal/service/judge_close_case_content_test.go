package service

import (
	"context"
	"strconv"
	"testing"
	"time"

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

// T41（FIX-10）：判官闭案回写内容状态 + 作者通知。closed_approve → 内容
// 条件恢复 published（守卫防覆盖 admin/AI 终态 banned）+ 重发索引事件；
// closed_reject → banned + ban_reason（不扣信誉分）；两态均通知作者
// （T11 契约）并失效内容缓存（T10 helper）。

func setupT41JudgeCloseTest(t *testing.T) (*JudgeService, *gorm.DB, *t11CaptureProducer, *miniredis.Miniredis) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.JudgeCase{}, &model.JudgeVote{},
		&model.ReputationLog{}, &model.OutboxEvent{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })
	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{Judge: config.JudgeConfig{MinVotesRequired: 2, PassThreshold: 0.6}}
	producer := &t11CaptureProducer{}
	notifSvc := NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)

	judgeSvc := NewJudgeService(repository.NewJudgeRepository(db), NewReputationService(db), cfg)
	judgeSvc.SetNotificationService(notifSvc)
	judgeSvc.SetContentOutcomeWriter(db, rdb, repository.NewOutboxRepository(db))
	return judgeSvc, db, producer, mr
}

func seedT41Users(t *testing.T, db *gorm.DB) (author, judge1, judge2 int64) {
	t.Helper()
	author = seedReviewUser(t, db)
	judge1 = seedReviewUser(t, db)
	judge2 = seedReviewUser(t, db)
	return
}

func seedT41Content(t *testing.T, db *gorm.DB, authorID int64, status string) int64 {
	t.Helper()
	content := model.ContentItem{
		Title:       "T41 judge content",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      status,
		IsPublic:    true,
	}
	require.NoError(t, db.Create(&content).Error)
	return content.ID
}

func seedT41JudgeCase(t *testing.T, db *gorm.DB, contentID int64) int64 {
	t.Helper()
	judgeCase := model.JudgeCase{
		TargetType: "article",
		TargetID:   contentID,
		Status:     "open",
		MinVotes:   2,
	}
	require.NoError(t, db.Create(&judgeCase).Error)
	return judgeCase.ID
}

func t41ContentStatus(t *testing.T, db *gorm.DB, id int64) (string, string) {
	t.Helper()
	var content model.ContentItem
	require.NoError(t, db.First(&content, id).Error)
	return content.Status, content.BanReason
}

func TestJudgeCloseApproveRestoresContentAndNotifiesAuthor(t *testing.T) {
	judgeSvc, db, producer, mr := setupT41JudgeCloseTest(t)
	author, judge1, judge2 := seedT41Users(t, db)
	contentID := seedT41Content(t, db, author, "under_review")
	caseID := seedT41JudgeCase(t, db, contentID)
	require.NoError(t, redisclient.SetJSON(context.Background(), "cache:content:"+strconv.FormatInt(contentID, 10), nil, time.Minute))
	require.True(t, mr.Exists("cache:content:"+strconv.FormatInt(contentID, 10)))

	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judge1))
	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judge2))

	status, _ := t41ContentStatus(t, db, contentID)
	require.Equal(t, "published", status, "closed_approve 应恢复内容展示（FIX-10）")

	var judgeCase model.JudgeCase
	require.NoError(t, db.First(&judgeCase, caseID).Error)
	require.Equal(t, "closed_approve", judgeCase.Status)

	var publishedEvents int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Where("event_type = ?", "content.published").Count(&publishedEvents).Error)
	require.Equal(t, int64(1), publishedEvents, "恢复展示应重发 TopicContentPublished 喂索引")

	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 1, "闭案应通知作者（T11 契约）")
	require.EqualValues(t, author, notifies[0]["user_id"])
	require.Contains(t, notifies[0]["body"], "已通过审核")

	require.False(t, mr.Exists("cache:content:"+strconv.FormatInt(contentID, 10)), "闭案回写应失效内容缓存（T10 helper）")
}

func TestJudgeCloseRejectBansContentWithoutReputationPenalty(t *testing.T) {
	judgeSvc, db, producer, _ := setupT41JudgeCloseTest(t)
	author, judge1, judge2 := seedT41Users(t, db)
	contentID := seedT41Content(t, db, author, "under_review")
	caseID := seedT41JudgeCase(t, db, contentID)

	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "reject"}, judge1))
	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "reject"}, judge2))

	status, banReason := t41ContentStatus(t, db, contentID)
	require.Equal(t, "banned", status, "closed_reject 应下架内容（FIX-10）")
	require.Contains(t, banReason, "judge", "ban_reason 应携带 judge_verdict 口径")

	var bannedEvents int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Where("event_type = ?", "content.banned").Count(&bannedEvents).Error)
	require.Equal(t, int64(1), bannedEvents, "下架应发 TopicContentBanned")

	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 1)
	require.EqualValues(t, author, notifies[0]["user_id"])
	require.Contains(t, notifies[0]["body"], "封禁")
	require.Contains(t, notifies[0]["body"], "judge", "通知 body 应带闭案 reason（T11 契约）")

	var reputationLogs int64
	require.NoError(t, db.Model(&model.ReputationLog{}).Count(&reputationLogs).Error)
	require.Equal(t, int64(0), reputationLogs, "判官闭案 reject 不扣信誉分（扣分语义属 AI/admin 通道）")
}

func TestJudgeCloseApproveDoesNotOverrideTerminalBan(t *testing.T) {
	judgeSvc, db, producer, _ := setupT41JudgeCloseTest(t)
	author, judge1, judge2 := seedT41Users(t, db)
	// admin/AI 终态 banned：众裁 late verdict 不得覆盖。
	contentID := seedT41Content(t, db, author, "banned")
	caseID := seedT41JudgeCase(t, db, contentID)

	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judge1))
	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judge2))

	status, _ := t41ContentStatus(t, db, contentID)
	require.Equal(t, "banned", status, "守卫：admin/AI 终态 banned 不被众裁 approve 覆盖")

	var publishedEvents int64
	require.NoError(t, db.Model(&model.OutboxEvent{}).Where("event_type = ?", "content.published").Count(&publishedEvents).Error)
	require.Equal(t, int64(0), publishedEvents, "未发生真实转换不应发索引事件")

	require.Len(t, producer.contentStatusNotifies(), 0, "守卫拦下的闭案不应发作者通知")
}

func TestJudgeSubmitVoteBelowThresholdDoesNotTouchContent(t *testing.T) {
	judgeSvc, db, producer, _ := setupT41JudgeCloseTest(t)
	author, judge1, _ := seedT41Users(t, db)
	contentID := seedT41Content(t, db, author, "under_review")
	caseID := seedT41JudgeCase(t, db, contentID)

	require.NoError(t, judgeSvc.SubmitVote(SubmitVoteInput{CaseID: caseID, Vote: "approve"}, judge1))

	status, _ := t41ContentStatus(t, db, contentID)
	require.Equal(t, "under_review", status, "票数未达 MinVotes 不闭案不回写")
	require.Len(t, producer.contentStatusNotifies(), 0)
}
