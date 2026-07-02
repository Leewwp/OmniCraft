package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

func TestAuthServiceCreateUserFromPendingEnsuresDefaultCollections(t *testing.T) {
	db := setupAuthServiceCollectionDB(t)
	userRepo := repository.NewUserRepository(db)
	collectionRepo := repository.NewCollectionRepository(db)
	authService := NewAuthService(userRepo, nil, &config.Config{})
	authService.SetCollectionRepository(collectionRepo)

	user, err := authService.CreateUserFromPending(&PendingRegistration{
		Email:           "verified@example.test",
		Username:        "verified-user",
		PasswordHash:    "hash",
		Reputation:      10,
		Role:            "user",
		PreferredLocale: "zh-CN",
	})

	if err != nil {
		t.Fatalf("CreateUserFromPending() error = %v", err)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("EmailVerifiedAt is nil, want verified user")
	}

	var collections []model.Collection
	if err := db.Where("user_id = ? AND is_default = ? AND deleted_at IS NULL", user.ID, true).
		Order("zone ASC").
		Find(&collections).Error; err != nil {
		t.Fatalf("load default collections: %v", err)
	}
	if len(collections) != 2 {
		t.Fatalf("default collections len = %d, want 2; collections=%#v", len(collections), collections)
	}

	got := map[string]model.Collection{}
	for _, collection := range collections {
		got[collection.Zone] = collection
	}
	assertAuthServiceDefaultCollection(t, got["fanwork"], "\u9ed8\u8ba4\u4e8c\u521b\u6536\u85cf")
	assertAuthServiceDefaultCollection(t, got["original"], "\u9ed8\u8ba4\u539f\u521b\u6536\u85cf")
}

func TestAuthServiceCreateUserFromPendingDoesNotFailWhenDefaultCollectionsFail(t *testing.T) {
	db := setupAuthServiceUserDB(t)
	userRepo := repository.NewUserRepository(db)
	authService := NewAuthService(userRepo, nil, &config.Config{})
	authService.SetCollectionRepository(failingDefaultCollectionEnsurer{})
	restoreLogger := silenceAuthServiceLogger()
	defer restoreLogger()

	user, err := authService.CreateUserFromPending(&PendingRegistration{
		Email:           "fallback@example.test",
		Username:        "fallback-user",
		PasswordHash:    "hash",
		Reputation:      10,
		Role:            "user",
		PreferredLocale: "zh-CN",
	})

	if err != nil {
		t.Fatalf("CreateUserFromPending() error = %v, want user creation to remain non-fatal", err)
	}
	if user.ID == 0 {
		t.Fatal("user ID = 0, want created user despite default collection failure")
	}
}

type failingDefaultCollectionEnsurer struct{}

func (failingDefaultCollectionEnsurer) EnsureDefaultCollection(context.Context, int64, string) (*model.Collection, error) {
	return nil, errors.New("default collection failure")
}

func setupAuthServiceCollectionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupAuthServiceUserDB(t)
	if err := db.AutoMigrate(&model.ContentItem{}, &model.Collection{}, &model.CollectionItem{}); err != nil {
		t.Fatalf("migrate collection models: %v", err)
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

func setupAuthServiceUserDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user model: %v", err)
	}
	return db
}

func assertAuthServiceDefaultCollection(t *testing.T, collection model.Collection, title string) {
	t.Helper()
	if collection.ID == 0 {
		t.Fatalf("missing default collection for zone")
	}
	if collection.Title != title {
		t.Fatalf("default title for zone %s = %q, want %q", collection.Zone, collection.Title, title)
	}
	if !collection.IsDefault || collection.IsPublic || collection.SortOrder != 0 {
		t.Fatalf("default collection = %#v, want default/private/sort_order 0", collection)
	}
}

func silenceAuthServiceLogger() func() {
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return func() {
		slog.SetDefault(previous)
	}
}
