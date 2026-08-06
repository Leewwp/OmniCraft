package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// GAP #17 后端只读扩展契约：GET /api/v1/contents?sort=recommended 在未传 zone
// （/recommend 推荐页）与 zone=original（/original 推荐排序）时都必须走
// handleRecommended 推荐管线（个性化引擎或 hot 兜底），不得落到仓库层
// created_at 默认排序。

func TestListContentsRecommendedSortRoutesToRecommendedPipeline(t *testing.T) {
	db := setupRecommendedSortTestDB(t)
	repo := repository.NewContentRepository(db)
	svc := NewContentService(repo)

	// A 最新创建但热度最低；B 最旧但热度中等；C 为二创、热度最高。
	now := time.Now()
	a := seedRecommendedSortContent(t, db, "A-original-hot1", "original", 1, now.Add(2*time.Minute))
	b := seedRecommendedSortContent(t, db, "B-original-hot50", "original", 50, now)
	c := seedRecommendedSortContent(t, db, "C-fanwork-hot100", "fanwork", 100, now.Add(1*time.Minute))

	t.Run("cross-zone no zone filter routes to recommended pipeline", func(t *testing.T) {
		got, total, err := svc.ListContents(repository.ListContentsFilter{
			Sort:     "recommended",
			Zone:     "",
			Page:     1,
			PageSize: 20,
		}, 0)
		if err != nil {
			t.Fatalf("ListContents() error = %v", err)
		}
		if total != 3 {
			t.Fatalf("total = %d, want 3", total)
		}
		want := []int64{c.ID, b.ID, a.ID} // hot 降序：二创 C 参与混排
		gotIDs := make([]int64, len(got))
		for i, item := range got {
			gotIDs[i] = item.ID
		}
		assertInt64SliceEqual(t, gotIDs, want)
	})

	t.Run("zone original keeps existing recommended sort contract", func(t *testing.T) {
		got, total, err := svc.ListContents(repository.ListContentsFilter{
			Sort:     "recommended",
			Zone:     "original",
			Page:     1,
			PageSize: 20,
		}, 0)
		if err != nil {
			t.Fatalf("ListContents() error = %v", err)
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2", total)
		}
		want := []int64{b.ID, a.ID}
		gotIDs := make([]int64, len(got))
		for i, item := range got {
			gotIDs[i] = item.ID
		}
		assertInt64SliceEqual(t, gotIDs, want)
	})
}

func setupRecommendedSortTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	// 生产 schema 有 hot_score（迁移维护），测试内存库手动补齐该列。
	if err := db.Exec("ALTER TABLE content_items ADD COLUMN hot_score REAL DEFAULT 0").Error; err != nil {
		t.Fatalf("add hot_score column: %v", err)
	}
	return db
}

func seedRecommendedSortContent(t *testing.T, db *gorm.DB, title, zone string, hotScore float64, createdAt time.Time) model.ContentItem {
	t.Helper()

	author := model.User{
		Email:        title + "@example.com",
		Username:     title,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	item := model.ContentItem{
		Title:       title,
		AuthorID:    author.ID,
		Zone:        zone,
		Category:    "game",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
		CreatedAt:   createdAt,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	if err := db.Model(&model.ContentItem{}).Where("id = ?", item.ID).Update("hot_score", hotScore).Error; err != nil {
		t.Fatalf("set hot_score: %v", err)
	}
	return item
}

func assertInt64SliceEqual(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v", got, want)
		}
	}
}
