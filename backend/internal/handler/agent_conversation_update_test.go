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

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/service"
)

// agentConvTestEnv wires the agent handler routes with a fixed viewer id so
// owner-scoping can be exercised per request.
type agentConvTestEnv struct {
	db       *gorm.DB
	provider *recordingAgentHTTPProvider
}

func newAgentConversationTestEnv(t *testing.T) (*agentConvTestEnv, func(viewer int64) *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// :memory: databases are per-connection; pin one connection so the
	// continuation ownership checks and the async title goroutine share the
	// same schema.
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}))

	provider := &recordingAgentHTTPProvider{deltas: []llm.ChatDelta{{Content: "answer"}, {Done: true}}}
	cfg := &config.Config{
		Agent: config.AgentConfig{
			WebAgentEnabled:        true,
			MaxUserMessageChars:    64,
			ChatMaxContextMsgs:     10,
			MaxToolCallsPerTurn:    8,
			MaxOutputTokens:        1200,
			CitationMaxCount:       5,
			ChatContextTokenBudget: 100000,
			RateLimitPerMinute:     5,
			RateLimitPerDay:        50,
			ConversationListLimit:  50,
			ConversationPageSize:   50,
		},
		RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60},
	}
	handler := &AgentHandler{
		agentSvc: service.NewAgentService(provider, nil, nil, nil, db, cfg),
		cfg:      cfg,
		db:       db,
	}
	quota, quotaCleanup := newTestQuotaReserver(t, cfg)
	t.Cleanup(quotaCleanup)
	handler.quota = quota

	buildRouter := func(viewer int64) *gin.Engine {
		router := gin.New()
		router.POST("/agent/chat/stream", func(c *gin.Context) {
			c.Set(middleware.UserIDKey, viewer)
			handler.ChatStream(c)
		})
		router.GET("/agent/conversations", func(c *gin.Context) {
			c.Set(middleware.UserIDKey, viewer)
			handler.ListConversations(c)
		})
		router.PATCH("/agent/conversations/:id", func(c *gin.Context) {
			c.Set(middleware.UserIDKey, viewer)
			handler.UpdateConversation(c)
		})
		return router
	}
	return &agentConvTestEnv{db: db, provider: provider}, buildRouter
}

