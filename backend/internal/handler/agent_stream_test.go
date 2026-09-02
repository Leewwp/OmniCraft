package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

type agentStreamSSEEvent struct {
	Name string
	Data map[string]any
}

func parseAgentSSE(t *testing.T, body string) []agentStreamSSEEvent {
	t.Helper()
	var out []agentStreamSSEEvent
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var name string
		var dataLines []string
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event:") {
				name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		var data map[string]any
		if len(dataLines) > 0 {
			if err := json.Unmarshal([]byte(strings.Join(dataLines, "")), &data); err != nil {
				t.Fatalf("parse SSE data %q: %v", block, err)
			}
		}
		out = append(out, agentStreamSSEEvent{Name: name, Data: data})
	}
	return out
}

func streamEventNames(events []agentStreamSSEEvent) []string {
	names := make([]string, 0, len(events))
	for _, e := range events {
		names = append(names, e.Name)
	}
	return names
}

func newAgentStreamTestHandler(t *testing.T, provider llm.LLMProvider, cfg *config.Config) (*AgentHandler, *miniredis.Miniredis, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.AgentConversation{}, &model.AgentMessage{}, &model.User{}, &model.ContentItem{},
		&model.ContentVersion{}, &model.RagChunk{}, &model.IndexProjectionStatus{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{ID: 1, Username: "author", Email: "a@example.com"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	now := time.Now()
	if err := db.Create(&model.ContentItem{ID: 88, Title: "Published Test Content", AuthorID: 1, Zone: "fanwork", ContentType: "mod", Status: "published", IsPublic: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}

	if cfg == nil {
		cfg = &config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, RateLimitPerMinute: 5, RateLimitPerDay: 50, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, ConversationListLimit: 50, ConversationPageSize: 20, ChatContextTokenBudget: 100000, MaxToolCallsPerTurn: 8, CitationMaxCount: 5, MaxOutputTokens: 1200}, RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60}}
	}
	if cfg.RateLimit.AgentWindowSec == 0 {
		cfg.RateLimit.AgentWindowSec = 86400
	}
	if cfg.RateLimit.AgentMinuteWindowSec == 0 {
		cfg.RateLimit.AgentMinuteWindowSec = 60
	}

	handler := &AgentHandler{
		agentSvc: service.NewAgentService(provider, nil, nil, nil, db, cfg),
		cfg:      cfg,
		db:       db,
		quota:    middleware.NewAgentQuotaReserver(rdb, cfg),
	}
	return handler, mr, db
}

