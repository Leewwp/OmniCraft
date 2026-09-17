package rageval

import (
	"fmt"
	"sort"

	"github.com/spf13/viper"
)

// PRGateThresholds is the parsed evals/thresholds.yaml contract. Tags are
// mapstructure because viper performs the decoding.
type PRGateThresholds struct {
	ArtifactVersion int                `mapstructure:"artifact_version"`
	Minimums        map[string]float64 `mapstructure:"minimums"`
	Maximums        map[string]float64 `mapstructure:"maximums"`
	Integrity       struct {
		MinCases              int `mapstructure:"min_cases"`
		MinRetrievalEvaluated int `mapstructure:"min_retrieval_evaluated"`
		MinUnanswerable       int `mapstructure:"min_unanswerable"`
		MinAnswerable         int `mapstructure:"min_answerable"`
	} `mapstructure:"integrity"`
}

// LoadPRGateThresholds reads and validates the threshold contract. It goes
// through viper (the config loader the backend already depends on) so the gate
// only ever evaluates a file that parses under the same rules as config.yaml.
func LoadPRGateThresholds(path string) (*PRGateThresholds, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read thresholds: %w", err)
	}
	var th PRGateThresholds
	if err := v.Unmarshal(&th); err != nil {
		return nil, fmt.Errorf("parse thresholds: %w", err)
	}
	if th.ArtifactVersion != 1 {
		return nil, fmt.Errorf("unsupported thresholds artifact_version %d", th.ArtifactVersion)
	}
	if len(th.Minimums) == 0 && len(th.Maximums) == 0 {
		return nil, fmt.Errorf("thresholds declare no minimums or maximums")
	}
	return &th, nil
}

// PRGateViolation is one failed assertion, reported with both sides so a red
// CI run is diagnosable without re-deriving the numbers.
type PRGateViolation struct {
	Metric string  `json:"metric"`
	Kind   string  `json:"kind"` // minimum | maximum | integrity
	Want   float64 `json:"want"`
	Got    float64 `json:"got"`
}

func (v PRGateViolation) String() string {
	switch v.Kind {
	case "minimum":
		return fmt.Sprintf("%s %.4f below required minimum %.4f", v.Metric, v.Got, v.Want)
	case "maximum":
		return fmt.Sprintf("%s %.4f above allowed maximum %.4f", v.Metric, v.Got, v.Want)
	default:
		return fmt.Sprintf("%s %.0f below required %.0f", v.Metric, v.Got, v.Want)
	}
}

// metricValue resolves a metric name against the computed set. Unknown names
// are a contract error, not a silent pass.
func metricValue(m PRGateMetrics, name string) (float64, bool) {
	switch name {
	case "context_recall_at_10":
		return m.ContextRecallAt10, true
	case "hit_rate_at_5":
		return m.HitRateAt5, true
	case "hit_rate_at_10":
		return m.HitRateAt10, true
	case "mrr":
		return m.MRR, true
	case "correct_answer_rate":
		return m.CorrectAnswerRate, true
	case "over_refusal_rate":
		return m.OverRefusalRate, true
	case "hallucination_rate":
		return m.HallucinationRate, true
	case "correct_refusal_rate":
		return m.CorrectRefusalRate, true
	default:
		return 0, false
	}
}

// EvaluatePRGate applies the threshold contract to a computed metric set.
// Violations are returned in a stable order (metric name) so CI logs diff
// cleanly between runs. An unknown metric name yields a violation rather than
// a silent pass: a typo in the YAML must fail loudly.
func EvaluatePRGate(m PRGateMetrics, th *PRGateThresholds) []PRGateViolation {
	var out []PRGateViolation
	for name, want := range th.Minimums {
		got, ok := metricValue(m, name)
		if !ok {
			out = append(out, PRGateViolation{Metric: name + " (unknown metric)", Kind: "integrity", Want: 1, Got: 0})
			continue
		}
		if got < want {
			out = append(out, PRGateViolation{Metric: name, Kind: "minimum", Want: want, Got: got})
		}
	}
	for name, want := range th.Maximums {
		got, ok := metricValue(m, name)
		if !ok {
			out = append(out, PRGateViolation{Metric: name + " (unknown metric)", Kind: "integrity", Want: 1, Got: 0})
			continue
		}
		if got > want {
			out = append(out, PRGateViolation{Metric: name, Kind: "maximum", Want: want, Got: got})
		}
	}
	integrity := []struct {
		name string
		got  int
		want int
	}{
		{"cases", m.Cases, th.Integrity.MinCases},
		{"retrieval_evaluated", m.RetrievalEvaluated, th.Integrity.MinRetrievalEvaluated},
		{"unanswerable", m.Unanswerable, th.Integrity.MinUnanswerable},
		{"answerable", m.Answerable, th.Integrity.MinAnswerable},
	}
	for _, check := range integrity {
		if check.want > 0 && check.got < check.want {
			out = append(out, PRGateViolation{
				Metric: check.name, Kind: "integrity",
				Want: float64(check.want), Got: float64(check.got),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Metric != out[j].Metric {
			return out[i].Metric < out[j].Metric
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
