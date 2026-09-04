package rageval

import (
	"math"
	"sort"
)

// Graded-relevance gains for the three-tier annotation (contract §1.3/§8-H3):
// expected items are the must-return truth (gain 1); acceptable items count
// as partial relevance gain (0.5); forbidden items never contribute gain.
const (
	GainExpected   = 1.0
	GainAcceptable = 0.5
)

// GradedGains builds the graded gain map from the expected (must-return) and
// acceptable (also-reasonable) tiers. Expected wins when an id appears in
// both tiers.
func GradedGains(expected, acceptable map[int64]bool) map[int64]float64 {
	gains := make(map[int64]float64, len(expected)+len(acceptable))
	for id := range acceptable {
		gains[id] = GainAcceptable
	}
	for id := range expected {
		gains[id] = GainExpected
	}
	return gains
}

// GradedNDCGAtK is the discounted cumulative gain at k over graded gains,
// normalised by the ideal ordering of every graded item (expected items rank
// above acceptable ones). Extends the v1 binary NDCGAt10: an acceptable-only
// hit yields partial credit instead of 0.
func GradedNDCGAtK(ranked []int64, gains map[int64]float64, k int) float64 {
	if len(gains) == 0 || k <= 0 {
		return 0
	}
	limited := k
	if limited > len(ranked) {
		limited = len(ranked)
	}
	var dcg float64
	for i := 0; i < limited; i++ {
		gain, ok := gains[ranked[i]]
		if !ok || gain <= 0 {
			continue
		}
		dcg += gain / math.Log2(float64(i+2))
	}
	ideal := make([]float64, 0, len(gains))
	for _, gain := range gains {
		if gain > 0 {
			ideal = append(ideal, gain)
		}
	}
	if len(ideal) == 0 {
		return 0
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(ideal)))
	limitIdeal := limited
	if limitIdeal > len(ideal) {
		limitIdeal = len(ideal)
	}
	var idcg float64
	for i := 0; i < limitIdeal; i++ {
		idcg += ideal[i] / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// TrapHitAtK returns the forbidden ids that appear within the first k ranked
// positions. For the no-answer layer this is a retrieval-layer diagnostic
// only (contract §1.1): a trap hit never fails a case by itself.
func TrapHitAtK(ranked []int64, forbidden map[int64]bool, k int) []int64 {
	var hits []int64
	seen := map[int64]bool{}
	for i, id := range ranked {
		if i >= k {
			break
		}
		if forbidden[id] && !seen[id] {
			hits = append(hits, id)
			seen[id] = true
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i] < hits[j] })
	return hits
}

// RateWithCI is a proportion with its raw numerator/denominator and a Wilson
// 95% confidence interval. Layer reports carry the raw counts so the freeze
// gate can audit denominators without re-deriving them (contract §8:
// 分层独立报告，原始分子/分母 + 置信区间).
type RateWithCI struct {
	Numerator   float64 `json:"numerator"`
	Denominator float64 `json:"denominator"`
	Value       float64 `json:"value"`
	CI95Low     float64 `json:"ci95_low"`
	CI95High    float64 `json:"ci95_high"`
}

// WilsonZ975 is the two-sided 95% normal quantile used by the interval.
const WilsonZ975 = 1.959963984540054

// NewRate builds the proportion and its Wilson score interval. A zero
// denominator yields the zero rate with an empty interval; small-denominator
// rates therefore stay honest instead of collapsing to 0 or 1.
func NewRate(numerator, denominator float64) RateWithCI {
	if denominator <= 0 {
		return RateWithCI{}
	}
	value := numerator / denominator
	low, high := WilsonInterval(numerator, denominator, WilsonZ975)
	return RateWithCI{
		Numerator:   numerator,
		Denominator: denominator,
		Value:       value,
		CI95Low:     low,
		CI95High:    high,
	}
}

// WilsonInterval returns the Wilson score interval for a binomial
// proportion, clamped to [0,1].
func WilsonInterval(successes, total, z float64) (low, high float64) {
	if total <= 0 {
		return 0, 0
	}
	if successes < 0 {
		successes = 0
	}
	if successes > total {
		successes = total
	}
	p := successes / total
	z2 := z * z
	denominator := 1 + z2/total
	centre := p + z2/(2*total)
	margin := z * math.Sqrt(p*(1-p)/total+z2/(4*total*total))
	low = (centre - margin) / denominator
	high = (centre + margin) / denominator
	if low < 0 {
		low = 0
	}
	if high > 1 {
		high = 1
	}
	return low, high
}

// rateAccumulator sums raw numerators and denominators across cases so layer
// aggregates are micro-averages of the underlying counts, not means of means.
type rateAccumulator struct {
	numerator   float64
	denominator float64
}

func (a *rateAccumulator) add(numerator, denominator float64) {
	a.numerator += numerator
	a.denominator += denominator
}

func (a *rateAccumulator) rate() RateWithCI {
	return NewRate(a.numerator, a.denominator)
}
