package service

import (
	"bytes"
	"context"
	"encoding/json"
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
	"omnicraft/backend/internal/repository"
)

type contractRetriever struct {
	result  AgentRetrievalResult
	err     error
	queries []string
	viewers []int64
}

func (r *contractRetriever) Retrieve(_ context.Context, query string, viewerID int64) (AgentRetrievalResult, error) {
	r.queries = append(r.queries, query)
	r.viewers = append(r.viewers, viewerID)
	return r.result, r.err
}

func newContractService(db *gorm.DB) *AgentService {
	return NewAgentService(
		&recordingToolProvider{},
		nil,
		nil,
		nil,
		db,
		&config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{
			WebAgentEnabled: true, MaxToolCallsPerTurn: 4, CitationMaxCount: 5,
			MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
		}},
	)
}

func TestSearchContentHonorsDisabledRAGFeatureFlag(t *testing.T) {
	db := seedAgentGroundingDB(t)
	provider := &recordingToolProvider{}
	svc := NewAgentService(
		provider,
		repository.NewEmbeddingRepository(db),
		repository.NewContentRepository(db),
		nil,
		db,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 4, CitationMaxCount: 5}},
	)
	svc.vectorSearch = func([]float32, int) ([]repository.EmbeddingSearchResult, error) {
		return []repository.EmbeddingSearchResult{{ContentItemID: 100, Score: 1}}, nil
	}
	retriever := &contractRetriever{result: AgentRetrievalResult{Candidates: []AgentRetrievalCandidate{contractCandidate()}}}
	svc.SetContentRetriever(retriever)

	outcome, err := svc.ExecuteTool(context.Background(), ToolSearchContent, json.RawMessage(`{"query":"find this"}`), 3, nil)
	require.NoError(t, err)
	require.Empty(t, retriever.queries)
	require.Len(t, outcome.Search, 1)
	require.Empty(t, outcome.Search[0].Source)
}

func TestNLSearchDisabledRAGFallbackKeepsLegacySummaryShape(t *testing.T) {
	db := seedAgentGroundingDB(t)
	require.NoError(t, db.AutoMigrate(&model.ContentTag{}))
	svc := NewAgentService(
		&recordingToolProvider{},
		nil,
		repository.NewContentRepository(db),
		nil,
		db,
		&config.Config{Agent: config.AgentConfig{WebAgentEnabled: true}},
	)
	svc.searchRepo = repository.NewSearchRepository(db)

	result, err := svc.NLSearch(context.Background(), "Published", 3)
	require.NoError(t, err)
	require.True(t, result.Degraded)
	require.NotEmpty(t, result.Results)
	require.Empty(t, result.Results[0].Source)
}

func contractCandidate() AgentRetrievalCandidate {
	return AgentRetrievalCandidate{
		ChunkKey:        strings.Repeat("a", 64),
		ContentID:       100,
		ContentVersion:  7,
		ChunkIndex:      2,
		ChunkingVersion: 1,
		IndexVersion:    1,
		Title:           "Published Public",
		Text:            "A server-owned excerpt.",
		Source:          "hybrid_rrf",
		Zone:            "original",
		ContentType:     "mod",
	}
}

