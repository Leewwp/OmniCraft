package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
)

func TestCollectionRepoCreateAndListByOwnerAndZone(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "collection-owner")
	other := seedCollectionRepoUser(t, db, 20, "collection-other")

	if _, err := repo.EnsureDefaultCollection(ctx, owner.ID, "original"); err != nil {
		t.Fatalf("EnsureDefaultCollection(original) error = %v", err)
	}
	if _, err := repo.CreateCollection(ctx, &model.Collection{
		UserID:   owner.ID,
		Title:    "owner custom original",
		Zone:     "original",
		IsPublic: true,
	}); err != nil {
		t.Fatalf("CreateCollection(owner original) error = %v", err)
	}
	if _, err := repo.CreateCollection(ctx, &model.Collection{
		UserID: owner.ID,
		Title:  "owner custom fanwork",
		Zone:   "fanwork",
	}); err != nil {
		t.Fatalf("CreateCollection(owner fanwork) error = %v", err)
	}
	if _, err := repo.CreateCollection(ctx, &model.Collection{
		UserID: other.ID,
		Title:  "other original",
		Zone:   "original",
	}); err != nil {
		t.Fatalf("CreateCollection(other original) error = %v", err)
	}

	items, err := repo.ListCollections(ctx, owner.ID, "original", nil)
	if err != nil {
		t.Fatalf("ListCollections() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2; items=%#v", len(items), items)
	}
	if !items[0].IsDefault || items[0].SortOrder != 0 {
		t.Fatalf("first item = %#v, want default collection first", items[0])
	}
	if items[1].Title != "owner custom original" || items[1].Zone != "original" || items[1].UserID != owner.ID {
		t.Fatalf("second item = %#v, want owner original custom collection", items[1])
	}
	if items[1].SortOrder <= items[0].SortOrder {
		t.Fatalf("custom sort_order = %d, want after default sort_order %d", items[1].SortOrder, items[0].SortOrder)
	}
	for _, item := range items {
		if item.ContainsItem || item.ItemID != nil {
			t.Fatalf("contains marker without content id = %#v, want false/nil", item)
		}
	}
}

func TestCollectionRepoGetPublicVisibleToAnonymous(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "public-owner")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "public original", "original", false, true, 1)

	got, err := repo.GetCollectionForViewer(ctx, collection.ID, nil)
	if err != nil {
		t.Fatalf("GetCollectionForViewer(public anonymous) error = %v", err)
	}
	if got.ID != collection.ID || !got.IsPublic {
		t.Fatalf("collection = %#v, want public collection %d", got, collection.ID)
	}
}

func TestCollectionRepoGetPrivateVisibleOnlyToOwner(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "private-owner")
	other := seedCollectionRepoUser(t, db, 20, "private-other")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "private original", "original", false, false, 1)

	if _, err := repo.GetCollectionForViewer(ctx, collection.ID, nil); !errors.Is(err, ErrCollectionNotFound) {
		t.Fatalf("anonymous private err = %v, want ErrCollectionNotFound", err)
	}
	if _, err := repo.GetCollectionForViewer(ctx, collection.ID, &other.ID); !errors.Is(err, ErrCollectionNotFound) {
		t.Fatalf("non-owner private err = %v, want ErrCollectionNotFound", err)
	}
	got, err := repo.GetCollectionForViewer(ctx, collection.ID, &owner.ID)
	if err != nil {
		t.Fatalf("owner private GetCollectionForViewer() error = %v", err)
	}
	if got.ID != collection.ID {
		t.Fatalf("collection ID = %d, want %d", got.ID, collection.ID)
	}
}

func TestCollectionRepoAddItemRejectsZoneMismatch(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "zone-owner")
	author := seedCollectionRepoUser(t, db, 20, "zone-author")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "original collection", "original", false, false, 1)
	content := seedCollectionRepoContent(t, db, 200, author.ID, "fanwork", "fanwork", "image", "published", nil)

	if _, err := repo.AddItem(ctx, collection.ID, owner.ID, content.ID, ""); !errors.Is(err, ErrZoneMismatch) {
		t.Fatalf("AddItem zone mismatch err = %v, want ErrZoneMismatch", err)
	}
}

