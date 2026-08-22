package rageval

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"omnicraft/backend/internal/model"
)

// Citation identifies one produced or expected citation by content id and
// version. A produced citation with ContentVersion 0 (version unknown, e.g.
// corpus rows without a content_versions row) matches any expected citation
// of the same content id.
type Citation struct {
	ContentID      int64 `json:"content_id"`
	ContentVersion int64 `json:"content_version"`
}

// EvidenceSpan mirrors one relevant_evidence entry: the annotated evidence
// span on (content_id, content_version), resolved to chunk keys under the
// current chunking_version at eval time.
type EvidenceSpan struct {
	ContentID      int64 `json:"content_id"`
	ContentVersion int64 `json:"content_version"`
	SourceStart    int   `json:"source_start"`
	SourceEnd      int   `json:"source_end"`
}

// ViewerContext mirrors the viewer_context jsonb payload: 0 means anonymous.
type ViewerContext struct {
	ViewerUserID int64 `json:"viewer_user_id"`
}

// Classification mirrors the classification jsonb payload: content type
// (mod/guide/article/...) and query popularity band (cold/hot/normal).
type Classification struct {
	ContentType string `json:"content_type"`
	Popularity  string `json:"popularity"`
}

// ParseInt64List reads a jsonb integer array (relevant/forbidden content
// ids) into a sorted deduped slice.
func ParseInt64List(raw model.JSONB) ([]int64, error) {
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	dedup := ids[:0]
	var last int64
	for i, id := range ids {
		if i == 0 || id != last {
			dedup = append(dedup, id)
			last = id
		}
	}
	return dedup, nil
}

// ParseCitations reads a jsonb citation array.
func ParseCitations(raw model.JSONB) ([]Citation, error) {
	var citations []Citation
	if err := json.Unmarshal(raw, &citations); err != nil {
		return nil, err
	}
	return citations, nil
}

// ParseEvidenceSpans reads a jsonb relevant_evidence array.
func ParseEvidenceSpans(raw model.JSONB) ([]EvidenceSpan, error) {
	var spans []EvidenceSpan
	if err := json.Unmarshal(raw, &spans); err != nil {
		return nil, err
	}
	return spans, nil
}

// ParseViewerContext reads the viewer_context jsonb payload.
func ParseViewerContext(raw model.JSONB) (ViewerContext, error) {
	var vc ViewerContext
	if err := json.Unmarshal(raw, &vc); err != nil {
		return vc, err
	}
	return vc, nil
}

// ParseClassification reads the classification jsonb payload.
func ParseClassification(raw model.JSONB) (Classification, error) {
	var c Classification
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	return c, nil
}

// RecallAtK is the share of relevant content ids present in the top-k
// ranked list. A case with no relevant ids scores 0.
func RecallAtK(ranked []int64, relevant map[int64]bool, k int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	hit := 0
	for i, id := range ranked {
		if i >= k {
			break
		}
		if relevant[id] {
			hit++
		}
	}
	return float64(hit) / float64(len(relevant))
}

