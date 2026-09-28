package service

// FT-5 (#697) turn-global citation numbering: the tool result the model sees
// stamps every citable result with a number that accumulates across all
// search calls of the turn, re-seen results keep their original number, the
// emitted citation pool preserves dropped slots (no renumbering), and orphan
// [n] markers are stripped by number set — the structural misalignment the
// 2026-09-28 DB forensics caught (model numbering by per-call local index
// while the pool accumulates globally).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
)

// queuedRetriever pops one canned result per Retrieve call so a single turn
// can fan out across search calls with different hit sets.
type queuedRetriever struct {
	results []AgentRetrievalResult
	calls   int
}

func (r *queuedRetriever) Retrieve(_ context.Context, _ string, _ int64) (AgentRetrievalResult, error) {
	if r.calls >= len(r.results) {
		return AgentRetrievalResult{}, nil
	}
	result := r.results[r.calls]
	r.calls++
	return result, nil
}

// numberedCandidate builds a retrieval candidate whose Text matches the
// seeded RagChunk.Text exactly — revalidation compares the citation excerpt
// against the current chunk truth (excerpt_mismatch).
func numberedCandidate(key, text string, contentID int64, title string) AgentRetrievalCandidate {
	return AgentRetrievalCandidate{
		ChunkKey:        key,
		ContentID:       contentID,
		ContentVersion:  7,
		ChunkIndex:      2,
		ChunkingVersion: 1,
		IndexVersion:    1,
		Title:           title,
		Text:            text,
		Source:          "hybrid_rrf",
		Zone:            "original",
		ContentType:     "mod",
	}
}

// seedNumberingDB seeds two fully-indexed contents (100 chunk a×64, 200 chunk
// b×64). Ghost candidates on unknown chunk keys enter the pool but fail
// revalidation, which is exactly the drop-a-middle-slot shape FT-5 must keep.
func seedNumberingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := seedCitationContractDB(t)
	require.NoError(t, db.Exec(
		"INSERT INTO content_items (id, title, author_id, zone, content_type, status, is_public, allow_copy, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		200, "Second Public", 1, "original", "mod", "published", true, true, time.Now(), time.Now(),
	).Error)
	require.NoError(t, db.Create(&model.ContentVersion{
		ID: 80, ContentItemID: 200, AuthorID: 1, VersionNumber: 7,
		StorageType: "full", StorageKey: "snapshot", Status: "active", IsLatest: true,
	}).Error)
	require.NoError(t, db.Create(&model.RagChunk{
		ContentID: 200, ContentVersion: 7, ChunkIndex: 2, ChunkKey: strings.Repeat("b", 64),
		ChunkingVersion: 1, Heading: "Heading", Text: "excerpt of Second Public",
		SourceStart: 0, SourceEnd: 24, Zone: "original", ContentType: "mod", IndexVersion: 1,
	}).Error)
	require.NoError(t, db.Create(&model.IndexProjectionStatus{
		ContentID: 200, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "model",
		State: "ready", IsCurrent: true, ErrorSummary: "",
	}).Error)
	return db
}

func numberedCitationService(t *testing.T, db *gorm.DB, provider llm.LLMProvider, retriever AgentContentRetriever) *AgentService {
	t.Helper()
	svc := NewAgentService(
		provider,
		nil,
		nil,
		nil,
		db,
		&config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{
			WebAgentEnabled: true, MaxToolCallsPerTurn: 6, CitationMaxCount: 5,
			MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
		}},
	)
	svc.hybridRetriever = retriever
	return svc
}

// toolSearchSummaries decodes the search arrays the model received across all
// tool messages of the turn, in order.
func toolSearchSummaries(t *testing.T, req llm.ChatRequest) [][]ContentSummary {
	t.Helper()
	var out [][]ContentSummary
	for _, msg := range req.Messages {
		if msg.Role != "tool" {
			continue
		}
		var decoded struct {
			Search []ContentSummary `json:"search"`
		}
		if err := json.Unmarshal([]byte(msg.Content), &decoded); err != nil || decoded.Search == nil {
			continue
		}
		out = append(out, decoded.Search)
	}
	return out
}

