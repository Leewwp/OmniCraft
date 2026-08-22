package rageval

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/repository"
	baseService "omnicraft/backend/internal/service"
	ragservice "omnicraft/backend/internal/service/rag"
)

const hybridEvalEnv = "OMNICRAFT_RAG_HYBRID_EVAL"
const hybridOpenSearchEnv = "OMNICRAFT_RAG_OPENSEARCH_URL"
const hybridEmbeddingModeEnv = "OMNICRAFT_RAG_EMBEDDING_MODE"
const hybridDiagnosisReportEnv = "OMNICRAFT_RAG_DIAG_REPORT"

type localProjectionEmbedder struct{ dimensions int }

func (e localProjectionEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vectors[i] = DeterministicEmbedding(text, e.dimensions)
	}
	return vectors, nil
}

func (e localProjectionEmbedder) GetEmbedding(_ context.Context, text string) ([]float32, error) {
	return DeterministicEmbedding(text, e.dimensions), nil
}

type unavailableKeywordRetriever struct{}

func (unavailableKeywordRetriever) Search(context.Context, string, int, int64) ([]ragservice.RetrievalCandidate, error) {
	return nil, ragservice.ErrRetrievalUnavailable
}

type localSearchProjection struct{}

func (localSearchProjection) UpsertContent(context.Context, string, []repository.SearchDocument) error {
	return nil
}

func (localSearchProjection) PruneStaleContent(context.Context, string, int64, []string) error {
	return nil
}

func (localSearchProjection) DeleteContent(context.Context, string, int64) error {
	return nil
}

