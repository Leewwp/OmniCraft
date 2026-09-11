package main

// answer-probe is a manual diagnostic for MiniMax-M3 tool-call compliance
// under the production system prompt. Finding (2026-08-28): with tools
// advertised and no explicit grounding instruction, the model
// probabilistically alternates between calling search_content and answering
// from general knowledge, independent of the max_tokens cap. This motivates
// the answer-eval runner's bounded retry and direct-answer-rate accounting.
// Never part of unit CI; requires real provider credentials.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

func toolDefs() []llm.ToolDefinition {
	// Mirror AgentService.ToolDefinitions (agent_tools.go) exactly.
	return []llm.ToolDefinition{
		{
			Name:        "search_content",
			Description: "Search viewer-visible published content. Returns compact content summaries.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "natural-language search query"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "get_content_detail",
			Description: "Get a compact summary of one viewer-visible content item.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content_id": map[string]interface{}{"type": "integer", "description": "content id"},
				},
				"required": []string{"content_id"},
			},
		},
	}
}

func runVariant(name, system, query string, provider llm.LLMProvider, maxTokens int) {
	req := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: query},
		},
		Tools:     toolDefs(),
		MaxTokens: maxTokens,
		Stream:    true,
	}
	var content strings.Builder
	toolNames := map[string]bool{}
	started := time.Now()
	firstContent := int64(0)
	err := provider.ChatStream(context.Background(), req, func(delta llm.ChatDelta) error {
		if delta.Content != "" {
			if firstContent == 0 {
				firstContent = time.Since(started).Milliseconds()
			}
			content.WriteString(delta.Content)
		}
		for _, tc := range delta.ToolCalls {
			if tc.Function.Name != "" {
				toolNames[tc.Function.Name] = true
			}
		}
		return nil
	})
	out := content.String()
	if len([]rune(out)) > 80 {
		out = string([]rune(out)[:80]) + "…"
	}
	fmt.Printf("[%s] err=%v tools=%v firstContentMs=%d totalMs=%d contentLen=%d content=%q\n",
		name, err, keys(toolNames), firstContent, time.Since(started).Milliseconds(), len([]rune(content.String())), out)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func main() {
	cfg := config.Load()
	provider := llm.NewProvider(cfg)
	query := "Blender 插件安装教程"
	minimal := "[OmniCraft Agent Context] surface=global"
	for rep := 1; rep <= 2; rep++ {
		runVariant(fmt.Sprintf("minimal@1200#%d", rep), minimal, query, provider, 1200)
	}
	for rep := 1; rep <= 2; rep++ {
		runVariant(fmt.Sprintf("minimal@2048#%d", rep), minimal, query, provider, 2048)
	}
	os.Exit(0)
}
