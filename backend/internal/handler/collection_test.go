package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	jwtutil "omnicraft/backend/internal/pkg/jwt"
)

func TestCollectionPublicDetailAccessibleWithoutAuthAndFiltersItems(t *testing.T) {
	router, db, _ := setupCollectionHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "detail-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	collection := seedCollectionHandlerCollection(t, db, 100, owner.ID, "Public reads", "original", false, true, 1)
	article := seedCollectionHandlerContent(t, db, 200, owner.ID, "Article item", "original", "article", "published", true)
	image := seedCollectionHandlerContent(t, db, 201, owner.ID, "Image item", "original", "image", "published", true)
	seedCollectionHandlerItem(t, db, 300, collection.ID, article.ID, "article note")
	seedCollectionHandlerItem(t, db, 301, collection.ID, image.ID, "image note")

	rec := requestCollection(t, router, "", http.MethodGet, "/api/v1/collections/100?page=1&page_size=20&content_type=article", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload collectionDetailPayload
	decodeCollectionJSON(t, rec, &payload)
	require.Equal(t, collection.ID, payload.Collection.ID)
	require.Equal(t, int64(1), payload.Total)
	require.Equal(t, 1, payload.Page)
	require.Equal(t, 20, payload.PageSize)
	require.Len(t, payload.Items, 1)
	require.Equal(t, article.ID, payload.Items[0].ContentItemID)
	require.Equal(t, "article", payload.Items[0].ContentItem.ContentType)
}

func TestCollectionPrivateDetailReturnsNotFoundForNonOwner(t *testing.T) {
	router, db, cfg := setupCollectionHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "private-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	other := seedCollectionHandlerUser(t, db, 20, "private-other", collectionHandlerUserState{Verified: true, Reputation: 10})
	seedCollectionHandlerCollection(t, db, 100, owner.ID, "Private reads", "original", false, false, 1)

	rec := requestCollection(t, router, collectionHandlerToken(t, cfg, other.ID, other.Role), http.MethodGet, "/api/v1/collections/100", "")

	assertCollectionError(t, rec, http.StatusNotFound, "COLLECTION_NOT_FOUND")
}

func TestCollectionListVisibilityAndOwnerSemantics(t *testing.T) {
	router, db, cfg := setupCollectionHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "list-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	other := seedCollectionHandlerUser(t, db, 20, "list-other", collectionHandlerUserState{Verified: true, Reputation: 10})
	publicCollection := seedCollectionHandlerCollection(t, db, 100, owner.ID, "Public list", "original", false, true, 1)
	privateCollection := seedCollectionHandlerCollection(t, db, 101, owner.ID, "Private list", "original", false, false, 2)
	seedCollectionHandlerCollection(t, db, 102, owner.ID, "Default original", "original", true, false, 0)
	seedCollectionHandlerCollection(t, db, 103, owner.ID, "Default fanwork", "fanwork", true, false, 0)

	anonymousNoOwner := requestCollection(t, router, "", http.MethodGet, "/api/v1/collections", "")
	assertCollectionError(t, anonymousNoOwner, http.StatusUnauthorized, "AUTH_REQUIRED")

	anonymousOwner := requestCollection(t, router, "", http.MethodGet, "/api/v1/collections?owner_id=10", "")
	assertCollectionListIDs(t, anonymousOwner, []int64{publicCollection.ID})
	assertCollectionListContainsDefaults(t, anonymousOwner)

	nonOwner := requestCollection(t, router, collectionHandlerToken(t, cfg, other.ID, other.Role), http.MethodGet, "/api/v1/collections?owner_id=10", "")
	assertCollectionListIDs(t, nonOwner, []int64{publicCollection.ID})

	ownerWithOwnerID := requestCollection(t, router, collectionHandlerToken(t, cfg, owner.ID, owner.Role), http.MethodGet, "/api/v1/collections?owner_id=10", "")
	assertCollectionListIDs(t, ownerWithOwnerID, []int64{102, 103, publicCollection.ID, privateCollection.ID})

	ownerImplicit := requestCollection(t, router, collectionHandlerToken(t, cfg, owner.ID, owner.Role), http.MethodGet, "/api/v1/collections", "")
	assertCollectionListIDs(t, ownerImplicit, []int64{102, 103, publicCollection.ID, privateCollection.ID})
}

func TestCollectionOwnListSelfHealsDefaultsWhileOwnerIDReadStaysReadOnly(t *testing.T) {
	t.Run("implicit own list self-heals both defaults", func(t *testing.T) {
		router, db, cfg := setupCollectionHandlerRouter(t)
		owner := seedCollectionHandlerUser(t, db, 10, "self-heal-handler-owner", collectionHandlerUserState{Verified: true, Reputation: 10})

		rec := requestCollection(t, router, collectionHandlerToken(t, cfg, owner.ID, owner.Role), http.MethodGet, "/api/v1/collections", "")

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var payload collectionListPayload
		decodeCollectionJSON(t, rec, &payload)
		require.Len(t, payload.Items, 2)
		require.ElementsMatch(t, []string{"original", "fanwork"}, []string{payload.Items[0].Zone, payload.Items[1].Zone})
		assertCollectionHandlerDefaultCount(t, db, owner.ID, 2)
	})

	t.Run("explicit owner id read never repairs another user", func(t *testing.T) {
		router, db, _ := setupCollectionHandlerRouter(t)
		owner := seedCollectionHandlerUser(t, db, 10, "read-only-handler-owner", collectionHandlerUserState{Verified: true, Reputation: 10})

		rec := requestCollection(t, router, "", http.MethodGet, "/api/v1/collections?owner_id=10", "")

		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var payload collectionListPayload
		decodeCollectionJSON(t, rec, &payload)
		require.Empty(t, payload.Items)
		assertCollectionHandlerDefaultCount(t, db, owner.ID, 0)
	})
}

func TestCollectionListContainsItemUsesContentZoneAndDetectsConflicts(t *testing.T) {
	router, db, cfg := setupCollectionHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "contains-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	seedCollectionHandlerCollection(t, db, 100, owner.ID, "Original collection", "original", false, false, 1)
	withItem := seedCollectionHandlerCollection(t, db, 101, owner.ID, "Fanwork has item", "fanwork", false, false, 1)
	withoutItem := seedCollectionHandlerCollection(t, db, 102, owner.ID, "Fanwork empty", "fanwork", false, false, 2)
	defaultOriginal := seedCollectionHandlerCollection(t, db, 103, owner.ID, "Default original", "original", true, false, 0)
	defaultFanwork := seedCollectionHandlerCollection(t, db, 104, owner.ID, "Default fanwork", "fanwork", true, false, 0)
	content := seedCollectionHandlerContent(t, db, 123, owner.ID, "Fanwork", "fanwork", "image", "published", true)
	item := seedCollectionHandlerItem(t, db, 301, withItem.ID, content.ID, "keeper")
	token := collectionHandlerToken(t, cfg, owner.ID, owner.Role)

	rec := requestCollection(t, router, token, http.MethodGet, "/api/v1/collections?zone=fanwork&content_item_id=123", "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload collectionListPayload
	decodeCollectionJSON(t, rec, &payload)
	require.Len(t, payload.Items, 3)
	seen := map[int64]collectionSummaryPayload{}
	for _, summary := range payload.Items {
		require.Equal(t, "fanwork", summary.Zone)
		seen[summary.ID] = summary
	}
	require.True(t, seen[withItem.ID].ContainsItem)
	require.NotNil(t, seen[withItem.ID].ItemID)
	require.Equal(t, item.ID, *seen[withItem.ID].ItemID)
	require.False(t, seen[withoutItem.ID].ContainsItem)
	require.Nil(t, seen[withoutItem.ID].ItemID)
	require.NotContains(t, seen, defaultOriginal.ID)
	require.False(t, seen[defaultFanwork.ID].ContainsItem)
	require.Nil(t, seen[defaultFanwork.ID].ItemID)

	conflict := requestCollection(t, router, token, http.MethodGet, "/api/v1/collections?zone=original&content_item_id=123", "")
	assertCollectionError(t, conflict, http.StatusBadRequest, "ZONE_MISMATCH")
}

func TestCollectionMutationsRequireAuthReputationAndOwnership(t *testing.T) {
	router, db, cfg := setupCollectionHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "mutation-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	other := seedCollectionHandlerUser(t, db, 20, "mutation-other", collectionHandlerUserState{Verified: true, Reputation: 10})
	lowRep := seedCollectionHandlerUser(t, db, 30, "mutation-low", collectionHandlerUserState{Verified: true, Reputation: 2})
	collection := seedCollectionHandlerCollection(t, db, 100, owner.ID, "Mutable", "original", false, false, 1)
	content := seedCollectionHandlerContent(t, db, 200, owner.ID, "Article", "original", "article", "published", true)
	ownerToken := collectionHandlerToken(t, cfg, owner.ID, owner.Role)
	otherToken := collectionHandlerToken(t, cfg, other.ID, other.Role)
	lowRepToken := collectionHandlerToken(t, cfg, lowRep.ID, lowRep.Role)

	noAuth := requestCollection(t, router, "", http.MethodPost, "/api/v1/collections", `{"title":"No auth","zone":"original"}`)
	assertCollectionError(t, noAuth, http.StatusUnauthorized, "UNAUTHORIZED")

	lowRepCreate := requestCollection(t, router, lowRepToken, http.MethodPost, "/api/v1/collections", `{"title":"Low rep","zone":"original"}`)
	assertCollectionError(t, lowRepCreate, http.StatusForbidden, "INSUFFICIENT_REPUTATION")

	created := requestCollection(t, router, ownerToken, http.MethodPost, "/api/v1/collections", `{"title":"  Created  ","description":"fresh","zone":"fanwork","is_public":true}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createPayload struct {
		Collection collectionSummaryPayload `json:"collection"`
	}
	decodeCollectionJSON(t, created, &createPayload)
	require.Equal(t, "Created", createPayload.Collection.Title)
	require.Equal(t, "fanwork", createPayload.Collection.Zone)
	require.True(t, createPayload.Collection.IsPublic)

	otherUpdate := requestCollection(t, router, otherToken, http.MethodPut, "/api/v1/collections/100", `{"title":"Stolen"}`)
	assertCollectionError(t, otherUpdate, http.StatusNotFound, "COLLECTION_NOT_FOUND")

	otherAdd := requestCollection(t, router, otherToken, http.MethodPost, "/api/v1/collections/100/items", `{"content_item_id":`+strconv.FormatInt(content.ID, 10)+`}`)
	assertCollectionError(t, otherAdd, http.StatusNotFound, "COLLECTION_NOT_FOUND")

	added := requestCollection(t, router, ownerToken, http.MethodPost, "/api/v1/collections/100/items", `{"content_item_id":`+strconv.FormatInt(content.ID, 10)+`,"note":"first"}`)
	require.Equal(t, http.StatusCreated, added.Code, added.Body.String())
	var addPayload struct {
		Item struct {
			ID   int64  `json:"id"`
			Note string `json:"note"`
		} `json:"item"`
	}
	decodeCollectionJSON(t, added, &addPayload)
	require.NotZero(t, addPayload.Item.ID)
	require.Equal(t, "first", addPayload.Item.Note)

	itemPath := "/api/v1/collections/100/items/" + strconv.FormatInt(addPayload.Item.ID, 10)
	otherNote := requestCollection(t, router, otherToken, http.MethodPut, itemPath, `{"note":"stolen"}`)
	assertCollectionError(t, otherNote, http.StatusNotFound, "COLLECTION_NOT_FOUND")

	updatedNote := requestCollection(t, router, ownerToken, http.MethodPut, itemPath, `{"note":"better"}`)
	require.Equal(t, http.StatusOK, updatedNote.Code, updatedNote.Body.String())

	otherRemove := requestCollection(t, router, otherToken, http.MethodDelete, itemPath, "")
	assertCollectionError(t, otherRemove, http.StatusNotFound, "COLLECTION_NOT_FOUND")

	removed := requestCollection(t, router, ownerToken, http.MethodDelete, itemPath, "")
	require.Equal(t, http.StatusOK, removed.Code, removed.Body.String())

	otherDelete := requestCollection(t, router, otherToken, http.MethodDelete, "/api/v1/collections/100", "")
	assertCollectionError(t, otherDelete, http.StatusNotFound, "COLLECTION_NOT_FOUND")

	deleted := requestCollection(t, router, ownerToken, http.MethodDelete, "/api/v1/collections/100", "")
	require.Equal(t, http.StatusOK, deleted.Code, deleted.Body.String())
	require.NotNil(t, loadCollectionHandlerCollection(t, db, collection.ID).DeletedAt)
}

func TestCollectionValidationAndSentinelErrorMapping(t *testing.T) {
	router, db, cfg := setupCollectionHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "validation-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	other := seedCollectionHandlerUser(t, db, 20, "validation-author", collectionHandlerUserState{Verified: true, Reputation: 10})
	defaultCollection := seedCollectionHandlerCollection(t, db, 99, owner.ID, "Default original", "original", true, false, 0)
	original := seedCollectionHandlerCollection(t, db, 100, owner.ID, "Original", "original", false, false, 1)
	fanworkContent := seedCollectionHandlerContent(t, db, 200, other.ID, "Fanwork", "fanwork", "image", "published", true)
	originalContent := seedCollectionHandlerContent(t, db, 201, other.ID, "Original content", "original", "article", "published", true)
	unpublished := seedCollectionHandlerContent(t, db, 202, other.ID, "Pending", "original", "article", "pending", true)
	seedCollectionHandlerItem(t, db, 300, original.ID, originalContent.ID, "")
	token := collectionHandlerToken(t, cfg, owner.ID, owner.Role)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{name: "blank title", method: http.MethodPost, path: "/api/v1/collections", body: `{"title":"   ","zone":"original"}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "title too long", method: http.MethodPost, path: "/api/v1/collections", body: `{"title":"` + strings.Repeat("x", 201) + `","zone":"original"}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "invalid zone", method: http.MethodPost, path: "/api/v1/collections", body: `{"title":"Bad zone","zone":"music"}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "create id rejected", method: http.MethodPost, path: "/api/v1/collections", body: `{"title":"Spoof","zone":"original","id":100}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "create unknown field rejected", method: http.MethodPost, path: "/api/v1/collections", body: `{"title":"Typo","zone":"original","visibile":true}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "create is_default rejected", method: http.MethodPost, path: "/api/v1/collections", body: `{"title":"Default","zone":"original","is_default":true}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "create user_id rejected", method: http.MethodPost, path: "/api/v1/collections", body: `{"title":"Spoof","zone":"original","user_id":20}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "update null rejected", method: http.MethodPut, path: "/api/v1/collections/100", body: `null`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "update unknown field rejected", method: http.MethodPut, path: "/api/v1/collections/100", body: `{"title":"Clean","created_at":"2026-07-02T00:00:00Z"}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "update is_default rejected", method: http.MethodPut, path: "/api/v1/collections/100", body: `{"is_default":true}`, status: http.StatusBadRequest, code: "VALIDATION_ERROR"},
		{name: "update zone immutable", method: http.MethodPut, path: "/api/v1/collections/100", body: `{"zone":"fanwork"}`, status: http.StatusBadRequest, code: "ZONE_IMMUTABLE"},
		{name: "zone mismatch", method: http.MethodPost, path: "/api/v1/collections/100/items", body: `{"content_item_id":` + strconv.FormatInt(fanworkContent.ID, 10) + `}`, status: http.StatusBadRequest, code: "ZONE_MISMATCH"},
		{name: "duplicate item", method: http.MethodPost, path: "/api/v1/collections/100/items", body: `{"content_item_id":` + strconv.FormatInt(originalContent.ID, 10) + `}`, status: http.StatusConflict, code: "DUPLICATE_COLLECTION_ITEM"},
		{name: "invalid content", method: http.MethodPost, path: "/api/v1/collections/100/items", body: `{"content_item_id":` + strconv.FormatInt(unpublished.ID, 10) + `}`, status: http.StatusBadRequest, code: "INVALID_CONTENT"},
		{name: "default collection protected", method: http.MethodDelete, path: "/api/v1/collections/" + strconv.FormatInt(defaultCollection.ID, 10), status: http.StatusBadRequest, code: "DEFAULT_COLLECTION_PROTECTED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestCollection(t, router, token, tt.method, tt.path, tt.body)
			assertCollectionError(t, rec, tt.status, tt.code)
		})
	}
}

type collectionHandlerUserState struct {
	Verified   bool
	Reputation int
	Banned     bool
}

type collectionListPayload struct {
	Items []collectionSummaryPayload `json:"items"`
	Total int64                      `json:"total"`
}

type collectionSummaryPayload struct {
	ID           int64  `json:"id"`
	UserID       int64  `json:"user_id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Zone         string `json:"zone"`
	IsDefault    bool   `json:"is_default"`
	IsPublic     bool   `json:"is_public"`
	SortOrder    int    `json:"sort_order"`
	ItemCount    int64  `json:"item_count"`
	ContainsItem bool   `json:"contains_item"`
	ItemID       *int64 `json:"item_id,omitempty"`
}

type collectionDetailPayload struct {
	Collection collectionSummaryPayload `json:"collection"`
	Items      []struct {
		ID            int64             `json:"id"`
		ContentItemID int64             `json:"content_item_id"`
		ContentItem   model.ContentItem `json:"content_item"`
	} `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func setupCollectionHandlerRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.Collection{}, &model.CollectionItem{}))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX idx_collections_one_default_per_zone ON collections (user_id, zone) WHERE is_default = TRUE`).Error)

	cfg := &config.Config{}
	cfg.JWT.Secret = "collection-handler-secret"
	cfg.Reputation.MinScoreForInteraction = 3
	cfg.Cache.UserStatusTTL = 300

	collectionHandler := NewCollectionHandler(db)
	optAuth := middleware.OptionalAuth(cfg, nil, db)
	authReq := middleware.AuthRequired(cfg, nil, db)
	interactionGuard := middleware.InteractionRequired(cfg, db, nil, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})

	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.GET("/collections", optAuth, collectionHandler.ListCollections)
	v1.GET("/collections/:id", optAuth, collectionHandler.GetCollection)
	v1.POST("/collections", authReq, interactionGuard, collectionHandler.CreateCollection)
	v1.PUT("/collections/:id", authReq, interactionGuard, collectionHandler.UpdateCollection)
	v1.DELETE("/collections/:id", authReq, interactionGuard, collectionHandler.DeleteCollection)
	v1.POST("/collections/:id/items", authReq, interactionGuard, collectionHandler.AddItem)
	v1.DELETE("/collections/:id/items/:itemId", authReq, interactionGuard, collectionHandler.RemoveItem)
	v1.PUT("/collections/:id/items/:itemId", authReq, interactionGuard, collectionHandler.UpdateItem)

	return router, db, cfg
}

