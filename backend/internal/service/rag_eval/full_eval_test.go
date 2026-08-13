package rageval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// Full-eval gates:
//   OMNICRAFT_RAG_FULL_EVAL=1            run the full-set evaluation against a
//                                        seeded local PostgreSQL (corpus from
//                                        backend/testdata/rag_eval_seed.sql).
//   OMNICRAFT_RAG_WRITE_ARTIFACTS=1      additionally rewrite the committed
//                                        golden-set fixture and baseline
//                                        artifact files (off in CI full-infra
//                                        so latency differences never dirty
//                                        the tree).
//
// The committed numbers in backend/testdata/rag_eval_baseline.json are real
// measurements produced by this test against the local corpus, never
// pre-filled.

const (
	fullEvalEnv          = "OMNICRAFT_RAG_FULL_EVAL"
	writeArtifacts       = "OMNICRAFT_RAG_WRITE_ARTIFACTS"
	evalDSNEnv           = "OMNICRAFT_RAG_EVAL_DSN"
	defaultEvalDSN       = "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable"
	baselineArtifactPath = "../../../testdata/rag_eval_baseline.json"
)

// distribution targets from the confirmed spec §6/§7.
const (
	minFullSetCases = 50
	minColdShare    = 0.20
	minZhShare      = 0.50
	minEnShare      = 0.20
	minMixedShare   = 0.10
)

func fullEvalEnabled() bool {
	return os.Getenv(fullEvalEnv) == "1"
}

func openEvalDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(evalDSNEnv)
	if strings.TrimSpace(dsn) == "" {
		dsn = defaultEvalDSN
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect eval postgres: %v", err)
	}
	return db
}

// TestFullSetContracts validates the full golden set in PostgreSQL: size and
// language/popularity distribution targets, per-case language-label
// consistency with the classifier and corpus presence. Runs only under
// OMNICRAFT_RAG_FULL_EVAL=1.
func TestFullSetContracts(t *testing.T) {
	if !fullEvalEnabled() {
		t.Skip("full-set contract check runs only with OMNICRAFT_RAG_FULL_EVAL=1 (full-infra)")
	}
	db := openEvalDB(t)
	ctx := context.Background()
	repo := repository.NewRagEvaluationRepository(db)

	cases, err := repo.ListActiveGoldenCases(ctx)
	if err != nil {
		t.Fatalf("list active golden cases: %v", err)
	}
	if len(cases) < minFullSetCases {
		t.Fatalf("full set has %d cases, want >= %d (run backend/testdata/rag_eval_seed.sql first)", len(cases), minFullSetCases)
	}

	counts := map[string]int{}
	cold := 0
	for _, c := range cases {
		if c.QueryLanguage != ClassifyQueryLanguage(c.Query) {
			t.Errorf("case %s query_language %q != classifier %q", c.CaseKey, c.QueryLanguage, ClassifyQueryLanguage(c.Query))
		}
		counts[c.QueryLanguage]++
		var cl Classification
		if err := json.Unmarshal(c.Classification, &cl); err != nil {
			t.Errorf("case %s invalid classification: %v", c.CaseKey, err)
			continue
		}
		if cl.Popularity == "cold" {
			cold++
		}
	}
	total := float64(len(cases))
	assertShare(t, "zh", float64(counts["zh"])/total, minZhShare)
	assertShare(t, "en", float64(counts["en"])/total, minEnShare)
	assertShare(t, "mixed", float64(counts["mixed"])/total, minMixedShare)
	assertShare(t, "cold", float64(cold)/total, minColdShare)

	// Corpus sanity: the evaluation corpus must exist alongside the demo data.
	var corpusCount int64
	if err := db.Raw(`SELECT count(*) FROM content_items WHERE id BETWEEN 1001 AND 1039`).Scan(&corpusCount).Error; err != nil {
		t.Fatalf("corpus count: %v", err)
	}
	if corpusCount < 30 {
		t.Fatalf("eval corpus too small (%d rows in 1001..1039); run backend/testdata/rag_eval_seed.sql", corpusCount)
	}
	t.Logf("full set: %d cases, zh=%d en=%d mixed=%d cold=%d", len(cases), counts["zh"], counts["en"], counts["mixed"], cold)
}

func assertShare(t *testing.T, name string, got, min float64) {
	t.Helper()
	if got < min {
		t.Errorf("%s share = %.3f, want >= %.3f", name, got, min)
	}
}

