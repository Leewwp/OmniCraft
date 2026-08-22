package rageval

import (
	"context"
	"fmt"
	"sort"
	"time"

	"omnicraft/backend/internal/model"
)

// Retrieved is one ranked retrieval hit.
type Retrieved struct {
	ContentID      int64   `json:"content_id"`
	ContentVersion int64   `json:"content_version,omitempty"` // latest version at eval time; 0 when unknown
	Score          float64 `json:"score"`
	// Evidence is retained only for opt-in local diagnosis reports. It is
	// excluded from committed evaluation artifacts and public contracts.
	Evidence *RetrievedEvidence `json:"-"`
}

// RetrievedEvidence carries the server-owned chunk fields needed to inspect a
// citation mismatch without exposing provider responses or internal scores.
type RetrievedEvidence struct {
	ChunkKey    string
	Title       string
	Heading     string
	Text        string
	SourceStart int
	SourceEnd   int
}

// Retriever produces the ranked retrieval list for one query. The keyword
// baseline adapter passes the viewer context into the production visibility
// scope; the vector baseline adapter ignores it (the existing raw pgvector
// path has no visibility predicates — that is exactly what the baseline
// measures).
type Retriever func(ctx context.Context, query string, viewerID int64, topK int) ([]Retrieved, error)

// RunSpec carries the version identity written to eval_runs and the artifact.
type RunSpec struct {
	RunKey           string
	RetrieverVersion string
	ChunkingVersion  string
	IndexVersion     string
	ArtifactPath     string
	DatasetChecksum  string
}

// CaseResult is the per-case measurement for one retriever.
type CaseResult struct {
	CaseKey           string      `json:"case_key"`
	Query             string      `json:"query"`
	RetrievedIDs      []int64     `json:"retrieved_ids"`
	Retrieved         []Retrieved `json:"retrieved"`
	RecallAt10        float64     `json:"recall_at_10"`
	RecallAt20        float64     `json:"recall_at_20"`
	MRR               float64     `json:"mrr"`
	NDCGAt10          float64     `json:"ndcg_at_10"`
	CitationPrecision float64     `json:"citation_precision"`
	CitationCoverage  float64     `json:"citation_coverage"`
	LeakedIDs         []int64     `json:"leaked_ids"`
	LatencyMs         float64     `json:"latency_ms"`
	Success           bool        `json:"success"`
	// ExpectedCitations is used by the opt-in diagnosis report only.
	ExpectedCitations []Citation `json:"-"`
}

// RetrieverMetrics aggregates one retriever's numbers over the dataset.
// Groundedness and answer relevance stay nil for retrieval-only runs
// (answer_evaluated=false); answer-producing runs fill them in.
type RetrieverMetrics struct {
	RecallAt10             float64  `json:"recall_at_10"`
	RecallAt20             float64  `json:"recall_at_20"`
	MRR                    float64  `json:"mrr"`
	NDCGAt10               float64  `json:"ndcg_at_10"`
	CitationPrecision      float64  `json:"citation_precision"`
	CitationCoverage       float64  `json:"citation_coverage"`
	Groundedness           *float64 `json:"groundedness"`
	AnswerRelevance        *float64 `json:"answer_relevance"`
	AnswerEvaluated        bool     `json:"answer_evaluated"`
	VisibilityLeakCount    int      `json:"visibility_leak_count"`
	P95RetrievalMs         float64  `json:"p95_retrieval_ms"`
	MeanLatencyMs          float64  `json:"mean_latency_ms"`
	DegradationSuccessRate float64  `json:"degradation_success_rate"`
	CaseCount              int      `json:"case_count"`
}

// Environment is the run environment snapshot written into the artifact and
// eval_runs.environment.
type Environment struct {
	GoVersion             string `json:"go_version"`
	GOOS                  string `json:"goos"`
	CorpusContents        int64  `json:"corpus_contents"`
	CorpusEmbeddings      int64  `json:"corpus_embeddings"`
	QueryEmbeddingModel   string `json:"query_embedding_model"`
	QueryEmbeddingStandin bool   `json:"query_embedding_standin"`
	Note                  string `json:"note"`
}

