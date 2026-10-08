package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/testutil"
)

func setupIPStatsServiceTest(t *testing.T) (*IPStatsService, *gorm.DB, *miniredis.Miniredis) {
	t.Helper()

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "005_ips.sql"))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewIPStatsService(db, rdb), db, mr
}

func seedStatsIP(t *testing.T, db *gorm.DB, name string, category any, status string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO ips (name, slug, category, status) VALUES (?, ?, ?, ?)`,
		name, name, category, status,
	).Error)
}

// #843：UpdateCategoryCounts 口径 = ips 表 status='approved' 按分类计数，
// 不再 JOIN content_items。本测试库不建 content_items 表——若 SQL 仍 JOIN
// 则直接报错，借以锁定纯 IP 口径。NULL 类目按既有 COALESCE 语义归并
// uncategorized（保留原行为）。
func TestIPStatsUpdateCategoryCountsApprovedOnly(t *testing.T) {
	svc, db, _ := setupIPStatsServiceTest(t)

	seedStatsIP(t, db, "ip-game-1", "game", "approved")
	seedStatsIP(t, db, "ip-game-2", "game", "approved")
	seedStatsIP(t, db, "ip-anime-1", "anime", "approved")
	seedStatsIP(t, db, "ip-nocat-1", nil, "approved")
	// 非 approved 状态一律不计入
	seedStatsIP(t, db, "ip-game-pending", "game", "pending")
	seedStatsIP(t, db, "ip-anime-rejected", "anime", "rejected")
	seedStatsIP(t, db, "ip-game-banned", "game", "banned")

	require.NoError(t, svc.UpdateCategoryCounts(context.Background()))

	counts, err := svc.GetCategoryCounts(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"game":          "2",
		"anime":         "1",
		"uncategorized": "1",
	}, counts)
}

// 周期重建是 Del+HSet 全量覆盖：旧哈希中的过期分类在重建后被清除，
// 不会残留（hot_rank 重建机制不变的前提语义）。
func TestIPStatsUpdateCategoryCountsReplacesStaleHash(t *testing.T) {
	svc, db, mr := setupIPStatsServiceTest(t)

	seedStatsIP(t, db, "ip-music-1", "music", "approved")
	mr.HSet("ip:category:counts", "stale", "999")

	require.NoError(t, svc.UpdateCategoryCounts(context.Background()))

	counts, err := svc.GetCategoryCounts(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]string{"music": "1"}, counts)
}
