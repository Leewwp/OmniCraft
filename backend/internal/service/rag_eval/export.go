package rageval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"omnicraft/backend/internal/model"
)

// GoldenSetSchemaVersion is the committed golden-set schema version (1).
const GoldenSetSchemaVersion = 1

// GoldenSetFile is the committed CI-subset fixture shape
// (backend/testdata/rag_golden_cases.json). PostgreSQL is the single source
// of truth; this file is a deterministic export carrying the dataset
// checksum so hand-maintained second truths are detectable.
type GoldenSetFile struct {
	SchemaVersion   int                 `json:"schema_version"`
	DatasetChecksum string              `json:"dataset_checksum"`
	Cases           []GoldenCasePayload `json:"cases"`
}

// GoldenCasePayload is the canonical exported shape of one golden case.
// Raw jsonb payloads round-trip byte-for-byte so the export never reorders
// or truncates stored evidence.
type GoldenCasePayload struct {
	CaseKey             string          `json:"case_key"`
	SchemaVersion       int             `json:"schema_version"`
	Query               string          `json:"query"`
	QueryLanguage       string          `json:"query_language"`
	ViewerContext       json.RawMessage `json:"viewer_context"`
	RelevantEvidence    json.RawMessage `json:"relevant_evidence"`
	RelevantContentIDs  json.RawMessage `json:"relevant_content_ids"`
	ExpectedCitations   json.RawMessage `json:"expected_citations"`
	ForbiddenContentIDs json.RawMessage `json:"forbidden_content_ids"`
	AnswerRubric        json.RawMessage `json:"answer_rubric"`
	Classification      json.RawMessage `json:"classification"`
	IsActive            bool            `json:"is_active"`
}

// ModelCaseToPayload converts a database row into the canonical export shape.
func ModelCaseToPayload(c model.EvalGoldenCase) (GoldenCasePayload, error) {
	valid := func(name string, raw model.JSONB) error {
		if !json.Valid(raw) {
			return fmt.Errorf("golden case %q has invalid jsonb %s: %s", c.CaseKey, name, string(raw))
		}
		return nil
	}
	for name, raw := range map[string]model.JSONB{
		"viewer_context":        c.ViewerContext,
		"relevant_evidence":     c.RelevantEvidence,
		"relevant_content_ids":  c.RelevantContentIDs,
		"expected_citations":    c.ExpectedCitations,
		"forbidden_content_ids": c.ForbiddenContentIDs,
		"answer_rubric":         c.AnswerRubric,
		"classification":        c.Classification,
	} {
		if err := valid(name, raw); err != nil {
			return GoldenCasePayload{}, err
		}
	}
	return GoldenCasePayload{
		CaseKey:             c.CaseKey,
		SchemaVersion:       c.SchemaVersion,
		Query:               c.Query,
		QueryLanguage:       c.QueryLanguage,
		ViewerContext:       json.RawMessage(c.ViewerContext),
		RelevantEvidence:    json.RawMessage(c.RelevantEvidence),
		RelevantContentIDs:  json.RawMessage(c.RelevantContentIDs),
		ExpectedCitations:   json.RawMessage(c.ExpectedCitations),
		ForbiddenContentIDs: json.RawMessage(c.ForbiddenContentIDs),
		AnswerRubric:        json.RawMessage(c.AnswerRubric),
		Classification:      json.RawMessage(c.Classification),
		IsActive:            c.IsActive,
	}, nil
}

// GoldenSetBody serialises the cases canonically (sorted by case_key) into
// the schema envelope without the dataset_checksum field. The checksum is
// computed over exactly these bytes.
func GoldenSetBody(cases []GoldenCasePayload) ([]byte, error) {
	sorted := append([]GoldenCasePayload(nil), cases...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CaseKey < sorted[j].CaseKey })
	return json.Marshal(struct {
		SchemaVersion int                 `json:"schema_version"`
		Cases         []GoldenCasePayload `json:"cases"`
	}{
		SchemaVersion: GoldenSetSchemaVersion,
		Cases:         sorted,
	})
}

