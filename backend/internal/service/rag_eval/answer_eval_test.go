package rageval

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
	service "omnicraft/backend/internal/service"
	ragservice "omnicraft/backend/internal/service/rag"
)

// The answer eval is the answer-producing counterpart of
// TestHybridGoldenSetEval: instead of only scoring retrieval, it runs each
// golden case through the real AgentService tool loop (real chat provider,
// real query embeddings) and measures first-token latency, token usage and
// the deterministic groundedness/answer-relevance proxies on the actual
// answers. Gates:
//
//	OMNICRAFT_AGENT_ANSWER_EVAL=1          enable the run (never set in CI)
//	OMNICRAFT_RAG_EMBEDDING_MODE=real      require real provider embeddings
//	OMNICRAFT_AGENT_ANSWER_REPORT=<path>   redacted artifact output path
//	OMNICRAFT_AGENT_ANSWER_GIT_COMMIT      optional run identity
//	OMNICRAFT_RAG_OPENSEARCH_URL           optional OpenSearch keyword source
//
// The conversation persistence writes to the evaluation database; the run
// must therefore target the isolated eval database (OMNICRAFT_RAG_EVAL_DSN).
const (
	answerEvalEnv         = "OMNICRAFT_AGENT_ANSWER_EVAL"
	answerEvalReportEnv   = "OMNICRAFT_AGENT_ANSWER_REPORT"
	answerEvalCommitEnv   = "OMNICRAFT_AGENT_ANSWER_GIT_COMMIT"
	answerEvalFilterEnv   = "OMNICRAFT_AGENT_ANSWER_CASES"
	answerEvalUserIDEnv   = "OMNICRAFT_AGENT_ANSWER_USER_ID"
	answerEvalAttemptsEnv = "OMNICRAFT_AGENT_ANSWER_MAX_ATTEMPTS"
	answerEvalCaseTimeot  = 90 * time.Second
	minAnsweredShare      = 0.5
)

// firstTokenMeasuringProvider wraps the real provider and records the latency
// of the first streamed content delta. The AgentService buffers tool-loop
// rounds and emits one final answer delta, so the provider seam is the only
// honest place to observe first-token latency.
type firstTokenMeasuringProvider struct {
	llm.LLMProvider
	mu           sync.Mutex
	startedAt    time.Time
	firstTokenMs int64
}

func (p *firstTokenMeasuringProvider) ChatStream(ctx context.Context, req llm.ChatRequest, handler func(delta llm.ChatDelta) error) error {
	p.mu.Lock()
	p.startedAt = time.Now()
	p.firstTokenMs = 0
	p.mu.Unlock()
	return p.LLMProvider.ChatStream(ctx, req, func(delta llm.ChatDelta) error {
		if delta.Content != "" {
			p.mu.Lock()
			if p.firstTokenMs == 0 {
				p.firstTokenMs = time.Since(p.startedAt).Milliseconds()
			}
			p.mu.Unlock()
		}
		return handler(delta)
	})
}

func (p *firstTokenMeasuringProvider) firstTokenLatencyMs() (int64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.firstTokenMs, p.firstTokenMs > 0
}

// answerEvalRAGAdapter maps the hybrid retriever contract onto the
// AgentService boundary. It mirrors the container's agentRAGRetriever and
// records the retrieved content ids so the visibility leak surface 1 and the
// no-answer judge's recommendation-truth check see the real retrieval ids.
type answerEvalRAGAdapter struct {
	retriever    *ragservice.HybridRetriever
	retrievedIDs []int64
	seenIDs      map[int64]bool
}

