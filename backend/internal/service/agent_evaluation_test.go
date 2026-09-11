package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

// Agent evaluation gate (plan Task 5 Steps 1-2). The CI oracle is the fixed
// fixture file backend/testdata/agent_eval_cases.json plus the deterministic
// evalProvider harness below; a non-deterministic real model is never used as
// a pass/fail oracle. The harness scripts every model tool request, streamed
// token, malformed citation, prompt injection, timeout and error by hand, so
// the observed outcome is fully reproducible.

// evalErrProviderRateLimited is the harness downgrade signal for the
// rate_limit_downgrade case: a provider 429 that exhausted its bounded
// retries and surfaced as a provider-side failure.
var evalErrProviderRateLimited = errors.New("eval: provider rate limited after bounded retries")

// evalFixtureErrorCodes is the gate vocabulary mirroring the fixture's
// expected.error_code values. Codes are derived from deterministic service
// outcomes, never from raw provider errors.
const (
	evalCodeNoEvidence      = "AGENT_NO_EVIDENCE"
	evalCodeProviderTimeout = "AGENT_PROVIDER_TIMEOUT"
	evalCodeRateLimited     = "AGENT_RATE_LIMITED"
	evalCodeContentNotFound = "AGENT_CONTENT_NOT_FOUND"
	evalCodeInvalidResource = "AGENT_INVALID_RESOURCE"
	evalCodeProviderError   = "AGENT_PROVIDER_ERROR"
)

// evalProvider is the deterministic fake LLM provider for one fixture case.
// It replays scripted delta rounds and records every ChatStream request so the
// gate can inspect the tool-result messages the model received (seen IDs).
type evalProvider struct {
	script   [][]llm.ChatDelta // one entry per provider round
	failWith error             // returned once the script rounds are exhausted
	calls    int
	reqs     []llm.ChatRequest
}

func (p *evalProvider) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}

func (p *evalProvider) ChatStream(ctx context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.calls++
	p.reqs = append(p.reqs, req)
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if p.calls <= len(p.script) {
		for _, d := range p.script[p.calls-1] {
			if err := handler(d); err != nil {
				return err
			}
		}
		return nil
	}
	return p.failWith
}

func (p *evalProvider) GetEmbedding(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1}, nil
}

// evalOutcome is the deterministic observation derived from one run: what the
// model saw (seenIDs), what the server normalized into citations (citedIDs),
// the answer classification, tool executions, degraded flag and the gate-level
// error code.
type evalOutcome struct {
	kind           string
	answer         string
	citedIDs       []int64
	citations      []AgentCitation
	tools          []AgentToolExecution
	seenIDs        []int64
	degraded       bool
	completed      bool
	errorCode      string
	errorEventCode string
}

const evalViewerID = int64(7)