// TestHybridGoldenSetEval runs the same full set as the T01 baselines. The
// embedding mode and OpenSearch endpoint are explicit: standin mode measures
// local fallback behavior, while real mode requires provider credentials.
// Set OMNICRAFT_RAG_HYBRID_EVAL=1 and OMNICRAFT_RAG_EMBEDDING_MODE=standin
// after loading rag_eval_seed.sql for the local-development run.
func TestHybridGoldenSetEval(t *testing.T) {
	if os.Getenv(hybridEvalEnv) != "1" {
		t.Skip("hybrid golden-set eval runs only with OMNICRAFT_RAG_HYBRID_EVAL=1")
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

	evalCfg := loadHybridEvalConfig(t)
	embeddingMode := strings.TrimSpace(os.Getenv(hybridEmbeddingModeEnv))
	if embeddingMode != "standin" && embeddingMode != "real" {
		t.Fatalf("%s must be standin or real", hybridEmbeddingModeEnv)
	}
	openSearchURL := os.Getenv(hybridOpenSearchEnv)
	if embeddingMode == "real" && openSearchURL == "" {
		t.Fatalf("%s is required for real embedding mode", hybridOpenSearchEnv)
	}
	var (
		chunkEmbedder  ragservice.ChunkEmbedder
		queryEmbedder  ragservice.QueryEmbedder
		embeddingModel string
	)
	if embeddingMode == "standin" {
		localEmbedder := localProjectionEmbedder{dimensions: evalCfg.Agent.EmbeddingDimensions}
		chunkEmbedder, queryEmbedder = localEmbedder, localEmbedder
		embeddingModel = "deterministic-sha256-standin-v1"
	} else {
		provider := llm.NewProvider(&evalCfg)
		chunkEmbedder = ragservice.NewProviderChunkEmbedder(provider)
		queryEmbedder = provider
		embeddingModel = evalCfg.RAG.Index.EmbeddingModel
	}
	var openSearch *repository.OpenSearchRepository
	var keywordSource ragservice.KeywordRetriever = unavailableKeywordRetriever{}
	var searchProjection ragservice.SearchProjection = localSearchProjection{}
	if openSearchURL != "" {
		openSearch = repository.NewOpenSearchRepositoryWithLimits(openSearchURL, &http.Client{Timeout: 2 * time.Second}, repository.OpenSearchResponseLimits{})
		keywordSource = ragservice.NewOpenSearchKeywordRetriever(openSearch)
		searchProjection = openSearch
		if err := db.WithContext(ctx).Exec("UPDATE index_projection_status SET last_indexed_at = NULL WHERE index_version = ?", evalCfg.RAG.Index.GenerationStart).Error; err != nil {
			t.Fatalf("reset local projection freshness: %v", err)
		}
	}
	if evalCfg.Agent.EmbeddingDimensions != config.RAGEmbeddingDimensions {
		t.Fatalf("agent embedding dimensions=%d, want %d", evalCfg.Agent.EmbeddingDimensions, config.RAGEmbeddingDimensions)
	}
	hybridCfg := evalCfg.RAG.Hybrid
	projection := ragservice.NewProjectionWithVersionLoader(
		db,
		ragservice.NewChunker(ragservice.ChunkerConfig{
			MaxTokens: evalCfg.RAG.Chunking.MaxTokens, OverlapTokens: evalCfg.RAG.Chunking.OverlapTokens,
			ChunkingVersion: evalCfg.RAG.Chunking.ChunkingVersion, TokenizerEncoding: evalCfg.RAG.Chunking.TokenizerEncoding,
		}),
		chunkEmbedder,
		searchProjection,
		baseService.NewVersionService(repository.NewVersionRepository(db), repository.NewContentRepository(db)),
		ragservice.ProjectionConfig{
			IndexVersion: evalCfg.RAG.Index.GenerationStart, EmbeddingModel: embeddingModel,
			EmbeddingDimensions: evalCfg.Agent.EmbeddingDimensions,
		},
	)
	var contentIDs []int64
	if err := db.WithContext(ctx).Table("content_items").Where("status = ? AND deleted_at IS NULL", "published").Order("id").Pluck("id", &contentIDs).Error; err != nil {
		t.Fatalf("list published content: %v", err)
	}
	for _, contentID := range contentIDs {
		if err := projection.SyncContent(ctx, contentID); err != nil {
			t.Fatalf("project content %d: %v", contentID, err)
		}
	}

	searchRepo := repository.NewSearchRepository(db)
	embeddingRepo := repository.NewEmbeddingRepository(db)
	retriever := ragservice.NewHybridRetriever(
		keywordSource,
		ragservice.NewPostgresKeywordRetriever(searchRepo),
		ragservice.NewPostgresVectorRetriever(embeddingRepo, embeddingModel),
		queryEmbedder,
		ragservice.NewDatabaseVisibilityFilter(db),
		hybridCfg,
	)
	datasetChecksum := hybridDatasetChecksum(t, cases)
	result, err := RunRetrievalEval(ctx, cases, func(ctx context.Context, query string, viewerID int64, _ int) ([]Retrieved, error) {
		got, err := retriever.Retrieve(ctx, query, viewerID)
		if err != nil {
			return nil, err
		}
		hits := make([]Retrieved, 0, len(got.Candidates))
		for rank, candidate := range got.Candidates {
			// The evaluator sorts by score to make provider output deterministic.
			// Preserve the retriever's already-fused order with an ordinal score;
			// the actual RRF score remains server-private and is never serialized.
			hits = append(hits, Retrieved{
				ContentID: candidate.ContentID, ContentVersion: int64(candidate.ContentVersion), Score: float64(len(got.Candidates) - rank),
				Evidence: &RetrievedEvidence{
					ChunkKey: candidate.ChunkKey, Title: candidate.Title, Heading: candidate.Heading,
					Text: candidate.Text, SourceStart: candidate.SourceStart, SourceEnd: candidate.SourceEnd,
				},
			})
		}
		return hits, nil
	}, hybridCfg.FinalTopK, RunSpec{
		RunKey:           "rag-hybrid-local-" + time.Now().UTC().Format("20060102T150405Z"),
		RetrieverVersion: "hybrid-rrf-pg-fallback-v1",
		ChunkingVersion:  strconv.Itoa(evalCfg.RAG.Chunking.ChunkingVersion),
		IndexVersion:     strconv.Itoa(evalCfg.RAG.Index.GenerationStart),
		DatasetChecksum:  datasetChecksum,
	}, Environment{
		GoVersion:             runtime.Version(),
		GOOS:                  runtime.GOOS,
		CorpusContents:        int64(len(contentIDs)),
		CorpusEmbeddings:      countChunkEmbeddings(t, db, embeddingModel),
		QueryEmbeddingModel:   embeddingModel,
		QueryEmbeddingStandin: embeddingMode == "standin",
		Note:                  hybridEnvironmentNote(embeddingMode, openSearchURL),
	})
	if err != nil {
		t.Fatalf("hybrid eval: %v", err)
	}
	keywordBaseline, vectorBaseline := runLocalBaselines(t, ctx, cases, db, queryEmbedder, embeddingMode == "standin", embeddingModel, hybridCfg.FinalTopK, datasetChecksum)
	t.Logf("hybrid before gate: cases=%d recall@10=%.3f mrr=%.3f ndcg@10=%.3f citation_precision=%.3f coverage=%.3f leak=%d p95=%.1fms degradation=%.3f corpus=%d chunks=%d", result.DatasetSize, result.Metrics.RecallAt10, result.Metrics.MRR, result.Metrics.NDCGAt10, result.Metrics.CitationPrecision, result.Metrics.CitationCoverage, result.Metrics.VisibilityLeakCount, result.Metrics.P95RetrievalMs, result.Metrics.DegradationSuccessRate, result.Environment.CorpusContents, result.Environment.CorpusEmbeddings)
	if gateErr := compareHybridMetrics(result.Metrics, keywordBaseline.Metrics, vectorBaseline.Metrics); gateErr != nil {
		if result.Environment.QueryEmbeddingStandin {
			t.Logf("hybrid metric gate NOT MET with deterministic embedding stand-in: %v", gateErr)
		} else {
			t.Fatalf("hybrid metric gate against same-run baselines: %v", gateErr)
		}
	}
	t.Logf("same-run baselines: keyword recall@10=%.3f mrr=%.3f ndcg@10=%.3f; vector recall@10=%.3f mrr=%.3f ndcg@10=%.3f", keywordBaseline.Metrics.RecallAt10, keywordBaseline.Metrics.MRR, keywordBaseline.Metrics.NDCGAt10, vectorBaseline.Metrics.RecallAt10, vectorBaseline.Metrics.MRR, vectorBaseline.Metrics.NDCGAt10)
	baselineData, err := os.ReadFile(baselineArtifactPath)
	if err != nil {
		t.Fatalf("read baseline artifact: %v", err)
	}
	baseline, err := ParseBaselineArtifact(baselineData)
	if err != nil {
		t.Fatalf("parse baseline artifact: %v", err)
	}
	if reportPath := strings.TrimSpace(os.Getenv(hybridDiagnosisReportEnv)); reportPath != "" {
		report := BuildCitationDiagnosisReport(result, CitationDiagnosisMetadata{
			RetrieverVersion:      "hybrid-rrf-pg-fallback-v1",
			HitEntity:             "chunk",
			CorpusScope:           "content_items.status=published AND deleted_at IS NULL",
			CorpusContents:        int64(len(contentIDs)),
			CorpusContentChecksum: contentIDChecksum(contentIDs),
			CorpusEmbeddings:      result.Environment.CorpusEmbeddings,
			RequestedTopK:         hybridCfg.FinalTopK,
			BM25TopK:              hybridCfg.BM25TopK,
			VectorTopK:            hybridCfg.VectorTopK,
			FinalTopK:             hybridCfg.FinalTopK,
			RRFK:                  hybridCfg.RRFK,
			DatasetChecksum:       datasetChecksum,
			ChunkingVersion:       strconv.Itoa(evalCfg.RAG.Chunking.ChunkingVersion),
			IndexVersion:          strconv.Itoa(evalCfg.RAG.Index.GenerationStart),
			EmbeddingModel:        embeddingModel,
			ProjectionGeneration:  evalCfg.RAG.Index.GenerationStart,
			QueryEmbeddingStandin: embeddingMode == "standin",
			CaseSampleLimit:       8,
		}, baseline)
		if err := WriteCitationDiagnosisReport(reportPath, report); err != nil {
			t.Fatalf("write citation diagnosis report: %v", err)
		}
		t.Logf("citation diagnosis report written to %s: raw_precision=%.3f dedup_precision=%.3f mean_hits=%.2f unique_hits=%.2f findings=%d", reportPath, report.CurrentMetrics.CitationPrecision, report.CurrentMetrics.DeduplicatedCitationPrecision, report.CurrentMetrics.MeanProducedHits, report.CurrentMetrics.MeanUniqueHits, len(report.Findings))
	}
	gateErr := compareHybridToBaseline(result, baseline)
	if gateErr != nil {
		if result.Environment.QueryEmbeddingStandin {
			t.Logf("hybrid metric gate NOT MET in local stand-in/fallback run: %v", gateErr)
		} else {
			t.Fatalf("hybrid metric gate: %v", gateErr)
		}
	}
	t.Logf("hybrid local: cases=%d recall@10=%.3f mrr=%.3f ndcg@10=%.3f citation_precision=%.3f coverage=%.3f leak=%d p95=%.1fms degradation=%.3f corpus=%d chunks=%d", result.DatasetSize, result.Metrics.RecallAt10, result.Metrics.MRR, result.Metrics.NDCGAt10, result.Metrics.CitationPrecision, result.Metrics.CitationCoverage, result.Metrics.VisibilityLeakCount, result.Metrics.P95RetrievalMs, result.Metrics.DegradationSuccessRate, result.Environment.CorpusContents, result.Environment.CorpusEmbeddings)
}

func contentIDChecksum(contentIDs []int64) string {
	values := make([]string, len(contentIDs))
	for i, contentID := range contentIDs {
		values[i] = strconv.FormatInt(contentID, 10)
	}
	return ChecksumOf([]byte(strings.Join(values, "\n")))
}

func loadHybridEvalConfig(t *testing.T) config.Config {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve hybrid eval test path")
	}
	v := viper.New()
	v.SetConfigFile(filepath.Join(filepath.Dir(file), "../../../config.yaml"))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read runtime config: %v", err)
	}
	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("decode runtime config: %v", err)
	}
	config.OverrideFromEnv(&cfg)
	return cfg
}

