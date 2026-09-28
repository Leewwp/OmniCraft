package service

// FT-6 (#698) 窄域守卫测试三族：剥离命中（取证同形态）/ 合法英文回答不
// 误伤 / 开关关闭直通；外加纯函数边界表与 trace 标记断言。

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

func TestStripBareEnglishReasoningPrefixTable(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  string
		runes int
	}{
		{
			name:  "strips the forensic bare-reasoning shape",
			in:    "The user is asking about the lighthouse keeper mod. I should search for it and summarize. 灯塔守望者是一个装饰类模组，支持自定义光源颜色。",
			want:  "灯塔守望者是一个装饰类模组，支持自定义光源颜色。",
			runes: 84,
		},
		{
			name: "strips leading whitespace after the prefix with the body",
			in:   "Sure, let me check that. 雨夜模组已找到。",
			want: "雨夜模组已找到。",
		},
		{
			name: "keeps a pure English answer (no CJK body)",
			in:   "The lighthouse keeper mod adds decorative lights. It supports custom colors.",
		},
		{
			name: "keeps Chinese-leading answers (no English prefix)",
			in:   "这个模组支持自定义光源。",
		},
		{
			name: "keeps an unfinished English run without a full sentence",
			in:   "The user asks about 雨夜模组的使用方式。",
		},
		{
			name: "keeps a single CJK rune followed by English (English body)",
			in:   "好 the mod supports custom colors.",
		},
		{
			name:  "strips the English lead-in but keeps body citation markers",
			in:    "I found two results. 第一篇 [1] 已核验，第二篇 [3] 为 IP 条目。",
			want:  "第一篇 [1] 已核验，第二篇 [3] 为 IP 条目。",
			runes: 21,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, runes := stripBareEnglishReasoningPrefix(tc.in)
			if tc.want == "" {
				if runes != 0 || got != tc.in {
					t.Fatalf("must not trigger: got (%q, %d)", got, runes)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("strip = %q, want %q", got, tc.want)
			}
			if runes <= 0 {
				t.Fatalf("strip must report a positive rune count, got %d", runes)
			}
		})
	}
}

// guardFixtureService builds a grounded one-search turn service with the
// guard explicitly enabled or disabled; the provider answer carries the
// forensic bare-reasoning shape.
func guardFixtureService(t *testing.T, guardEnabled bool, answer string) (*AgentService, *streamToolProvider) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.ContentItem{}, &model.ContentVersion{},
		&model.RagChunk{}, &model.IndexProjectionStatus{}, &model.AgentConversation{}, &model.AgentMessage{},
	))
	require.NoError(t, db.Create(&model.User{ID: 1, Username: "author", Email: "author@example.com"}).Error)
	now := time.Now()
	require.NoError(t, db.Exec(
		"INSERT INTO content_items (id, title, author_id, zone, content_type, status, is_public, allow_copy, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		100, "Published Public", 1, "original", "mod", "published", true, true, now, now,
	).Error)
	require.NoError(t, db.Create(&model.ContentVersion{
		ID: 70, ContentItemID: 100, AuthorID: 1, VersionNumber: 7,
		StorageType: "full", StorageKey: "snapshot", Status: "active", IsLatest: true,
	}).Error)
	require.NoError(t, db.Create(&model.RagChunk{
		ContentID: 100, ContentVersion: 7, ChunkIndex: 2, ChunkKey: strings.Repeat("a", 64),
		ChunkingVersion: 1, Heading: "Heading", Text: "A server-owned excerpt.",
		SourceStart: 0, SourceEnd: 24, Zone: "original", ContentType: "mod", IndexVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&model.IndexProjectionStatus{
		ContentID: 100, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "model",
		State: "ready", IsCurrent: true, ErrorSummary: "",
	}).Error)

	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"灯塔"}`)},
		{{Content: answer}, {Done: true}},
	}}
	retriever := &queuedRetriever{results: []AgentRetrievalResult{
		{Candidates: []AgentRetrievalCandidate{numberedCandidate(strings.Repeat("a", 64), "A server-owned excerpt.", 100, "Published Public")}},
	}}
	svc := NewAgentService(
		provider,
		nil,
		nil,
		nil,
		db,
		&config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{
			WebAgentEnabled: true, MaxToolCallsPerTurn: 4, CitationMaxCount: 5,
			MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
			AnswerBareReasoningGuard: config.AgentAnswerBareReasoningGuardConfig{Enabled: guardEnabled},
		}},
	)
	svc.hybridRetriever = retriever
	return svc, provider
}

func guardAnswer(t *testing.T, svc *AgentService) string {
	t.Helper()
	var done *AgentStreamEvent
	require.NoError(t, svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "灯塔"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch},
		func(event AgentStreamEvent) error {
			if event.Type == AgentEventDone {
				done = &event
			}
			return nil
		}))
	require.NotNil(t, done)
	require.Equal(t, AgentAnswerKind("grounded_content"), done.AnswerKind)
	return done.Answer
}

func TestChatStreamGuardStripsBareReasoningOnGroundedTurn(t *testing.T) {
	svc, _ := guardFixtureService(t, true,
		"The user is asking about the lighthouse mod. Let me search and answer. 灯塔守望者模组支持自定义光源颜色。")
	var logs strings.Builder
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previous)

	answer := guardAnswer(t, svc)
	require.NotContains(t, answer, "The user is asking", "leaked prefix must be stripped")
	require.True(t, strings.HasPrefix(answer, "灯塔"), "body must start at the Chinese answer")
	// 可观测先行：命中记 trace 事件与剥离长度。
	require.Contains(t, logs.String(), "answer_bare_reasoning_guard")
	require.Contains(t, logs.String(), "stripped")

	// 落库与终稿同一 answer：历史回放不再现泄漏。
	var stored model.AgentMessage
	require.NoError(t, svc.db.WithContext(context.Background()).
		Where("role = ?", "assistant").Order("id DESC").First(&stored).Error)
	require.NotNil(t, stored.Content)
	require.Equal(t, answer, *stored.Content, "persisted body must match the guarded final answer")
}

func TestChatStreamGuardKeepsLegitimateEnglishAnswer(t *testing.T) {
	svc, _ := guardFixtureService(t, true,
		"The lighthouse keeper mod adds decorative lights. It supports custom colors.")
	answer := guardAnswer(t, svc)
	require.Equal(t, "The lighthouse keeper mod adds decorative lights. It supports custom colors.", answer,
		"an English answer to an English-flavoured turn must pass through (no CJK body condition blocks)")
}

func TestChatStreamGuardDisabledPassesThrough(t *testing.T) {
	svc, _ := guardFixtureService(t, false,
		"The user is asking about the lighthouse mod. Let me search. 灯塔守望者模组支持自定义光源颜色。")
	answer := guardAnswer(t, svc)
	require.Contains(t, answer, "The user is asking", "disabled guard must pass everything through")
}
