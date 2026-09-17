package rageval

// RetrievalHeadlineMetrics is the zero-cost headline metric set of one
// retrieval configuration (SP-22 E4 grid runner): the same ID-based
// definitions the PR gate asserts (pr_gate.go), computed from retrieval-layer
// per-case rows instead of a frozen generation snapshot. It exists so the
// grid scan and the E5 trend page share one metric vocabulary with the PR
// gate instead of three slightly different formulas.
//
// The refusal side is a PROXY: without generation there is no answer status,
// so over-refusal is modelled as "an answerable case whose top-10 retrieval
// contains none of the expected ids" — under the server-side no-evidence rule
// the agent would have no citable evidence for that case. The real confusion
// matrix (with hallucination) stays a generation-layer artifact.
type RetrievalHeadlineMetrics struct {
	RetrievalEvaluated   int     `json:"retrieval_evaluated"`
	RetrievalNotEvaluated int    `json:"retrieval_not_evaluated"`
	RetrievalFailed      int     `json:"retrieval_failed"`
	AnswerableEvaluated  int     `json:"answerable_evaluated"`

	ContextRecallAt5  float64 `json:"context_recall_at_5"`
	ContextRecallAt10 float64 `json:"context_recall_at_10"`
	HitRateAt5        float64 `json:"hit_rate_at_5"`
	HitRateAt10       float64 `json:"hit_rate_at_10"`
	MRR               float64 `json:"mrr"`
	NDCGAt5           float64 `json:"ndcg_at_5"`

	WouldRefuseCount    int     `json:"would_refuse_count"`
	OverRefusalProxy    float64 `json:"over_refusal_proxy_rate"`
	CorrectRetrieveRate float64 `json:"correct_retrieve_rate"`

	MeanLatencyMs float64 `json:"mean_latency_ms"`
}

// goldNoAnswerLayerKey matches the PR gate's unanswerable layer constant; the
// answerable side of the proxy matrix is every other layer.
const goldNoAnswerLayerKey = "no_answer"

// ComputeRetrievalHeadline aggregates retrieval per-case rows into the
// headline set. Cases whose expected tier is empty (nothing to retrieve by
// definition) and cases whose retrieval call failed are excluded from the
// rate denominators and reported as separate counts, so a config that fails
// half its calls cannot hide behind the survivors' averages.
func ComputeRetrievalHeadline(perCase []LayeredCaseResult) RetrievalHeadlineMetrics {
	var m RetrievalHeadlineMetrics
	var recall5, recall10, mrr, ndcg5, latency float64
	var hit5, hit10, n int
	var latencyN int

	for _, c := range perCase {
		if !c.Success {
			m.RetrievalFailed++
			continue
		}
		if len(c.ExpectedIDs) == 0 {
			m.RetrievalNotEvaluated++
			continue
		}
		m.RetrievalEvaluated++
		n++
		recall5 += c.RecallAt5
		recall10 += c.RecallAt10
		mrr += c.MRR
		ndcg5 += c.GradedNDCGAt5
		latency += c.LatencyMs
		latencyN++
		if c.RecallAt5 > 0 {
			hit5++
		}
		if c.RecallAt10 > 0 {
			hit10++
		}
		if c.PrimaryLayer != goldNoAnswerLayerKey {
			m.AnswerableEvaluated++
			if c.RecallAt10 == 0 {
				// No expected id in the top-10 slice: under the deterministic
				// no-evidence rule this turn would end refused.
				m.WouldRefuseCount++
			}
		}
	}

	if n > 0 {
		m.ContextRecallAt5 = recall5 / float64(n)
		m.ContextRecallAt10 = recall10 / float64(n)
		m.HitRateAt5 = float64(hit5) / float64(n)
		m.HitRateAt10 = float64(hit10) / float64(n)
		m.MRR = mrr / float64(n)
		m.NDCGAt5 = ndcg5 / float64(n)
	}
	if latencyN > 0 {
		m.MeanLatencyMs = latency / float64(latencyN)
	}
	if m.AnswerableEvaluated > 0 {
		m.OverRefusalProxy = float64(m.WouldRefuseCount) / float64(m.AnswerableEvaluated)
		m.CorrectRetrieveRate = 1 - m.OverRefusalProxy
	}
	return m
}
