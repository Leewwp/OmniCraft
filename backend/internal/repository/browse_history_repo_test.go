package repository

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

func TestBrowseHistoryListAppliesRetentionWindow(t *testing.T) {
	db := setupBrowseHistoryRepoDB(t)
	repo := NewBrowseHistoryRepository(db)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	author := seedBrowseHistoryUser(t, db, 10, "author-retention")
	viewer := seedBrowseHistoryUser(t, db, 20, "viewer-retention")
	recent := seedBrowseHistoryContent(t, db, 101, author.ID, "recent", "article", "published", nil)
	old := seedBrowseHistoryContent(t, db, 102, author.ID, "old", "article", "published", nil)
	seedBrowseHistoryRow(t, db, 1, viewer.ID, recent.ID, now.Add(-6*24*time.Hour))
	seedBrowseHistoryRow(t, db, 2, viewer.ID, old.ID, now.Add(-8*24*time.Hour))

	items, total, err := repo.ListByUserFiltered(BrowseHistoryListOptions{
		UserID:        viewer.ID,
		RetentionDays: 7,
		Now:           now,
		Page:          1,
		PageSize:      20,
	})

	if err != nil {
		t.Fatalf("ListByUserFiltered() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1; items=%#v", total, len(items), items)
	}
	if items[0].ID != 1 || items[0].Content == nil || items[0].Content.ID != recent.ID {
		t.Fatalf("item = %#v, want retained recent content", items[0])
	}
}

func TestBrowseHistoryListFiltersByContentTypeAndDateRange(t *testing.T) {
	db := setupBrowseHistoryRepoDB(t)
	repo := NewBrowseHistoryRepository(db)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	author := seedBrowseHistoryUser(t, db, 10, "author-filter")
	viewer := seedBrowseHistoryUser(t, db, 20, "viewer-filter")
	article := seedBrowseHistoryContent(t, db, 101, author.ID, "article", "article", "published", nil)
	video := seedBrowseHistoryContent(t, db, 102, author.ID, "video", "video", "published", nil)
	outside := seedBrowseHistoryContent(t, db, 103, author.ID, "outside", "article", "published", nil)
	seedBrowseHistoryRow(t, db, 1, viewer.ID, article.ID, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC))
	seedBrowseHistoryRow(t, db, 2, viewer.ID, video.ID, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC))
	seedBrowseHistoryRow(t, db, 3, viewer.ID, outside.ID, time.Date(2026, 7, 2, 1, 0, 0, 0, time.UTC))

	items, total, err := repo.ListByUserFiltered(BrowseHistoryListOptions{
		UserID:        viewer.ID,
		ContentType:   "article",
		StartDate:     &start,
		EndDate:       &end,
		RetentionDays: 7,
		Now:           now,
		Page:          1,
		PageSize:      20,
	})

	if err != nil {
		t.Fatalf("ListByUserFiltered() error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Content == nil || items[0].Content.ID != article.ID {
		t.Fatalf("filtered result total=%d items=%#v, want only article within date range", total, items)
	}
}

