package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

func TestSeriesCreateAndListOwned(t *testing.T) {
	db := setupSeriesServiceDB(t)
	owner := seedSeriesUser(t, db, 1, "owner")
	svc := NewSeriesService(repository.NewSeriesRepository(db))

	created, err := svc.CreateSeries(context.Background(), owner.ID, "  First arc  ", "  notes  ", "original")
	if err != nil {
		t.Fatalf("CreateSeries() error = %v", err)
	}
	if created.Title != "First arc" || created.Description != "notes" || created.OwnerID != owner.ID || created.Zone != "original" {
		t.Fatalf("created series = %#v", created)
	}
	items, err := svc.ListOwnedSeries(context.Background(), owner.ID, "original")
	if err != nil {
		t.Fatalf("ListOwnedSeries() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("owned series = %#v, want created series", items)
	}
}

func TestSeriesAddOwnerAuthoredContent(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	content := seedSeriesContent(t, db, 10, owner.ID, "original", "pending", false, "owner pending")

	item, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
	if err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}
	if item.SortOrder != 0 || item.ContentItemID != content.ID {
		t.Fatalf("added item = %#v", item)
	}
}

func TestSeriesAnonymousDetailDoesNotExposePendingStudioItems(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	pending := seedSeriesContent(t, db, 10, owner.ID, "original", "pending", false, "private draft title")
	published := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "public chapter")

	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, pending.ID); err != nil {
		t.Fatalf("AddItem() pending Studio content error = %v", err)
	}
	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, published.ID); err != nil {
		t.Fatalf("AddItem() published content error = %v", err)
	}

	detail, err := svc.GetSeriesDetail(context.Background(), series.ID, 0)
	if err != nil {
		t.Fatalf("GetSeriesDetail() error = %v", err)
	}
	if detail.ItemCount != 1 || len(detail.Items) != 1 {
		t.Fatalf("anonymous detail counts = item_count:%d items:%d, want 1 and 1", detail.ItemCount, len(detail.Items))
	}
	if detail.Items[0].ContentItem.ID != published.ID || detail.Items[0].ContentItem.Title != published.Title {
		t.Fatalf("anonymous detail items = %#v, want only published chapter", detail.Items)
	}
}

func TestSeriesAddContributorContent(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	author := seedSeriesUser(t, db, 2, "author")
	content := seedSeriesContent(t, db, 10, author.ID, "original", "published", false, "contributed")
	if err := db.Create(&model.ContentContributor{ContentItemID: content.ID, UserID: owner.ID}).Error; err != nil {
		t.Fatalf("seed contributor: %v", err)
	}

	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID); err != nil {
		t.Fatalf("AddItem() contributor error = %v", err)
	}
}

func TestSeriesRejectsUnrelatedContent(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	author := seedSeriesUser(t, db, 2, "unrelated")
	content := seedSeriesContent(t, db, 10, author.ID, "original", "published", false, "unrelated")

	_, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
	if !errors.Is(err, repository.ErrContentNotOwnedOrContributed) {
		t.Fatalf("AddItem() error = %v, want CONTENT_NOT_OWNED_OR_CONTRIBUTED", err)
	}
}

func TestSeriesRejectsZoneMismatch(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	content := seedSeriesContent(t, db, 10, owner.ID, "fanwork", "published", false, "wrong zone")

	_, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
	if !errors.Is(err, repository.ErrSeriesZoneMismatch) {
		t.Fatalf("AddItem() error = %v, want ZONE_MISMATCH", err)
	}
}

func TestSeriesRejectsDeletedBannedOrAuthorDeletedContent(t *testing.T) {
	for _, status := range []string{"under_review", "banned", "author_deleted"} {
		t.Run(status, func(t *testing.T) {
			db, svc, owner, series := seedSeriesServiceFixture(t, "original")
			content := seedSeriesContent(t, db, 10, owner.ID, "original", status, false, status)
			_, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
			if !errors.Is(err, repository.ErrSeriesContentUnavailable) {
				t.Fatalf("AddItem() error = %v, want unavailable", err)
			}
		})
	}
	t.Run("soft deleted", func(t *testing.T) {
		db, svc, owner, series := seedSeriesServiceFixture(t, "original")
		content := seedSeriesContent(t, db, 10, owner.ID, "original", "published", true, "deleted")
		_, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
		if !errors.Is(err, repository.ErrSeriesContentUnavailable) {
			t.Fatalf("AddItem() error = %v, want unavailable", err)
		}
	})
}

