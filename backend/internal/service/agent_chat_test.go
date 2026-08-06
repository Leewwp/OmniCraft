package service

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestChatStreamStoresServerOwnedContentContext(t *testing.T) {
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
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10}},
	)

	contentID := int64(88)
	chatCtx := &model.AgentChatContext{
		Surface:   model.AgentChatSurfaceContent,
		ContentID: &contentID,
	}
	resolved, err := svc.ResolveChatContext(context.Background(), 7, chatCtx)
	if err != nil {
		t.Fatalf("ResolveChatContext: %v", err)
	}

	var traceID string
	var doneKind AgentAnswerKind
	err = svc.ChatStream(
		context.Background(),
		7,
		[]llm.ChatMessage{{Role: "user", Content: "What is this content about?"}},
		resolved,
		func(ev AgentStreamEvent) error {
			if ev.Type == AgentEventDone {
				traceID = ev.TraceID
				doneKind = ev.AnswerKind
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if doneKind != AgentAnswerNoEvidence {
		t.Fatalf("done answer_kind = %s, want no_evidence without a fetched citation", doneKind)
	}

	var conv model.AgentConversation
	if err := db.First(&conv).Error; err != nil {
		t.Fatal("expected ChatStream to create a persisted conversation")
	}
	if conv.ContextType != "content" {
		t.Fatalf("ContextType = %q, want content", conv.ContextType)
	}
	if conv.ContextID == nil || *conv.ContextID != contentID {
		t.Fatalf("ContextID = %#v, want %d", conv.ContextID, contentID)
	}

	if len(provider.lastRequest.Messages) == 0 || provider.lastRequest.Messages[0].Role != "system" {
		t.Fatalf("provider request = %#v, want server-owned system prompt prepended", provider.lastRequest.Messages)
	}
	system := provider.lastRequest.Messages[0].Content
	if !strings.Contains(system, "Published Test Content") {
		t.Fatalf("system prompt = %q, want server-reloaded content title", system)
	}
	if strings.Contains(system, "Route:") || strings.Contains(system, "client") {
		t.Fatalf("system prompt = %q, must not contain client-authored route/summary", system)
	}
	if traceID == "" || len(traceID) != 32 {
		t.Fatalf("done callback traceID = %q, want 32-hex trace id", traceID)
	}
}

func TestChatStreamGlobalSurfaceHasNoContentContext(t *testing.T) {
	db := setupAgentChatDB(t)
	provider := &recordingAgentChatProvider{}
	svc := NewAgentService(
		provider,
		nil,
		nil,
		nil,
		db,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10}},
	)

	resolved, err := svc.ResolveChatContext(context.Background(), 7, &model.AgentChatContext{Surface: model.AgentChatSurfaceGlobal})
	if err != nil {
		t.Fatalf("ResolveChatContext: %v", err)
	}

	err = svc.ChatStream(
		context.Background(),
		7,
		[]llm.ChatMessage{{Role: "user", Content: "hi"}},
		resolved,
		func(ev AgentStreamEvent) error { return nil },
	)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}

	var conv model.AgentConversation
	if err := db.First(&conv).Error; err != nil {
		t.Fatal("expected ChatStream to create a persisted conversation")
	}
	if conv.ContextType != "general" {
		t.Fatalf("ContextType = %q, want general", conv.ContextType)
	}
	if conv.ContextID != nil {
		t.Fatalf("ContextID = %#v, want nil for global surface", conv.ContextID)
	}
	if len(provider.lastRequest.Messages) == 0 || provider.lastRequest.Messages[0].Role != "system" {
		t.Fatalf("provider request = %#v, want system prompt prepended", provider.lastRequest.Messages)
	}
	if !strings.Contains(provider.lastRequest.Messages[0].Content, "global") {
		t.Fatalf("system prompt = %q, want global surface prompt", provider.lastRequest.Messages[0].Content)
	}
}

func setupAgentChatDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}, &model.User{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Create(&model.User{ID: 1, Username: "author", Email: "author@example.com"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	now := time.Now()
	if err := db.Create(&model.ContentItem{ID: 88, Title: "Published Test Content", AuthorID: 1, Zone: "fanwork", ContentType: "mod", Status: "published", IsPublic: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed content: %v", err)
	}
	return db
}
