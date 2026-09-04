package rageval

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"omnicraft/backend/internal/model"
)

// v2LayeredCases is a synthetic six-layer dataset shaped exactly like the
// frozen 069 rows (schema_version=2): principal_key in viewer_context,
// expected tiers in expected_citations, acceptable in answer_rubric,
// forbidden with reasons, layer/split/strategy in classification.
func v2LayeredCases() []model.EvalGoldenCase {
	return []model.EvalGoldenCase{
		{
			CaseKey: "ke-0001", SchemaVersion: 2, Query: "在IP内：星海遗响", QueryLanguage: "zh",
			ViewerContext:     model.JSONB(`{"principal_key":"anon"}`),
			ExpectedCitations: model.JSONB(`[{"content_id":11,"content_version":1}]`),
			Classification:    model.JSONB(`{"content_type":"mod","primary_layer":"known_item_exact","split":"dev","ip_scope":true}`),
			RelevantEvidence:  model.JSONB(`[{"content_id":11,"content_version":1,"source_start":10,"source_end":40,"chunk_key":"k11-a","chunking_version":1}]`),
		},
		{
			CaseKey: "ke-0002", SchemaVersion: 2, Query: "Blender 插件安装教程", QueryLanguage: "mixed",
			ViewerContext:     model.JSONB(`{"principal_key":"anon"}`),
			ExpectedCitations: model.JSONB(`[{"content_id":12,"content_version":1}]`),
			Classification:    model.JSONB(`{"content_type":"guide","primary_layer":"known_item_exact","split":"test","ip_scope":false}`),
		},
		{
			CaseKey: "sd-0001", SchemaVersion: 2, Query: "怎么给游戏加模组", QueryLanguage: "zh",
			ViewerContext:     model.JSONB(`{"principal_key":"anon"}`),
			ExpectedCitations: model.JSONB(`[{"content_id":21,"content_version":1}]`),
			AnswerRubric:      model.JSONB(`{"acceptable_content_ids":[22]}`),
			Classification:    model.JSONB(`{"content_type":"mod","primary_layer":"semantic_discovery","split":"dev"}`),
		},
		{
			CaseKey: "hn-0001", SchemaVersion: 2, Query: "星海遗响 相似mod", QueryLanguage: "zh",
			ViewerContext:       model.JSONB(`{"principal_key":"anon"}`),
			ExpectedCitations:   model.JSONB(`[{"content_id":31,"content_version":1}]`),
			ForbiddenContentIDs: model.JSONB(`[32]`),
			AnswerRubric:        model.JSONB(`{"forbidden_reasons":{"32":"same IP, wrong modloader"}}`),
			Classification:      model.JSONB(`{"content_type":"mod","primary_layer":"hard_neighbor","split":"dev"}`),
		},
		{
			CaseKey: "na-0001", SchemaVersion: 2, Query: "有没有叫星海遗响的作品", QueryLanguage: "zh",
			ViewerContext:  model.JSONB(`{"principal_key":"anon"}`),
			Classification: model.JSONB(`{"content_type":"other","primary_layer":"no_answer","split":"dev","no_answer_strategy":"strict_not_found"}`),
		},
		{
			CaseKey: "na-0002", SchemaVersion: 2, Query: "推荐一些星际题材新作", QueryLanguage: "zh",
			ViewerContext:  model.JSONB(`{"principal_key":"anon"}`),
			Classification: model.JSONB(`{"content_type":"other","primary_layer":"no_answer","split":"dev","no_answer_strategy":"related_recommendation_allowed"}`),
		},
		{
			CaseKey: "vi-0001", SchemaVersion: 2, Query: "未公开的机密mod", QueryLanguage: "zh",
			ViewerContext:       model.JSONB(`{"principal_key":"anon"}`),
			ExpectedCitations:   model.JSONB(`[{"content_id":41,"content_version":1}]`),
			ForbiddenContentIDs: model.JSONB(`[42]`),
			Classification:      model.JSONB(`{"content_type":"mod","primary_layer":"visibility","split":"dev","corpus_visibility":"private"}`),
		},
	}
}