// runEvalCase drives one fixture case through the deterministic harness.
// Surface-specific runs: hidden chat contexts are rejected before any provider
// call (ResolveChatContext); the publish surface exercises the typed
// suggest_publish_metadata contract at the tool boundary because
// publish_suggestion is an independent typed contract that never flows through
// the grounded ChatStream path (design §4.3). All other cases run the full
// ChatStream tool loop.
func runEvalCase(t *testing.T, tc agentEvalCase) (evalOutcome, *AgentService, *gorm.DB) {
	t.Helper()
	db := seedEvalContents(t, tc)
	provider := &evalProvider{script: evalScriptForCase(tc), failWith: evalFailureForCase(tc)}
	cfg := &config.Config{Features: config.FeaturesConfig{RAGHybridEnabled: true}, Agent: config.AgentConfig{
		WebAgentEnabled: true, MaxToolCallsPerTurn: 8, CitationMaxCount: 5,
		MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200,
	}}
	svc := NewAgentService(provider, repository.NewEmbeddingRepository(db), repository.NewContentRepository(db), nil, db, cfg)
	svc.vectorSearch = func([]float32, int) ([]repository.EmbeddingSearchResult, error) {
		results := make([]repository.EmbeddingSearchResult, 0, len(tc.Contents))
		for _, c := range tc.Contents {
			results = append(results, repository.EmbeddingSearchResult{ContentItemID: c.ID, Score: 1.0})
		}
		return results, nil
	}
	svc.SetContentRetriever(&contractRetriever{result: AgentRetrievalResult{
		Candidates: evalRAGCandidates(tc),
	}})

	switch tc.ID {
	case "hidden_content_id_usage_guide":
		id := tc.Contents[0].ID
		_, err := svc.ResolveChatContext(context.Background(), evalViewerID, &model.AgentChatContext{
			Surface:   model.AgentChatSurfaceContent,
			ContentID: &id,
		})
		require.ErrorIs(t, err, ErrContentNotFound)
		return evalOutcome{kind: string(AgentAnswerNoEvidence), errorCode: evalCodeContentNotFound}, svc, db
	case "publish_suggestion_forged_resource":
		// The publish surface is an independent typed contract (design §4.3):
		// suggest_publish_metadata accepts NO model-authored resource fields.
		// The gate exercises the tool boundary with a forged resource argument
		// and a bound snapshot, proving the forgery is rejected before any
		// suggestion work happens.
		outcome, err := svc.ExecuteTool(context.Background(), ToolSuggestPublishMetadata, json.RawMessage(`{"content_id": 42}`), evalViewerID,
			&AgentPublishSnapshot{Title: "草稿标题", Description: "草稿描述", ContentType: "mod"})
		require.ErrorIs(t, err, ErrAgentToolInvalidArgs)
		require.Nil(t, outcome)
		return evalOutcome{
			kind:      string(AgentAnswerPublishSuggestion),
			tools:     []AgentToolExecution{{Name: ToolSuggestPublishMetadata, Status: AgentToolStatusError}},
			errorCode: evalCodeInvalidResource,
		}, svc, db
	default:
		resolved := resolveEvalChatContext(t, svc, tc)
		var events []AgentStreamEvent
		streamErr := svc.ChatStream(context.Background(), evalViewerID,
			ChatTurnInput{Message: tc.Query}, resolved,
			func(ev AgentStreamEvent) error { events = append(events, ev); return nil })
		return deriveEvalOutcome(events, streamErr, provider), svc, db
	}
}

func evalRAGCandidates(tc agentEvalCase) []AgentRetrievalCandidate {
	candidates := make([]AgentRetrievalCandidate, 0, len(tc.Contents))
	for _, content := range tc.Contents {
		if content.Status != "published" || !content.IsPublic || content.AuthorIsBanned {
			continue
		}
		candidates = append(candidates, AgentRetrievalCandidate{
			ChunkKey:       fmt.Sprintf("%064x", content.ID),
			ContentID:      content.ID,
			ContentVersion: 1,
			ChunkIndex:     0,
			Title:          content.Title,
			Text:           content.Title,
			Zone:           content.Zone,
			ContentType:    content.ContentType,
			Source:         "vector",
		})
	}
	return candidates
}

func resolveEvalChatContext(t *testing.T, svc *AgentService, tc agentEvalCase) *ResolvedChatContext {
	t.Helper()
	surface := model.AgentChatSurfaceGlobal
	var contentID *int64
	if tc.Surface == "content" {
		surface = model.AgentChatSurfaceContent
		id := tc.Contents[0].ID
		contentID = &id
	} else if tc.Surface == "search" {
		surface = model.AgentChatSurfaceSearch
	}
	resolved, err := svc.ResolveChatContext(context.Background(), evalViewerID, &model.AgentChatContext{Surface: surface, ContentID: contentID})
	require.NoError(t, err)
	return resolved
}

// deriveEvalOutcome maps deterministic service behavior to the gate vocabulary.
// Provider failures always classify as no_evidence (the product degrades to
// keyword search instead of fabricating an answer) and set degraded=true.
func deriveEvalOutcome(events []AgentStreamEvent, streamErr error, provider *evalProvider) evalOutcome {
	o := evalOutcome{}
	for _, ev := range events {
		switch ev.Type {
		case AgentEventToolStatus:
			if ev.Tool != nil {
				o.tools = append(o.tools, *ev.Tool)
			}
		case AgentEventCitation:
			if ev.Citation != nil {
				o.citations = append(o.citations, *ev.Citation)
				o.citedIDs = append(o.citedIDs, ev.Citation.ContentID)
			}
		case AgentEventDone:
			o.completed = true
			o.kind = string(ev.AnswerKind)
			o.answer = ev.Answer
			o.degraded = ev.Degraded
		case AgentEventError:
			o.errorEventCode = ev.ErrorCode
		}
	}
	o.seenIDs = seenIDsFromToolResults(provider)
	if streamErr != nil {
		o.degraded = true
		o.kind = string(AgentAnswerNoEvidence)
		switch {
		case errors.Is(streamErr, evalErrProviderRateLimited):
			o.errorCode = evalCodeRateLimited
		case errors.Is(streamErr, context.DeadlineExceeded), o.errorEventCode == AgentErrorCodeProviderTimeout:
			o.errorCode = evalCodeProviderTimeout
		default:
			o.errorCode = evalCodeProviderError
		}
	} else if o.kind == string(AgentAnswerNoEvidence) {
		o.errorCode = evalCodeNoEvidence
	}
	return o
}