// TestFullEvalBaseline runs the keyword-only and vector-only baselines over
// the full golden set through the production repositories, records eval_runs
// rows, regenerates the CI-subset fixture (drift guard) and, when
// OMNICRAFT_RAG_WRITE_ARTIFACTS=1, rewrites the committed golden-set and
// baseline artifact files. All numbers are real measurements.
func TestFullEvalBaseline(t *testing.T) {
	if !fullEvalEnabled() {
		t.Skip("full baseline eval runs only with OMNICRAFT_RAG_FULL_EVAL=1 (full-infra)")
	}
	ctx := context.Background()
	db := openEvalDB(t)
	evalRepo := repository.NewRagEvaluationRepository(db)

	cases, err := evalRepo.ListActiveGoldenCases(ctx)
	if err != nil {
		t.Fatalf("list active golden cases: %v", err)
	}
	if len(cases) < minFullSetCases {
		t.Fatalf("full set has %d cases; seed first via backend/testdata/rag_eval_seed.sql", len(cases))
	}

	// Dataset checksum over the canonical body of the active set.
	payloads := make([]GoldenCasePayload, 0, len(cases))
	for _, c := range cases {
		p, err := ModelCaseToPayload(c)
		if err != nil {
			t.Fatalf("convert %s: %v", c.CaseKey, err)
		}
		payloads = append(payloads, p)
	}
	body, err := GoldenSetBody(payloads)
	if err != nil {
		t.Fatalf("golden set body: %v", err)
	}
	datasetChecksum := ChecksumOf(body)

	// Standin embeddings for corpus content that has none (real embeddings
	// from previous provider runs are preserved by DO NOTHING).
	contentIDs := caseContentIDs(cases)
	inserted := ensureStandinEmbeddings(ctx, t, db, contentIDs, 1536)
	t.Logf("standin embeddings inserted for %d corpus rows (real embeddings preserved)", inserted)

	env := Environment{
		GoVersion:             runtime.Version(),
		GOOS:                  runtime.GOOS,
		CorpusContents:        corpusCount(ctx, t, db),
		CorpusEmbeddings:      corpusEmbeddingCount(ctx, t, db),
		QueryEmbeddingModel:   "deterministic-sha256-standin-v1 (local dev; no provider credentials)",
		QueryEmbeddingStandin: true,
		Note:                  "local development measurement; regenerate with a real embedding provider before production claims",
	}

	// keyword-only baseline (existing tsvector + visibility scope path).
	searchRepo := repository.NewSearchRepository(db)
	kw := (&ProductionKeywordAdapter{Search: searchRepo, DB: db}).Retrieve
	kwSpec := RunSpec{
		RunKey:           runKeyFor("keyword", datasetChecksum),
		RetrieverVersion: "keyword-tsvector-simple-v1",
		ChunkingVersion:  "none",
		IndexVersion:     "content-search-index-v1",
		ArtifactPath:     baselineArtifactPath,
		DatasetChecksum:  datasetChecksum,
	}
	kwResult, err := RunRetrievalEval(ctx, cases, kw, 20, kwSpec, env)
	if err != nil {
		t.Fatalf("keyword baseline eval: %v", err)
	}

	// vector-only baseline (existing raw pgvector path + deterministic
	// standin query embeddings).
	embeddingRepo := repository.NewEmbeddingRepository(db)
	vec := (&ProductionVectorAdapter{Embeddings: embeddingRepo, DB: db, Dims: 1536}).Retrieve
	vecSpec := RunSpec{
		RunKey:           runKeyFor("vector", datasetChecksum),
		RetrieverVersion: "vector-pgvector-ivfflat-v1",
		ChunkingVersion:  "none",
		IndexVersion:     "content-embeddings-v1",
		ArtifactPath:     baselineArtifactPath,
		DatasetChecksum:  datasetChecksum,
	}
	vecResult, err := RunRetrievalEval(ctx, cases, vec, 20, vecSpec, env)
	if err != nil {
		t.Fatalf("vector baseline eval: %v", err)
	}

	// eval_runs rows: PostgreSQL is the run-record truth source.
	for _, r := range []*RetrieverResult{kwResult, vecResult} {
		if err := evalRepo.UpsertEvalRun(ctx, resultToRun(r)); err != nil {
			t.Fatalf("upsert eval run %s: %v", r.RunKey, err)
		}
		if _, err := evalRepo.GetEvalRunByKey(ctx, r.RunKey); err != nil {
			t.Fatalf("eval run %s must be readable after upsert: %v", r.RunKey, err)
		}
	}
	t.Logf("keyword baseline: recall@10=%.3f mrr=%.3f ndcg@10=%.3f leak=%d p95=%.1fms",
		kwResult.Metrics.RecallAt10, kwResult.Metrics.MRR, kwResult.Metrics.NDCGAt10,
		kwResult.Metrics.VisibilityLeakCount, kwResult.Metrics.P95RetrievalMs)
	t.Logf("vector baseline: recall@10=%.3f mrr=%.3f ndcg@10=%.3f leak=%d p95=%.1fms",
		vecResult.Metrics.RecallAt10, vecResult.Metrics.MRR, vecResult.Metrics.NDCGAt10,
		vecResult.Metrics.VisibilityLeakCount, vecResult.Metrics.P95RetrievalMs)

	// CI-subset drift guard + optional rewrite.
	regenerateGoldenSetFixture(t, db, evalRepo)

	if os.Getenv(writeArtifacts) != "1" {
		t.Log("OMNICRAFT_RAG_WRITE_ARTIFACTS not set; committed artifact files left untouched")
		return
	}
	writeBaselineArtifact(t, []*RetrieverResult{kwResult, vecResult}, datasetChecksum, len(cases))
}

