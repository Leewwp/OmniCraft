package scheduler

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
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

func TestBrowseHistoryCleanupOnlyOneReplicaDeletesExpiredRows(t *testing.T) {
	db := setupBrowseHistoryCleanupPostgresDB(t)
	blocked := make(chan struct{})
	release := make(chan struct{})
	var blockFirst sync.Once
	if err := db.Callback().Delete().Before("gorm:delete").Register("test:block_first_history_cleanup", func(tx *gorm.DB) {
		if tx.Statement.Table != "browse_history" {
			return
		}
		blockFirst.Do(func() {
			close(blocked)
			<-release
		})
	}); err != nil {
		t.Fatalf("register delete callback: %v", err)
	}

	now := time.Date(2026, 7, 2, 3, 0, 0, 0, mustShanghaiLocation(t))
	first := NewBrowseHistoryCleanup(db, &config.BrowseHistoryConfig{RetentionDays: 7, CleanupTime: "03:00"})
	second := NewBrowseHistoryCleanup(db, &config.BrowseHistoryConfig{RetentionDays: 7, CleanupTime: "03:00"})
	first.now = func() time.Time { return now }
	second.now = func() time.Time { return now }

	type outcome struct {
		result browseHistoryCleanupRunResult
		err    error
	}
	firstDone := make(chan outcome, 1)
	go func() {
		result, err := first.runOnceWithStatus()
		firstDone <- outcome{result: result, err: err}
	}()

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("first cleanup did not reach the delete while holding the leader lock")
	}

	secondResult, err := second.runOnceWithStatus()
	if err != nil {
		t.Fatalf("second cleanup returned error: %v", err)
	}
	if secondResult.AcquiredLeader || secondResult.Deleted != 0 {
		t.Fatalf("second cleanup result = %#v, want skipped without deletion", secondResult)
	}

	close(release)
	select {
	case outcome := <-firstDone:
		if outcome.err != nil {
			t.Fatalf("first cleanup returned error: %v", outcome.err)
		}
		if !outcome.result.AcquiredLeader || outcome.result.Deleted != 1 {
			t.Fatalf("first cleanup result = %#v, want leader deleting one row", outcome.result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first cleanup did not finish after release")
	}

	afterCommit, err := second.runOnceWithStatus()
	if err != nil {
		t.Fatalf("cleanup after commit returned error: %v", err)
	}
	if !afterCommit.AcquiredLeader {
		t.Fatalf("cleanup after commit result = %#v, want released lock", afterCommit)
	}
}

func TestBrowseHistoryCleanupReleasesLeaderLockAfterFailure(t *testing.T) {
	db := setupBrowseHistoryCleanupPostgresDB(t)
	wantErr := errors.New("forced cleanup failure")
	var failFirst sync.Once
	if err := db.Callback().Delete().Before("gorm:delete").Register("test:fail_first_history_cleanup", func(tx *gorm.DB) {
		if tx.Statement.Table == "browse_history" {
			failFirst.Do(func() { tx.AddError(wantErr) })
		}
	}); err != nil {
		t.Fatalf("register delete callback: %v", err)
	}

	now := time.Date(2026, 7, 2, 3, 0, 0, 0, mustShanghaiLocation(t))
	cleanup := NewBrowseHistoryCleanup(db, &config.BrowseHistoryConfig{RetentionDays: 7, CleanupTime: "03:00"})
	cleanup.now = func() time.Time { return now }

	failed, err := cleanup.runOnceWithStatus()
	if !errors.Is(err, wantErr) {
		t.Fatalf("first cleanup error = %v, want %v", err, wantErr)
	}
	if !failed.AcquiredLeader {
		t.Fatalf("failed cleanup result = %#v, want leader acquisition before failure", failed)
	}

	retry, err := cleanup.runOnceWithStatus()
	if err != nil {
		t.Fatalf("retry cleanup returned error: %v", err)
	}
	if !retry.AcquiredLeader || retry.Deleted != 1 {
		t.Fatalf("retry cleanup result = %#v, want released lock and one deletion", retry)
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

func setupBrowseHistoryCleanupPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.BrowseHistory{}); err != nil {
		t.Fatalf("migrate postgres browse history: %v", err)
	}
	seedBrowseHistoryCleanupRows(t, db)
	return db
}

func seedBrowseHistoryCleanupRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	author := model.User{ID: 1, Email: "cleanup-author@example.test", Username: "cleanup-author", PasswordHash: "hash"}
	viewer := model.User{ID: 2, Email: "cleanup-viewer@example.test", Username: "cleanup-viewer", PasswordHash: "hash"}
	if err := db.Create(&[]model.User{author, viewer}).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	content := model.ContentItem{
		ID:          1,
		Title:       "expired content",
		AuthorID:    author.ID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "published",
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	now := time.Date(2026, 7, 2, 3, 0, 0, 0, mustShanghaiLocation(t))
	row := model.BrowseHistory{ID: 1, UserID: viewer.ID, ContentItemID: content.ID, ViewedAt: now.Add(-8 * 24 * time.Hour)}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create history row: %v", err)
	}
}

func mustShanghaiLocation(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load Asia/Shanghai: %v", err)
	}
	return loc
}
