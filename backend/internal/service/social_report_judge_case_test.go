package service

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

// T42（FIX-11）：举报达阈值 auto-hide 必须触发众裁（创建判官案件）并通知
// 作者——与 Green review 路径行为一致；banned 终态不被举报打回 under_review。

func setupT42ReportJudgeTest(t *testing.T) (*SocialService, *gorm.DB, *t11CaptureProducer, *ReviewService) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.Report{}, &model.JudgeCase{},
	))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })
	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{Social: config.SocialConfig{ReportAutoHideRate: 0.05}, Judge: config.JudgeConfig{MinVotesRequired: 3}}

	producer := &t11CaptureProducer{}
	notifSvc := NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)

	reviewSvc := NewReviewService(db, rdb, cfg, NewReputationService(db))

	socialSvc := NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		cfg, rdb, nil,
	)
	socialSvc.SetNotificationService(notifSvc)
	socialSvc.SetJudgeCaseEnsurer(reviewSvc)
	return socialSvc, db, producer, reviewSvc
}

func seedT42AuthorAndContent(t *testing.T, db *gorm.DB, status string) (authorID int64, contentID int64) {
	t.Helper()
	authorID = seedReviewUser(t, db)
	content := model.ContentItem{
		Title:       "T42 report content",
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      status,
		IsPublic:    true,
		ViewCount:   100,
	}
	require.NoError(t, db.Create(&content).Error)
	return authorID, content.ID
}

func TestReportAutoHideCreatesJudgeCaseAndNotifiesAuthor(t *testing.T) {
	socialSvc, db, producer, _ := setupT42ReportJudgeTest(t)
	authorID, contentID := seedT42AuthorAndContent(t, db, "published")

	// 5/100 = 0.05 ≥ ReportAutoHideRate，触发 auto-hide。
	for i := 0; i < 5; i++ {
		reporter := seedReviewUser(t, db)
		require.NoError(t, socialSvc.Report("content", contentID, reporter, "spam", ""))
	}

	var content model.ContentItem
	require.NoError(t, db.First(&content, contentID).Error)
	require.Equal(t, "under_review", content.Status, "举报达阈值应触发 auto-hide")

	var caseCount int64
	require.NoError(t, db.Model(&model.JudgeCase{}).
		Where("target_id = ? AND status = ?", contentID, "open").Count(&caseCount).Error)
	require.Equal(t, int64(1), caseCount, "auto-hide 必须创建判官案件触发众裁（FIX-11）——不能无限期滞留")

	notifies := producer.contentStatusNotifies()
	require.Len(t, notifies, 1, "auto-hide 应通知作者（T11 契约）")
	require.EqualValues(t, authorID, notifies[0]["user_id"])
	require.Contains(t, notifies[0]["body"], "人工复核")
	require.Contains(t, notifies[0]["body"], "举报")
}

func TestReportAutoHideDoesNotDowngradeBannedTerminalState(t *testing.T) {
	socialSvc, db, producer, _ := setupT42ReportJudgeTest(t)
	_, contentID := seedT42AuthorAndContent(t, db, "banned")

	for i := 0; i < 5; i++ {
		reporter := seedReviewUser(t, db)
		require.NoError(t, socialSvc.Report("content", contentID, reporter, "spam", ""))
	}

	var content model.ContentItem
	require.NoError(t, db.First(&content, contentID).Error)
	require.Equal(t, "banned", content.Status, "守卫：banned 终态不被举报打回 under_review")

	var caseCount int64
	require.NoError(t, db.Model(&model.JudgeCase{}).Where("target_id = ?", contentID).Count(&caseCount).Error)
	require.Equal(t, int64(0), caseCount, "banned 终态不建案（无众裁出口需求）")
	require.Len(t, producer.contentStatusNotifies(), 0, "状态未变不应通知")
}

func TestReportBelowThresholdKeepsPublishedWithoutCase(t *testing.T) {
	socialSvc, db, producer, _ := setupT42ReportJudgeTest(t)
	_, contentID := seedT42AuthorAndContent(t, db, "published")

	for i := 0; i < 2; i++ {
		reporter := seedReviewUser(t, db)
		require.NoError(t, socialSvc.Report("content", contentID, reporter, "spam", ""))
	}

	var content model.ContentItem
	require.NoError(t, db.First(&content, contentID).Error)
	require.Equal(t, "published", content.Status, "未达阈值不隐藏")

	var caseCount int64
	require.NoError(t, db.Model(&model.JudgeCase{}).Where("target_id = ?", contentID).Count(&caseCount).Error)
	require.Equal(t, int64(0), caseCount)
	require.Len(t, producer.contentStatusNotifies(), 0)
}
