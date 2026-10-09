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
)

// setupGuestChatDB mirrors the guest conversation fixtures on the in-memory
// harness: the shared agent tables plus the guest projection struct.
func setupGuestChatDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	// The shared table is created post-089 (user_id nullable + guest columns)
	// via raw SQL: two AutoMigrate authorities on one table make GORM align
	// the schema to the last struct (dropping user_id), and sqlite cannot
	// ALTER a NOT NULL away. Every column both shapes write exists here.
	if err := db.Exec(`CREATE TABLE agent_conversations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id BIGINT,
		context_type VARCHAR(50) NOT NULL DEFAULT '',
		context_id BIGINT,
		title VARCHAR(200),
		pinned_at DATETIME,
		is_guest BOOLEAN NOT NULL DEFAULT FALSE,
		guest_device_key VARCHAR(64),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create agent_conversations: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentMessage{}, &model.User{}, &model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func guestService(db *gorm.DB, provider llm.LLMProvider, rt ToolRuntime) *AgentService {
	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled:     true,
		MaxUserMessageChars: 4000,
		ChatMaxContextMsgs:  10,
		MaxToolCallsPerTurn: 8,
		CitationMaxCount:    5,
		MaxOutputTokens:     100,
		Guest: config.AgentGuestConfig{
			MaxTotalTurns:       3,
			ConversationTTLDays: 7,
			MaxConcurrentTurns:  3,
			CookieMaxAgeHours:   8760,
		},
	}}
	svc := newAgentServiceWithChatStreamer(provider, provider, nil, nil, nil, db, cfg)
	if rt != nil {
		svc.SetToolRuntime(rt)
	}
	return svc
}

func runGuestTurn(t *testing.T, svc *AgentService, deviceKey string, conversationID int64, message string) error {
	t.Helper()
	return svc.ChatStream(
		context.Background(),
		0, // guest turns run under the anonymous public viewer
		ChatTurnInput{ConversationID: conversationID, Message: message, GuestDeviceKey: deviceKey},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal},
		func(AgentStreamEvent) error { return nil },
	)
}

func assertGuestOwnership(t *testing.T, db *gorm.DB, convID int64, deviceKey string) {
	t.Helper()
	var userID *int64
	var isGuest bool
	var key *string
	if err := db.Raw(`SELECT user_id, is_guest, guest_device_key FROM agent_conversations WHERE id = ?`, convID).
		Row().Scan(&userID, &isGuest, &key); err != nil {
		t.Fatalf("read guest ownership: %v", err)
	}
	if userID != nil {
		t.Fatalf("guest conversation %d must never claim a user id, got %d", convID, *userID)
	}
	if !isGuest || key == nil || *key != deviceKey {
		t.Fatalf("guest conversation flags wrong: is_guest=%v key=%v", isGuest, key)
	}
}

func TestGuestTurnCreatesDeviceOwnedConversation(t *testing.T) {
	db := setupGuestChatDB(t)
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{{Content: "grounded answer"}, {Done: true}},
	}}
	svc := guestService(db, provider, nil)

	if err := runGuestTurn(t, svc, "device-1", 0, "first question"); err != nil {
		t.Fatalf("guest first turn: %v", err)
	}
	var conv model.AgentConversation
	if err := db.First(&conv).Error; err != nil {
		t.Fatalf("read conversation: %v", err)
	}
	assertGuestOwnership(t, db, conv.ID, "device-1")

	// Second turn continues the same conversation.
	if err := runGuestTurn(t, svc, "device-1", conv.ID, "second question"); err != nil {
		t.Fatalf("guest second turn: %v", err)
	}
	var msgCount int64
	db.Raw(`SELECT count(*) FROM agent_messages WHERE conversation_id = ? AND role = 'user'`, conv.ID).Scan(&msgCount)
	if msgCount != 2 {
		t.Fatalf("user rows = %d, want 2", msgCount)
	}

	// Regeneration is a new turn: it consumes another request and appends a
	// new user row — never a quota-free replay.
	if err := runGuestTurn(t, svc, "device-1", conv.ID, "second question"); err != nil {
		t.Fatalf("guest regenerate turn: %v", err)
	}
	db.Raw(`SELECT count(*) FROM agent_messages WHERE conversation_id = ? AND role = 'user'`, conv.ID).Scan(&msgCount)
	if msgCount != 3 {
		t.Fatalf("user rows after regenerate = %d, want 3", msgCount)
	}
}

func TestGuestConversationIsInvisibleAcrossDevices(t *testing.T) {
	db := setupGuestChatDB(t)
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{{Content: "answer"}, {Done: true}},
	}}
	svc := guestService(db, provider, nil)
	if err := runGuestTurn(t, svc, "device-1", 0, "question"); err != nil {
		t.Fatalf("first device turn: %v", err)
	}
	var conv model.AgentConversation
	db.First(&conv)

	// The cross-device continuation must fail with the owner-scoped miss; the
	// stream surfaces the typed event, not a panic.
	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), 0,
		ChatTurnInput{ConversationID: conv.ID, Message: "hi-jack", GuestDeviceKey: "device-2"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal},
		func(ev AgentStreamEvent) error { events = append(events, ev); return nil })
	if !errors.Is(err, ErrAgentConversationNotFound) {
		t.Fatalf("cross-device continuation must fail with the owner-scoped miss, got %v", err)
	}
	if len(events) == 0 || events[len(events)-1].Type != AgentEventError {
		t.Fatalf("cross-device continuation must end in an error event, got %#v", events)
	}
	if code := events[len(events)-1].ErrorCode; code != AgentErrorCodeConversationNotFound {
		t.Fatalf("error code = %s, want %s", code, AgentErrorCodeConversationNotFound)
	}
}

func TestEnsureGuestConversationUsableEnforcesOwnershipAndExpiry(t *testing.T) {
	db := setupGuestChatDB(t)
	svc := guestService(db, nil, nil)

	// Seed one live and one expired guest conversation.
	live := &model.AgentGuestConversation{IsGuest: true, GuestDeviceKey: "device-1", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	expired := &model.AgentGuestConversation{IsGuest: true, GuestDeviceKey: "device-1", CreatedAt: time.Now().Add(-8 * 24 * time.Hour), UpdatedAt: time.Now().Add(-8 * 24 * time.Hour)}
	if err := db.Create(live).Error; err != nil {
		t.Fatalf("seed live: %v", err)
	}
	if err := db.Create(expired).Error; err != nil {
		t.Fatalf("seed expired: %v", err)
	}

	if err := svc.EnsureGuestConversationUsable(context.Background(), "device-1", live.ID); err != nil {
		t.Fatalf("live conversation must be usable: %v", err)
	}
	if err := svc.EnsureGuestConversationUsable(context.Background(), "device-1", expired.ID); !errors.Is(err, ErrAgentConversationExpired) {
		t.Fatalf("expired conversation must be refused read and write, got %v", err)
	}
	if err := svc.EnsureGuestConversationUsable(context.Background(), "device-2", live.ID); !errors.Is(err, ErrAgentConversationNotFound) {
		t.Fatalf("foreign device must see the owner-scoped miss, got %v", err)
	}
}

func TestListGuestConversationsScopedAndExpiredFiltered(t *testing.T) {
	db := setupGuestChatDB(t)
	svc := guestService(db, nil, nil)

	rows := []*model.AgentGuestConversation{
		{IsGuest: true, GuestDeviceKey: "device-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{IsGuest: true, GuestDeviceKey: "device-1", CreatedAt: time.Now().Add(-9 * 24 * time.Hour), UpdatedAt: time.Now()},
		{IsGuest: true, GuestDeviceKey: "device-2", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, err := svc.ListGuestConversations(context.Background(), "device-1", 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].GuestDeviceKey != "device-1" {
		t.Fatalf("list = %+v, want only the live own conversation", got)
	}
}

// countingToolRuntime records which tools reached the execution seam.
type countingToolRuntime struct {
	inner    ToolRuntime
	defs     []llm.ToolDefinition
	executed []string
}

func (r *countingToolRuntime) ToolDefinitions(_ context.Context, _ int64) []llm.ToolDefinition {
	return r.defs
}

func (r *countingToolRuntime) ExecuteTool(ctx context.Context, name string, raw json.RawMessage, scope ToolScope) (*AgentToolOutcome, error) {
	r.executed = append(r.executed, name)
	return r.inner.ExecuteTool(ctx, name, raw, scope)
}

// TestGuestToolSurfaceDoubleWhitelist drives a guest turn whose model emits
// one allowed and two privileged tool calls. Definitions shown to the model
// carry only the public read-only allowlist; privileged names never reach the
// inner execution seam (execution-side check), even when the model forges them.
func TestGuestToolSurfaceDoubleWhitelist(t *testing.T) {
	db := setupGuestChatDB(t)
	inner := &fakeLoopRuntime{byName: map[string]struct{}{}}
	counting := &countingToolRuntime{
		inner: inner,
		defs: []llm.ToolDefinition{
			{Name: ToolSearchContent},
			{Name: ToolSearchIPs},
			{Name: ToolGetContentDetail},
			{Name: ToolGetUsageGuide},
			{Name: ToolSuggestPublishMetadata},
			{Name: ToolGenerateImage},
			{Name: "mcp_home_server_tool"},
		},
	}
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{
			toolCallDelta(ToolGenerateImage, `{"prompt":"a cat"}`),
			toolCallDelta("mcp_home_server_tool", `{}`),
			toolCallDelta(ToolSearchContent, `{"query":"piano"}`),
		},
		{{Content: "done"}, {Done: true}},
	}}
	svc := guestService(db, provider, counting)

	if err := runGuestTurn(t, svc, "device-1", 0, "draw and search"); err != nil {
		t.Fatalf("guest tool turn: %v", err)
	}

	// Execution side: only the allowed tool ever reached the real executor.
	if len(counting.executed) != 1 || counting.executed[0] != ToolSearchContent {
		t.Fatalf("executed = %v, want only search_content", counting.executed)
	}
	for name := range inner.byName {
		if name != ToolSearchContent {
			t.Fatalf("privileged tool %q escaped the guest whitelist", name)
		}
	}
	// The forged privileged calls degrade to the safe unknown-tool result.
	var privilegedUnknown int
	var conv model.AgentConversation
	db.First(&conv)
	var toolsRow model.AgentMessage
	if err := db.Where("conversation_id = ? AND role = 'assistant' AND tool_calls IS NOT NULL", conv.ID).
		Order("created_at ASC").First(&toolsRow).Error; err == nil {
		raw, _ := json.Marshal(toolsRow.ToolCalls["steps"])
		privilegedUnknown = strings.Count(string(raw), "(unknown)")
	}
	if privilegedUnknown < 2 {
		t.Fatalf("forged privileged calls must surface as unknown-tool steps, got %d", privilegedUnknown)
	}
}

// defsOnlyRuntime advertises a fixed definition surface and forwards
// executions, recording every name that reaches it.
type defsOnlyRuntime struct {
	defs     []llm.ToolDefinition
	inner    ToolRuntime
	executed []string
}

func (r *defsOnlyRuntime) ToolDefinitions(context.Context, int64) []llm.ToolDefinition { return r.defs }

func (r *defsOnlyRuntime) ExecuteTool(ctx context.Context, name string, raw json.RawMessage, scope ToolScope) (*AgentToolOutcome, error) {
	r.executed = append(r.executed, name)
	if r.inner != nil {
		return r.inner.ExecuteTool(ctx, name, raw, scope)
	}
	return &AgentToolOutcome{}, nil
}

func TestGuestDefinitionsExposeOnlyPublicReadTools(t *testing.T) {
	db := setupGuestChatDB(t)
	inner := &fakeLoopRuntime{}
	defsRt := &defsOnlyRuntime{
		defs: []llm.ToolDefinition{
			{Name: ToolSearchContent},
			{Name: ToolSuggestPublishMetadata},
			{Name: ToolGenerateImage},
			{Name: "mcp_x"},
		},
		inner: inner,
	}
	svc := guestService(db, nil, defsRt)
	runtime := svc.turnToolRuntime(ChatTurnInput{GuestDeviceKey: "device-1"})
	defs := runtime.ToolDefinitions(context.Background(), 0)
	if len(defs) != 1 || defs[0].Name != ToolSearchContent {
		t.Fatalf("guest definitions = %+v, want only search_content", defs)
	}

	// Privileged execution is refused at the wrapper without touching the
	// inner seam.
	if _, err := runtime.ExecuteTool(context.Background(), ToolGenerateImage, json.RawMessage(`{}`), ToolScope{ViewerID: 7}); !errors.Is(err, ErrAgentToolUnknown) {
		t.Fatalf("privileged execution must fail as unknown tool, got %v", err)
	}
	if len(inner.byName) != 0 {
		t.Fatalf("privileged execution reached the inner seam: %v", inner.byName)
	}
	if len(defsRt.executed) != 0 {
		t.Fatalf("privileged execution bypassed the wrapper: %v", defsRt.executed)
	}

	// Logged-in turns keep the full runtime untouched.
	full := svc.turnToolRuntime(ChatTurnInput{})
	if _, isGuest := full.(*guestToolRuntime); isGuest {
		t.Fatal("logged-in turns must not ride the guest whitelist wrapper")
	}
}

func TestGuestAutoTitleSkipped(t *testing.T) {
	db := setupGuestChatDB(t)
	provider := &scriptedStreamProvider{rounds: [][]llm.ChatDelta{
		{{Content: "answer"}, {Done: true}},
	}}
	svc := guestService(db, provider, nil)
	if err := runGuestTurn(t, svc, "device-1", 0, "question"); err != nil {
		t.Fatalf("guest turn: %v", err)
	}
	var conv model.AgentConversation
	db.First(&conv)
	if conv.Title != nil {
		t.Fatalf("guest conversations must not spend an auto-title provider call, got title %q", *conv.Title)
	}
}

func TestGuestCleanupWorkerDeletesExpiredKeepsCounterShape(t *testing.T) {
	db := setupGuestChatDB(t)
	old := time.Now().Add(-9 * 24 * time.Hour)
	rows := []*model.AgentGuestConversation{
		{IsGuest: true, GuestDeviceKey: "device-1", CreatedAt: old, UpdatedAt: old},
		{IsGuest: true, GuestDeviceKey: "device-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	user := &model.User{ID: 5, Username: "u5", Email: "u5@example.com"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	userConv := &model.AgentConversation{UserID: user.ID, CreatedAt: old, UpdatedAt: old}
	if err := db.Create(userConv).Error; err != nil {
		t.Fatalf("seed user conversation: %v", err)
	}

	cleaner := NewGuestConversationCleaner(db, 7*24*time.Hour)
	deleted, err := cleaner.RunOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want exactly the expired guest conversation", deleted)
	}
	var remaining []model.AgentGuestConversation
	db.Where("is_guest = ?", true).Find(&remaining)
	if len(remaining) != 1 || remaining[0].GuestDeviceKey != "device-1" {
		t.Fatalf("remaining guest rows = %+v", remaining)
	}
	var userRows int64
	db.Model(&model.AgentConversation{}).Where("user_id IS NOT NULL").Count(&userRows)
	if userRows != 1 {
		t.Fatalf("cleanup must never touch user conversations, left %d", userRows)
	}
}

func TestGuestConversationCleanerFallsBackToSevenDays(t *testing.T) {
	cleaner := NewGuestConversationCleaner(nil, 0)
	if cleaner.ttl != 7*24*time.Hour {
		t.Fatalf("fallback ttl = %v, want the read-path 7-day window", cleaner.ttl)
	}
}