// ChecksumOf returns the "sha256:<hex>" checksum of a canonical body.
func ChecksumOf(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// BuildGoldenSetFile assembles the committed fixture bytes: canonical body
// plus the dataset_checksum header computed over that body.
func BuildGoldenSetFile(cases []GoldenCasePayload) ([]byte, error) {
	body, err := GoldenSetBody(cases)
	if err != nil {
		return nil, err
	}
	checksum := ChecksumOf(body)
	return json.Marshal(GoldenSetFile{
		SchemaVersion:   GoldenSetSchemaVersion,
		DatasetChecksum: checksum,
		Cases:           cases,
	})
}

// ParseGoldenSetFile validates a committed fixture: schema version, case
// shape and dataset_checksum consistency with the canonical body.
func ParseGoldenSetFile(data []byte) (*GoldenSetFile, error) {
	var file GoldenSetFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse golden set file: %w", err)
	}
	if file.SchemaVersion != GoldenSetSchemaVersion {
		return nil, fmt.Errorf("golden set schema_version = %d, want %d", file.SchemaVersion, GoldenSetSchemaVersion)
	}
	body, err := GoldenSetBody(file.Cases)
	if err != nil {
		return nil, err
	}
	if got := ChecksumOf(body); got != file.DatasetChecksum {
		return nil, errors.New("dataset_checksum does not match the canonical case body")
	}
	return &file, nil
}

// BaselineArtifact is the committed eval baseline
// (backend/testdata/rag_eval_baseline.json): real measured numbers for the
// keyword-only and vector-only retrievers over the full golden set.
type BaselineArtifact struct {
	ArtifactVersion  int               `json:"artifact_version"`
	DatasetChecksum  string            `json:"dataset_checksum"`
	DatasetSize      int               `json:"dataset_size"`
	Retrievers       []RetrieverResult `json:"retrievers"`
	ArtifactChecksum string            `json:"artifact_checksum"`
}

// BaselineArtifactVersion is the committed baseline schema version (1).
const BaselineArtifactVersion = 1

// BaselineArtifactBody serialises the artifact canonically without the
// artifact_checksum field; the checksum is computed over exactly these bytes.
func BaselineArtifactBody(retrievers []RetrieverResult, datasetChecksum string, datasetSize int) ([]byte, error) {
	return json.Marshal(struct {
		ArtifactVersion int               `json:"artifact_version"`
		DatasetChecksum string            `json:"dataset_checksum"`
		DatasetSize     int               `json:"dataset_size"`
		Retrievers      []RetrieverResult `json:"retrievers"`
	}{
		ArtifactVersion: BaselineArtifactVersion,
		DatasetChecksum: datasetChecksum,
		DatasetSize:     datasetSize,
		Retrievers:      retrievers,
	})
}

// BuildBaselineArtifact assembles the committed baseline bytes: canonical
// body plus the artifact_checksum header computed over that body.
func BuildBaselineArtifact(retrievers []RetrieverResult, datasetChecksum string, datasetSize int) ([]byte, error) {
	body, err := BaselineArtifactBody(retrievers, datasetChecksum, datasetSize)
	if err != nil {
		return nil, err
	}
	return json.Marshal(BaselineArtifact{
		ArtifactVersion:  BaselineArtifactVersion,
		DatasetChecksum:  datasetChecksum,
		DatasetSize:      datasetSize,
		Retrievers:       retrievers,
		ArtifactChecksum: ChecksumOf(body),
	})
}

// ParseBaselineArtifact validates a committed baseline: schema version and
// artifact_checksum consistency with the canonical body.
func ParseBaselineArtifact(data []byte) (*BaselineArtifact, error) {
	var artifact BaselineArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("parse baseline artifact: %w", err)
	}
	if artifact.ArtifactVersion != BaselineArtifactVersion {
		return nil, fmt.Errorf("baseline artifact_version = %d, want %d", artifact.ArtifactVersion, BaselineArtifactVersion)
	}
	body, err := BaselineArtifactBody(artifact.Retrievers, artifact.DatasetChecksum, artifact.DatasetSize)
	if err != nil {
		return nil, err
	}
	if got := ChecksumOf(body); got != artifact.ArtifactChecksum {
		return nil, errors.New("artifact_checksum does not match the canonical body")
	}
	return &artifact, nil
}
