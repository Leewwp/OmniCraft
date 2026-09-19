package rag

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/service/promptregistry"
)

// Spec-fixed sanitization caps (SP-24 R4 #575): the config knob
// max_prefix_tokens only feeds the prompt instruction; these bounds keep
// every stored prefix a one-line note regardless of model verbosity
// (expander-cap precedent: deliberately not config knobs).
const (
	contextualMaxPrefixRunes  = 240
	contextualMaxOutputTokens = 200
	contextualMaxChunkRunes   = 2000
	contextualMaxTitleRunes   = 200
)

// ContextualPromptOverhead is the fixed prompt chrome (system line +
// template) the cost estimator adds on top of document + chunk tokens.
const ContextualPromptOverhead = 150

// contextualSystemPrompt is the fixed system line (expander pattern: system
// stays in code, the user message renders the versioned slot).
const contextualSystemPrompt = "你是检索索引的 chunk 上下文标注器，只输出一句给检索用的片段定位说明，不输出任何别的内容。"

// annotationChatProvider is the narrow chat capability the annotator needs;
// *llm providers and test fakes both satisfy it.
type annotationChatProvider interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// ChunkAnnotator rewrites chunk texts at ingestion time (contextual
// retrieval). Implementations must be fail-open: a failed annotation keeps
// the original chunk text.
type ChunkAnnotator interface {
	Annotate(ctx context.Context, doc SourceDocument, chunks []Chunk) []Chunk
}

// ContextAnnotator prepends a short LLM-written situation prefix to every
// chunk text (Anthropic contextual retrieval). The stored chunk text feeds
// the embedding input, the lexical index and the citation surface, so one
// prepend upgrades every retrieval path. Failures fail open per chunk: the
// unannotated chunk keeps flowing, the projection never gates on the LLM.
type ContextAnnotator struct {
	provider annotationChatProvider
	prompts  *promptregistry.PromptResolver
	cfg      config.RAGContextualConfig
	pacer    *startPacer
}

// NewContextAnnotator builds the annotator; prompts may be nil (builtin
// template). The returned annotator is safe to share across goroutines.
func NewContextAnnotator(provider annotationChatProvider, cfg config.RAGContextualConfig, prompts *promptregistry.PromptResolver) *ContextAnnotator {
	interval := time.Duration(cfg.RequestIntervalMS) * time.Millisecond
	return &ContextAnnotator{
		provider: provider,
		prompts:  prompts,
		cfg:      cfg,
		pacer:    newStartPacer(interval),
	}
}

// startPacer enforces a minimum interval between request starts across all
// worker goroutines of all Annotate calls sharing the annotator (batch rate
// limiting, #575 budget control).
type startPacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newStartPacer(interval time.Duration) *startPacer {
	return &startPacer{interval: interval}
}

