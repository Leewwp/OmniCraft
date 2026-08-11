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
			Zone  string `json:"zone"`
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
	if loaded.SourceOriginal.Zone != "original" {
		t.Fatalf("source_original zone = %q, want original", loaded.SourceOriginal.Zone)
	}
}

func TestCreateContentRoutePublishesFanworkWithSourceFanwork(t *testing.T) {
	router, db, token, _ := setupPublishRoute(t, publishRouteUserState{Verified: true, Reputation: 10})
	source := createPublishContent(t, db, "Published fanwork source", "fanwork", "published")

	body := `{
		"title":"Derived fanwork",
		"zone":"fanwork",
		"content_type":"article",
		"is_public":true,
		"allow_copy":true,
		"source_fanwork_id":` + strconv.FormatInt(source.ID, 10) + `
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
			ID              int64  `json:"id"`
			SourceFanworkID *int64 `json:"source_fanwork_id"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v; body = %s", err, rec.Body.String())
	}
	if created.Content.SourceFanworkID == nil || *created.Content.SourceFanworkID != source.ID {
		t.Fatalf("source_fanwork_id = %#v, want %d", created.Content.SourceFanworkID, source.ID)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/contents/"+strconv.FormatInt(created.Content.ID, 10), nil)
	router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body = %s", getRec.Code, getRec.Body.String())
	}

	var loaded struct {
		Content struct {
			SourceFanworkID *int64 `json:"source_fanwork_id"`
		} `json:"content"`
		SourceFanwork struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
			Zone  string `json:"zone"`
		} `json:"source_fanwork"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &loaded); err != nil {
		t.Fatalf("decode get response: %v; body = %s", err, getRec.Body.String())
	}
	if loaded.Content.SourceFanworkID == nil || *loaded.Content.SourceFanworkID != source.ID {
		t.Fatalf("loaded source_fanwork_id = %#v, want %d", loaded.Content.SourceFanworkID, source.ID)
	}
	if loaded.SourceFanwork.ID != source.ID || loaded.SourceFanwork.Title != source.Title {
		t.Fatalf("source_fanwork = %#v, want id=%d title=%q", loaded.SourceFanwork, source.ID, source.Title)
	}
	if loaded.SourceFanwork.Zone != "fanwork" {
		t.Fatalf("source_fanwork zone = %q, want fanwork", loaded.SourceFanwork.Zone)
	}
}

func TestUpdateContentRouteRejectsSourceImmutable(t *testing.T) {
	router, db, token, _ := setupPublishRoute(t, publishRouteUserState{Verified: true, Reputation: 10})
	source := createPublishContent(t, db, "Published fanwork source", "fanwork", "published")
	original := createPublishSourceOriginal(t, db, "Published original", "published")
	otherFanwork := createPublishContent(t, db, "Another fanwork", "fanwork", "published")

	created := createPublishContent(t, db, "Attributed fanwork", "fanwork", "published")
	if err := db.Model(&model.ContentItem{}).Where("id = ?", created.ID).
		Update("source_fanwork_id", source.ID).Error; err != nil {
		t.Fatalf("set source_fanwork_id: %v", err)
	}

	tests := []struct {
		name    string
		payload string
	}{
		{name: "ip_id is immutable", payload: `{"ip_id": 5}`},
		{name: "source_original_id is immutable", payload: `{"source_original_id":` + strconv.FormatInt(original.ID, 10) + `}`},
		{name: "source_fanwork_id is immutable", payload: `{"source_fanwork_id":` + strconv.FormatInt(otherFanwork.ID, 10) + `}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/contents/"+strconv.FormatInt(created.ID, 10), bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte("SOURCE_IMMUTABLE")) {
				t.Fatalf("body = %s, want SOURCE_IMMUTABLE", rec.Body.String())
			}
		})
	}

	var stored model.ContentItem
	if err := db.First(&stored, created.ID).Error; err != nil {
		t.Fatalf("load stored content: %v", err)
	}
	if stored.SourceFanworkID == nil || *stored.SourceFanworkID != source.ID {
		t.Fatalf("stored source_fanwork_id = %#v, want %d preserved", stored.SourceFanworkID, source.ID)
	}
	if stored.SourceOriginalID != nil {
		t.Fatalf("stored source_original_id = %#v, want nil", stored.SourceOriginalID)
	}
	if stored.IPID != nil {
		t.Fatalf("stored ip_id = %#v, want nil", stored.IPID)
	}
}