// principalAwareStubRetriever answers from a per-principal script so the
// double-principal expansion of the visibility layer is observable.
func principalAwareStubRetriever(t *testing.T, calls map[string][]int64) RetrieverV2 {
	t.Helper()
	return func(_ context.Context, query string, viewer ViewerIdentity, topK int) ([]Retrieved, error) {
		if query == "boom" {
			return nil, errors.New("retriever exploded")
		}
		ids := calls[viewer.PrincipalKey]
		hits := make([]Retrieved, 0, len(ids))
		for _, id := range ids {
			hits = append(hits, Retrieved{ContentID: id, Score: float64(100 - len(hits))})
		}
		return hits, nil
	}
}

// stubRegistry resolves both v2 runtime principals without a database;
// fixture:viewer-anon maps to a scripted numeric id.
type stubRegistry struct{ fixtureID int64 }

func (r stubRegistry) ResolvePrincipal(_ context.Context, key string) (ViewerIdentity, error) {
	switch key {
	case PrincipalAnon:
		return ViewerIdentity{PrincipalKey: key, IsAnonymous: true}, nil
	case PrincipalFixtureViewerAnon:
		return ViewerIdentity{PrincipalKey: key, ViewerUserID: r.fixtureID, IsFixture: true}, nil
	default:
		return ViewerIdentity{}, classifyPrincipalError(key)
	}
}

func layerGroup(t *testing.T, result *LayeredRetrievalResult, key string) *LayerMetrics {
	t.Helper()
	for _, g := range result.Groups {
		if g.GroupKey == key {
			return g
		}
	}
	t.Fatalf("group %q not found in %+v", key, result.Groups)
	return nil
}

func TestRunLayeredRetrievalEvalGroupingAndTiers(t *testing.T) {
	calls := map[string][]int64{
		// ke hit + sd expected+acceptable + hn expected+trap + vi expected and
		// the vi leak (42 is vi-0001's restricted doc)
		PrincipalAnon: {11, 21, 22, 31, 41, 32, 42},
	}
	retriever := principalAwareStubRetriever(t, calls)
	ctx := context.Background()

	result, err := RunLayeredRetrievalEval(ctx, v2LayeredCases(), stubRegistry{fixtureID: 500}, retriever,
		LayeredEvalOptions{Split: SplitDev, IncludeNoSplitCases: false, TopK: 10}, RunSpec{RunKey: "v2-test"}, Environment{})
	if err != nil {
		t.Fatalf("layered eval: %v", err)
	}

	// Split filter: ke-0002 is the test-split row and must not run.
	for _, cr := range result.PerCase {
		if cr.CaseKey == "ke-0002" {
			t.Fatal("test-split case leaked into a dev run")
		}
	}
	if result.DatasetSize != 6 {
		t.Fatalf("dataset size = %d, want 6 dev cases", result.DatasetSize)
	}

	// ke: full-title hit at rank 1 → recall@1 = 1
	ke := layerGroup(t, result, "known_item_exact/ip_scoped")
	if ke.CaseCount != 1 || ke.RecallAt1.Value != 1 || ke.RecallAt1.Denominator != 1 {
		t.Fatalf("ke group = %+v", ke)
	}
	// 「在IP内：」 uses a full-width colon: the ASCII-colon marker (#319
	// subset) must not fire on it.
	for _, cr := range result.PerCase {
		if cr.CaseKey == "ke-0001" && cr.ASCIIColon {
			t.Fatal("full-width colon must not count as the ASCII-colon subset")
		}
	}

	// sd: expected at rank 2, acceptable at rank 3 → graded nDCG uses the
	// ideal (expected first) ordering.
	sd := layerGroup(t, result, "semantic_discovery")
	if sd.CaseCount != 1 || sd.RecallAt10.Value != 1 {
		t.Fatalf("sd group = %+v", sd)
	}
	if sd.NDCGAt10.Value <= 0.5 || sd.NDCGAt10.Value >= 1 {
		t.Fatalf("sd graded ndcg = %f, want strictly between 0.5 and 1 (acceptable partial credit)", sd.NDCGAt10.Value)
	}

	// hn: expected retrieved AND forbidden hit within top-10 (both reported)
	hn := layerGroup(t, result, "hard_neighbor")
	if hn.RecallAt10.Value != 1 || hn.TrapHitAt10.Value != 1 || hn.TrapHitAt10.Denominator != 1 {
		t.Fatalf("hn group = %+v", hn)
	}

	// na: both strategies report as their own groups; no expected tier means
	// recall denominators are 0 (rates stay honest, not zero-pass).
	strict := layerGroup(t, result, "no_answer/strict_not_found")
	related := layerGroup(t, result, "no_answer/related_recommendation_allowed")
	if strict.CaseCount != 1 || related.CaseCount != 1 {
		t.Fatalf("na groups = %+v / %+v", strict, related)
	}
	if strict.RecallAt10.Denominator != 0 {
		t.Fatalf("na recall denominator = %f, want 0", strict.RecallAt10.Denominator)
	}

	// vi: the single visibility case expanded into both principals
	anon := layerGroup(t, result, "visibility/anon")
	fixture := layerGroup(t, result, "visibility/"+PrincipalFixtureViewerAnon)
	if anon.CaseCount != 1 || fixture.CaseCount != 1 {
		t.Fatalf("vi groups = %+v / %+v", anon, fixture)
	}
	// the stub returns the forbidden 42 to every principal → leak rate 1
	if anon.VisibilityLeakRate.Value != 1 {
		t.Fatalf("vi anon leak rate = %+v", anon.VisibilityLeakRate)
	}
}