func (p *startPacer) wait(ctx context.Context) error {
	if p == nil || p.interval <= 0 {
		return nil
	}
	p.mu.Lock()
	now := time.Now()
	start := p.next
	if start.Before(now) {
		start = now
	}
	p.next = start.Add(p.interval)
	p.mu.Unlock()
	if delay := time.Until(start); delay > 0 {
		select {
		case <-time.After(delay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Annotate returns a copy of chunks with each text prefixed by its situation
// note. It never fails: a nil annotator, a nil provider, a cancelled context
// or a per-chunk LLM failure all yield the original chunk texts.
func (a *ContextAnnotator) Annotate(ctx context.Context, doc SourceDocument, chunks []Chunk) []Chunk {
	if a == nil || a.provider == nil || len(chunks) == 0 {
		return chunks
	}
	annotated := make([]Chunk, len(chunks))
	copy(annotated, chunks)
	workers := a.cfg.Concurrency
	if workers < 1 {
		workers = 1
	}
	if workers > len(annotated) {
		workers = len(annotated)
	}
	jobs := make(chan int)
	var annotatedCount, failedCount atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if err := a.pacer.wait(ctx); err != nil {
					// Cancellation skips the chunk; count it so the summary
					// line never underreports fail-open outcomes.
					failedCount.Add(1)
					continue
				}
				prefix, ok := a.prefixFor(ctx, doc, annotated[i])
				if !ok {
					failedCount.Add(1)
					continue
				}
				annotated[i].Text = prefix + "\n" + annotated[i].Text
				annotatedCount.Add(1)
			}
		}()
	}
	for i := range annotated {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	slog.Info("contextual annotation done",
		"content_id", doc.ContentID, "chunks", len(chunks),
		"annotated", annotatedCount.Load(), "failed", failedCount.Load())
	return annotated
}

// prefixFor renders the slot prompt for one chunk and calls the annotation
// model once. Any error or unusable reply returns ok=false (fail-open).
func (a *ContextAnnotator) prefixFor(ctx context.Context, doc SourceDocument, chunk Chunk) (string, bool) {
	prompt := a.prompts.RenderSlot(ctx, promptregistry.SlotContextualAnnotation, map[string]string{
		"title":      truncateRunes(doc.Title, contextualMaxTitleRunes),
		"document":   a.docContext(doc.Text),
		"chunk":      truncateRunes(chunk.Text, contextualMaxChunkRunes),
		"max_tokens": strconv.Itoa(a.cfg.MaxPrefixTokens),
	})
	resp, err := a.provider.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: contextualSystemPrompt},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   contextualMaxOutputTokens,
		Temperature: 0.2,
	})
	if err != nil || resp == nil {
		slog.Warn("contextual annotation skipped", "policy", "fail_open",
			"content_id", doc.ContentID, "chunk_index", chunk.ChunkIndex, "reason", "annotation_call_failed")
		return "", false
	}
	prefix := sanitizeContextualPrefix(resp.Content)
	if prefix == "" {
		slog.Warn("contextual annotation produced no usable prefix", "policy", "fail_open",
			"content_id", doc.ContentID, "chunk_index", chunk.ChunkIndex)
		return "", false
	}
	return prefix, true
}

// docContext bounds the document excerpt handed to the model via
// BoundDocContext.
func (a *ContextAnnotator) docContext(text string) string {
	return BoundDocContext(text, a.cfg.DocContextChars)
}

// BoundDocContext bounds a document excerpt to limit runes (<=0 → 2000).
// Short documents pass whole; longer ones keep head plus tail so chunks near
// the end still get position-aware context. Shared with the cost estimator
// so the estimate measures the real prompt size.
func BoundDocContext(text string, limit int) string {
	if limit <= 0 {
		limit = 2000
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	head := limit * 6 / 10
	tail := limit - head
	return string(runes[:head]) + "\n…\n" + string(runes[len(runes)-tail:])
}

// contextualLabelPattern strips a leading 「上下文：」/“Context:”-style label
// (with or without wrapping quotes/brackets) the model may add despite the
// prompt forbidding it. The colon makes it safe against a note that merely
// starts with the word.
var contextualLabelPattern = regexp.MustCompile(`(?i)^[「『"'“]?\s*(?:上下文\s*[：:]|context\s*[：:])\s*[」』"'”]?\s*`)

// sanitizeContextualPrefix turns a model reply into a one-line note: trims
// space, strips matched surrounding quotes and a leading 「上下文：」-style
// label (quote-strip → label-strip → quote-strip, so a quoted label and a
// labeled quote both come off), collapses all whitespace runs to single
// spaces, caps the length.
func sanitizeContextualPrefix(reply string) string {
	stripQuotes := func(prefix string) string {
		for _, quote := range []struct{ open, close string }{
			{"“", "”"}, {"「", "」"}, {"『", "』"}, {"\"", "\""}, {"'", "'"},
		} {
			if strings.HasPrefix(prefix, quote.open) && strings.HasSuffix(prefix, quote.close) && len(prefix) >= len(quote.open)+len(quote.close) {
				return strings.TrimSpace(prefix[len(quote.open) : len(prefix)-len(quote.close)])
			}
		}
		return prefix
	}
	prefix := stripQuotes(strings.TrimSpace(reply))
	prefix = contextualLabelPattern.ReplaceAllString(prefix, "")
	prefix = stripQuotes(prefix)
	prefix = collapseWhitespace(prefix)
	if runes := []rune(prefix); len(runes) > contextualMaxPrefixRunes {
		prefix = string(runes[:contextualMaxPrefixRunes])
	}
	return prefix
}

func collapseWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