// RetrieverResult is the full measurement of one retriever over the dataset.
type RetrieverResult struct {
	RunKey           string           `json:"run_key"`
	RetrieverVersion string           `json:"retriever_version"`
	ChunkingVersion  string           `json:"chunking_version"`
	IndexVersion     string           `json:"index_version"`
	DatasetChecksum  string           `json:"dataset_checksum"`
	DatasetSize      int              `json:"dataset_size"`
	Metrics          RetrieverMetrics `json:"metrics"`
	PerCase          []CaseResult     `json:"per_case"`
	Environment      Environment      `json:"environment"`
}

// caseInput is the parsed oracle for one golden case.
type caseInput struct {
	caseKey           string
	query             string
	viewerID          int64
	relevant          map[int64]bool
	forbidden         map[int64]bool
	expectedCitations map[Citation]bool
}

// RunRetrievalEval evaluates one retriever over the golden cases. Ranking is
// determinised after retrieval: equal scores are broken by content id
// ascending. A failed case is marked unsuccessful with zero metrics; the
// remaining cases still aggregate.
func RunRetrievalEval(ctx context.Context, cases []model.EvalGoldenCase, retrieve Retriever, topK int, spec RunSpec, env Environment) (*RetrieverResult, error) {
	inputs, err := toCaseInputs(cases)
	if err != nil {
		return nil, err
	}
	return runRetrievalEval(ctx, inputs, retrieve, topK, spec, env), nil
}

func toCaseInputs(cases []model.EvalGoldenCase) ([]caseInput, error) {
	inputs := make([]caseInput, 0, len(cases))
	for _, c := range cases {
		relevant, err := ParseInt64List(c.RelevantContentIDs)
		if err != nil {
			return nil, fmt.Errorf("case %q relevant_content_ids: %w", c.CaseKey, err)
		}
		forbidden, err := ParseInt64List(c.ForbiddenContentIDs)
		if err != nil {
			return nil, fmt.Errorf("case %q forbidden_content_ids: %w", c.CaseKey, err)
		}
		expected, err := ParseCitations(c.ExpectedCitations)
		if err != nil {
			return nil, fmt.Errorf("case %q expected_citations: %w", c.CaseKey, err)
		}
		vc, err := ParseViewerContext(c.ViewerContext)
		if err != nil {
			return nil, fmt.Errorf("case %q viewer_context: %w", c.CaseKey, err)
		}
		inputs = append(inputs, caseInput{
			caseKey:           c.CaseKey,
			query:             c.Query,
			viewerID:          vc.ViewerUserID,
			relevant:          toSet(relevant),
			forbidden:         toSet(forbidden),
			expectedCitations: citationsToSet(expected),
		})
	}
	return inputs, nil
}