func TestRunLayeredRetrievalEvalTestSplitGate(t *testing.T) {
	ctx := context.Background()
	retriever := principalAwareStubRetriever(t, nil)

	if _, err := RunLayeredRetrievalEval(ctx, v2LayeredCases(), StaticPrincipalRegistry{}, retriever,
		LayeredEvalOptions{Split: SplitTest}, RunSpec{}, Environment{}); err == nil {
		t.Fatal("test split without the explicit flag must be refused")
	}
	if _, err := RunLayeredRetrievalEval(ctx, v2LayeredCases(), StaticPrincipalRegistry{}, retriever,
		LayeredEvalOptions{Split: SplitTest, ConfirmTestSplitRun: true}, RunSpec{}, Environment{}); err != nil {
		t.Fatalf("test split with the flag must run: %v", err)
	}
}

func TestRunLayeredRetrievalEvalEvidenceResolution(t *testing.T) {
	calls := map[string][]int64{PrincipalAnon: {11}}
	chunks := map[int64][]model.RagChunk{
		11: {
			{ContentID: 11, ChunkIndex: 0, ChunkKey: "k11-a", SourceStart: 0, SourceEnd: 50},
			{ContentID: 11, ChunkIndex: 1, ChunkKey: "k11-b", SourceStart: 50, SourceEnd: 100},
		},
	}
	source := ChunkSource(func(contentID int64) ([]model.RagChunk, error) { return chunks[contentID], nil })

	result, err := RunLayeredRetrievalEval(context.Background(), v2LayeredCases(), stubRegistry{fixtureID: 500},
		principalAwareStubRetriever(t, calls),
		LayeredEvalOptions{Split: SplitDev, TopK: 10, ChunkSource: source}, RunSpec{}, Environment{})
	if err != nil {
		t.Fatalf("layered eval with chunk source: %v", err)
	}

	var keCase *LayeredCaseResult
	for i := range result.PerCase {
		if result.PerCase[i].CaseKey == "ke-0001" {
			keCase = &result.PerCase[i]
		}
	}
	if keCase == nil {
		t.Fatal("ke-0001 missing from per-case results")
	}
	// span [10,40) resolves into chunk k11-a; the snapshot key matches so no
	// drift; the retrieved id 11 is the span's content → evidence hit.
	if keCase.EvidenceTotal != 1 || keCase.EvidenceResolved != 1 || keCase.EvidenceSnapshotDrift != 0 || !keCase.EvidenceHitAt10 {
		t.Fatalf("ke evidence fields = %+v", keCase)
	}
	ke := layerGroup(t, result, "known_item_exact/ip_scoped")
	if ke.EvidenceResolved.Value != 1 || ke.EvidenceResolved.Denominator != 1 {
		t.Fatalf("ke evidence group = %+v", ke.EvidenceResolved)
	}
}

