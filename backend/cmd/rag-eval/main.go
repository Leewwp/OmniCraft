package main

// rag-eval is the A-04 evaluation runner (#286): it drives the production
// agent paths (retrieval through AgentService.ExecuteTool, generation through
// AgentService.ChatStream) over the frozen golden set v2 and emits redacted
// per-case JSONL plus a layered summary with raw numerators/denominators and
// Wilson intervals. One process measures exactly one retrieval configuration:
// the A-04 ablation switches (features.rag_{hybrid,query_expansion,rerank}_enabled)
// are selected by CONFIG_OVERRIDE_PATH, mirroring cmd/rag-probe. The
// five-config loop and the comparison table are produced by
// scripts/corpus/a04_ablation.py.
//
// Fail-closed posture: runtime identity is asserted against the sanctioned
// evaluation set before any case runs; split counts are pinned; the test
// split needs -confirm-test-run (H4: final test runs once). Provider failures
// are recorded as provider_error — never mocked around.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"gorm.io/gorm"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/database"
	"omnicraft/backend/internal/pkg/llm"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	rageval "omnicraft/backend/internal/service/rag_eval"
)

// sanctioned chat identities: canonical MiniMax-M3 plus the openai_compat
// qwen-plus evaluation variant (agent-runtime-matrix.md L2/L3).
var sanctionedChat = map[string]string{
	"minimax":       "MiniMax-M3",
	"openai_compat": "qwen-plus",
}

const (
	sanctionedEmbeddingModel = "text-embedding-v4"
	sanctionedEmbeddingDims  = 1536
	// frozen split sizes (freeze-report-v2.md): fail-closed pins.
	expectDevCases  = 156
	expectTestCases = 40
	// pathological sd cases (freeze report §4): expected_never_retrieved,
	// reported single-column, excluded from routine layer interpretation.
)

var pathologicalCases = []string{"sd-0027", "sd-0040"}

type genRun struct {
	Phase          string                       `json:"phase"` // always "generation" (checkpoint/resume key)
	CaseKey        string                       `json:"case_key"`
	PrincipalKey   string                       `json:"principal_key"`
	Query          string                       `json:"query"`
	Status         string                       `json:"status"` // answered | no_evidence | degraded | provider_error
	AnswerKind     string                       `json:"answer_kind,omitempty"`
	Answer         string                       `json:"answer,omitempty"`
	Citations      []rageval.AnswerEvalCitation `json:"citations,omitempty"`
	ToolSteps      []rageval.ToolStepRecord     `json:"tool_steps,omitempty"`
	RetrievedIDs   []int64                      `json:"retrieved_ids,omitempty"`
	Degraded       bool                         `json:"degraded"`
	DegradedReason string                       `json:"degraded_reason,omitempty"`
	ErrorCode      string                       `json:"error_code,omitempty"`
	Attempts       int                          `json:"attempts"`

	Deterministic     *rageval.DeterministicJudgeResult `json:"deterministic,omitempty"`
	NoAnswer          *rageval.NoAnswerJudgeResult      `json:"no_answer,omitempty"`
	VisibilityLeak    *rageval.VisibilityLeakReport     `json:"visibility_leak,omitempty"`
	CitationPrecision *float64                          `json:"citation_precision,omitempty"`
	LatencyMs         float64                           `json:"latency_ms"`
}

type checkpointLine struct {
	Phase        string `json:"phase"` // retrieval | generation
	CaseKey      string `json:"case_key"`
	PrincipalKey string `json:"principal_key"`
}

