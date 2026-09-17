package rageval

import (
	"path/filepath"
	"testing"
)

// The gate test asserts against the artifacts that actually ship — the frozen
// snapshot in backend/testdata and the threshold contract in evals/ — so a
// stale test copy can never diverge from what CI enforces.
const (
	gateSnapshotPath   = "../../../testdata/rag_pr_gate_snapshot.jsonl"
	gateThresholdsPath = "../../../../evals/thresholds.yaml"
)

// SP-22 E3 PR gate contract: the frozen snapshot plus the shipped thresholds
// must pass, and a stricter bar must fail. This is the red/green proof the
// ticket asks for ("故意调低阈值能红、正常绿") without needing CI to run a
// doctored file.

func loadGateSnapshot(t *testing.T) *PRGateSnapshot {
	t.Helper()
	snap, err := LoadPRGateSnapshot(gateSnapshotPath)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	return snap
}

func TestPRGateMetricsOnFrozenSnapshot(t *testing.T) {
	snap := loadGateSnapshot(t)
	m := ComputePRGateMetrics(snap)

	if m.Cases != len(snap.Cases) {
		t.Fatalf("case count = %d, want %d", m.Cases, len(snap.Cases))
	}
	if m.Answerable+m.Unanswerable != m.Cases {
		t.Fatalf("confusion split %d+%d != %d", m.Answerable, m.Unanswerable, m.Cases)
	}
	if m.AnsweredAnswerable+m.RefusedAnswerable != m.Answerable {
		t.Fatalf("answerable outcomes do not partition the bucket")
	}
	if m.AnsweredUnanswerable+m.RefusedUnanswerable != m.Unanswerable {
		t.Fatalf("unanswerable outcomes do not partition the bucket")
	}
	if m.RetrievalEvaluated+m.RetrievalNotEvaluated != m.Cases {
		t.Fatalf("retrieval buckets do not cover every case")
	}
	// Rate sanity: probabilities, and the complementary pairs must sum to 1.
	for name, pair := range map[string][2]float64{
		"answerable":   {m.CorrectAnswerRate, m.OverRefusalRate},
		"unanswerable": {m.HallucinationRate, m.CorrectRefusalRate},
	} {
		sum := pair[0] + pair[1]
		if sum < 0.9999 || sum > 1.0001 {
			t.Fatalf("%s rates sum to %v, want 1", name, sum)
		}
	}
}

func TestPRGatePassesShippedThresholds(t *testing.T) {
	snap := loadGateSnapshot(t)
	thresholds, err := LoadPRGateThresholds(gateThresholdsPath)
	if err != nil {
		t.Fatalf("load thresholds: %v", err)
	}
	if violations := EvaluatePRGate(ComputePRGateMetrics(snap), thresholds); len(violations) != 0 {
		t.Fatalf("shipped thresholds should pass, got %v", violations)
	}
}

func TestPRGateFailsWhenThresholdsAreRaised(t *testing.T) {
	snap := loadGateSnapshot(t)
	thresholds, err := LoadPRGateThresholds(gateThresholdsPath)
	if err != nil {
		t.Fatalf("load thresholds: %v", err)
	}
	// Impossible bar: nothing reaches recall 1.0 on real data. The gate must
	// report the violation rather than pass quietly.
	thresholds.Minimums["context_recall_at_10"] = 1.0
	thresholds.Minimums["mrr"] = 1.0
	violations := EvaluatePRGate(ComputePRGateMetrics(snap), thresholds)
	if len(violations) != 2 {
		t.Fatalf("want 2 violations for the raised bar, got %d: %v", len(violations), violations)
	}
	for _, v := range violations {
		if v.Kind != "minimum" {
			t.Fatalf("violation kind = %q, want minimum", v.Kind)
		}
		if v.Got >= v.Want {
			t.Fatalf("violation %s has got %v >= want %v", v.Metric, v.Got, v.Want)
		}
	}
}

