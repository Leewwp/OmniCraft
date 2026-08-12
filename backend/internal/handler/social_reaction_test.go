package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

type reactionPayload struct {
	Counts struct {
		Like    int64 `json:"like"`
		Dislike int64 `json:"dislike"`
	} `json:"counts"`
	ViewerReaction *string          `json:"viewer_reaction"`
	Action         string           `json:"action"`
	Reactions      []model.Reaction `json:"reactions"`
}

func newReactionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Reaction{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedReactionUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, u := range []model.User{
		{ID: 7, Email: "viewer@example.com", Username: "viewer", Reputation: 10},
		{ID: 8, Email: "other@example.com", Username: "other", Reputation: 10},
		{ID: 9, Email: "third@example.com", Username: "third", Reputation: 10},
		{ID: 10, Email: "lowrep@example.com", Username: "lowrep", Reputation: 2},
	} {
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed user %d: %v", u.ID, err)
		}
	}
}

func newReactionRouter(h *SocialHandler) *gin.Engine {
	router := gin.New()
	router.POST("/reactions", func(c *gin.Context) {
		setTestUser(c)
		h.React(c)
	})
	router.GET("/reactions", func(c *gin.Context) {
		setTestUser(c)
		h.ListReactions(c)
	})
	return router
}

func setTestUser(c *gin.Context) {
	if raw := c.GetHeader("X-Test-User"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			c.Set(middleware.UserIDKey, id)
		}
	}
}

func postReaction(t *testing.T, router *gin.Engine, userID int64, targetType string, targetID int64, reaction string) reactionPayload {
	t.Helper()
	rec := httptest.NewRecorder()
	payload := `{"target_type":"` + targetType + `","target_id":` + strconv.FormatInt(targetID, 10) + `,"reaction":"` + reaction + `"}`
	req := httptest.NewRequest(http.MethodPost, "/reactions", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", strconv.FormatInt(userID, 10))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out reactionPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func getReactions(t *testing.T, router *gin.Engine, userID int64, targetType string, targetID int64) reactionPayload {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reactions?target_type="+targetType+"&target_id="+strconv.FormatInt(targetID, 10), nil)
	req.Header.Set("X-Test-User", strconv.FormatInt(userID, 10))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out reactionPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

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

func TestReactCreateSetsViewerReactionAndCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newReactionTestDB(t)
	seedReactionUsers(t, db)
	h := NewSocialHandler(db, &config.Config{}, nil)
	router := newReactionRouter(h)

	out := postReaction(t, router, 7, "content", 47, "like")
	if out.Action != "created" {
		t.Fatalf("action = %q, want created", out.Action)
	}
	if out.ViewerReaction == nil || *out.ViewerReaction != "like" {
		t.Fatalf("viewer_reaction = %#v, want like", out.ViewerReaction)
	}
	if out.Counts.Like != 1 || out.Counts.Dislike != 0 {
		t.Fatalf("counts = %#v, want like=1 dislike=0", out.Counts)
	}

	readBack := getReactions(t, router, 7, "content", 47)
	if readBack.ViewerReaction == nil || *readBack.ViewerReaction != "like" {
		t.Fatalf("read-back viewer_reaction = %#v, want like", readBack.ViewerReaction)
	}
	if readBack.Counts.Like != 1 || readBack.Counts.Dislike != 0 {
		t.Fatalf("read-back counts = %#v, want like=1 dislike=0", readBack.Counts)
	}
}

func TestReactRepeatRemovesReaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newReactionTestDB(t)
	seedReactionUsers(t, db)
	h := NewSocialHandler(db, &config.Config{}, nil)
	router := newReactionRouter(h)

	if out := postReaction(t, router, 7, "content", 47, "like"); out.Action != "created" {
		t.Fatalf("first action = %q, want created", out.Action)
	}
	out := postReaction(t, router, 7, "content", 47, "like")
	if out.Action != "removed" {
		t.Fatalf("second action = %q, want removed", out.Action)
	}
	if out.ViewerReaction != nil {
		t.Fatalf("viewer_reaction = %#v, want null after cancel", *out.ViewerReaction)
	}
	if out.Counts.Like != 0 || out.Counts.Dislike != 0 {
		t.Fatalf("counts = %#v, want like=0 dislike=0", out.Counts)
	}

	readBack := getReactions(t, router, 7, "content", 47)
	if readBack.ViewerReaction != nil || readBack.Counts.Like != 0 {
		t.Fatalf("read-back = %#v, want neutral", readBack)
	}
}

func TestReactSwitchUpdatesAtomically(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newReactionTestDB(t)
	seedReactionUsers(t, db)
	h := NewSocialHandler(db, &config.Config{}, nil)
	router := newReactionRouter(h)

	if out := postReaction(t, router, 7, "content", 47, "like"); out.Action != "created" {
		t.Fatalf("like action = %q, want created", out.Action)
	}
	out := postReaction(t, router, 7, "content", 47, "dislike")
	if out.Action != "updated" {
		t.Fatalf("switch action = %q, want updated", out.Action)
	}
	if out.ViewerReaction == nil || *out.ViewerReaction != "dislike" {
		t.Fatalf("viewer_reaction = %#v, want dislike", out.ViewerReaction)
	}
	if out.Counts.Like != 0 || out.Counts.Dislike != 1 {
		t.Fatalf("counts = %#v, want like=0 dislike=1", out.Counts)
	}

	var rowCount int64
	if err := db.Model(&model.Reaction{}).Where("user_id = 7 AND target_type = ? AND target_id = 47", "content").Count(&rowCount).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("row count = %d, want 1 (mutual exclusion)", rowCount)
	}

	readBack := getReactions(t, router, 7, "content", 47)
	if readBack.ViewerReaction == nil || *readBack.ViewerReaction != "dislike" {
		t.Fatalf("read-back viewer_reaction = %#v, want dislike", readBack.ViewerReaction)
	}
	if readBack.Counts.Like != 0 || readBack.Counts.Dislike != 1 {
		t.Fatalf("read-back counts = %#v, want like=0 dislike=1", readBack.Counts)
	}

	out = postReaction(t, router, 7, "content", 47, "like")
	if out.Action != "updated" || out.ViewerReaction == nil || *out.ViewerReaction != "like" {
		t.Fatalf("switch-back = %#v, want updated/like", out)
	}
	if out.Counts.Like != 1 || out.Counts.Dislike != 0 {
		t.Fatalf("switch-back counts = %#v, want like=1 dislike=0", out.Counts)
	}
}

func TestReactAnonymousUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newReactionTestDB(t)
	h := NewSocialHandler(db, &config.Config{}, nil)
	router := newReactionRouter(h)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/reactions", strings.NewReader(`{"target_type":"content","target_id":47,"reaction":"like"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
}

func TestReactLowReputationForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newReactionTestDB(t)
	seedReactionUsers(t, db)
	h := NewSocialHandler(db, &config.Config{}, nil)
	router := newReactionRouter(h)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/reactions", strings.NewReader(`{"target_type":"content","target_id":47,"reaction":"like"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", "10")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", rec.Code, rec.Body.String())
	}
}

func TestListReactionsValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newReactionTestDB(t)
	h := NewSocialHandler(db, &config.Config{}, nil)
	router := newReactionRouter(h)

	for _, tc := range []struct {
		name  string
		query string
	}{
		{"missing target_type", "/reactions?target_id=47"},
		{"missing target_id", "/reactions?target_type=content"},
		{"invalid target_id", "/reactions?target_type=content&target_id=abc"},
		{"invalid target_type", "/reactions?target_type=fanwork&target_id=47"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.query, nil))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}