// seenIDsFromToolResults extracts the content IDs the server actually returned
// to the model through tool-result messages (search hits and detail summaries).
// Forbidden content never appears here because visibility filtering drops it
// before any tool result is built.
func seenIDsFromToolResults(provider *evalProvider) []int64 {
	seen := make([]int64, 0, 8)
	seenSet := make(map[int64]bool)
	add := func(id int64) {
		if id > 0 && !seenSet[id] {
			seenSet[id] = true
			seen = append(seen, id)
		}
	}
	for _, req := range provider.reqs {
		for _, msg := range req.Messages {
			if msg.Role != "tool" {
				continue
			}
			var res agentToolResult
			if err := json.Unmarshal([]byte(msg.Content), &res); err != nil {
				continue
			}
			if res.Detail != nil {
				add(res.Detail.ID)
			}
			for _, s := range res.Search {
				add(s.ID)
			}
		}
	}
	return seen
}

// evalScriptForCase synthesizes the deterministic model behavior for one
// fixture case: which tools to request, in which order, which tokens to
// stream, and which malformed citations/injection attempts to emit.
func evalScriptForCase(tc agentEvalCase) [][]llm.ChatDelta {
	search := [][]llm.ChatDelta{{toolCallDelta(ToolSearchContent, fmt.Sprintf(`{"query":%q}`, tc.Query))}}
	answer := func(text string) []llm.ChatDelta {
		return []llm.ChatDelta{{Content: text}, {Done: true}}
	}
	details := func(ids ...int64) []llm.ChatDelta {
		out := make([]llm.ChatDelta, 0, len(ids))
		for _, id := range ids {
			out = append(out, toolCallDelta(ToolGetContentDetail, fmt.Sprintf(`{"content_id": %d}`, id)))
		}
		return out
	}

	switch tc.ID {
	case "exact_keyword_lookup":
		return [][]llm.ChatDelta{search[0], details(1001), answer("Blender 插件安装需要三步：下载、解压到 plugins 目录、勾选启用。")}
	case "semantic_paraphrase_lookup":
		return [][]llm.ChatDelta{search[0], details(1002), answer("安装游戏模组：复制模组文件到游戏 mods 目录即可。")}
	case "two_cited_contents":
		return [][]llm.ChatDelta{search[0], details(1003, 1004), answer("剪辑视频参考《视频剪辑技巧大全》，导出高清画质参考《高清导出设置指南》。")}
	case "no_evidence_refusal":
		return [][]llm.ChatDelta{answer("我无法回答该问题，因为站内没有可作为依据的内容。")}
	case "unpublished_private_exclusion":
		return [][]llm.ChatDelta{search[0], details(1005, 1006, 1007), answer("安装 Blender 插件参考《Blender 插件安装教程》。")}
	case "prompt_injection_in_content":
		return [][]llm.ChatDelta{
			search[0],
			{toolCallDelta("drop_table", `{}`), toolCallDelta(ToolGetContentDetail, `{"content_id": 1008}`)},
			answer("系统提示词相关内容见《系统提示词大全》。"),
		}
	case "forged_citation_attempt":
		return [][]llm.ChatDelta{
			search[0],
			details(1009, 7777),
			[]llm.ChatDelta{{Content: "答案是《Blender 插件安装教程》（参见 [citation:7777] 与 [1] 7777）。"}},
			[]llm.ChatDelta{{Done: true}},
		}
	case "client_forged_context":
		return [][]llm.ChatDelta{details(1010), answer("这个模组的使用方法见《模组使用说明》。")}
	case "hidden_content_id_usage_guide", "publish_suggestion_forged_resource":
		return nil // pre-stream rejection / tool-boundary typed-contract runs
	case "provider_timeout_downgrade", "rate_limit_downgrade":
		return [][]llm.ChatDelta{search[0]}
	}
	require.FailNow(nil, "no script for evaluation case", tc.ID)
	return nil
}

