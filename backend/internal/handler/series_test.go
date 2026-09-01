package handler

import (
	"encoding/json"
	"net/http"
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
)

func TestSeriesRoutesHappyPathAndCompactDetail(t *testing.T) {
	router, db, cfg := setupSeriesHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "series-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	viewer := seedCollectionHandlerUser(t, db, 11, "series-viewer", collectionHandlerUserState{Verified: true, Reputation: 10})
	token := collectionHandlerToken(t, cfg, owner.ID, owner.Role)
	viewerToken := collectionHandlerToken(t, cfg, viewer.ID, viewer.Role)

	created := requestCollection(t, router, token, http.MethodPost, "/api/v1/series", `{"title":"  First arc  ","description":"  notes  ","zone":"original"}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createPayload struct {
		Series struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"series"`
	}
	decodeCollectionJSON(t, created, &createPayload)
	require.NotZero(t, createPayload.Series.ID)
	require.Equal(t, "First arc", createPayload.Series.Title)

	pending := seedCollectionHandlerContent(t, db, 100, owner.ID, "pending secret", "original", "article", "pending", true)
	published := seedCollectionHandlerContent(t, db, 101, owner.ID, "published chapter", "original", "article", "published", true)
	published.CoverImageURL = "https://example.test/cover.png"
	require.NoError(t, db.Save(&published).Error)

	for _, contentID := range []int64{pending.ID, published.ID} {
		added := requestCollection(t, router, token, http.MethodPost, seriesPath(createPayload.Series.ID)+"/items", `{"content_item_id":`+itoa64(contentID)+`}`)
		require.Equal(t, http.StatusCreated, added.Code, added.Body.String())
	}
	pendingCover := requestCollection(t, router, token, http.MethodPut, seriesPath(createPayload.Series.ID), `{"cover_content_id":`+itoa64(pending.ID)+`}`)
	require.Equal(t, http.StatusOK, pendingCover.Code, pendingCover.Body.String())

	publicDetail := requestCollection(t, router, "", http.MethodGet, seriesPath(createPayload.Series.ID), "")
	require.Equal(t, http.StatusOK, publicDetail.Code, publicDetail.Body.String())
	require.NotContains(t, publicDetail.Body.String(), "pending secret")
	require.NotContains(t, publicDetail.Body.String(), owner.Email)
	require.NotContains(t, publicDetail.Body.String(), "password_hash")
	require.NotContains(t, publicDetail.Body.String(), "cover_content_id")
	var publicPayload seriesDetailPayload
	decodeCollectionJSON(t, publicDetail, &publicPayload)
	require.Equal(t, int64(1), publicPayload.Series.ItemCount)
	require.Len(t, publicPayload.Items, 1)
	require.Equal(t, published.ID, publicPayload.Items[0].Content.ID)
	require.Equal(t, published.CoverImageURL, *publicPayload.Series.Cover)
	nonOwnerDetail := requestCollection(t, router, viewerToken, http.MethodGet, seriesPath(createPayload.Series.ID), "")
	require.Equal(t, http.StatusOK, nonOwnerDetail.Code, nonOwnerDetail.Body.String())
	require.NotContains(t, nonOwnerDetail.Body.String(), pending.Title)
	require.NotContains(t, nonOwnerDetail.Body.String(), "cover_content_id")

	ownerPublicDetail := requestCollection(t, router, token, http.MethodGet, seriesPath(createPayload.Series.ID), "")
	require.Equal(t, http.StatusOK, ownerPublicDetail.Code, ownerPublicDetail.Body.String())
	require.NotContains(t, ownerPublicDetail.Body.String(), pending.Title)
	require.NotContains(t, ownerPublicDetail.Body.String(), "cover_content_id")
	ownerDetail := requestCollection(t, router, token, http.MethodGet, seriesPath(createPayload.Series.ID)+"?manage=true", "")
	require.Equal(t, http.StatusOK, ownerDetail.Code, ownerDetail.Body.String())
	require.Contains(t, ownerDetail.Body.String(), `"cover_content_id":100`)
	var ownerPayload seriesDetailPayload
	decodeCollectionJSON(t, ownerDetail, &ownerPayload)
	require.Len(t, ownerPayload.Items, 2)
	clearedCover := requestCollection(t, router, token, http.MethodPut, seriesPath(createPayload.Series.ID), `{"cover_content_id":null}`)
	require.Equal(t, http.StatusOK, clearedCover.Code, clearedCover.Body.String())
	var clearedSeries model.ContentSeries
	require.NoError(t, db.First(&clearedSeries, createPayload.Series.ID).Error)
	require.Nil(t, clearedSeries.CoverContentID)

	list := requestCollection(t, router, token, http.MethodGet, "/api/v1/series?zone=original", "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	var listPayload struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	decodeCollectionJSON(t, list, &listPayload)
	require.Equal(t, 1, listPayload.Total)
	require.Equal(t, createPayload.Series.ID, listPayload.Items[0].ID)

	reordered := requestCollection(t, router, token, http.MethodPut, seriesPath(createPayload.Series.ID)+"/items/reorder", `{"item_ids":[`+itoa64(ownerPayload.Items[1].ID)+`,`+itoa64(ownerPayload.Items[0].ID)+`]}`)
	require.Equal(t, http.StatusOK, reordered.Code, reordered.Body.String())
	var reorderedItems []model.ContentSeriesItem
	require.NoError(t, db.Where("series_id = ?", createPayload.Series.ID).Order("sort_order ASC").Find(&reorderedItems).Error)
	require.Equal(t, []int64{ownerPayload.Items[1].ID, ownerPayload.Items[0].ID}, []int64{reorderedItems[0].ID, reorderedItems[1].ID})

	updated := requestCollection(t, router, token, http.MethodPut, seriesPath(createPayload.Series.ID), `{"title":"Updated arc","cover_content_id":`+itoa64(published.ID)+`}`)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	require.Contains(t, updated.Body.String(), "Updated arc")

	removed := requestCollection(t, router, token, http.MethodDelete, seriesPath(createPayload.Series.ID)+"/items/"+itoa64(ownerPayload.Items[0].ID), "")
	require.Equal(t, http.StatusOK, removed.Code, removed.Body.String())
	var removedCount int64
	require.NoError(t, db.Model(&model.ContentSeriesItem{}).Where("id = ?", ownerPayload.Items[0].ID).Count(&removedCount).Error)
	require.Zero(t, removedCount)

	deleted := requestCollection(t, router, token, http.MethodDelete, seriesPath(createPayload.Series.ID), "")
	require.Equal(t, http.StatusOK, deleted.Code, deleted.Body.String())
	notFound := requestCollection(t, router, "", http.MethodGet, seriesPath(createPayload.Series.ID), "")
	assertCollectionError(t, notFound, http.StatusNotFound, "SERIES_NOT_FOUND")
}

func TestSeriesRoutesRequireAuthInteractionAndOwner(t *testing.T) {
	router, db, cfg := setupSeriesHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "series-auth-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	other := seedCollectionHandlerUser(t, db, 20, "series-auth-other", collectionHandlerUserState{Verified: true, Reputation: 10})
	lowRep := seedCollectionHandlerUser(t, db, 30, "series-auth-low", collectionHandlerUserState{Verified: true, Reputation: 2})
	unverified := seedCollectionHandlerUser(t, db, 40, "series-auth-unverified", collectionHandlerUserState{Verified: false, Reputation: 10})
	series := seedSeriesHandlerSeries(t, db, 100, owner.ID, "protected", "original")

	noAuthList := requestCollection(t, router, "", http.MethodGet, "/api/v1/series", "")
	assertCollectionError(t, noAuthList, http.StatusUnauthorized, "UNAUTHORIZED")
	noAuthCandidates := requestCollection(t, router, "", http.MethodGet, "/api/v1/series/candidates?zone=original", "")
	assertCollectionError(t, noAuthCandidates, http.StatusUnauthorized, "UNAUTHORIZED")
	noAuthCreate := requestCollection(t, router, "", http.MethodPost, "/api/v1/series", `{"title":"x","zone":"original"}`)
	assertCollectionError(t, noAuthCreate, http.StatusUnauthorized, "UNAUTHORIZED")
	lowRepCreate := requestCollection(t, router, collectionHandlerToken(t, cfg, lowRep.ID, lowRep.Role), http.MethodPost, "/api/v1/series", `{"title":"x","zone":"original"}`)
	assertCollectionError(t, lowRepCreate, http.StatusForbidden, "INSUFFICIENT_REPUTATION")

	mutations := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/series", `{"title":"x","zone":"original"}`},
		{http.MethodPut, seriesPath(series.ID), `{"title":"x"}`},
		{http.MethodDelete, seriesPath(series.ID), ""},
		{http.MethodPost, seriesPath(series.ID) + "/items", `{"content_item_id":1}`},
		{http.MethodDelete, seriesPath(series.ID) + "/items/1", ""},
		{http.MethodPut, seriesPath(series.ID) + "/items/reorder", `{"item_ids":[]}`},
	}
	for _, mutation := range mutations {
		noAuth := requestCollection(t, router, "", mutation.method, mutation.path, mutation.body)
		assertCollectionError(t, noAuth, http.StatusUnauthorized, "UNAUTHORIZED")
		lowRepResponse := requestCollection(t, router, collectionHandlerToken(t, cfg, lowRep.ID, lowRep.Role), mutation.method, mutation.path, mutation.body)
		assertCollectionError(t, lowRepResponse, http.StatusForbidden, "INSUFFICIENT_REPUTATION")
		notVerified := requestCollection(t, router, collectionHandlerToken(t, cfg, unverified.ID, unverified.Role), mutation.method, mutation.path, mutation.body)
		assertCollectionError(t, notVerified, http.StatusForbidden, "EMAIL_NOT_VERIFIED")
	}

	otherToken := collectionHandlerToken(t, cfg, other.ID, other.Role)
	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPut, seriesPath(series.ID), `{"title":"stolen"}`},
		{http.MethodDelete, seriesPath(series.ID), ""},
		{http.MethodPut, seriesPath(series.ID) + "/items/reorder", `{"item_ids":[]}`},
	} {
		rec := requestCollection(t, router, otherToken, request.method, request.path, request.body)
		assertCollectionError(t, rec, http.StatusForbidden, "NOT_SERIES_OWNER")
	}
}

