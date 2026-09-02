package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func setupTrendingRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.IP{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	author := model.User{
		Email:        "trending-author@example.com",
		Username:     "trending-author",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	bannedAuthor := model.User{
		Email:        "trending-banned@example.com",
		Username:     "trending-banned",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
		IsBanned:     true,
	}
	if err := db.Create(&bannedAuthor).Error; err != nil {
		t.Fatalf("create banned author: %v", err)
	}

	now := time.Now()
	contents := []model.ContentItem{
		{Title: "Trending visible published", AuthorID: author.ID, Zone: "original", ContentType: "image", Status: "published", IsPublic: true, AllowCopy: true},
		{Title: "Trending banned content", AuthorID: author.ID, Zone: "original", ContentType: "image", Status: "banned", IsPublic: true, AllowCopy: true},
		{Title: "Trending under review", AuthorID: author.ID, Zone: "original", ContentType: "image", Status: "under_review", IsPublic: true, AllowCopy: true},
		{Title: "Trending private", AuthorID: author.ID, Zone: "original", ContentType: "image", Status: "published", IsPublic: false, AllowCopy: true},
		{Title: "Trending deleted", AuthorID: author.ID, Zone: "original", ContentType: "image", Status: "published", IsPublic: true, AllowCopy: true, DeletedAt: &now},
		{Title: "Trending banned author", AuthorID: bannedAuthor.ID, Zone: "original", ContentType: "image", Status: "published", IsPublic: true, AllowCopy: true},
	}
	for i := range contents {
		if err := db.Create(&contents[i]).Error; err != nil {
			t.Fatalf("create content %d: %v", i, db.Error)
		}
	}
	// GORM omits zero-value fields with a default tag on insert, so flip the
	// private fixture after creation (same approach as the visibility tests).
	if err := db.Model(&model.ContentItem{}).Where("id = ?", contents[3].ID).Update("is_public", false).Error; err != nil {
		t.Fatalf("mark private: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	// Hot rank members are content IDs (writers: likes/views/rebuild).
	scores := []float64{90, 80, 70, 60, 50, 40}
	for i, c := range contents {
		if err := rdb.ZAdd(t.Context(), "rank:hot:contents", redis.Z{Score: scores[i], Member: c.ID}).Err(); err != nil {
			t.Fatalf("zadd: %v", err)
		}
	}

	handler := NewSearchHandler(service.NewSearchService(repository.NewSearchRepository(db), rdb), &config.Config{})
	router := gin.New()
	router.GET("/api/v1/search/trending", handler.Trending)
	return router
}

func TestSearchTrendingContractResolvesTitlesAndFiltersVisibility(t *testing.T) {
	router := setupTrendingRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/trending", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Trending []struct {
			Text      string `json:"text"`
			Score     int64  `json:"score"`
			ContentID int64  `json:"content_id"`
		} `json:"trending"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}

	if len(body.Trending) != 1 {
		t.Fatalf("len(trending) = %d, want 1 (only the visible published item); body = %s", len(body.Trending), rec.Body.String())
	}
	item := body.Trending[0]
	if item.Text != "Trending visible published" {
		t.Fatalf("text = %q, want real content title", item.Text)
	}
	if item.Score != 90 {
		t.Fatalf("score = %d, want 90", item.Score)
	}
	if item.ContentID == 0 {
		t.Fatalf("content_id missing from trending contract")
	}
}

func TestSearchTrendingContractToleratesEmptyRank(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.IP{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	handler := NewSearchHandler(service.NewSearchService(repository.NewSearchRepository(db), rdb), &config.Config{})
	router := gin.New()
	router.GET("/api/v1/search/trending", handler.Trending)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/trending", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Trending []json.RawMessage `json:"trending"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Trending == nil {
		t.Fatalf("trending = null, want empty array for graceful degradation")
	}
	if len(body.Trending) != 0 {
		t.Fatalf("len(trending) = %d, want 0", len(body.Trending))
	}
}
