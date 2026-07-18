package repository

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
)

func TestSeriesRepositoryListMembershipsUsesVisibleOneBasedNavigationAndReturnsAll(t *testing.T) {
	db := setupSeriesRepositoryDB(t)
	owner := seedSeriesRepositoryUser(t, db, 1, "membership-owner")
	previous := seedSeriesRepositoryContent(t, db, 10, owner.ID, "published", true, "previous")
	current := seedSeriesRepositoryContent(t, db, 11, owner.ID, "published", true, "current")
	next := seedSeriesRepositoryContent(t, db, 12, owner.ID, "published", true, "next")
	pending := seedSeriesRepositoryContent(t, db, 13, owner.ID, "pending", true, "pending secret")

	base := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	for index := 0; index < 4; index++ {
		series := model.ContentSeries{
			ID:        int64(100 + index),
			Title:     fmt.Sprintf("series-%d", index),
			OwnerID:   owner.ID,
			Zone:      "original",
			CreatedAt: base.Add(time.Duration(index) * time.Minute),
			UpdatedAt: base.Add(time.Duration(index) * time.Minute),
		}
		if err := db.Create(&series).Error; err != nil {
			t.Fatalf("create series %d: %v", index, err)
		}
		for itemIndex, content := range []model.ContentItem{pending, previous, current, next} {
			item := model.ContentSeriesItem{
				SeriesID:      series.ID,
				ContentItemID: content.ID,
				SortOrder:     itemIndex,
			}
			if err := db.Create(&item).Error; err != nil {
				t.Fatalf("create series item %d/%d: %v", index, itemIndex, err)
			}
		}
	}
	var queryCount atomic.Int64
	if err := db.Callback().Query().Before("gorm:query").Register("test:count_membership_queries", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatalf("register query counter: %v", err)
	}
	if err := db.Callback().Row().Before("gorm:row").Register("test:count_membership_row_queries", func(*gorm.DB) {
		queryCount.Add(1)
	}); err != nil {
		t.Fatalf("register row query counter: %v", err)
	}

	memberships, err := NewSeriesRepository(db).ListMembershipsForContent(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("ListMembershipsForContent() error = %v", err)
	}
	if len(memberships) != 4 {
		t.Fatalf("memberships length = %d, want all 4", len(memberships))
	}
	if got := queryCount.Load(); got > 3 {
		t.Fatalf("membership query count = %d, want at most 3 regardless of series count", got)
	}
	if memberships[0].SeriesID != 103 || memberships[3].SeriesID != 100 {
		t.Fatalf("membership order = %#v, want updated_at DESC then id", memberships)
	}
	for _, membership := range memberships {
		if membership.SeriesZone != "original" {
			t.Fatalf("membership zone = %q, want original", membership.SeriesZone)
		}
		if membership.CurrentIndex != 2 || membership.Total != 3 {
			t.Fatalf("membership position = %#v, want visible 2/3", membership)
		}
		if membership.Previous == nil || membership.Previous.ID != previous.ID || membership.Previous.Title != previous.Title {
			t.Fatalf("previous = %#v, want previous content", membership.Previous)
		}
		if membership.Next == nil || membership.Next.ID != next.ID || membership.Next.Title != next.Title {
			t.Fatalf("next = %#v, want next content", membership.Next)
		}
	}
}

