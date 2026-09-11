package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/llm"
)

// fakeHandlerGreen stands in for the Green text scan seam at the handler
// boundary; AgentService.SetGreenScanner accepts it because the unexported
// interface only exposes exported types.
type fakeHandlerGreen struct {
	result  string
	scanErr error
	scanned []string
}

func (f *fakeHandlerGreen) TextModeration(_ context.Context, text string) (*aliyun.GreenScanResult, error) {
	f.scanned = append(f.scanned, text)
	if f.scanErr != nil {
		return nil, f.scanErr
	}
	return &aliyun.GreenScanResult{Result: f.result}, nil
}

// A-05: a blocked chat input is rejected with the unified CONTENT_BLOCKED
// code before the stream opens, before quota reservation and before any
// Provider call.
func TestChatStreamHandlerRejectsBlockedInput(t *testing.T) {
	provider := &recordingAgentHTTPProvider{deltas: []llm.ChatDelta{{Content: "answer"}, {Done: true}}}
	handler, _, _ := newAgentStreamTestHandler(t, provider, nil)
	handler.agentSvc.SetGreenScanner(&fakeHandlerGreen{result: "block"})

	rec := postAgentChat(t, handler, 1, `{"message": "违规输入"}`)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "body = %s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "CONTENT_BLOCKED")
	require.Equal(t, 0, provider.calls, "blocked input must never reach the provider")
}

// A-05 / A4: in release mode a moderation outage fails closed — the turn is
// rejected with MODERATION_UNAVAILABLE instead of bypassing the gate.
func TestChatStreamHandlerModerationUnavailableFailsClosedInRelease(t *testing.T) {
	provider := &recordingAgentHTTPProvider{deltas: []llm.ChatDelta{{Content: "answer"}, {Done: true}}}
	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "release"},
		Agent: config.AgentConfig{
			WebAgentEnabled: true, RateLimitPerMinute: 5, RateLimitPerDay: 50,
			MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, ConversationListLimit: 50,
			ConversationPageSize: 20, ChatContextTokenBudget: 100000,
			MaxToolCallsPerTurn: 8, CitationMaxCount: 5, MaxOutputTokens: 1200,
		},
		RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60},
	}
	handler, _, _ := newAgentStreamTestHandler(t, provider, cfg)
	handler.agentSvc.SetGreenScanner(&fakeHandlerGreen{scanErr: errors.New("green down")})

	rec := postAgentChat(t, handler, 1, `{"message": "任何输入"}`)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, "body = %s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "MODERATION_UNAVAILABLE")
	require.Equal(t, 0, provider.calls)
}

// A-05 / A4: outside release mode the explicit not-configured signal fails
// open — the chat turn still streams without local Green credentials.
func TestChatStreamHandlerModerationNotConfiguredFailsOpenInDev(t *testing.T) {
	provider := &recordingAgentHTTPProvider{deltas: []llm.ChatDelta{{Content: "answer"}, {Done: true}}}
	handler, _, _ := newAgentStreamTestHandler(t, provider, nil)
	handler.agentSvc.SetGreenScanner(&fakeHandlerGreen{scanErr: aliyun.ErrGreenNotConfigured})

	rec := postAgentChat(t, handler, 1, `{"message": "正常输入"}`)

	require.Equal(t, http.StatusOK, rec.Code, "body = %s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "event:delta", "fail-open turn must stream normally")
	require.Equal(t, 1, provider.calls)
}

// A-05: the conversation history projection redacts answers flagged by the
// post-turn output audit — content omitted, moderation marker exposed — while
// clean answers and think rows keep their information.
func TestConversationMessagesRedactFlaggedAnswers(t *testing.T) {
	handler, _, db := newAgentStreamTestHandler(t, nil, nil)

	now := time.Now().UTC()
	require.NoError(t, db.Create(&model.AgentConversation{ID: 5, UserID: 1, ContextType: "general", CreatedAt: now, UpdatedAt: now}).Error)
	question, blocked, clean, think := "问题", "被屏蔽的回答", "正常回答", "思考内容"
	rows := []model.AgentMessage{
		{ID: 51, ConversationID: 5, Role: "user", Content: &question, CreatedAt: now.Add(1 * time.Second)},
		{ID: 52, ConversationID: 5, Role: "assistant", Content: &blocked, ToolCalls: model.JSONMap{"moderation": "blocked"}, CreatedAt: now.Add(2 * time.Second)},
		{ID: 53, ConversationID: 5, Role: "assistant", Content: &clean, CreatedAt: now.Add(3 * time.Second)},
		{ID: 54, ConversationID: 5, Role: "assistant", Content: &think, ToolCalls: model.JSONMap{"phase": "think"}, CreatedAt: now.Add(4 * time.Second)},
	}
	for i := range rows {
		require.NoError(t, db.Create(&rows[i]).Error)
	}

	router := gin.New()
	router.GET("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.GetConversationMessages(c)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agent/conversations/5", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body = %s", rec.Body.String())
	var payload struct {
		Messages []map[string]any `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Messages, 4)

	byID := make(map[float64]map[string]any, len(payload.Messages))
	for _, m := range payload.Messages {
		byID[m["id"].(float64)] = m
	}
	require.NotContains(t, byID[52], "content", "flagged answer content must be redacted")
	require.Equal(t, "blocked", byID[52]["moderation"])
	require.Equal(t, "正常回答", byID[53]["content"])
	require.NotContains(t, byID[53], "moderation")
	require.Equal(t, "think", byID[54]["phase"], "think rows keep their phase marker")
	require.NotContains(t, rec.Body.String(), "被屏蔽的回答", "raw flagged text must not appear anywhere in the response")
}
