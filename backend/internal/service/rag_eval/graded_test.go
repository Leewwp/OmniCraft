package rageval

import (
	"math"
	"testing"

	"omnicraft/backend/internal/model"
)

func TestGradedGains(t *testing.T) {
	expected := map[int64]bool{1: true}
	acceptable := map[int64]bool{2: true, 1: true} // 1 in both tiers: expected wins
	gains := GradedGains(expected, acceptable)
	if gains[1] != GainExpected || gains[2] != GainAcceptable {
		t.Fatalf("gains = %v", gains)
	}
}

func TestGradedNDCGAtK(t *testing.T) {
	// ranked [2, 3, 1]: acceptable at rank 1, noise at rank 2, expected at 3.
	ranked := []int64{2, 3, 1}
	gains := map[int64]float64{1: GainExpected, 2: GainAcceptable}
	got := GradedNDCGAtK(ranked, gains, 10)
	want := (GainAcceptable/math.Log2(2) + GainExpected/math.Log2(4)) /
		(GainExpected/math.Log2(2) + GainAcceptable/math.Log2(3))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("graded ndcg = %f, want %f", got, want)
	}

	// an acceptable-only hit scores partial credit, unlike the binary metric
	onlyAcceptable := GradedNDCGAtK([]int64{2}, map[int64]float64{2: GainAcceptable}, 10)
	if onlyAcceptable != 1 {
		t.Fatalf("acceptable-only perfect ranking = %f, want 1", onlyAcceptable)
	}

	if GradedNDCGAtK([]int64{1}, nil, 10) != 0 {
		t.Fatal("empty gains must score 0")
	}
	if GradedNDCGAtK([]int64{1}, map[int64]float64{1: GainExpected}, 0) != 0 {
		t.Fatal("k=0 must score 0")
	}
}

func TestTrapHitAtK(t *testing.T) {
	forbidden := map[int64]bool{9: true, 8: true}
	hits := TrapHitAtK([]int64{1, 9, 8, 5, 9}, forbidden, 3)
	if len(hits) != 2 || hits[0] != 8 || hits[1] != 9 {
		t.Fatalf("trap hits = %v, want deduped [8 9] within top-3", hits)
	}
	if got := TrapHitAtK([]int64{1, 9}, forbidden, 1); len(got) != 0 {
		t.Fatalf("trap outside top-k = %v, want none", got)
	}
}

func TestWilsonInterval(t *testing.T) {
	low, high := WilsonInterval(0, 10, WilsonZ975)
	if low > 1e-9 || high <= 0 || high > 0.35 {
		t.Fatalf("0/10 interval = [%f, %f]", low, high)
	}
	low, high = WilsonInterval(10, 10, WilsonZ975)
	if high < 1-1e-9 || low < 0.65 {
		t.Fatalf("10/10 interval = [%f, %f]", low, high)
	}
	low, high = WilsonInterval(5, 10, WilsonZ975)
	if low > 0.5 || high < 0.5 || low >= high {
		t.Fatalf("5/10 interval = [%f, %f], want a straddling interval", low, high)
	}
	if low, high := WilsonInterval(3, 0, WilsonZ975); low != 0 || high != 0 {
		t.Fatal("zero denominator must yield the zero interval")
	}
}

func TestNewRate(t *testing.T) {
	rate := NewRate(3, 4)
	if rate.Value != 0.75 || rate.Numerator != 3 || rate.Denominator != 4 {
		t.Fatalf("rate = %+v", rate)
	}
	if rate.CI95Low >= rate.Value || rate.CI95High <= rate.Value {
		t.Fatalf("CI must bracket the point estimate: %+v", rate)
	}
	if zero := NewRate(0, 0); zero.Value != 0 || zero.Denominator != 0 {
		t.Fatalf("zero rate = %+v", zero)
	}
}

