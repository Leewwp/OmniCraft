package rageval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"omnicraft/backend/internal/model"
)

// RetrieverV2 is the v2 retriever seam: it consumes the resolved viewer
// identity instead of a bare numeric id, so per-case principal switching
// (H2/§1.2) reaches the production visibility scope. Adapters wrap the v1
// Retriever by ignoring or mapping the identity.
type RetrieverV2 func(ctx context.Context, query string, viewer ViewerIdentity, topK int) ([]Retrieved, error)

// LayeredEvalOptions configures one layered run.
type LayeredEvalOptions struct {
	// Split filters by classification.split: "" keeps every case (annotated
	// or not), "dev"/"test" keep only that split. Legacy rows without a
	// split annotation survive only with IncludeNoSplitCases.
	Split               string
	IncludeNoSplitCases bool
	// ConfirmTestSplitRun is the H4 explicit flag: a "test" split run
	// without it is refused (final test runs once; A-04 tuning reads dev).
	ConfirmTestSplitRun bool
	// TopK is the retrieval depth (default 20).
	TopK int
	// ChunkSource optionally enables H5 span resolution: annotated
	// [start,end) spans are re-resolved onto the current chunking_version.
	ChunkSource ChunkSource
}

// LayeredCaseResult is one (case × principal) measurement.
type LayeredCaseResult struct {
	CaseKey       string `json:"case_key"`
	Query         string `json:"query"`
	SchemaVersion int    `json:"schema_version"`
	PrimaryLayer  string `json:"primary_layer"`
	GroupKey      string `json:"group_key"`
	Split         string `json:"split"`
	PrincipalKey  string `json:"principal_key"`
	// IPScope and ASCIIColon feed the ke sub-reports (contract §8: ip_scoped
	// sub-report; the ASCII-colon subset splits around the #319 fix).
	IPScope    *bool `json:"ip_scope,omitempty"`
	ASCIIColon bool  `json:"ascii_colon"`

	RetrievedIDs   []int64 `json:"retrieved_ids"`
	ExpectedIDs    []int64 `json:"expected_ids"`
	AcceptableIDs  []int64 `json:"acceptable_ids,omitempty"`
	ForbiddenIDs   []int64 `json:"forbidden_ids,omitempty"`
	RecallAt1      float64 `json:"recall_at_1"`
	RecallAt3      float64 `json:"recall_at_3"`
	RecallAt10     float64 `json:"recall_at_10"`
	RecallAt20     float64 `json:"recall_at_20"`
	MRR            float64 `json:"mrr"`
	GradedNDCGAt10 float64 `json:"graded_ndcg_at_10"`
	TrapHitAt10    []int64 `json:"trap_hit_at_10,omitempty"`
	LeakedIDs      []int64 `json:"leaked_ids,omitempty"`

	EvidenceTotal         int  `json:"evidence_total,omitempty"`
	EvidenceResolved      int  `json:"evidence_resolved,omitempty"`
	EvidenceSnapshotDrift int  `json:"evidence_snapshot_drift,omitempty"`
	EvidenceHitAt10       bool `json:"evidence_hit_at_10,omitempty"`

	LatencyMs float64 `json:"latency_ms"`
	Success   bool    `json:"success"`
	Error     string  `json:"error,omitempty"`
}

// LayerMetrics aggregates one reporting group with raw numerator/denominator
// and Wilson 95% intervals (contract §8: 分层独立报告，原始分子/分母 + 置信区
// 间，不给综合准确率). Recall rates micro-average over the expected tiers;
// MRR/nDCG average per case; trap and leak rates are case shares.
type LayerMetrics struct {
	GroupKey     string `json:"group_key"`
	PrimaryLayer string `json:"primary_layer"`
	SubKey       string `json:"sub_key,omitempty"`
	Split        string `json:"split,omitempty"`
	CaseCount    int    `json:"case_count"`
	Successful   int    `json:"successful"`

	RecallAt1  RateWithCI `json:"recall_at_1"`
	RecallAt3  RateWithCI `json:"recall_at_3"`
	RecallAt10 RateWithCI `json:"recall_at_10"`
	RecallAt20 RateWithCI `json:"recall_at_20"`
	MRR        RateWithCI `json:"mrr"`
	NDCGAt10   RateWithCI `json:"ndcg_at_10"`

	TrapHitAt10        RateWithCI `json:"trap_hit_at_10"`
	VisibilityLeakRate RateWithCI `json:"visibility_leak_rate"`
	EvidenceResolved   RateWithCI `json:"evidence_resolved"`
}