func postAgentChat(t *testing.T, handler *AgentHandler, userID int64, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST("/agent/chat/stream", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, userID)
		handler.ChatStream(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/agent/chat/stream", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestAgentChatStreamEmitsTypedSSEEvents(t *testing.T) {
	handler, mr, _ := newAgentStreamTestHandler(t, &recordingAgentHTTPProvider{
		deltas: []llm.ChatDelta{
			{Content: "hello "},
			{Content: "world"},
			{Done: true},
		},
	}, nil)

	rec := postAgentChat(t, handler, 7, `{"message": "hi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	events := parseAgentSSE(t, rec.Body.String())
	names := streamEventNames(events)
	if names[0] != "start" {
		t.Fatalf("first event = %s, want start; events = %v", names[0], names)
	}
	if names[len(names)-1] != "done" {
		t.Fatalf("last event = %s, want done; events = %v", names[len(names)-1], names)
	}
	for _, want := range []string{"usage", "done"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("events %v missing %q", names, want)
		}
	}

	var doneData map[string]any
	for _, e := range events {
		if e.Name == "done" {
			doneData = e.Data
		}
	}
	if doneData == nil {
		t.Fatal("done event missing")
	}
	if traceID, _ := doneData["trace_id"].(string); len(traceID) != 32 {
		t.Fatalf("done trace_id = %q, want 32-hex", traceID)
	}
	if degraded, _ := doneData["degraded"].(bool); degraded {
		t.Fatal("done degraded = true, want false")
	}
	if answer, _ := doneData["answer"].(string); answer != "" {
		t.Fatalf("no-evidence done answer = %q, want empty", answer)
	}
	for _, name := range names {
		if name == "delta" {
			t.Fatalf("no-evidence stream must not expose model delta: %v", names)
		}
	}

	// Reservation was consumed exactly once for this request.
	if _, err := mr.Get(handler.quota.DayKey(7)); err != nil {
		t.Fatalf("day quota key missing after successful stream: %v", err)
	}
}

type handlerContractRetriever struct {
	result service.AgentRetrievalResult
}

func (r *handlerContractRetriever) Retrieve(_ context.Context, _ string, _ int64) (service.AgentRetrievalResult, error) {
	return r.result, nil
}

func TestAgentChatStreamFlagOnEmitsExpandedCitationAndDegradedStatus(t *testing.T) {
	const chunkKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	cases := []struct {
		name         string
		degraded     string
		source       string
		wantDegraded bool
	}{
		{name: "grounded", source: "hybrid_rrf"},
		{name: "keyword fallback", degraded: "keyword_pg", source: "bm25", wantDegraded: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &oneShotStreamProvider{rounds: [][]llm.ChatDelta{
				{{ToolCalls: []llm.ToolCall{{
					ID: "call_1", Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: service.ToolSearchContent, Arguments: `{"query":"find this"}`},
				}}}},
				{{Content: "grounded answer", Done: true}},
			}}
			cfg := &config.Config{
				Features: config.FeaturesConfig{RAGHybridEnabled: true},
				Agent: config.AgentConfig{
					WebAgentEnabled: true, RateLimitPerMinute: 5, RateLimitPerDay: 50,
					MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, ChatContextTokenBudget: 100000, MaxToolCallsPerTurn: 8,
					CitationMaxCount: 5, MaxOutputTokens: 1200,
				},
			}
			handler, _, db := newAgentStreamTestHandler(t, provider, cfg)
			now := time.Now()
			if err := db.Create(&model.ContentVersion{ID: 880, ContentItemID: 88, AuthorID: 1, VersionNumber: 1, StorageType: "full", StorageKey: "handler-rag", Status: "active", IsLatest: true}).Error; err != nil {
				t.Fatalf("seed content version: %v", err)
			}
			if err := db.Create(&model.RagChunk{ContentID: 88, ContentVersion: 1, ChunkIndex: 0, ChunkKey: chunkKey, ChunkingVersion: 1, Text: "Published Test Content", SourceStart: 0, SourceEnd: 22, Zone: "fanwork", ContentType: "mod", IndexVersion: 1, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				t.Fatalf("seed rag chunk: %v", err)
			}
			if err := db.Create(&model.IndexProjectionStatus{ContentID: 88, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "test", State: "ready", IsCurrent: true, ErrorSummary: "", CreatedAt: now}).Error; err != nil {
				t.Fatalf("seed projection status: %v", err)
			}
			handler.agentSvc.SetContentRetriever(&handlerContractRetriever{result: service.AgentRetrievalResult{
				Candidates: []service.AgentRetrievalCandidate{{
					ChunkKey: chunkKey, ContentID: 88, ContentVersion: 1, ChunkIndex: 0,
					Title: "Published Test Content", Text: "Published Test Content", Zone: "fanwork",
					ContentType: "mod", Source: tc.source,
				}},
				Degraded: tc.degraded,
			}})

			rec := postAgentChat(t, handler, 7, `{"message": "find this"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			events := parseAgentSSE(t, rec.Body.String())
			var citationData, doneData map[string]any
			for _, event := range events {
				switch event.Name {
				case "citation":
					citationData, _ = event.Data["citation"].(map[string]any)
				case "done":
					doneData = event.Data
				}
			}
			if citationData == nil {
				t.Fatalf("expanded citation event missing: %s", rec.Body.String())
			}
			if citationData["content_version"] != float64(1) || citationData["chunk_key"] != chunkKey || citationData["route"] != "/content/88" || citationData["source"] != tc.source {
				t.Fatalf("citation = %#v, want version/chunk/route/source contract", citationData)
			}
			if doneData == nil {
				t.Fatal("done event missing")
			}
			if got, _ := doneData["degraded"].(bool); got != tc.wantDegraded {
				t.Fatalf("done degraded = %v, want %v", got, tc.wantDegraded)
			}
		})
	}
}

