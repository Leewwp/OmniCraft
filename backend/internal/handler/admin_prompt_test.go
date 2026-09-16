package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/service/promptregistry"
)

func setupAdminPromptRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AdminAuditLog{}, &model.PromptRegistry{}, &model.PromptLabel{}))
	cfg := &config.Config{}
	cfg.JWT.Secret = "prompt-admin-contract-secret"

	repo := repository.NewPromptRegistryRepository(db)
	require.NoError(t, promptregistry.SeedV1(t.Context(), repo))
	resolver := promptregistry.NewPromptResolver(repo)
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	handler := NewAdminPromptHandler(repo, resolver, auditSvc)

	admin := model.User{Email: "prompt-admin@example.com", Username: "prompt-admin", PasswordHash: "hash", Role: "admin"}
	require.NoError(t, db.Create(&admin).Error)
	token := mustToken(t, cfg, admin.ID, admin.Role)
	router := gin.New()
	group := router.Group("/api/v1/admin", middleware.AuthRequired(cfg, nil, db), middleware.AdminRequired())
	group.GET("/prompts", handler.ListSlots)
	group.GET("/prompts/:name/versions", handler.ListVersions)
	group.POST("/prompts/:name/versions", handler.CreateVersion)
	group.POST("/prompts/:name/labels", handler.SetLabel)
	return router, db, token
}

func doPromptRequest(t *testing.T, router *gin.Engine, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAdminPromptFullFlow(t *testing.T) {
	router, db, token := setupAdminPromptRouter(t)

	// Slot overview exposes every seeded slot with production at v1.
	rec := doPromptRequest(t, router, token, http.MethodGet, "/api/v1/admin/prompts", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var listResp struct {
		Slots []struct {
			Name              string `json:"name"`
			ProductionVersion int    `json:"production_version"`
			LatestVersion     int    `json:"latest_version"`
		} `json:"slots"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	require.Len(t, listResp.Slots, len(promptregistry.Slots))
	for _, slot := range listResp.Slots {
		require.Equal(t, 1, slot.ProductionVersion, slot.Name)
		require.Equal(t, 1, slot.LatestVersion, slot.Name)
	}

	// Version history lists v1 with labels.
	rec = doPromptRequest(t, router, token, http.MethodGet, "/api/v1/admin/prompts/conversation_title_prompt/versions", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	// Unknown slot is a 404, never a 500.
	rec = doPromptRequest(t, router, token, http.MethodGet, "/api/v1/admin/prompts/nope/versions", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)

	// A version violating the placeholder contract is rejected before any
	// write (missing required + unknown token both fail).
	rec = doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/versions", map[string]string{
		"content": "标题：{{wrong}}",
	})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	rec = doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/versions", map[string]string{
		"content": "no placeholder",
	})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	// A valid v2 is created immutably.
	rec = doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/versions", map[string]string{
		"content": "生成标题：{{first_user_message}}",
	})
	require.Equal(t, http.StatusCreated, rec.Code)
	var createResp struct {
		Version int `json:"version"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createResp))
	require.Equal(t, 2, createResp.Version)

	// Staging points at v2, then production moves (rollback = move back).
	rec = doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/labels", map[string]any{
		"label": "staging", "version": 2,
	})
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/labels", map[string]any{
		"label": "production", "version": 2,
	})
	require.Equal(t, http.StatusOK, rec.Code)

	// Moving to a nonexistent version is a 404.
	rec = doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/labels", map[string]any{
		"label": "production", "version": 99,
	})
	require.Equal(t, http.StatusNotFound, rec.Code)

	// Unknown label value is rejected.
	rec = doPromptRequest(t, router, token, http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/labels", map[string]any{
		"label": "canary", "version": 1,
	})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	// Both writes left audit rows attributed to the admin.
	var audits []model.AdminAuditLog
	require.NoError(t, db.Where("target_type = ?", "prompt").Order("id").Find(&audits).Error)
	require.GreaterOrEqual(t, len(audits), 3)
	require.Equal(t, "prompt.version.create", audits[0].Action)
	require.Equal(t, "prompt.label.set", audits[1].Action)

	// The production move is live through the resolver (invalidate ran).
	rec = doPromptRequest(t, router, token, http.MethodGet, "/api/v1/admin/prompts", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	for _, slot := range listResp.Slots {
		if slot.Name == "conversation_title_prompt" {
			require.Equal(t, 2, slot.ProductionVersion)
		}
	}
}

// Non-admin callers are rejected by the route group (admin guard contract).
func TestAdminPromptRequiresAdmin(t *testing.T) {
	router, db, _ := setupAdminPromptRouter(t)
	pleb := model.User{Email: "prompt-pleb@example.com", Username: "prompt-pleb", PasswordHash: "hash", Role: "user"}
	require.NoError(t, db.Create(&pleb).Error)
	cfg := &config.Config{}
	cfg.JWT.Secret = "prompt-admin-contract-secret"
	token := mustToken(t, cfg, pleb.ID, pleb.Role)
	rec := doPromptRequest(t, router, token, http.MethodGet, "/api/v1/admin/prompts", nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
}
