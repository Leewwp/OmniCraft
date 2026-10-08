package service

import (
	"strconv"
	"strings"
	"testing"

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

// #846 三路径一致性（service 层）：详情缓存（cache:content:{id}）与列表
// 缓存（cache:content:list:*）存的是 model 序列化 JSON，hot 路径 rank ZSET
// 只存 ID、载荷每次经 BatchGetByIDs 回源——三条路径返回的 ip 摘要必须同一
// 形状 {id, name, cover_url}。命中一致性的证明方式：首读（未命中回源）后
// 删掉 DB 的 ips 行，复读仍带 ip ⇒ 值来自缓存且与回源行一致。

func setupIPSummaryCacheService(t *testing.T) (*ContentService, *gorm.DB, *miniredis.Miniredis, *model.IP, *model.ContentItem) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	previous := redisclient.Client
	redisclient.Client = rdb
	t.Cleanup(func() { redisclient.Client = previous })

	author := model.User{Email: "a846-cache-author@example.test", Username: "a846-cache-author", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	ip := model.IP{
		Name:     "关联 IP 缓存一致性",
		Slug:     "a846-cache-ip",
		CoverURL: "https://oss.example.com/ips/a846-cache-cover.jpg",
		Status:   "approved",
	}
	require.NoError(t, db.Create(&ip).Error)
	fanwork := model.ContentItem{
		Title:       "a846 cache fanwork",
		AuthorID:    author.ID,
		Zone:        "fanwork",
		IPID:        &ip.ID,
		ContentType: "image",
		Status:      "published",
		IsPublic:    true,
	}
	require.NoError(t, db.Create(&fanwork).Error)

	svc := NewContentServiceWithCache(
		repository.NewContentRepository(db),
		nil,
		rdb,
		&config.CacheConfig{ContentListTTL: 300, ContentDetailTTL: 300},
	)
	return svc, db, mr, &ip, &fanwork
}

func requireCachedIPSummary(t *testing.T, got *model.IP, want *model.IP) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.ID, got.ID)
	require.Equal(t, want.Name, got.Name)
	require.Equal(t, want.CoverURL, got.CoverURL)
	require.Empty(t, got.Slug)
	require.Empty(t, got.Status)
}

func TestGetContentIPSummaryConsistentBetweenCacheMissAndHit(t *testing.T) {
	svc, db, mr, ip, fanwork := setupIPSummaryCacheService(t)

	// 未命中：FindByID 回源挂载摘要并写入详情缓存。
	first, err := svc.GetContent(fanwork.ID)
	require.NoError(t, err)
	requireCachedIPSummary(t, first.IP, ip)

	detailKey := "cache:content:" + strconv.FormatInt(fanwork.ID, 10)
	require.True(t, mr.Exists(detailKey), "published 公开内容首读后必须落详情缓存")
	raw, err := mr.Get(detailKey)
	require.NoError(t, err)
	require.True(t, strings.Contains(raw, `"ip"`), "缓存 JSON 必须携带 ip 字段（缓存写的是序列化 model）")

	// 删掉 DB 的 IP 行后复读仍带同一摘要 ⇒ 值来自缓存，命中与未命中形状一致。
	require.NoError(t, db.Where("id = ?", ip.ID).Delete(&model.IP{}).Error)
	second, err := svc.GetContent(fanwork.ID)
	require.NoError(t, err)
	requireCachedIPSummary(t, second.IP, ip)
	require.Equal(t, first.IP, second.IP)
}

func TestListContentsIPSummaryConsistentBetweenCacheMissAndHit(t *testing.T) {
	svc, db, mr, ip, _ := setupIPSummaryCacheService(t)

	filter := repository.ListContentsFilter{Zone: "fanwork", Sort: "newest", TimeRange: "all", Page: 1, PageSize: 20}

	// 未命中：repo.ListContents 挂载摘要后写列表缓存。
	first, total, err := svc.ListContents(filter, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, first, 1)
	requireCachedIPSummary(t, first[0].IP, ip)

	listKeys := mr.Keys()
	foundListKey := false
	for _, key := range listKeys {
		if strings.HasPrefix(key, "cache:content:list:") {
			foundListKey = true
			raw, err := mr.Get(key)
			require.NoError(t, err)
			require.True(t, strings.Contains(raw, `"ip"`), "列表缓存 JSON 必须携带 ip 字段")
		}
	}
	require.True(t, foundListKey, "首读后必须落列表缓存键")

	// 删掉 DB 的 IP 行后同条件复读仍带同一摘要 ⇒ 命中路径与回源路径一致。
	require.NoError(t, db.Where("id = ?", ip.ID).Delete(&model.IP{}).Error)
	second, _, err := svc.ListContents(filter, 0)
	require.NoError(t, err)
	require.Len(t, second, 1)
	requireCachedIPSummary(t, second[0].IP, ip)
	require.Equal(t, first[0].IP, second[0].IP)
}

func TestHotContentsIPSummaryHydrated(t *testing.T) {
	svc, _, mr, ip, fanwork := setupIPSummaryCacheService(t)

	// hot 路径：rank ZSET 只存内容 ID，载荷每次经 BatchGetByIDs 回源挂载。
	added, err := mr.ZAdd("rank:hot:contents", 1.0, strconv.FormatInt(fanwork.ID, 10))
	require.NoError(t, err)
	require.True(t, added)

	contents, total, err := svc.ListContents(repository.ListContentsFilter{
		Zone:      "fanwork",
		Sort:      "hot",
		TimeRange: "all",
		Page:      1,
		PageSize:  1,
	}, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, contents, 1)
	requireCachedIPSummary(t, contents[0].IP, ip)

	// hot 路径不落载荷缓存：返回值直接来自回源 hydrate，断言列表缓存键未新增。
	for _, key := range mr.Keys() {
		require.False(t, strings.HasPrefix(key, "cache:content:list:"), "hot 走 rank 回源，不应写列表缓存键")
	}
}
