package rag

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	"omnicraft/backend/internal/pkg/auxcache"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/service/promptregistry"
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
	// prompts renders the versioned query_expansion_prompt slot (SP-21 T5);
	// nil resolver uses the compiled-in builtin.
	prompts *promptregistry.PromptResolver
	// cache reuses expansion terms for an identical query (SP-24 R6). nil
	// or disabled = every expansion calls the LLM, the pre-R6 behavior.
	cache *auxcache.Cache
}

func NewLLMQueryExpander(provider expansionChatProvider) *LLMQueryExpander {
	return &LLMQueryExpander{provider: provider}
}

// SetPromptResolver wires the shared registry resolver (container).
func (e *LLMQueryExpander) SetPromptResolver(r *promptregistry.PromptResolver) {
	e.prompts = r
}

// SetResultCache wires the query-hash TTL cache (container; nil-safe).
func (e *LLMQueryExpander) SetResultCache(c *auxcache.Cache) {
	e.cache = c
}

func (e *LLMQueryExpander) Expand(ctx context.Context, query string) []string {
	if e == nil || e.provider == nil {
		return nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	// SP-24 R6: identical queries reuse the cached terms (nil or disabled
	// cache = bypass). Only usable term sets are cached, so a transient
	// failure never pins an empty expansion.
	cacheKey := auxcache.HashKey("expander", query)
	if cached, ok := e.cache.Get(ctx, cacheKey); ok {
		var terms []string
		if err := json.Unmarshal([]byte(cached), &terms); err == nil && len(terms) > 0 {
			return terms
		}
	}
	prompt := e.prompts.RenderSlot(ctx, promptregistry.SlotQueryExpansion, map[string]string{
		"max_terms": strconv.Itoa(queryExpansionMaxTerms),
		"query":     truncateRunes(query, queryExpansionMaxTermRunes*2),
	})
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
		return nil
	}
	if payload, err := json.Marshal(terms); err == nil {
		e.cache.Set(ctx, cacheKey, string(payload))
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