func toSet(ids []int64) map[int64]bool {
	if len(ids) == 0 {
		return map[int64]bool{}
	}
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func citationsToSet(citations []Citation) map[Citation]bool {
	if len(citations) == 0 {
		return map[Citation]bool{}
	}
	set := make(map[Citation]bool, len(citations))
	for _, c := range citations {
		set[c] = true
	}
	return set
}

func runRetrievalEval(ctx context.Context, inputs []caseInput, retrieve Retriever, topK int, spec RunSpec, env Environment) *RetrieverResult {
	result := &RetrieverResult{
		RunKey:           spec.RunKey,
		RetrieverVersion: spec.RetrieverVersion,
		ChunkingVersion:  spec.ChunkingVersion,
		IndexVersion:     spec.IndexVersion,
		DatasetChecksum:  spec.DatasetChecksum,
		DatasetSize:      len(inputs),
		Environment:      env,
	}
	var (
		totalRecall10, totalRecall20, totalMRR, totalNDCG float64
		totalCitationPrecision, totalCitationCoverage     float64
		durations                                         []float64
		failed, leakCount                                 int
	)
	for _, in := range inputs {
		start := time.Now()
		err := runCaseRetrieval(ctx, in, retrieve, topK, result)
		latency := float64(time.Since(start).Microseconds()) / 1000.0
		cr := &result.PerCase[len(result.PerCase)-1]
		cr.LatencyMs = latency
		durations = append(durations, latency)
		if err != nil {
			failed++
			cr.Success = false
			continue
		}
		cr.Success = true
		totalRecall10 += cr.RecallAt10
		totalRecall20 += cr.RecallAt20
		totalMRR += cr.MRR
		totalNDCG += cr.NDCGAt10
		totalCitationPrecision += cr.CitationPrecision
		totalCitationCoverage += cr.CitationCoverage
		leakCount += len(cr.LeakedIDs)
	}
	metrics := &result.Metrics
	metrics.CaseCount = len(inputs)
	metrics.P95RetrievalMs = P95(durations)
	if len(durations) > 0 {
		var sum float64
		for _, d := range durations {
			sum += d
		}
		metrics.MeanLatencyMs = sum / float64(len(durations))
	}
	metrics.DegradationSuccessRate = DegradationSuccessRate(len(inputs), failed)
	successful := len(inputs) - failed
	if successful > 0 {
		metrics.RecallAt10 = totalRecall10 / float64(successful)
		metrics.RecallAt20 = totalRecall20 / float64(successful)
		metrics.MRR = totalMRR / float64(successful)
		metrics.NDCGAt10 = totalNDCG / float64(successful)
		metrics.CitationPrecision = totalCitationPrecision / float64(successful)
		metrics.CitationCoverage = totalCitationCoverage / float64(successful)
	}
	metrics.VisibilityLeakCount = leakCount
	metrics.AnswerEvaluated = false
	return result
}

// runCaseRetrieval executes one retriever call, computes the per-case
// metrics and appends the CaseResult. A non-nil error marks failure.
func runCaseRetrieval(ctx context.Context, in caseInput, retrieve Retriever, topK int, result *RetrieverResult) error {
	hits, err := retrieve(ctx, in.query, in.viewerID, topK)
	cr := CaseResult{CaseKey: in.caseKey, Query: in.query, ExpectedCitations: citationsFromSet(in.expectedCitations)}
	if err != nil {
		result.PerCase = append(result.PerCase, cr)
		return fmt.Errorf("retrieve %q: %w", in.caseKey, err)
	}
	hits = determinize(hits)
	ids := make([]int64, len(hits))
	for i, h := range hits {
		ids[i] = h.ContentID
	}
	cr.RetrievedIDs = ids
	cr.Retrieved = hits
	cr.RecallAt10 = RecallAtK(ids, in.relevant, 10)
	cr.RecallAt20 = RecallAtK(ids, in.relevant, 20)
	cr.MRR = MRR(ids, in.relevant)
	cr.NDCGAt10 = NDCGAt10(ids, in.relevant)
	produced := citationsOf(hits)
	cr.CitationPrecision = CitationPrecision(produced, in.expectedCitations)
	cr.CitationCoverage = CitationCoverage(produced, in.expectedCitations)
	cr.LeakedIDs = VisibilityLeaks(ids, in.forbidden)
	result.PerCase = append(result.PerCase, cr)
	return nil
}

func citationsFromSet(set map[Citation]bool) []Citation {
	if len(set) == 0 {
		return nil
	}
	out := make([]Citation, 0, len(set))
	for citation := range set {
		out = append(out, citation)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ContentID != out[j].ContentID {
			return out[i].ContentID < out[j].ContentID
		}
		return out[i].ContentVersion < out[j].ContentVersion
	})
	return out
}

// determinize imposes the deterministic tie-break: score descending, then
// content id ascending. The production retrievers only order by score, so
// equal-score rows would otherwise order nondeterministically.
func determinize(hits []Retrieved) []Retrieved {
	sorted := append([]Retrieved(nil), hits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score > sorted[j].Score
		}
		return sorted[i].ContentID < sorted[j].ContentID
	})
	return sorted
}

func citationsOf(hits []Retrieved) []Citation {
	out := make([]Citation, 0, len(hits))
	for _, h := range hits {
		out = append(out, Citation{ContentID: h.ContentID, ContentVersion: h.ContentVersion})
	}
	return out
}