func TestCreateContentRouteRejectsInvalidSourceLink(t *testing.T) {
	router, db, token, _ := setupPublishRoute(t, publishRouteUserState{Verified: true, Reputation: 10})
	nonOriginal := createPublishContent(t, db, "Fanwork source candidate", "fanwork", "published")
	unpublishedOriginal := createPublishSourceOriginal(t, db, "Pending original", "pending")

	tests := []struct {
		name          string
		payload       string
		wantCode      int
		wantBodyToken string
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
			wantCode:      http.StatusBadRequest,
			wantBodyToken: "SOURCE_NOT_ALLOWED_FOR_ORIGINAL",
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
			wantCode:      http.StatusBadRequest,
			wantBodyToken: "SOURCE_ORIGINAL_UNAVAILABLE",
		},
		{
			name: "original zone cannot carry source_fanwork_id",
			payload: `{
				"title":"Original with fanwork source",
				"zone":"original",
				"content_type":"article",
				"is_public":true,
				"allow_copy":true,
				"source_fanwork_id":` + strconv.FormatInt(nonOriginal.ID, 10) + `
			}`,
			wantCode:      http.StatusBadRequest,
			wantBodyToken: "SOURCE_NOT_ALLOWED_FOR_ORIGINAL",
		},
		{
			name: "fanwork without IP or source is rejected",
			payload: `{
				"title":"Fanwork no source",
				"zone":"fanwork",
				"content_type":"article",
				"is_public":true,
				"allow_copy":true
			}`,
			wantCode:      http.StatusBadRequest,
			wantBodyToken: "FANWORK_SOURCE_REQUIRED",
		},
		{
			name: "fanwork with both source IDs is rejected",
			payload: `{
				"title":"Fanwork two sources",
				"zone":"fanwork",
				"content_type":"article",
				"is_public":true,
				"allow_copy":true,
				"source_original_id":` + strconv.FormatInt(unpublishedOriginal.ID, 10) + `,
				"source_fanwork_id":` + strconv.FormatInt(nonOriginal.ID, 10) + `
			}`,
			wantCode:      http.StatusBadRequest,
			wantBodyToken: "MULTIPLE_SOURCE_CONFLICT",
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
			if !bytes.Contains(rec.Body.Bytes(), []byte(tt.wantBodyToken)) {
				t.Fatalf("body = %s, want %s", rec.Body.String(), tt.wantBodyToken)
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

func TestCreateContentRouteRejectsInvalidMediaSets(t *testing.T) {
	router, _, token, _ := setupPublishRoute(t, publishRouteUserState{Verified: true, Reputation: 10})

	imageAttachment := func(sortOrder int, width, height int) string {
		return `{"file_type":"image","oss_key":"uploads/1/image/` + strconv.Itoa(sortOrder) + `.png","sort_order":` + strconv.Itoa(sortOrder) + `,"width":` + strconv.Itoa(width) + `,"height":` + strconv.Itoa(height) + `}`
	}
	videoAttachment := func(sortOrder int) string {
		return `{"file_type":"video","oss_key":"uploads/1/video/` + strconv.Itoa(sortOrder) + `.mp4","sort_order":` + strconv.Itoa(sortOrder) + `,"width":1280,"height":720}`
	}
	tenImages := func() string {
		parts := make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			parts = append(parts, imageAttachment(i, 100, 100))
		}
		return `{"title":"many","zone":"original","content_type":"image","attachments":[` + joinComma(parts) + `]}`
	}()

	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "image content with a single attachment is too few",
			payload: `{"title":"few","zone":"original","content_type":"image","attachments":[` + imageAttachment(0, 100, 100) + `]}`,
		},
		{name: "image content with ten attachments is too many", payload: tenImages},
		{
			name:    "image content cannot carry video attachments",
			payload: `{"title":"mixed","zone":"original","content_type":"image","attachments":[` + imageAttachment(0, 100, 100) + `,` + videoAttachment(1) + `]}`,
		},
		{
			name:    "video content cannot carry image attachments",
			payload: `{"title":"mixed2","zone":"original","content_type":"video","attachments":[` + videoAttachment(0) + `,` + imageAttachment(1, 100, 100) + `]}`,
		},
		{
			name:    "negative sort_order is rejected",
			payload: `{"title":"neg","zone":"original","content_type":"image","attachments":[` + imageAttachment(-1, 100, 100) + `,` + imageAttachment(0, 100, 100) + `]}`,
		},
		{
			name:    "duplicate sort_order is rejected",
			payload: `{"title":"dup","zone":"original","content_type":"image","attachments":[` + imageAttachment(0, 100, 100) + `,` + imageAttachment(0, 200, 200) + `]}`,
		},
		{
			name:    "non-positive width is rejected",
			payload: `{"title":"w0","zone":"original","content_type":"image","attachments":[{"file_type":"image","oss_key":"uploads/1/image/a.png","sort_order":0,"width":0,"height":100},` + imageAttachment(1, 100, 100) + `]}`,
		},
		{
			name:    "non-positive height is rejected",
			payload: `{"title":"h0","zone":"original","content_type":"video","attachments":[{"file_type":"video","oss_key":"uploads/1/video/a.mp4","sort_order":0,"width":1280,"height":-1}]}`,
		},
		{
			name:    "media content cannot accept an arbitrary client cover_image_url",
			payload: `{"title":"cover","zone":"original","content_type":"image","cover_image_url":"https://evil.example.com/cover.png","attachments":[` + imageAttachment(0, 100, 100) + `,` + imageAttachment(1, 100, 100) + `]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/contents", bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte("MEDIA_SET_INVALID")) {
				t.Fatalf("body = %s, want MEDIA_SET_INVALID", rec.Body.String())
			}
		})
	}
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
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
	router.PATCH("/api/v1/contents/:id", authReq, handler.UpdateContent)

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