func TestSeriesCandidatesReturnsOwnedAndContributedManageableContent(t *testing.T) {
	router, db, cfg := setupSeriesHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "candidate-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	other := seedCollectionHandlerUser(t, db, 20, "candidate-author", collectionHandlerUserState{Verified: true, Reputation: 10})
	owned := seedCollectionHandlerContent(t, db, 100, owner.ID, "owned pending", "original", "article", "pending", true)
	contributed := seedCollectionHandlerContent(t, db, 101, other.ID, "contributed chapter", "original", "article", "published", true)
	_ = seedCollectionHandlerContent(t, db, 102, other.ID, "unrelated", "original", "article", "published", true)
	require.NoError(t, db.Create(&model.ContentContributor{ContentItemID: contributed.ID, UserID: owner.ID}).Error)

	token := collectionHandlerToken(t, cfg, owner.ID, owner.Role)
	rec := requestCollection(t, router, token, http.MethodGet, "/api/v1/series/candidates?zone=original", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), owned.Title)
	require.Contains(t, rec.Body.String(), contributed.Title)
	require.NotContains(t, rec.Body.String(), "unrelated")
	require.NotContains(t, rec.Body.String(), owner.Email)
}

func TestSeriesRoutesMapDomainErrorsAndRejectInvalidInput(t *testing.T) {
	router, db, cfg := setupSeriesHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "series-errors-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	other := seedCollectionHandlerUser(t, db, 20, "series-errors-other", collectionHandlerUserState{Verified: true, Reputation: 10})
	series := seedSeriesHandlerSeries(t, db, 100, owner.ID, "errors", "original")
	ownerToken := collectionHandlerToken(t, cfg, owner.ID, owner.Role)
	unrelated := seedCollectionHandlerContent(t, db, 200, other.ID, "unrelated", "original", "article", "published", true)
	wrongZone := seedCollectionHandlerContent(t, db, 201, owner.ID, "wrong zone", "fanwork", "article", "published", true)
	duplicate := seedCollectionHandlerContent(t, db, 202, owner.ID, "duplicate", "original", "article", "published", true)
	coverOnly := seedCollectionHandlerContent(t, db, 203, owner.ID, "cover only", "original", "article", "published", true)
	require.NoError(t, db.Create(&model.ContentSeriesItem{SeriesID: series.ID, ContentItemID: duplicate.ID, SortOrder: 0}).Error)

	assertCollectionError(t, requestCollection(t, router, "", http.MethodGet, "/api/v1/series/999", ""), http.StatusNotFound, "SERIES_NOT_FOUND")
	assertCollectionError(t, requestCollection(t, router, ownerToken, http.MethodPost, seriesPath(series.ID)+"/items", `{"content_item_id":`+itoa64(unrelated.ID)+`}`), http.StatusBadRequest, "CONTENT_NOT_OWNED_OR_CONTRIBUTED")
	assertCollectionError(t, requestCollection(t, router, ownerToken, http.MethodPost, seriesPath(series.ID)+"/items", `{"content_item_id":`+itoa64(wrongZone.ID)+`}`), http.StatusBadRequest, "ZONE_MISMATCH")
	assertCollectionError(t, requestCollection(t, router, ownerToken, http.MethodPost, seriesPath(series.ID)+"/items", `{"content_item_id":`+itoa64(duplicate.ID)+`}`), http.StatusConflict, "DUPLICATE_SERIES_ITEM")
	assertCollectionError(t, requestCollection(t, router, ownerToken, http.MethodPut, seriesPath(series.ID), `{"cover_content_id":`+itoa64(coverOnly.ID)+`}`), http.StatusBadRequest, "COVER_NOT_IN_SERIES")

	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/series/not-an-id", ""},
		{http.MethodPost, "/api/v1/series", `{"title":"x","zone":"invalid"}`},
		{http.MethodPut, seriesPath(series.ID), `{"zone":"fanwork"}`},
		{http.MethodPost, seriesPath(series.ID) + "/items", `{}`},
		{http.MethodPut, seriesPath(series.ID) + "/items/reorder", `{"item_ids":null}`},
		{http.MethodGet, "/api/v1/series/0", ""},
		{http.MethodDelete, seriesPath(series.ID) + "/items/0", ""},
		{http.MethodPost, seriesPath(series.ID) + "/items", `{"content_item_id":0}`},
		{http.MethodPut, seriesPath(series.ID) + "/items/reorder", `{"item_ids":[-1]}`},
		{http.MethodPost, "/api/v1/series", `{"title":`},
		{http.MethodPost, "/api/v1/series", `{"title":"x","zone":"original","extra":true}`},
		{http.MethodPost, seriesPath(series.ID) + "/items", `{"content_item_id":203,"extra":true}`},
		{http.MethodPut, seriesPath(series.ID) + "/items/reorder", `{"item_ids":[],"extra":true}`},
	} {
		rec := requestCollection(t, router, ownerToken, request.method, request.path, request.body)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestContentDetailSeriesMembershipsReturnsAllVisibleNavigation(t *testing.T) {
	router, db, _ := setupSeriesHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "membership-handler-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	previous := seedCollectionHandlerContent(t, db, 100, owner.ID, "previous", "original", "article", "published", true)
	current := seedCollectionHandlerContent(t, db, 101, owner.ID, "current", "original", "article", "published", true)
	next := seedCollectionHandlerContent(t, db, 102, owner.ID, "next", "original", "article", "published", true)
	pending := seedCollectionHandlerContent(t, db, 103, owner.ID, "pending secret", "original", "article", "pending", true)
	base := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	for index := 0; index < 4; index++ {
		series := seedSeriesHandlerSeries(t, db, int64(200+index), owner.ID, "membership-"+itoa64(int64(index)), "original")
		require.NoError(t, db.Model(&series).UpdateColumn("updated_at", base.Add(time.Duration(index)*time.Minute)).Error)
		for order, content := range []model.ContentItem{pending, previous, current, next} {
			require.NoError(t, db.Create(&model.ContentSeriesItem{SeriesID: series.ID, ContentItemID: content.ID, SortOrder: order}).Error)
		}
	}

	rec := requestCollection(t, router, "", http.MethodGet, "/api/v1/contents/"+itoa64(current.ID), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), pending.Title)
	var payload struct {
		Memberships []struct {
			SeriesID     int64  `json:"series_id"`
			SeriesTitle  string `json:"series_title"`
			CurrentIndex int    `json:"current_index"`
			Total        int    `json:"total"`
			Previous     *struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
			} `json:"previous"`
			Next *struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
			} `json:"next"`
		} `json:"series_memberships"`
	}
	decodeCollectionJSON(t, rec, &payload)
	require.Len(t, payload.Memberships, 4)
	require.Equal(t, int64(203), payload.Memberships[0].SeriesID)
	for _, membership := range payload.Memberships {
		require.Equal(t, 2, membership.CurrentIndex)
		require.Equal(t, 3, membership.Total)
		require.Equal(t, previous.ID, membership.Previous.ID)
		require.Equal(t, next.ID, membership.Next.ID)
	}
}

