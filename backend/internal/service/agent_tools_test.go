package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
)

// --- fixtures -----------------------------------------------------------------

type recordingToolProvider struct {
	chatCalls   int
	streamCalls int
	lastRequest llm.ChatRequest
}

func (p *recordingToolProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	p.chatCalls++
	return &llm.ChatResponse{}, nil
}

func (p *recordingToolProvider) ChatStream(_ context.Context, req llm.ChatRequest, _ func(delta llm.ChatDelta) error) error {
	p.streamCalls++
	p.lastRequest = req
	return nil
}

func (p *recordingToolProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

func seedAgentGroundingDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	users := []model.User{
		{ID: 1, Username: "visible-author", Email: "v@example.com"},
		{ID: 2, Username: "banned-author", Email: "b@example.com", IsBanned: true},
		{ID: 3, Username: "viewer", Email: "viewer@example.com"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	now := time.Now()
	// NOTE: GORM replaces zero values of `default`-tagged fields on Create
	// (IsPublic default:true would store true), so fixtures are inserted with
	// raw SQL to keep deterministic visibility states.
	contents := []model.ContentItem{
		{ID: 100, Title: "Published Public", AuthorID: 1, Zone: "original", ContentType: "mod", Status: "published", IsPublic: true, CreatedAt: now, UpdatedAt: now},
		{ID: 101, Title: "Private Other Author", AuthorID: 1, Zone: "original", ContentType: "mod", Status: "published", IsPublic: false, CreatedAt: now, UpdatedAt: now},
		{ID: 102, Title: "Banned Author Content", AuthorID: 2, Zone: "original", ContentType: "mod", Status: "published", IsPublic: true, CreatedAt: now, UpdatedAt: now},
		{ID: 103, Title: "Under Review", AuthorID: 1, Zone: "original", ContentType: "mod", Status: "under_review", IsPublic: true, CreatedAt: now, UpdatedAt: now},
		{ID: 104, Title: "Soft Deleted", AuthorID: 1, Zone: "original", ContentType: "mod", Status: "published", IsPublic: true, DeletedAt: &now, CreatedAt: now, UpdatedAt: now},
		{ID: 105, Title: "Viewer Own Private", AuthorID: 3, Zone: "original", ContentType: "mod", Status: "published", IsPublic: false, CreatedAt: now, UpdatedAt: now},
	}
	for _, c := range contents {
		if err := db.Exec(
			"INSERT INTO content_items (id, title, description, author_id, zone, content_type, status, is_public, created_at, updated_at, deleted_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
			c.ID, c.Title, c.Description, c.AuthorID, c.Zone, c.ContentType, c.Status, c.IsPublic, c.CreatedAt, c.UpdatedAt, c.DeletedAt,
		).Error; err != nil {
			t.Fatalf("seed content %d: %v", c.ID, err)
		}
	}
	return db
}

func newToolTestService(db *gorm.DB) (*AgentService, *recordingToolProvider) {
	provider := &recordingToolProvider{}
	svc := NewAgentService(
		provider,
		nil,
		repository.NewContentRepository(db),
		nil,
		db,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 2, CitationMaxCount: 5, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200}},
	)
	return svc, provider
}

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return b
}

// --- TestAgentToolPolicy --------------------------------------------------------