func TestSeriesRepositoryListVisibleItemsUsesSharedContentVisibility(t *testing.T) {
	db := setupSeriesRepositoryDB(t)
	owner := seedSeriesRepositoryUser(t, db, 1, "visible-owner")
	bannedAuthor := seedSeriesRepositoryUser(t, db, 2, "banned-author")
	bannedAuthor.IsBanned = true
	if err := db.Save(&bannedAuthor).Error; err != nil {
		t.Fatalf("ban author: %v", err)
	}
	deletedAuthor := seedSeriesRepositoryUser(t, db, 3, "deleted-author")
	now := time.Now()
	deletedAuthor.DeletedAt = &now
	if err := db.Save(&deletedAuthor).Error; err != nil {
		t.Fatalf("delete author: %v", err)
	}
	bannedIP := model.IP{ID: 10, Name: "blocked", Slug: "blocked", Status: "banned"}
	if err := db.Create(&bannedIP).Error; err != nil {
		t.Fatalf("create banned IP: %v", err)
	}

	series := model.ContentSeries{ID: 100, Title: "visibility", OwnerID: owner.ID, Zone: "original"}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}
	visible := seedSeriesRepositoryContent(t, db, 10, owner.ID, "published", true, "visible")
	private := seedSeriesRepositoryContent(t, db, 11, owner.ID, "published", false, "private")
	banned := seedSeriesRepositoryContent(t, db, 12, bannedAuthor.ID, "published", true, "banned author")
	deleted := seedSeriesRepositoryContent(t, db, 13, deletedAuthor.ID, "published", true, "deleted author")
	blockedIP := seedSeriesRepositoryContent(t, db, 14, owner.ID, "published", true, "banned ip")
	blockedIP.IPID = &bannedIP.ID
	if err := db.Save(&blockedIP).Error; err != nil {
		t.Fatalf("set banned IP: %v", err)
	}
	for order, content := range []model.ContentItem{visible, private, banned, deleted, blockedIP} {
		if err := db.Create(&model.ContentSeriesItem{SeriesID: series.ID, ContentItemID: content.ID, SortOrder: order}).Error; err != nil {
			t.Fatalf("create item %d: %v", order, err)
		}
	}

	items, err := NewSeriesRepository(db).ListVisibleSeriesItems(context.Background(), series.ID, 0)
	if err != nil {
		t.Fatalf("ListVisibleSeriesItems() error = %v", err)
	}
	if len(items) != 1 || items[0].ContentItemID != visible.ID {
		t.Fatalf("visible items = %#v, want only shared-scope visible content", items)
	}
}

func TestSeriesRepositoryItemMutationsRefreshSeriesUpdatedAt(t *testing.T) {
	db := setupSeriesRepositoryDB(t)
	owner := seedSeriesRepositoryUser(t, db, 1, "timestamp-owner")
	first := seedSeriesRepositoryContent(t, db, 10, owner.ID, "published", true, "first")
	second := seedSeriesRepositoryContent(t, db, 11, owner.ID, "published", true, "second")
	series := model.ContentSeries{ID: 100, Title: "timestamped", OwnerID: owner.ID, Zone: "original"}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}
	firstItem := model.ContentSeriesItem{SeriesID: series.ID, ContentItemID: first.ID, SortOrder: 0}
	if err := db.Create(&firstItem).Error; err != nil {
		t.Fatalf("create first item: %v", err)
	}
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := NewSeriesRepository(db)
	resetTimestamp := func() {
		db.Model(&model.ContentSeries{}).Where("id = ?", series.ID).UpdateColumn("updated_at", base)
	}
	assertRefreshed := func(operation string) {
		var refreshed model.ContentSeries
		if err := db.First(&refreshed, series.ID).Error; err != nil {
			t.Fatalf("load series after %s: %v", operation, err)
		}
		if !refreshed.UpdatedAt.After(base) {
			t.Fatalf("updated_at after %s = %v, want after %v", operation, refreshed.UpdatedAt, base)
		}
	}

	resetTimestamp()
	if _, err := repo.AddItem(context.Background(), series.ID, owner.ID, second.ID); err != nil {
		t.Fatalf("add item: %v", err)
	}
	assertRefreshed("add")

	var items []model.ContentSeriesItem
	if err := db.Where("series_id = ?", series.ID).Order("sort_order ASC").Find(&items).Error; err != nil {
		t.Fatalf("load items: %v", err)
	}
	resetTimestamp()
	if err := repo.ReorderItems(context.Background(), series.ID, owner.ID, []int64{items[1].ID, items[0].ID}); err != nil {
		t.Fatalf("reorder items: %v", err)
	}
	assertRefreshed("reorder")

	resetTimestamp()
	if err := repo.RemoveItem(context.Background(), series.ID, owner.ID, firstItem.ID); err != nil {
		t.Fatalf("remove item: %v", err)
	}
	assertRefreshed("remove")
}