func TestCollectionRepoAddItemRejectsUnpublishedOrDeletedContent(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	owner := seedCollectionRepoUser(t, db, 10, "invalid-content-owner")
	author := seedCollectionRepoUser(t, db, 20, "invalid-content-author")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "original collection", "original", false, false, 1)
	pending := seedCollectionRepoContent(t, db, 200, author.ID, "pending", "original", "article", "pending", nil)
	deleted := seedCollectionRepoContent(t, db, 201, author.ID, "deleted", "original", "article", "published", &now)

	for _, content := range []model.ContentItem{pending, deleted} {
		if _, err := repo.AddItem(ctx, collection.ID, owner.ID, content.ID, ""); !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("AddItem invalid content %d err = %v, want ErrInvalidContent", content.ID, err)
		}
	}
}

func TestCollectionRepoAddItemRejectsDuplicate(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "duplicate-owner")
	author := seedCollectionRepoUser(t, db, 20, "duplicate-author")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "original collection", "original", false, false, 1)
	content := seedCollectionRepoContent(t, db, 200, author.ID, "published", "original", "article", "published", nil)

	if _, err := repo.AddItem(ctx, collection.ID, owner.ID, content.ID, "first"); err != nil {
		t.Fatalf("AddItem first error = %v", err)
	}
	if _, err := repo.AddItem(ctx, collection.ID, owner.ID, content.ID, "second"); !errors.Is(err, ErrDuplicateCollectionItem) {
		t.Fatalf("AddItem duplicate err = %v, want ErrDuplicateCollectionItem", err)
	}
}

func TestCollectionRepoDeleteRejectsDefaultCollection(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "default-delete-owner")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "default original", "original", true, false, 0)

	if err := repo.DeleteCollection(ctx, collection.ID, owner.ID); !errors.Is(err, ErrDefaultCollectionProtected) {
		t.Fatalf("DeleteCollection(default) err = %v, want ErrDefaultCollectionProtected", err)
	}
	var got model.Collection
	if err := db.First(&got, collection.ID).Error; err != nil {
		t.Fatalf("load collection after delete rejection: %v", err)
	}
	if got.DeletedAt != nil {
		t.Fatalf("default collection DeletedAt = %v, want nil", got.DeletedAt)
	}
}

func TestCollectionRepoDetailFiltersUnavailableContent(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	owner := seedCollectionRepoUser(t, db, 10, "detail-owner")
	author := seedCollectionRepoUser(t, db, 20, "detail-author")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "original collection", "original", false, true, 1)
	publishedArticle := seedCollectionRepoContent(t, db, 200, author.ID, "published article", "original", "article", "published", nil)
	publishedVideo := seedCollectionRepoContent(t, db, 201, author.ID, "published video", "original", "video", "published", nil)
	pending := seedCollectionRepoContent(t, db, 202, author.ID, "pending", "original", "article", "pending", nil)
	deleted := seedCollectionRepoContent(t, db, 203, author.ID, "deleted", "original", "article", "published", &now)
	seedCollectionRepoItem(t, db, 300, collection.ID, publishedArticle.ID, "")
	seedCollectionRepoItem(t, db, 301, collection.ID, publishedVideo.ID, "")
	seedCollectionRepoItem(t, db, 302, collection.ID, pending.ID, "")
	seedCollectionRepoItem(t, db, 303, collection.ID, deleted.ID, "")

	items, total, err := repo.ListItems(ctx, collection.ID, 1, 20, "article")
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total/items = %d/%d, want 1/1; items=%#v", total, len(items), items)
	}
	if items[0].ContentItem.ID != publishedArticle.ID {
		t.Fatalf("item content ID = %d, want %d", items[0].ContentItem.ID, publishedArticle.ID)
	}
}