func hybridDatasetChecksum(t *testing.T, cases []model.EvalGoldenCase) string {
	t.Helper()
	payloads := make([]GoldenCasePayload, 0, len(cases))
	for _, c := range cases {
		payload, err := ModelCaseToPayload(c)
		if err != nil {
			t.Fatalf("convert golden case %s: %v", c.CaseKey, err)
		}
		payloads = append(payloads, payload)
	}
	body, err := GoldenSetBody(payloads)
	if err != nil {
		t.Fatalf("golden set body: %v", err)
	}
	return ChecksumOf(body)
}

func hybridEnvironmentNote(embeddingMode, openSearchURL string) string {
	if embeddingMode == "standin" && openSearchURL == "" {
		return "local development measurement; deterministic embedding stand-in; OpenSearch explicitly unavailable; do not use as production evidence"
	}
	if embeddingMode == "standin" {
		return "local development measurement; real local OpenSearch with deterministic embedding stand-in; do not use as production evidence"
	}
	return "provider-backed embedding measurement; verify credentials, corpus, and external service provenance before production claims"
}

func runLocalBaselines(t *testing.T, ctx context.Context, cases []model.EvalGoldenCase, db *gorm.DB, queryEmbedder ragservice.QueryEmbedder, standin bool, embeddingModel string, topK int, datasetChecksum string) (*RetrieverResult, *RetrieverResult) {
	t.Helper()
	env := Environment{GoVersion: runtime.Version(), GOOS: runtime.GOOS, QueryEmbeddingModel: embeddingModel, QueryEmbeddingStandin: standin, Note: "same-run baseline for hybrid comparison"}
	searchRepo := repository.NewSearchRepository(db)
	keyword, err := RunRetrievalEval(ctx, cases, (&ProductionKeywordAdapter{Search: searchRepo, DB: db}).Retrieve, topK, RunSpec{RunKey: "rag-hybrid-local-keyword", RetrieverVersion: "keyword-tsvector-simple-v1", DatasetChecksum: datasetChecksum}, env)
	if err != nil {
		t.Fatalf("same-run keyword baseline: %v", err)
	}
	vector, err := RunRetrievalEval(ctx, cases, (&ProductionVectorAdapter{Embeddings: repository.NewEmbeddingRepository(db), DB: db, Dims: config.RAGEmbeddingDimensions, QueryEmbedder: queryEmbedder}).Retrieve, topK, RunSpec{RunKey: "rag-hybrid-local-vector", RetrieverVersion: "vector-pgvector-ivfflat-v1", DatasetChecksum: datasetChecksum}, env)
	if err != nil {
		t.Fatalf("same-run vector baseline: %v", err)
	}
	return keyword, vector
}