func (r *answerEvalRAGAdapter) Retrieve(ctx context.Context, query string, viewerID int64) (service.AgentRetrievalResult, error) {
	result, err := r.retriever.Retrieve(ctx, query, viewerID)
	if err != nil {
		return service.AgentRetrievalResult{}, err
	}
	if r.seenIDs == nil {
		r.seenIDs = map[int64]bool{}
	}
	candidates := make([]service.AgentRetrievalCandidate, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		if !r.seenIDs[candidate.ContentID] {
			r.seenIDs[candidate.ContentID] = true
			r.retrievedIDs = append(r.retrievedIDs, candidate.ContentID)
		}
		candidates = append(candidates, service.AgentRetrievalCandidate{
			ChunkKey:        candidate.ChunkKey,
			ContentID:       candidate.ContentID,
			ContentVersion:  candidate.ContentVersion,
			ChunkIndex:      candidate.ChunkIndex,
			ChunkingVersion: candidate.ChunkingVersion,
			IndexVersion:    candidate.IndexVersion,
			Title:           candidate.Title,
			Heading:         candidate.Heading,
			Text:            candidate.Text,
			Zone:            candidate.Zone,
			ContentType:     candidate.ContentType,
			Source:          candidate.Source,
		})
	}
	return service.AgentRetrievalResult{Candidates: candidates, Degraded: result.Degraded}, nil
}

// answerEvalStreamCollector captures the typed stream events the runner needs.
// Events arrive sequentially from one ChatStream call, so plain fields suffice.
type answerEvalStreamCollector struct {
	answer         string
	citations      []service.AgentCitation
	usage          *service.AgentUsage
	kind           service.AgentAnswerKind
	degraded       bool
	degradedReason string
	errorCode      string
	toolCalls      int
	toolSteps      []ToolStepRecord
}

func (c *answerEvalStreamCollector) handle(ev service.AgentStreamEvent) error {
	switch ev.Type {
	case service.AgentEventDelta:
		if ev.Delta != "" {
			c.answer = ev.Delta
		}
	case service.AgentEventCitation:
		if ev.Citation != nil {
			c.citations = append(c.citations, *ev.Citation)
		}
	case service.AgentEventUsage:
		if ev.Usage != nil {
			c.usage = ev.Usage
		}
	case service.AgentEventToolStatus:
		c.toolCalls++
		if ev.Tool != nil {
			c.toolSteps = append(c.toolSteps, ToolStepRecord{Name: ev.Tool.Name, ArgsSummary: ev.Tool.ArgsSummary})
		}
	case service.AgentEventDone:
		c.kind = ev.AnswerKind
		c.degraded = ev.Degraded
		c.degradedReason = ev.DegradedReason
		if ev.Usage != nil {
			c.usage = ev.Usage
		}
		if ev.Answer != "" {
			c.answer = ev.Answer
		}
		if len(ev.Citations) > 0 {
			c.citations = ev.Citations
		}
	case service.AgentEventError:
		c.errorCode = ev.ErrorCode
	}
	return nil
}

