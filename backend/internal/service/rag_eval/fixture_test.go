package rageval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

// GoldenSetFixturePath is the committed CI-subset fixture, deterministically
// exported from PostgreSQL by the full-eval test. It is never hand-maintained.
const GoldenSetFixturePath = "../../../testdata/rag_golden_cases.json"

// CISubsetCaseKeys is the deterministic 12-case CI subset: the twelve
// migrated agent-eval cases (existing agent eval + RAG cases). Full-set
// evaluation runs outside CI (full-infra, env-gated).
var CISubsetCaseKeys = []string{
	"exact_keyword_lookup",
	"semantic_paraphrase_lookup",
	"two_cited_contents",
	"no_evidence_refusal",
	"unpublished_private_exclusion",
	"prompt_injection_in_content",
	"forged_citation_attempt",
	"client_forged_context",
	"hidden_content_id_usage_guide",
	"publish_suggestion_forged_resource",
	"provider_timeout_downgrade",
	"rate_limit_downgrade",
}

// TestGoldenSetFixtureSchema validates the committed CI subset: schema
// version, exact case composition, per-case field contract and the
// dataset_checksum consistency (ParseGoldenSetFile recomputes it over the
// canonical body). Any hand-edit of the fixture breaks this test.
func TestGoldenSetFixtureSchema(t *testing.T) {
	data, err := os.ReadFile(GoldenSetFixturePath)
	if err != nil {
		t.Fatalf("read golden set fixture: %v", err)
	}
	file, err := ParseGoldenSetFile(data)
	if err != nil {
		t.Fatalf("golden set fixture failed validation: %v", err)
	}
	if file.SchemaVersion != GoldenSetSchemaVersion {
		t.Fatalf("schema_version = %d, want %d", file.SchemaVersion, GoldenSetSchemaVersion)
	}
	if len(file.Cases) != len(CISubsetCaseKeys) {
		t.Fatalf("subset case count = %d, want %d", len(file.Cases), len(CISubsetCaseKeys))
	}
	seen := map[string]bool{}
	for _, c := range file.Cases {
		if seen[c.CaseKey] {
			t.Fatalf("duplicate case_key %q in fixture", c.CaseKey)
		}
		seen[c.CaseKey] = true
		if c.CaseKey == "" || c.Query == "" {
			t.Fatalf("case missing key or query: %+v", c)
		}
		if c.SchemaVersion != GoldenSetSchemaVersion {
			t.Fatalf("case %s schema_version = %d", c.CaseKey, c.SchemaVersion)
		}
		if c.QueryLanguage != ClassifyQueryLanguage(c.Query) {
			t.Fatalf("case %s query_language %q != classifier %q", c.CaseKey, c.QueryLanguage, ClassifyQueryLanguage(c.Query))
		}
		var cl Classification
		if err := json.Unmarshal(c.Classification, &cl); err != nil {
			t.Fatalf("case %s invalid classification: %v", c.CaseKey, err)
		}
		if cl.ContentType == "" {
			t.Fatalf("case %s classification missing content_type", c.CaseKey)
		}
		if cl.Popularity != "cold" && cl.Popularity != "hot" && cl.Popularity != "normal" {
			t.Fatalf("case %s popularity = %q", c.CaseKey, cl.Popularity)
		}
	}
	for _, want := range CISubsetCaseKeys {
		if !seen[want] {
			t.Fatalf("CI subset case %q is missing from the fixture", want)
		}
	}
}

// TestGoldenSetFixtureExportConsistency proves the committed fixture equals a
// deterministic export of the same rows from PostgreSQL: the twelve cases are
// inserted into an ephemeral database through the production repository, read
// back in canonical order and re-exported; body bytes and checksum must match
// the committed file byte-for-byte. No second truth can drift in.
func TestGoldenSetFixtureExportConsistency(t *testing.T) {
	data, err := os.ReadFile(GoldenSetFixturePath)
	if err != nil {
		t.Fatalf("read golden set fixture: %v", err)
	}
	file, err := ParseGoldenSetFile(data)
	if err != nil {
		t.Fatalf("golden set fixture failed validation: %v", err)
	}
	wantBody, err := GoldenSetBody(file.Cases)
	if err != nil {
		t.Fatalf("canonical body of committed fixture: %v", err)
	}

	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "..", "migrations", "069_rag_evaluation.sql"))

	repo := repository.NewRagEvaluationRepository(db)
	ctx := t.Context()
	for _, payload := range file.Cases {
		c := payloadToModel(payload)
		if err := repo.UpsertGoldenCase(ctx, &c); err != nil {
			t.Fatalf("upsert case %s: %v", payload.CaseKey, err)
		}
	}
	cases, err := repo.ListActiveGoldenCases(ctx)
	if err != nil {
		t.Fatalf("list golden cases: %v", err)
	}
	if len(cases) != len(file.Cases) {
		t.Fatalf("exported case count = %d, want %d", len(cases), len(file.Cases))
	}
	payloads := make([]GoldenCasePayload, 0, len(cases))
	for _, c := range cases {
		p, err := ModelCaseToPayload(c)
		if err != nil {
			t.Fatalf("convert %s: %v", c.CaseKey, err)
		}
		payloads = append(payloads, p)
	}
	gotBody, err := GoldenSetBody(payloads)
	if err != nil {
		t.Fatalf("canonical body of database export: %v", err)
	}
	if string(gotBody) != string(wantBody) {
		t.Fatal("database export differs from committed fixture; regenerate via OMNICRAFT_RAG_FULL_EVAL + OMNICRAFT_RAG_WRITE_ARTIFACTS")
	}
	if got := ChecksumOf(gotBody); got != file.DatasetChecksum {
		t.Fatalf("export checksum %q != fixture checksum %q", got, file.DatasetChecksum)
	}
}

func payloadToModel(p GoldenCasePayload) model.EvalGoldenCase {
	return model.EvalGoldenCase{
		CaseKey:             p.CaseKey,
		SchemaVersion:       p.SchemaVersion,
		Query:               p.Query,
		QueryLanguage:       p.QueryLanguage,
		ViewerContext:       model.JSONB(p.ViewerContext),
		RelevantEvidence:    model.JSONB(p.RelevantEvidence),
		RelevantContentIDs:  model.JSONB(p.RelevantContentIDs),
		ExpectedCitations:   model.JSONB(p.ExpectedCitations),
		ForbiddenContentIDs: model.JSONB(p.ForbiddenContentIDs),
		AnswerRubric:        model.JSONB(p.AnswerRubric),
		Classification:      model.JSONB(p.Classification),
		IsActive:            p.IsActive,
	}
}
