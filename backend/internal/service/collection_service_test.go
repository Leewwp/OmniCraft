package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
)

func TestFavoriteCompatibilityFavoriteKeepsLegacyRowAndAddsDefaultCollectionItem(t *testing.T) {
	db := setupCollectionServiceDB(t)
	user := seedCollectionServiceUser(t, db, 10, "favorite-user")
	author := seedCollectionServiceUser(t, db, 20, "favorite-author")
	content := seedCollectionServiceContent(t, db, 100, author.ID, "fanwork", "image")
	svc := NewSocialService(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		&config.Config{},
	)

	if err := svc.Favorite(user.ID, content.ID); err != nil {
		t.Fatalf("Favorite() error = %v", err)
	}
	if err := svc.Favorite(user.ID, content.ID); err != nil {
		t.Fatalf("Favorite() duplicate error = %v", err)
	}

	assertCollectionServiceFavoriteCount(t, db, user.ID, content.ID, 1)
	defaultCollection := loadCollectionServiceDefaultCollection(t, db, user.ID, "fanwork")
	assertCollectionServiceCollectionItemCount(t, db, defaultCollection.ID, content.ID, 1)

	var originalDefaults int64
	if err := db.Model(&model.Collection{}).
		Where("user_id = ? AND zone = ? AND is_default = ? AND deleted_at IS NULL", user.ID, "original", true).
		Count(&originalDefaults).Error; err != nil {
		t.Fatalf("count original defaults: %v", err)
	}
	if originalDefaults != 0 {
		t.Fatalf("original default collections = %d, want 0 for fanwork favorite", originalDefaults)
	}
}

func TestFavoriteCompatibilityFavoriteRollsBackLegacyRowWhenDefaultWriteFails(t *testing.T) {
	db := setupCollectionServiceDB(t)
	user := seedCollectionServiceUser(t, db, 10, "favorite-rollback-user")
	missingContentID := int64(999)
	svc := NewSocialService(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		&config.Config{},
	)

	if err := svc.Favorite(user.ID, missingContentID); err == nil {
		t.Fatal("Favorite() error = nil, want default collection write failure")
	}

	assertCollectionServiceFavoriteCount(t, db, user.ID, missingContentID, 0)
}

func TestFavoriteCompatibilityUnfavoriteRemovesLegacyRowAndDefaultItemOnly(t *testing.T) {
	db := setupCollectionServiceDB(t)
	user := seedCollectionServiceUser(t, db, 10, "unfavorite-user")
	author := seedCollectionServiceUser(t, db, 20, "unfavorite-author")
	content := seedCollectionServiceContent(t, db, 100, author.ID, "original", "article")
	defaultCollection := seedCollectionServiceCollection(t, db, 200, user.ID, "default original", "original", true, false, 0)
	customCollection := seedCollectionServiceCollection(t, db, 201, user.ID, "custom original", "original", false, false, 1)
	seedCollectionServiceFavorite(t, db, user.ID, content.ID)
	seedCollectionServiceCollectionItem(t, db, defaultCollection.ID, content.ID)
	seedCollectionServiceCollectionItem(t, db, customCollection.ID, content.ID)
	svc := NewSocialService(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		&config.Config{},
	)

	if err := svc.Unfavorite(user.ID, content.ID); err != nil {
		t.Fatalf("Unfavorite() error = %v", err)
	}

	assertCollectionServiceFavoriteCount(t, db, user.ID, content.ID, 0)
	assertCollectionServiceCollectionItemCount(t, db, defaultCollection.ID, content.ID, 0)
	assertCollectionServiceCollectionItemCount(t, db, customCollection.ID, content.ID, 1)
}

func TestFavoriteCompatibilityUnfavoriteRollsBackLegacyDeleteWhenDefaultDeleteFails(t *testing.T) {
	db := setupCollectionServiceDBWithoutCollectionItems(t)
	user := seedCollectionServiceUser(t, db, 10, "unfavorite-rollback-user")
	author := seedCollectionServiceUser(t, db, 20, "unfavorite-rollback-author")
	content := seedCollectionServiceContent(t, db, 100, author.ID, "original", "article")
	seedCollectionServiceFavorite(t, db, user.ID, content.ID)
	svc := NewSocialService(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		&config.Config{},
	)

	if err := svc.Unfavorite(user.ID, content.ID); err == nil {
		t.Fatal("Unfavorite() error = nil, want default collection delete failure")
	}

	assertCollectionServiceFavoriteCount(t, db, user.ID, content.ID, 1)
}