func TestAgentAnswerEval(t *testing.T) {
	if os.Getenv(answerEvalEnv) != "1" {
		t.Skip("answer eval runs only with OMNICRAFT_AGENT_ANSWER_EVAL=1 (real provider, real cost)")
	}
	embeddingMode := strings.TrimSpace(os.Getenv(hybridEmbeddingModeEnv))
	if embeddingMode != "real" {
		t.Fatalf("%s must be real: answers and query embeddings must come from the real provider", hybridEmbeddingModeEnv)
	}
	reportPath := strings.TrimSpace(os.Getenv(answerEvalReportEnv))
	if reportPath == "" {
		t.Fatalf("%s is required (redacted artifact path)", answerEvalReportEnv)
	}

	ctx := context.Background()
	db := openEvalDB(t)
	evalRepo := repository.NewRagEvaluationRepository(db)
	cases, err := evalRepo.ListActiveGoldenCases(ctx)
	if err != nil {
		t.Fatalf("list active golden cases: %v", err)
	}
	if len(cases) < minFullSetCases {
		t.Fatalf("golden set has %d cases, want >= %d", len(cases), minFullSetCases)
	}
	// Optional case-key filter for a bounded smoke run; the filter is part of
	// the run identity because it changes the dataset the summary describes.
	caseFilter := map[string]bool{}
	filtered := false
	if rawFilter := strings.TrimSpace(os.Getenv(answerEvalFilterEnv)); rawFilter != "" {
		filtered = true
		for _, key := range strings.Split(rawFilter, ",") {
			if key = strings.TrimSpace(key); key != "" {
				caseFilter[key] = true
			}
		}
		kept := cases[:0]
		for _, c := range cases {
			if caseFilter[c.CaseKey] {
				kept = append(kept, c)
			}
		}
		if len(kept) == 0 {
			t.Fatalf("case filter %q matched none of %d cases", rawFilter, len(cases))
		}
		cases = kept
	}

	evalCfg := loadHybridEvalConfig(t)
	if strings.TrimSpace(evalCfg.Agent.LLMAPIKey) == "" {
		t.Fatalf("real answer eval requires AGENT_LLM_API_KEY (source .env before running)")
	}
	if evalCfg.Agent.EmbeddingDimensions != config.RAGEmbeddingDimensions {
		t.Fatalf("agent embedding dimensions=%d, want %d", evalCfg.Agent.EmbeddingDimensions, config.RAGEmbeddingDimensions)
	}
	// The runner's purpose is real answers over the frozen hybrid corpus; both
	// switches are forced here and disclosed in the artifact note.
	evalCfg.Agent.WebAgentEnabled = true
	evalCfg.Features.RAGHybridEnabled = true

	embeddingModel := evalCfg.RAG.Index.EmbeddingModel
	provider := llm.NewProvider(&evalCfg)
	searchRepo := repository.NewSearchRepository(db)
	embeddingRepo := repository.NewEmbeddingRepository(db)

	keywordSourceName := "postgres-keyword"
	var keywordFallback ragservice.KeywordRetriever
	if openSearchURL := strings.TrimSpace(os.Getenv(hybridOpenSearchEnv)); openSearchURL != "" {
		openSearch := repository.NewOpenSearchRepositoryWithLimits(openSearchURL, &http.Client{Timeout: 2 * time.Second}, repository.OpenSearchResponseLimits{})
		keywordFallback = ragservice.NewOpenSearchKeywordRetriever(openSearch)
		keywordSourceName = "postgres-keyword+opensearch-fallback"
	}
	retriever := ragservice.NewHybridRetriever(
		ragservice.NewPostgresKeywordRetriever(searchRepo),
		keywordFallback,
		ragservice.NewPostgresVectorRetriever(embeddingRepo, embeddingModel),
		provider,
		ragservice.NewDatabaseVisibilityFilter(db),
		evalCfg.RAG.Hybrid,
	)

	projectionGeneration := currentProjectionGeneration(t, db)
	corpusEmbeddings := countChunkEmbeddings(t, db, embeddingModel)
	if corpusEmbeddings <= 0 {
		t.Fatalf("current corpus has no chunk embeddings for model %q; rebuild the frozen generation before running", embeddingModel)
	}
	var corpusContents int64
	if err := db.WithContext(ctx).Table("content_items").Where("status = ? AND deleted_at IS NULL", "published").Count(&corpusContents).Error; err != nil {
		t.Fatalf("count published content: %v", err)
	}
	datasetChecksum := hybridDatasetChecksum(t, cases)
	corpusChecksum := contentCorpusChecksum(t, db)
	chunkRepo := repository.NewRagChunkRepository(db)

	// H2: answer eval switches identity per case principal instead of one
	// global chat user. Anonymous principals (and legacy numeric viewer 0)
	// still chat as a seeded user owning no private corpus content — the
	// visibility scope equals the anonymous viewer because ChatStream
	// requires a users FK; fixture principals chat as the fixture account.
	anonChatUserID := resolveAnswerEvalChatUser(t, db)
	registry := &FixturePrincipalRegistry{DB: db}
	maxAttempts := 2
	if raw := strings.TrimSpace(os.Getenv(answerEvalAttemptsEnv)); raw != "" {
		parsed := 0
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err != nil || parsed < 1 || parsed > 5 {
			t.Fatalf("%s must be an integer in [1,5]", answerEvalAttemptsEnv)
		}
		maxAttempts = parsed
	}

	results := make([]AnswerEvalCaseResult, 0, len(cases))
	for _, c := range cases {
		vc, err := ParseViewerContext(c.ViewerContext)
		if err != nil {
			t.Fatalf("case %s viewer_context: %v", c.CaseKey, err)
		}
		cl, err := ParseClassificationV2(c.Classification)
		if err != nil {
			t.Fatalf("case %s classification: %v", c.CaseKey, err)
		}
		principalKeys := PrincipalKeysForCase(cl.PrimaryLayer, vc.PrincipalKey)
		for _, principalKey := range principalKeys {
			identity, err := ResolveViewerIdentity(ctx, ViewerContext{PrincipalKey: principalKey}, registry)
			if err != nil {
				t.Fatalf("case %s principal %q: %v", c.CaseKey, principalKey, err)
			}
			chatUserID := identity.ViewerUserID
			if identity.IsAnonymous {
				chatUserID = anonChatUserID
			}
			result := runAnswerEvalCase(ctx, db, chunkRepo, provider, retriever, searchRepo, &evalCfg, c, chatUserID, maxAttempts)
			result.PrincipalKey = identity.PrincipalKey
			result.PrimaryLayer = cl.PrimaryLayer
			result.Split = cl.Split
			result.NoAnswerStrategy = cl.NoAnswerStrategy
			applyAnswerLayerJudges(ctx, t, db, c, cl, &result, identity)
			results = append(results, result)
			t.Logf("case %s [principal=%s layer=%s]: status=%s kind=%s attempts=%d direct=%d first_token=%s total=%dms prompt=%s completion=%s groundedness=%s citations=%d evidence=%d",
				result.CaseKey, result.PrincipalKey, result.PrimaryLayer, result.Status, result.AnswerKind, result.Attempts, result.DirectAnswers, msPtrString(result.FirstTokenMs), result.TotalMs,
				intPtrString(result.PromptTokens), intPtrString(result.CompletionTokens), floatPtrString(result.Groundedness),
				len(result.Citations), result.EvidenceChunks)
		}
	}

	answered := countAnswerStatus(results, AnswerStatusAnswered)
	answeredShare := float64(answered) / float64(len(results))
	minShare := minAnsweredShare
	if filtered {
		// A filtered smoke run is a wiring check, not a summary artifact; only
		// require at least one answered case.
		minShare = 1.0 / float64(len(results))
	}
	if answeredShare < minShare {
		t.Fatalf("answered share %.2f (%d/%d) < %.2f; the run is not a valid answer-eval artifact",
			answeredShare, answered, len(results), minShare)
	}
	if filtered {
		meta := AnswerEvalMetadata{
			RanAt:               time.Now().UTC().Format(time.RFC3339),
			Provider:            evalCfg.Agent.LLMProvider,
			ChatModel:           evalCfg.Agent.LLMModel,
			EmbeddingModel:      embeddingModel,
			EmbeddingProvider:   evalCfg.Agent.EmbeddingProvider,
			ChatAPIBase:         evalCfg.Agent.LLMAPIBase,
			EmbeddingAPIBase:    evalCfg.Agent.EmbeddingAPIBase,
			EmbeddingDimensions: evalCfg.Agent.EmbeddingDimensions,
			FeatureFlags: map[string]bool{
				"hybrid":          evalCfg.Features.RAGHybridEnabled,
				"query_expansion": evalCfg.Features.RAGQueryExpansionEnabled,
				"rerank":          evalCfg.Features.RAGRerankEnabled,
			},
			Note: "filtered smoke run; not a summary artifact",
		}
		report := BuildAnswerEvalReport(meta, results)
		if err := WriteAnswerEvalReport(reportPath, report); err != nil {
			t.Fatalf("write answer eval smoke report: %v", err)
		}
		t.Logf("answer eval SMOKE report (filtered) written to %s", reportPath)
		return
	}

	providerName := evalCfg.Agent.LLMProvider
	if providerName == "" {
		providerName = "unknown"
	}
	meta := AnswerEvalMetadata{
		RanAt:               time.Now().UTC().Format(time.RFC3339),
		GitCommit:           strings.TrimSpace(os.Getenv(answerEvalCommitEnv)),
		Provider:            providerName,
		ChatModel:           evalCfg.Agent.LLMModel,
		EmbeddingModel:      embeddingModel,
		EmbeddingProvider:   evalCfg.Agent.EmbeddingProvider,
		ChatAPIBase:         evalCfg.Agent.LLMAPIBase,
		EmbeddingAPIBase:    evalCfg.Agent.EmbeddingAPIBase,
		EmbeddingDimensions: evalCfg.Agent.EmbeddingDimensions,
		FeatureFlags: map[string]bool{
			"hybrid":          evalCfg.Features.RAGHybridEnabled,
			"query_expansion": evalCfg.Features.RAGQueryExpansionEnabled,
			"rerank":          evalCfg.Features.RAGRerankEnabled,
		},
		QueryEmbeddingStandin: false,
		RetrieverVersion:      "hybrid-rrf-pg-fallback-v1",
		KeywordSource:         keywordSourceName,
		DatasetChecksum:       datasetChecksum,
		CorpusContentChecksum: corpusChecksum,
		CorpusContents:        corpusContents,
		CorpusEmbeddings:      corpusEmbeddings,
		ProjectionGeneration:  projectionGeneration,
		ChunkingVersion:       fmt.Sprintf("%d", evalCfg.RAG.Chunking.ChunkingVersion),
		IndexVersion:          fmt.Sprintf("%d", projectionGeneration),
		FinalTopK:             evalCfg.RAG.Hybrid.FinalTopK,
		GoVersion:             runtime.Version(),
		ChatViewerUserID:      anonChatUserID,
		PrincipalMode:         "per-case-v2",
		Note: "local answer-producing measurement over the frozen current-v1 corpus; " +
			"real provider chat and query embeddings; in-process AgentService tool loop " +
			"(not HTTP); chat identity switches per case principal (H2): anonymous " +
			"principals chat as a seeded user owning no private corpus content so the " +
			"retrieval scope equals the anonymous viewer, fixture principals chat as " +
			"the fixture account; groundedness is a deterministic verbatim-containment " +
			"proxy, not a production quality certificate",
	}
	report := BuildAnswerEvalReport(meta, results)
	if err := WriteAnswerEvalReport(reportPath, report); err != nil {
		t.Fatalf("write answer eval report: %v", err)
	}
	t.Logf("answer eval report written to %s: answered=%d no_evidence=%d degraded=%d provider_errors=%d groundedness=%s relevance=%s first_token_p50=%sms p95=%sms total_p50=%sms tokens_per_answer=%s direct_answer_rate=%s",
		reportPath, report.Summary.Answered, report.Summary.NoEvidence, report.Summary.Degraded, report.Summary.ProviderErrors,
		floatPtrString(report.Summary.Groundedness), floatPtrString(report.Summary.AnswerRelevance),
		floatPtrString(report.Summary.FirstTokenP50Ms), floatPtrString(report.Summary.FirstTokenP95Ms),
		floatPtrString(report.Summary.TotalP50Ms), floatPtrString(report.Summary.TokensPerAnswerMean),
		floatPtrString(report.Summary.DirectAnswerRate))
}

