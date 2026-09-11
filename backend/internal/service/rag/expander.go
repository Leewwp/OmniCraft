package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"omnicraft/backend/internal/pkg/llm"
)

// Spec-fixed expansion caps (SP-13 A-03): 3–5 terms, one short capped chat
// call. They are deliberately not config knobs.
const (
	queryExpansionMaxTerms        = 5
	queryExpansionMaxTermRunes    = 50
	queryExpansionMaxOutputTokens = 200
)

// expansionChatProvider is the narrow chat capability the query expander
// needs; *llm providers and test fakes both satisfy it.
type expansionChatProvider interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// LLMQueryExpander expands one colloquial user query into sibling retrieval
// terms with one short non-streaming chat call. Any failure is swallowed
// (structured log): expansion is an enhancement, never a gate — retrieval
// continues with the original query.
type LLMQueryExpander struct {
	provider expansionChatProvider
}

func NewLLMQueryExpander(provider expansionChatProvider) *LLMQueryExpander {
	return &LLMQueryExpander{provider: provider}
}

func (e *LLMQueryExpander) Expand(ctx context.Context, query string) []string {
	if e == nil || e.provider == nil {
		return nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	prompt := fmt.Sprintf(
		"把下面的用户输入扩展为最多 %d 个用于站内内容检索的中文检索词，覆盖同义词、别名与相关表述，不要解释。只输出一个 JSON 字符串数组。\n输入：%s",
		queryExpansionMaxTerms, truncateRunes(query, queryExpansionMaxTermRunes*2),
	)
	resp, err := e.provider.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "你是站内检索查询扩展器，只输出 JSON 字符串数组。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   queryExpansionMaxOutputTokens,
		Temperature: 0.2,
	})
	if err != nil || resp == nil {
		slog.Warn("agent query expansion skipped", "policy", "fail_open", "reason", "expansion_call_failed")
		return nil
	}
	terms := parseExpansionTerms(resp.Content, query)
	if len(terms) == 0 {
		slog.Warn("agent query expansion produced no usable terms", "policy", "fail_open")
	}
	return terms
}

// parseExpansionTerms extracts a JSON string array from a model reply,
// tolerating markdown code fences, and sanitizes: trimmed, non-empty, not a
// duplicate of the original query, deduped case-insensitively, capped.
func parseExpansionTerms(reply, originalQuery string) []string {
	content := strings.TrimSpace(reply)
	if start := strings.Index(content, "["); start >= 0 {
		if end := strings.LastIndex(content, "]"); end > start {
			content = content[start : end+1]
		}
	}
	var raw []string
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil
	}
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(originalQuery)): true}
	terms := make([]string, 0, queryExpansionMaxTerms)
	for _, term := range raw {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if runes := []rune(term); len(runes) > queryExpansionMaxTermRunes {
			term = string(runes[:queryExpansionMaxTermRunes])
		}
		key := strings.ToLower(term)
		if seen[key] {
			continue
		}
		seen[key] = true
		terms = append(terms, term)
		if len(terms) >= queryExpansionMaxTerms {
			break
		}
	}
	return terms
}

func truncateRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}
