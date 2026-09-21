package repository

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
)

// FR-03（中-7）：GetFollowerStats 的 SQL 不得再引用不存在的
// follows.deleted_at 列（Unfollow 走硬删，无该列）——dry-run 捕获断言文本形态。
func TestGetFollowerStatsSQLHasNoDeletedAtPredicate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Follow{}, &model.User{}, &model.IP{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var captured []string
	session := db.Session(&gorm.Session{DryRun: true, Logger: logger.New(
		&captureWriter{lines: &captured}, logger.Config{LogLevel: logger.Info},
	)})
	repo := NewFollowRepository(session)

	// dry-run 下 PG 方言的 daily 查询在 sqlite 构建期不执行、错误在 .Error
	// 检查处返回——此处只关心捕获到的 SQL 文本形态。
	_, _ = repo.GetFollowerStats(1, 30)

	joined := strings.Join(captured, "\n")
	if strings.Contains(joined, "deleted_at") {
		t.Fatalf("follower stats SQL must not reference the non-existent follows.deleted_at column:\n%s", joined)
	}
}

// FR-03（中-7）：查询错误必须传播。sqlite 没有 generate_series/NOW()，
// daily 原生 SQL 必然失败——修前吞错返回 (stats, nil)，修后必须返回 error。
func TestGetFollowerStatsPropagatesQueryErrors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Follow{}, &model.User{}, &model.IP{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.Follow{FollowerID: 2, TargetType: "user", TargetID: 1}).Error; err != nil {
		t.Fatalf("seed follow: %v", err)
	}

	repo := NewFollowRepository(db)
	if _, err := repo.GetFollowerStats(1, 30); err == nil {
		t.Fatal("query failures inside GetFollowerStats must propagate, not be swallowed")
	}
}
