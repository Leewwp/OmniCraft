package rageval

import (
	"reflect"
	"testing"
)

// Metric conventions pinned here follow rag design §7:
//   - recall@K, MRR, nDCG@10 are content-level over the ranked list;
//   - citation precision/coverage use (content_id, content_version) identity;
//   - visibility leak count counts forbidden ids in the produced list;
//   - P95 latency over collected per-query durations.
func TestRecallAtK(t *testing.T) {
	relevant := map[int64]bool{3: true, 7: true}
	ranked := []int64{1, 3, 5, 7, 9}
	if got := RecallAtK(ranked, relevant, 3); got != 0.5 {
		t.Errorf("RecallAtK top-3 = %v, want 0.5", got)
	}
	if got := RecallAtK(ranked, relevant, 5); got != 1.0 {
		t.Errorf("RecallAtK top-5 = %v, want 1.0", got)
	}
	if got := RecallAtK(ranked, relevant, 20); got != 1.0 {
		t.Errorf("RecallAtK top-20 = %v, want 1.0", got)
	}
	if got := RecallAtK(nil, relevant, 10); got != 0 {
		t.Errorf("RecallAtK empty = %v, want 0", got)
	}
}

func TestMRR(t *testing.T) {
	relevant := map[int64]bool{7: true, 9: true}
	if got := MRR([]int64{1, 3, 5, 7, 9}, relevant); got != 0.25 {
		t.Errorf("MRR = %v, want 0.25 (first relevant at rank 4)", got)
	}
	if got := MRR([]int64{7, 1, 9}, relevant); got != 1.0 {
		t.Errorf("MRR first position = %v, want 1.0", got)
	}
	if got := MRR([]int64{1, 2}, relevant); got != 0 {
		t.Errorf("MRR no relevant = %v, want 0", got)
	}
}

func TestNDCGAt10(t *testing.T) {
	// Relevant at ranks 1 and 3 of 10 (1-based): DCG = 1/log2(2) + 1/log2(4)
	// = 1 + 0.5; IDCG over 2 relevant = 1 + 1/log2(3) ≈ 1.6309.
	relevant := map[int64]bool{7: true, 3: true}
	got := NDCGAt10([]int64{7, 9, 3, 1, 2, 4, 5, 6, 8, 10}, relevant)
	if want := (1.0 + 1.0/2) / (1.0 + 1.0/1.5849625007211563); got != want {
		t.Errorf("NDCG@10 = %v, want %v", got, want)
	}
	if got := NDCGAt10([]int64{1, 2}, relevant); got != 0 {
		t.Errorf("NDCG@10 no relevant = %v, want 0", got)
	}
}

func TestCitationPrecisionAndCoverage(t *testing.T) {
	expected := map[Citation]bool{
		{ContentID: 1, ContentVersion: 1}: true,
		{ContentID: 2, ContentVersion: 1}: true,
	}
	produced := []Citation{
		{ContentID: 1, ContentVersion: 1},
		{ContentID: 9, ContentVersion: 1},
	}
	if got := CitationPrecision(produced, expected); got != 0.5 {
		t.Errorf("citation precision = %v, want 0.5", got)
	}
	if got := CitationCoverage(produced, expected); got != 0.5 {
		t.Errorf("citation coverage = %v, want 0.5", got)
	}
	if got := CitationPrecision(nil, expected); got != 1 {
		t.Errorf("citation precision on empty output = %v, want 1", got)
	}
	if got := CitationCoverage(produced, map[Citation]bool{}); got != 0 {
		t.Errorf("citation coverage with empty expected and non-empty output = %v, want 0", got)
	}
	if got := CitationCoverage(nil, map[Citation]bool{}); got != 1 {
		t.Errorf("citation coverage with both empty = %v, want 1", got)
	}
	// Unknown retrieved version (0) matches any expected citation of the same
	// content id so corpus rows without version rows stay comparable.
	if got := CitationPrecision([]Citation{{ContentID: 1, ContentVersion: 0}}, expected); got != 1 {
		t.Errorf("citation precision with unknown version = %v, want 1", got)
	}
}

func TestDeduplicatedCitationMetricsKeepFirstRankedContent(t *testing.T) {
	expected := map[Citation]bool{{ContentID: 1, ContentVersion: 1}: true}
	produced := []Citation{
		{ContentID: 1, ContentVersion: 1},
		{ContentID: 1, ContentVersion: 1},
		{ContentID: 9, ContentVersion: 1},
	}
	got := DeduplicateCitations(produced)
	if len(got) != 2 || got[0] != produced[0] || got[1] != produced[2] {
		t.Fatalf("deduplicated citations = %#v, want first-ranked unique identities", got)
	}
	if precision := CitationPrecisionDeduplicated(produced, expected); precision != 0.5 {
		t.Errorf("deduplicated citation precision = %v, want 0.5", precision)
	}
	if coverage := CitationCoverageDeduplicated(produced, expected); coverage != 1 {
		t.Errorf("deduplicated citation coverage = %v, want 1", coverage)
	}
}

func TestVisibilityLeaks(t *testing.T) {
	forbidden := map[int64]bool{1006: true, 1007: true}
	leaked := VisibilityLeaks([]int64{1001, 1007, 1002, 1006, 1003}, forbidden)
	if want := []int64{1006, 1007}; !reflect.DeepEqual(leaked, want) {
		t.Errorf("leaked ids = %v, want %v (sorted, deduped)", leaked, want)
	}
	if got := VisibilityLeaks([]int64{1001, 1002}, forbidden); len(got) != 0 {
		t.Errorf("leaks on clean output = %v, want none", got)
	}
}

func TestP95(t *testing.T) {
	// Nearest-rank estimator: for 10 samples p95 is the highest value.
	durations := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	if got := P95(durations); got != 100 {
		t.Errorf("P95 = %v, want 100", got)
	}
	if got := P95([]float64{5}); got != 5 {
		t.Errorf("P95 single = %v, want 5", got)
	}
	if got := P95(nil); got != 0 {
		t.Errorf("P95 empty = %v, want 0", got)
	}
}

func TestDegradationSuccessRate(t *testing.T) {
	if got := DegradationSuccessRate(10, 2); got != 0.8 {
		t.Errorf("degradation success rate = %v, want 0.8", got)
	}
	if got := DegradationSuccessRate(0, 0); got != 1 {
		t.Errorf("degradation success rate on zero runs = %v, want 1", got)
	}
}

func TestGroundedness(t *testing.T) {
	evidence := []string{
		"燕麦打底，加酸奶、蓝莓和香蕉，最后淋一点蜂蜜。",
		"前 30 天基本只会大白点，第 60 天稳定出心形。",
	}
	sentences := []string{
		"燕麦打底，加酸奶、蓝莓和香蕉",
		"这句话在证据里找不到依据",
	}
	if got := Groundedness(sentences, evidence); got != 0.5 {
		t.Errorf("groundedness = %v, want 0.5", got)
	}
}

func TestAnswerRelevance(t *testing.T) {
	// High token overlap with the query; determinism is what matters here,
	// not semantic quality.
	high := AnswerRelevance("哑铃全身训练计划", "哑铃全身训练")
	low := AnswerRelevance("今天天气很好", "哑铃全身训练")
	if high <= low {
		t.Errorf("answer relevance overlap must rank query-token overlap higher: high=%v low=%v", high, low)
	}
}
