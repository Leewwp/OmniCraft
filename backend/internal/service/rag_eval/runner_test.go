package rageval

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"omnicraft/backend/internal/model"
)

func runnerCaseForTest(t *testing.T, key, query string, relevant []int64, forbidden []int64) model.EvalGoldenCase {
	t.Helper()
	relJSON := model.JSONB(`[]`)
	if len(relevant) > 0 {
		relJSON = model.JSONB(`[` + joinInt64s(relevant) + `]`)
	}
	forbJSON := model.JSONB(`[]`)
	if len(forbidden) > 0 {
		forbJSON = model.JSONB(`[` + joinInt64s(forbidden) + `]`)
	}
	return model.EvalGoldenCase{
		CaseKey:             key,
		SchemaVersion:       1,
		Query:               query,
		QueryLanguage:       "zh",
		ViewerContext:       model.JSONB(`{"viewer_user_id":0}`),
		RelevantContentIDs:  relJSON,
		ForbiddenContentIDs: forbJSON,
		ExpectedCitations:   model.JSONB(`[{"content_id":1,"content_version":1}]`),
		IsActive:            true,
	}
}

func joinInt64s(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func TestRunRetrievalEvalDeterministicTieBreak(t *testing.T) {
	cases := []model.EvalGoldenCase{
		runnerCaseForTest(t, "tie", "query", []int64{1002, 1001}, nil),
	}
	kw := func(_ context.Context, query string, viewerID int64, topK int) ([]Retrieved, error) {
		// Equal scores: the harness must break the tie by content id asc.
		return []Retrieved{
			{ContentID: 1002, Score: 1.0},
			{ContentID: 1001, Score: 1.0},
		}, nil
	}
	result, err := RunRetrievalEval(context.Background(), cases, kw, 20, RunSpec{}, Environment{})
	if err != nil {
		t.Fatalf("RunRetrievalEval: %v", err)
	}
	wantIDs := []int64{1001, 1002}
	if !reflect.DeepEqual(result.PerCase[0].RetrievedIDs, wantIDs) {
		t.Fatalf("tie-break order = %v, want %v", result.PerCase[0].RetrievedIDs, wantIDs)
	}
	if result.PerCase[0].MRR != 1.0 {
		t.Errorf("MRR = %v, want 1.0 (relevant at rank 1 after tie-break)", result.PerCase[0].MRR)
	}
}

func TestRunRetrievalEvalAggregatesMetrics(t *testing.T) {
	cases := []model.EvalGoldenCase{
		runnerCaseForTest(t, "hit", "query", []int64{1001}, nil),
		runnerCaseForTest(t, "miss", "query", []int64{2001}, nil),
		runnerCaseForTest(t, "leak", "query", []int64{1001}, []int64{3003}),
	}
	kw := func(_ context.Context, query string, viewerID int64, topK int) ([]Retrieved, error) {
		return []Retrieved{{ContentID: 1001, Score: 1.0}, {ContentID: 3003, Score: 0.5}, {ContentID: 9999, Score: 0.1}}, nil
	}
	result, err := RunRetrievalEval(context.Background(), cases, kw, 20, RunSpec{}, Environment{})
	if err != nil {
		t.Fatalf("RunRetrievalEval: %v", err)
	}
	if result.Metrics.CaseCount != 3 {
		t.Errorf("case count = %d, want 3", result.Metrics.CaseCount)
	}
	// hit: recall 1 (relevant 1001 retrieved at rank 1); miss: 0; leak: 1.
	if result.Metrics.RecallAt10 != 2.0/3.0 {
		t.Errorf("aggregate recall@10 = %v, want 2/3", result.Metrics.RecallAt10)
	}
	if result.Metrics.MRR != 2.0/3.0 {
		t.Errorf("aggregate MRR = %v, want 2/3", result.Metrics.MRR)
	}
	// The leak case outputs forbidden 3003; the other two do not.
	if result.Metrics.VisibilityLeakCount != 1 {
		t.Errorf("visibility leak count = %d, want 1", result.Metrics.VisibilityLeakCount)
	}
	if !reflect.DeepEqual(result.PerCase[2].LeakedIDs, []int64{3003}) {
		t.Errorf("leaked ids = %v, want [3003]", result.PerCase[2].LeakedIDs)
	}
	if result.Metrics.DegradationSuccessRate != 1 {
		t.Errorf("degradation success rate = %v, want 1", result.Metrics.DegradationSuccessRate)
	}
	if result.Metrics.AnswerEvaluated {
		t.Error("retrieval-only runs must not claim answer evaluation")
	}
	if result.Metrics.Groundedness != nil || result.Metrics.AnswerRelevance != nil {
		t.Error("retrieval-only runs must record nil groundedness/relevance")
	}
	if result.Metrics.P95RetrievalMs < 0 || result.Metrics.MeanLatencyMs < 0 {
		t.Error("latency metrics must be non-negative")
	}
	for _, pc := range result.PerCase {
		if pc.LatencyMs < 0 {
			t.Errorf("case %s latency = %v, want >= 0", pc.CaseKey, pc.LatencyMs)
		}
	}
	if result.DatasetSize != 3 {
		t.Errorf("dataset size = %d, want 3", result.DatasetSize)
	}
}

func TestRunRetrievalEvalFailureCounting(t *testing.T) {
	cases := []model.EvalGoldenCase{
		runnerCaseForTest(t, "ok", "ok-query", []int64{1001}, nil),
		runnerCaseForTest(t, "fail", "fail-query", []int64{1001}, nil),
	}
	kw := func(_ context.Context, query string, viewerID int64, topK int) ([]Retrieved, error) {
		if query == "fail-query" {
			return nil, errRetrievalForTest
		}
		return []Retrieved{{ContentID: 1001, Score: 1.0}}, nil
	}
	result, err := RunRetrievalEval(context.Background(), cases, kw, 20, RunSpec{}, Environment{})
	if err != nil {
		t.Fatalf("RunRetrievalEval: %v", err)
	}
	if result.Metrics.DegradationSuccessRate != 0.5 {
		t.Errorf("degradation success rate = %v, want 0.5", result.Metrics.DegradationSuccessRate)
	}
	if result.PerCase[1].Success {
		t.Error("failed case must be marked unsuccessful")
	}
	if result.PerCase[1].RecallAt10 != 0 {
		t.Errorf("failed case recall = %v, want 0", result.PerCase[1].RecallAt10)
	}
}

var errRetrievalForTest = &retrievalError{msg: "simulated retrieval failure"}

type retrievalError struct{ msg string }

func (e *retrievalError) Error() string { return e.msg }
