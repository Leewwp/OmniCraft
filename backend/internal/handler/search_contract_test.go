package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func TestSearchContentsRouteAppliesContentTypeAndTimeRangeFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.ContentTag{}, &model.IP{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	author := model.User{
		Email:        "search-route-author@example.com",
		Username:     "search-route-author",
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}

	recentImage := model.ContentItem{
		Title:       "Route Needle recent image",
		AuthorID:    author.ID,
		Zone:        "original",
		Category:    "game",
		ContentType: "image",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&recentImage).Error; err != nil {
		t.Fatalf("create recent image: %v", err)
	}

	oldImage := model.ContentItem{
		Title:       "Route Needle old image",
		AuthorID:    author.ID,
		Zone:        "original",
		Category:    "game",
		ContentType: "image",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
		CreatedAt:   time.Now().AddDate(0, 0, -12),
		UpdatedAt:   time.Now().AddDate(0, 0, -12),
	}
	if err := db.Create(&oldImage).Error; err != nil {
		t.Fatalf("create old image: %v", err)
	}

	recentArticle := model.ContentItem{
		Title:       "Route Needle recent article",
		AuthorID:    author.ID,
		Zone:        "original",
		Category:    "game",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&recentArticle).Error; err != nil {
		t.Fatalf("create recent article: %v", err)
	}

	handler := NewSearchHandler(service.NewSearchService(repository.NewSearchRepository(db), nil), &config.Config{})
	router := gin.New()
	router.GET("/api/v1/contents/search", handler.SearchContents)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contents/search?q=Needle&content_type=image&time_range=week", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Items []struct {
			Title       string `json:"title"`
			ContentType string `json:"content_type"`
		} `json:"items"`
		Total     int64  `json:"total"`
		TimeRange string `json:"time_range"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}

	if body.TimeRange != "week" {
		t.Fatalf("time_range = %q, want week", body.TimeRange)
	}
	if body.Total != 1 {
		t.Fatalf("total = %d, want 1", body.Total)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(body.Items))
	}
	if body.Items[0].Title != "Route Needle recent image" {
		t.Fatalf("item title = %q, want Route Needle recent image", body.Items[0].Title)
	}
	if body.Items[0].ContentType != "image" {
		t.Fatalf("item content_type = %q, want image", body.Items[0].ContentType)
	}
}
