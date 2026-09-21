package service

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

type flushCaptureWriter struct{ lines *[]string }

func (w *flushCaptureWriter) Printf(format string, args ...interface{}) {
	*w.lines = append(*w.lines, fmt.Sprintf(format, args...))
}

func setupFlushService(t *testing.T) (*ContentService, *redis.Client, *miniredis.Miniredis, *gorm.DB) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	db, err := gorm.Open(sqlite.Open("file:flushtest?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ContentItem{}))
	repo := repository.NewContentRepository(db)
	svc := NewContentServiceWithDeps(repo, nil, rdb)
	return svc, rdb, mr, db
}

func seedDownloadZSet(t *testing.T, rdb *redis.Client) {
	t.Helper()
	require.NoError(t, rdb.ZAdd(context.Background(), "rank:download:counts", redis.Z{Score: 2, Member: "7"}).Err())
	require.NoError(t, rdb.ZAdd(context.Background(), "rank:download:counts", redis.Z{Score: 1, Member: "9"}).Err())
}

// FR-03（中-12）硬条件：生成的 UPDATE 语句必须把 CASE 表达式直接赋给
// download_count。历史 bug：caseStmt 自带 "download_count = " 前缀 →
// SET download_count = download_count = CASE ...（PG SQLSTATE 42804 每次
// flush 必败）；sqlite 宽松类型测不出，故以 dry-run 捕获 SQL 断言形态。
func TestFlushDownloadCountsGeneratesDirectCaseAssignment(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	seedDownloadZSet(t, rdb)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ContentItem{}))

	var captured []string
	session := db.Session(&gorm.Session{DryRun: true, Logger: logger.New(
		&flushCaptureWriter{lines: &captured}, logger.Config{LogLevel: logger.Info},
	)})
	repo := repository.NewContentRepository(session)
	svc := NewContentServiceWithDeps(repo, nil, rdb)
	require.NoError(t, svc.FlushDownloadCounts(context.Background()))

	joined := ""
	for _, line := range captured {
		joined += line + "\n"
	}
	doubleAssign := regexp.MustCompile("`?download_count`?=\\s*download_count\\s*=")
	directAssign := regexp.MustCompile("`?download_count`?=\\s*CASE\\s+`?id`?\\s+WHEN")
	if doubleAssign.MatchString(joined) {
		t.Fatalf("generated SQL assigns a boolean comparison to download_count (PG 42804):\n%s", joined)
	}
	if !directAssign.MatchString(joined) {
		t.Fatalf("generated SQL must assign the CASE expression directly:\n%s", joined)
	}
}

// FR-03（中-12）数据丢失面：DB 更新失败时 Redis 增量必须保留待下轮重放。
// 修前 ZRem pipeline 先于 DB 执行 → 失败的 flush 已把增量清掉 = 永久丢失。
func TestFlushDownloadCountsKeepsRedisIncrementsOnDBFailure(t *testing.T) {
	svc, rdb, mr, db := setupFlushService(t)
	defer mr.Close()
	seedDownloadZSet(t, rdb)

	// 注入 DB 故障：content_items 表消失 → UPDATE 必败。
	require.NoError(t, db.Migrator().DropTable("content_items"))

	if err := svc.FlushDownloadCounts(context.Background()); err == nil {
		t.Fatal("flush must fail when the DB update fails")
	}

	members, err := rdb.ZRangeWithScores(context.Background(), "rank:download:counts", 0, -1).Result()
	require.NoError(t, err)
	if len(members) != 2 {
		t.Fatalf("failed flush must keep redis increments for replay, got %d members: %#v", len(members), members)
	}
}

// FR-03（中-12）成功路径：DB 落库 + 增量清空。
func TestFlushDownloadCountsPersistsAndClearsOnSuccess(t *testing.T) {
	svc, rdb, mr, db := setupFlushService(t)
	defer mr.Close()
	seedDownloadZSet(t, rdb)

	for _, id := range []int64{7, 9} {
		require.NoError(t, db.Create(&model.ContentItem{
			ID: id, Title: "t", AuthorID: 1, Zone: "fanwork",
			ContentType: "text", Status: "published", DownloadCount: 10,
		}).Error)
	}

	require.NoError(t, svc.FlushDownloadCounts(context.Background()))

	var c7, c9 int64
	require.NoError(t, db.Table("content_items").Where("id = ?", 7).Pluck("download_count", &c7).Error)
	require.NoError(t, db.Table("content_items").Where("id = ?", 9).Pluck("download_count", &c9).Error)
	if c7 != 12 || c9 != 11 {
		t.Fatalf("download_count must persist deltas: id7=%d (want 12), id9=%d (want 11)", c7, c9)
	}
	members, err := rdb.ZRangeWithScores(context.Background(), "rank:download:counts", 0, -1).Result()
	require.NoError(t, err)
	if len(members) != 0 {
		t.Fatalf("successful flush must clear redis increments, got %#v", members)
	}
}
