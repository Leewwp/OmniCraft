package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

func setupAdminTraceRouter(t *testing.T) (*gin.Engine, *repository.AgentTraceRepository, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AgentTraceRun{}, &model.AgentTraceNode{}))
	cfg := &config.Config{}
	cfg.JWT.Secret = "trace-admin-contract-secret"
	repo := repository.NewAgentTraceRepository(db)
	handler := NewAdminTraceHandler(repo)
	admin := model.User{Email: "trace-admin@example.com", Username: "trace-admin", PasswordHash: "hash", Role: "admin"}
	require.NoError(t, db.Create(&admin).Error)
	token := mustToken(t, cfg, admin.ID, admin.Role)
	router := gin.New()
	group := router.Group("/api/v1/admin", middleware.AuthRequired(cfg, nil, db), middleware.AdminRequired())
	group.GET("/traces", handler.ListTraces)
	group.GET("/traces/stats", handler.Stats)
	return router, repo, token
}

func seedTraceRuns(t *testing.T, repo *repository.AgentTraceRepository) {
	t.Helper()
	ctx := t.Context()
	now := time.Now()
	mk := func(trace string, minsAgo int, status string, dur, ttft int64, modelName, kind, routing string) {
		started := now.Add(-time.Duration(minsAgo) * time.Minute)
		ended := started.Add(time.Duration(dur) * time.Millisecond)
		user := int64(7)
		require.NoError(t, repo.UpsertRuns(ctx, []model.AgentTraceRun{{
			TraceID: trace, Status: status, StartedAt: started, EndedAt: &ended,
			DurationMs: &dur, TTFTMs: &ttft, Model: modelName, AnswerKind: kind,
			UserID: &user, Surface: "global", RoutingEvents: model.JSONB(routing),
		}}))
	}
	mk("aaaa1111aaaa1111aaaa1111aaaa1111", 5, model.AgentTraceStatusSuccess, 1200, 300, "minimax-m3", "grounded_content", "[]")
	mk("bbbb2222bbbb2222bbbb2222bbbb2222", 10, model.AgentTraceStatusSuccess, 2400, 500, "minimax-m3", "grounded_content", `[{"from":"minimax-m3","to":"deepseek-chat","reason":"provider_error"}]`)
	mk("cccc3333cccc3333cccc3333cccc3333", 20, model.AgentTraceStatusError, 800, 0, "deepseek-chat", "", "[]")
	mk("dddd4444dddd4444dddd4444dddd4444", 40, model.AgentTraceStatusSuccess, 3600, 700, "minimax-m3", "conversational", "[]")
}

func TestAdminTraceListFiltersAndPagination(t *testing.T) {
	router, repo, token := setupAdminTraceRouter(t)
	seedTraceRuns(t, repo)

	get := func(query string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/traces"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	// No filter: all four runs, newest first.
	require.Equal(t, http.StatusOK, get(""))
	// Exact trace id hit.
	require.Equal(t, http.StatusOK, get("?trace_id=aaaa1111aaaa1111aaaa1111aaaa1111"))
	// Status filter.
	require.Equal(t, http.StatusOK, get("?status=ERROR"))
	// Model + answer kind.
	require.Equal(t, http.StatusOK, get("?model=minimax-m3&answer_kind=conversational"))
	// Pagination.
	require.Equal(t, http.StatusOK, get("?page=2&page_size=2"))
	// Invalid numerics are 400, not silent ignore.
	require.Equal(t, http.StatusBadRequest, get("?user_id=abc"))
	require.Equal(t, http.StatusBadRequest, get("?conversation_id=-3"))
	require.Equal(t, http.StatusBadRequest, get("?page_size=1000"))
	require.Equal(t, http.StatusBadRequest, get("?from=not-a-time"))

	// Verify filter semantics through the body.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/traces?status=ERROR", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "cccc3333cccc3333cccc3333cccc3333")
	require.NotContains(t, rec.Body.String(), "aaaa1111aaaa1111aaaa1111aaaa1111")
}

func TestAdminTraceStatsGlobalAggregates(t *testing.T) {
	router, repo, token := setupAdminTraceRouter(t)
	seedTraceRuns(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/traces/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Success rate over terminal runs = 3 SUCCESS of 4 terminal (3 SUCCESS +
	// 1 ERROR), RUNNING excluded entirely.
	require.Contains(t, body, `"success_rate":0.75`)
	// Routing fallback rate = 1 routed run of 4 total.
	require.Contains(t, body, `"routing_rate":0.25`)
	require.Contains(t, body, `"routing_fallbacks":1`)
	// Durations 1200/2400/800/3600 -> avg 2000, p95 >= 3600*0.85.
	require.Contains(t, body, `"avg_duration_ms":2000`)
	require.Contains(t, body, `"total_runs":4`)
	// TTFT mean over 4 = (300+500+0+700)/4 = 375.
	require.Contains(t, body, `"avg_ttft_ms":375`)

	// Bounded window excludes the 40-minute-old run.
	from := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/traces/stats?from="+url.QueryEscape(from), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"total_runs":3`)
}

func TestAdminTraceRequiresAdmin(t *testing.T) {
	router, _, _ := setupAdminTraceRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/traces", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