func TestSeriesRejectsDuplicateItem(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	content := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "duplicate")
	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID); err != nil {
		t.Fatalf("first AddItem() error = %v", err)
	}
	_, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
	if !errors.Is(err, repository.ErrDuplicateSeriesItem) {
		t.Fatalf("duplicate AddItem() error = %v, want duplicate", err)
	}
}

func TestSeriesAddItemAppendsAfterMaxSortOrder(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	first := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "first")
	second := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "second")
	seedSeriesItem(t, db, 100, series.ID, first.ID, 7)

	item, err := svc.AddItem(context.Background(), series.ID, owner.ID, second.ID)
	if err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}
	if item.SortOrder != 8 {
		t.Fatalf("SortOrder = %d, want 8", item.SortOrder)
	}
}

func TestSeriesAddItemConcurrentAppendKeepsStableUniqueSortOrder(t *testing.T) {
	db := setupSeriesServicePostgres(t)
	const itemCount = 8
	var wg sync.WaitGroup
	errs := make(chan error, itemCount)
	for i := 0; i < itemCount; i++ {
		contentID := int64(100 + i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc := NewSeriesService(repository.NewSeriesRepository(db))
			_, err := svc.AddItem(context.Background(), 10, 1, contentID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent AddItem() error = %v", err)
		}
	}

	var rows []model.ContentSeriesItem
	if err := db.Order("sort_order ASC").Find(&rows, "series_id = ?", 10).Error; err != nil {
		t.Fatalf("load appended items: %v", err)
	}
	if len(rows) != itemCount {
		t.Fatalf("appended rows = %d, want %d", len(rows), itemCount)
	}
	for index, row := range rows {
		if row.SortOrder != index {
			t.Fatalf("sort orders = %#v, want contiguous 0..%d", rows, itemCount-1)
		}
	}
}

func TestSeriesRejectsCoverNotInSeries(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	content := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "not added")

	_, err := svc.UpdateSeries(context.Background(), series.ID, owner.ID, repository.SeriesPatch{CoverContentID: &content.ID})
	if !errors.Is(err, repository.ErrCoverNotInSeries) {
		t.Fatalf("UpdateSeries() error = %v, want COVER_NOT_IN_SERIES", err)
	}
}

func TestSeriesCoverFallsBackWhenCoverContentDeleted(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	deletedCover := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "deleted cover")
	fallback := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "fallback")
	deletedCover.CoverImageURL = "https://example.test/deleted.png"
	fallback.CoverImageURL = "https://example.test/fallback.png"
	if err := db.Save(&deletedCover).Error; err != nil {
		t.Fatalf("save deleted cover: %v", err)
	}
	if err := db.Save(&fallback).Error; err != nil {
		t.Fatalf("save fallback cover: %v", err)
	}
	firstItem, err := svc.AddItem(context.Background(), series.ID, owner.ID, deletedCover.ID)
	if err != nil {
		t.Fatalf("add deleted cover: %v", err)
	}
	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, fallback.ID); err != nil {
		t.Fatalf("add fallback: %v", err)
	}
	if _, err := svc.UpdateSeries(context.Background(), series.ID, owner.ID, repository.SeriesPatch{CoverContentID: &deletedCover.ID}); err != nil {
		t.Fatalf("set cover: %v", err)
	}
	now := time.Now()
	if err := db.Model(&model.ContentItem{}).Where("id = ?", deletedCover.ID).Update("deleted_at", now).Error; err != nil {
		t.Fatalf("delete cover content: %v", err)
	}

	detail, err := svc.GetSeriesDetail(context.Background(), series.ID, 0)
	if err != nil {
		t.Fatalf("GetSeriesDetail() error = %v", err)
	}
	if detail.Cover == nil || *detail.Cover != fallback.CoverImageURL {
		t.Fatalf("cover = %#v, want fallback %q; first item=%d", detail.Cover, fallback.CoverImageURL, firstItem.ID)
	}
}