func resolveAnswerEvalChatUser(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	if raw := strings.TrimSpace(os.Getenv(answerEvalUserIDEnv)); raw != "" {
		var parsed int64
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err != nil || parsed <= 0 {
			t.Fatalf("%s must be a positive user id", answerEvalUserIDEnv)
		}
		return parsed
	}
	var userID int64
	err := db.Raw(`
		SELECT MIN(u.id) FROM users u
		WHERE NOT EXISTS (
			SELECT 1 FROM content_items ci
			WHERE ci.author_id = u.id
			  AND ci.status = 'published'
			  AND ci.deleted_at IS NULL
			  AND ci.is_public = FALSE
		)`).Scan(&userID).Error
	if err != nil || userID <= 0 {
		t.Fatalf("no seeded user without private corpus content; pass %s explicitly", answerEvalUserIDEnv)
	}
	return userID
}

// applyAnswerLayerJudges runs the v2 layer-specific judges on one case: the
// no-answer dual-strategy judge (H6) and the four-surface visibility leak
// accounting (H7). Cases without a layer annotation (v1) skip both.
func applyAnswerLayerJudges(ctx context.Context, t *testing.T, db *gorm.DB, golden model.EvalGoldenCase, cl Classification, result *AnswerEvalCaseResult, identity ViewerIdentity) {
	t.Helper()
	switch cl.PrimaryLayer {
	case LayerNoAnswer:
		if cl.NoAnswerStrategy == "" {
			return
		}
		switch result.Status {
		case AnswerStatusAnswered:
			// Judged on the grounded answer as before.
		case AnswerStatusNoEvidence:
			// The model answered directly without retrieval: the raw answer
			// text is kept on the result precisely so this judge can score
			// refusal behavior. A substantive non-refusal direct answer must
			// count as a no-answer failure, not silently pass. Skip only when
			// nothing was produced.
			if strings.TrimSpace(result.Answer) == "" {
				return
			}
		default:
			return
		}
		rubric, err := ParseAnswerRubric(golden.AnswerRubric)
		if err != nil {
			t.Fatalf("case %s answer_rubric: %v", golden.CaseKey, err)
		}
		citations, err := ParseCitations(golden.ExpectedCitations)
		if err != nil {
			t.Fatalf("case %s expected_citations: %v", golden.CaseKey, err)
		}
		var expectedIDs []int64
		for _, citation := range citations {
			expectedIDs = append(expectedIDs, citation.ContentID)
		}
		judge := JudgeNoAnswer(NoAnswerJudgeInput{
			Strategy:        cl.NoAnswerStrategy,
			Answer:          result.Answer,
			Citations:       result.Citations,
			RetrievedIDs:    result.RetrievedIDs,
			ExpectedIDs:     expectedIDs,
			AcceptableIDs:   rubric.AcceptableContentIDs,
			MustNotClaim:    rubric.MustNotClaim,
			KnownContentIDs: loadAnswerEvalKnownContentIDs(ctx, t, db),
		})
		result.NoAnswer = &judge
	case LayerVisibility:
		forbidden, err := ParseInt64List(golden.ForbiddenContentIDs)
		if err != nil || len(forbidden) == 0 {
			return
		}
		leak := EvaluateVisibilityLeaks(VisibilityLeakInput{
			ForbiddenIDs: forbidden,
			ForbiddenDoc: loadAnswerEvalForbiddenDocs(ctx, t, db, forbidden),
			RetrievedIDs: result.RetrievedIDs,
			Citations:    result.Citations,
			Answer:       result.Answer,
			ToolSteps:    result.ToolSteps,
		})
		result.VisibilityLeak = &leak
	}
}