func caseContentIDs(cases []model.EvalGoldenCase) []int64 {
	set := map[int64]bool{}
	for _, c := range cases {
		relevant, err := ParseInt64List(c.RelevantContentIDs)
		if err == nil {
			for _, id := range relevant {
				set[id] = true
			}
		}
		forbidden, err := ParseInt64List(c.ForbiddenContentIDs)
		if err == nil {
			for _, id := range forbidden {
				set[id] = true
			}
		}
	}
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

// ensureStandinEmbeddings inserts deterministic standin embeddings for corpus
// content without an embedding row. Real embeddings are never overwritten.
func ensureStandinEmbeddings(ctx context.Context, t *testing.T, db *gorm.DB, contentIDs []int64, dims int) int64 {
	t.Helper()
	if len(contentIDs) == 0 {
		return 0
	}
	var rows []struct {
		ID          int64
		Title       string
		Description string
	}
	if err := db.Raw(`
		SELECT ci.id, ci.title, COALESCE(ci.description, '') AS description
		FROM content_items ci
		LEFT JOIN content_embeddings ce ON ce.content_item_id = ci.id
		WHERE ci.id IN ? AND ce.content_item_id IS NULL
	`, contentIDs).Scan(&rows).Error; err != nil {
		t.Fatalf("load corpus rows without embeddings: %v", err)
	}
	var inserted int64
	for _, r := range rows {
		vec := DeterministicEmbedding(r.Title+" "+r.Description, dims)
		res := db.Exec(`
			INSERT INTO content_embeddings (content_item_id, embedding, embedded_at)
			VALUES (?, ?::vector, NOW())
			ON CONFLICT (content_item_id) DO NOTHING
		`, r.ID, formatVector(vec))
		if res.Error != nil {
			t.Fatalf("insert standin embedding for content %d: %v", r.ID, res.Error)
		}
		inserted += res.RowsAffected
	}
	return inserted
}

func formatVector(v []float32) string {
	parts := make([]string, 0, len(v))
	for _, f := range v {
		parts = append(parts, fmt.Sprintf("%g", f))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func corpusCount(ctx context.Context, t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM content_items`).Scan(&n).Error; err != nil {
		t.Fatalf("corpus count: %v", err)
	}
	return n
}

func corpusEmbeddingCount(ctx context.Context, t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM content_embeddings`).Scan(&n).Error; err != nil {
		t.Fatalf("embedding count: %v", err)
	}
	return n
}

func runKeyFor(retriever, datasetChecksum string) string {
	short := datasetChecksum
	if len(short) > 12 {
		short = short[len("sha256:"):][:12]
	}
	return fmt.Sprintf("rag-baseline-%s-%s", retriever, short)
}

func resultToRun(r *RetrieverResult) *model.EvalRun {
	metrics, _ := json.Marshal(r.Metrics)
	environment, _ := json.Marshal(r.Environment)
	return &model.EvalRun{
		RunKey:           r.RunKey,
		DatasetChecksum:  strings.TrimPrefix(r.DatasetChecksum, "sha256:"),
		RetrieverVersion: r.RetrieverVersion,
		ChunkingVersion:  r.ChunkingVersion,
		IndexVersion:     r.IndexVersion,
		Metrics:          model.JSONB(metrics),
		Environment:      model.JSONB(environment),
		ArtifactPath:     "backend/testdata/rag_eval_baseline.json",
	}
}

// regenerateGoldenSetFixture re-exports the 12-case CI subset from the
// database and compares it byte-for-byte with the committed fixture; when
// OMNICRAFT_RAG_WRITE_ARTIFACTS=1 the file is rewritten.
func regenerateGoldenSetFixture(t *testing.T, db *gorm.DB, evalRepo *repository.RagEvaluationRepository) {
	t.Helper()
	ctx := context.Background()
	cases, err := evalRepo.ListActiveGoldenCases(ctx)
	if err != nil {
		t.Fatalf("list active golden cases: %v", err)
	}
	subset := make([]GoldenCasePayload, 0, len(CISubsetCaseKeys))
	byKey := map[string]model.EvalGoldenCase{}
	for _, c := range cases {
		byKey[c.CaseKey] = c
	}
	for _, key := range CISubsetCaseKeys {
		c, ok := byKey[key]
		if !ok {
			t.Fatalf("CI subset case %q missing from the database", key)
		}
		p, err := ModelCaseToPayload(c)
		if err != nil {
			t.Fatalf("convert %s: %v", key, err)
		}
		subset = append(subset, p)
	}
	generated, err := BuildGoldenSetFile(subset)
	if err != nil {
		t.Fatalf("build golden set file: %v", err)
	}
	committed, err := os.ReadFile(GoldenSetFixturePath)
	if os.IsNotExist(err) && os.Getenv(writeArtifacts) == "1" {
		if err := os.WriteFile(GoldenSetFixturePath, generated, 0o644); err != nil {
			t.Fatalf("write golden set fixture: %v", err)
		}
		t.Log("golden set fixture generated from the database")
		return
	}
	if err != nil {
		t.Fatalf("read committed golden set fixture: %v", err)
	}
	if string(generated) != string(committed) {
		if os.Getenv(writeArtifacts) == "1" {
			if err := os.WriteFile(GoldenSetFixturePath, generated, 0o644); err != nil {
				t.Fatalf("rewrite golden set fixture: %v", err)
			}
			t.Log("golden set fixture regenerated from the database")
		} else {
			t.Fatalf("database export of the CI subset differs from the committed fixture; regenerate with OMNICRAFT_RAG_WRITE_ARTIFACTS=1")
		}
	}
}

func writeBaselineArtifact(t *testing.T, retrievers []*RetrieverResult, datasetChecksum string, datasetSize int) {
	t.Helper()
	results := make([]RetrieverResult, 0, len(retrievers))
	for _, r := range retrievers {
		results = append(results, *r)
	}
	artifact, err := BuildBaselineArtifact(results, datasetChecksum, datasetSize)
	if err != nil {
		t.Fatalf("build baseline artifact: %v", err)
	}
	if err := os.WriteFile(baselineArtifactPath, artifact, 0o644); err != nil {
		t.Fatalf("write baseline artifact: %v", err)
	}
	if _, err := ParseBaselineArtifact(artifact); err != nil {
		t.Fatalf("written baseline artifact failed self-validation: %v", err)
	}
	t.Logf("baseline artifact written to %s", baselineArtifactPath)
}

// TestBaselineArtifactSchema validates the committed baseline artifact
// (CI-safe): schema version, checksum integrity, retriever identity and
// metric sanity. Latency numbers are real measurements and may vary between
// runs, so only structure and ranges are asserted.
func TestBaselineArtifactSchema(t *testing.T) {
	data, err := os.ReadFile(baselineArtifactPath)
	if err != nil {
		t.Fatalf("read baseline artifact: %v", err)
	}
	artifact, err := ParseBaselineArtifact(data)
	if err != nil {
		t.Fatalf("baseline artifact failed validation: %v", err)
	}
	if len(artifact.Retrievers) != 2 {
		t.Fatalf("retriever count = %d, want 2 (keyword-only, vector-only)", len(artifact.Retrievers))
	}
	versions := map[string]bool{}
	for _, r := range artifact.Retrievers {
		versions[r.RetrieverVersion] = true
		if len(r.PerCase) != artifact.DatasetSize {
			t.Errorf("retriever %s per_case=%d, dataset_size=%d", r.RetrieverVersion, len(r.PerCase), artifact.DatasetSize)
		}
		m := r.Metrics
		if m.RecallAt10 < 0 || m.RecallAt10 > 1 || m.RecallAt20 < 0 || m.RecallAt20 > 1 ||
			m.MRR < 0 || m.MRR > 1 || m.NDCGAt10 < 0 || m.NDCGAt10 > 1 ||
			m.CitationPrecision < 0 || m.CitationPrecision > 1 ||
			m.CitationCoverage < 0 || m.CitationCoverage > 1 {
			t.Errorf("retriever %s metrics out of range: %+v", r.RetrieverVersion, m)
		}
		if m.DegradationSuccessRate < 0 || m.DegradationSuccessRate > 1 {
			t.Errorf("retriever %s degradation success rate out of range: %v", r.RetrieverVersion, m.DegradationSuccessRate)
		}
		if m.CaseCount != artifact.DatasetSize {
			t.Errorf("retriever %s case_count=%d, dataset_size=%d", r.RetrieverVersion, m.CaseCount, artifact.DatasetSize)
		}
	}
	for _, want := range []string{"keyword-tsvector-simple-v1", "vector-pgvector-ivfflat-v1"} {
		if !versions[want] {
			t.Errorf("baseline missing retriever version %q", want)
		}
	}
}