func TestSeriesPendingItemIsManageableWithoutLeakingThroughPublicDetail(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	pending := seedSeriesContent(t, db, 10, owner.ID, "original", "pending", false, "private draft title")
	published := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "public chapter")
	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, pending.ID); err != nil {
		t.Fatalf("add pending item: %v", err)
	}
	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, published.ID); err != nil {
		t.Fatalf("add published item: %v", err)
	}

	publicDetail, err := svc.GetSeriesDetail(context.Background(), series.ID, 0)
	if err != nil {
		t.Fatalf("GetSeriesDetail() public error = %v", err)
	}
	if publicDetail.ItemCount != 1 || len(publicDetail.Items) != 1 || publicDetail.Items[0].ContentItem.Title != published.Title {
		t.Fatalf("public detail leaked pending item: %#v", publicDetail.Items)
	}

	ownerDetail, err := svc.GetSeriesManagementDetail(context.Background(), series.ID, owner.ID)
	if err != nil {
		t.Fatalf("GetSeriesDetail() owner error = %v", err)
	}
	if ownerDetail.ItemCount != 2 || len(ownerDetail.Items) != 2 {
		t.Fatalf("owner management detail = %#v, want both items", ownerDetail.Items)
	}
}

func TestSeriesReorderIsTransactional(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	first := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "first")
	second := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "second")
	firstItem, _ := svc.AddItem(context.Background(), series.ID, owner.ID, first.ID)
	secondItem, _ := svc.AddItem(context.Background(), series.ID, owner.ID, second.ID)

	if err := svc.ReorderItems(context.Background(), series.ID, owner.ID, []int64{secondItem.ID, firstItem.ID}); err != nil {
		t.Fatalf("ReorderItems() error = %v", err)
	}
	assertSeriesItemOrders(t, db, series.ID, map[int64]int{secondItem.ID: 0, firstItem.ID: 1})
}

func TestSeriesReorderRejectsMissingOrForeignItems(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	first := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "first")
	second := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "second")
	firstItem, _ := svc.AddItem(context.Background(), series.ID, owner.ID, first.ID)
	secondItem, _ := svc.AddItem(context.Background(), series.ID, owner.ID, second.ID)
	otherSeries, _ := svc.CreateSeries(context.Background(), owner.ID, "Other", "", "original")
	foreignContent := seedSeriesContent(t, db, 12, owner.ID, "original", "published", false, "foreign")
	foreignItem, _ := svc.AddItem(context.Background(), otherSeries.ID, owner.ID, foreignContent.ID)

	for name, ids := range map[string][]int64{
		"missing": {firstItem.ID},
		"foreign": {firstItem.ID, foreignItem.ID},
	} {
		t.Run(name, func(t *testing.T) {
			err := svc.ReorderItems(context.Background(), series.ID, owner.ID, ids)
			if !errors.Is(err, repository.ErrSeriesItemSetMismatch) {
				t.Fatalf("ReorderItems() error = %v, want item-set mismatch", err)
			}
			assertSeriesItemOrders(t, db, series.ID, map[int64]int{firstItem.ID: 0, secondItem.ID: 1})
		})
	}
}

func TestSeriesServiceListsContentMemberships(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	content := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "membership")
	if _, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID); err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}

	memberships, err := svc.ListMembershipsForContent(context.Background(), content.ID)
	if err != nil {
		t.Fatalf("ListMembershipsForContent() error = %v", err)
	}
	if len(memberships) != 1 || memberships[0].SeriesID != series.ID {
		t.Fatalf("memberships = %#v, want current series", memberships)
	}
}