func main() {
	split := flag.String("split", "dev", "golden-set split: dev (default) or test")
	confirmTest := flag.Bool("confirm-test-run", false, "H4 gate: required for split=test (final test runs once)")
	label := flag.String("label", "", "config label recorded in the header")
	out := flag.String("out", "", "per-case JSONL output path (checkpoint file)")
	summaryPath := flag.String("summary", "", "summary JSON output path")
	resume := flag.Bool("resume", false, "skip (case,principal,phase) records already present in -out")
	maxCases := flag.Int("max-cases", 0, "cap the number of cases (smoke only; 0 = all)")
	skipGeneration := flag.Bool("skip-generation", false, "retrieval measurement only")
	rejudge := flag.Bool("rejudge", false, "re-apply the deterministic judges to stored generation rows (no provider calls) and rewrite the summary")
	sleepMs := flag.Int("sleep-ms", 150, "pause between generation calls (provider courtesy)")
	flag.Parse()
	if *out == "" || *summaryPath == "" {
		slog.Error("usage: rag-eval -label C0-v4-only -out runs.jsonl -summary summary.json [-split dev] [-resume]")
		os.Exit(2)
	}

	cfg := config.Load()

	// ---- fail-closed identity assertions (agent-runtime-matrix conformance)
	wantModel, ok := sanctionedChat[strings.TrimSpace(cfg.Agent.LLMProvider)]
	if !ok || wantModel != strings.TrimSpace(cfg.Agent.LLMModel) {
		slog.Error("unsanctioned chat identity", "provider", cfg.Agent.LLMProvider, "model", cfg.Agent.LLMModel,
			"sanctioned", sanctionedChat)
		os.Exit(3)
	}
	if strings.TrimSpace(cfg.RAG.Index.EmbeddingModel) != sanctionedEmbeddingModel {
		slog.Error("unsanctioned embedding model", "model", cfg.RAG.Index.EmbeddingModel, "want", sanctionedEmbeddingModel)
		os.Exit(3)
	}
	switch strings.TrimSpace(cfg.RAG.Hybrid.KeywordSource) {
	case "", "postgres":
	default:
		slog.Error("keyword_source must be postgres (canonical) for the ablation", "got", cfg.RAG.Hybrid.KeywordSource)
		os.Exit(3)
	}
	if !cfg.Agent.WebAgentEnabled {
		slog.Error("agent.web_agent_enabled is off: generation would be disabled (set it in the override yaml)")
		os.Exit(3)
	}
	if cfg.Features.RAGRerankEnabled && strings.TrimSpace(cfg.RAG.Rerank.Model) == "" {
		slog.Error("rerank switch on but rag.rerank.model is empty")
		os.Exit(3)
	}

	db := database.Init(cfg)
	rdb := redisclient.Init(cfg)
	defer rdb.Close()

	// embedder self-check: the query embedder must be live and 1536-dim,
	// otherwise C0 silently degrades and the measurement lies.
	probeVec, embErr := llm.NewProvider(cfg).GetEmbedding(context.Background(), "self check")
	if embErr != nil {
		slog.Error("EMBEDDER SELF-CHECK FAILED", "model", cfg.RAG.Index.EmbeddingModel,
			"api_base", cfg.Agent.EmbeddingAPIBase, "error", embErr)
		os.Exit(3)
	}
	if len(probeVec) != sanctionedEmbeddingDims {
		slog.Error("embedding dims mismatch", "got", len(probeVec), "want", sanctionedEmbeddingDims)
		os.Exit(3)
	}

	ctr := container.NewContainer(db, rdb, cfg)

	// ---- case loading + split gate
	ctx := context.Background()
	evalRepo := repository.NewRagEvaluationRepository(db)
	all, err := evalRepo.ListActiveGoldenCases(ctx)
	if err != nil {
		slog.Error("list golden cases", "error", err)
		os.Exit(1)
	}
	if err := rageval.ValidateTestSplitGate(*split, *confirmTest); err != nil {
		slog.Error("split gate", "error", err)
		os.Exit(2)
	}
	selected, err := rageval.FilterCasesBySplit(all, *split, false)
	if err != nil {
		slog.Error("filter split", "error", err)
		os.Exit(1)
	}
	want := expectDevCases
	if *split == "test" {
		want = expectTestCases
	}
	if *maxCases > 0 {
		if *maxCases > len(selected) {
			slog.Error("-max-cases exceeds split size", "max", *maxCases, "split", len(selected))
			os.Exit(2)
		}
	} else if len(selected) != want {
		slog.Error("split size drift", "split", *split, "got", len(selected), "want", want)
		os.Exit(3)
	}
	if *maxCases > 0 {
		selected = selected[:*maxCases]
	}

	// dataset checksum over the canonical body of the full active set.
	payloads := make([]rageval.GoldenCasePayload, 0, len(all))
	for _, c := range all {
		p, err := rageval.ModelCaseToPayload(c)
		if err != nil {
			slog.Error("convert case", "case", c.CaseKey, "error", err)
			os.Exit(1)
		}
		payloads = append(payloads, p)
	}
	body, err := rageval.GoldenSetBody(payloads)
	if err != nil {
		slog.Error("golden set body", "error", err)
		os.Exit(1)
	}
	datasetChecksum := rageval.ChecksumOf(body)

	// public-content universe for the no-answer recommendation-truth check.
	var publicIDList []int64
	if err := db.Raw(`SELECT id FROM content_items WHERE is_public = true`).Scan(&publicIDList).Error; err != nil {
		slog.Error("public id universe", "error", err)
		os.Exit(1)
	}
	publicIDs := make(map[int64]bool, len(publicIDList))
	for _, id := range publicIDList {
		publicIDs[id] = true
	}

	registry := &rageval.FixturePrincipalRegistry{DB: db}
	// generation runs under the seeded fixture user even for the anon
	// principal (anonymous-equivalent account; principal.go contract note).
	var fixtureUserID int64
	if err := db.Raw(`SELECT id FROM users WHERE email = ?`, rageval.FixtureViewerAnonEmail).Scan(&fixtureUserID).Error; err != nil || fixtureUserID == 0 {
		slog.Error("fixture viewer account missing", "email", rageval.FixtureViewerAnonEmail, "error", err)
		os.Exit(3)
	}

	// ---- checkpoint bookkeeping
	done := map[string]bool{}
	priorGen := map[string]genRun{}
	if *resume {
		if existing, err := os.ReadFile(*out); err == nil {
			for _, line := range strings.Split(string(existing), "\n") {
				if line == "" || strings.HasPrefix(line, `{"kind"`) {
					continue
				}
				var cl checkpointLine
				if json.Unmarshal([]byte(line), &cl) == nil && cl.Phase != "" {
					done[cl.Phase+"|"+cl.CaseKey+"|"+cl.PrincipalKey] = true
				}
				var probe genRun
				if json.Unmarshal([]byte(line), &probe) == nil && probe.CaseKey != "" && probe.Status != "" {
					// generation row (legacy rows carry no phase field)
					priorGen[probe.CaseKey+"|"+probe.PrincipalKey] = probe
				}
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		slog.Error("mkdir out", "error", err)
		os.Exit(1)
	}
	fh, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("open out", "error", err)
		os.Exit(1)
	}
	defer fh.Close()
	enc := json.NewEncoder(fh)
	if !*resume {
		header, _ := json.Marshal(map[string]any{
			"kind": "rag-eval", "label": *label, "split": *split,
			"switches": map[string]bool{
				"hybrid": cfg.Features.RAGHybridEnabled, "query_expansion": cfg.Features.RAGQueryExpansionEnabled,
				"rerank": cfg.Features.RAGRerankEnabled,
			},
			"runtime": map[string]any{
				"chat":           map[string]string{"provider": cfg.Agent.LLMProvider, "model": cfg.Agent.LLMModel, "api_base": cfg.Agent.LLMAPIBase},
				"embedding":      map[string]string{"provider": cfg.Agent.EmbeddingProvider, "model": cfg.RAG.Index.EmbeddingModel, "api_base": cfg.Agent.EmbeddingAPIBase},
				"rerank":         map[string]string{"provider": cfg.RAG.Rerank.Provider, "model": cfg.RAG.Rerank.Model},
				"keyword_source": cfg.RAG.Hybrid.KeywordSource,
			},
			"embedding_dims": len(probeVec), "dataset_checksum": datasetChecksum,
			"cases": len(selected), "ran_at": time.Now().UTC().Format(time.RFC3339),
		})
		fmt.Fprintln(fh, string(header))
	}

	if *rejudge {
		rejudgeAndExit(*out, *summaryPath, db, all, publicIDs)
	}

	// ---- phase 1: retrieval measurement via the production search tool
	retrieve := func(ctx context.Context, query string, identity rageval.ViewerIdentity, topK int) ([]rageval.Retrieved, error) {
		args, _ := json.Marshal(map[string]string{"query": query})
		outcome, err := ctr.AgentService.ExecuteTool(ctx, "search_content", json.RawMessage(args), identity.ViewerUserID, nil)
		if err != nil {
			return nil, err
		}
		hits := make([]rageval.Retrieved, 0, len(outcome.Search))
		seen := map[int64]bool{}
		// ContentSummary carries no score, so the production rank order is
		// preserved with positional descending scores (determinize sorts by
		// score); without this the id tiebreak would scramble ranks.
		for i, s := range outcome.Search {
			if seen[s.ID] {
				continue
			}
			seen[s.ID] = true
			hits = append(hits, rageval.Retrieved{
				ContentID: s.ID, ContentVersion: int64(s.ContentVersion),
				Score: float64(len(outcome.Search) - i),
			})
		}
		if len(hits) > topK {
			hits = hits[:topK]
		}
		return hits, nil
	}

	retrievalResult, err := rageval.RunLayeredRetrievalEval(ctx, selected, registry, retrieve,
		rageval.LayeredEvalOptions{Split: *split, ConfirmTestSplitRun: *confirmTest, TopK: 20},
		rageval.RunSpec{
			RunKey:           fmt.Sprintf("a04-%s-%s", *label, *split),
			RetrieverVersion: "agent-search-content-tool-v1",
			ChunkingVersion:  "041-current",
			IndexVersion:     "v4-1536",
			DatasetChecksum:  datasetChecksum,
		},
		rageval.Environment{Note: "A-04 ablation measurement; redacted local artifact"})
	if err != nil {
		slog.Error("layered retrieval eval", "error", err)
		os.Exit(1)
	}
	retrievedByCase := map[string][]int64{}
	for _, pc := range retrievalResult.PerCase {
		retrievedByCase[pc.CaseKey+"|"+pc.PrincipalKey] = pc.RetrievedIDs
		if *resume && done["retrieval|"+pc.CaseKey+"|"+pc.PrincipalKey] {
			continue
		}
		if err := enc.Encode(map[string]any{
			"phase": "retrieval", "case_key": pc.CaseKey, "principal_key": pc.PrincipalKey,
			"retrieved_ids": pc.RetrievedIDs, "recall_at_5": pc.RecallAt5, "recall_at_10": pc.RecallAt10,
			"ndcg_at_5": pc.GradedNDCGAt5, "mrr": pc.MRR, "success": pc.Success, "error": pc.Error,
		}); err != nil {
			slog.Error("encode retrieval row", "error", err)
			os.Exit(1)
		}
	}

	// ---- phase 2: generation + deterministic judging
	var genRuns []genRun
	var answerCases []rageval.AnswerEvalCaseResult
	if !*skipGeneration {
		for _, c := range selected {
			cl, err := rageval.ParseClassificationV2(c.Classification)
			if err != nil {
				slog.Error("classification", "case", c.CaseKey, "error", err)
				os.Exit(1)
			}
			rubric, err := rageval.ParseAnswerRubric(c.AnswerRubric)
			if err != nil {
				slog.Error("rubric", "case", c.CaseKey, "error", err)
				os.Exit(1)
			}
			citations, err := rageval.ParseCitations(c.ExpectedCitations)
			if err != nil {
				slog.Error("expected citations", "case", c.CaseKey, "error", err)
				os.Exit(1)
			}
			var expectedIDs []int64
			for _, cit := range citations {
				expectedIDs = append(expectedIDs, cit.ContentID)
			}
			forbiddenIDs, err := rageval.ParseInt64List(c.ForbiddenContentIDs)
			if err != nil {
				slog.Error("forbidden ids", "case", c.CaseKey, "error", err)
				os.Exit(1)
			}
			forbiddenDocs := forbiddenDocsOf(ctx, db, forbiddenIDs)

			for _, principalKey := range rageval.PrincipalKeysForCase(cl.Layer(), principalKeyOf(c)) {
				if prior, ok := priorGen[c.CaseKey+"|"+principalKey]; ok {
					// resumed checkpointed record: still counted in the summary
					genRuns = append(genRuns, prior)
					answerCases = append(answerCases, answerCaseOf(c, cl, prior))
					continue
				}
				run := generateOne(ctx, ctr, c, principalKey, fixtureUserID, retrievedByCase)
				run.Phase = "generation"
				judgeGeneration(c.Query, cl, rubric, expectedIDs, forbiddenIDs, forbiddenDocs, publicIDs, &run)
				genRuns = append(genRuns, run)
				answerCases = append(answerCases, answerCaseOf(c, cl, run))
				if err := enc.Encode(run); err != nil {
					slog.Error("encode generation row", "error", err)
					os.Exit(1)
				}
				time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
			}
		}
	}

	// ---- summary
	summary := map[string]any{
		"kind": "a04-eval-summary", "label": *label, "split": *split,
		"switches": map[string]bool{
			"hybrid": cfg.Features.RAGHybridEnabled, "query_expansion": cfg.Features.RAGQueryExpansionEnabled,
			"rerank": cfg.Features.RAGRerankEnabled,
		},
		"runtime": map[string]any{
			"chat":           map[string]string{"provider": cfg.Agent.LLMProvider, "model": cfg.Agent.LLMModel, "api_base": cfg.Agent.LLMAPIBase},
			"embedding":      map[string]string{"provider": cfg.Agent.EmbeddingProvider, "model": cfg.RAG.Index.EmbeddingModel, "api_base": cfg.Agent.EmbeddingAPIBase},
			"rerank":         map[string]string{"provider": cfg.RAG.Rerank.Provider, "model": cfg.RAG.Rerank.Model},
			"keyword_source": cfg.RAG.Hybrid.KeywordSource,
			"embedding_dims": len(probeVec),
		},
		"dataset_checksum":   datasetChecksum,
		"cases":              len(selected),
		"retrieval_groups":   retrievalResult.Groups,
		"pathological_cases": pathologicalCases,
		"generation_layers":  rageval.BuildLayeredAnswerSummary(answerCases),
		"citation_precision": citationPrecisionByLayer(genRuns),
		"deterministic_pass": deterministicPassByLayer(genRuns),
		"ran_at":             time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		slog.Error("marshal summary", "error", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*summaryPath, append(data, '\n'), 0o644); err != nil {
		slog.Error("write summary", "error", err)
		os.Exit(1)
	}
	slog.Info("rag-eval done", "label", *label, "split", *split, "cases", len(selected),
		"generation_runs", len(genRuns), "summary", *summaryPath)
}

// judgeGeneration applies the deterministic judges to one produced run in
// place. It is shared by the live run and -rejudge so judge refinements can
// be re-applied to stored answers without re-calling the provider.
func judgeGeneration(query string, cl rageval.Classification, rubric rageval.AnswerRubric,
	expectedIDs, forbiddenIDs []int64, forbiddenDocs []rageval.ForbiddenDoc,
	publicIDs map[int64]bool, run *genRun) {
	run.Deterministic = ptrOf(rageval.JudgeDeterministicAssertions(run.Answer, rubric.DeterministicAssertions))
	if cl.Layer() == rageval.LayerNoAnswer {
		run.NoAnswer = ptrOfNoAnswer(rageval.JudgeNoAnswer(rageval.NoAnswerJudgeInput{
			Strategy: cl.NoAnswerStrategy, Answer: run.Answer, Citations: run.Citations,
			RetrievedIDs: run.RetrievedIDs, ExpectedIDs: expectedIDs,
			AcceptableIDs: rubric.AcceptableContentIDs, MustNotClaim: rubric.MustNotClaim,
			KnownContentIDs: publicIDs,
		}))
	}
	if cl.Layer() == rageval.LayerVisibility {
		// Title echo: when the forbidden title is part of the user's own
		// query, restating it is not a disclosure — the title surface is
		// suppressed for those docs (summary and bounded-id matching stay).
		queryNorm := rageval.NormalizeAssertionText(query)
		docs := make([]rageval.ForbiddenDoc, len(forbiddenDocs))
		for i, d := range forbiddenDocs {
			if d.Title != "" && strings.Contains(queryNorm, rageval.NormalizeAssertionText(d.Title)) {
				d.Title = ""
			}
			docs[i] = d
		}
		run.VisibilityLeak = ptrOfLeak(rageval.EvaluateVisibilityLeaks(rageval.VisibilityLeakInput{
			ForbiddenIDs: forbiddenIDs, ForbiddenDoc: docs, RetrievedIDs: run.RetrievedIDs,
			Citations: run.Citations, Answer: run.Answer, ToolSteps: run.ToolSteps,
		}))
	}
	if len(run.Citations) > 0 {
		tier := map[int64]bool{}
		for _, id := range expectedIDs {
			tier[id] = true
		}
		for _, id := range rubric.AcceptableContentIDs {
			tier[id] = true
		}
		hits := 0
		for _, cit := range run.Citations {
			if tier[cit.ContentID] {
				hits++
			}
		}
		p := float64(hits) / float64(len(run.Citations))
		run.CitationPrecision = &p
	}
}

// generateOne drives one ChatStream turn and collects the four observable
// surfaces plus status buckets. Provider failures after one bounded retry are
// recorded as provider_error (external-input-missing semantics: never mocked).
func generateOne(ctx context.Context, ctr *container.ServiceContainer, c model.EvalGoldenCase, principalKey string, fixtureUserID int64, retrievedByCase map[string][]int64) genRun {
	run := genRun{
		CaseKey: c.CaseKey, PrincipalKey: principalKey, Query: c.Query,
		RetrievedIDs: retrievedByCase[c.CaseKey+"|"+principalKey],
	}
	for attempt := 1; attempt <= 2; attempt++ {
		run.Attempts = attempt
		start := time.Now()
		var answer strings.Builder
		var citations []rageval.AnswerEvalCitation
		var toolSteps []rageval.ToolStepRecord
		errorCode := ""
		turnCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		err := ctr.AgentService.ChatStream(turnCtx, fixtureUserID, service.ChatTurnInput{Message: c.Query}, nil,
			func(ev service.AgentStreamEvent) error {
				switch ev.Type {
				case service.AgentEventDelta:
					answer.WriteString(ev.Delta)
				case service.AgentEventToolStatus:
					if ev.Tool != nil {
						toolSteps = append(toolSteps, rageval.ToolStepRecord{Name: ev.Tool.Name, ArgsSummary: ev.Tool.ArgsSummary})
					}
				case service.AgentEventDone:
					run.AnswerKind = string(ev.AnswerKind)
					for _, cit := range ev.Citations {
						citations = append(citations, rageval.AnswerEvalCitation{
							ContentID: cit.ContentID, ContentVersion: cit.ContentVersion, ChunkKey: cit.ChunkKey,
							ChunkIndex: cit.ChunkIndex, Title: cit.Title, Zone: cit.Zone, Source: cit.Source,
						})
					}
					run.Degraded = ev.Degraded
					run.DegradedReason = ev.DegradedReason
				case service.AgentEventError:
					errorCode = ev.ErrorCode
				}
				return nil
			})
		cancel()
		run.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
		run.Answer = answer.String()
		run.Citations = citations
		run.ToolSteps = toolSteps
		run.ErrorCode = errorCode
		switch {
		case errorCode != "":
			run.Status = rageval.AnswerStatusProvError
			if attempt == 1 && (errorCode == "AGENT_PROVIDER_ERROR" || errorCode == "AGENT_PROVIDER_TIMEOUT") {
				slog.Warn("provider failure, one retry", "case", c.CaseKey, "code", errorCode)
				time.Sleep(2 * time.Second)
				continue
			}
		case run.AnswerKind == string(service.AgentAnswerNoEvidence):
			run.Status = rageval.AnswerStatusNoEvidence
		case run.Degraded:
			run.Status = rageval.AnswerStatusDegraded
		case err == nil && run.Answer != "":
			run.Status = rageval.AnswerStatusAnswered
		default:
			run.Status = rageval.AnswerStatusProvError
			run.ErrorCode = "STREAM_INCOMPLETE"
		}
		return run
	}
	return run
}

func principalKeyOf(c model.EvalGoldenCase) string {
	vc, err := rageval.ParseViewerContext(c.ViewerContext)
	if err != nil {
		return ""
	}
	return vc.PrincipalKey
}

// forbiddenDocsOf loads the restricted documents' title/description for the
// visibility leak text surfaces (best-effort enrichment of the id tier).
func forbiddenDocsOf(ctx context.Context, db *gorm.DB, ids []int64) []rageval.ForbiddenDoc {
	if len(ids) == 0 {
		return nil
	}
	var rows []struct {
		ID          int64
		Title       string
		Description string
	}
	if err := db.WithContext(ctx).
		Table("content_items").
		Select("id, title, COALESCE(description, '') AS description").
		Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		slog.Error("load forbidden docs", "error", err)
		os.Exit(1)
	}
	docs := make([]rageval.ForbiddenDoc, 0, len(rows))
	for _, r := range rows {
		docs = append(docs, rageval.ForbiddenDoc{ContentID: r.ID, Title: r.Title, Summary: r.Description})
	}
	return docs
}

func answerCaseOf(c model.EvalGoldenCase, cl rageval.Classification, run genRun) rageval.AnswerEvalCaseResult {
	ac := rageval.AnswerEvalCaseResult{
		CaseKey: c.CaseKey, Query: c.Query, Status: run.Status, AnswerKind: run.AnswerKind,
		Attempts: run.Attempts, Citations: run.Citations, ToolSteps: run.ToolSteps,
		Degraded: run.Degraded, DegradedReason: run.DegradedReason, ErrorCode: run.ErrorCode,
		RetrievedIDs: run.RetrievedIDs, PrincipalKey: run.PrincipalKey,
		PrimaryLayer: cl.Layer(), Split: cl.Split,
		NoAnswerStrategy: cl.NoAnswerStrategy, NoAnswer: run.NoAnswer, VisibilityLeak: run.VisibilityLeak,
	}
	return ac
}

func ptrOf(r rageval.DeterministicJudgeResult) *rageval.DeterministicJudgeResult { return &r }
func ptrOfNoAnswer(r rageval.NoAnswerJudgeResult) *rageval.NoAnswerJudgeResult   { return &r }
func ptrOfLeak(r rageval.VisibilityLeakReport) *rageval.VisibilityLeakReport     { return &r }

type ratePair struct {
	Layer string
	Rate  rageval.RateWithCI
}

func citationPrecisionByLayer(runs []genRun) map[string]rageval.RateWithCI {
	type acc struct{ num, den float64 }
	accs := map[string]*acc{}
	for _, r := range runs {
		if len(r.Citations) == 0 {
			continue
		}
		layer := layerOfCase(r.CaseKey)
		a := accs[layer]
		if a == nil {
			a = &acc{}
			accs[layer] = a
		}
		a.den += float64(len(r.Citations))
		if r.CitationPrecision != nil {
			a.num += *r.CitationPrecision * float64(len(r.Citations))
		}
	}
	out := map[string]rageval.RateWithCI{}
	for layer, a := range accs {
		out[layer] = rageval.NewRate(a.num, a.den)
	}
	return out
}

func deterministicPassByLayer(runs []genRun) map[string]rageval.RateWithCI {
	type acc struct{ num, den float64 }
	accs := map[string]*acc{}
	for _, r := range runs {
		if r.Deterministic == nil || r.Deterministic.Total == 0 ||
			r.Deterministic.Total == r.Deterministic.Invalid {
			continue // no judgeable assertions (single-rune claims are invalid)
		}
		layer := layerOfCase(r.CaseKey)
		a := accs[layer]
		if a == nil {
			a = &acc{}
			accs[layer] = a
		}
		a.den++
		if r.Deterministic.Pass {
			a.num++
		}
	}
	out := map[string]rageval.RateWithCI{}
	for layer, a := range accs {
		out[layer] = rageval.NewRate(a.num, a.den)
	}
	return out
}

func layerOfCase(caseKey string) string {
	// golden-set case keys are `<layer>-<seq>` (ke-0001, sd-0001, ...).
	if idx := strings.Index(caseKey, "-"); idx > 0 {
		return caseKey[:idx]
	}
	return "unknown"
}

// rejudgeAndExit re-runs the deterministic judges over stored generation
// rows (judge vocabulary refinements, title-echo handling) without touching
// the provider, rewrites the checkpoint rows and refreshes the summary's
// generation-side fields. Retrieval groups are carried over from the previous
// summary: retrieval is not re-measured.
func rejudgeAndExit(outPath, summaryPath string, db *gorm.DB, all []model.EvalGoldenCase, publicIDs map[int64]bool) {
	byKey := make(map[string]model.EvalGoldenCase, len(all))
	for _, c := range all {
		byKey[c.CaseKey] = c
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		slog.Error("rejudge: read rows", "error", err)
		os.Exit(1)
	}
	var kept []string
	var genRuns []genRun
	var answerCases []rageval.AnswerEvalCaseResult
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		var probe genRun
		if json.Unmarshal([]byte(line), &probe) != nil || probe.CaseKey == "" || probe.Status == "" {
			kept = append(kept, line)
			continue
		}
		c, ok := byKey[probe.CaseKey]
		if !ok {
			slog.Error("rejudge: unknown case", "case", probe.CaseKey)
			os.Exit(1)
		}
		classification, err := rageval.ParseClassificationV2(c.Classification)
		if err != nil {
			slog.Error("rejudge: classification", "case", c.CaseKey, "error", err)
			os.Exit(1)
		}
		rubric, err := rageval.ParseAnswerRubric(c.AnswerRubric)
		if err != nil {
			slog.Error("rejudge: rubric", "case", c.CaseKey, "error", err)
			os.Exit(1)
		}
		citations, err := rageval.ParseCitations(c.ExpectedCitations)
		if err != nil {
			slog.Error("rejudge: expected citations", "case", c.CaseKey, "error", err)
			os.Exit(1)
		}
		var expectedIDs []int64
		for _, cit := range citations {
			expectedIDs = append(expectedIDs, cit.ContentID)
		}
		forbiddenIDs, err := rageval.ParseInt64List(c.ForbiddenContentIDs)
		if err != nil {
			slog.Error("rejudge: forbidden ids", "case", c.CaseKey, "error", err)
			os.Exit(1)
		}
		forbiddenDocs := forbiddenDocsOf(context.Background(), db, forbiddenIDs)
		run := probe
		run.Phase = "generation"
		judgeGeneration(c.Query, classification, rubric, expectedIDs, forbiddenIDs, forbiddenDocs, publicIDs, &run)
		genRuns = append(genRuns, run)
		answerCases = append(answerCases, answerCaseOf(c, classification, run))
		encoded, err := json.Marshal(run)
		if err != nil {
			slog.Error("rejudge: encode row", "error", err)
			os.Exit(1)
		}
		kept = append(kept, string(encoded))
	}
	if err := os.WriteFile(outPath, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		slog.Error("rejudge: rewrite rows", "error", err)
		os.Exit(1)
	}

	// refresh the generation-side summary fields, keeping retrieval groups
	prevRaw, err := os.ReadFile(summaryPath)
	if err != nil {
		slog.Error("rejudge: read previous summary", "error", err)
		os.Exit(1)
	}
	var prev map[string]any
	if err := json.Unmarshal(prevRaw, &prev); err != nil {
		slog.Error("rejudge: parse previous summary", "error", err)
		os.Exit(1)
	}
	prev["generation_layers"] = rageval.BuildLayeredAnswerSummary(answerCases)
	prev["citation_precision"] = citationPrecisionByLayer(genRuns)
	prev["deterministic_pass"] = deterministicPassByLayer(genRuns)
	prev["rejudged_at"] = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(prev, "", "  ")
	if err != nil {
		slog.Error("rejudge: marshal summary", "error", err)
		os.Exit(1)
	}
	if err := os.WriteFile(summaryPath, append(data, '\n'), 0o644); err != nil {
		slog.Error("rejudge: write summary", "error", err)
		os.Exit(1)
	}
	slog.Info("rag-eval rejudge done", "rows", len(genRuns), "summary", summaryPath)
	os.Exit(0)
}

// rejudgeAndExit re-runs the deterministic judges over stored generation
// rows (judge vocabulary refinements, title-echo handling) without touching
// the provider, rewrites the checkpoint rows and refreshes the summary's
// generation-side fields. Retrieval groups are carried over from the previous
// summary: retrieval is not re-measured.
func rejudgeAndExist(outPath, summaryPath string, all []model.EvalGoldenCase, publicIDs map[int64]bool) {
}
