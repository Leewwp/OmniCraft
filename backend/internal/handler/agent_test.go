package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/service"
)

type fakeAgentHTTPProvider struct {
	chatContent string
	err         error
	streamErr   error
	lastRequest llm.ChatRequest
	deltas      []llm.ChatDelta
}

func (p *fakeAgentHTTPProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{Content: p.chatContent}, nil
}

func (p *fakeAgentHTTPProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	if p.streamErr != nil {
		return p.streamErr
	}
	return handler(llm.ChatDelta{Done: true})
}

func (p *fakeAgentHTTPProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

type recordingAgentHTTPProvider struct {
	fakeAgentHTTPProvider
	calls       int
	lastRequest llm.ChatRequest
	deltas      []llm.ChatDelta
}

func (p *recordingAgentHTTPProvider) ChatStream(_ context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.calls++
	p.lastRequest = req
	deltas := p.deltas
	if len(deltas) == 0 {
		deltas = []llm.ChatDelta{{Done: true}}
	}
	for _, delta := range deltas {
		if err := handler(delta); err != nil {
			return err
		}
	}
	return p.streamErr
}

func TestAgentConversationEndpointsRespectFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	handler := NewAgentHandler(db, &config.Config{Agent: config.AgentConfig{WebAgentEnabled: false}}, nil)
	router := gin.New()
	router.GET("/agent/conversations", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListConversations(c)
	})
	router.GET("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.GetConversationMessages(c)
	})

	for _, path := range []string{"/agent/conversations", "/agent/conversations/1"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d, want 503; body = %s", path, rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); !strings.Contains(body, "FEATURE_DISABLED") {
			t.Fatalf("%s body missing FEATURE_DISABLED: %s", path, body)
		}
	}
}

func TestListConversationsReportsDatabaseFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	handler := NewAgentHandler(db, &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, ConversationListLimit: 50}}, nil)
	if err := db.Exec("DROP TABLE agent_conversations").Error; err != nil {
		// The table does not exist in the intentionally minimal fixture; the
		// subsequent query still exercises the database-error path.
		_ = err
	}
	router := gin.New()
	router.GET("/agent/conversations", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListConversations(c)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agent/conversations", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s; database failures must not look like an empty list", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AGENT_ERROR") {
		t.Fatalf("body = %s, want stable AGENT_ERROR", rec.Body.String())
	}
}

func TestGetConversationReportsDatabaseFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	handler := NewAgentHandler(db, &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, ConversationPageSize: 20}}, nil)
	router := gin.New()
	router.GET("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.GetConversationMessages(c)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agent/conversations/1", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s; database failures must not look like not found", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AGENT_ERROR") {
		t.Fatalf("body = %s, want stable AGENT_ERROR", rec.Body.String())
	}
}

func TestGetConversationMessagesPaginatesConfiguredPage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	conv := model.AgentConversation{ID: 1, UserID: 1, ContextType: "general"}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	for i := 0; i < 3; i++ {
		content := "message"
		if err := db.Create(&model.AgentMessage{ConversationID: conv.ID, Role: "user", Content: &content}).Error; err != nil {
			t.Fatalf("seed message %d: %v", i, err)
		}
	}
	handler := NewAgentHandler(db, &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, ConversationPageSize: 2}}, nil)
	router := gin.New()
	router.GET("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.GetConversationMessages(c)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agent/conversations/1?page=2&page_size=1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Messages []model.AgentMessage `json:"messages"`
		Page     int                  `json:"page"`
		HasMore  bool                 `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Messages) != 1 || payload.Page != 2 || !payload.HasMore {
		t.Fatalf("page payload = %#v, want one message, page=2, has_more=true", payload)
	}
}

func TestUploadAssistHandlerReturnsStructuredResponse(t *testing.T) {
	handler := newAgentHandlerForTest(t, &fakeAgentHTTPProvider{
		chatContent: `{"suggested_tags":["tag-a","tag-b"],"suggested_category":"gaming","suggested_title":"Better title","suggested_description":"Sharper summary"}`,
	}, true)
	router := gin.New()
	router.POST("/agent/upload-assist", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(42))
		handler.UploadAssist(c)
	})

	body := bytes.NewBufferString(`{"title":"Original","description":"Desc","filename":"cover.png","content_type":"gaming"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/upload-assist", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		SuggestedTags        []string `json:"suggested_tags"`
		SuggestedCategory    string   `json:"suggested_category"`
		SuggestedTitle       string   `json:"suggested_title"`
		SuggestedDescription string   `json:"suggested_description"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.SuggestedCategory != "gaming" || payload.SuggestedTitle != "Better title" || payload.SuggestedDescription != "Sharper summary" {
		t.Fatalf("payload = %#v, want structured upload assist response", payload)
	}
	if len(payload.SuggestedTags) != 2 {
		t.Fatalf("SuggestedTags = %#v, want parsed tags", payload.SuggestedTags)
	}
}

func TestComplianceCheckHandlerReturnsStructuredResponse(t *testing.T) {
	handler := newAgentHandlerForTest(t, &fakeAgentHTTPProvider{
		chatContent: `{"risk_level":"warning","reason":"possible copyright issue","suggestions":["add attribution"]}`,
	}, true)
	router := gin.New()
	router.POST("/agent/compliance-check", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(42))
		handler.ComplianceCheck(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/agent/compliance-check", bytes.NewBufferString(`{"title":"Original","description":"Desc","content_type":"article"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		RiskLevel   string   `json:"risk_level"`
		Reason      string   `json:"reason"`
		Suggestions []string `json:"suggestions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.RiskLevel != "warning" || payload.Reason != "possible copyright issue" {
		t.Fatalf("payload = %#v, want structured compliance response", payload)
	}
	if len(payload.Suggestions) != 1 || payload.Suggestions[0] != "add attribution" {
		t.Fatalf("Suggestions = %#v, want parsed suggestion", payload.Suggestions)
	}
}

func TestAgentHandlersRejectInvalidJSON(t *testing.T) {
	handler := newAgentHandlerForTest(t, &fakeAgentHTTPProvider{chatContent: `{"risk_level":"safe"}`}, true)
	router := gin.New()
	router.POST("/agent/upload-assist", handler.UploadAssist)
	router.POST("/agent/compliance-check", handler.ComplianceCheck)

	for _, path := range []string{"/agent/upload-assist", "/agent/compliance-check"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"title":`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400; body = %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
			t.Fatalf("%s body = %s, want VALIDATION_ERROR", path, rec.Body.String())
		}
	}
}