func requestCollection(t *testing.T, router *gin.Engine, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(rec, req)
	return rec
}

func collectionHandlerToken(t *testing.T, cfg *config.Config, userID int64, role string) string {
	t.Helper()

	pair, err := jwtutil.GenerateTokenPair(userID, role, cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)
	return pair.AccessToken
}

func seedCollectionHandlerUser(t *testing.T, db *gorm.DB, id int64, username string, state collectionHandlerUserState) model.User {
	t.Helper()

	now := time.Now()
	user := model.User{
		ID:           id,
		Email:        username + "@example.com",
		Username:     username,
		PasswordHash: "hash",
		Reputation:   state.Reputation,
		Role:         "user",
		IsBanned:     state.Banned,
	}
	if state.Verified {
		user.EmailVerifiedAt = &now
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedCollectionHandlerCollection(t *testing.T, db *gorm.DB, id, userID int64, title, zone string, isDefault, isPublic bool, sortOrder int) model.Collection {
	t.Helper()

	collection := model.Collection{
		ID:          id,
		UserID:      userID,
		Title:       title,
		Description: title + " description",
		Zone:        zone,
		IsDefault:   isDefault,
		IsPublic:    isPublic,
		SortOrder:   sortOrder,
	}
	require.NoError(t, db.Create(&collection).Error)
	return collection
}

func seedCollectionHandlerContent(t *testing.T, db *gorm.DB, id, authorID int64, title, zone, contentType, status string, isPublic bool) model.ContentItem {
	t.Helper()

	content := model.ContentItem{
		ID:          id,
		Title:       title,
		AuthorID:    authorID,
		Zone:        zone,
		Category:    "game",
		ContentType: contentType,
		Status:      status,
		IsPublic:    isPublic,
		AllowCopy:   true,
	}
	require.NoError(t, db.Create(&content).Error)
	return content
}

func seedCollectionHandlerItem(t *testing.T, db *gorm.DB, id, collectionID, contentItemID int64, note string) model.CollectionItem {
	t.Helper()

	item := model.CollectionItem{
		ID:            id,
		CollectionID:  collectionID,
		ContentItemID: contentItemID,
		Note:          note,
	}
	require.NoError(t, db.Create(&item).Error)
	return item
}

func loadCollectionHandlerCollection(t *testing.T, db *gorm.DB, id int64) model.Collection {
	t.Helper()

	var collection model.Collection
	require.NoError(t, db.Unscoped().First(&collection, id).Error)
	return collection
}

func decodeCollectionJSON(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()

	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), target), rec.Body.String())
}

func assertCollectionError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	require.Equal(t, status, rec.Code, rec.Body.String())
	var payload struct {
		Code string `json:"code"`
	}
	decodeCollectionJSON(t, rec, &payload)
	require.Equal(t, code, payload.Code, rec.Body.String())
}

func assertCollectionListIDs(t *testing.T, rec *httptest.ResponseRecorder, want []int64) {
	t.Helper()

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload collectionListPayload
	decodeCollectionJSON(t, rec, &payload)
	require.Equal(t, int64(len(want)), payload.Total)
	got := make([]int64, 0, len(payload.Items))
	for _, item := range payload.Items {
		got = append(got, item.ID)
	}
	require.ElementsMatch(t, want, got)
}

func assertCollectionHandlerDefaultCount(t *testing.T, db *gorm.DB, userID, want int64) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.Collection{}).
		Where("user_id = ? AND is_default = ? AND deleted_at IS NULL", userID, true).
		Count(&count).Error)
	require.Equal(t, want, count)
}

func assertCollectionListContainsDefaults(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	var payload collectionListPayload
	decodeCollectionJSON(t, rec, &payload)
	for _, item := range payload.Items {
		require.False(t, item.ContainsItem)
		require.Nil(t, item.ItemID)
	}
}
