package rageval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const citationDiagnosisSchemaVersion = 1

// CitationDiagnosisMetadata describes the retrieval entity and corpus scope
// so a report cannot be mistaken for a legacy content-level baseline.
type CitationDiagnosisMetadata struct {
	RetrieverVersion      string `json:"retriever_version"`
	HitEntity             string `json:"hit_entity"`
	CorpusScope           string `json:"corpus_scope"`
	CorpusContents        int64  `json:"corpus_contents"`
	CorpusContentChecksum string `json:"corpus_content_checksum,omitempty"`
	CorpusEmbeddings      int64  `json:"corpus_embeddings"`
	RequestedTopK         int    `json:"requested_top_k"`
	BM25TopK              int    `json:"bm25_top_k"`
	VectorTopK            int    `json:"vector_top_k"`
	FinalTopK             int    `json:"final_top_k"`
	RRFK                  int    `json:"rrf_k"`
	DatasetChecksum       string `json:"dataset_checksum"`
	ChunkingVersion       string `json:"chunking_version"`
	IndexVersion          string `json:"index_version"`
	EmbeddingModel        string `json:"embedding_model"`
	ProjectionGeneration  int    `json:"projection_generation"`
	QueryEmbeddingStandin bool   `json:"query_embedding_standin"`
	CaseSampleLimit       int    `json:"case_sample_limit,omitempty"`
}

type CitationDiagnosisMetrics struct {
	CitationPrecision             float64 `json:"citation_precision"`
	CitationCoverage              float64 `json:"citation_coverage"`
	DeduplicatedCitationPrecision float64 `json:"deduplicated_citation_precision"`
	DeduplicatedCitationCoverage  float64 `json:"deduplicated_citation_coverage"`
	MeanProducedHits              float64 `json:"mean_produced_hits"`
	MeanUniqueHits                float64 `json:"mean_unique_content_hits"`
	CaseCount                     int     `json:"case_count"`
}

type CitationDiagnosisHit struct {
	Rank        int     `json:"rank"`
	ContentID   int64   `json:"content_id"`
	Version     int64   `json:"content_version,omitempty"`
	Score       float64 `json:"score"`
	ChunkKey    string  `json:"chunk_key,omitempty"`
	Title       string  `json:"title,omitempty"`
	Heading     string  `json:"heading,omitempty"`
	TextExcerpt string  `json:"text_excerpt,omitempty"`
	SourceStart int     `json:"source_start,omitempty"`
	SourceEnd   int     `json:"source_end,omitempty"`
}

type CitationDiagnosisCase struct {
	CaseKey                   string                 `json:"case_key"`
	Query                     string                 `json:"query"`
	ExpectedCitations         []Citation             `json:"expected_citations,omitempty"`
	RawCitationPrecision      float64                `json:"raw_citation_precision"`
	RawCitationCoverage       float64                `json:"raw_citation_coverage"`
	DeduplicatedCitationPrec  float64                `json:"deduplicated_citation_precision"`
	DeduplicatedCitationCover float64                `json:"deduplicated_citation_coverage"`
	ProducedHitCount          int                    `json:"produced_hit_count"`
	UniqueContentHitCount     int                    `json:"unique_content_hit_count"`
	Hits                      []CitationDiagnosisHit `json:"hits,omitempty"`
}

type CitationDiagnosisBaseline struct {
	RetrieverVersion string           `json:"retriever_version"`
	HitEntity        string           `json:"hit_entity"`
	ChunkingVersion  string           `json:"chunking_version"`
	IndexVersion     string           `json:"index_version"`
	DatasetSize      int              `json:"dataset_size"`
	CorpusContents   int64            `json:"corpus_contents"`
	CorpusEmbeddings int64            `json:"corpus_embeddings"`
	ObservedMaxHits  int              `json:"observed_max_hits"`
	Metrics          RetrieverMetrics `json:"metrics"`
}

type CitationDiagnosisReport struct {
	SchemaVersion  int                         `json:"schema_version"`
	GeneratedAt    string                      `json:"generated_at"`
	Current        CitationDiagnosisMetadata   `json:"current"`
	CurrentMetrics CitationDiagnosisMetrics    `json:"current_metrics"`
	Baselines      []CitationDiagnosisBaseline `json:"baselines"`
	Findings       []string                    `json:"findings"`
	Cases          []CitationDiagnosisCase     `json:"cases"`
}

