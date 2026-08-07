package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
	"omnicraft/backend/internal/pkg/rediskeys"
)

func TestCreateContentRoutePublishesFanworkAndReadsBackSourceRelation(t *testing.T) {
	router, db, token, _ := setupPublishRoute(t, publishRouteUserState{Verified: true, Reputation: 10})
	source := createPublishSourceOriginal(t, db, "Published original", "published")

	body := `{
		"title":"Fanwork with source",
		"zone":"fanwork",
		"content_type":"article",
		"is_public":true,
		"allow_copy":true,
		"source_original_id":` + strconv.FormatInt(source.ID, 10) + `
	}`

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var created struct {
		Content struct {
			ID               int64  `json:"id"`
			Zone             string `json:"zone"`
			SourceOriginalID *int64 `json:"source_original_id"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v; body = %s", err, rec.Body.String())
	}
	if created.Content.Zone != "fanwork" {
		t.Fatalf("zone = %q, want fanwork", created.Content.Zone)
	}
	if created.Content.SourceOriginalID == nil || *created.Content.SourceOriginalID != source.ID {
		t.Fatalf("source_original_id = %#v, want %d", created.Content.SourceOriginalID, source.ID)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+strconv.FormatInt(created.Content.ID, 10), nil)
	router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body = %s", getRec.Code, getRec.Body.String())
	}

	var loaded struct {
		Content struct {
			SourceOriginalID *int64 `json:"source_original_id"`
		} `json:"content"`
		SourceOriginal struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"source_original"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &loaded); err != nil {
		t.Fatalf("decode get response: %v; body = %s", err, getRec.Body.String())
	}
	if loaded.Content.SourceOriginalID == nil || *loaded.Content.SourceOriginalID != source.ID {
		t.Fatalf("loaded source_original_id = %#v, want %d", loaded.Content.SourceOriginalID, source.ID)
	}
	if loaded.SourceOriginal.ID != source.ID || loaded.SourceOriginal.Title != source.Title {
		t.Fatalf("source_original = %#v, want id=%d title=%q", loaded.SourceOriginal, source.ID, source.Title)
	}
}

func TestCreateContentRouteRejectsInvalidSourceOriginal(t *testing.T) {
	router, db, token, _ := setupPublishRoute(t, publishRouteUserState{Verified: true, Reputation: 10})
	nonOriginal := createPublishContent(t, db, "Fanwork source candidate", "fanwork", "published")
	unpublishedOriginal := createPublishSourceOriginal(t, db, "Pending original", "pending")

	tests := []struct {
		name     string
		payload  string
		wantCode int
	}{
		{
			name: "original zone cannot carry source_original_id",
			payload: `{
				"title":"Original with source",
				"zone":"original",
				"content_type":"article",
				"is_public":true,
				"allow_copy":true,
				"source_original_id":` + strconv.FormatInt(unpublishedOriginal.ID, 10) + `
			}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name: "fanwork cannot reference non-original content",
			payload: `{
				"title":"Fanwork invalid source",
				"zone":"fanwork",
				"content_type":"article",
				"is_public":true,
				"allow_copy":true,
				"source_original_id":` + strconv.FormatInt(nonOriginal.ID, 10) + `
			}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name: "fanwork cannot reference unpublished original",
			payload: `{
				"title":"Fanwork unpublished source",
				"zone":"fanwork",
				"content_type":"article",
				"is_public":true,
				"allow_copy":true,
				"source_original_id":` + strconv.FormatInt(unpublishedOriginal.ID, 10) + `
			}`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte("INVALID_SOURCE_ORIGINAL")) {
				t.Fatalf("body = %s, want INVALID_SOURCE_ORIGINAL", rec.Body.String())
			}
		})
	}
}

func TestCreateContentRouteEnforcesPublishPermissions(t *testing.T) {
	tests := []struct {
		name          string
		state         publishRouteUserState
		wantCode      int
		wantBodyToken string
	}{
		{name: "unverified", state: publishRouteUserState{Verified: false, Reputation: 10}, wantCode: http.StatusForbidden, wantBodyToken: "EMAIL_NOT_VERIFIED"},
		{name: "low reputation", state: publishRouteUserState{Verified: true, Reputation: 2}, wantCode: http.StatusForbidden, wantBodyToken: "INSUFFICIENT_REPUTATION"},
		{name: "banned", state: publishRouteUserState{Verified: true, Reputation: 10, Banned: true}, wantCode: http.StatusUnauthorized, wantBodyToken: "USER_BANNED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _, token, _ := setupPublishRoute(t, tt.state)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", bytes.NewBufferString(`{
				"title":"Blocked publish",
				"zone":"fanwork",
				"content_type":"article",
				"is_public":true,
				"allow_copy":true
			}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantBodyToken)) {
				t.Fatalf("body = %s, want %s", rec.Body.String(), tt.wantBodyToken)
			}
		})
	}
}