func TestCollectionServiceAddItemClearsRecommendationCache(t *testing.T) {
	db := setupCollectionServiceDB(t)
	user := seedCollectionServiceUser(t, db, 10, "collection-user")
	author := seedCollectionServiceUser(t, db, 20, "collection-author")
	content := seedCollectionServiceContent(t, db, 100, author.ID, "original", "article")
	collection := seedCollectionServiceCollection(t, db, 200, user.ID, "custom original", "original", false, false, 1)
	mr := miniredis.RunT(t)
	restoreRedis := useCollectionServiceRedis(t, mr)
	defer restoreRedis()
	ctx := context.Background()
	userCacheKey := "rec:original:10:test"
	otherCacheKey := "rec:original:99:test"
	if err := redisclient.Client.Set(ctx, userCacheKey, "cached", 0).Err(); err != nil {
		t.Fatalf("seed user rec cache: %v", err)
	}
	if err := redisclient.Client.Set(ctx, otherCacheKey, "cached", 0).Err(); err != nil {
		t.Fatalf("seed other rec cache: %v", err)
	}
	svc := NewCollectionService(repository.NewCollectionRepository(db), repository.NewContentRepository(db))

	if _, err := svc.AddItem(ctx, collection.ID, user.ID, content.ID, "keeper"); err != nil {
		t.Fatalf("AddItem() error = %v", err)
	}

	if mr.Exists(userCacheKey) {
		t.Fatalf("user recommendation cache key %q still exists after collection add", userCacheKey)
	}
	if !mr.Exists(otherCacheKey) {
		t.Fatalf("other user recommendation cache key %q was removed", otherCacheKey)
	}
}

func setupCollectionServiceDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.Favorite{}, &model.Collection{}, &model.CollectionItem{}); err != nil {
		t.Fatalf("migrate collection service models: %v", err)
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

func setupCollectionServiceDBWithoutCollectionItems(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.Favorite{}, &model.Collection{}); err != nil {
		t.Fatalf("migrate collection service models without items: %v", err)
	}
	return db
}

func seedCollectionServiceUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
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

func seedCollectionServiceContent(t *testing.T, db *gorm.DB, id, authorID int64, zone, contentType string) model.ContentItem {
	t.Helper()
	content := model.ContentItem{
		ID:          id,
		Title:       zone + " content",
		AuthorID:    authorID,
		Zone:        zone,
		Category:    "game",
		ContentType: contentType,
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content %s: %v", zone, err)
	}
	return content
}

func seedCollectionServiceCollection(t *testing.T, db *gorm.DB, id, userID int64, title, zone string, isDefault, isPublic bool, sortOrder int) model.Collection {
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

func seedCollectionServiceFavorite(t *testing.T, db *gorm.DB, userID, contentID int64) {
	t.Helper()
	if err := db.Create(&model.Favorite{UserID: userID, ContentItemID: contentID}).Error; err != nil {
		t.Fatalf("create favorite user=%d content=%d: %v", userID, contentID, err)
	}
}

func seedCollectionServiceCollectionItem(t *testing.T, db *gorm.DB, collectionID, contentID int64) model.CollectionItem {
	t.Helper()
	item := model.CollectionItem{
		CollectionID:  collectionID,
		ContentItemID: contentID,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create collection item collection=%d content=%d: %v", collectionID, contentID, err)
	}
	return item
}

func loadCollectionServiceDefaultCollection(t *testing.T, db *gorm.DB, userID int64, zone string) model.Collection {
	t.Helper()
	var collection model.Collection
	if err := db.Where("user_id = ? AND zone = ? AND is_default = ? AND deleted_at IS NULL", userID, zone, true).
		First(&collection).Error; err != nil {
		t.Fatalf("load default collection user=%d zone=%s: %v", userID, zone, err)
	}
	return collection
}

func assertCollectionServiceFavoriteCount(t *testing.T, db *gorm.DB, userID, contentID, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.Favorite{}).
		Where("user_id = ? AND content_item_id = ?", userID, contentID).
		Count(&count).Error; err != nil {
		t.Fatalf("count favorites user=%d content=%d: %v", userID, contentID, err)
	}
	if count != want {
		t.Fatalf("favorite count user=%d content=%d = %d, want %d", userID, contentID, count, want)
	}
}

func assertCollectionServiceCollectionItemCount(t *testing.T, db *gorm.DB, collectionID, contentID, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(&model.CollectionItem{}).
		Where("collection_id = ? AND content_item_id = ?", collectionID, contentID).
		Count(&count).Error; err != nil {
		t.Fatalf("count collection items collection=%d content=%d: %v", collectionID, contentID, err)
	}
	if count != want {
		t.Fatalf("collection item count collection=%d content=%d = %d, want %d", collectionID, contentID, count, want)
	}
}

func useCollectionServiceRedis(t *testing.T, mr *miniredis.Miniredis) func() {
	t.Helper()
	previous := redisclient.Client
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisclient.Client = rdb
	return func() {
		redisclient.Client = previous
		_ = rdb.Close()
	}
}