func TestAgentToolPolicy(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc, provider := newToolTestService(db)
	ctx := context.Background()
	const viewerID = int64(3)

	t.Run("rejects unknown tools including injection-shaped names", func(t *testing.T) {
		for _, name := range []string{"drop_table", "search_content; DROP TABLE content_items", "suggest_publish_metadata --all", "get_secret"} {
			_, err := svc.ExecuteTool(ctx, name, rawJSON(t, map[string]any{}), viewerID, nil)
			if err == nil || !errors.Is(err, ErrAgentToolUnknown) {
				t.Fatalf("tool %q err = %v, want ErrAgentToolUnknown", name, err)
			}
		}
		if provider.chatCalls+provider.streamCalls != 0 {
			t.Fatal("unknown tools must never reach the provider")
		}
	})

	t.Run("registered tool names are server-owned constants", func(t *testing.T) {
		names := svc.RegisteredToolNames()
		want := map[string]bool{"search_content": true, "get_content_detail": true, "get_usage_guide": true, "suggest_publish_metadata": true}
		if len(names) != len(want) {
			t.Fatalf("registered tools = %v, want exactly %d", names, len(want))
		}
		for _, n := range names {
			if !want[n] {
				t.Fatalf("unexpected registered tool %q", n)
			}
		}
	})

	t.Run("tool args reject unknown fields", func(t *testing.T) {
		_, err := svc.ExecuteTool(ctx, "get_content_detail", rawJSON(t, map[string]any{"content_id": 100, "mode": "delete"}), viewerID, nil)
		if err == nil || !errors.Is(err, ErrAgentToolInvalidArgs) {
			t.Fatalf("unknown arg field err = %v, want ErrAgentToolInvalidArgs", err)
		}
		_, err = svc.ExecuteTool(ctx, "search_content", rawJSON(t, map[string]any{"query": "cats", "limit": 99999}), viewerID, nil)
		if err == nil || !errors.Is(err, ErrAgentToolInvalidArgs) {
			t.Fatalf("unknown arg field err = %v, want ErrAgentToolInvalidArgs", err)
		}
	})

	t.Run("tool args reject invalid limits", func(t *testing.T) {
		for _, bad := range []any{-1, 0, "abc", 1.5} {
			_, err := svc.ExecuteTool(ctx, "get_content_detail", rawJSON(t, map[string]any{"content_id": bad}), viewerID, nil)
			if err == nil || !errors.Is(err, ErrAgentToolInvalidArgs) {
				t.Fatalf("content_id=%v err = %v, want ErrAgentToolInvalidArgs", bad, err)
			}
		}
		longQuery := strings.Repeat("a", defaultMaxToolQueryLength+1)
		_, err := svc.ExecuteTool(ctx, "search_content", rawJSON(t, map[string]any{"query": longQuery}), viewerID, nil)
		if err == nil || !errors.Is(err, ErrAgentToolInvalidArgs) {
			t.Fatalf("oversized query err = %v, want ErrAgentToolInvalidArgs", err)
		}
	})

	t.Run("suggest_publish_metadata accepts no resource args and needs a bound snapshot", func(t *testing.T) {
		_, err := svc.ExecuteTool(ctx, "suggest_publish_metadata", rawJSON(t, map[string]any{"content_id": 100}), viewerID, nil)
		if err == nil || !errors.Is(err, ErrAgentToolInvalidArgs) {
			t.Fatalf("model-supplied resource args err = %v, want ErrAgentToolInvalidArgs", err)
		}
		_, err = svc.ExecuteTool(ctx, "suggest_publish_metadata", rawJSON(t, map[string]any{"draft_id": 7}), viewerID, nil)
		if err == nil || !errors.Is(err, ErrAgentToolInvalidArgs) {
			t.Fatalf("model-supplied draft id err = %v, want ErrAgentToolInvalidArgs", err)
		}
		outcome, err := svc.ExecuteTool(ctx, "suggest_publish_metadata", rawJSON(t, map[string]any{}), viewerID, &AgentPublishSnapshot{Title: "My Title", Description: "My Description"})
		if err != nil {
			t.Fatalf("snapshot-bound publish suggestion err = %v, want success", err)
		}
		if outcome.Suggest == nil || !strings.Contains(outcome.Suggest.SuggestedTitle, "My Title") {
			t.Fatalf("suggestion outcome = %#v, want snapshot-backed suggestion", outcome)
		}
	})

	t.Run("tool-call limit is config-driven and stops the loop", func(t *testing.T) {
		policy := svc.ToolPolicy()
		if policy.MaxCallsPerTurn != 2 {
			t.Fatalf("MaxCallsPerTurn = %d, want 2 from config", policy.MaxCallsPerTurn)
		}
		if !policy.AllowToolCall(0) || !policy.AllowToolCall(1) {
			t.Fatal("AllowToolCall must allow calls below the limit")
		}
		if policy.AllowToolCall(2) || policy.AllowToolCall(99) {
			t.Fatal("AllowToolCall must stop the loop at the limit")
		}
	})
}