// H4: split filtering keeps annotated splits, optional legacy rows and
// rejects unknown split values.
func TestFilterCasesBySplit(t *testing.T) {
	cases := []model.EvalGoldenCase{
		{CaseKey: "dev-case", Classification: model.JSONB(`{"primary_layer":"known_item_exact","split":"dev"}`)},
		{CaseKey: "test-case", Classification: model.JSONB(`{"primary_layer":"known_item_exact","split":"test"}`)},
		{CaseKey: "legacy-case", Classification: model.JSONB(`{"content_type":"mod","popularity":"cold"}`)},
	}

	dev, err := FilterCasesBySplit(cases, SplitDev, false)
	if err != nil || len(dev) != 1 || dev[0].CaseKey != "dev-case" {
		t.Fatalf("dev filter = %v (%d), err %v", caseKeys(dev), len(dev), err)
	}
	test, err := FilterCasesBySplit(cases, SplitTest, false)
	if err != nil || len(test) != 1 || test[0].CaseKey != "test-case" {
		t.Fatalf("test filter = %v, err %v", caseKeys(test), err)
	}
	all, err := FilterCasesBySplit(cases, "", true)
	if err != nil || len(all) != 3 {
		t.Fatalf("unfiltered+legacy = %v, err %v", caseKeys(all), err)
	}
	noLegacy, err := FilterCasesBySplit(cases, "", false)
	if err != nil || len(noLegacy) != 2 {
		t.Fatalf("unfiltered without legacy = %v, err %v", caseKeys(noLegacy), err)
	}
	if _, err := FilterCasesBySplit(cases, "holdout", false); err == nil {
		t.Fatal("unknown split must error")
	}
}

// H4: the test split refuses to run without the explicit confirmation flag.
func TestValidateTestSplitGate(t *testing.T) {
	if err := ValidateTestSplitGate(SplitDev, false); err != nil {
		t.Fatalf("dev run must never need confirmation: %v", err)
	}
	if err := ValidateTestSplitGate("", false); err != nil {
		t.Fatalf("unfiltered run must never need confirmation: %v", err)
	}
	if err := ValidateTestSplitGate(SplitTest, false); err == nil {
		t.Fatal("test split without the flag must be refused")
	}
	if err := ValidateTestSplitGate(SplitTest, true); err != nil {
		t.Fatalf("test split with the flag must run: %v", err)
	}
}

func TestParseClassificationV2(t *testing.T) {
	cl, err := ParseClassificationV2(model.JSONB(`{
		"content_type": "mod", "popularity": "cold",
		"primary_layer": "no_answer", "split": "dev",
		"no_answer_strategy": "strict_not_found",
		"ip_scope": true, "ip": 12, "language": "zh",
		"temperature_band": "cold", "corpus_visibility": "public"
	}`))
	if err != nil {
		t.Fatalf("parse v2 classification: %v", err)
	}
	if cl.PrimaryLayer != LayerNoAnswer || cl.Split != SplitDev ||
		cl.NoAnswerStrategy != NoAnswerStrictNotFound || cl.IPScope == nil || !*cl.IPScope ||
		cl.Language != "zh" || cl.CorpusVisibility != "public" {
		t.Fatalf("v2 classification = %+v", cl)
	}
	if !cl.IsKnownLayer() {
		t.Fatal("no_answer must be a known layer")
	}

	legacy, err := ParseClassificationV2(model.JSONB(`{"content_type":"article","popularity":"hot"}`))
	if err != nil {
		t.Fatalf("parse legacy classification: %v", err)
	}
	if legacy.PrimaryLayer != "" || legacy.Layer() != "unknown" || legacy.IsKnownLayer() {
		t.Fatalf("legacy classification = %+v", legacy)
	}
}

func TestParseAnswerRubric(t *testing.T) {
	rubric, err := ParseAnswerRubric(model.JSONB(`{
		"acceptable_content_ids": [5, 6],
		"forbidden_reasons": {"7": "same IP, different modloader"},
		"must_not_claim": ["《星海遗响》"]
	}`))
	if err != nil {
		t.Fatalf("parse v2 rubric: %v", err)
	}
	if len(rubric.AcceptableContentIDs) != 2 || rubric.ForbiddenReasons["7"] == "" || len(rubric.MustNotClaim) != 1 {
		t.Fatalf("v2 rubric = %+v", rubric)
	}

	legacy, err := ParseAnswerRubric(model.JSONB(`{"judge_criteria":["x"],"min_judge_score":4}`))
	if err != nil || len(legacy.JudgeCriteria) != 1 || legacy.MinJudgeScore != 4 || len(legacy.MustNotClaim) != 0 {
		t.Fatalf("legacy rubric = %+v, err %v", legacy, err)
	}
	if empty, err := ParseAnswerRubric(model.JSONB(`null`)); err != nil || len(empty.AcceptableContentIDs) != 0 {
		t.Fatalf("null rubric = %+v, err %v", empty, err)
	}
}

func caseKeys(cases []model.EvalGoldenCase) []string {
	keys := make([]string, 0, len(cases))
	for _, c := range cases {
		keys = append(keys, c.CaseKey)
	}
	return keys
}
