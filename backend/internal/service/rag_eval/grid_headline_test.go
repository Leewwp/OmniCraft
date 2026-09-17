package rageval

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComputeRetrievalHeadline(t *testing.T) {
	perCase := []LayeredCaseResult{
		// answerable, expected hit at rank 1
		{CaseKey: "sd-0001", PrimaryLayer: "semantic_detail", ExpectedIDs: []int64{1},
			RetrievedIDs: []int64{1, 2}, RecallAt5: 1, RecallAt10: 1, MRR: 1, GradedNDCGAt5: 1,
			LatencyMs: 100, Success: true},
		// answerable, expected only at rank 9 (in top-10 but not top-5)
		{CaseKey: "be-0002", PrimaryLayer: "beyond_evidence", ExpectedIDs: []int64{9},
			RetrievedIDs: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9}, RecallAt5: 0, RecallAt10: 1,
			MRR: 1.0 / 9.0, GradedNDCGAt5: 0, LatencyMs: 200, Success: true},
		// answerable, expected never retrieved -> would-refuse proxy
		{CaseKey: "hn-0003", PrimaryLayer: "hybrid_noise", ExpectedIDs: []int64{42},
			RetrievedIDs: []int64{1, 2}, RecallAt5: 0, RecallAt10: 0, MRR: 0,
			LatencyMs: 50, Success: true},
		// unanswerable layer with an expected set still counts as evaluated,
		// but never enters the answerable proxy denominator
		{CaseKey: "na-0004", PrimaryLayer: goldNoAnswerLayerKey, ExpectedIDs: []int64{7},
			RetrievedIDs: []int64{1, 7}, RecallAt5: 1, RecallAt10: 1, MRR: 0.5,
			LatencyMs: 80, Success: true},
		// empty expected tier: skipped by definition
		{CaseKey: "na-0005", PrimaryLayer: goldNoAnswerLayerKey,
			RetrievedIDs: []int64{1}, LatencyMs: 10, Success: true},
		// failed retrieval: excluded from rates, reported separately
		{CaseKey: "sd-0006", PrimaryLayer: "semantic_detail", ExpectedIDs: []int64{3},
			RetrievedIDs: nil, LatencyMs: 0, Success: false, Error: "provider down"},
	}

	m := ComputeRetrievalHeadline(perCase)

	if m.RetrievalEvaluated != 4 || m.RetrievalNotEvaluated != 1 || m.RetrievalFailed != 1 {
		t.Fatalf("counts: evaluated=%d not_evaluated=%d failed=%d", m.RetrievalEvaluated, m.RetrievalNotEvaluated, m.RetrievalFailed)
	}
	if m.AnswerableEvaluated != 3 || m.WouldRefuseCount != 1 {
		t.Fatalf("answerable=%d would_refuse=%d", m.AnswerableEvaluated, m.WouldRefuseCount)
	}
	wantRecall5 := 2.0 / 4.0
	wantRecall10 := 3.0 / 4.0
	if diff(m.ContextRecallAt5, wantRecall5) || diff(m.ContextRecallAt10, wantRecall10) {
		t.Fatalf("recall5=%v want %v; recall10=%v want %v", m.ContextRecallAt5, wantRecall5, m.ContextRecallAt10, wantRecall10)
	}
	if diff(m.HitRateAt5, 0.5) || diff(m.HitRateAt10, 0.75) {
		t.Fatalf("hit5=%v hit10=%v", m.HitRateAt5, m.HitRateAt10)
	}
	wantMRR := (1 + 1.0/9.0 + 0 + 0.5) / 4
	if diff(m.MRR, wantMRR) {
		t.Fatalf("mrr=%v want %v", m.MRR, wantMRR)
	}
	if diff(m.OverRefusalProxy, 1.0/3.0) || diff(m.CorrectRetrieveRate, 2.0/3.0) {
		t.Fatalf("over_refusal=%v correct=%v", m.OverRefusalProxy, m.CorrectRetrieveRate)
	}
	if diff(m.MeanLatencyMs, (100+200+50+80)/4.0) {
		t.Fatalf("mean latency=%v", m.MeanLatencyMs)
	}

	// JSON stability: the field names are the E5 trend page contract.
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"context_recall_at_10", "hit_rate_at_5", "hit_rate_at_10", "mrr", "over_refusal_proxy_rate"} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("missing contract key %q in %s", key, raw)
		}
	}
}

func TestComputeRetrievalHeadlineEmpty(t *testing.T) {
	m := ComputeRetrievalHeadline(nil)
	if m.RetrievalEvaluated != 0 || m.OverRefusalProxy != 0 || m.MRR != 0 {
		t.Fatalf("empty input must stay zero: %+v", m)
	}
}

func diff(a, b float64) bool {
	if a-b > 1e-9 || b-a > 1e-9 {
		return true
	}
	return false
}