func TestSeriesRepositoryManagementItemsIncludeSoftDeletedContentSummary(t *testing.T) {
	db := setupSeriesRepositoryDB(t)
	owner := seedSeriesRepositoryUser(t, db, 1, "orphan-owner")
	content := seedSeriesRepositoryContent(t, db, 10, owner.ID, "published", true, "deleted chapter")
	deletedAt := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	if err := db.Model(&content).Update("deleted_at", deletedAt).Error; err != nil {
		t.Fatalf("soft-delete content: %v", err)
	}
	series := model.ContentSeries{ID: 100, Title: "orphan series", OwnerID: owner.ID, Zone: "original"}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}
	if err := db.Create(&model.ContentSeriesItem{SeriesID: series.ID, ContentItemID: content.ID, SortOrder: 0}).Error; err != nil {
		t.Fatalf("create series item: %v", err)
	}
	items, err := NewSeriesRepository(db).ListSeriesItems(context.Background(), series.ID)
	if err != nil {
		t.Fatalf("ListSeriesItems() error = %v", err)
	}
	if len(items) != 1 || items[0].ContentItem.ID != content.ID || items[0].ContentItem.Title != content.Title {
		t.Fatalf("management item = %#v, want soft-deleted content summary", items)
	}
}

func TestSeriesRepositoryListMembershipsHidesInvisibleCurrentContent(t *testing.T) {
	db := setupSeriesRepositoryDB(t)
	owner := seedSeriesRepositoryUser(t, db, 1, "hidden-owner")
	current := seedSeriesRepositoryContent(t, db, 10, owner.ID, "pending", true, "hidden current")
	series := model.ContentSeries{ID: 100, Title: "hidden series", OwnerID: owner.ID, Zone: "original"}
	if err := db.Create(&series).Error; err != nil {
		t.Fatalf("create series: %v", err)
	}
	if err := db.Create(&model.ContentSeriesItem{SeriesID: series.ID, ContentItemID: current.ID}).Error; err != nil {
		t.Fatalf("create series item: %v", err)
	}

	memberships, err := NewSeriesRepository(db).ListMembershipsForContent(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("ListMembershipsForContent() error = %v", err)
	}
	if len(memberships) != 0 {
		t.Fatalf("memberships = %#v, want none for invisible current content", memberships)
	}
}

func TestSeriesRepositoryListCandidatesIncludesOwnedAndContributedManageableContent(t *testing.T) {
	db := setupSeriesRepositoryDB(t)
	owner := seedSeriesRepositoryUser(t, db, 1, "candidate-owner")
	other := seedSeriesRepositoryUser(t, db, 2, "candidate-author")
	ownedPending := seedSeriesRepositoryContent(t, db, 10, owner.ID, "pending", true, "owned pending")
	contributed := seedSeriesRepositoryContent(t, db, 11, other.ID, "published", true, "contributed chapter")
	unrelated := seedSeriesRepositoryContent(t, db, 12, other.ID, "published", true, "unrelated")
	banned := seedSeriesRepositoryContent(t, db, 13, owner.ID, "banned", true, "blocked")
	if err := db.Create(&model.ContentContributor{ContentItemID: contributed.ID, UserID: owner.ID}).Error; err != nil {
		t.Fatalf("seed contributor: %v", err)
	}

	items, err := NewSeriesRepository(db).ListCandidateContents(context.Background(), owner.ID, "original", "chapter", 50)
	if err != nil {
		t.Fatalf("ListCandidateContents() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != contributed.ID {
		t.Fatalf("searched candidates = %#v, want contributed content only", items)
	}
	items, err = NewSeriesRepository(db).ListCandidateContents(context.Background(), owner.ID, "original", "", 50)
	if err != nil {
		t.Fatalf("ListCandidateContents() error = %v", err)
	}
	if len(items) != 2 || items[0].ID == unrelated.ID || items[0].ID == banned.ID || items[1].ID == unrelated.ID || items[1].ID == banned.ID {
		t.Fatalf("candidates = %#v, want owned pending and contributed published; owned=%d", items, ownedPending.ID)
	}
}

func setupSeriesRepositoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentContributor{}, &model.ContentSeries{}, &model.ContentSeriesItem{}); err != nil {
		t.Fatalf("migrate series repository models: %v", err)
	}
	return db
}

func seedSeriesRepositoryUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	user := model.User{ID: id, Email: username + "@example.test", Username: username, PasswordHash: "hash", Role: "user", Reputation: 10}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func seedSeriesRepositoryContent(t *testing.T, db *gorm.DB, id, authorID int64, status string, public bool, title string) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       title,
		AuthorID:    authorID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      status,
		IsPublic:    public,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content %q: %v", title, err)
	}
	if !public {
		if err := db.Model(&content).Update("is_public", false).Error; err != nil {
			t.Fatalf("make content %q private: %v", title, err)
		}
		content.IsPublic = false
	}
	return content
}