func TestSeriesOwnerIsEnforcedAcrossDestructiveMutations(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	other := seedSeriesUser(t, db, 2, "not-owner")
	content := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "protected")
	item, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
	if err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}
	contributed := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "contributor cannot manage")
	if err := db.Create(&model.ContentContributor{ContentItemID: contributed.ID, UserID: other.ID}).Error; err != nil {
		t.Fatalf("seed contributor: %v", err)
	}
	if _, err := svc.AddItem(context.Background(), series.ID, other.ID, contributed.ID); !errors.Is(err, repository.ErrNotSeriesOwner) {
		t.Fatalf("contributor AddItem() error = %v, want NOT_SERIES_OWNER", err)
	}
	var unauthorizedMemberships int64
	if err := db.Model(&model.ContentSeriesItem{}).
		Where("series_id = ? AND content_item_id = ?", series.ID, contributed.ID).
		Count(&unauthorizedMemberships).Error; err != nil {
		t.Fatalf("count unauthorized memberships: %v", err)
	}
	if unauthorizedMemberships != 0 {
		t.Fatalf("unauthorized membership count = %d, want 0", unauthorizedMemberships)
	}
	title := "stolen"

	for operation, err := range map[string]error{
		"update": func() error {
			_, err := svc.UpdateSeries(context.Background(), series.ID, other.ID, repository.SeriesPatch{Title: &title})
			return err
		}(),
		"remove":  svc.RemoveItem(context.Background(), series.ID, other.ID, item.ID),
		"reorder": svc.ReorderItems(context.Background(), series.ID, other.ID, []int64{item.ID}),
		"delete":  svc.DeleteSeries(context.Background(), series.ID, other.ID),
	} {
		if !errors.Is(err, repository.ErrNotSeriesOwner) {
			t.Fatalf("%s error = %v, want NOT_SERIES_OWNER", operation, err)
		}
	}

	var persisted model.ContentSeries
	if err := db.First(&persisted, series.ID).Error; err != nil {
		t.Fatalf("series removed by non-owner: %v", err)
	}
	if persisted.Title != series.Title {
		t.Fatalf("series title = %q, want unchanged %q", persisted.Title, series.Title)
	}
	var itemCount int64
	if err := db.Model(&model.ContentSeriesItem{}).Where("id = ?", item.ID).Count(&itemCount).Error; err != nil {
		t.Fatalf("count protected item: %v", err)
	}
	if itemCount != 1 {
		t.Fatalf("protected item count = %d, want 1", itemCount)
	}
}

func TestSeriesOwnerCanUpdateRemoveAndDelete(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	content := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "managed")
	item, err := svc.AddItem(context.Background(), series.ID, owner.ID, content.ID)
	if err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}
	title := "Updated"
	updated, err := svc.UpdateSeries(context.Background(), series.ID, owner.ID, repository.SeriesPatch{Title: &title, CoverContentID: &content.ID})
	if err != nil || updated.Title != title {
		t.Fatalf("UpdateSeries() = %#v, %v", updated, err)
	}
	if err := svc.RemoveItem(context.Background(), series.ID, owner.ID, item.ID); err != nil {
		t.Fatalf("RemoveItem() error = %v", err)
	}
	if err := db.First(&model.ContentSeriesItem{}, item.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("removed item lookup error = %v, want not found", err)
	}
	if err := svc.DeleteSeries(context.Background(), series.ID, owner.ID); err != nil {
		t.Fatalf("DeleteSeries() error = %v", err)
	}
	if err := db.First(&model.ContentSeries{}, series.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted series lookup error = %v, want not found", err)
	}
}

func TestSeriesReorderRollsBackWhenAnUpdateFails(t *testing.T) {
	db, svc, owner, series := seedSeriesServiceFixture(t, "original")
	first := seedSeriesContent(t, db, 10, owner.ID, "original", "published", false, "first")
	second := seedSeriesContent(t, db, 11, owner.ID, "original", "published", false, "second")
	firstItem, _ := svc.AddItem(context.Background(), series.ID, owner.ID, first.ID)
	secondItem, _ := svc.AddItem(context.Background(), series.ID, owner.ID, second.ID)

	var updateCount atomic.Int64
	callbackName := "test:fail_second_series_item_update"
	if err := db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "content_series_items" && updateCount.Add(1) == 2 {
			tx.AddError(errors.New("forced second update failure"))
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	defer db.Callback().Update().Remove(callbackName)

	if err := svc.ReorderItems(context.Background(), series.ID, owner.ID, []int64{secondItem.ID, firstItem.ID}); err == nil {
		t.Fatal("ReorderItems() error = nil, want forced failure")
	}
	assertSeriesItemOrders(t, db, series.ID, map[int64]int{firstItem.ID: 0, secondItem.ID: 1})
}

func setupSeriesServiceDB(t *testing.T) *gorm.DB {
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
		t.Fatalf("migrate series service models: %v", err)
	}
	return db
}

