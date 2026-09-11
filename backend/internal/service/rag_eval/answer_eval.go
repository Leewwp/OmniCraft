package rageval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// answerEvalSchemaVersion identifies the answer-evaluation artifact contract.
// The artifact is a redacted local measurement: it must never carry provider
// credentials, raw provider payloads, or internal prompt bodies.
const answerEvalSchemaVersion = "agent-answer-eval-v1"

// Answer-eval case status buckets. `answered` cases form the effective
// denominator for groundedness/relevance/latency aggregates; no-evidence,
// degraded and provider-error cases are reported separately instead of being
// silently counted as zeros.
const (
	AnswerStatusAnswered   = "answered"
	AnswerStatusNoEvidence = "no_evidence"
	AnswerStatusDegraded   = "degraded"
	AnswerStatusProvError  = "provider_error"
)

// AnswerEvalCitation is the redacted citation identity recorded per case.
// It mirrors the server-owned AgentCitation contract without internal scores.
type AnswerEvalCitation struct {
	ContentID      int64  `json:"content_id"`
	ContentVersion int    `json:"content_version"`
	ChunkKey       string `json:"chunk_key,omitempty"`
	ChunkIndex     int    `json:"chunk_index,omitempty"`
	Title          string `json:"title"`
	Zone           string `json:"zone"`
	Source         string `json:"source,omitempty"`
}

// AnswerEvalCaseResult is one golden case's answer measurement. Groundedness
// and answer relevance stay nil for cases outside the `answered` bucket so
// the summary can use effective denominators.
type AnswerEvalCaseResult struct {
	CaseKey          string               `json:"case_key"`
	Query            string               `json:"query"`
	QueryLanguage    string               `json:"query_language"`
	Status           string               `json:"status"`
	AnswerKind       string               `json:"answer_kind,omitempty"`
	Answer           string               `json:"answer,omitempty"`
	Attempts         int                  `json:"attempts"`
	DirectAnswers    int                  `json:"direct_answers"`
	ToolCalls        int                  `json:"tool_calls"`
	Citations        []AnswerEvalCitation `json:"citations,omitempty"`
	EvidenceChunks   int                  `json:"evidence_chunks"`
	EvidenceLookups  int                  `json:"evidence_lookups"`
	Groundedness     *float64             `json:"groundedness"`
	AnswerRelevance  *float64             `json:"answer_relevance"`
	FirstTokenMs     *int64               `json:"first_token_ms"`
	TotalMs          int64                `json:"total_ms"`
	PromptTokens     *int                 `json:"prompt_tokens"`
	CompletionTokens *int                 `json:"completion_tokens"`
	Degraded         bool                 `json:"degraded"`
	DegradedReason   string               `json:"degraded_reason,omitempty"`
	ErrorCode        string               `json:"error_code,omitempty"`

	// ToolSteps mirrors the SSE tool-step events the client saw (display-safe
	// summaries only). It is the fourth visibility leak surface (H7).
	ToolSteps []ToolStepRecord `json:"tool_steps,omitempty"`

	// RetrievedIDs are the content ids the retrieval tool actually returned
	// for the final attempt (visibility leak surface 1 and the no-answer
	// judge's recommendation-truth input). Empty when retrieval never ran.
	RetrievedIDs []int64 `json:"retrieved_ids,omitempty"`

	// v2 annotation fields (H2/H6/H7): the principal the case ran under, its
	// layer/split, the dual-strategy no-answer judge outcome and the
	// four-surface visibility leak accounting. v1 runs leave them zero.
	PrincipalKey     string                `json:"principal_key,omitempty"`
	PrimaryLayer     string                `json:"primary_layer,omitempty"`
	Split            string                `json:"split,omitempty"`
	NoAnswerStrategy string                `json:"no_answer_strategy,omitempty"`
	NoAnswer         *NoAnswerJudgeResult  `json:"no_answer,omitempty"`
	VisibilityLeak   *VisibilityLeakReport `json:"visibility_leak,omitempty"`
}