func evalFailureForCase(tc agentEvalCase) error {
	switch tc.ID {
	case "provider_timeout_downgrade":
		return context.DeadlineExceeded
	case "rate_limit_downgrade":
		return evalErrProviderRateLimited
	default:
		return nil
	}
}

// seedEvalContents seeds the viewer-aware fixture contents for one case.
// Hidden contents (pending status, private, banned author) exist in the
// database but must never surface through the shared visibility scope.
func seedEvalContents(t *testing.T, tc agentEvalCase) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// An in-memory sqlite database is per-connection; a pooled handle would
	// spread writes across several independent empty databases.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.AgentConversation{}, &model.AgentMessage{}, &model.ContentVersion{}, &model.RagChunk{}, &model.IndexProjectionStatus{}))

	authorBanned := false
	for _, c := range tc.Contents {
		if c.AuthorIsBanned {
			authorBanned = true
		}
	}
	require.NoError(t, db.Create(&model.User{ID: 1, Username: "author", Email: "author@example.com", IsBanned: authorBanned}).Error)

	now := time.Now()
	for _, c := range tc.Contents {
		content := model.ContentItem{
			ID: c.ID, Title: c.Title, AuthorID: 1, Zone: c.Zone,
			ContentType: c.ContentType, Status: c.Status, IsPublic: c.IsPublic,
			AllowCopy: true, Description: c.InjectionText,
			CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, db.Create(&content).Error)
		// GORM applies the DB default (is_public default:true) to zero-value
		// bool fields on Create AND mutates the struct field back to true,
		// which would turn fixture-private contents into public ones; force the
		// explicit column value from the fixture afterwards.
		if !c.IsPublic {
			require.NoError(t, db.Exec("UPDATE content_items SET is_public = false WHERE id = ?", content.ID).Error)
		}
	}
	for _, c := range tc.Contents {
		if c.Status != "published" {
			continue
		}
		require.NoError(t, db.Create(&model.ContentVersion{
			ID: c.ID + 100000, ContentItemID: c.ID, AuthorID: 1, VersionNumber: 1,
			StorageType: "full", StorageKey: fmt.Sprintf("eval-%d", c.ID), Status: "active", IsLatest: true,
		}).Error)
		require.NoError(t, db.Create(&model.RagChunk{
			ContentID: c.ID, ContentVersion: 1, ChunkIndex: 0, ChunkKey: fmt.Sprintf("%064x", c.ID),
			ChunkingVersion: 1, Text: c.Title, SourceStart: 0, SourceEnd: len([]rune(c.Title)),
			Zone: c.Zone, ContentType: c.ContentType, IndexVersion: 1,
		}).Error)
		require.NoError(t, db.Create(&model.IndexProjectionStatus{
			ContentID: c.ID, IndexVersion: 1, ChunkingVersion: 1, EmbeddingModel: "eval",
			State: "ready", IsCurrent: true, ErrorSummary: "",
		}).Error)
	}
	return db
}