func seedSeriesServiceFixture(t *testing.T, zone string) (*gorm.DB, *SeriesService, model.User, model.ContentSeries) {
	t.Helper()
	db := setupSeriesServiceDB(t)
	owner := seedSeriesUser(t, db, 1, "series-owner")
	svc := NewSeriesService(repository.NewSeriesRepository(db))
	series, err := svc.CreateSeries(context.Background(), owner.ID, "Series", "", zone)
	if err != nil {
		t.Fatalf("create fixture series: %v", err)
	}
	return db, svc, owner, *series
}

func seedSeriesUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	user := model.User{ID: id, Email: username + "@example.test", Username: username, PasswordHash: "hash", Role: "user", Reputation: 10}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func seedSeriesContent(t *testing.T, db *gorm.DB, id, authorID int64, zone, status string, deleted bool, title string) model.ContentItem {
	t.Helper()
	content := model.ContentItem{ID: id, Title: title, AuthorID: authorID, Zone: zone, Category: "game", ContentType: "article", Status: status, IsPublic: true}
	if deleted {
		now := time.Now()
		content.DeletedAt = &now
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content %s: %v", title, err)
	}
	return content
}

func seedSeriesItem(t *testing.T, db *gorm.DB, id, seriesID, contentID int64, order int) model.ContentSeriesItem {
	t.Helper()
	item := model.ContentSeriesItem{ID: id, SeriesID: seriesID, ContentItemID: contentID, SortOrder: order}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create series item: %v", err)
	}
	return item
}

func assertSeriesItemOrders(t *testing.T, db *gorm.DB, seriesID int64, want map[int64]int) {
	t.Helper()
	var rows []model.ContentSeriesItem
	if err := db.Where("series_id = ?", seriesID).Find(&rows).Error; err != nil {
		t.Fatalf("load series item orders: %v", err)
	}
	if len(rows) != len(want) {
		t.Fatalf("series rows = %#v, want %d rows", rows, len(want))
	}
	for _, row := range rows {
		if row.SortOrder != want[row.ID] {
			t.Fatalf("item %d sort_order = %d, want %d", row.ID, row.SortOrder, want[row.ID])
		}
	}
}

func setupSeriesServicePostgres(t *testing.T) *gorm.DB {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	if err := db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE ips (
			id BIGSERIAL PRIMARY KEY,
			status VARCHAR(20) NOT NULL DEFAULT 'approved'
		);
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			description TEXT,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			zone VARCHAR(10) NOT NULL,
			ip_id BIGINT REFERENCES ips(id) ON DELETE SET NULL,
			content_type VARCHAR(20) NOT NULL,
			cover_image_url TEXT,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			is_public BOOLEAN NOT NULL DEFAULT TRUE,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE content_contributors (
			content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			PRIMARY KEY (content_item_id, user_id)
		);
		INSERT INTO users (id, email, password_hash, username)
		VALUES (1, 'pg-series-owner@example.test', 'hash', 'pg-series-owner');
	`).Error; err != nil {
		t.Fatalf("create PostgreSQL series base tables: %v", err)
	}
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "059_create_content_series.sql"))
	if err := db.Exec(`
		INSERT INTO content_series (id, title, owner_id, zone)
		VALUES (10, 'Concurrent series', 1, 'original');
	`).Error; err != nil {
		t.Fatalf("seed PostgreSQL series: %v", err)
	}
	for i := 0; i < 8; i++ {
		if err := db.Exec(`
			INSERT INTO content_items (id, title, author_id, zone, content_type, status)
			VALUES (?, ?, 1, 'original', 'article', 'published')
		`, 100+i, fmt.Sprintf("content-%d", i)).Error; err != nil {
			t.Fatalf("seed PostgreSQL content %d: %v", i, err)
		}
	}
	return db
}
