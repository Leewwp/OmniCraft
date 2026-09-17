package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func setupAdminLLMCostRouter(t *testing.T) (*gin.Engine, *repository.AgentTraceRepository, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AgentTraceRun{}, &model.AgentTraceNode{}))
	cfg := &config.Config{}
	cfg.JWT.Secret = "llm-cost-admin-contract-secret"
	// Rate table: only deepseek-chat carries rates; minimax-m3 stays
	// "unknown" so its tokens aggregate without an estimate.
	cfg.Agent.Models = []config.AgentModelConfig{
		{ID: "deepseek", Model: "deepseek-chat", CostInPerMTokens: 2, CostOutPerMTokens: 8},
	}
	repo := repository.NewAgentTraceRepository(db)
	handler := NewAdminLLMCostHandler(repo, cfg)
	admin := model.User{Email: "llm-cost-admin@example.com", Username: "llm-cost-admin", PasswordHash: "hash", Role: "admin"}
	require.NoError(t, db.Create(&admin).Error)
	token := mustToken(t, cfg, admin.ID, admin.Role)
	router := gin.New()
	group := router.Group("/api/v1/admin", middleware.AuthRequired(cfg, nil, db), middleware.AdminRequired())
	group.GET("/llm-costs", handler.Ledger)
	return router, repo, token
}

func int64PtrCost(v int64) *int64 { return &v }

func seedCostNodes(t *testing.T, repo *repository.AgentTraceRepository) {
	t.Helper()
	ctx := t.Context()
	day1 := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	runs := []model.AgentTraceRun{
		{TraceID: "t-a", Status: model.AgentTraceStatusSuccess, StartedAt: day1, ConversationID: int64PtrCost(42)},
		{TraceID: "t-b", Status: model.AgentTraceStatusSuccess, StartedAt: day2, ConversationID: int64PtrCost(42)},
		{TraceID: "t-c", Status: model.AgentTraceStatusSuccess, StartedAt: day1, ConversationID: int64PtrCost(43)},
	}
	require.NoError(t, repo.UpsertRuns(ctx, runs))
	i500, o100 := int64(500), int64(100)
	i1000, o200 := int64(1000), int64(200)
	i300, o60 := int64(300), int64(60)
	nodes := []model.AgentTraceNode{
		// Priced model: 500 in / 100 out @ (2, 8) => 0.0018 CNY.
		{TraceID: "t-a", NodeKey: "llm_round_1", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day1, Model: "deepseek-chat", TokensIn: &i500, TokensOut: &o100},
		// Unknown-rate model: tokens count, cost stays 0, unpriced.
		{TraceID: "t-c", NodeKey: "llm_round_1", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day1, Model: "minimax-m3", TokensIn: &i1000, TokensOut: &o200},
		// Second priced cell on day 2 for conv 42.
		{TraceID: "t-b", NodeKey: "llm_round_1", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day2, Model: "deepseek-chat", TokensIn: &i300, TokensOut: &o60},
	}
	require.NoError(t, repo.UpsertNodes(ctx, nodes))
}

func llmCostResponse(t *testing.T, router *gin.Engine, token, query string) (int, map[string]json.RawMessage) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/llm-costs"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	body := map[string]json.RawMessage{}
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	}
	return rec.Code, body
}

func TestAdminLLMCostLedger(t *testing.T) {
	router, repo, token := setupAdminLLMCostRouter(t)
	seedCostNodes(t, repo)

	code, body := llmCostResponse(t, router, token, "?from=2026-09-16T00:00:00Z&to=2026-09-18T00:00:00Z")
	require.Equal(t, http.StatusOK, code)

	var totals struct {
		TokensIn       int64   `json:"tokens_in"`
		TokensOut      int64   `json:"tokens_out"`
		CostCNY        float64 `json:"cost_cny"`
		UnpricedModels int     `json:"unpriced_models"`
	}
	require.NoError(t, json.Unmarshal(body["totals"], &totals))
	require.Equal(t, int64(1800), totals.TokensIn)
	require.Equal(t, int64(360), totals.TokensOut)
	// 800 in / 160 out @ (2, 8) CNY per 1M tokens => 0.00288.
	require.InDelta(t, 0.00288, totals.CostCNY, 1e-12)
	require.Equal(t, 1, totals.UnpricedModels)

	var rates map[string]llmCostRateView
	require.NoError(t, json.Unmarshal(body["rates"], &rates))
	require.Equal(t, llmCostRateView{In: 2, Out: 8}, rates["deepseek-chat"])
	require.NotContains(t, rates, "minimax-m3")

	var byModel []llmCostModelRow
	require.NoError(t, json.Unmarshal(body["by_model"], &byModel))
	require.Len(t, byModel, 2)
	// Sorted by token volume: minimax (1200/240) first, deepseek second.
	require.Equal(t, "minimax-m3", byModel[0].Model)
	require.False(t, byModel[0].Estimated)
	require.Equal(t, "deepseek-chat", byModel[1].Model)
	require.True(t, byModel[1].Estimated)
	require.InDelta(t, 0.00288, byModel[1].CostCNY, 1e-12)

	var byDay []llmCostDayRow
	require.NoError(t, json.Unmarshal(body["by_day"], &byDay))
	require.Len(t, byDay, 2)
	require.Equal(t, "2026-09-16", byDay[0].Day)
	require.InDelta(t, 0.0018, byDay[0].CostCNY, 1e-12)
	require.Equal(t, "2026-09-17", byDay[1].Day)
	require.InDelta(t, 0.00108, byDay[1].CostCNY, 1e-12)

	var convs []llmCostConversationRow
	require.NoError(t, json.Unmarshal(body["top_conversations"], &convs))
	require.Len(t, convs, 2)
	// Conv 42 (priced, 0.00288) ranks above conv 43 (unpriced, 0 cost).
	require.Equal(t, int64(42), convs[0].ConversationID)
	require.Equal(t, int64(2), convs[0].Turns)
	require.True(t, convs[0].Estimated)
	require.Equal(t, int64(43), convs[1].ConversationID)
	require.Equal(t, int64(1), convs[1].Turns)
	require.False(t, convs[1].Estimated)

	// Window bounds prune: only day 2 survives.
	code, body = llmCostResponse(t, router, token, "?from=2026-09-17T00:00:00Z&to=2026-09-18T00:00:00Z")
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, json.Unmarshal(body["by_day"], &byDay))
	require.Len(t, byDay, 1)
	require.Equal(t, "2026-09-17", byDay[0].Day)

	// Bad bounds are 400, empty ledger is 200 with empty arrays.
	require.Equal(t, http.StatusBadRequest, llmCostResponseCode(t, router, token, "?from=not-a-time"))
}

func llmCostResponseCode(t *testing.T, router *gin.Engine, token, query string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/llm-costs"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

func TestAdminLLMCostLedgerEmpty(t *testing.T) {
	router, repo, token := setupAdminLLMCostRouter(t)
	_ = repo

	code, body := llmCostResponse(t, router, token, "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "[]", string(body["by_model"]))
	require.Equal(t, "[]", string(body["by_day"]))
	require.Equal(t, "[]", string(body["top_conversations"]))

	// Admin guard: anonymous gets 401.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/llm-costs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