// AnswerEvalMetadata records the run identity required to reproduce or audit
// the measurement. It intentionally contains no credential material.
type AnswerEvalMetadata struct {
	RanAt          string `json:"ran_at"`
	GitCommit      string `json:"git_commit,omitempty"`
	Provider       string `json:"provider"`
	ChatModel      string `json:"chat_model"`
	EmbeddingModel string `json:"embedding_model"`
	// Split-wiring identity (canonical profile: minimax chat +
	// openai_compat/DashScope embeddings). Endpoints are host bases only,
	// never credentials.
	EmbeddingProvider     string          `json:"embedding_provider,omitempty"`
	ChatAPIBase           string          `json:"chat_api_base,omitempty"`
	EmbeddingAPIBase      string          `json:"embedding_api_base,omitempty"`
	EmbeddingDimensions   int             `json:"embedding_dimensions,omitempty"`
	FeatureFlags          map[string]bool `json:"feature_flags,omitempty"`
	QueryEmbeddingStandin bool            `json:"query_embedding_standin"`
	RetrieverVersion      string          `json:"retriever_version"`
	KeywordSource         string          `json:"keyword_source"`
	DatasetChecksum       string          `json:"dataset_checksum"`
	CorpusContentChecksum string          `json:"corpus_content_checksum"`
	CorpusContents        int64           `json:"corpus_contents"`
	CorpusEmbeddings      int64           `json:"corpus_embeddings"`
	ProjectionGeneration  int             `json:"projection_generation"`
	ChunkingVersion       string          `json:"chunking_version"`
	IndexVersion          string          `json:"index_version"`
	FinalTopK             int             `json:"final_top_k"`
	GoVersion             string          `json:"go_version"`
	ChatViewerUserID      int64           `json:"chat_viewer_user_id"`
	// PrincipalMode discloses how chat identity was chosen: "global-v1" (one
	// seeded user for every case) or "per-case-v2" (H2: each case runs its
	// annotated principal; anonymous principals chat as the seeded
	// anonymous-equivalent user).
	PrincipalMode string `json:"principal_mode,omitempty"`
	Note          string `json:"note"`
}

// AnswerEvalSummary aggregates the per-case results with effective
// denominators. Latency percentiles and token totals only cover `answered`
// cases; no-evidence/degraded/provider-error buckets are counted separately.
type AnswerEvalSummary struct {
	Cases                 int      `json:"cases"`
	Answered              int      `json:"answered"`
	NoEvidence            int      `json:"no_evidence"`
	Degraded              int      `json:"degraded"`
	ProviderErrors        int      `json:"provider_errors"`
	Groundedness          *float64 `json:"groundedness_mean"`
	AnswerRelevance       *float64 `json:"answer_relevance_mean"`
	FirstTokenP50Ms       *float64 `json:"first_token_p50_ms"`
	FirstTokenP95Ms       *float64 `json:"first_token_p95_ms"`
	TotalP50Ms            *float64 `json:"total_p50_ms"`
	TotalP95Ms            *float64 `json:"total_p95_ms"`
	PromptTokensTotal     int64    `json:"prompt_tokens_total"`
	CompletionTokensTotal int64    `json:"completion_tokens_total"`
	TokensPerAnswerMean   *float64 `json:"tokens_per_answer_mean"`
	TotalAttempts         int      `json:"total_attempts"`
	DirectAnswerAttempts  int      `json:"direct_answer_attempts"`
	DirectAnswerRate      *float64 `json:"direct_answer_rate"`
}

// AnswerEvalReport is the redacted artifact for one answer-producing run.
type AnswerEvalReport struct {
	SchemaVersion string                 `json:"schema_version"`
	Metadata      AnswerEvalMetadata     `json:"metadata"`
	Summary       AnswerEvalSummary      `json:"summary"`
	Cases         []AnswerEvalCaseResult `json:"cases"`
	// Layers carries the v2 per-layer answer aggregates when the run used
	// annotated v2 cases (na/vi answer-layer metrics); v1 runs omit it.
	Layers map[string]AnswerLayerSummary `json:"layers,omitempty"`
}

// AnswerLayerSummary aggregates the v2 answer-layer metrics of one layer
// group (contract §8: na 回答层 strict=拒答率+零编造、related=诚实声明率+推荐
// 真实性；vi 四泄漏面泄漏率=0 硬门).
type AnswerLayerSummary struct {
	GroupKey         string     `json:"group_key"`
	CaseCount        int        `json:"case_count"`
	Answered         int        `json:"answered"`
	NoEvidence       int        `json:"no_evidence"`
	Degraded         int        `json:"degraded"`
	ProviderErrors   int        `json:"provider_errors"`
	NoAnswerPass     RateWithCI `json:"no_answer_pass"`
	NoAnswerHardFail RateWithCI `json:"no_answer_hard_fail"`
	RefusalRate      RateWithCI `json:"refusal_rate"`
	HonestDisclaimer RateWithCI `json:"honest_disclaimer_rate"`
	LeakFreeRate     RateWithCI `json:"leak_free_rate"`
}

