package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// SP-16 #447: the public guide endpoint contract — merged template+specifics
// view, ETag/304 + s-maxage caching, anonymous visibility gate, author-only
// studio write path.
func setupUsageGuideTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentUsageGuide{},
	))

	cfg := &config.Config{}
	cfg.JWT.Secret = "usage-guide-test-secret"

	contentRepo := repository.NewContentRepository(db)
	guideSvc := service.NewUsageGuideService(repository.NewUsageGuideRepository(db), contentRepo)
	h := NewUsageGuideHandler(guideSvc, contentRepo)

	optAuth := middleware.OptionalAuth(cfg, nil, db)
	authReq := middleware.AuthRequired(cfg, nil, db)

	r := gin.New()
	r.GET("/api/v1/contents/:id/guide", optAuth, h.GetGuide)
	r.GET("/api/v1/contents/:id/guide/specifics", authReq, h.GetAuthorGuide)
	r.PUT("/api/v1/contents/:id/guide", authReq, h.SaveGuide)
	return r, db, cfg
}

func seedUsageGuideFixtures(t *testing.T, db *gorm.DB) (pubID, privID, pendingID, authorID, otherID int64) {
	t.Helper()
	now := time.Now()
	author := model.User{ID: 701, Email: "ug-author@example.com", Username: "ug_author", PasswordHash: "x", Reputation: 10, Role: "user", EmailVerifiedAt: &now}
	other := model.User{ID: 702, Email: "ug-other@example.com", Username: "ug_other", PasswordHash: "x", Reputation: 10, Role: "user", EmailVerifiedAt: &now}
	require.NoError(t, db.Create(&author).Error)
	require.NoError(t, db.Create(&other).Error)

	pub := model.ContentItem{ID: 711, Title: "ug pub", AuthorID: 701, Zone: "original", ContentType: "mod", Status: "published", IsPublic: true}
	priv := model.ContentItem{ID: 712, Title: "ug priv", AuthorID: 701, Zone: "original", ContentType: "mod", Status: "published", IsPublic: false}
	pending := model.ContentItem{ID: 713, Title: "ug pending", AuthorID: 701, Zone: "original", ContentType: "mod", Status: "pending", IsPublic: true}
	require.NoError(t, db.Create(&pub).Error)
	require.NoError(t, db.Create(&priv).Error)
	require.NoError(t, db.Create(&pending).Error)
	return 711, 712, 713, 701, 702
}

func usageGuideGet(t *testing.T, r *gin.Engine, path, token, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func usageGuideToken(t *testing.T, cfg *config.Config, userID int64) string {
	t.Helper()
	pair, err := jwtutil.GenerateTokenPair(userID, "user", cfg.JWT.Secret, 120, 7)
	require.NoError(t, err)
	return pair.AccessToken
}

func TestUsageGuideEndpointContract(t *testing.T) {
	r, db, cfg := setupUsageGuideTestRouter(t)
	pubID, privID, pendingID, authorID, otherID := seedUsageGuideFixtures(t, db)

	t.Run("anonymous merged view degrades to pure template with cache headers", func(t *testing.T) {
		rec := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), "", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Header().Get("Cache-Control"), "s-maxage=300")
		etag := rec.Header().Get("ETag")
		require.NotEmpty(t, etag)
		body := rec.Body.String()
		require.Contains(t, body, `"steps"`)
		require.Contains(t, body, `"safety"`)
		require.Contains(t, body, `"template_version"`)
		require.NotContains(t, body, `"source"`, "pure template view must not claim a specifics source")
	})

	t.Run("conditional request returns 304 with the same etag", func(t *testing.T) {
		first := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), "", "")
		etag := first.Header().Get("ETag")
		require.NotEmpty(t, etag)
		second := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), "", etag)
		require.Equal(t, http.StatusNotModified, second.Code)
		require.Equal(t, etag, second.Header().Get("ETag"))
		require.Empty(t, second.Body.String())
	})

	t.Run("non-public content 404s anonymously and serves the author", func(t *testing.T) {
		for _, id := range []int64{privID, pendingID} {
			rec := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide", id), "", "")
			require.Equal(t, http.StatusNotFound, rec.Code, "id=%d", id)
		}
		author := usageGuideToken(t, cfg, authorID)
		rec := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide", privID), author, "")
		require.Equal(t, http.StatusOK, rec.Code, "author keeps guide access to own private content")
	})

	t.Run("author saves specifics; merged view prefers specifics and rotates etag", func(t *testing.T) {
		before := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), "", "")
		etagBefore := before.Header().Get("ETag")

		author := usageGuideToken(t, cfg, authorID)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), strings.NewReader(`{
			"locale": "zh",
			"requirements": ["Minecraft 1.20.1+", "Forge 47"],
			"steps": ["备份存档", "把 jar 放进 mods 目录"],
			"notes": "冲突时先移除旧版。",
			"source": "author"
		}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+author)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		after := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), "", "")
		require.Equal(t, http.StatusOK, after.Code)
		body := after.Body.String()
		require.Contains(t, body, "备份存档")
		require.Contains(t, body, `"source":"author"`)
		require.Contains(t, body, `"safety"`, "safety floor stays served from the template")
		require.NotEqual(t, etagBefore, after.Header().Get("ETag"), "saving specifics must rotate the etag")
	})

	t.Run("non-author cannot save", func(t *testing.T) {
		other := usageGuideToken(t, cfg, otherID)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), strings.NewReader(`{"locale":"zh","steps":["x"],"source":"author"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+other)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("invalid locale and payload rejected", func(t *testing.T) {
		author := usageGuideToken(t, cfg, authorID)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/contents/%d/guide", pubID), strings.NewReader(`{"locale":"fr","steps":["x"]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+author)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("en locale serves the en template", func(t *testing.T) {
		rec := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide?locale=en", pubID), "", "")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"locale":"en"`)
	})

	t.Run("author specifics endpoint returns the saved row", func(t *testing.T) {
		author := usageGuideToken(t, cfg, authorID)
		rec := usageGuideGet(t, r, fmt.Sprintf("/api/v1/contents/%d/guide/specifics?locale=zh", pubID), author, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "备份存档")
	})
}