// LayeredRetrievalResult is the full v2 measurement of one retriever.
type LayeredRetrievalResult struct {
	RunKey           string              `json:"run_key"`
	RetrieverVersion string              `json:"retriever_version"`
	ChunkingVersion  string              `json:"chunking_version"`
	IndexVersion     string              `json:"index_version"`
	DatasetChecksum  string              `json:"dataset_checksum"`
	DatasetSize      int                 `json:"dataset_size"`
	Split            string              `json:"split"`
	Groups           []*LayerMetrics     `json:"groups"`
	PerCase          []LayeredCaseResult `json:"per_case"`
	Environment      Environment         `json:"environment"`
}

// layeredCaseInput is the parsed v2 oracle of one golden case.
type layeredCaseInput struct {
	caseKey        string
	query          string
	schemaVer      int
	classification Classification
	rubric         AnswerRubric
	expected       map[int64]bool
	acceptable     map[int64]bool
	forbidden      map[int64]bool
	spans          []EvidenceSpan
	principalKey   string
}

// RunLayeredRetrievalEval evaluates one retriever over the golden cases with
// the v2 contract: per-case principal resolution (visibility cases run both
// principals), three-tier scoring, per-layer grouping and the split gate.
func RunLayeredRetrievalEval(ctx context.Context, cases []model.EvalGoldenCase, registry PrincipalRegistry, retrieve RetrieverV2, opts LayeredEvalOptions, spec RunSpec, env Environment) (*LayeredRetrievalResult, error) {
	if err := ValidateTestSplitGate(opts.Split, opts.ConfirmTestSplitRun); err != nil {
		return nil, err
	}
	selected, err := FilterCasesBySplit(cases, opts.Split, opts.IncludeNoSplitCases)
	if err != nil {
		return nil, err
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}

	result := &LayeredRetrievalResult{
		RunKey:           spec.RunKey,
		RetrieverVersion: spec.RetrieverVersion,
		ChunkingVersion:  spec.ChunkingVersion,
		IndexVersion:     spec.IndexVersion,
		DatasetChecksum:  spec.DatasetChecksum,
		DatasetSize:      len(selected),
		Split:            strings.TrimSpace(opts.Split),
		Environment:      env,
	}
	groups := map[string]*layerAccumulator{}

	for _, c := range selected {
		input, err := parseLayeredCase(c)
		if err != nil {
			return nil, err
		}
		vc, err := ParseViewerContext(c.ViewerContext)
		if err != nil {
			return nil, fmt.Errorf("case %q viewer_context: %w", c.CaseKey, err)
		}
		principalKeys := PrincipalKeysForCase(input.classification.PrimaryLayer, vc.PrincipalKey)
		for _, key := range principalKeys {
			identity, err := ResolveViewerIdentity(ctx, ViewerContext{PrincipalKey: key}, registry)
			if err != nil {
				return nil, fmt.Errorf("case %q principal %q: %w", c.CaseKey, key, err)
			}
			start := time.Now()
			caseResult, err := runLayeredCase(ctx, input, key, identity, retrieve, topK, opts)
			caseResult.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
			if err != nil {
				caseResult.Success = false
				caseResult.Error = err.Error()
			} else {
				caseResult.Success = true
			}
			result.PerCase = append(result.PerCase, *caseResult)
			groupFor(groups, caseResult).add(caseResult)
		}
	}

	result.Groups = make([]*LayerMetrics, 0, len(groups))
	for key, acc := range groups {
		result.Groups = append(result.Groups, acc.metrics(key))
	}
	sort.Slice(result.Groups, func(i, j int) bool {
		if result.Groups[i].GroupKey != result.Groups[j].GroupKey {
			return result.Groups[i].GroupKey < result.Groups[j].GroupKey
		}
		return result.Groups[i].Split < result.Groups[j].Split
	})
	return result, nil
}