func (e *agentConvTestEnv) do(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func seedConversation(t *testing.T, db *gorm.DB, userID int64, title *string, pinnedAt *time.Time, updatedAt time.Time) model.AgentConversation {
	t.Helper()
	conv := model.AgentConversation{UserID: userID, ContextType: "general", Title: title, PinnedAt: pinnedAt, CreatedAt: updatedAt, UpdatedAt: updatedAt}
	require.NoError(t, db.Create(&conv).Error)
	return conv
}

func strPtrOf(s string) *string { return &s }

func timePtr(t time.Time) *time.Time { return &t }

func convPath(id int64) string { return "/agent/conversations/" + strconv.FormatInt(id, 10) }

// A-01: PATCH supports rename and pin/unpin, owner-scoped with 404 for
// foreign/missing ids and 400 for invalid payloads.
func TestUpdateConversationRenameAndPin(t *testing.T) {
	env, routerFor := newAgentConversationTestEnv(t)
	router := routerFor(9)
	conv := seedConversation(t, env.db, 9, nil, nil, time.Now())

	rec := env.do(t, router, http.MethodPatch, convPath(conv.ID), `{"title":"新标题"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated model.AgentConversation
	require.NoError(t, env.db.First(&updated, conv.ID).Error)
	require.NotNil(t, updated.Title)
	require.Equal(t, "新标题", *updated.Title)

	rec = env.do(t, router, http.MethodPatch, convPath(conv.ID), `{"pinned":true}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var pinned struct {
		Conversation model.AgentConversation `json:"conversation"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pinned))
	require.NotNil(t, pinned.Conversation.PinnedAt, "pin must set pinned_at")

	rec = env.do(t, router, http.MethodPatch, convPath(conv.ID), `{"pinned":false}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var unpinned struct {
		Conversation model.AgentConversation `json:"conversation"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &unpinned))
	require.Nil(t, unpinned.Conversation.PinnedAt, "unpin must clear pinned_at")
}

func TestUpdateConversationForeignReturns404(t *testing.T) {
	env, routerFor := newAgentConversationTestEnv(t)
	conv := seedConversation(t, env.db, 9, nil, nil, time.Now())

	rec := env.do(t, routerFor(8), http.MethodPatch, convPath(conv.ID), `{"title":"劫持"}`)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	rec = env.do(t, routerFor(9), http.MethodPatch, convPath(999999), `{"title":"幽灵"}`)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestUpdateConversationValidation(t *testing.T) {
	env, routerFor := newAgentConversationTestEnv(t)
	router := routerFor(9)
	conv := seedConversation(t, env.db, 9, nil, nil, time.Now())

	for _, body := range []string{
		`{}`,
		`{"title":""}`,
		`{"title":"   "}`,
		`{"title":"` + strings.Repeat("长", 201) + `"}`,
	} {
		rec := env.do(t, router, http.MethodPatch, convPath(conv.ID), body)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body %s -> %s", body, rec.Body.String())
	}
}

// A-01: list ordering = pinned group by pinned_at desc first, then the rest
// by updated_at desc.
func TestListConversationsPinnedFirstThenUpdatedAt(t *testing.T) {
	env, routerFor := newAgentConversationTestEnv(t)
	now := time.Now()
	seedConversation(t, env.db, 9, nil, nil, now.Add(-1*time.Hour))
	seedConversation(t, env.db, 9, nil, timePtr(now.Add(-2*time.Hour)), now.Add(-2*time.Hour))
	seedConversation(t, env.db, 9, nil, nil, now)
	pinnedNew := seedConversation(t, env.db, 9, nil, timePtr(now.Add(-1*time.Minute)), now.Add(-3*time.Hour))
	seedConversation(t, env.db, 8, nil, nil, now) // foreign — excluded

	rec := env.do(t, routerFor(9), http.MethodGet, "/agent/conversations", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Conversations []model.AgentConversation `json:"conversations"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Conversations, 4)
	first := payload.Conversations[0]
	require.NotNil(t, first.PinnedAt, "pinned group must come first: %#v", payload.Conversations)
	require.Equal(t, pinnedNew.ID, first.ID, "newest pinned first: %#v", payload.Conversations)
	sawUnpinned := false
	for _, conv := range payload.Conversations[1:] {
		if conv.PinnedAt == nil {
			sawUnpinned = true
			continue
		}
		require.False(t, sawUnpinned, "pinned entries must precede unpinned: %#v", payload.Conversations)
	}
}

// A-01 hard cutover (user decision 2026-09-02): the request body is
// {conversation_id?, message}; the legacy {messages:[...]} body is rejected
// with 400 so the old client cannot silently keep its old semantics.
func TestChatStreamNewBodyAcceptedLegacyRejected(t *testing.T) {
	env, routerFor := newAgentConversationTestEnv(t)
	router := routerFor(9)

	// legacy body → 400 before quota/provider
	rec := env.do(t, router, http.MethodPost, "/agent/chat/stream", `{"messages":[{"role":"user","content":"old style"}]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Zero(t, env.provider.calls, "legacy body must not reach the provider")

	// new body: first turn without conversation_id
	rec = env.do(t, router, http.MethodPost, "/agent/chat/stream", `{"message":"新契约第一轮"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"type":"start"`, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"conversation_id":`, rec.Body.String())

	var convs []model.AgentConversation
	require.NoError(t, env.db.Find(&convs).Error)
	require.Len(t, convs, 1)
	convID := convs[0].ID

	// new body: continuation turn
	rec = env.do(t, router, http.MethodPost, "/agent/chat/stream", `{"conversation_id":`+strconv.FormatInt(convID, 10)+`,"message":"第二轮"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var messages []model.AgentMessage
	require.NoError(t, env.db.Where("conversation_id = ?", convID).Order("id ASC").Find(&messages).Error)
	require.Len(t, messages, 4, "two turns = user/assistant/user/assistant; got %#v", messages)

	// missing conversation → 404, provider untouched
	env.provider.calls = 0
	rec = env.do(t, router, http.MethodPost, "/agent/chat/stream", `{"conversation_id":999999,"message":"ghost"}`)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Zero(t, env.provider.calls, "missing conversation must be rejected before quota and provider")
}

// A-01 review fix: PATCH must not bump updated_at (GORM auto-touch is
// deliberately skipped via UpdateColumns) so renames/unpins never reorder
// the conversation list.
func TestUpdateConversationDoesNotBumpUpdatedAt(t *testing.T) {
	env, routerFor := newAgentConversationTestEnv(t)
	router := routerFor(9)
	before := time.Now().Add(-time.Hour)
	conv := seedConversation(t, env.db, 9, nil, nil, before)

	rec := env.do(t, router, http.MethodPatch, convPath(conv.ID), `{"title":"改名不应扰动排序"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var after model.AgentConversation
	require.NoError(t, env.db.First(&after, conv.ID).Error)
	require.WithinDuration(t, before, after.UpdatedAt, time.Second, "rename must not bump updated_at")

	rec = env.do(t, router, http.MethodPatch, convPath(conv.ID), `{"pinned":true}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, env.db.First(&after, conv.ID).Error)
	require.WithinDuration(t, before, after.UpdatedAt, time.Second, "pin must not bump updated_at")
}