func TestSearchContentUsesHybridRetrieverAndNeverSerializesScore(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc := newContractService(db)
	retriever := &contractRetriever{result: AgentRetrievalResult{Candidates: []AgentRetrievalCandidate{contractCandidate()}}}
	svc.hybridRetriever = retriever

	outcome, err := svc.ExecuteTool(context.Background(), ToolSearchContent, json.RawMessage(`{"query":"find this"}`), 3, nil)
	require.NoError(t, err)
	require.Len(t, retriever.queries, 1)
	require.Equal(t, "find this", retriever.queries[0])
	require.Equal(t, []int64{3}, retriever.viewers)
	require.Len(t, outcome.Search, 1)
	require.Equal(t, "hybrid_rrf", outcome.Search[0].Source)
	require.Equal(t, int64(100), outcome.Search[0].ID)

	encoded, err := json.Marshal(outcome.Search)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"score"`)
}

func TestEnabledRAGRequiresHybridRetriever(t *testing.T) {
	svc := NewAgentService(
		&recordingToolProvider{},
		nil,
		nil,
		nil,
		nil,
		&config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{WebAgentEnabled: true}},
	)

	outcome, err := svc.ExecuteTool(context.Background(), ToolSearchContent, json.RawMessage(`{"query":"find this"}`), 3, nil)
	require.Error(t, err)
	require.Nil(t, outcome)
}

func seedCitationContractDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// :memory: databases are per-connection; pin one connection so the async
	// auto-title goroutine sees the same schema.
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
		"INSERT INTO content_items (id, title, author_id, zone, content_type, status, is_public, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		100, "Published Public", 1, "original", "mod", "published", true, now, now,
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
	return db
}

func validContractCitation() AgentCitation {
	return AgentCitation{
		ContentID:      100,
		ContentVersion: 7,
		ChunkKey:       strings.Repeat("a", 64),
		ChunkIndex:     2,
		Title:          "Published Public",
		Zone:           "original",
		Route:          "/original/100",
		Excerpt:        "A server-owned excerpt.",
		Source:         "hybrid_rrf",
	}
}

func TestRevalidateCitationsRejectsForgedFieldsAndLogsReasons(t *testing.T) {
	db := seedCitationContractDB(t)
	svc := newContractService(db)

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	valid := validContractCitation()
	for name, forged := range map[string]AgentCitation{
		"missing chunk key": func() AgentCitation { c := valid; c.ChunkKey = ""; return c }(),
		"wrong version":     func() AgentCitation { c := valid; c.ContentVersion = 6; return c }(),
		"wrong title":       func() AgentCitation { c := valid; c.Title = "forged"; return c }(),
		"wrong route":       func() AgentCitation { c := valid; c.Route = "/original/999"; return c }(),
		"wrong source":      func() AgentCitation { c := valid; c.Source = "forged"; return c }(),
	} {
		t.Run(name, func(t *testing.T) {
			logs.Reset()
			require.Empty(t, svc.RevalidateCitations(context.Background(), 3, []AgentCitation{forged}))
			require.NotContains(t, logs.String(), "content_id")
			require.Contains(t, logs.String(), "rejected_count=1")
		})
	}

	require.Equal(t, []AgentCitation{valid}, svc.RevalidateCitations(context.Background(), 3, []AgentCitation{valid}))
	require.Contains(t, logs.String(), "citation_revalidation")
	require.Contains(t, logs.String(), "reason")
}

func TestChatStreamEmitsExpandedCitationFromSearchResult(t *testing.T) {
	db := seedCitationContractDB(t)
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"find this"}`)},
		{{Content: "answer"}, {Done: true}},
	}}
	svc := NewAgentService(provider, nil, nil, nil, db, &config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{
		WebAgentEnabled: true, MaxToolCallsPerTurn: 4, CitationMaxCount: 5,
		MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
	}})
	svc.hybridRetriever = &contractRetriever{result: AgentRetrievalResult{Candidates: []AgentRetrievalCandidate{contractCandidate()}}}

	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "find this"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch},
		func(event AgentStreamEvent) error { events = append(events, event); return nil })
	require.NoError(t, err)

	var citation *AgentCitation
	for _, event := range events {
		if event.Type == AgentEventCitation {
			citation = event.Citation
		}
	}
	require.NotNil(t, citation)
	require.Equal(t, validContractCitation(), *citation)
}

func TestChatStreamDoesNotInventDetailCitationProvenance(t *testing.T) {
	db := seedCitationContractDB(t)
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolGetContentDetail, `{"content_id":100}`)},
		{{Content: "answer"}, {Done: true}},
	}}
	svc := NewAgentService(provider, nil, nil, nil, db, &config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{
		WebAgentEnabled: true, MaxToolCallsPerTurn: 4, CitationMaxCount: 5,
		MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
	}})

	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "show details"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch},
		func(event AgentStreamEvent) error { events = append(events, event); return nil })
	require.NoError(t, err)
	for _, event := range events {
		require.NotEqual(t, AgentEventCitation, event.Type, "detail without retrieval provenance must not become a citation")
	}
}

func TestChatStreamKeepsDetailCitationSourceBoundToChunk(t *testing.T) {
	db := seedCitationContractDB(t)
	require.NoError(t, db.Create(&model.RagChunk{
		ContentID: 100, ContentVersion: 7, ChunkIndex: 0, ChunkKey: strings.Repeat("b", 64),
		ChunkingVersion: 1, Text: "The first chunk.", SourceStart: 0, SourceEnd: 16,
		Zone: "original", ContentType: "mod", IndexVersion: 1,
	}).Error)
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"find this"}`)},
		{toolCallDelta(ToolGetContentDetail, `{"content_id":100}`)},
		{{Content: "answer", Done: true}},
	}}
	svc := NewAgentService(provider, nil, nil, nil, db, &config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{
		WebAgentEnabled: true, MaxToolCallsPerTurn: 4, CitationMaxCount: 5,
		MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
	}})
	svc.hybridRetriever = &contractRetriever{result: AgentRetrievalResult{Candidates: []AgentRetrievalCandidate{contractCandidate()}}}

	var events []AgentStreamEvent
	require.NoError(t, svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "find this"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch},
		func(event AgentStreamEvent) error { events = append(events, event); return nil }))

	var citations []AgentCitation
	for _, event := range events {
		if event.Type == AgentEventCitation && event.Citation != nil {
			citations = append(citations, *event.Citation)
		}
	}
	require.Len(t, citations, 1)
	require.Equal(t, strings.Repeat("a", 64), citations[0].ChunkKey)
	require.Equal(t, "hybrid_rrf", citations[0].Source)
}

func TestExpandedCitationSSEPayloadHasNoScoreOrVisibilityFlag(t *testing.T) {
	citation := validContractCitation()
	payload, err := json.Marshal(AgentStreamEvent{Type: AgentEventCitation, Citation: &citation})
	require.NoError(t, err)
	encoded := string(payload)
	require.Contains(t, encoded, `"content_version":7`)
	require.Contains(t, encoded, `"chunk_key"`)
	require.Contains(t, encoded, `"route":"/original/100"`)
	require.Contains(t, encoded, `"source":"hybrid_rrf"`)
	require.NotContains(t, encoded, `"score"`)
	require.NotContains(t, encoded, `"visibility_checked"`)
}