// parseLayeredCase reads the v2 oracle: classification (layer/split/strategy),
// the three tiers (expected citations, acceptable ids, forbidden ids) and the
// evidence spans.
func parseLayeredCase(c model.EvalGoldenCase) (layeredCaseInput, error) {
	cl, err := ParseClassificationV2(c.Classification)
	if err != nil {
		return layeredCaseInput{}, fmt.Errorf("case %q classification: %w", c.CaseKey, err)
	}
	rubric, err := ParseAnswerRubric(c.AnswerRubric)
	if err != nil {
		return layeredCaseInput{}, fmt.Errorf("case %q answer_rubric: %w", c.CaseKey, err)
	}
	citations, err := ParseCitations(c.ExpectedCitations)
	if err != nil {
		return layeredCaseInput{}, fmt.Errorf("case %q expected_citations: %w", c.CaseKey, err)
	}
	expected := make(map[int64]bool, len(citations))
	for _, citation := range citations {
		expected[citation.ContentID] = true
	}
	// Defensive fallback for v2-shaped rows still carrying the v1 relevant
	// list: an empty expected tier would silently zero every recall rate.
	if len(expected) == 0 {
		relevant, relErr := ParseInt64List(c.RelevantContentIDs)
		if relErr == nil {
			for _, id := range relevant {
				expected[id] = true
			}
		}
	}
	forbiddenList, err := ParseInt64List(c.ForbiddenContentIDs)
	if err != nil {
		return layeredCaseInput{}, fmt.Errorf("case %q forbidden_content_ids: %w", c.CaseKey, err)
	}
	spans, err := ParseEvidenceSpans(c.RelevantEvidence)
	if err != nil {
		return layeredCaseInput{}, fmt.Errorf("case %q relevant_evidence: %w", c.CaseKey, err)
	}
	return layeredCaseInput{
		caseKey:        c.CaseKey,
		query:          c.Query,
		schemaVer:      c.SchemaVersion,
		classification: cl,
		rubric:         rubric,
		expected:       expected,
		acceptable:     toSet(rubric.AcceptableContentIDs),
		forbidden:      toSet(forbiddenList),
		spans:          spans,
	}, nil
}

func runLayeredCase(ctx context.Context, in layeredCaseInput, principalKey string, identity ViewerIdentity, retrieve RetrieverV2, topK int, opts LayeredEvalOptions) (*LayeredCaseResult, error) {
	cr := &LayeredCaseResult{
		CaseKey:       in.caseKey,
		Query:         in.query,
		SchemaVersion: in.schemaVer,
		PrimaryLayer:  in.classification.Layer(),
		GroupKey:      groupKeyOf(in.classification, principalKey),
		Split:         in.classification.Split,
		PrincipalKey:  principalKey,
		IPScope:       in.classification.IPScope,
		ASCIIColon:    strings.Contains(in.query, ":"),
		ExpectedIDs:   sortedIDs(in.expected),
		AcceptableIDs: sortedIDs(in.acceptable),
		ForbiddenIDs:  sortedIDs(in.forbidden),
	}
	hits, err := retrieve(ctx, in.query, identity, topK)
	if err != nil {
		return cr, fmt.Errorf("retrieve %q: %w", in.caseKey, err)
	}
	hits = determinize(hits)
	ids := make([]int64, len(hits))
	for i, h := range hits {
		ids[i] = h.ContentID
	}
	cr.RetrievedIDs = ids
	cr.RecallAt1 = RecallAtK(ids, in.expected, 1)
	cr.RecallAt3 = RecallAtK(ids, in.expected, 3)
	cr.RecallAt10 = RecallAtK(ids, in.expected, 10)
	cr.RecallAt20 = RecallAtK(ids, in.expected, 20)
	cr.MRR = MRR(ids, in.expected)
	cr.GradedNDCGAt10 = GradedNDCGAtK(ids, GradedGains(in.expected, in.acceptable), 10)
	cr.TrapHitAt10 = TrapHitAtK(ids, in.forbidden, 10)
	cr.LeakedIDs = VisibilityLeaks(ids, in.forbidden)

	if opts.ChunkSource != nil {
		resolutions, err := ResolveEvidenceSpans(in.spans, opts.ChunkSource)
		if err != nil {
			return cr, fmt.Errorf("case %q evidence spans: %w", in.caseKey, err)
		}
		evidenceContent := map[int64]bool{}
		for _, resolution := range resolutions {
			cr.EvidenceTotal++
			if resolution.Degenerate {
				continue
			}
			if resolution.Resolved {
				cr.EvidenceResolved++
				evidenceContent[resolution.ContentID] = true
			}
			if resolution.SnapshotDrift {
				cr.EvidenceSnapshotDrift++
			}
		}
		cr.EvidenceHitAt10 = EvidenceContentHitAtK(ids, evidenceContent, 10)
	}
	return cr, nil
}

