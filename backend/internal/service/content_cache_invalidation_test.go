package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
)

// T10（FIX-38）service 层：举报 auto-hide 写点与共享失效 helper 的行为。

func TestInvalidateContentCachesNilRedisNoPanic(t *testing.T) {
	require.NotPanics(t, func() {
		InvalidateContentCaches(nil, 1)
	}, "本地栈无 redis（rdb=nil）时失效 helper 必须是 no-op，不得 panic")
}

func TestSocialReportAutoHideInvalidatesContentCache(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.Report{}))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	cfg := &config.Config{Social: config.SocialConfig{ReportAutoHideRate: 0.05}}
	author := model.User{Email: "t10-author@example.test", Username: "t10-author", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	content := model.ContentItem{
		Title:       "T10 auto-hide content",
		AuthorID:    author.ID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		ViewCount:   100,
	}
	require.NoError(t, db.Create(&content).Error)

	// 预热公共读路径的详情缓存（等价于 GetContent 对 published 内容的写入）。
	require.NoError(t, redisclient.SetJSON(context.Background(), "cache:content:"+strconv.FormatInt(content.ID, 10), content, 5*time.Minute))
	require.True(t, mr.Exists("cache:content:"+strconv.FormatInt(content.ID, 10)))

	socialSvc := NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		cfg, rdb, nil,
	)

	// 5/100 = 0.05 ≥ ReportAutoHideRate，触发 auto-hide。
	for i := 0; i < 5; i++ {
		reporter := model.User{
			Email:        "t10-reporter-" + strconv.FormatInt(int64(i), 10) + "@example.test",
			Username:     "t10-reporter-" + strconv.FormatInt(int64(i), 10),
			PasswordHash: "hash",
			Reputation:   10,
		}
		require.NoError(t, db.Create(&reporter).Error)
		require.NoError(t, socialSvc.Report("content", content.ID, reporter.ID, "spam", ""))
	}

	var updated model.ContentItem
	require.NoError(t, db.First(&updated, content.ID).Error)
	require.Equal(t, "under_review", updated.Status, "举报达阈值应触发 auto-hide")

	require.False(t, mr.Exists("cache:content:"+strconv.FormatInt(content.ID, 10)),
		"auto-hide 写点必须立即失效详情缓存（FIX-38），否则隐藏内容在 TTL 窗口内仍可读")
}