func TestBrowseHistoryListReturnsNullContentForUnavailableContent(t *testing.T) {
	db := setupBrowseHistoryRepoDB(t)
	repo := NewBrowseHistoryRepository(db)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	deletedAt := now.Add(-time.Hour)
	author := seedBrowseHistoryUser(t, db, 10, "author-unavailable")
	viewer := seedBrowseHistoryUser(t, db, 20, "viewer-unavailable")
	published := seedBrowseHistoryContent(t, db, 101, author.ID, "published", "article", "published", nil)
	underReview := seedBrowseHistoryContent(t, db, 102, author.ID, "under-review", "article", "under_review", nil)
	deleted := seedBrowseHistoryContent(t, db, 103, author.ID, "deleted", "article", "published", &deletedAt)
	seedBrowseHistoryRow(t, db, 1, viewer.ID, published.ID, now.Add(-time.Hour))
	seedBrowseHistoryRow(t, db, 2, viewer.ID, underReview.ID, now.Add(-2*time.Hour))
	seedBrowseHistoryRow(t, db, 3, viewer.ID, deleted.ID, now.Add(-3*time.Hour))

	items, total, err := repo.ListByUserFiltered(BrowseHistoryListOptions{
		UserID:        viewer.ID,
		RetentionDays: 7,
		Now:           now,
		Page:          1,
		PageSize:      20,
	})

	if err != nil {
		t.Fatalf("ListByUserFiltered() error = %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("total/items = %d/%d, want 3/3", total, len(items))
	}
	if items[0].Content == nil || items[0].ContentItem == nil || items[0].Content.ID != published.ID {
		t.Fatalf("published item = %#v, want content aliases", items[0])
	}
	for _, item := range items[1:] {
		if item.Content != nil || item.ContentItem != nil {
			t.Fatalf("unavailable item = %#v, want nil content aliases", item)
		}
	}
}

func TestBrowseHistoryListCountMatchesFilteredRows(t *testing.T) {
	db := setupBrowseHistoryRepoDB(t)
	repo := NewBrowseHistoryRepository(db)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	author := seedBrowseHistoryUser(t, db, 10, "author-count")
	viewer := seedBrowseHistoryUser(t, db, 20, "viewer-count")
	for i := int64(1); i <= 3; i++ {
		content := seedBrowseHistoryContent(t, db, 100+i, author.ID, "article", "article", "published", nil)
		seedBrowseHistoryRow(t, db, i, viewer.ID, content.ID, now.Add(-time.Duration(i)*time.Hour))
	}
	video := seedBrowseHistoryContent(t, db, 200, author.ID, "video", "video", "published", nil)
	seedBrowseHistoryRow(t, db, 10, viewer.ID, video.ID, now.Add(-time.Hour))

	items, total, err := repo.ListByUserFiltered(BrowseHistoryListOptions{
		UserID:        viewer.ID,
		ContentType:   "article",
		RetentionDays: 7,
		Now:           now,
		Page:          1,
		PageSize:      2,
	})

	if err != nil {
		t.Fatalf("ListByUserFiltered() error = %v", err)
	}
	if total != 3 || len(items) != 2 {
		t.Fatalf("total/page items = %d/%d, want filtered total 3 and page size 2", total, len(items))
	}
}

func TestBrowseHistoryDeleteByIDsScopesToCurrentUserAndLimit(t *testing.T) {
	db := setupBrowseHistoryRepoDB(t)
	repo := NewBrowseHistoryRepository(db)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	author := seedBrowseHistoryUser(t, db, 10, "author-delete")
	viewer := seedBrowseHistoryUser(t, db, 20, "viewer-delete")
	otherViewer := seedBrowseHistoryUser(t, db, 30, "other-viewer-delete")
	contentA := seedBrowseHistoryContent(t, db, 101, author.ID, "a", "article", "published", nil)
	contentB := seedBrowseHistoryContent(t, db, 102, author.ID, "b", "article", "published", nil)
	contentC := seedBrowseHistoryContent(t, db, 103, author.ID, "c", "article", "published", nil)
	seedBrowseHistoryRow(t, db, 1, viewer.ID, contentA.ID, now.Add(-time.Hour))
	seedBrowseHistoryRow(t, db, 2, viewer.ID, contentB.ID, now.Add(-time.Hour))
	seedBrowseHistoryRow(t, db, 3, otherViewer.ID, contentC.ID, now.Add(-time.Hour))

	if err := repo.DeleteByUserAndIDs(viewer.ID, []int64{1, 3}); err != nil {
		t.Fatalf("DeleteByUserAndIDs() error = %v", err)
	}

	var remaining []model.BrowseHistory
	if err := db.Order("id").Find(&remaining).Error; err != nil {
		t.Fatalf("load remaining: %v", err)
	}
	got := make([]int64, 0, len(remaining))
	for _, row := range remaining {
		got = append(got, row.ID)
	}
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("remaining ids = %v, want [2 3]", got)
	}
	if err := repo.DeleteByUserAndIDs(viewer.ID, nil); err != nil {
		t.Fatalf("DeleteByUserAndIDs(nil) error = %v", err)
	}
	var count int64
	if err := db.Model(&model.BrowseHistory{}).Count(&count).Error; err != nil {
		t.Fatalf("count remaining: %v", err)
	}
	if count != 2 {
		t.Fatalf("count after nil ids = %d, want 2", count)
	}
}

func TestBrowseHistoryDeleteExpiredUsesRetentionDays(t *testing.T) {
	db := setupBrowseHistoryRepoDB(t)
	repo := NewBrowseHistoryRepository(db)
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	author := seedBrowseHistoryUser(t, db, 10, "author-expired")
	viewer := seedBrowseHistoryUser(t, db, 20, "viewer-expired")
	recent := seedBrowseHistoryContent(t, db, 101, author.ID, "recent", "article", "published", nil)
	boundary := seedBrowseHistoryContent(t, db, 102, author.ID, "boundary", "article", "published", nil)
	expired := seedBrowseHistoryContent(t, db, 103, author.ID, "expired", "article", "published", nil)
	seedBrowseHistoryRow(t, db, 1, viewer.ID, recent.ID, now.Add(-6*24*time.Hour))
	seedBrowseHistoryRow(t, db, 2, viewer.ID, boundary.ID, now.Add(-7*24*time.Hour))
	seedBrowseHistoryRow(t, db, 3, viewer.ID, expired.ID, now.Add(-7*24*time.Hour-time.Nanosecond))

	deleted, err := repo.DeleteExpired(7, now)
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	var remaining []model.BrowseHistory
	if err := db.Order("id").Find(&remaining).Error; err != nil {
		t.Fatalf("load remaining: %v", err)
	}
	if len(remaining) != 2 || remaining[0].ID != 1 || remaining[1].ID != 2 {
		t.Fatalf("remaining = %#v, want ids 1 and 2", remaining)
	}
}

func setupBrowseHistoryRepoDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.BrowseHistory{}); err != nil {
		t.Fatalf("migrate browse history: %v", err)
	}
	return db
}

func seedBrowseHistoryUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	user := model.User{
		ID:           id,
		Email:        username + "@example.test",
		Username:     username,
		PasswordHash: "hash",
		Role:         "user",
		Reputation:   10,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func seedBrowseHistoryContent(t *testing.T, db *gorm.DB, id int64, authorID int64, title, contentType, status string, deletedAt *time.Time) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       title,
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: contentType,
		Status:      status,
		IsPublic:    true,
		AllowCopy:   true,
		DeletedAt:   deletedAt,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content %s: %v", title, err)
	}
	return content
}

func seedBrowseHistoryRow(t *testing.T, db *gorm.DB, id, userID, contentItemID int64, viewedAt time.Time) {
	t.Helper()
	row := model.BrowseHistory{
		ID:            id,
		UserID:        userID,
		ContentItemID: contentItemID,
		ViewedAt:      viewedAt,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create history row %d: %v", id, err)
	}
}