func TestPRGateFailsOnMaximumAndIntegrityBreach(t *testing.T) {
	snap := loadGateSnapshot(t)
	thresholds, err := LoadPRGateThresholds(gateThresholdsPath)
	if err != nil {
		t.Fatalf("load thresholds: %v", err)
	}
	thresholds.Maximums["hallucination_rate"] = 0.10
	thresholds.Integrity.MinCases = len(snap.Cases) + 1
	violations := EvaluatePRGate(ComputePRGateMetrics(snap), thresholds)
	kinds := map[string]int{}
	for _, v := range violations {
		kinds[v.Kind]++
	}
	if kinds["maximum"] != 1 || kinds["integrity"] != 1 {
		t.Fatalf("want one maximum and one integrity violation, got %v", violations)
	}
}

func TestPRGateUnknownMetricFailsLoudly(t *testing.T) {
	snap := loadGateSnapshot(t)
	thresholds, err := LoadPRGateThresholds(gateThresholdsPath)
	if err != nil {
		t.Fatalf("load thresholds: %v", err)
	}
	thresholds.Minimums["contxt_recall_at_10"] = 0.5 // typo must not pass
	violations := EvaluatePRGate(ComputePRGateMetrics(snap), thresholds)
	if len(violations) != 1 || violations[0].Kind != "integrity" {
		t.Fatalf("a typo'd metric must raise an integrity violation, got %v", violations)
	}
}

func TestPRGateSnapshotRejectsForeignArtifacts(t *testing.T) {
	if _, err := LoadPRGateSnapshot(filepath.Join("testdata", "ablation-judge-replay.jsonl")); err == nil {
		t.Fatal("expected a non-gate artifact to be rejected")
	}
}

func TestPRGateConfusionMatrixUsesGoldenLayer(t *testing.T) {
	// A case is unanswerable only by its golden layer; a no_answer case that
	// the run answered counts as a hallucination, and one refused as correct.
	snap := &PRGateSnapshot{
		SnapshotVersion: 1,
		Kind:            "rag-pr-gate-snapshot",
		Cases: []PRGateCaseRow{
			{CaseKey: "a", Layer: "known_item_exact", ExpectedIDs: []int64{1}, RetrievedIDs: []int64{1}, AnswerStatus: "answered"},
			{CaseKey: "b", Layer: "known_item_exact", ExpectedIDs: []int64{2}, RetrievedIDs: []int64{3}, AnswerStatus: "no_evidence"},
			{CaseKey: "c", Layer: "no_answer", AnswerStatus: "answered", AnswerKind: "grounded_content"},
			{CaseKey: "d", Layer: "no_answer", AnswerStatus: "no_evidence", AnswerKind: "no_evidence"},
		},
	}
	m := ComputePRGateMetrics(snap)
	if m.Answerable != 2 || m.Unanswerable != 2 {
		t.Fatalf("buckets = %d/%d, want 2/2", m.Answerable, m.Unanswerable)
	}
	if m.CorrectAnswerRate != 0.5 || m.OverRefusalRate != 0.5 {
		t.Fatalf("answerable rates = %v/%v, want 0.5/0.5", m.CorrectAnswerRate, m.OverRefusalRate)
	}
	if m.HallucinationRate != 0.5 || m.CorrectRefusalRate != 0.5 {
		t.Fatalf("unanswerable rates = %v/%v, want 0.5/0.5", m.HallucinationRate, m.CorrectRefusalRate)
	}
	if m.ContextRecallAt10 != 0.5 || m.HitRateAt5 != 0.5 {
		t.Fatalf("one of two evaluated cases hits, want 0.5/0.5, got %v/%v", m.ContextRecallAt10, m.HitRateAt5)
	}
	if m.RetrievalEvaluated != 2 || m.RetrievalNotEvaluated != 2 {
		t.Fatalf("retrieval coverage = %d/%d, want 2/2", m.RetrievalEvaluated, m.RetrievalNotEvaluated)
	}
}
