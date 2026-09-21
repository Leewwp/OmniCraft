package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
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

// FR-01（低-18）：access token 黑名单键不得以明文形态保存原始 JWT。
// 行为链：签发 → Logout → IsTokenBlacklisted 命中 + 全键空间无原始 token 明文。
func TestLogoutBlacklistNeverStoresRawAccessToken(t *testing.T) {
	if testing.Short() {
		t.Skip("miniredis needed")
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	db := setupAuthServiceUserDB(t)
	userRepo := repository.NewUserRepository(db)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "blacklist-test-secret", AccessTokenTTL: 120, RefreshTokenTTL: 7},
	}
	authService := NewAuthService(userRepo, rdb, cfg)

	user := &model.User{
		Email:        "blacklist@example.test",
		Username:     "blacklist-user",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	pair, err := authService.IssueTokenPairForUser(user)
	if err != nil {
		t.Fatalf("issue token pair: %v", err)
	}

	if err := authService.Logout(pair.AccessToken); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if !authService.IsTokenBlacklisted(pair.AccessToken) {
		t.Fatal("access token must be blacklisted after logout")
	}

	ctx := context.Background()
	keys, err := rdb.Keys(ctx, "*").Result()
	if err != nil {
		t.Fatalf("scan keys: %v", err)
	}
	for _, key := range keys {
		if strings.Contains(key, pair.AccessToken) {
			t.Fatalf("redis key %q contains the raw access token; blacklist keys must store a digest", key)
		}
	}
}
