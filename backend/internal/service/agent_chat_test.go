package service

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

type recordingAgentChatProvider struct {
	lastRequest llm.ChatRequest
	deltas      []llm.ChatDelta
}

func (p *recordingAgentChatProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}

func (p *recordingAgentChatProvider) ChatStream(_ context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.lastRequest = req
	for _, delta := range p.deltas {
		if err := handler(delta); err != nil {
			return err
		}
	}
	return nil
}

func (p *recordingAgentChatProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func TestChatStreamStoresContentContextAndInjectsPageContext(t *testing.T) {
	db := setupAgentChatDB(t)
	provider := &recordingAgentChatProvider{
		deltas: []llm.ChatDelta{
			{Content: "assistant reply"},
			{Done: true},
		},
	}
	svc := NewAgentService(
		provider,
		nil,
		nil,
		nil,
		db,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	contentID := int64(88)
	pageCtx := &model.AgentPageContext{
		Route:        "/contents/88",
		ContentID:    &contentID,
		ContentTitle: "Review Target",
		ContentType:  "article",
	}

	var conversationID int64
	err := svc.ChatStream(
		context.Background(),
		42,
		[]llm.ChatMessage{{Role: "user", Content: "Summarize this page"}},
		pageCtx,
		func(_ string, done bool, convID int64) error {
			if done {
				conversationID = convID
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if conversationID == 0 {
		t.Fatal("expected ChatStream to create a persisted conversation")
	}

	if len(provider.lastRequest.Messages) != 2 {
		t.Fatalf("provider messages len = %d, want 2", len(provider.lastRequest.Messages))
	}
	systemMsg := provider.lastRequest.Messages[0]
	if systemMsg.Role != "system" {
		t.Fatalf("first provider role = %q, want system", systemMsg.Role)
	}
	for _, want := range []string{
		"[Page Context]",
		"Current page: /contents/88",
		"Content title: Review Target",
		"Content type: article",
		"Content ID: 88",
	} {
		if !strings.Contains(systemMsg.Content, want) {
			t.Fatalf("system prompt = %q, want substring %q", systemMsg.Content, want)
		}
	}

	var conversation model.AgentConversation
	if err := db.First(&conversation, conversationID).Error; err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if conversation.ContextType != "content" {
		t.Fatalf("ContextType = %q, want content", conversation.ContextType)
	}
	if conversation.ContextID == nil || *conversation.ContextID != contentID {
		t.Fatalf("ContextID = %#v, want %d", conversation.ContextID, contentID)
	}

	var stored []model.AgentMessage
	if err := db.Where("conversation_id = ?", conversationID).Order("id ASC").Find(&stored).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored messages len = %d, want 2", len(stored))
	}
	if stored[0].Role != "user" || stored[0].Content == nil || *stored[0].Content != "Summarize this page" {
		t.Fatalf("stored user message = %#v, want original user content", stored[0])
	}
	if stored[1].Role != "assistant" || stored[1].Content == nil || *stored[1].Content != "assistant reply" {
		t.Fatalf("stored assistant message = %#v, want streamed assistant reply", stored[1])
	}
}

func TestChatStreamUsesPageContextTypeForRouteOnlyContext(t *testing.T) {
	db := setupAgentChatDB(t)
	provider := &recordingAgentChatProvider{
		deltas: []llm.ChatDelta{{Done: true}},
	}
	svc := NewAgentService(
		provider,
		nil,
		nil,
		nil,
		db,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	err := svc.ChatStream(
		context.Background(),
		7,
		[]llm.ChatMessage{{Role: "user", Content: "Where am I?"}},
		&model.AgentPageContext{Route: "/studio/overview"},
		func(_ string, _ bool, _ int64) error { return nil },
	)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var conversation model.AgentConversation
	if err := db.First(&conversation).Error; err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if conversation.ContextType != "page" {
		t.Fatalf("ContextType = %q, want page", conversation.ContextType)
	}
	if conversation.ContextID != nil {
		t.Fatalf("ContextID = %#v, want nil for route-only page context", conversation.ContextID)
	}
	if len(provider.lastRequest.Messages) == 0 || provider.lastRequest.Messages[0].Role != "system" {
		t.Fatalf("provider request = %#v, want system prompt prepended", provider.lastRequest.Messages)
	}
	if !strings.Contains(provider.lastRequest.Messages[0].Content, "Current page: /studio/overview") {
		t.Fatalf("system prompt = %q, want route context", provider.lastRequest.Messages[0].Content)
	}
}

func setupAgentChatDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