// --- TestAgentGrounding ----------------------------------------------------------

func TestAgentGrounding(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc, provider := newToolTestService(db)
	ctx := context.Background()
	const viewerID = int64(3)

	t.Run("hidden content never reaches the provider through tools", func(t *testing.T) {
		hidden := []int64{101, 102, 103, 104, 999}
		for _, id := range hidden {
			outcome, err := svc.ExecuteTool(ctx, "get_content_detail", rawJSON(t, map[string]any{"content_id": id}), viewerID, nil)
			if err == nil || !errors.Is(err, ErrContentNotFound) {
				t.Fatalf("get_content_detail(%d) err = %v, want ErrContentNotFound", id, err)
			}
			if outcome != nil && outcome.Detail != nil {
				t.Fatalf("get_content_detail(%d) leaked detail %#v", id, outcome.Detail)
			}
			_, err = svc.ExecuteTool(ctx, "get_usage_guide", rawJSON(t, map[string]any{"content_id": id}), viewerID, nil)
			if err == nil || !errors.Is(err, ErrContentNotFound) {
				t.Fatalf("get_usage_guide(%d) err = %v, want ErrContentNotFound", id, err)
			}
		}
		if provider.chatCalls != 0 {
			t.Fatal("hidden content IDs must never reach the provider")
		}
	})

	t.Run("visible content resolves through the shared resolver", func(t *testing.T) {
		outcome, err := svc.ExecuteTool(ctx, "get_content_detail", rawJSON(t, map[string]any{"content_id": 100}), viewerID, nil)
		if err != nil {
			t.Fatalf("get_content_detail(100) err = %v", err)
		}
		if outcome.Detail == nil || outcome.Detail.ID != 100 || outcome.Detail.Title != "Published Public" {
			t.Fatalf("detail = %#v, want content 100 summary", outcome.Detail)
		}
		// Viewer-owned private content is visible to its author.
		outcome, err = svc.ExecuteTool(ctx, "get_content_detail", rawJSON(t, map[string]any{"content_id": 105}), viewerID, nil)
		if err != nil || outcome.Detail == nil {
			t.Fatalf("viewer-owned private content err = %v outcome = %#v", err, outcome)
		}
	})

	t.Run("usage-guide endpoints reuse the same viewer-aware resolver", func(t *testing.T) {
		_, err := svc.UsageGuide(ctx, viewerID, 101)
		if err == nil || !errors.Is(err, ErrContentNotFound) {
			t.Fatalf("UsageGuide(private) err = %v, want ErrContentNotFound", err)
		}
		_, err = svc.UsageGuide(ctx, viewerID, 102)
		if err == nil || !errors.Is(err, ErrContentNotFound) {
			t.Fatalf("UsageGuide(banned author) err = %v, want ErrContentNotFound", err)
		}
		if err := svc.UsageGuideStream(ctx, viewerID, 103, func(string, bool) error { return nil }); !errors.Is(err, ErrContentNotFound) {
			t.Fatalf("UsageGuideStream(under review) err = %v, want ErrContentNotFound", err)
		}
		if provider.chatCalls+provider.streamCalls != 0 {
			t.Fatal("hidden usage-guide requests must never reach the provider")
		}
		if err := svc.UsageGuideStream(ctx, viewerID, 100, func(string, bool) error { return nil }); err != nil {
			t.Fatalf("UsageGuideStream(visible) err = %v, want success", err)
		}
		if provider.streamCalls != 1 {
			t.Fatalf("UsageGuideStream(visible) provider stream calls = %d, want 1", provider.streamCalls)
		}
	})

	t.Run("chat context is server-owned: reloads visibility and rejects hidden ids before provider", func(t *testing.T) {
		beforeStream := provider.streamCalls
		hiddenID := int64(101)
		chatCtx := &model.AgentChatContext{Surface: model.AgentChatSurfaceContent, ContentID: &hiddenID}
		_, err := svc.ResolveChatContext(ctx, viewerID, chatCtx)
		if err == nil || !errors.Is(err, ErrContentNotFound) {
			t.Fatalf("ResolveChatContext(hidden content ctx) err = %v, want ErrContentNotFound", err)
		}
		if provider.streamCalls != beforeStream {
			t.Fatal("hidden chat context must be rejected before the provider is called")
		}
		var count int64
		db.Model(&model.AgentConversation{}).Count(&count)
		if count != 0 {
			t.Fatalf("conversations created for hidden context = %d, want 0", count)
		}
	})

	t.Run("chat context reloads server-owned title instead of client summaries", func(t *testing.T) {
		beforeStream := provider.streamCalls
		visibleID := int64(100)
		chatCtx := &model.AgentChatContext{Surface: model.AgentChatSurfaceContent, ContentID: &visibleID}
		resolved, err := svc.ResolveChatContext(ctx, viewerID, chatCtx)
		if err != nil {
			t.Fatalf("ResolveChatContext(visible): %v", err)
		}
		if err := svc.ChatStream(ctx, viewerID, []llm.ChatMessage{{Role: "user", Content: "tell me"}}, resolved, func(AgentStreamEvent) error { return nil }); err != nil {
			t.Fatalf("ChatStream(visible) err = %v", err)
		}
		if provider.streamCalls != beforeStream+1 {
			t.Fatalf("provider stream calls delta = %d, want 1", provider.streamCalls-beforeStream)
		}
		system := provider.lastRequest.Messages[0].Content
		if !strings.Contains(system, "Published Public") {
			t.Fatalf("system prompt = %q, want server-loaded title", system)
		}
		if strings.Contains(system, "client summary") || strings.Contains(system, "Route:") {
			t.Fatalf("system prompt = %q, must not contain client-authored summaries", system)
		}
		var conv model.AgentConversation
		if err := db.First(&conv).Error; err != nil {
			t.Fatalf("conversation: %v", err)
		}
		if conv.ContextType != "content" || conv.ContextID == nil || *conv.ContextID != 100 {
			t.Fatalf("conversation context = %s/%v, want content/100", conv.ContextType, conv.ContextID)
		}
	})

	t.Run("every returned citation is revalidated after model output", func(t *testing.T) {
		citations := []AgentCitation{
			{ContentID: 100, Title: "Published Public", Zone: "original"},
			{ContentID: 101, Title: "Private", Zone: "original"},
			{ContentID: 102, Title: "Banned", Zone: "original"},
			{ContentID: 103, Title: "Under Review", Zone: "original"},
			{ContentID: 104, Title: "Soft Deleted", Zone: "original"},
			{ContentID: 9999, Title: "Missing", Zone: "original"},
			{ContentID: 0, Title: "", Zone: ""},
		}
		valid := svc.RevalidateCitations(ctx, viewerID, citations)
		if len(valid) != 1 || valid[0].ContentID != 100 {
			t.Fatalf("revalidated citations = %#v, want only content 100", valid)
		}
	})

	t.Run("grounded answers without valid citations become no_evidence", func(t *testing.T) {
		if got := ClassifyGroundedAnswer([]AgentCitation{}); got != AgentAnswerNoEvidence {
			t.Fatalf("ClassifyGroundedAnswer([]) = %s, want no_evidence", got)
		}
		if got := ClassifyGroundedAnswer([]AgentCitation{{ContentID: 100, Title: "T", Zone: "original"}}); got != AgentAnswerGroundedContent {
			t.Fatalf("ClassifyGroundedAnswer([valid]) = %s, want grounded_content", got)
		}
		if got := ClassifyGroundedAnswer(nil); got != AgentAnswerNoEvidence {
			t.Fatalf("ClassifyGroundedAnswer(nil) = %s, want no_evidence", got)
		}
	})

	t.Run("trace ids are generated and stable across requests", func(t *testing.T) {
		a, b := newTraceID(), newTraceID()
		if a == "" || a == b || len(a) != 32 {
			t.Fatalf("trace ids %q %q, want two distinct 32-hex values", a, b)
		}
	})
}