// groupKeyOf derives the reporting group: ke splits by ip_scoped/global, the
// no-answer layer by judge strategy, the visibility layer by principal;
// every other layer reports as a single group.
func groupKeyOf(cl Classification, principalKey string) string {
	layer := cl.Layer()
	switch layer {
	case LayerKnownItemExact:
		if cl.IPScope != nil && *cl.IPScope {
			return layer + "/ip_scoped"
		}
		return layer + "/global"
	case LayerNoAnswer:
		strategy := cl.NoAnswerStrategy
		if strategy == "" {
			strategy = "unannotated"
		}
		return layer + "/" + strategy
	case LayerVisibility:
		return layer + "/" + principalKey
	default:
		return layer
	}
}

// layerAccumulator micro-aggregates the raw numerators/denominators of one
// group across its cases.
type layerAccumulator struct {
	groupKey     string
	primaryLayer string
	subKey       string
	split        string
	caseCount    int
	successful   int

	recall1, recall3, recall10, recall20 rateAccumulator
	mrr                                  rateAccumulator
	ndcg10                               rateAccumulator
	trapHit                              rateAccumulator
	leak                                 rateAccumulator
	evidenceResolved                     rateAccumulator
}

func groupFor(groups map[string]*layerAccumulator, cr *LayeredCaseResult) *layerAccumulator {
	key := cr.GroupKey
	if cr.Split != "" {
		key += "@" + cr.Split
	}
	acc, ok := groups[key]
	if !ok {
		layer, sub := cr.PrimaryLayer, ""
		if idx := strings.Index(cr.GroupKey, "/"); idx >= 0 {
			layer, sub = cr.GroupKey[:idx], cr.GroupKey[idx+1:]
		}
		acc = &layerAccumulator{groupKey: cr.GroupKey, primaryLayer: layer, subKey: sub, split: cr.Split}
		groups[key] = acc
	}
	return acc
}

func (a *layerAccumulator) add(cr *LayeredCaseResult) {
	a.caseCount++
	if !cr.Success {
		return
	}
	a.successful++
	a.recall1.add(cr.RecallAt1*float64(len(cr.ExpectedIDs)), float64(len(cr.ExpectedIDs)))
	a.recall3.add(cr.RecallAt3*float64(len(cr.ExpectedIDs)), float64(len(cr.ExpectedIDs)))
	a.recall10.add(cr.RecallAt10*float64(len(cr.ExpectedIDs)), float64(len(cr.ExpectedIDs)))
	a.recall20.add(cr.RecallAt20*float64(len(cr.ExpectedIDs)), float64(len(cr.ExpectedIDs)))
	if len(cr.ExpectedIDs) > 0 {
		a.mrr.add(cr.MRR, 1)
	}
	if len(cr.ExpectedIDs) > 0 || len(cr.AcceptableIDs) > 0 {
		a.ndcg10.add(cr.GradedNDCGAt10, 1)
	}
	if len(cr.ForbiddenIDs) > 0 {
		a.trapHit.add(boolFloat(len(cr.TrapHitAt10) > 0), 1)
		// the leak rate is a visibility/hn metric: layers without a
		// forbidden tier contribute no denominator
		a.leak.add(boolFloat(len(cr.LeakedIDs) > 0), 1)
	}
	if cr.EvidenceTotal > 0 {
		a.evidenceResolved.add(float64(cr.EvidenceResolved), float64(cr.EvidenceTotal))
	}
}

func (a *layerAccumulator) metrics(groupKey string) *LayerMetrics {
	return &LayerMetrics{
		GroupKey:           a.groupKey,
		PrimaryLayer:       a.primaryLayer,
		SubKey:             a.subKey,
		Split:              a.split,
		CaseCount:          a.caseCount,
		Successful:         a.successful,
		RecallAt1:          a.recall1.rate(),
		RecallAt3:          a.recall3.rate(),
		RecallAt10:         a.recall10.rate(),
		RecallAt20:         a.recall20.rate(),
		MRR:                a.mrr.rate(),
		NDCGAt10:           a.ndcg10.rate(),
		TrapHitAt10:        a.trapHit.rate(),
		VisibilityLeakRate: a.leak.rate(),
		EvidenceResolved:   a.evidenceResolved.rate(),
	}
}

func sortedIDs(set map[int64]bool) []int64 {
	if len(set) == 0 {
		return nil
	}
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
