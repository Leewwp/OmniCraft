package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// agentEvalContent seeds a deterministic content record for an evaluation case.
type agentEvalContent struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	ContentType    string `json:"content_type"`
	Zone           string `json:"zone"`
	Status         string `json:"status"`
	IsPublic       bool   `json:"is_public"`
	AuthorIsBanned bool   `json:"author_is_banned"`
	InjectionText  string `json:"injection_text,omitempty"`
}

// agentEvalExpect is the deterministic oracle for one evaluation case.
type agentEvalExpect struct {
	AnswerKind          string   `json:"answer_kind"`
	ExpectedContentIDs  []int64  `json:"expected_content_ids"`
	ForbiddenContentIDs []int64  `json:"forbidden_content_ids"`
	ExpectedToolNames   []string `json:"expected_tool_names"`
	Degraded            bool     `json:"degraded"`
	CitationSource      string   `json:"citation_source,omitempty"`
	ErrorCode           string   `json:"error_code,omitempty"`
}

type agentEvalCase struct {
	ID       string             `json:"id"`
	Category string             `json:"category"`
	Query    string             `json:"query"`
	Surface  string             `json:"surface"`
	Contents []agentEvalContent `json:"contents"`
	Expected agentEvalExpect    `json:"expected"`
}

type agentEvalFixture struct {
	SchemaVersion int             `json:"schema_version"`
	Cases         []agentEvalCase `json:"cases"`
}

// TestAgentEvalFixture validates the fixed evaluation fixture file: every case
// has a unique id, a surface, a query, a deterministic expected outcome, and
// only non-negative content ids. The fixture is the CI oracle for groundedness,
// refusal, injection, visibility and downgrade behaviour.
func TestAgentEvalFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/agent_eval_cases.json")
	require.NoError(t, err)

	var fixture agentEvalFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	require.Equal(t, 2, fixture.SchemaVersion)
	require.NotEmpty(t, fixture.Cases)

	seen := make(map[string]bool, len(fixture.Cases))
	for _, tc := range fixture.Cases {
		require.NotEmpty(t, tc.ID, "case missing id")
		require.False(t, seen[tc.ID], "duplicate case id %q", tc.ID)
		seen[tc.ID] = true

		require.NotEmpty(t, tc.Query, "case %s missing query", tc.ID)
		require.NotEmpty(t, tc.Surface, "case %s missing surface", tc.ID)
		require.NotEmpty(t, tc.Expected.AnswerKind, "case %s missing expected.answer_kind", tc.ID)

		require.True(t,
			len(tc.Expected.ExpectedContentIDs) > 0 || tc.Expected.ErrorCode != "" || tc.Expected.Degraded,
			"case %s must declare a positive expected outcome", tc.ID)

		for _, id := range tc.Expected.ExpectedContentIDs {
			require.Positive(t, id, "case %s has non-positive expected content id", tc.ID)
		}
		for _, id := range tc.Expected.ForbiddenContentIDs {
			require.Positive(t, id, "case %s has non-positive forbidden content id", tc.ID)
		}
	}

	// The required coverage matrix must be present (plan Task 1 Step 4).
	required := []string{
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
	for _, id := range required {
		require.True(t, seen[id], "required evaluation case %q is missing", id)
	}
}