func TestTurnGlobalCitationNumbersAccumulateAcrossSearchCalls(t *testing.T) {
	db := seedNumberingDB(t)
	retriever := &queuedRetriever{results: []AgentRetrievalResult{
		{Candidates: []AgentRetrievalCandidate{
			numberedCandidate(strings.Repeat("a", 64), "A server-owned excerpt.", 100, "Published Public"),
			numberedCandidate(strings.Repeat("c", 64), "excerpt of Ghost", 300, "Ghost"),
		}},
		{Candidates: []AgentRetrievalCandidate{
			numberedCandidate(strings.Repeat("b", 64), "excerpt of Second Public", 200, "Second Public"),
			numberedCandidate(strings.Repeat("a", 64), "A server-owned excerpt.", 100, "Published Public"),
		}},
	}}
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchContent, `{"query":"q1"}`)},
		{toolCallDelta(ToolSearchContent, `{"query":"q2"}`)},
		{{Content: "第一篇 [1] 幽灵 [2] 第二篇 [3]"}, {Done: true}},
	}}
	svc := numberedCitationService(t, db, provider, retriever)

	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "find"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch},
		func(event AgentStreamEvent) error { events = append(events, event); return nil })
	require.NoError(t, err)

	// Tool outputs: first call numbers its citable results 1..2, the second
	// call continues at 3 and the re-seen first result keeps its original 1.
	searches := toolSearchSummaries(t, provider.lastReq)
	require.Len(t, searches, 2, "two search calls must leave two tool results")
	require.Equal(t, 1, searches[0][0].Cite)
	require.Equal(t, 2, searches[0][1].Cite)
	require.Equal(t, 3, searches[1][0].Cite)
	require.Equal(t, 1, searches[1][1].Cite, "re-seen result keeps its original number")

	// Emitted citations: the ghost (number 2) is dropped by revalidation and
	// the survivors keep their pool numbers — no compression, gap stays.
	var done *AgentStreamEvent
	for i := range events {
		if events[i].Type == AgentEventDone {
			done = &events[i]
		}
	}
	require.NotNil(t, done)
	require.Len(t, done.Citations, 2)
	require.Equal(t, 1, done.Citations[0].Number)
	require.Equal(t, int64(100), done.Citations[0].ContentID)
	require.Equal(t, 3, done.Citations[1].Number)
	require.Equal(t, int64(200), done.Citations[1].ContentID)

	// The dropped number's [2] marker is stripped; [1] and [3] survive so
	// each remaining marker still points at the card it was written for.
	require.Equal(t, "第一篇 [1] 幽灵  第二篇 [3]", done.Answer)
}

func TestIPCitationsJoinTurnGlobalNumbering(t *testing.T) {
	db := seedCitationContractDB(t)
	seededIP := model.IP{
		Name: "星轨文库", Slug: "xinggui", Category: "novel", Status: "approved",
		Description: "一个连续的幻想世界。",
	}
	require.NoError(t, db.Create(&seededIP).Error)
	retriever := &queuedRetriever{results: []AgentRetrievalResult{
		{Candidates: []AgentRetrievalCandidate{numberedCandidate(strings.Repeat("a", 64), "A server-owned excerpt.", 100, "Published Public")}},
	}}
	provider := &streamToolProvider{rounds: [][]llm.ChatDelta{
		{toolCallDelta(ToolSearchIPs, `{"query":"星轨"}`)},
		{toolCallDelta(ToolSearchContent, `{"query":"星轨"}`)},
		{{Content: "IP [1] 内容 [2]"}, {Done: true}},
	}}
	svc := numberedCitationService(t, db, provider, retriever)
	svc.ipSearch = func(_ context.Context, _, _ string, _ int) ([]model.IP, error) {
		return []model.IP{{ID: seededIP.ID, Name: "星轨文库", Slug: "xinggui", Category: "novel", Status: "approved"}}, nil
	}

	var events []AgentStreamEvent
	err := svc.ChatStream(context.Background(), 3, ChatTurnInput{Message: "星轨"},
		&ResolvedChatContext{Surface: model.AgentChatSurfaceSearch},
		func(event AgentStreamEvent) error { events = append(events, event); return nil })
	require.NoError(t, err)

	var done *AgentStreamEvent
	for i := range events {
		if events[i].Type == AgentEventDone {
			done = &events[i]
		}
	}
	require.NotNil(t, done)
	require.Len(t, done.Citations, 2)
	require.Equal(t, 1, done.Citations[0].Number)
	require.Equal(t, "ip", done.Citations[0].Zone)
	require.Equal(t, "星轨文库", done.Citations[0].Title)
	require.Equal(t, 2, done.Citations[1].Number)
	require.Equal(t, int64(100), done.Citations[1].ContentID)
	require.Equal(t, "IP [1] 内容 [2]", done.Answer)
}

// citationKeptNumbers fallback: citations without pool numbers (positional
// legacy paths) degrade to 1..n instead of empty.
func TestCitationKeptNumbersFallsBackToPositional(t *testing.T) {
	got := citationKeptNumbers([]AgentCitation{{ContentID: 1}, {ContentID: 2, Number: 5}, {ContentID: 3}})
	require.Equal(t, []int{1, 5, 3}, got)
}