func TestContentDetailSeriesMembershipsHidesInvisibleCurrentContent(t *testing.T) {
	router, db, _ := setupSeriesHandlerRouter(t)
	owner := seedCollectionHandlerUser(t, db, 10, "hidden-membership-owner", collectionHandlerUserState{Verified: true, Reputation: 10})
	current := seedCollectionHandlerContent(t, db, 100, owner.ID, "pending current", "original", "article", "pending", true)
	series := seedSeriesHandlerSeries(t, db, 200, owner.ID, "hidden membership title", "original")
	require.NoError(t, db.Create(&model.ContentSeriesItem{SeriesID: series.ID, ContentItemID: current.ID}).Error)

	// FIX-12+43：pending 内容匿名直接 404（比「隐藏 membership」更强的收口）。
	rec := requestCollection(t, router, "", http.MethodGet, "/api/v1/contents/"+itoa64(current.ID), "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), series.Title)
	var payload struct {
		Memberships []json.RawMessage `json:"series_memberships"`
	}
	decodeCollectionJSON(t, rec, &payload)
	require.Empty(t, payload.Memberships)
}

type seriesDetailPayload struct {
	Series struct {
		ID        int64   `json:"id"`
		Title     string  `json:"title"`
		Cover     *string `json:"cover"`
		ItemCount int64   `json:"item_count"`
		Owner     struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"owner"`
	} `json:"series"`
	Items []struct {
		ID        int64 `json:"id"`
		SortOrder int   `json:"sort_order"`
		Content   struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"content"`
	} `json:"items"`
}

func setupSeriesHandlerRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentContributor{},
		&model.ContentAttachment{}, &model.ContentTag{}, &model.BrowseHistory{},
		&model.ContentSeries{}, &model.ContentSeriesItem{},
	))

	cfg := &config.Config{}
	cfg.JWT.Secret = "series-handler-secret"
	cfg.Reputation.MinScoreForInteraction = 3
	cfg.Cache.UserStatusTTL = 300

	handler := NewSeriesHandler(db)
	optAuth := middleware.OptionalAuth(cfg, nil, db)
	authReq := middleware.AuthRequired(cfg, nil, db)
	guard := middleware.InteractionRequired(cfg, db, nil, middleware.InteractionPolicy{RequireVerifiedEmail: true, RequireReputation: true})
	router := gin.New()
	v1 := router.Group("/api/v1")
	v1.POST("/series", authReq, guard, handler.CreateSeries)
	v1.GET("/series", authReq, handler.ListSeries)
	v1.GET("/series/candidates", authReq, handler.ListCandidates)
	v1.GET("/series/:id", optAuth, handler.GetSeries)
	v1.PUT("/series/:id", authReq, guard, handler.UpdateSeries)
	v1.DELETE("/series/:id", authReq, guard, handler.DeleteSeries)
	v1.POST("/series/:id/items", authReq, guard, handler.AddItem)
	v1.DELETE("/series/:id/items/:itemId", authReq, guard, handler.RemoveItem)
	v1.PUT("/series/:id/items/reorder", authReq, guard, handler.ReorderItems)
	contentHandler := NewContentHandler(db, cfg, nil)
	v1.GET("/contents/:id", optAuth, contentHandler.GetContent)
	return router, db, cfg
}

func seedSeriesHandlerSeries(t *testing.T, db *gorm.DB, id, ownerID int64, title, zone string) model.ContentSeries {
	t.Helper()
	series := model.ContentSeries{ID: id, OwnerID: ownerID, Title: title, Zone: zone}
	require.NoError(t, db.Create(&series).Error)
	return series
}

func seriesPath(id int64) string { return "/api/v1/series/" + itoa64(id) }

func itoa64(id int64) string {
	encoded, _ := json.Marshal(id)
	return strings.Trim(string(encoded), `"`)
}