func TestRunLayeredRetrievalEvalFailureAndDegenerate(t *testing.T) {
	cases := []model.EvalGoldenCase{
		{
			CaseKey: "bad-0001", SchemaVersion: 2, Query: "boom", QueryLanguage: "zh",
			ViewerContext:  model.JSONB(`{"principal_key":"anon"}`),
			Classification: model.JSONB(`{"content_type":"other","primary_layer":"semantic_discovery","split":"dev"}`),
		},
		{
			// v1-shaped row inside a v2 run: numeric viewer, no layer, no split
			CaseKey: "legacy-0001", SchemaVersion: 1, Query: "old query", QueryLanguage: "zh",
			ViewerContext:      model.JSONB(`{"viewer_user_id":7}`),
			RelevantContentIDs: model.JSONB(`[71]`),
			Classification:     model.JSONB(`{"content_type":"mod","popularity":"cold"}`),
		},
	}
	retriever := principalAwareStubRetriever(t, map[string][]int64{PrincipalAnon: {71}})
	result, err := RunLayeredRetrievalEval(context.Background(), cases, StaticPrincipalRegistry{}, retriever,
		LayeredEvalOptions{Split: "", IncludeNoSplitCases: true, TopK: 10}, RunSpec{}, Environment{})
	if err != nil {
		t.Fatalf("layered eval mixed rows: %v", err)
	}

	var failed, legacy *LayeredCaseResult
	for i := range result.PerCase {
		switch result.PerCase[i].CaseKey {
		case "bad-0001":
			failed = &result.PerCase[i]
		case "legacy-0001":
			legacy = &result.PerCase[i]
		}
	}
	if failed == nil || legacy == nil {
		t.Fatalf("per-case = %+v", result.PerCase)
	}
	// a retrieval failure marks the case unsuccessful; the aggregate keeps
	// going with the successful denominator only.
	if failed.Success || failed.Error == "" {
		t.Fatalf("failed case = %+v", failed)
	}
	unknown := layerGroup(t, result, "unknown")
	if unknown.CaseCount != 1 || unknown.Successful != 1 {
		t.Fatalf("unknown group = %+v", unknown)
	}
	// the failed retrieval lands in its own annotated layer (semantic_discovery)
	// and only lowers that group's successful count
	sd := layerGroup(t, result, "semantic_discovery")
	if sd.CaseCount != 1 || sd.Successful != 0 {
		t.Fatalf("semantic_discovery group = %+v, want the failed case only", sd)
	}
	// legacy numeric identity flows through the v1 path (user 7)
	if legacy.PrincipalKey != PrincipalAnon || legacy.SchemaVersion != 1 {
		t.Fatalf("legacy case = %+v", legacy)
	}
	// relevant_content_ids fallback feeds the expected tier for v1-shaped rows
	if len(legacy.ExpectedIDs) != 1 || legacy.ExpectedIDs[0] != 71 || legacy.RecallAt10 != 1 {
		t.Fatalf("legacy tiers = %+v", legacy)
	}
}

