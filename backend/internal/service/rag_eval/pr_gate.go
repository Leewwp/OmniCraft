package rageval

import (
	"encoding/json"
	"fmt"
	"os"
)

// PRGateSnapshot is the frozen input of the PR-level zero-cost eval gate
// (SP-22 E3): both sides of every assertion are recorded at snapshot time so
// the gate runs with no provider calls, no LLM cost and no database access.
//
// Expected side: golden labels exported from eval_golden_cases.
// Actual side:   the recorded generation run's retrieved ids + answer status.
type PRGateSnapshot struct {
	SnapshotVersion int             `json:"snapshot_version"`
	Kind            string          `json:"kind"`
	DatasetChecksum string          `json:"dataset_checksum"`
	SourceLabel     string          `json:"source_label"`
	Cases           []PRGateCaseRow `json:"cases"`
}

// PRGateCaseRow is one frozen case: what the golden set expects, what the
// recorded run retrieved, and how the turn ended.
type PRGateCaseRow struct {
	CaseKey      string  `json:"case_key"`
	Split        string  `json:"split"`
	Layer        string  `json:"layer"`
	ExpectedIDs  []int64 `json:"expected_ids"`
	RetrievedIDs []int64 `json:"retrieved_ids"`
	AnswerStatus string  `json:"answer_status"`
	AnswerKind   string  `json:"answer_kind"`
}

// LoadPRGateSnapshot reads and validates a snapshot file.
func LoadPRGateSnapshot(path string) (*PRGateSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	var snap PRGateSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}
	if snap.Kind != "rag-pr-gate-snapshot" {
		return nil, fmt.Errorf("unexpected snapshot kind %q", snap.Kind)
	}
	if snap.SnapshotVersion != 1 {
		return nil, fmt.Errorf("unsupported snapshot version %d", snap.SnapshotVersion)
	}
	if len(snap.Cases) == 0 {
		return nil, fmt.Errorf("snapshot has no cases")
	}
	return &snap, nil
}

// PRGateMetrics is the zero-cost metric set the PR gate asserts on.
//
// Retrieval layer (ID-based, expected citations vs retrieved ids):
//   - context recall @10: share of expected ids present in the top 10
//   - hit rate @5/@10:    share of cases with at least one expected id in top k
//   - MRR:                mean reciprocal rank of the first expected id
//
// Refusal confusion matrix (golden no_answer layer = unanswerable):
//   - correct answer rate: answered / answerable
//   - over-refusal rate:   refused / answerable
//   - hallucination rate:  answered / unanswerable
//   - correct refusal rate: refused / unanswerable
type PRGateMetrics struct {
	Cases                 int     `json:"cases"`
	Answerable            int     `json:"answerable"`
	Unanswerable          int     `json:"unanswerable"`
	ContextRecallAt10     float64 `json:"context_recall_at_10"`
	HitRateAt5            float64 `json:"hit_rate_at_5"`
	HitRateAt10           float64 `json:"hit_rate_at_10"`
	MRR                   float64 `json:"mrr"`
	CorrectAnswerRate     float64 `json:"correct_answer_rate"`
	OverRefusalRate       float64 `json:"over_refusal_rate"`
	HallucinationRate     float64 `json:"hallucination_rate"`
	CorrectRefusalRate    float64 `json:"correct_refusal_rate"`
	AnsweredAnswerable    int     `json:"answered_answerable"`
	RefusedAnswerable     int     `json:"refused_answerable"`
	AnsweredUnanswerable  int     `json:"answered_unanswerable"`
	RefusedUnanswerable   int     `json:"refused_unanswerable"`
	RetrievalEvaluated    int     `json:"retrieval_evaluated"`
	RetrievalNotEvaluated int     `json:"retrieval_not_evaluated"`
}

// goldNoAnswerLayer is the golden classification layer whose cases must be
// answered with a refusal; every other layer is answerable.
const goldNoAnswerLayer = "no_answer"

// refused reports whether the recorded turn ended in a refusal rather than an
// answer: either the turn produced no answer evidence, or it is explicitly
// classified as a no-evidence outcome.
func (c PRGateCaseRow) refused() bool {
	return c.AnswerStatus == "no_evidence" || c.AnswerKind == "no_evidence"
}

// ComputePRGateMetrics derives every gate metric from the frozen snapshot.
func ComputePRGateMetrics(snap *PRGateSnapshot) PRGateMetrics {
	var m PRGateMetrics
	m.Cases = len(snap.Cases)

	var recallSum, mrrSum, hit5, hit10 float64
	var recallN int

	for _, c := range snap.Cases {
		answerable := c.Layer != goldNoAnswerLayer
		refused := c.refused()
		if answerable {
			m.Answerable++
			if refused {
				m.RefusedAnswerable++
			} else {
				m.AnsweredAnswerable++
			}
		} else {
			m.Unanswerable++
			if refused {
				m.RefusedUnanswerable++
			} else {
				m.AnsweredUnanswerable++
			}
		}

		// Retrieval-layer metrics need a non-empty expected set; cases whose
		// golden label is "there is nothing to retrieve" stay out of them.
		if len(c.ExpectedIDs) == 0 {
			m.RetrievalNotEvaluated++
			continue
		}
		m.RetrievalEvaluated++
		relevant := make(map[int64]bool, len(c.ExpectedIDs))
		for _, id := range c.ExpectedIDs {
			relevant[id] = true
		}
		ranked := dedupeIDs(c.RetrievedIDs)
		recallSum += RecallAtK(ranked, relevant, 10)
		mrrSum += MRR(ranked, relevant)
		recallN++
		if hitAtK(ranked, relevant, 5) {
			hit5++
		}
		if hitAtK(ranked, relevant, 10) {
			hit10++
		}
	}

	if recallN > 0 {
		m.ContextRecallAt10 = recallSum / float64(recallN)
		m.MRR = mrrSum / float64(recallN)
		m.HitRateAt5 = hit5 / float64(recallN)
		m.HitRateAt10 = hit10 / float64(recallN)
	}
	if m.Answerable > 0 {
		m.CorrectAnswerRate = float64(m.AnsweredAnswerable) / float64(m.Answerable)
		m.OverRefusalRate = float64(m.RefusedAnswerable) / float64(m.Answerable)
	}
	if m.Unanswerable > 0 {
		m.HallucinationRate = float64(m.AnsweredUnanswerable) / float64(m.Unanswerable)
		m.CorrectRefusalRate = float64(m.RefusedUnanswerable) / float64(m.Unanswerable)
	}
	return m
}

// dedupeIDs keeps the recorded retrieval rank but drops repeats (a repeated id
// must not inflate rank-based metrics).
func dedupeIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func hitAtK(ranked []int64, relevant map[int64]bool, k int) bool {
	limit := k
	if len(ranked) < limit {
		limit = len(ranked)
	}
	for i := 0; i < limit; i++ {
		if relevant[ranked[i]] {
			return true
		}
	}
	return false
}