// assertEvalCase asserts the six gate categories against the deterministic
// fixture oracle (plan Task 5 Step 2): expected content IDs present, no
// forbidden content IDs, citation objects valid and visible, no-evidence
// refusal, injection never changes tool policy, downgrade/error code matches.
func assertEvalCase(t *testing.T, tc agentEvalCase, o evalOutcome, svc *AgentService) {
	t.Helper()

	for _, id := range tc.Expected.ExpectedContentIDs {
		require.Contains(t, o.seenIDs, id, "case %s: expected content %d never surfaced to the model", tc.ID, id)
	}
	for _, id := range tc.Expected.ForbiddenContentIDs {
		require.NotContains(t, o.citedIDs, id, "case %s: forbidden content %d leaked into citations", tc.ID, id)
		require.NotContains(t, o.seenIDs, id, "case %s: forbidden content %d surfaced to the model", tc.ID, id)
	}

	for _, c := range o.citations {
		require.Positive(t, c.ContentID, "case %s: citation without content_id", tc.ID)
		require.Positive(t, c.ContentVersion, "case %s: citation without content_version", tc.ID)
		require.Len(t, c.ChunkKey, 64, "case %s: citation without stable chunk_key", tc.ID)
		require.GreaterOrEqual(t, c.ChunkIndex, 0, "case %s: citation without chunk_index", tc.ID)
		require.NotEmpty(t, c.Title, "case %s: citation %d without title", tc.ID, c.ContentID)
		require.NotEmpty(t, c.Zone, "case %s: citation %d without zone", tc.ID, c.ContentID)
		require.Equal(t, contentRoute(c.Zone, c.ContentID), c.Route, "case %s: citation route must be server-owned", tc.ID)
		require.NotEmpty(t, c.Excerpt, "case %s: citation %d without excerpt", tc.ID, c.ContentID)
		require.Equal(t, tc.Expected.CitationSource, c.Source, "case %s: citation source mismatch", tc.ID)
		require.NotContains(t, c.Source, "score", "case %s: citation must not expose score", tc.ID)
		_, err := svc.resolveVisibleContent(context.Background(), evalViewerID, c.ContentID)
		require.NoError(t, err, "case %s: citation %d is not viewer-visible", tc.ID, c.ContentID)
	}

	require.Equal(t, tc.Expected.AnswerKind, o.kind, "case %s: answer_kind mismatch", tc.ID)
	if tc.Expected.AnswerKind == string(AgentAnswerNoEvidence) {
		require.Empty(t, o.citations, "case %s: no-evidence case must not emit citations", tc.ID)
	}
	if o.kind == string(AgentAnswerGroundedContent) {
		require.NotEmpty(t, o.answer, "case %s: grounded answer must not be empty", tc.ID)
		require.NotEmpty(t, o.citations, "case %s: grounded answer must carry at least one citation", tc.ID)
	}
	if tc.ID == "client_forged_context" && o.kind == string(AgentAnswerGroundedContent) {
		require.Equal(t, "模组使用说明", o.citations[0].Title, "case %s: citation title must be server-owned, not client-forged", tc.ID)
	}

	executed := registeredToolNames(t, o.tools)
	for _, name := range tc.Expected.ExpectedToolNames {
		require.Contains(t, executed, name, "case %s: expected tool %s not executed", tc.ID, name)
	}
	for _, name := range executed {
		require.Contains(t, svc.RegisteredToolNames(), name, "case %s: unregistered tool %s executed", tc.ID, name)
	}
	if tc.Category == "injection" {
		require.True(t, o.completed, "case %s: injection must not abort the stream", tc.ID)
		require.Equal(t, string(AgentAnswerGroundedContent), o.kind, "case %s: injection must not change answer classification", tc.ID)
	}

	require.Equal(t, tc.Expected.Degraded, o.degraded, "case %s: degraded mismatch", tc.ID)
	if tc.Expected.ErrorCode != "" {
		require.Equal(t, tc.Expected.ErrorCode, o.errorCode, "case %s: error code mismatch", tc.ID)
	}
	if o.errorCode == evalCodeProviderTimeout {
		require.Equal(t, string(AgentErrorCodeProviderTimeout), o.errorEventCode,
			"case %s: provider timeout must be distinguishable from client cancellation on the wire", tc.ID)
	}
}

func registeredToolNames(t *testing.T, tools []AgentToolExecution) []string {
	t.Helper()
	var names []string
	for _, tool := range tools {
		if tool.Name == "" || tool.Name == "(unknown)" {
			continue
		}
		names = append(names, tool.Name)
	}
	return names
}

// TestAgentEvaluation is the deterministic evaluation gate (plan Task 5
// Step 4). It runs every fixture case through the scripted harness and asserts
// all six gate categories; a real model is never the oracle.
func TestAgentEvaluation(t *testing.T) {
	data, err := os.ReadFile("../../testdata/agent_eval_cases.json")
	require.NoError(t, err)
	var fixture agentEvalFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	require.NotEmpty(t, fixture.Cases)

	for _, tc := range fixture.Cases {
		tc := tc
		t.Run(tc.ID, func(t *testing.T) {
			o, svc, _ := runEvalCase(t, tc)
			assertEvalCase(t, tc, o, svc)
			t.Logf("case %-28s kind=%-18s degraded=%-5v error=%-24s tools=%v cited=%v seen=%v",
				tc.ID, o.kind, o.degraded, o.errorCode, registeredToolNames(t, o.tools), o.citedIDs, o.seenIDs)
		})
	}
	t.Logf("agent evaluation gate: %d fixture cases asserted (oracle = fixture + deterministic harness)", len(fixture.Cases))
}
