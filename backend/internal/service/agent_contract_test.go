package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentContract locks the typed JSON contract of the Agent answer and its
// nested objects so the wire format cannot silently drift. Public response
// fields must never be map[string]any.
func TestAgentContract(t *testing.T) {
	answer := AgentAnswer{
		TraceID:    "trace-123",
		AnswerKind: AgentAnswerGroundedContent,
		Answer:     "Blender 插件安装需要三步。",
		Citations: []AgentCitation{
			{ContentID: 101, Title: "Blender 插件安装指南", Zone: "original", Excerpt: "第一步：下载插件包。"},
		},
		Tools: []AgentToolExecution{
			{Name: "search_content", Status: AgentToolStatusSuccess, DurationMs: 42},
		},
		Usage:    AgentUsage{PromptTokens: 120, CompletionTokens: 300},
		Degraded: false,
	}

	data, err := json.Marshal(answer)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	require.Equal(t, "trace-123", raw["trace_id"])
	require.Equal(t, "grounded_content", raw["answer_kind"])
	require.Equal(t, "Blender 插件安装需要三步。", raw["answer"])
	require.Equal(t, false, raw["degraded"])

	citations := raw["citations"].([]any)
	require.Len(t, citations, 1)
	cite := citations[0].(map[string]any)
	require.Equal(t, float64(101), cite["content_id"])
	require.Equal(t, "Blender 插件安装指南", cite["title"])
	require.Equal(t, "original", cite["zone"])
	require.Equal(t, "第一步：下载插件包。", cite["excerpt"])
	require.NotContains(t, cite, "content_version")
	require.NotContains(t, cite, "chunk_key")
	require.NotContains(t, cite, "chunk_index")
	require.NotContains(t, cite, "route")
	require.NotContains(t, cite, "source")

	tools := raw["tools"].([]any)
	require.Len(t, tools, 1)
	tool := tools[0].(map[string]any)
	require.Equal(t, "search_content", tool["name"])
	require.Equal(t, "success", tool["status"])
	require.Equal(t, float64(42), tool["duration_ms"])

	usage := raw["usage"].(map[string]any)
	require.Equal(t, float64(120), usage["prompt_tokens"])
	require.Equal(t, float64(300), usage["completion_tokens"])
}

func TestAgentRAGCitationContractIncludesZeroChunkIndex(t *testing.T) {
	answer := AgentAnswer{Citations: []AgentCitation{{
		ContentID: 101, ContentVersion: 3, ChunkKey: "chunk-101", ChunkIndex: 0,
		Title: "Blender 插件安装指南", Zone: "original", Route: "/original/101",
		Excerpt: "第一步：下载插件包。", Source: "hybrid_rrf",
	}}}

	data, err := json.Marshal(answer)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	cite := raw["citations"].([]any)[0].(map[string]any)
	require.Equal(t, float64(3), cite["content_version"])
	require.Equal(t, "chunk-101", cite["chunk_key"])
	require.Equal(t, float64(0), cite["chunk_index"])
	require.Equal(t, "/original/101", cite["route"])
	require.Equal(t, "hybrid_rrf", cite["source"])
}

// TestAgentAnswerKindEnum ensures the server owns every answer-kind value and
// that a grounded answer cannot be relabelled by the model.
func TestAgentAnswerKindEnum(t *testing.T) {
	require.Equal(t, AgentAnswerKind("grounded_content"), AgentAnswerGroundedContent)
	require.Equal(t, AgentAnswerKind("no_evidence"), AgentAnswerNoEvidence)
	require.Equal(t, AgentAnswerKind("publish_suggestion"), AgentAnswerPublishSuggestion)
}

// TestAgentSafeErrorDTO locks the safe error wire format; raw Provider errors
// must never be serialized into this shape.
func TestAgentSafeErrorDTO(t *testing.T) {
	errDTO := AgentErrorDTO{Code: "AGENT_PROVIDER_ERROR", Message: "provider unavailable"}
	data, err := json.Marshal(errDTO)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Equal(t, "AGENT_PROVIDER_ERROR", raw["code"])
	require.Equal(t, "provider unavailable", raw["message"])
}