func TestChatStreamHandlerRejectsOversizedMessages(t *testing.T) {
	provider := &recordingAgentHTTPProvider{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	cfg := &config.Config{
		Agent: config.AgentConfig{
			WebAgentEnabled:        true,
			MaxUserMessageChars:    8,
			ChatMaxContextMsgs:     2,
			ChatContextTokenBudget: 100000,
			MaxToolCallsPerTurn:    1,
			MaxOutputTokens:        1200,
			CitationMaxCount:       5,
		},
	}
	handler := &AgentHandler{
		agentSvc: service.NewAgentService(provider, nil, nil, nil, db, cfg),
		cfg:      cfg,
	}
	router := gin.New()
	router.POST("/agent/chat/stream", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(9))
		handler.ChatStream(c)
	})

	body := bytes.NewBufferString(`{"message": "0123456789ABCDEF"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/chat/stream", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("body = %s, want VALIDATION_ERROR", rec.Body.String())
	}
	if provider.calls != 0 {
		t.Fatal("oversized message must not reach the provider")
	}
}

// TestChatStreamHandlerAssemblesServerSideContextWindow locks the A-01
// server-side context assembly at the handler seam: the client sends only the
// new message; the provider request is system + the stored conversation
// history bounded by ChatMaxContextMsgs, and no client-authored role can leak
// (there is no message array in the request body at all).
func TestChatStreamHandlerAssemblesServerSideContextWindow(t *testing.T) {
	provider := &recordingAgentHTTPProvider{
		deltas: []llm.ChatDelta{{Content: "answer"}, {Done: true}},
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	// :memory: databases are per-connection; pin one connection so follow-up
	// turns and the async title goroutine see the same schema.
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		t.Fatalf("migrate agent chat tables: %v", err)
	}
	// Seed an owner conversation with a long stored history.
	conv := model.AgentConversation{UserID: 9, ContextType: "general"}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	seedRoles := []string{"user", "assistant"}
	seedContents := []string{"old question", "old answer"}
	for i := range seedRoles {
		if err := db.Create(&model.AgentMessage{ConversationID: conv.ID, Role: seedRoles[i], Content: &seedContents[i]}).Error; err != nil {
			t.Fatalf("seed message: %v", err)
		}
	}
	cfg := &config.Config{
		Agent: config.AgentConfig{
			WebAgentEnabled:        true,
			RateLimitPerMinute:     5,
			RateLimitPerDay:        50,
			MaxUserMessageChars:    64,
			ChatMaxContextMsgs:     2,
			ChatContextTokenBudget: 100000,
			MaxToolCallsPerTurn:    8,
			MaxOutputTokens:        1200,
			CitationMaxCount:       5,
		},
		RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60},
	}
	quota, quotaCleanup := newTestQuotaReserver(t, cfg)
	defer quotaCleanup()
	handler := &AgentHandler{
		agentSvc: service.NewAgentService(provider, nil, nil, nil, db, cfg),
		cfg:      cfg,
		quota:    quota,
	}
	router := gin.New()
	router.POST("/agent/chat/stream", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(9))
		handler.ChatStream(c)
	})

	body := bytes.NewBufferString(`{"conversation_id": ` + strconv.FormatInt(conv.ID, 10) + `, "message": "tail"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/chat/stream", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	msgs := provider.lastRequest.Messages
	if len(msgs) != 3 {
		t.Fatalf("provider messages len = %d, want 3 (system + newest 2 history messages under the cap); got %#v", len(msgs), msgs)
	}
	if msgs[0].Role != "system" {
		t.Fatalf("first message = %#v, want server-owned system prompt", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "old answer" {
		t.Fatalf("first history message = %#v, want the stored assistant answer", msgs[1])
	}
	if msgs[2].Role != "user" || msgs[2].Content != "tail" {
		t.Fatalf("last message = %#v, want the new turn message", msgs[2])
	}
}

func newAgentHandlerForTest(t *testing.T, provider llm.LLMProvider, enabled bool) *AgentHandler {
	t.Helper()

	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:       enabled,
		RateLimitPerMinute:    5,
		RateLimitPerDay:       50,
		MaxUserMessageChars:   4000,
		ChatMaxContextMsgs:    10,
		ConversationListLimit: 50,
		ConversationPageSize:  20,
		MaxToolCallsPerTurn:   8,
		MaxOutputTokens:       1200,
		CitationMaxCount:      5,
	}, RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60}}
	quota, quotaCleanup := newTestQuotaReserver(t, cfg)
	t.Cleanup(quotaCleanup)
	return &AgentHandler{
		agentSvc: service.NewAgentService(provider, nil, nil, nil, nil, cfg),
		cfg:      cfg,
		quota:    quota,
	}
}

func newTestQuotaReserver(t *testing.T, cfg *config.Config) (*middleware.AgentQuotaReserver, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}
	return middleware.NewAgentQuotaReserver(rdb, cfg), cleanup
}