func TestAgentChatStreamFeatureDisabledReturns503(t *testing.T) {
	handler, _, _ := newAgentStreamTestHandler(t, &fakeAgentHTTPProvider{}, &config.Config{Agent: config.AgentConfig{WebAgentEnabled: false}})
	rec := postAgentChat(t, handler, 7, `{"message": "hi"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "FEATURE_DISABLED") {
		t.Fatalf("body = %s, want FEATURE_DISABLED", rec.Body.String())
	}
}

func TestAgentChatStreamOversizedMessageRejectedWithoutQuota(t *testing.T) {
	provider := &recordingAgentHTTPProvider{}
	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:        true,
		RateLimitPerMinute:     5,
		RateLimitPerDay:        50,
		MaxUserMessageChars:    8,
		ChatMaxContextMsgs:     10,
		ChatContextTokenBudget: 100000,
		MaxToolCallsPerTurn:    8,
		MaxOutputTokens:        1200,
		CitationMaxCount:       5,
	}, RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60}}
	handler, mr, _ := newAgentStreamTestHandler(t, provider, cfg)

	rec := postAgentChat(t, handler, 7, `{"message": "0123456789ABCDEF"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 VALIDATION_ERROR; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("body = %s, want VALIDATION_ERROR", rec.Body.String())
	}
	if _, err := mr.Get(handler.quota.DayKey(7)); err == nil {
		t.Fatal("oversized message must not consume quota")
	}
	if len(provider.lastRequest.Messages) != 0 && provider.lastRequest.Messages != nil {
		t.Fatalf("provider request = %#v, oversized message must never reach provider", provider.lastRequest)
	}
}

func TestAgentChatStreamMissingToolLimitsRejectedBeforeQuota(t *testing.T) {
	provider := &recordingAgentHTTPProvider{}
	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:        true,
		RateLimitPerMinute:     5,
		RateLimitPerDay:        50,
		MaxUserMessageChars:    4000,
		ChatMaxContextMsgs:     10,
		ChatContextTokenBudget: 100000,
		ConversationListLimit:  50,
		ConversationPageSize:   20,
	}, RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60}}
	handler, mr, _ := newAgentStreamTestHandler(t, provider, cfg)

	rec := postAgentChat(t, handler, 7, `{"message": "hi"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AGENT_CONFIG_INVALID") {
		t.Fatalf("body = %s, want AGENT_CONFIG_INVALID", rec.Body.String())
	}
	if _, err := mr.Get(handler.quota.DayKey(7)); err == nil {
		t.Fatal("invalid tool limits must be rejected before quota reservation")
	}
	if provider.calls != 0 {
		t.Fatal("invalid tool limits must never reach the provider")
	}
}

func TestAgentChatStreamHiddenContextRejectedBeforeReservation(t *testing.T) {
	provider := &recordingAgentHTTPProvider{}
	handler, mr, _ := newAgentStreamTestHandler(t, provider, nil)

	rec := postAgentChat(t, handler, 7, `{"message": "hi", "context":{"surface":"content","content_id":999}}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NOT_FOUND") {
		t.Fatalf("body = %s, want NOT_FOUND", rec.Body.String())
	}
	if _, err := mr.Get(handler.quota.DayKey(7)); err == nil {
		t.Fatal("hidden chat context rejection must happen before quota reservation")
	}
	if provider.calls != 0 {
		t.Fatal("hidden chat context must never reach the provider")
	}
}

func TestAgentChatStreamQuotaExceededReturns429(t *testing.T) {
	provider := &recordingAgentHTTPProvider{}
	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:        true,
		RateLimitPerMinute:     5,
		RateLimitPerDay:        1,
		MaxUserMessageChars:    4000,
		ChatMaxContextMsgs:     10,
		ChatContextTokenBudget: 100000,
		MaxToolCallsPerTurn:    8,
		MaxOutputTokens:        1200,
		CitationMaxCount:       5,
	}}
	handler, _, _ := newAgentStreamTestHandler(t, provider, cfg)

	rec1 := postAgentChat(t, handler, 7, `{"message": "first"}`)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body = %s", rec1.Code, rec1.Body.String())
	}
	rec2 := postAgentChat(t, handler, 7, `{"message": "second"}`)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429; body = %s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), "AGENT_RATE_LIMIT_EXCEEDED") {
		t.Fatalf("body = %s, want AGENT_RATE_LIMIT_EXCEEDED", rec2.Body.String())
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1 (only the admitted request reaches the provider)", provider.calls)
	}
}

