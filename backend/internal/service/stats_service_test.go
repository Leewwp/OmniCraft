package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// FIX-02 (T02): the IP stats count must use the IP status vocabulary
// (pending/approved/rejected/banned) — "published" is a content status and
// made Active IPs permanently 0.
func TestStatsSummaryCountsApprovedIPsOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.IP{}, &model.ContentItem{}, &model.User{}))

	require.NoError(t, db.Create(&model.IP{Slug: "a1", Name: "approved-1", Status: "approved"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "a2", Name: "approved-2", Status: "approved"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "p1", Name: "pending", Status: "pending"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "r1", Name: "rejected", Status: "rejected"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "b1", Name: "banned", Status: "banned"}).Error)
	// contents keep the published vocabulary — must not affect the IP count
	require.NoError(t, db.Create(&model.ContentItem{Title: "c1", Status: "published"}).Error)

	svc := NewStatsService(db, nil)
	summary, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), summary.IPs, "only approved IPs count toward Active IPs")
	require.Equal(t, int64(1), summary.Contents, "content stats keep the published status word")
}

// #411 F3：分区统计——内容数按 zone 过滤、「创作者」改区内去重作者数、
// 缓存键按 zone 隔离、无 zone 请求维持全局语义。
func seedZoneStats(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.IP{}, &model.ContentItem{}, &model.User{}))

	require.NoError(t, db.Create(&model.IP{Slug: "ip1", Name: "approved", Status: "approved"}).Error)
	// 三个注册用户（全局口径的 users 应为 3）
	require.NoError(t, db.Create(&model.User{Email: "u1@t.local", Username: "u1", PasswordHash: "x"}).Error)
	require.NoError(t, db.Create(&model.User{Email: "u2@t.local", Username: "u2", PasswordHash: "x"}).Error)
	require.NoError(t, db.Create(&model.User{Email: "u3@t.local", Username: "u3", PasswordHash: "x"}).Error)

	mk := func(title, zone, status string, author int64, deleted bool) model.ContentItem {
		item := model.ContentItem{Title: title, Zone: zone, Status: status, AuthorID: author}
		if author == 0 {
			item.AuthorID = 0
		}
		if deleted {
			require.NoError(t, db.Create(&item).Error)
			require.NoError(t, db.Exec("UPDATE content_items SET deleted_at = ? WHERE title = ?", time.Now(), title).Error)
			return item
		}
		require.NoError(t, db.Create(&item).Error)
		return item
	}
	// fanwork：2 位作者 3 条发布 + 1 条软删 + 1 条 pending（都不应计入）
	mk("f1", "fanwork", "published", 1, false)
	mk("f2", "fanwork", "published", 1, false)
	mk("f3", "fanwork", "published", 2, false)
	mk("f-del", "fanwork", "published", 3, true)
	mk("f-pend", "fanwork", "pending", 3, false)
	// original：1 位作者 2 条发布
	mk("o1", "original", "published", 2, false)
	mk("o2", "original", "published", 2, false)
	return db
}

func TestStatsSummaryGlobalStaysCompatible(t *testing.T) {
	db := seedZoneStats(t)
	svc := NewStatsService(db, nil)
	summary, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.Users, "global users keeps the registered-user semantics")
	require.Equal(t, int64(1), summary.IPs)
	require.Equal(t, int64(5), summary.Contents, "global contents = all published non-deleted (3 fanwork + 2 original)")
}

func TestStatsSummaryZoneFanworkFiltersAndCountsDistinctAuthors(t *testing.T) {
	db := seedZoneStats(t)
	svc := NewStatsService(db, nil)
	summary, err := svc.GetSummaryForZone(context.Background(), "fanwork")
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.Contents, "zone contents exclude soft-deleted and non-published")
	require.Equal(t, int64(2), summary.Users, "zone creators = distinct published authors in the zone")
	require.Equal(t, int64(1), summary.IPs, "active IPs keep the global approved semantics")
}

func TestStatsSummaryZoneOriginal(t *testing.T) {
	db := seedZoneStats(t)
	svc := NewStatsService(db, nil)
	summary, err := svc.GetSummaryForZone(context.Background(), "original")
	require.NoError(t, err)
	require.Equal(t, int64(2), summary.Contents)
	require.Equal(t, int64(1), summary.Users, "both zones may legitimately share the same creator count")
}

func TestStatsCacheKeysAreZoneIsolated(t *testing.T) {
	db := seedZoneStats(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewStatsService(db, rdb)

	_, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	globalSummary, err := svc.GetSummaryForZone(context.Background(), "fanwork")
	require.NoError(t, err)
	require.Equal(t, int64(3), globalSummary.Contents, "fanwork summary must not be served from the global cache entry")

	require.Equal(t, "stats:summary", statsCacheKeyForZone(StatsZoneGlobal))
	require.Equal(t, "stats:summary:zone:fanwork", statsCacheKeyForZone(StatsZoneFanwork))
	require.Equal(t, "stats:summary:zone:original", statsCacheKeyForZone(StatsZoneOriginal))
	require.True(t, mr.Exists("stats:summary"))
	require.True(t, mr.Exists("stats:summary:zone:fanwork"))
	require.False(t, mr.Exists("stats:summary:zone:original"), "unqueried zone must not write a cache entry")

	// 全局缓存未被分区结果污染：独立读全局键应为全局数值。
	var cached StatsSummary
	raw, err := mr.Get("stats:summary")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(raw), &cached))
	require.Equal(t, int64(5), cached.Contents)
}

func TestValidStatsZoneVocabulary(t *testing.T) {
	require.True(t, ValidStatsZone(""))
	require.True(t, ValidStatsZone("original"))
	require.True(t, ValidStatsZone("fanwork"))
	require.False(t, ValidStatsZone("all"))
	require.False(t, ValidStatsZone("Fanwork"))
}
