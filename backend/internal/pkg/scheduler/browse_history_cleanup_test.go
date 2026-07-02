package scheduler

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
)

func TestBrowseHistoryCleanupNextRunUsesAsiaShanghai(t *testing.T) {
	cleanup := NewBrowseHistoryCleanup(nil, &config.BrowseHistoryConfig{RetentionDays: 7, CleanupTime: "03:00"})
	loc := mustShanghaiLocation(t)
	now := time.Date(2026, 7, 2, 2, 50, 0, 0, loc)

	next := cleanup.nextRun(now)

	want := time.Date(2026, 7, 2, 3, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextRun = %s, want %s", next, want)
	}
}

func TestBrowseHistoryCleanupSchedulesTomorrowWhenTodayTimePassed(t *testing.T) {
	cleanup := NewBrowseHistoryCleanup(nil, &config.BrowseHistoryConfig{RetentionDays: 7, CleanupTime: "03:00"})
	loc := mustShanghaiLocation(t)
	now := time.Date(2026, 7, 2, 3, 1, 0, 0, loc)

	next := cleanup.nextRun(now)

	want := time.Date(2026, 7, 3, 3, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("nextRun = %s, want %s", next, want)
	}
}

func TestBrowseHistoryCleanupDeletesExpiredRowsAndReschedules(t *testing.T) {
	db := setupBrowseHistoryCleanupDB(t)
	cfg := &config.BrowseHistoryConfig{RetentionDays: 7, CleanupTime: "03:00"}
	cleanup := NewBrowseHistoryCleanup(db, cfg)
	loc := mustShanghaiLocation(t)
	now := time.Date(2026, 7, 2, 3, 0, 0, 0, loc)
	cleanup.now = func() time.Time { return now }

	deleted, err := cleanup.runOnce()
	if err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	var count int64
	if err := db.Model(&model.BrowseHistory{}).Count(&count).Error; err != nil {
		t.Fatalf("count history: %v", err)
	}
	if count != 2 {
		t.Fatalf("remaining count = %d, want 2", count)
	}
	next := cleanup.nextRun(now)
	if !next.After(now) {
		t.Fatalf("next run = %s, want after %s", next, now)
	}
}

func TestBrowseHistoryCleanupInvalidConfigFallsBackAndLogs(t *testing.T) {
	cleanup := NewBrowseHistoryCleanup(nil, &config.BrowseHistoryConfig{RetentionDays: 0, CleanupTime: "not-a-time"})
	loc := mustShanghaiLocation(t)
	now := time.Date(2026, 7, 2, 2, 50, 0, 0, loc)

	next := cleanup.nextRun(now)

	want := time.Date(2026, 7, 2, 3, 0, 0, 0, loc)
	if cleanup.retentionDays() != 7 {
		t.Fatalf("retentionDays = %d, want fallback 7", cleanup.retentionDays())
	}
	if !next.Equal(want) {
		t.Fatalf("nextRun with invalid config = %s, want fallback %s", next, want)
	}
}

func TestBrowseHistoryCleanupStopPreventsNextCallback(t *testing.T) {
	cleanup := NewBrowseHistoryCleanup(nil, &config.BrowseHistoryConfig{RetentionDays: 7, CleanupTime: "03:00"})
	called := make(chan struct{}, 1)
	cleanup.scheduleAfter(10*time.Millisecond, func() {
		called <- struct{}{}
	})

	cleanup.Stop()
	time.Sleep(30 * time.Millisecond)

	select {
	case <-called:
		t.Fatal("cleanup callback ran after Stop")
	default:
	}
}

func setupBrowseHistoryCleanupDB(t *testing.T) *gorm.DB {
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
		t.Fatalf("migrate: %v", err)
	}
	author := model.User{ID: 1, Email: "cleanup-author@example.test", Username: "cleanup-author", PasswordHash: "hash"}
	viewer := model.User{ID: 2, Email: "cleanup-viewer@example.test", Username: "cleanup-viewer", PasswordHash: "hash"}
	if err := db.Create(&[]model.User{author, viewer}).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	for i := int64(1); i <= 3; i++ {
		content := model.ContentItem{
			ID:          i,
			Title:       "content",
			AuthorID:    author.ID,
			Zone:        "original",
			ContentType: "article",
			Status:      "published",
		}
		if err := db.Create(&content).Error; err != nil {
			t.Fatalf("create content: %v", err)
		}
	}
	now := time.Date(2026, 7, 2, 3, 0, 0, 0, mustShanghaiLocation(t))
	rows := []model.BrowseHistory{
		{ID: 1, UserID: viewer.ID, ContentItemID: 1, ViewedAt: now.Add(-6 * 24 * time.Hour)},
		{ID: 2, UserID: viewer.ID, ContentItemID: 2, ViewedAt: now.Add(-7 * 24 * time.Hour)},
		{ID: 3, UserID: viewer.ID, ContentItemID: 3, ViewedAt: now.Add(-7*24*time.Hour - time.Nanosecond)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create history rows: %v", err)
	}
	return db
}

func mustShanghaiLocation(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load Asia/Shanghai: %v", err)
	}
	return loc
}