func TestAgentChatStreamRedisUnavailableFailsClosed(t *testing.T) {
	provider := &recordingAgentHTTPProvider{}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	addr := mr.Addr()
	mr.Close()
	deadRDB := redis.NewClient(&redis.Options{Addr: addr})
	defer deadRDB.Close()

	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:        true,
		RateLimitPerMinute:     5,
		RateLimitPerDay:        50,
		MaxUserMessageChars:    4000,
		ChatMaxContextMsgs:     10,
		ChatContextTokenBudget: 100000,
		MaxToolCallsPerTurn:    8,
		MaxOutputTokens:        1200,
		CitationMaxCount:       5,
	}}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	handler := &AgentHandler{
		agentSvc: service.NewAgentService(provider, nil, nil, nil, db, cfg),
		cfg:      cfg,
		db:       db,
		quota:    middleware.NewAgentQuotaReserver(deadRDB, cfg),
	}

	rec := postAgentChat(t, handler, 7, `{"message": "hi"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 fail-closed; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AGENT_QUOTA_UNAVAILABLE") {
		t.Fatalf("body = %s, want AGENT_QUOTA_UNAVAILABLE", rec.Body.String())
	}
	if provider.calls != 0 {
		t.Fatal("fail-closed request must never reach the provider")
	}
}

func TestAgentChatStreamProviderErrorEmitsSafeSSEError(t *testing.T) {
	provider := &recordingAgentHTTPProvider{fakeAgentHTTPProvider: fakeAgentHTTPProvider{streamErr: errors.New("RAW SECRET provider failure")}}
	handler, mr, _ := newAgentStreamTestHandler(t, provider, nil)

	rec := postAgentChat(t, handler, 7, `{"message": "hi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with SSE error event; body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "RAW") || strings.Contains(rec.Body.String(), "SECRET") {
		t.Fatalf("raw provider error leaked: %s", rec.Body.String())
	}
	events := parseAgentSSE(t, rec.Body.String())
	var errorEvent *agentStreamSSEEvent
	for i := range events {
		if events[i].Name == "error" {
			errorEvent = &events[i]
		}
	}
	if errorEvent == nil {
		t.Fatalf("no SSE error event in %v", streamEventNames(events))
	}
	if code, _ := errorEvent.Data["error_code"].(string); code != "AGENT_PROVIDER_ERROR" {
		t.Fatalf("error event code = %q, want AGENT_PROVIDER_ERROR", code)
	}
	if degraded, _ := errorEvent.Data["degraded"].(bool); !degraded {
		t.Fatalf("error event degraded = %v, want true", degraded)
	}
	if reason, _ := errorEvent.Data["degraded_reason"].(string); reason != "provider_error" {
		t.Fatalf("error event degraded_reason = %q, want provider_error", reason)
	}
	if _, err := mr.Get(handler.quota.DayKey(7)); err != nil {
		t.Fatalf("provider-error stream must still consume its reservation (day key missing): %v", err)
	}
}

