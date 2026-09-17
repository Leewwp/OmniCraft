package handler

import (
	"bytes"
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
	"omnicraft/backend/internal/service"
)

// setupAdminEvalRouter wires the E5 eval endpoints against an in-memory
// sqlite database with one traced conversation turn: user question, answer
// message with validated citations, and the matching agent_trace_runs row.
func setupAdminEvalRouter(t *testing.T) (*gin.Engine, *gorm.DB, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.AdminAuditLog{}, &model.AgentTraceRun{},
		&model.EvalGoldenCase{}, &model.EvalRun{}, &model.AgentMessage{},
		&model.AgentConversation{},
	))
	cfg := &config.Config{}
	cfg.JWT.Secret = "eval-admin-contract-secret"

	evalRepo := repository.NewRagEvaluationRepository(db)
	traceRepo := repository.NewAgentTraceRepository(db)
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	h := NewAdminEvalHandler(db, evalRepo, traceRepo, auditSvc)

	admin := model.User{Email: "eval-admin@example.com", Username: "eval-admin", PasswordHash: "hash", Role: "admin"}
	require.NoError(t, db.Create(&admin).Error)
	token := mustToken(t, cfg, admin.ID, admin.Role)
	router := gin.New()
	group := router.Group("/api/v1/admin", middleware.AuthRequired(cfg, nil, db), middleware.AdminRequired())
	group.GET("/evals/runs", h.ListRuns)
	group.GET("/evals/drafts", h.ListDrafts)
	group.POST("/evals/drafts", h.CreateDraftFromTrace)
	group.DELETE("/evals/drafts/:case_key", h.DeleteDraft)

	question := "测试问题：灯塔老人五十年点灯的短篇"
	answer := "找到了，是《灯火学会看海的地方》[1]。"
	conversation := model.AgentConversation{}
	require.NoError(t, db.Create(&conversation).Error)
	userMsg := model.AgentMessage{ConversationID: conversation.ID, Role: "user", Content: &question}
	require.NoError(t, db.Create(&userMsg).Error)
	answerMsg := model.AgentMessage{
		ConversationID: conversation.ID, Role: "assistant", Content: &answer,
		Citations: []model.AgentCitation{{
			ContentID: 1491, ContentVersion: 1, ChunkKey: "ck-1", ChunkIndex: 0,
			Title: "灯火学会看海的地方", Zone: "original",
		}},
	}
	require.NoError(t, db.Create(&answerMsg).Error)

	traceID := "traceval0000000001"
	started := time.Now().UTC()
	require.NoError(t, db.Create(&model.AgentTraceRun{
		TraceID: traceID, ConversationID: &conversation.ID, MessageID: &answerMsg.ID,
		UserID: &admin.ID, Surface: "web", Status: "SUCCESS",
		StartedAt: started, EndedAt: &started, Model: "MiniMax-M3", AnswerKind: "grounded_content",
	}).Error)
	return router, db, token, traceID
}

func doEvalRequest(t *testing.T, router *gin.Engine, token, method, path string, body any) *httptest.ResponseRecorder {
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAdminEvalDraftLifecycle(t *testing.T) {
	router, db, token, traceID := setupAdminEvalRouter(t)

	// create from trace
	w := doEvalRequest(t, router, token, http.MethodPost, "/api/v1/admin/evals/drafts", map[string]string{"trace_id": traceID})
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct {
		CaseKey string                `json:"case_key"`
		Created bool                  `json:"created"`
		Draft   model.EvalGoldenCase  `json:"draft"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.True(t, created.Created)
	require.Equal(t, "draft-trace-"+traceID, created.CaseKey)
	require.Equal(t, "draft", created.Draft.Status)
	require.Equal(t, traceID, created.Draft.SourceTraceID)
	require.Contains(t, string(created.Draft.ExpectedCitations), "1491")

	// idempotent: same trace again keeps the first draft
	w = doEvalRequest(t, router, token, http.MethodPost, "/api/v1/admin/evals/drafts", map[string]string{"trace_id": traceID})
	require.Equal(t, http.StatusOK, w.Code)
	var again struct {
		CaseKey string `json:"case_key"`
		Created bool   `json:"created"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &again))
	require.False(t, again.Created)
	require.Equal(t, created.CaseKey, again.CaseKey)

	// drafts list shows it
	w = doEvalRequest(t, router, token, http.MethodGet, "/api/v1/admin/evals/drafts", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Drafts []model.EvalGoldenCase `json:"drafts"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Drafts, 1)

	// the frozen-set reader never sees the draft
	count, err := repository.NewRagEvaluationRepository(db).ListActiveGoldenCases(t.Context())
	require.NoError(t, err)
	require.Empty(t, count)

	// unknown trace -> 404
	w = doEvalRequest(t, router, token, http.MethodPost, "/api/v1/admin/evals/drafts", map[string]string{"trace_id": "no-such-trace-id"})
	require.Equal(t, http.StatusNotFound, w.Code)

	// delete draft
	w = doEvalRequest(t, router, token, http.MethodDelete, "/api/v1/admin/evals/drafts/"+created.CaseKey, nil)
	require.Equal(t, http.StatusOK, w.Code)
	w = doEvalRequest(t, router, token, http.MethodDelete, "/api/v1/admin/evals/drafts/"+created.CaseKey, nil)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminEvalRunsList(t *testing.T) {
	router, db, token, _ := setupAdminEvalRouter(t)
	repo := repository.NewRagEvaluationRepository(db)
	require.NoError(t, repo.UpsertEvalRun(t.Context(), &model.EvalRun{
		RunKey: "grid-g0-dev", DatasetChecksum: "abc", RetrieverVersion: "v1",
		ChunkingVersion: "c1", IndexVersion: "i1",
		Metrics: model.JSONB(`{"retrieval_headline":{"mrr":0.93}}`),
		Environment:     model.JSONB(`{"axes":{"rrf_k":60}}`),
		ArtifactPath:    "artifacts/x.jsonl",
	}))

	w := doEvalRequest(t, router, token, http.MethodGet, "/api/v1/admin/evals/runs", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Runs []model.EvalRun `json:"runs"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out.Runs, 1)
	require.Equal(t, "grid-g0-dev", out.Runs[0].RunKey)
	// JSONB must surface as raw JSON, not base64 (SP-21 T3 lesson)
	require.Contains(t, w.Body.String(), `"retrieval_headline"`)
}
