package rageval

import (
	"encoding/json"
	"testing"

	"omnicraft/backend/internal/model"
)

func payloadForTest(t *testing.T, caseKey, query, lang string) GoldenCasePayload {
	t.Helper()
	return GoldenCasePayload{
		CaseKey:             caseKey,
		SchemaVersion:       1,
		Query:               query,
		QueryLanguage:       lang,
		ViewerContext:       json.RawMessage(`{"viewer_user_id":0}`),
		RelevantEvidence:    json.RawMessage(`[{"content_id":1001,"content_version":1,"source_start":0,"source_end":0}]`),
		RelevantContentIDs:  json.RawMessage(`[1001]`),
		ExpectedCitations:   json.RawMessage(`[{"content_id":1001,"content_version":1}]`),
		ForbiddenContentIDs: json.RawMessage(`[]`),
		AnswerRubric:        json.RawMessage(`{"deterministic_assertions":["must cite 1001"],"min_judge_score":4}`),
		Classification:      json.RawMessage(`{"content_type":"mod","popularity":"cold"}`),
		IsActive:            true,
	}
}

func TestGoldenSetBodySortsByCaseKey(t *testing.T) {
	b := payloadForTest(t, "case-b", "q-b", "zh")
	a := payloadForTest(t, "case-a", "q-a", "zh")
	bodyAB, err := GoldenSetBody([]GoldenCasePayload{b, a})
	if err != nil {
		t.Fatalf("GoldenSetBody: %v", err)
	}
	bodyBA, err := GoldenSetBody([]GoldenCasePayload{a, b})
	if err != nil {
		t.Fatalf("GoldenSetBody: %v", err)
	}
	if string(bodyAB) != string(bodyBA) {
		t.Fatal("GoldenSetBody must be order-independent (sorted by case_key)")
	}
	if len(bodyAB) == 0 || bodyAB[0] != '{' {
		t.Fatalf("unexpected body shape: %s", bodyAB)
	}
}

func TestGoldenSetChecksumStability(t *testing.T) {
	a := payloadForTest(t, "case-a", "q-a", "zh")
	body, err := GoldenSetBody([]GoldenCasePayload{a})
	if err != nil {
		t.Fatalf("GoldenSetBody: %v", err)
	}
	if got := ChecksumOf(body); got == "" || len(got) != len("sha256:")+64 {
		t.Fatalf("checksum shape = %q", got)
	}
	if ChecksumOf(body) != ChecksumOf(body) {
		t.Fatal("checksum must be deterministic")
	}
}

func TestBuildAndParseGoldenSetFile(t *testing.T) {
	cases := []GoldenCasePayload{
		payloadForTest(t, "case-b", "q-b", "zh"),
		payloadForTest(t, "case-a", "q-a", "en"),
	}
	file, err := BuildGoldenSetFile(cases)
	if err != nil {
		t.Fatalf("BuildGoldenSetFile: %v", err)
	}
	parsed, err := ParseGoldenSetFile(file)
	if err != nil {
		t.Fatalf("ParseGoldenSetFile: %v", err)
	}
	if parsed.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", parsed.SchemaVersion)
	}
	if len(parsed.Cases) != 2 {
		t.Fatalf("case count = %d, want 2", len(parsed.Cases))
	}
	// Parse must validate the checksum against the canonical body.
	if parsed.DatasetChecksum == "" {
		t.Fatal("dataset_checksum missing")
	}
	tampered := append([]byte(nil), file...)
	for i, b := range tampered {
		if b == '"' && i > len(tampered)-80 {
			tampered[i] = '\''
			break
		}
	}
	if _, err := ParseGoldenSetFile(tampered); err == nil {
		t.Fatal("ParseGoldenSetFile must reject a tampered checksum")
	}
}

func TestModelCaseToPayloadRoundTrip(t *testing.T) {
	c := model.EvalGoldenCase{
		CaseKey:             "round-trip",
		SchemaVersion:       1,
		Query:               "测试查询",
		QueryLanguage:       "zh",
		ViewerContext:       model.JSONB(`{"viewer_user_id":42}`),
		RelevantEvidence:    model.JSONB(`[{"content_id":1001,"content_version":1,"source_start":0,"source_end":0}]`),
		RelevantContentIDs:  model.JSONB(`[1001]`),
		ExpectedCitations:   model.JSONB(`[{"content_id":1001,"content_version":1}]`),
		ForbiddenContentIDs: model.JSONB(`[9999]`),
		AnswerRubric:        model.JSONB(`{"min_judge_score":4}`),
		Classification:      model.JSONB(`{"content_type":"article","popularity":"hot"}`),
		IsActive:            true,
	}
	p, err := ModelCaseToPayload(c)
	if err != nil {
		t.Fatalf("ModelCaseToPayload: %v", err)
	}
	if p.CaseKey != c.CaseKey || p.Query != c.Query || p.QueryLanguage != c.QueryLanguage {
		t.Errorf("payload scalar mismatch: %+v", p)
	}
	if string(p.ViewerContext) != `{"viewer_user_id":42}` {
		t.Errorf("viewer_context payload = %s", p.ViewerContext)
	}
	// The payload must round-trip through the canonical body unchanged.
	body, err := GoldenSetBody([]GoldenCasePayload{p})
	if err != nil {
		t.Fatalf("GoldenSetBody: %v", err)
	}
	var decoded GoldenSetFile
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(decoded.Cases) != 1 || string(decoded.Cases[0].RelevantContentIDs) != `[1001]` {
		t.Fatalf("round-trip mismatch: %+v", decoded.Cases)
	}
}
