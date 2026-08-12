package service

import (
	"context"
	"math"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
)

func TestRecommendationCollectionItemsBuildUserProfile(t *testing.T) {
	db := setupRecommendationServiceTestDB(t)
	user := seedRecommendationUser(t, db, 1)
	content := seedRecommendationContent(t, db, 10, user.ID, "original", "article", "game")
	seedRecommendationEmbedding(t, db, content.ID, "[3,4]")
	collection := seedRecommendationCollection(t, db, user.ID, "original")
	seedRecommendationCollectionItem(t, db, collection.ID, content.ID)

	svc := &RecommendationService{db: db}
	if svc.isColdStart(user.ID, 1) {
		t.Fatal("isColdStart() = true for user with collection item interaction, want false")
	}

	profile, err := svc.buildUserProfile(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("buildUserProfile() error = %v", err)
	}
	assertRecommendationVectorApprox(t, profile, []float32{0.6, 0.8})
}

func TestRecommendationCollectionItemsBothContributeToProfile(t *testing.T) {
	db := setupRecommendationServiceTestDB(t)
	user := seedRecommendationUser(t, db, 1)
	first := seedRecommendationContent(t, db, 10, user.ID, "original", "article", "game")
	second := seedRecommendationContent(t, db, 11, user.ID, "original", "video", "game")
	seedRecommendationEmbedding(t, db, first.ID, "[1,0]")
	seedRecommendationEmbedding(t, db, second.ID, "[0,1]")
	collection := seedRecommendationCollection(t, db, user.ID, "original")
	seedRecommendationCollectionItem(t, db, collection.ID, first.ID)
	seedRecommendationCollectionItem(t, db, collection.ID, second.ID)

	svc := &RecommendationService{db: db}
	profile, err := svc.buildUserProfile(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("buildUserProfile() error = %v", err)
	}
	assertRecommendationVectorApprox(t, profile, []float32{0.70710677, 0.70710677})
}

// 收藏信号权重（x2）只由收藏成员关系承担（Testing Decision 15）。
func TestRecommendationCollectionMembershipKeepsFavoriteWeight(t *testing.T) {
	db := setupRecommendationServiceTestDB(t)
	user := seedRecommendationUser(t, db, 1)
	browsed := seedRecommendationContent(t, db, 10, user.ID, "original", "article", "game")
	favorited := seedRecommendationContent(t, db, 11, user.ID, "original", "template", "efficiency")
	seedRecommendationEmbedding(t, db, browsed.ID, "[1,0]")
	seedRecommendationEmbedding(t, db, favorited.ID, "[0,1]")
	seedRecommendationBrowseHistory(t, db, user.ID, browsed.ID)
	collection := seedRecommendationCollection(t, db, user.ID, "original")
	seedRecommendationCollectionItem(t, db, collection.ID, favorited.ID)

	svc := &RecommendationService{db: db}
	collectionProfile, err := svc.buildUserProfile(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("collection buildUserProfile() error = %v", err)
	}
	assertRecommendationVectorApprox(t, collectionProfile, []float32{0.4472136, 0.8944272})
}

func setupRecommendationServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.BrowseHistory{}, &model.Reaction{}, &model.Collection{}, &model.CollectionItem{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Exec(`CREATE TABLE content_embeddings (
		content_item_id INTEGER PRIMARY KEY,
		embedding TEXT NOT NULL,
		embedded_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create content_embeddings: %v", err)
	}
	return db
}

func seedRecommendationUser(t *testing.T, db *gorm.DB, id int64) model.User {
	t.Helper()

	user := model.User{
		ID:           id,
		Email:        "rec-user@example.com",
		Username:     "rec-user",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedRecommendationContent(t *testing.T, db *gorm.DB, id, authorID int64, zone, contentType, category string) model.ContentItem {
	t.Helper()

	content := model.ContentItem{
		ID:          id,
		Title:       "Recommendation content",
		AuthorID:    authorID,
		Zone:        zone,
		ContentType: contentType,
		Category:    category,
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}
	return content
}

func seedRecommendationEmbedding(t *testing.T, db *gorm.DB, contentID int64, embedding string) {
	t.Helper()

	if err := db.Exec("INSERT INTO content_embeddings (content_item_id, embedding) VALUES (?, ?)", contentID, embedding).Error; err != nil {
		t.Fatalf("seed embedding: %v", err)
	}
}

func seedRecommendationBrowseHistory(t *testing.T, db *gorm.DB, userID, contentID int64) {
	t.Helper()

	if err := db.Create(&model.BrowseHistory{UserID: userID, ContentItemID: contentID}).Error; err != nil {
		t.Fatalf("seed browse history: %v", err)
	}
}

func seedRecommendationCollection(t *testing.T, db *gorm.DB, userID int64, zone string) model.Collection {
	t.Helper()

	collection := model.Collection{
		UserID:    userID,
		Title:     "Recommendation collection",
		Zone:      zone,
		IsPublic:  true,
		SortOrder: 1,
	}
	if err := db.Create(&collection).Error; err != nil {
		t.Fatalf("seed collection: %v", err)
	}
	return collection
}

func seedRecommendationCollectionItem(t *testing.T, db *gorm.DB, collectionID, contentID int64) {
	t.Helper()

	if err := db.Create(&model.CollectionItem{CollectionID: collectionID, ContentItemID: contentID}).Error; err != nil {
		t.Fatalf("seed collection item: %v", err)
	}
}

func assertRecommendationVectorApprox(t *testing.T, got, want []float32) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("vector length = %d, want %d; got=%v", len(got), len(want), got)
	}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 0.0001 {
			t.Fatalf("vector[%d] = %.8f, want %.8f; got=%v", i, got[i], want[i], got)
		}
	}
}
