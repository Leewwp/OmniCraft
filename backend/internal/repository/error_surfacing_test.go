package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// SP-25 FR-10（低-27）：repo 层吞错家族修复的故障注入验证——DB 故障必须
// 显性报错，不得以零值/空结果伪装成功（伪 total、伪 hasContent 放行删除）。

func brokenDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return db
}

func TestGetFollowersSurfacesDBFailure(t *testing.T) {
	repo := NewFollowRepository(brokenDB(t))
	_, _, err := repo.GetFollowers("user", 1, 1, 20)
	if err == nil {
		t.Fatal("GetFollowers must surface DB failure instead of returning a fake empty page")
	}
}

func TestGetVoteStatsSurfacesDBFailure(t *testing.T) {
	repo := NewJudgeRepository(brokenDB(t))
	_, _, err := repo.GetVoteStats(1)
	if err == nil {
		t.Fatal("GetVoteStats must surface DB failure instead of zero-vote pseudo stats")
	}
}

func TestHasLinkedContentSurfacesDBFailure(t *testing.T) {
	repo := NewCategoryRepository(brokenDB(t))
	// 删除守卫 fail-closed：查询故障返回错误（服务层据此拒绝删除）。
	_, err := repo.HasLinkedContent(1)
	if err == nil {
		t.Fatal("HasLinkedContent must surface DB failure; a swallowed error would let category deletion proceed")
	}
}