// loadAnswerEvalKnownContentIDs is the existence universe for the
// related-recommendation truth check: published, non-deleted, public content.
func loadAnswerEvalKnownContentIDs(ctx context.Context, t *testing.T, db *gorm.DB) map[int64]bool {
	t.Helper()
	var ids []int64
	err := db.WithContext(ctx).
		Table("content_items").
		Where("status = ? AND deleted_at IS NULL AND is_public = TRUE", "published").
		Pluck("id", &ids).Error
	if err != nil {
		t.Fatalf("load known content ids: %v", err)
	}
	known := make(map[int64]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	return known
}

// loadAnswerEvalForbiddenDocs loads the title/description identity of the
// restricted documents for answer/tool-surface mention detection.
func loadAnswerEvalForbiddenDocs(ctx context.Context, t *testing.T, db *gorm.DB, ids []int64) []ForbiddenDoc {
	t.Helper()
	var rows []struct {
		ID          int64
		Title       string
		Description string
	}
	err := db.WithContext(ctx).
		Table("content_items").
		Select("id, title, COALESCE(description, '') AS description").
		Where("id IN ?", ids).
		Scan(&rows).Error
	if err != nil {
		t.Fatalf("load forbidden docs: %v", err)
	}
	docs := make([]ForbiddenDoc, 0, len(rows))
	for _, r := range rows {
		docs = append(docs, ForbiddenDoc{ContentID: r.ID, Title: r.Title, Summary: r.Description})
	}
	return docs
}

func runAnswerEvalCase(ctx context.Context, db *gorm.DB, chunkRepo *repository.RagChunkRepository, provider llm.LLMProvider, retriever *ragservice.HybridRetriever, searchRepo *repository.SearchRepository, cfg *config.Config, golden model.EvalGoldenCase, chatUserID int64, maxAttempts int) AnswerEvalCaseResult {
	result := AnswerEvalCaseResult{
		CaseKey:       golden.CaseKey,
		Query:         golden.Query,
		QueryLanguage: golden.QueryLanguage,
		Status:        AnswerStatusProvError,
	}
	if _, err := ParseViewerContext(golden.ViewerContext); err != nil {
		result.ErrorCode = "viewer_context_invalid"
		return result
	}
	// MiniMax-M3 tool-call compliance under the minimal production prompt is
	// probabilistic: the model sometimes answers directly without retrieval.
	// Retry a bounded number of times, count the direct-answer attempts, and
	// measure the attempt that actually engaged retrieval.
	directAnswers := 0
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptResult, terminal := runAnswerEvalAttempt(ctx, db, chunkRepo, provider, retriever, searchRepo, cfg, golden, chatUserID)
		if !terminal {
			directAnswers++
			if attempt == maxAttempts {
				result.Attempts = attempt
				result.DirectAnswers = directAnswers
				result.Status = AnswerStatusNoEvidence
				result.AnswerKind = attemptResult.AnswerKind
				result.Answer = attemptResult.Answer
				result.RetrievedIDs = attemptResult.RetrievedIDs
				result.ToolCalls = attemptResult.ToolCalls
				result.FirstTokenMs = attemptResult.FirstTokenMs
				result.TotalMs = attemptResult.TotalMs
				result.PromptTokens = attemptResult.PromptTokens
				result.CompletionTokens = attemptResult.CompletionTokens
				return result
			}
			continue
		}
		result.Attempts = attempt
		result.DirectAnswers = directAnswers
		result.Status = attemptResult.Status
		result.AnswerKind = attemptResult.AnswerKind
		result.Answer = attemptResult.Answer
		result.RetrievedIDs = attemptResult.RetrievedIDs
		result.ToolCalls = attemptResult.ToolCalls
		result.ToolSteps = attemptResult.ToolSteps
		result.Citations = attemptResult.Citations
		result.EvidenceChunks = attemptResult.EvidenceChunks
		result.EvidenceLookups = attemptResult.EvidenceLookups
		result.Groundedness = attemptResult.Groundedness
		result.AnswerRelevance = attemptResult.AnswerRelevance
		result.FirstTokenMs = attemptResult.FirstTokenMs
		result.TotalMs = attemptResult.TotalMs
		result.PromptTokens = attemptResult.PromptTokens
		result.CompletionTokens = attemptResult.CompletionTokens
		result.Degraded = attemptResult.Degraded
		result.DegradedReason = attemptResult.DegradedReason
		result.ErrorCode = attemptResult.ErrorCode
		return result
	}
	return result
}

