package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/testutil"
)

// setupAgentTokenHandlerTest mounts the token-management routes exactly as
// routes.go does (authReq + RequireJWTChannel) so the PAT-rejection contract
// is exercised through the real wiring.
func setupAgentTokenHandlerTest(t *testing.T) (*gin.Engine, *service.AgentAccessTokenService, *gorm.DB, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			email_verified_at TIMESTAMPTZ,
			reputation INT NOT NULL DEFAULT 10,
			deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "079_agent_access_tokens.sql"))

	cfg := &config.Config{
		JWT:         config.JWTConfig{Secret: "agent-token-handler-test-secret"},
		AgentAccess: config.AgentAccessConfig{MaxTokensPerUser: 5},
	}

	var userID int64
	require.NoError(t, db.Raw(`
		INSERT INTO users (email, username, email_verified_at, reputation)
		VALUES ('pathandler@example.com', 'pathandler', NOW(), 10) RETURNING id
	`).Scan(&userID).Error)

	svc := service.NewAgentAccessTokenService(repository.NewAgentAccessTokenRepository(db), cfg)
	handler := NewAgentAccessTokenHandler(svc)

	engine := gin.New()
	v1 := engine.Group("/api/v1")
	users := v1.Group("/users")
	{
		users.GET("/me/agent-tokens", func(c *gin.Context) {
			c.Set(middleware.UserIDKey, userID)
			c.Next()
		}, middleware.RequireJWTChannel(), handler.List)
		users.POST("/me/agent-tokens", func(c *gin.Context) {
			c.Set(middleware.UserIDKey, userID)
			c.Next()
		}, middleware.RequireJWTChannel(), handler.Create)
		users.DELETE("/me/agent-tokens/:id", func(c *gin.Context) {
			c.Set(middleware.UserIDKey, userID)
			c.Next()
		}, middleware.RequireJWTChannel(), handler.Revoke)
	}

	return engine, svc, db, userID
}

func agentTokenRequest(t *testing.T, engine *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var payload []byte
	switch b := body.(type) {
	case nil:
	case string:
		payload = []byte(b)
	default:
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		payload = raw
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	out := map[string]any{}
	if bytes.HasPrefix(w.Body.Bytes(), []byte("{")) {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	}
	return w, out
}

func TestAgentTokenHandlerCreateListRevoke(t *testing.T) {
	engine, svc, _, userID := setupAgentTokenHandlerTest(t)

	// create: plaintext returned exactly once next to the projection
	w, body := agentTokenRequest(t, engine, http.MethodPost, "/api/v1/users/me/agent-tokens", map[string]any{
		"name":   "Claude Code",
		"scopes": []string{"download", "upload"},
	})
	require.Equal(t, http.StatusCreated, w.Code)
	token, ok := body["token"].(string)
	require.True(t, ok, "create must return the one-time plaintext token")
	require.Equal(t, "oc_pat_", token[:7])
	require.Len(t, token, 50)
	info, ok := body["token_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Claude Code", info["name"])
	require.Equal(t, token[:12], info["token_prefix"])

	// the issued token authenticates (chain works end to end)
	identity, err := svc.Authenticate(t.Context(), token)
	require.NoError(t, err)
	require.Equal(t, userID, identity.UserID)

	// list: no plaintext, prefix only
	w, body = agentTokenRequest(t, engine, http.MethodGet, "/api/v1/users/me/agent-tokens", nil)
	require.Equal(t, http.StatusOK, w.Code)
	tokens, ok := body["tokens"].([]any)
	require.True(t, ok)
	require.Len(t, tokens, 1)
	first := tokens[0].(map[string]any)
	require.Equal(t, token[:12], first["token_prefix"])
	require.NotContains(t, first, "token")
	blob, err := json.Marshal(body)
	require.NoError(t, err)
	require.NotContains(t, string(blob), token)

	// revoke
	tokenID := int64(info["id"].(float64))
	w, _ = agentTokenRequest(t, engine, http.MethodDelete, "/api/v1/users/me/agent-tokens/"+jsonIntString(tokenID), nil)
	require.Equal(t, http.StatusOK, w.Code)

	_, err = svc.Authenticate(t.Context(), token)
	require.ErrorIs(t, err, service.ErrAgentTokenNotFound)

	// revoked token disappears from the list
	w, body = agentTokenRequest(t, engine, http.MethodGet, "/api/v1/users/me/agent-tokens", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, body["tokens"])
}

func TestAgentTokenHandlerValidation(t *testing.T) {
	engine, _, _, _ := setupAgentTokenHandlerTest(t)

	w, body := agentTokenRequest(t, engine, http.MethodPost, "/api/v1/users/me/agent-tokens", map[string]any{
		"name":   "",
		"scopes": []string{"download"},
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "AGENT_TOKEN_INVALID", body["code"])

	w, body = agentTokenRequest(t, engine, http.MethodPost, "/api/v1/users/me/agent-tokens", map[string]any{
		"name":   "bad scopes",
		"scopes": []string{"admin"},
	})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "AGENT_TOKEN_INVALID", body["code"])

	w, body = agentTokenRequest(t, engine, http.MethodPost, "/api/v1/users/me/agent-tokens", "{not-json")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "INVALID_REQUEST", body["code"])

	// unknown revoke target
	w, body = agentTokenRequest(t, engine, http.MethodDelete, "/api/v1/users/me/agent-tokens/999", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, "AGENT_TOKEN_NOT_FOUND", body["code"])

	// malformed id
	w, _ = agentTokenRequest(t, engine, http.MethodDelete, "/api/v1/users/me/agent-tokens/not-a-number", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func jsonIntString(v int64) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