func TestRunLayeredRetrievalEvalUnknownPrincipalFailsClosed(t *testing.T) {
	cases := []model.EvalGoldenCase{
		{
			CaseKey: "ke-0999", SchemaVersion: 2, Query: "q", QueryLanguage: "zh",
			ViewerContext:  model.JSONB(`{"principal_key":"fixture:author:a01"}`),
			Classification: model.JSONB(`{"content_type":"mod","primary_layer":"known_item_exact","split":"dev"}`),
		},
	}
	if _, err := RunLayeredRetrievalEval(context.Background(), cases, StaticPrincipalRegistry{},
		principalAwareStubRetriever(t, nil), LayeredEvalOptions{Split: SplitDev}, RunSpec{}, Environment{}); !errors.Is(err, ErrPrincipalV21Reserved) {
		t.Fatalf("v2.1 principal error = %v, want ErrPrincipalV21Reserved", err)
	}
}

func TestRunLayeredRetrievalEvalIPScopeSubreport(t *testing.T) {
	result, err := RunLayeredRetrievalEval(context.Background(), v2LayeredCases(), StaticPrincipalRegistry{},
		principalAwareStubRetriever(t, map[string][]int64{PrincipalAnon: {12}}),
		LayeredEvalOptions{Split: SplitTest, ConfirmTestSplitRun: true, TopK: 10}, RunSpec{}, Environment{})
	if err != nil {
		t.Fatalf("test-split run: %v", err)
	}
	global := layerGroup(t, result, "known_item_exact/global")
	if global.CaseCount != 1 {
		t.Fatalf("global ke group = %+v", global)
	}
	var keCase *LayeredCaseResult
	for i := range result.PerCase {
		if result.PerCase[i].CaseKey == "ke-0002" {
			keCase = &result.PerCase[i]
		}
	}
	if keCase == nil || keCase.IPScope == nil || *keCase.IPScope {
		t.Fatalf("global ke case = %+v", keCase)
	}
	if keCase.ASCIIColon {
		t.Fatal("query without an ASCII colon must not flag the #319 subset")
	}
}

func TestGroupKeyOf(t *testing.T) {
	ipScoped := true
	cases := []struct {
		cl        Classification
		principal string
		want      string
	}{
		{Classification{PrimaryLayer: LayerKnownItemExact, IPScope: &ipScoped}, PrincipalAnon, "known_item_exact/ip_scoped"},
		{Classification{PrimaryLayer: LayerKnownItemExact}, PrincipalAnon, "known_item_exact/global"},
		{Classification{PrimaryLayer: LayerNoAnswer, NoAnswerStrategy: NoAnswerStrictNotFound}, PrincipalAnon, "no_answer/strict_not_found"},
		{Classification{PrimaryLayer: LayerVisibility}, PrincipalAnon, "visibility/anon"},
		{Classification{PrimaryLayer: LayerBodyEvidence}, PrincipalAnon, "body_evidence"},
		{Classification{}, PrincipalAnon, "unknown"},
	}
	for _, tc := range cases {
		if got := groupKeyOf(tc.cl, tc.principal); got != tc.want {
			t.Fatalf("groupKeyOf(%+v) = %q, want %q", tc.cl, got, tc.want)
		}
	}
}

func TestLayeredCaseResultJSONShape(t *testing.T) {
	// The artifact must carry raw numerator/denominator + CI (§8), never a
	// bare accuracy number.
	ke := LayerMetrics{GroupKey: "known_item_exact/global", RecallAt1: NewRate(9, 12)}
	if ke.RecallAt1.Value != 0.75 || ke.RecallAt1.Numerator != 9 || ke.RecallAt1.Denominator != 12 {
		t.Fatalf("ke metrics = %+v", ke)
	}
	if fmt.Sprintf("%.3f", ke.RecallAt1.CI95Low) == fmt.Sprintf("%.3f", ke.RecallAt1.CI95High) {
		t.Fatalf("CI must not be a point: %+v", ke.RecallAt1)
	}
}
