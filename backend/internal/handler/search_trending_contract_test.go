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
		// #781: visible fanwork member so zone filtering is observable.
		{Title: "Trending fanwork visible", AuthorID: author.ID, Zone: "fanwork", ContentType: "image", Status: "published", IsPublic: true, AllowCopy: true},
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
	scores := []float64{90, 80, 70, 60, 50, 40, 85}
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

type trendingBody struct {
	Trending []struct {
		Text      string `json:"text"`
		Score     int64  `json:"score"`
		ContentID int64  `json:"content_id"`
	} `json:"trending"`
}

func getTrending(t *testing.T, router *gin.Engine, path string) trendingBody {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200; body = %s", path, rec.Code, rec.Body.String())
	}
	var body trendingBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response for %s: %v; body = %s", path, err, rec.Body.String())
	}
	return body
}

func TestSearchTrendingContractResolvesTitlesAndFiltersVisibility(t *testing.T) {
	router := setupTrendingRouter(t)

	// No zone param: legacy site-wide mixed window (#781 keeps this behavior).
	body := getTrending(t, router, "/api/v1/search/trending")

	if len(body.Trending) != 2 {
		t.Fatalf("len(trending) = %d, want 2 (visible published original + fanwork)", len(body.Trending))
	}
	if body.Trending[0].Text != "Trending visible published" {
		t.Fatalf("text[0] = %q, want real content title", body.Trending[0].Text)
	}
	if body.Trending[0].Score != 90 {
		t.Fatalf("score[0] = %d, want 90", body.Trending[0].Score)
	}
	if body.Trending[0].ContentID == 0 {
		t.Fatalf("content_id missing from trending contract")
	}
	if body.Trending[1].Text != "Trending fanwork visible" {
		t.Fatalf("text[1] = %q, want visible fanwork in mixed ranking", body.Trending[1].Text)
	}
}

func TestSearchTrendingContractZoneFilterReturnsOnlyRequestedZone(t *testing.T) {
	router := setupTrendingRouter(t)

	original := getTrending(t, router, "/api/v1/search/trending?zone=original")
	if len(original.Trending) != 1 {
		t.Fatalf("zone=original len(trending) = %d, want 1 (fanwork member filtered out)", len(original.Trending))
	}
	if original.Trending[0].Text != "Trending visible published" {
		t.Fatalf("zone=original text = %q, want the original-zone member", original.Trending[0].Text)
	}

	fanwork := getTrending(t, router, "/api/v1/search/trending?zone=fanwork")
	if len(fanwork.Trending) != 1 {
		t.Fatalf("zone=fanwork len(trending) = %d, want 1", len(fanwork.Trending))
	}
	if fanwork.Trending[0].Text != "Trending fanwork visible" {
		t.Fatalf("zone=fanwork text = %q, want the fanwork member", fanwork.Trending[0].Text)
	}
}

// TestSearchTrendingContractZoneFilterFillsBeyondScaledWindow covers the #781
// fetch-window fix: with limit=5 the legacy over-fetch window is limit*3 = 15
// members. Fifteen visible fanworks at the top of the rank would exhaust that
// window before the single visible original (rank 16) is reached, yielding an
// empty original-zone list — the zone path therefore fetches the full 300
// ceiling and still resolves the out-of-window original.
func TestSearchTrendingContractZoneFilterFillsBeyondScaledWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.IP{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	author := model.User{
		Email:        "trending-window-author@example.com",
		Username:     "trending-window-author",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = rdb.Close() })

	// 15 visible fanworks (scores 200..186) fill the legacy limit*3 window,
	// then one visible original sits at rank 16 (score 100).
	for i := 0; i < 15; i++ {
		content := model.ContentItem{
			Title:       "Window fanwork filler",
			AuthorID:    author.ID,
			Zone:        "fanwork",
			ContentType: "image",
			Status:      "published",
			IsPublic:    true,
			AllowCopy:   true,
		}
		if err := db.Create(&content).Error; err != nil {
			t.Fatalf("create fanwork %d: %v", i, err)
		}
		if err := rdb.ZAdd(t.Context(), "rank:hot:contents", redis.Z{Score: float64(200 - i), Member: content.ID}).Err(); err != nil {
			t.Fatalf("zadd fanwork %d: %v", i, err)
		}
	}
	original := model.ContentItem{
		Title:       "Window original beyond scaled rank",
		AuthorID:    author.ID,
		Zone:        "original",
		ContentType: "image",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&original).Error; err != nil {
		t.Fatalf("create original: %v", err)
	}
	if err := rdb.ZAdd(t.Context(), "rank:hot:contents", redis.Z{Score: 100, Member: original.ID}).Err(); err != nil {
		t.Fatalf("zadd original: %v", err)
	}

	handler := NewSearchHandler(service.NewSearchService(repository.NewSearchRepository(db), rdb), &config.Config{})
	router := gin.New()
	router.GET("/api/v1/search/trending", handler.Trending)

	body := getTrending(t, router, "/api/v1/search/trending?zone=original&limit=5")
	if len(body.Trending) != 1 {
		t.Fatalf("zone=original len(trending) = %d, want 1 (out-of-scaled-window original must be resolved)", len(body.Trending))
	}
	if body.Trending[0].Text != "Window original beyond scaled rank" {
		t.Fatalf("text = %q, want the rank-16 original", body.Trending[0].Text)
	}
	if body.Trending[0].ContentID != original.ID {
		t.Fatalf("content_id = %d, want %d", body.Trending[0].ContentID, original.ID)
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