func compareHybridMetrics(got, keyword, vector RetrieverMetrics) error {
	var failures []string
	check := func(name string, value, keywordValue, vectorValue float64) {
		want := keywordValue
		if vectorValue > want {
			want = vectorValue
		}
		if value < want {
			failures = append(failures, fmt.Sprintf("%s=%.3f < max(keyword=%.3f, vector=%.3f)", name, value, keywordValue, vectorValue))
		}
	}
	check("recall@10", got.RecallAt10, keyword.RecallAt10, vector.RecallAt10)
	check("mrr", got.MRR, keyword.MRR, vector.MRR)
	check("ndcg@10", got.NDCGAt10, keyword.NDCGAt10, vector.NDCGAt10)
	if got.VisibilityLeakCount != 0 {
		failures = append(failures, fmt.Sprintf("visibility leaks=%d", got.VisibilityLeakCount))
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func compareHybridToBaseline(result *RetrieverResult, baseline *BaselineArtifact) error {
	if baseline == nil || len(baseline.Retrievers) != 2 {
		return fmt.Errorf("baseline must contain keyword and vector retrievers")
	}
	if result.DatasetSize != baseline.DatasetSize || result.DatasetChecksum != baseline.DatasetChecksum {
		return fmt.Errorf("dataset identity mismatch: size=%d baseline=%d checksum=%q baseline=%q", result.DatasetSize, baseline.DatasetSize, result.DatasetChecksum, baseline.DatasetChecksum)
	}
	var keyword, vector RetrieverMetrics
	var haveKeyword, haveVector bool
	for _, retriever := range baseline.Retrievers {
		switch retriever.RetrieverVersion {
		case "keyword-tsvector-simple-v1":
			if retriever.ChunkingVersion != "none" || retriever.IndexVersion != "content-search-index-v1" {
				return fmt.Errorf("keyword baseline identity mismatch: chunking=%q index=%q", retriever.ChunkingVersion, retriever.IndexVersion)
			}
			keyword = retriever.Metrics
			haveKeyword = true
		case "vector-pgvector-ivfflat-v1":
			if retriever.ChunkingVersion != "none" || retriever.IndexVersion != "content-embeddings-v1" {
				return fmt.Errorf("vector baseline identity mismatch: chunking=%q index=%q", retriever.ChunkingVersion, retriever.IndexVersion)
			}
			vector = retriever.Metrics
			haveVector = true
		}
	}
	if !haveKeyword || !haveVector {
		return fmt.Errorf("baseline identities missing: keyword=%t vector=%t", haveKeyword, haveVector)
	}
	for _, retriever := range baseline.Retrievers {
		if retriever.Environment.CorpusContents != 0 && result.Environment.CorpusContents != 0 && retriever.Environment.CorpusContents != result.Environment.CorpusContents {
			return fmt.Errorf("baseline corpus=%d, current corpus=%d", retriever.Environment.CorpusContents, result.Environment.CorpusContents)
		}
		if retriever.Environment.QueryEmbeddingStandin != result.Environment.QueryEmbeddingStandin {
			return fmt.Errorf("baseline embedding standin=%t, current=%t", retriever.Environment.QueryEmbeddingStandin, result.Environment.QueryEmbeddingStandin)
		}
	}
	return compareHybridMetrics(result.Metrics, keyword, vector)
}

func countChunkEmbeddings(t *testing.T, db *gorm.DB, embeddingModel string) int64 {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT count(*) FROM chunk_embeddings WHERE embedding_model = ?", embeddingModel).Scan(&count).Error; err != nil {
		t.Fatalf("count chunk embeddings: %v", err)
	}
	return count
}