// MRR is the reciprocal rank of the first relevant content, 0 when none.
func MRR(ranked []int64, relevant map[int64]bool) float64 {
	for i, id := range ranked {
		if relevant[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// NDCGAt10 is the discounted cumulative gain at 10 normalised by the ideal
// ordering over the relevant set (binary relevance).
func NDCGAt10(ranked []int64, relevant map[int64]bool) float64 {
	relevantCount := 0
	for _, id := range ranked {
		if relevant[id] {
			relevantCount++
		}
	}
	if relevantCount == 0 {
		return 0
	}
	k := len(ranked)
	if k > 10 {
		k = 10
	}
	var dcg float64
	for i := 0; i < k; i++ {
		if relevant[ranked[i]] {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}
	var idcg float64
	for i := 0; i < relevantCount; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	return dcg / idcg
}

// CitationPrecision is the share of produced citations that are expected. An
// empty produced list scores 1 (nothing wrong produced).
func CitationPrecision(produced []Citation, expected map[Citation]bool) float64 {
	if len(produced) == 0 {
		return 1
	}
	hit := 0
	for _, c := range produced {
		if expected[citeWithVersion(c, expected)] {
			hit++
		}
	}
	return float64(hit) / float64(len(produced))
}

// DeduplicateCitations collapses repeated chunk hits to one content/version
// identity while preserving the first-ranked citation.
func DeduplicateCitations(produced []Citation) []Citation {
	if len(produced) == 0 {
		return nil
	}
	seen := make(map[Citation]bool, len(produced))
	out := make([]Citation, 0, len(produced))
	for _, citation := range produced {
		if seen[citation] {
			continue
		}
		seen[citation] = true
		out = append(out, citation)
	}
	return out
}

// CitationPrecisionDeduplicated measures content-level precision for a
// chunk-level result. It is separate from CitationPrecision so historical
// raw-hit metrics remain unchanged.
func CitationPrecisionDeduplicated(produced []Citation, expected map[Citation]bool) float64 {
	return CitationPrecision(DeduplicateCitations(produced), expected)
}

// CitationCoverageDeduplicated measures content-level coverage after repeated
// chunk hits have been collapsed.
func CitationCoverageDeduplicated(produced []Citation, expected map[Citation]bool) float64 {
	return CitationCoverage(DeduplicateCitations(produced), expected)
}

// CitationCoverage is the share of expected citations produced at least
// once. An empty expected set scores 1 when nothing was produced and 0
// otherwise.
func CitationCoverage(produced []Citation, expected map[Citation]bool) float64 {
	if len(expected) == 0 {
		if len(produced) == 0 {
			return 1
		}
		return 0
	}
	producedSet := make(map[Citation]bool, len(produced))
	for _, p := range produced {
		producedSet[citeWithVersion(p, expected)] = true
	}
	covered := 0
	for c := range expected {
		if producedSet[c] {
			covered++
		}
	}
	return float64(covered) / float64(len(expected))
}

// citeWithVersion normalises an unknown version (0) to any expected citation
// of the same content id for matching purposes.
func citeWithVersion(p Citation, expected map[Citation]bool) Citation {
	if p.ContentVersion != 0 {
		return p
	}
	for c := range expected {
		if c.ContentID == p.ContentID {
			return c
		}
	}
	return p
}

// VisibilityLeaks returns the sorted, deduped forbidden content ids that
// appear in the produced list.
func VisibilityLeaks(produced []int64, forbidden map[int64]bool) []int64 {
	var leaks []int64
	seen := map[int64]bool{}
	for _, id := range produced {
		if forbidden[id] && !seen[id] {
			leaks = append(leaks, id)
			seen[id] = true
		}
	}
	sort.Slice(leaks, func(i, j int) bool { return leaks[i] < leaks[j] })
	return leaks
}

// P95 returns the 95th percentile of the collected per-query durations in
// milliseconds (0 when empty).
func P95(durations []float64) float64 {
	if len(durations) == 0 {
		return 0
	}
	sorted := append([]float64(nil), durations...)
	sort.Float64s(sorted)
	idx := int(math.Ceil(0.95*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

// DegradationSuccessRate is the share of queries that returned successfully
// under the measured conditions (1 when no queries ran).
func DegradationSuccessRate(total, failed int) float64 {
	if total == 0 {
		return 1
	}
	return 1 - float64(failed)/float64(total)
}

// Groundedness is the deterministic share of answer sentences that appear
// verbatim in at least one evidence text (normalised by trimming whitespace
// and trailing punctuation). Judge-based scoring is layered on top of this
// in answer-producing runs.
func Groundedness(sentences []string, evidenceTexts []string) float64 {
	if len(sentences) == 0 {
		return 1
	}
	normalized := make([]string, len(evidenceTexts))
	for i, e := range evidenceTexts {
		normalized[i] = normalizeEvidence(e)
	}
	supported := 0
	for _, s := range sentences {
		norm := normalizeEvidence(s)
		if norm == "" {
			continue
		}
		for _, e := range normalized {
			if e != "" && (len(e) >= len(norm)) && containsNormalized(e, norm) {
				supported++
				break
			}
		}
	}
	return float64(supported) / float64(len(sentences))
}

// AnswerRelevance is a deterministic lexical proxy (token overlap with the
// query) used until judge-based scoring is available; it never replaces the
// judge in answer-producing runs. CJK tokens also match as substrings
// because Chinese text has no spaces and the query term usually appears
// verbatim inside the answer token.
func AnswerRelevance(answer, query string) float64 {
	aTokens := wordTokens(answer)
	qTokens := wordTokens(query)
	if len(aTokens) == 0 || len(qTokens) == 0 {
		return 0
	}
	overlap := 0
	for _, qt := range qTokens {
		for i, at := range aTokens {
			if at == "" {
				continue
			}
			if qt == at || (isPureCJK(qt) && strings.Contains(at, qt)) {
				overlap++
				aTokens[i] = ""
				break
			}
		}
	}
	return 2 * float64(overlap) / float64(len(aTokens)+len(qTokens))
}

func isPureCJK(s string) bool {
	for _, r := range s {
		if !isCJK(r) {
			return false
		}
	}
	return len(s) > 0
}

func normalizeEvidence(s string) string {
	return strings.TrimRight(s, "。！？.;；!? \t\n")
}

func containsNormalized(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
