package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

func TestListReactionsReturnsAggregatesAndViewerReaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Reaction{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{ID: 7, Email: "viewer@example.com", Username: "viewer", Reputation: 10}).Error; err != nil {
		t.Fatalf("seed viewer: %v", err)
	}
	if err := db.Create(&model.User{ID: 8, Email: "other@example.com", Username: "other", Reputation: 10}).Error; err != nil {
		t.Fatalf("seed other: %v", err)
	}
	if err := db.Create(&model.User{ID: 9, Email: "third@example.com", Username: "third", Reputation: 10}).Error; err != nil {
		t.Fatalf("seed third: %v", err)
	}
	if err := db.Create(&model.Reaction{UserID: 7, TargetType: "content", TargetID: 47, Reaction: "like"}).Error; err != nil {
		t.Fatalf("seed viewer reaction: %v", err)
	}
	if err := db.Create(&model.Reaction{UserID: 8, TargetType: "content", TargetID: 47, Reaction: "like"}).Error; err != nil {
		t.Fatalf("seed other like: %v", err)
	}
	if err := db.Create(&model.Reaction{UserID: 9, TargetType: "content", TargetID: 47, Reaction: "dislike"}).Error; err != nil {
		t.Fatalf("seed third dislike: %v", err)
	}

	h := NewSocialHandler(db, &config.Config{}, nil)
	router := gin.New()
	router.GET("/reactions", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(7))
		h.ListReactions(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reactions?target_type=content&target_id=47", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Counts struct {
			Like    int64 `json:"like"`
			Dislike int64 `json:"dislike"`
		} `json:"counts"`
		ViewerReaction *string          `json:"viewer_reaction"`
		Reactions      []model.Reaction `json:"reactions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Counts.Like != 2 || payload.Counts.Dislike != 1 {
		t.Fatalf("counts = %#v, want like=2 dislike=1", payload.Counts)
	}
	if payload.ViewerReaction == nil || *payload.ViewerReaction != "like" {
		t.Fatalf("viewer_reaction = %#v, want like", payload.ViewerReaction)
	}
	if payload.Reactions != nil {
		t.Fatalf("raw reaction rows must not be returned: %#v", payload.Reactions)
	}
}

func TestListReactionsAnonymousViewerReactionIsNull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Reaction{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := NewSocialHandler(db, &config.Config{}, nil)
	router := gin.New()
	router.GET("/reactions", h.ListReactions)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/reactions?target_type=content&target_id=47", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		ViewerReaction *string `json:"viewer_reaction"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ViewerReaction != nil {
		t.Fatalf("anonymous viewer_reaction = %q, want null", *payload.ViewerReaction)
	}
}