func TestCreateContentRouteRejectsPublishFrozen(t *testing.T) {
	router, db, token, mr := setupPublishRoute(t, publishRouteUserState{Verified: true, Reputation: 10})
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	var viewer model.User
	if err := db.Where("email = ?", "publish-viewer@example.com").First(&viewer).Error; err != nil {
		t.Fatalf("load viewer: %v", err)
	}

	// review_service.applyRepeatViolationPenalty writes the canonical key with
	// value "1" and a TTL; the publish guard must block on that exact key.
	if err := rdb.Set(context.Background(), rediskeys.PublishFreezeKey(viewer.ID), "1", 7*24*time.Hour).Err(); err != nil {
		t.Fatalf("set freeze key: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", bytes.NewBufferString(`{
		"title":"Frozen publish",
		"zone":"fanwork",
		"content_type":"article",
		"is_public":true,
		"allow_copy":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("PUBLISH_FROZEN")) {
		t.Fatalf("body = %s, want PUBLISH_FROZEN", rec.Body.String())
	}
}

type publishRouteUserState struct {
	Verified   bool
	Reputation int
	Banned     bool
}

func setupPublishRoute(t *testing.T, state publishRouteUserState) (*gin.Engine, *gorm.DB, string, *miniredis.Miniredis) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.ContentAttachment{}, &model.ContentTag{}, &model.IP{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.JWT.Secret = "publish-route-secret"
	cfg.Reputation.MinScoreForInteraction = 3
	cfg.Cache.PublishFreezeTTL = 604800

	handler := NewContentHandler(db, cfg, nil)
	authReq := middleware.AuthRequired(cfg, nil, db)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	publishGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail:   true,
		RequireReputation:      true,
		RequireNoPublishFreeze: true,
	})

	router := gin.New()
	router.POST("/api/v1/contents", authReq, publishGuard, handler.CreateContent)
	router.GET("/api/v1/contents/:id", handler.GetContent)

	now := time.Now()
	viewer := model.User{
		Email:        "publish-viewer@example.com",
		Username:     "publish-viewer",
		PasswordHash: "hash",
		Reputation:   state.Reputation,
		Role:         "user",
		IsBanned:     state.Banned,
	}
	if state.Verified {
		viewer.EmailVerifiedAt = &now
	}
	if err := db.Create(&viewer).Error; err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	pair, err := jwtutil.GenerateTokenPair(viewer.ID, viewer.Role, cfg.JWT.Secret, 120, 7)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})

	return router, db, pair.AccessToken, mr
}

func createPublishSourceOriginal(t *testing.T, db *gorm.DB, title, status string) model.ContentItem {
	t.Helper()
	return createPublishContent(t, db, title, "original", status)
}

func createPublishContent(t *testing.T, db *gorm.DB, title, zone, status string) model.ContentItem {
	t.Helper()
	author := model.User{
		Email:        title + "@example.com",
		Username:     title,
		PasswordHash: "hash",
		Reputation:   10,
		Role:         "user",
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	item := model.ContentItem{
		Title:       title,
		AuthorID:    author.ID,
		Zone:        zone,
		Category:    "game",
		ContentType: "article",
		Status:      status,
		IsPublic:    true,
		AllowCopy:   true,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	return item
}