func TestCollectionRepoListItemsAppliesSharedVisibility(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	owner := seedCollectionRepoUser(t, db, 10, "visibility-owner")
	publicAuthor := seedCollectionRepoUser(t, db, 20, "visibility-public-author")
	viewerAuthor := seedCollectionRepoUser(t, db, 30, "visibility-viewer-author")
	otherPrivateAuthor := seedCollectionRepoUser(t, db, 40, "visibility-private-other")
	bannedAuthor := seedCollectionRepoUserWithState(t, db, 50, "visibility-banned-author", true, nil)
	deletedAuthor := seedCollectionRepoUserWithState(t, db, 60, "visibility-deleted-author", false, &now)
	bannedIP := seedCollectionRepoIP(t, db, 70, "visibility-banned-ip", "banned")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "public original collection", "original", false, true, 1)

	visiblePublic := seedCollectionRepoContentWithVisibility(t, db, 200, publicAuthor.ID, "visible public", "original", "article", "published", true, nil, nil)
	privateViewer := seedCollectionRepoContentWithVisibility(t, db, 201, viewerAuthor.ID, "private viewer", "original", "article", "published", false, nil, nil)
	privateOther := seedCollectionRepoContentWithVisibility(t, db, 202, otherPrivateAuthor.ID, "private other", "original", "article", "published", false, nil, nil)
	bannedAuthorContent := seedCollectionRepoContentWithVisibility(t, db, 203, bannedAuthor.ID, "banned author", "original", "article", "published", true, nil, nil)
	deletedAuthorContent := seedCollectionRepoContentWithVisibility(t, db, 204, deletedAuthor.ID, "deleted author", "original", "article", "published", true, nil, nil)
	bannedIPContent := seedCollectionRepoContentWithVisibility(t, db, 205, publicAuthor.ID, "banned ip", "original", "article", "published", true, &bannedIP.ID, nil)
	seedCollectionRepoItem(t, db, 300, collection.ID, visiblePublic.ID, "")
	seedCollectionRepoItem(t, db, 301, collection.ID, privateViewer.ID, "")
	seedCollectionRepoItem(t, db, 302, collection.ID, privateOther.ID, "")
	seedCollectionRepoItem(t, db, 303, collection.ID, bannedAuthorContent.ID, "")
	seedCollectionRepoItem(t, db, 304, collection.ID, deletedAuthorContent.ID, "")
	seedCollectionRepoItem(t, db, 305, collection.ID, bannedIPContent.ID, "")

	anonymousItems, anonymousTotal, err := repo.ListItemsForViewer(ctx, collection.ID, 0, 1, 20, "article")
	if err != nil {
		t.Fatalf("ListItemsForViewer(anonymous) error = %v", err)
	}
	assertCollectionRepoItemTitles(t, anonymousItems, []string{"visible public"})
	if anonymousTotal != 1 {
		t.Fatalf("anonymous total = %d, want 1", anonymousTotal)
	}

	viewerItems, viewerTotal, err := repo.ListItemsForViewer(ctx, collection.ID, viewerAuthor.ID, 1, 20, "article")
	if err != nil {
		t.Fatalf("ListItemsForViewer(viewer) error = %v", err)
	}
	assertCollectionRepoItemTitles(t, viewerItems, []string{"private viewer", "visible public"})
	if viewerTotal != 2 {
		t.Fatalf("viewer total = %d, want 2", viewerTotal)
	}
}

func TestCollectionRepoListCollectionsCountsVisibleItemsForViewer(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "count-owner")
	other := seedCollectionRepoUser(t, db, 20, "count-other")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "public original collection", "original", false, true, 1)
	visiblePublic := seedCollectionRepoContentWithVisibility(t, db, 200, other.ID, "visible public", "original", "article", "published", true, nil, nil)
	privateOwner := seedCollectionRepoContentWithVisibility(t, db, 201, owner.ID, "private owner", "original", "article", "published", false, nil, nil)
	privateOther := seedCollectionRepoContentWithVisibility(t, db, 202, other.ID, "private other", "original", "article", "published", false, nil, nil)
	seedCollectionRepoItem(t, db, 300, collection.ID, visiblePublic.ID, "")
	seedCollectionRepoItem(t, db, 301, collection.ID, privateOwner.ID, "")
	seedCollectionRepoItem(t, db, 302, collection.ID, privateOther.ID, "")

	anonymousItems, err := repo.ListCollectionsForViewer(ctx, owner.ID, 0, "original", nil)
	if err != nil {
		t.Fatalf("ListCollectionsForViewer(anonymous) error = %v", err)
	}
	if len(anonymousItems) != 1 || anonymousItems[0].ItemCount != 1 {
		t.Fatalf("anonymous collection summaries = %#v, want one visible public item", anonymousItems)
	}

	ownerItems, err := repo.ListCollectionsForViewer(ctx, owner.ID, owner.ID, "original", nil)
	if err != nil {
		t.Fatalf("ListCollectionsForViewer(owner) error = %v", err)
	}
	if len(ownerItems) != 1 || ownerItems[0].ItemCount != 2 {
		t.Fatalf("owner collection summaries = %#v, want public plus owner-private item", ownerItems)
	}
}