func TestAgentChatStreamForbiddenToolIDConsumesReservation(t *testing.T) {
	provider := &oneShotStreamProvider{
		rounds: [][]llm.ChatDelta{
			{
				{
					ToolCalls: []llm.ToolCall{{
						ID:   "call_1",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{Name: "get_content_detail", Arguments: `{"content_id": 999}`},
					}},
				},
			},
			{{Content: "final", Done: true}},
		},
	}
	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:        true,
		RateLimitPerMinute:     5,
		RateLimitPerDay:        1,
		MaxUserMessageChars:    4000,
		ChatMaxContextMsgs:     10,
		ChatContextTokenBudget: 100000,
		MaxToolCallsPerTurn:    8,
		MaxOutputTokens:        1200,
		CitationMaxCount:       5,
	}}
	handler, _, _ := newAgentStreamTestHandler(t, provider, cfg)

	rec := postAgentChat(t, handler, 7, `{"message": "find it"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("stream status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	events := parseAgentSSE(t, rec.Body.String())
	names := streamEventNames(events)
	var toolStatus *agentStreamSSEEvent
	for i := range events {
		if events[i].Name == "tool_status" {
			toolStatus = &events[i]
		}
	}
	if toolStatus == nil {
		t.Fatalf("no tool_status event in %v", names)
	}
	toolData, _ := toolStatus.Data["tool"].(map[string]any)
	status, _ := toolData["status"].(string)
	if status != "error" {
		t.Fatalf("forbidden tool id status = %q, want uniform error; events = %v", status, names)
	}
	if names[len(names)-1] != "done" {
		t.Fatalf("last event = %s, want done after forbidden tool id", names[len(names)-1])
	}

	// The forbidden-tool stream consumed its reservation: the next request hits
	// the day limit even though no content was ever visible to the model.
	rec2 := postAgentChat(t, handler, 7, `{"message": "second"}`)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429 (first stream consumed its reservation); body = %s", rec2.Code, rec2.Body.String())
	}
}

// oneShotStreamProvider plays each ChatStream round exactly once, like a real
// Provider does, so handler tests can drive a two-round tool loop.
type oneShotStreamProvider struct {
	rounds [][]llm.ChatDelta
	calls  int
}

func (p *oneShotStreamProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}

func (p *oneShotStreamProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	if p.calls >= len(p.rounds) {
		return nil
	}
	round := p.rounds[p.calls]
	p.calls++
	for _, delta := range round {
		if err := handler(delta); err != nil {
			return err
		}
	}
	return nil
}

func (p *oneShotStreamProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func TestAgentConversationHistoryDoesNotConsumeQuota(t *testing.T) {
	handler, mr, db := newAgentStreamTestHandler(t, &recordingAgentHTTPProvider{}, nil)
	now := time.Now()
	conv := model.AgentConversation{UserID: 7, ContextType: "general", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	msg := "stored"
	if err := db.Create(&model.AgentMessage{ConversationID: conv.ID, Role: "user", Content: &msg, CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}

	router := gin.New()
	router.GET("/agent/conversations", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(7))
		handler.ListConversations(c)
	})
	router.GET("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(7))
		handler.GetConversationMessages(c)
	})

	for _, path := range []string{"/agent/conversations", fmt.Sprintf("/agent/conversations/%d", conv.ID)} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, rec.Code)
		}
	}
	if _, err := mr.Get(handler.quota.DayKey(7)); err == nil {
		t.Fatal("conversation-history reads must not consume generation quota")
	}
}

func TestAgentConversationDeleteOwnedOnlyAndIdempotent(t *testing.T) {
	handler, mr, db := newAgentStreamTestHandler(t, &recordingAgentHTTPProvider{}, nil)
	now := time.Now()
	owned := model.AgentConversation{UserID: 7, ContextType: "general", CreatedAt: now, UpdatedAt: now}
	foreign := model.AgentConversation{UserID: 8, ContextType: "general", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&owned).Error; err != nil {
		t.Fatalf("seed owned: %v", err)
	}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatalf("seed foreign: %v", err)
	}
	ownedMsg := "owned content"
	if err := db.Create(&model.AgentMessage{ConversationID: owned.ID, Role: "user", Content: &ownedMsg, CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed owned message: %v", err)
	}
	foreignMsg := "foreign content"
	if err := db.Create(&model.AgentMessage{ConversationID: foreign.ID, Role: "user", Content: &foreignMsg, CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed foreign message: %v", err)
	}

	router := gin.New()
	router.DELETE("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(7))
		handler.DeleteConversation(c)
	})

	del := func(id int64) int {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/agent/conversations/%d", id), nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := del(owned.ID); code != http.StatusNoContent {
		t.Fatalf("delete owned status = %d, want 204", code)
	}
	var count int64
	db.Model(&model.AgentMessage{}).Where("conversation_id = ?", owned.ID).Count(&count)
	if count != 0 {
		t.Fatalf("owned messages remaining = %d, want cascade delete", count)
	}
	if code := del(owned.ID); code != http.StatusNoContent {
		t.Fatalf("delete owned again status = %d, want idempotent 204", code)
	}
	if code := del(foreign.ID); code != http.StatusNoContent {
		t.Fatalf("delete foreign status = %d, want indistinguishable 204", code)
	}
	var foreignRows int64
	db.Model(&model.AgentConversation{}).Where("id = ?", foreign.ID).Count(&foreignRows)
	if foreignRows != 1 {
		t.Fatalf("foreign conversation deleted: rows = %d, want intact", foreignRows)
	}
	if code := del(0); code != http.StatusBadRequest {
		t.Fatalf("delete invalid id status = %d, want 400", code)
	}
	if _, err := mr.Get(handler.quota.DayKey(7)); err == nil {
		t.Fatal("conversation deletion must not consume generation quota")
	}
}

func TestAgentConversationDeleteDBFailureReturnsStableError(t *testing.T) {
	handler, _, db := newAgentStreamTestHandler(t, &recordingAgentHTTPProvider{}, nil)
	now := time.Now()
	conv := model.AgentConversation{UserID: 7, ContextType: "general", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&conv).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	msg := "must survive"
	if err := db.Create(&model.AgentMessage{ConversationID: conv.ID, Role: "user", Content: &msg, CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}
	// Force the cascade delete to fail by dropping the message table.
	if err := db.Migrator().DropTable(&model.AgentMessage{}); err != nil {
		t.Fatalf("drop message table: %v", err)
	}

	router := gin.New()
	router.DELETE("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(7))
		handler.DeleteConversation(c)
	})
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/agent/conversations/%d", conv.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 stable error; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AGENT_ERROR") {
		t.Fatalf("body = %s, want AGENT_ERROR stable code", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "no such table") {
		t.Fatalf("raw DB error leaked: %s", rec.Body.String())
	}
	var convs int64
	db.Model(&model.AgentConversation{}).Where("id = ?", conv.ID).Count(&convs)
	if convs != 1 {
		t.Fatal("failed delete must not partially remove the conversation")
	}
}