// BuildCitationDiagnosisReport compares one current run with the immutable
// baseline artifact and makes entity/K/corpus mismatches explicit.
func BuildCitationDiagnosisReport(result *RetrieverResult, metadata CitationDiagnosisMetadata, baseline *BaselineArtifact) CitationDiagnosisReport {
	report := CitationDiagnosisReport{
		SchemaVersion: citationDiagnosisSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Current:       metadata,
		Baselines:     make([]CitationDiagnosisBaseline, 0),
		Findings:      make([]string, 0),
		Cases:         make([]CitationDiagnosisCase, 0),
	}
	if baseline != nil {
		for _, retriever := range baseline.Retrievers {
			report.Baselines = append(report.Baselines, CitationDiagnosisBaseline{
				RetrieverVersion: retriever.RetrieverVersion,
				HitEntity:        baselineHitEntity(retriever.RetrieverVersion),
				ChunkingVersion:  retriever.ChunkingVersion,
				IndexVersion:     retriever.IndexVersion,
				DatasetSize:      retriever.DatasetSize,
				CorpusContents:   retriever.Environment.CorpusContents,
				CorpusEmbeddings: retriever.Environment.CorpusEmbeddings,
				ObservedMaxHits:  maxObservedHits(retriever.PerCase),
				Metrics:          retriever.Metrics,
			})
		}
	}
	if result == nil {
		return report
	}
	var rawPrecision, rawCoverage, dedupPrecision, dedupCoverage float64
	var producedHits, uniqueHits int
	evaluatedCases := 0
	for _, perCase := range result.PerCase {
		if !perCase.Success {
			continue
		}
		evaluatedCases++
		produced := citationsOf(perCase.Retrieved)
		expected := make(map[Citation]bool, len(perCase.ExpectedCitations))
		for _, citation := range perCase.ExpectedCitations {
			expected[citation] = true
		}
		unique := DeduplicateCitations(produced)
		caseReport := CitationDiagnosisCase{
			CaseKey:                   perCase.CaseKey,
			Query:                     perCase.Query,
			ExpectedCitations:         perCase.ExpectedCitations,
			RawCitationPrecision:      CitationPrecision(produced, expected),
			RawCitationCoverage:       CitationCoverage(produced, expected),
			DeduplicatedCitationPrec:  CitationPrecisionDeduplicated(produced, expected),
			DeduplicatedCitationCover: CitationCoverageDeduplicated(produced, expected),
			ProducedHitCount:          len(produced),
			UniqueContentHitCount:     len(unique),
			Hits:                      diagnosisHits(perCase.Retrieved),
		}
		if metadata.CaseSampleLimit <= 0 || len(report.Cases) < metadata.CaseSampleLimit {
			report.Cases = append(report.Cases, caseReport)
		}
		rawPrecision += caseReport.RawCitationPrecision
		rawCoverage += caseReport.RawCitationCoverage
		dedupPrecision += caseReport.DeduplicatedCitationPrec
		dedupCoverage += caseReport.DeduplicatedCitationCover
		producedHits += caseReport.ProducedHitCount
		uniqueHits += caseReport.UniqueContentHitCount
	}
	if evaluatedCases > 0 {
		count := float64(evaluatedCases)
		report.CurrentMetrics = CitationDiagnosisMetrics{
			CitationPrecision:             rawPrecision / count,
			CitationCoverage:              rawCoverage / count,
			DeduplicatedCitationPrecision: dedupPrecision / count,
			DeduplicatedCitationCoverage:  dedupCoverage / count,
			MeanProducedHits:              float64(producedHits) / count,
			MeanUniqueHits:                float64(uniqueHits) / count,
			CaseCount:                     evaluatedCases,
		}
		report.Findings = append(report.Findings, fmt.Sprintf("current raw citation precision=%.3f coverage=%.3f; deduplicated precision=%.3f coverage=%.3f", report.CurrentMetrics.CitationPrecision, report.CurrentMetrics.CitationCoverage, report.CurrentMetrics.DeduplicatedCitationPrecision, report.CurrentMetrics.DeduplicatedCitationCoverage))
	}
	report.Findings = append(report.Findings, diagnosisFindings(metadata, report.Baselines)...)
	return report
}

func diagnosisHits(hits []Retrieved) []CitationDiagnosisHit {
	out := make([]CitationDiagnosisHit, 0, len(hits))
	for rank, hit := range hits {
		item := CitationDiagnosisHit{Rank: rank + 1, ContentID: hit.ContentID, Version: hit.ContentVersion, Score: hit.Score}
		if hit.Evidence != nil {
			item.ChunkKey = hit.Evidence.ChunkKey
			item.Title = hit.Evidence.Title
			item.Heading = hit.Evidence.Heading
			item.TextExcerpt = excerpt(hit.Evidence.Text, 320)
			item.SourceStart = hit.Evidence.SourceStart
			item.SourceEnd = hit.Evidence.SourceEnd
		}
		out = append(out, item)
	}
	return out
}

func excerpt(value string, max int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func baselineHitEntity(version string) string {
	if strings.HasPrefix(version, "keyword-") || strings.HasPrefix(version, "vector-") {
		return "content"
	}
	return "unknown"
}

func maxObservedHits(cases []CaseResult) int {
	max := 0
	for _, perCase := range cases {
		if len(perCase.Retrieved) > max {
			max = len(perCase.Retrieved)
		}
	}
	return max
}

func diagnosisFindings(current CitationDiagnosisMetadata, baselines []CitationDiagnosisBaseline) []string {
	findings := make([]string, 0)
	for _, baseline := range baselines {
		if baseline.HitEntity != current.HitEntity {
			findings = append(findings, fmt.Sprintf("entity mismatch: current=%s baseline[%s]=%s", current.HitEntity, baseline.RetrieverVersion, baseline.HitEntity))
		}
		if baseline.CorpusContents != 0 && current.CorpusContents != 0 && baseline.CorpusContents != current.CorpusContents {
			findings = append(findings, fmt.Sprintf("corpus count mismatch: current=%d baseline[%s]=%d", current.CorpusContents, baseline.RetrieverVersion, baseline.CorpusContents))
		}
		if current.RequestedTopK != 0 && baseline.ObservedMaxHits != 0 && current.RequestedTopK != baseline.ObservedMaxHits {
			findings = append(findings, fmt.Sprintf("candidate cardinality mismatch: current_requested_k=%d baseline[%s]_observed_max=%d", current.RequestedTopK, baseline.RetrieverVersion, baseline.ObservedMaxHits))
		}
	}
	return findings
}

// WriteCitationDiagnosisReport writes an opt-in local report and never emits
// credentials or raw provider responses.
func WriteCitationDiagnosisReport(path string, report CitationDiagnosisReport) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("citation diagnosis report path is empty")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal citation diagnosis report: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create citation diagnosis report directory: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write citation diagnosis report: %w", err)
	}
	return nil
}