func TestCollectionRepoListCollectionsForViewerFiltersPrivateCollections(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "visibility-list-owner")
	other := seedCollectionRepoUser(t, db, 20, "visibility-list-other")
	publicCollection := seedCollectionRepoCollection(t, db, 100, owner.ID, "public collection", "original", false, true, 1)
	privateCollection := seedCollectionRepoCollection(t, db, 101, owner.ID, "private collection", "original", false, false, 2)

	anonymousItems, err := repo.ListCollectionsForViewer(ctx, owner.ID, 0, "original", nil)
	if err != nil {
		t.Fatalf("ListCollectionsForViewer(anonymous) error = %v", err)
	}
	if len(anonymousItems) != 1 || anonymousItems[0].ID != publicCollection.ID {
		t.Fatalf("anonymous items = %#v, want only public collection %d", anonymousItems, publicCollection.ID)
	}

	otherItems, err := repo.ListCollectionsForViewer(ctx, owner.ID, other.ID, "original", nil)
	if err != nil {
		t.Fatalf("ListCollectionsForViewer(non-owner) error = %v", err)
	}
	if len(otherItems) != 1 || otherItems[0].ID != publicCollection.ID {
		t.Fatalf("non-owner items = %#v, want only public collection %d", otherItems, publicCollection.ID)
	}

	ownerItems, err := repo.ListCollectionsForViewer(ctx, owner.ID, owner.ID, "original", nil)
	if err != nil {
		t.Fatalf("ListCollectionsForViewer(owner) error = %v", err)
	}
	if len(ownerItems) != 2 {
		t.Fatalf("owner items len = %d, want public and private collections; private=%d", len(ownerItems), privateCollection.ID)
	}
}

func TestCollectionRepoListItemsForViewerRejectsPrivateCollectionForNonOwner(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "private-items-owner")
	other := seedCollectionRepoUser(t, db, 20, "private-items-other")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "private original collection", "original", false, false, 1)
	content := seedCollectionRepoContent(t, db, 200, owner.ID, "visible public", "original", "article", "published", nil)
	seedCollectionRepoItem(t, db, 300, collection.ID, content.ID, "")

	if _, _, err := repo.ListItemsForViewer(ctx, collection.ID, 0, 1, 20, ""); !errors.Is(err, ErrCollectionNotFound) {
		t.Fatalf("anonymous ListItemsForViewer err = %v, want ErrCollectionNotFound", err)
	}
	if _, _, err := repo.ListItemsForViewer(ctx, collection.ID, other.ID, 1, 20, ""); !errors.Is(err, ErrCollectionNotFound) {
		t.Fatalf("non-owner ListItemsForViewer err = %v, want ErrCollectionNotFound", err)
	}

	if items, total, err := repo.ListItemsForViewer(ctx, collection.ID, owner.ID, 1, 20, ""); err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("owner ListItemsForViewer items/total/err = %d/%d/%v, want one visible item", len(items), total, err)
	}
}

func TestCollectionRepoAddItemRejectsInvisibleContent(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "invisible-add-owner")
	other := seedCollectionRepoUser(t, db, 20, "invisible-add-other")
	collection := seedCollectionRepoCollection(t, db, 100, owner.ID, "original collection", "original", false, false, 1)
	privateOther := seedCollectionRepoContentWithVisibility(t, db, 200, other.ID, "private other", "original", "article", "published", false, nil, nil)

	if _, err := repo.AddItem(ctx, collection.ID, owner.ID, privateOther.ID, ""); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("AddItem invisible content err = %v, want ErrInvalidContent", err)
	}
}

func TestCollectionRepoListCollectionsMarksContainsItem(t *testing.T) {
	db := setupCollectionRepoDB(t)
	repo := NewCollectionRepository(db)
	ctx := context.Background()
	owner := seedCollectionRepoUser(t, db, 10, "contains-owner")
	author := seedCollectionRepoUser(t, db, 20, "contains-author")
	content := seedCollectionRepoContent(t, db, 200, author.ID, "published", "original", "article", "published", nil)
	withItem := seedCollectionRepoCollection(t, db, 100, owner.ID, "has item", "original", false, false, 1)
	withoutItem := seedCollectionRepoCollection(t, db, 101, owner.ID, "does not have item", "original", false, false, 2)
	seedCollectionRepoCollection(t, db, 102, owner.ID, "fanwork collection", "fanwork", false, false, 1)
	existing := seedCollectionRepoItem(t, db, 300, withItem.ID, content.ID, "")

	items, err := repo.ListCollections(ctx, owner.ID, "", &content.ID)
	if err != nil {
		t.Fatalf("ListCollections(content item) error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want same-zone original collections only; items=%#v", len(items), items)
	}

	seen := map[int64]CollectionSummary{}
	for _, item := range items {
		seen[item.ID] = item
		if item.Zone != "original" {
			t.Fatalf("item zone = %q, want original when content item zone supplies omitted filter", item.Zone)
		}
	}
	if got := seen[withItem.ID]; !got.ContainsItem || got.ItemID == nil || *got.ItemID != existing.ID {
		t.Fatalf("with-item summary = %#v, want contains true and item id %d", got, existing.ID)
	}
	if got := seen[withoutItem.ID]; got.ContainsItem || got.ItemID != nil {
		t.Fatalf("without-item summary = %#v, want contains false and nil item id", got)
	}
}

func setupCollectionRepoDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.Collection{}, &model.CollectionItem{}); err != nil {
		t.Fatalf("migrate collection repo: %v", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX idx_collections_one_default_per_zone
		ON collections (user_id, zone)
		WHERE is_default = TRUE
	`).Error; err != nil {
		t.Fatalf("create default collection unique index: %v", err)
	}
	return db
}

func seedCollectionRepoUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	return seedCollectionRepoUserWithState(t, db, id, username, false, nil)
}

func seedCollectionRepoUserWithState(t *testing.T, db *gorm.DB, id int64, username string, banned bool, deletedAt *time.Time) model.User {
	t.Helper()
	user := model.User{
		ID:           id,
		Email:        username + "@example.test",
		Username:     username,
		PasswordHash: "hash",
		Role:         "user",
		Reputation:   10,
		IsBanned:     banned,
		DeletedAt:    deletedAt,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func seedCollectionRepoContent(t *testing.T, db *gorm.DB, id, authorID int64, title, zone, contentType, status string, deletedAt *time.Time) model.ContentItem {
	t.Helper()
	return seedCollectionRepoContentWithVisibility(t, db, id, authorID, title, zone, contentType, status, true, nil, deletedAt)
}

func seedCollectionRepoContentWithVisibility(t *testing.T, db *gorm.DB, id, authorID int64, title, zone, contentType, status string, isPublic bool, ipID *int64, deletedAt *time.Time) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       title,
		AuthorID:    authorID,
		Zone:        zone,
		Category:    "game",
		ContentType: contentType,
		Status:      status,
		IsPublic:    isPublic,
		AllowCopy:   true,
		IPID:        ipID,
		DeletedAt:   deletedAt,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content %s: %v", title, err)
	}
	if !isPublic {
		if err := db.Model(&model.ContentItem{}).Where("id = ?", content.ID).Update("is_public", false).Error; err != nil {
			t.Fatalf("mark content %s private: %v", title, err)
		}
		content.IsPublic = false
	}
	return content
}

func seedCollectionRepoIP(t *testing.T, db *gorm.DB, id int64, slug, status string) model.IP {
	t.Helper()
	ip := model.IP{
		ID:     id,
		Name:   slug,
		Slug:   slug,
		Status: status,
	}
	if err := db.Create(&ip).Error; err != nil {
		t.Fatalf("create ip %s: %v", slug, err)
	}
	return ip
}

func seedCollectionRepoCollection(t *testing.T, db *gorm.DB, id, userID int64, title, zone string, isDefault, isPublic bool, sortOrder int) model.Collection {
	t.Helper()
	collection := model.Collection{
		ID:          id,
		UserID:      userID,
		Title:       title,
		Description: "",
		Zone:        zone,
		IsDefault:   isDefault,
		IsPublic:    isPublic,
		SortOrder:   sortOrder,
	}
	if err := db.Create(&collection).Error; err != nil {
		t.Fatalf("create collection %s: %v", title, err)
	}
	return collection
}

func seedCollectionRepoItem(t *testing.T, db *gorm.DB, id, collectionID, contentItemID int64, note string) model.CollectionItem {
	t.Helper()
	item := model.CollectionItem{
		ID:            id,
		CollectionID:  collectionID,
		ContentItemID: contentItemID,
		Note:          note,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create collection item %d: %v", id, err)
	}
	return item
}

func assertCollectionRepoItemTitles(t *testing.T, items []model.CollectionItem, want []string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("item len = %d, want %d; items=%#v", len(items), len(want), items)
	}
	got := make(map[string]bool, len(items))
	for _, item := range items {
		got[item.ContentItem.Title] = true
	}
	for _, title := range want {
		if !got[title] {
			t.Fatalf("missing title %q in items %#v", title, items)
		}
	}
}
