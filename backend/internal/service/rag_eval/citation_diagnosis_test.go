package rageval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCitationDiagnosisReportExplainsMismatchedBaseline(t *testing.T) {
	result := &RetrieverResult{
		DatasetSize: 1,
		PerCase: []CaseResult{{
			CaseKey:           "duplicate-chunks",
			Query:             "安装模组",
			Success:           true,
			ExpectedCitations: []Citation{{ContentID: 1, ContentVersion: 1}},
			Retrieved: []Retrieved{
				{ContentID: 1, ContentVersion: 1, Score: 3, Evidence: &RetrievedEvidence{ChunkKey: "chunk-1", Title: "命中", Text: "答案片段"}},
				{ContentID: 1, ContentVersion: 1, Score: 2, Evidence: &RetrievedEvidence{ChunkKey: "chunk-2", Title: "命中", Text: "重复片段"}},
				{ContentID: 9, ContentVersion: 1, Score: 1, Evidence: &RetrievedEvidence{ChunkKey: "chunk-9", Title: "相似", Text: "相似但非答案"}},
			},
		}},
	}
	baseline := &BaselineArtifact{Retrievers: []RetrieverResult{
		{
			RetrieverVersion: "keyword-tsvector-simple-v1",
			DatasetSize:      63,
			Environment:      Environment{CorpusContents: 253, CorpusEmbeddings: 201},
			PerCase:          []CaseResult{{Retrieved: []Retrieved{{ContentID: 1}, {ContentID: 2}, {ContentID: 3}, {ContentID: 4}, {ContentID: 5}}}},
		},
		{
			RetrieverVersion: "vector-pgvector-ivfflat-v1",
			DatasetSize:      63,
			Environment:      Environment{CorpusContents: 253, CorpusEmbeddings: 201},
			PerCase:          []CaseResult{{Retrieved: make([]Retrieved, 20)}},
		},
	}}
	report := BuildCitationDiagnosisReport(result, CitationDiagnosisMetadata{
		RetrieverVersion: "hybrid-rrf-pg-fallback-v1",
		HitEntity:        "chunk",
		CorpusContents:   169,
		RequestedTopK:    10,
	}, baseline)

	if got := report.CurrentMetrics.CitationPrecision; got != 2.0/3.0 {
		t.Errorf("raw citation precision = %v, want 2/3", got)
	}
	if got := report.CurrentMetrics.DeduplicatedCitationPrecision; got != 0.5 {
		t.Errorf("deduplicated citation precision = %v, want 0.5", got)
	}
	if got := report.CurrentMetrics.MeanProducedHits; got != 3 {
		t.Errorf("mean produced hits = %v, want 3", got)
	}
	if got := report.CurrentMetrics.MeanUniqueHits; got != 2 {
		t.Errorf("mean unique hits = %v, want 2", got)
	}
	joined := strings.Join(report.Findings, "\n")
	for _, want := range []string{"entity mismatch", "corpus count mismatch", "candidate cardinality mismatch"} {
		if !strings.Contains(joined, want) {
			t.Errorf("findings do not contain %q: %s", want, joined)
		}
	}
	if len(report.Cases[0].Hits) != 3 || report.Cases[0].Hits[0].TextExcerpt != "答案片段" {
		t.Errorf("diagnostic hit evidence = %#v", report.Cases[0].Hits)
	}
}

func TestWriteCitationDiagnosisReportCreatesRedactedArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "diagnosis.json")
	if err := WriteCitationDiagnosisReport(path, CitationDiagnosisReport{SchemaVersion: citationDiagnosisSchemaVersion}); err != nil {
		t.Fatalf("write diagnosis report: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("diagnosis report missing: %v", err)
	}
}
