package repository

import (
	"testing"

	"omnicraft/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// #668：主 feed handler 迁 pageQuery 后回显有效参数，仓储层既有归一防线
// （content_repo.go：page<1→1、page_size<1 或 >100→20）由本测试锁死——
// handler 与仓储双层各自独立成立，任何一层漂移都会在此失败。

func setupPaginationLockDB(t *testing.T) *ContentRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rows := make([]model.ContentItem, 0, 45)
	for i := 0; i < 45; i++ {
		rows = append(rows, model.ContentItem{
			AuthorID: 1,
			Status:   "published",
			Title:    "锁定项",
			Zone:     "original",
		})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	return NewContentRepository(db)
}

func TestListContentsRepoNormalizesPaginationDefense(t *testing.T) {
	repo := setupPaginationLockDB(t)

	cases := []struct {
		name               string
		page, pageSize     int
		wantPage, wantSize int
	}{
		{"zero page becomes 1", 0, 20, 1, 20},
		{"negative page becomes 1", -3, 20, 1, 20},
		{"zero size falls back to 20", 1, 0, 1, 20},
		{"oversize size falls back to 20", 2, 100000, 2, 20},
		{"in-range passes", 2, 20, 2, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, total, err := repo.ListContents(ListContentsFilter{
				Status:   "published",
				Page:     tc.page,
				PageSize: tc.pageSize,
			})
			if err != nil {
				t.Fatalf("ListContents: %v", err)
			}
			if total != 45 {
				t.Fatalf("total = %d, want 45", total)
			}
			wantRows := tc.wantSize
			if tc.wantPage > 1 && 45-(tc.wantPage-1)*tc.wantSize < wantRows {
				wantRows = 45 - (tc.wantPage-1)*tc.wantSize
			}
			if len(items) != wantRows {
				t.Fatalf("repo returned %d rows for page=%d size=%d, want %d (normalized to page=%d size=%d)",
					len(items), tc.page, tc.pageSize, wantRows, tc.wantPage, tc.wantSize)
			}
		})
	}
}