// runAnswerEvalAttempt performs one chat turn. terminal is false when the
// stream succeeded, nothing degraded, and the model produced no citations —
// i.e. it answered directly without engaging retrieval.
func runAnswerEvalAttempt(ctx context.Context, db *gorm.DB, chunkRepo *repository.RagChunkRepository, provider llm.LLMProvider, retriever *ragservice.HybridRetriever, searchRepo *repository.SearchRepository, cfg *config.Config, golden model.EvalGoldenCase, chatUserID int64) (AnswerEvalCaseResult, bool) {
	attempt := AnswerEvalCaseResult{Status: AnswerStatusProvError}
	caseCtx, cancel := context.WithTimeout(ctx, answerEvalCaseTimeot)
	defer cancel()

	measured := &firstTokenMeasuringProvider{LLMProvider: provider}
	agentSvc := service.NewAgentService(measured, repository.NewEmbeddingRepository(db), repository.NewContentRepository(db), nil, db, cfg)
	agentSvc.SetSearchRepository(searchRepo)
	ragAdapter := &answerEvalRAGAdapter{retriever: retriever}
	agentSvc.SetContentRetriever(ragAdapter)

	collector := &answerEvalStreamCollector{}
	startedAt := time.Now()
	streamErr := agentSvc.ChatStream(caseCtx, chatUserID, service.ChatTurnInput{Message: golden.Query}, &service.ResolvedChatContext{Surface: model.AgentChatSurfaceGlobal}, collector.handle)
	attempt.TotalMs = time.Since(startedAt).Milliseconds()
	if firstTokenMs, ok := measured.firstTokenLatencyMs(); ok {
		attempt.FirstTokenMs = &firstTokenMs
	}

	if streamErr != nil {
		attempt.Status = AnswerStatusProvError
		attempt.ErrorCode = collector.errorCode
		if attempt.ErrorCode == "" {
			attempt.ErrorCode = "provider_error"
		}
		return attempt, true
	}
	attempt.RetrievedIDs = ragAdapter.retrievedIDs
	attempt.AnswerKind = string(collector.kind)
	attempt.ToolCalls = collector.toolCalls
	attempt.ToolSteps = collector.toolSteps
	if collector.usage != nil {
		prompt := collector.usage.PromptTokens
		completion := collector.usage.CompletionTokens
		attempt.PromptTokens = &prompt
		attempt.CompletionTokens = &completion
	}
	for _, citation := range collector.citations {
		attempt.Citations = append(attempt.Citations, AnswerEvalCitation{
			ContentID:      citation.ContentID,
			ContentVersion: citation.ContentVersion,
			ChunkKey:       citation.ChunkKey,
			ChunkIndex:     citation.ChunkIndex,
			Title:          citation.Title,
			Zone:           citation.Zone,
			Source:         citation.Source,
		})
	}

	if collector.degraded {
		attempt.Status = AnswerStatusDegraded
		attempt.Degraded = true
		attempt.DegradedReason = collector.degradedReason
		return attempt, true
	}
	if len(collector.citations) == 0 {
		// The service contract discards no-evidence answers for the client,
		// but the model still produced text. Keep it on the attempt so the
		// no-answer layer can judge refusal behavior on direct answers
		// instead of silently skipping them.
		attempt.Answer = strings.TrimSpace(collector.answer)
		attempt.Status = AnswerStatusNoEvidence
		return attempt, false
	}

	// Evidence join: resolve each citation to its viewer-visible chunk text
	// through the same citation-truth seam the service enforces.
	evidenceTexts := make([]string, 0, len(collector.citations))
	for _, citation := range collector.citations {
		attempt.EvidenceLookups++
		truth, truthErr := chunkRepo.LoadVisibleCitationTruth(caseCtx, chatUserID, repository.CitationLookup{
			ContentID:      citation.ContentID,
			ContentVersion: citation.ContentVersion,
			ChunkIndex:     citation.ChunkIndex,
			ChunkKey:       citation.ChunkKey,
		})
		if truthErr == nil && strings.TrimSpace(truth.Text) != "" {
			evidenceTexts = append(evidenceTexts, truth.Text)
			attempt.EvidenceChunks++
		}
	}

	answer := strings.TrimSpace(collector.answer)
	if answer == "" || len(evidenceTexts) == 0 {
		attempt.Status = AnswerStatusNoEvidence
		return attempt, true
	}
	grounded, relevance, _ := ScoreAnswerCase(answer, golden.Query, evidenceTexts)
	attempt.Groundedness = grounded
	attempt.AnswerRelevance = relevance
	attempt.Answer = answer
	attempt.Status = AnswerStatusAnswered
	return attempt, true
}

func countAnswerStatus(results []AnswerEvalCaseResult, status string) int {
	count := 0
	for i := range results {
		if results[i].Status == status {
			count++
		}
	}
	return count
}

func msPtrString(v *int64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

func intPtrString(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

func floatPtrString(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.3f", *v)
}
