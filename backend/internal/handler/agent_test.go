package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
			WebAgentEnabled:     true,
			MaxUserMessageChars: 8,
			ChatMaxContextMsgs:  2,
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

	body := bytes.NewBufferString(`{
		"messages": [
			{"role": "user", "content": "0123456789ABCDEF"}
		]
	}`)
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

func TestChatStreamHandlerFiltersRolesAndLimitsContextWindow(t *testing.T) {
	provider := &recordingAgentHTTPProvider{
		deltas: []llm.ChatDelta{{Content: "answer"}, {Done: true}},
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		t.Fatalf("migrate agent chat tables: %v", err)
	}
	cfg := &config.Config{
		Agent: config.AgentConfig{
			WebAgentEnabled:     true,
			MaxUserMessageChars: 64,
			ChatMaxContextMsgs:  2,
		},
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

	body := bytes.NewBufferString(`{
		"messages": [
			{"role": "system", "content": "drop-me"},
			{"role": "assistant", "content": "old assistant"},
			{"role": "user", "content": "1234567890"},
			{"role": "user", "content": "tail"}
		]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/chat/stream", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if len(provider.lastRequest.Messages) != 3 {
		t.Fatalf("provider messages len = %d, want 3 (server-owned system prompt + 2 filtered user messages)", len(provider.lastRequest.Messages))
	}
	if provider.lastRequest.Messages[0].Role != "system" {
		t.Fatalf("first message = %#v, want server-owned system prompt", provider.lastRequest.Messages[0])
	}
	if provider.lastRequest.Messages[1].Role != "user" || provider.lastRequest.Messages[1].Content != "1234567890" {
		t.Fatalf("first forwarded message = %#v, want last context user message", provider.lastRequest.Messages[1])
	}
	if provider.lastRequest.Messages[2].Role != "user" || provider.lastRequest.Messages[2].Content != "tail" {
		t.Fatalf("second forwarded message = %#v, want last context message", provider.lastRequest.Messages[2])
	}
	clientRoles := provider.lastRequest.Messages[1:]
	for _, msg := range clientRoles {
		if msg.Role == "system" {
			t.Fatalf("client-authored role leaked into provider request: %#v", clientRoles)
		}
	}
}

func newAgentHandlerForTest(t *testing.T, provider llm.LLMProvider, enabled bool) *AgentHandler {
	t.Helper()

	cfg := &config.Config{Agent: config.AgentConfig{WebAgentEnabled: enabled}}
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
