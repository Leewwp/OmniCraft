package rageval

import (
	"encoding/json"
	"fmt"
	"strings"

	"omnicraft/backend/internal/model"
)

// Golden Set v2 primary layers (annotation contract §2). The six layers are
// the grouping dimension of every v2 report; no aggregate accuracy across
// layers is produced.
const (
	LayerKnownItemExact    = "known_item_exact"
	LayerSemanticDiscovery = "semantic_discovery"
	LayerBodyEvidence      = "body_evidence"
	LayerHardNeighbor      = "hard_neighbor"
	LayerNoAnswer          = "no_answer"
	LayerVisibility        = "visibility"
)

// V2Layers is the canonical layer list in contract order.
var V2Layers = []string{
	LayerKnownItemExact,
	LayerSemanticDiscovery,
	LayerBodyEvidence,
	LayerHardNeighbor,
	LayerNoAnswer,
	LayerVisibility,
}

// Split values (contract §7): group-aware 80/20, deterministic, written to
// classification.split. A-04 tuning reads dev only; the final test split runs
// once and needs an explicit confirmation flag (H4).
const (
	SplitDev  = "dev"
	SplitTest = "test"
)

// No-answer judge strategies (contract §1.1).
const (
	NoAnswerStrictNotFound          = "strict_not_found"
	NoAnswerRelatedRecommendationOK = "related_recommendation_allowed"
)

// AnswerRubric mirrors the v2 answer_rubric jsonb payload (contract §3). The
// v1 fields stay first-class so legacy fixtures parse unchanged.
type AnswerRubric struct {
	JudgeCriteria           []string          `json:"judge_criteria,omitempty"`
	MinJudgeScore           int               `json:"min_judge_score,omitempty"`
	DeterministicAssertions []string          `json:"deterministic_assertions,omitempty"`
	AcceptableContentIDs    []int64           `json:"acceptable_content_ids,omitempty"`
	ForbiddenReasons        map[string]string `json:"forbidden_reasons,omitempty"`
	MustNotClaim            []string          `json:"must_not_claim,omitempty"`
}

// ParseAnswerRubric reads the answer_rubric jsonb payload.
func ParseAnswerRubric(raw model.JSONB) (AnswerRubric, error) {
	var rubric AnswerRubric
	if len(raw) == 0 || string(raw) == "null" {
		return rubric, nil
	}
	if err := json.Unmarshal(raw, &rubric); err != nil {
		return AnswerRubric{}, err
	}
	return rubric, nil
}

// ParseClassificationV2 reads the classification jsonb including the v2
// annotation fields (primary_layer/split/no_answer_strategy/dimensions).
// Legacy v1 classifications parse with the v2 fields empty.
func ParseClassificationV2(raw model.JSONB) (Classification, error) {
	var c Classification
	if len(raw) == 0 || string(raw) == "null" {
		return c, nil
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return Classification{}, err
	}
	return c, nil
}

// FilterCasesBySplit keeps the cases whose classification.split matches.
// Legacy cases without a split annotation are kept only when includeNoSplit
// is set (the v1 path never calls this filter).
func FilterCasesBySplit(cases []model.EvalGoldenCase, split string, includeNoSplit bool) ([]model.EvalGoldenCase, error) {
	split = strings.TrimSpace(split)
	if split != "" && split != SplitDev && split != SplitTest {
		return nil, fmt.Errorf("split filter %q must be %q, %q or empty", split, SplitDev, SplitTest)
	}
	kept := make([]model.EvalGoldenCase, 0, len(cases))
	for _, c := range cases {
		cl, err := ParseClassificationV2(c.Classification)
		if err != nil {
			return nil, fmt.Errorf("case %q classification: %w", c.CaseKey, err)
		}
		switch {
		case cl.Split == "":
			if includeNoSplit {
				kept = append(kept, c)
			}
		case split == "" || cl.Split == split:
			kept = append(kept, c)
		}
	}
	return kept, nil
}

// ValidateTestSplitGate enforces the H4 explicit-flag rule: running the test
// split must pass ConfirmTestSplitRun=true. Dev runs and unfiltered runs
// never need it. Splitting on "" with the flag set is a caller bug.
func ValidateTestSplitGate(split string, confirmTest bool) error {
	split = strings.TrimSpace(split)
	if split != SplitTest {
		return nil
	}
	if !confirmTest {
		return fmt.Errorf("refusing to run the %s split: set ConfirmTestSplitRun=true explicitly (final test runs once; A-04 tuning reads %s only)", SplitTest, SplitDev)
	}
	return nil
}