// SplitAnswerSentences splits an answer into sentences for deterministic
// groundedness containment. CJK sentence marks and Latin terminators are
// both boundaries; newlines always separate. Empty fragments are dropped.
func SplitAnswerSentences(answer string) []string {
	sentences := make([]string, 0, 8)
	current := strings.Builder{}
	flush := func() {
		s := strings.TrimSpace(current.String())
		current.Reset()
		if s != "" {
			sentences = append(sentences, s)
		}
	}
	for _, r := range answer {
		switch r {
		case '。', '！', '？', '；', '!', '?', ';', '\n', '\r':
			flush()
		case '.':
			// Keep decimal points inside numbers (3.5) as one sentence.
			if current.Len() > 0 {
				prev := lastRune(current.String())
				if unicode.IsDigit(prev) {
					current.WriteRune(r)
					continue
				}
			}
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return sentences
}

func lastRune(s string) rune {
	runes := []rune(s)
	if len(runes) == 0 {
		return 0
	}
	return runes[len(runes)-1]
}

// ScoreAnswerCase computes the deterministic answer metrics for one case.
// Groundedness is the verbatim sentence-containment share against the
// evidence texts the citations resolved to; answer relevance is the lexical
// query-overlap proxy. Both stay nil when the case produced no scoreable
// answer (empty answer, no citations, or no resolvable evidence).
func ScoreAnswerCase(answer, query string, evidenceTexts []string) (grounded *float64, relevance *float64, sentenceCount int) {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" || len(evidenceTexts) == 0 {
		return nil, nil, 0
	}
	sentences := SplitAnswerSentences(trimmed)
	if len(sentences) == 0 {
		return nil, nil, 0
	}
	g := Groundedness(sentences, evidenceTexts)
	r := AnswerRelevance(trimmed, query)
	return &g, &r, len(sentences)
}

// BuildAnswerEvalReport computes the summary aggregates. Groundedness and
// answer relevance use only scoreable (answered with evidence) cases as the
// denominator; latency and token aggregates use answered cases only.
func BuildAnswerEvalReport(meta AnswerEvalMetadata, cases []AnswerEvalCaseResult) *AnswerEvalReport {
	summary := AnswerEvalSummary{Cases: len(cases)}
	var (
		groundedVals   []float64
		relevanceVals  []float64
		firstTokenVals []float64
		totalVals      []float64
		tokensTotal    float64
		tokenCases     int
	)
	for i := range cases {
		c := &cases[i]
		summary.TotalAttempts += c.Attempts
		summary.DirectAnswerAttempts += c.DirectAnswers
		switch c.Status {
		case AnswerStatusAnswered:
			summary.Answered++
			if c.Groundedness != nil {
				groundedVals = append(groundedVals, *c.Groundedness)
			}
			if c.AnswerRelevance != nil {
				relevanceVals = append(relevanceVals, *c.AnswerRelevance)
			}
			if c.FirstTokenMs != nil {
				firstTokenVals = append(firstTokenVals, float64(*c.FirstTokenMs))
			}
			totalVals = append(totalVals, float64(c.TotalMs))
			if c.PromptTokens != nil && c.CompletionTokens != nil {
				summary.PromptTokensTotal += int64(*c.PromptTokens)
				summary.CompletionTokensTotal += int64(*c.CompletionTokens)
				tokensTotal += float64(*c.PromptTokens + *c.CompletionTokens)
				tokenCases++
			}
		case AnswerStatusNoEvidence:
			summary.NoEvidence++
		case AnswerStatusDegraded:
			summary.Degraded++
		case AnswerStatusProvError:
			summary.ProviderErrors++
		}
	}
	summary.Groundedness = meanOf(groundedVals)
	summary.AnswerRelevance = meanOf(relevanceVals)
	if len(firstTokenVals) > 0 {
		p50 := percentileOf(firstTokenVals, 0.50)
		p95 := P95(firstTokenVals)
		summary.FirstTokenP50Ms = &p50
		summary.FirstTokenP95Ms = &p95
	}
	if len(totalVals) > 0 {
		p50 := percentileOf(totalVals, 0.50)
		p95 := P95(totalVals)
		summary.TotalP50Ms = &p50
		summary.TotalP95Ms = &p95
	}
	if tokenCases > 0 {
		mean := tokensTotal / float64(tokenCases)
		summary.TokensPerAnswerMean = &mean
	}
	if summary.TotalAttempts > 0 {
		rate := float64(summary.DirectAnswerAttempts) / float64(summary.TotalAttempts)
		summary.DirectAnswerRate = &rate
	}
	return &AnswerEvalReport{
		SchemaVersion: answerEvalSchemaVersion,
		Metadata:      meta,
		Summary:       summary,
		Cases:         cases,
		Layers:        BuildLayeredAnswerSummary(cases),
	}
}

// BuildLayeredAnswerSummary aggregates the v2 answer-layer metrics per
// primary-layer group. It returns nil when no case carries a layer
// annotation (v1 runs), so the artifact shape stays v1 for v1 data.
func BuildLayeredAnswerSummary(cases []AnswerEvalCaseResult) map[string]AnswerLayerSummary {
	type layerAcc struct {
		summary          AnswerLayerSummary
		noAnswerTotal    float64
		noAnswerPass     float64
		noAnswerHardFail float64
		strictTotal      float64
		refused          float64
		relatedTotal     float64
		disclaimed       float64
		leakTotal        float64
		leakFree         float64
	}
	accs := map[string]*layerAcc{}
	for i := range cases {
		c := &cases[i]
		if c.PrimaryLayer == "" {
			continue
		}
		key := c.PrimaryLayer
		if c.NoAnswerStrategy != "" {
			key += "/" + c.NoAnswerStrategy
		}
		acc := accs[key]
		if acc == nil {
			acc = &layerAcc{}
			acc.summary.GroupKey = key
			accs[key] = acc
		}
		acc.summary.CaseCount++
		switch c.Status {
		case AnswerStatusAnswered:
			acc.summary.Answered++
		case AnswerStatusNoEvidence:
			acc.summary.NoEvidence++
		case AnswerStatusDegraded:
			acc.summary.Degraded++
		case AnswerStatusProvError:
			acc.summary.ProviderErrors++
		}
		if c.NoAnswer != nil {
			acc.noAnswerTotal++
			if c.NoAnswer.Pass {
				acc.noAnswerPass++
			}
			if c.NoAnswer.HardFail {
				acc.noAnswerHardFail++
			}
			switch c.NoAnswer.Strategy {
			case NoAnswerStrictNotFound:
				acc.strictTotal++
				if c.NoAnswer.Refused {
					acc.refused++
				}
			case NoAnswerRelatedRecommendationOK:
				acc.relatedTotal++
				if c.NoAnswer.HonestDisclaimer {
					acc.disclaimed++
				}
			}
		}
		if c.VisibilityLeak != nil {
			acc.leakTotal++
			if c.VisibilityLeak.AllZero {
				acc.leakFree++
			}
		}
	}
	if len(accs) == 0 {
		return nil
	}
	out := make(map[string]AnswerLayerSummary, len(accs))
	for key, acc := range accs {
		s := acc.summary
		s.NoAnswerPass = NewRate(acc.noAnswerPass, acc.noAnswerTotal)
		s.NoAnswerHardFail = NewRate(acc.noAnswerHardFail, acc.noAnswerTotal)
		s.RefusalRate = NewRate(acc.refused, acc.strictTotal)
		s.HonestDisclaimer = NewRate(acc.disclaimed, acc.relatedTotal)
		s.LeakFreeRate = NewRate(acc.leakFree, acc.leakTotal)
		out[key] = s
	}
	return out
}

func meanOf(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	return &mean
}

func percentileOf(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
}

// WriteAnswerEvalReport persists the redacted artifact. The struct itself has
// no credential field; this writer additionally refuses paths inside the
// repository's backend/.env location as a belt-and-suspenders guard.
func WriteAnswerEvalReport(path string, report *AnswerEvalReport) error {
	if report == nil {
		return fmt.Errorf("answer eval report is nil")
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("answer eval report path is empty")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal answer eval report: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create answer eval report directory: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write answer eval report: %w", err)
	}
	return nil
}
