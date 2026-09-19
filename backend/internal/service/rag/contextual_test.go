package rag

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/llm"
)

// stubAnnotationProvider records every chat request and answers from a
// canned queue (one reply per chunk, in order); it can also fail.
type stubAnnotationProvider struct {
	mu          sync.Mutex
	requests    []llm.ChatRequest
	replies     []string
	err         error
	blockFor    time.Duration
	maxInFlight int
	inFlight    int
	peak        int
}

func (p *stubAnnotationProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.inFlight++
	if p.inFlight > p.peak {
		p.peak = p.inFlight
	}
	p.mu.Unlock()
	if p.blockFor > 0 {
		time.Sleep(p.blockFor)
	}
	p.mu.Lock()
	p.inFlight--
	var reply string
	if len(p.replies) > 0 {
		reply, p.replies = p.replies[0], p.replies[1:]
	}
	err := p.err
	p.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &llm.ChatResponse{Content: reply}, nil
}

func testContextualConfig() config.RAGContextualConfig {
	return config.RAGContextualConfig{
		Enabled: true, Provider: "deepseek", Model: "deepseek-chat",
		MaxPrefixTokens: 100, DocContextChars: 2000,
		Concurrency: 2, RequestIntervalMS: 0, TimeoutSec: 30, MaxRetries: 2,
	}
}

func contextualDoc() SourceDocument {
	return SourceDocument{ContentID: 42, ContentVersion: 3, Title: "星轨下的制琴师", Text: "第一章琴房的木料清单。\n第二章漆面工艺的细节。"}
}

func TestContextualAnnotatorPrependsSanitizedPrefix(t *testing.T) {
	provider := &stubAnnotationProvider{replies: []string{"位于第一章：介绍琴房木料的段落。"}}
	annotator := NewContextAnnotator(provider, testContextualConfig(), nil)
	chunks := []Chunk{{ChunkIndex: 0, ChunkKey: "k0", Text: "Guide\npublished body", SourceStart: 0, SourceEnd: 10}}

	annotated := annotator.Annotate(context.Background(), contextualDoc(), chunks)

	require.Len(t, annotated, 1)
	require.Equal(t, "位于第一章：介绍琴房木料的段落。\nGuide\npublished body", annotated[0].Text)
	require.Equal(t, "k0", annotated[0].ChunkKey, "identity stays span-bound; the prefix is additive")
	// the input slice must not be mutated (annotation works on a copy)
	require.Equal(t, "Guide\npublished body", chunks[0].Text)
}

func TestContextualAnnotatorPromptCarriesDocChunkAndTitle(t *testing.T) {
	provider := &stubAnnotationProvider{replies: []string{"prefix"}}
	annotator := NewContextAnnotator(provider, testContextualConfig(), nil)
	doc := contextualDoc()
	chunks := []Chunk{{ChunkIndex: 0, Text: "published body"}}

	annotator.Annotate(context.Background(), doc, chunks)

	require.Len(t, provider.requests, 1)
	req := provider.requests[0]
	require.Equal(t, "system", req.Messages[0].Role)
	user := req.Messages[1].Content
	require.Contains(t, user, "星轨下的制琴师")
	require.Contains(t, user, "木料清单")
	require.Contains(t, user, "published body")
	require.Contains(t, user, "100", "max_prefix_tokens renders into the instruction")
	require.Positive(t, req.MaxTokens)
}

func TestContextualAnnotatorFailOpenOnProviderError(t *testing.T) {
	provider := &stubAnnotationProvider{err: errors.New("deepseek down")}
	annotator := NewContextAnnotator(provider, testContextualConfig(), nil)
	chunks := []Chunk{{ChunkIndex: 0, Text: "a"}, {ChunkIndex: 1, Text: "b"}}

	annotated := annotator.Annotate(context.Background(), contextualDoc(), chunks)

	require.Equal(t, "a", annotated[0].Text)
	require.Equal(t, "b", annotated[1].Text)
}

func TestContextualAnnotatorFailOpenOnUnusableReply(t *testing.T) {
	provider := &stubAnnotationProvider{replies: []string{"   「上下文：」\n\"  \"", ""}}
	annotator := NewContextAnnotator(provider, testContextualConfig(), nil)
	chunks := []Chunk{{ChunkIndex: 0, Text: "a"}, {ChunkIndex: 1, Text: "b"}}

	annotated := annotator.Annotate(context.Background(), contextualDoc(), chunks)

	require.Equal(t, "a", annotated[0].Text)
	require.Equal(t, "b", annotated[1].Text)
}

func TestSanitizeContextualPrefix(t *testing.T) {
	cases := map[string]string{
		"  第一章木料清单。  ":                                   "第一章木料清单。",
		"上下文：第一章木料清单。":                                   "第一章木料清单。",
		"「上下文：」第一章木料清单。":                                 "第一章木料清单。",
		"“Context: chapter one wood list”":               "chapter one wood list",
		"“第一章木料清单。”":                                     "第一章木料清单。",
		"「第一章\n木料清单。」":                                   "第一章 木料清单。",
		strings.Repeat("长", contextualMaxPrefixRunes+50): strings.Repeat("长", contextualMaxPrefixRunes),
	}
	for reply, want := range cases {
		require.Equal(t, want, sanitizeContextualPrefix(reply), "reply %q", reply)
	}
}

func TestContextualAnnotatorNilSafety(t *testing.T) {
	var nilAnnotator *ContextAnnotator
	chunks := []Chunk{{Text: "a"}}
	require.Equal(t, chunks, nilAnnotator.Annotate(context.Background(), contextualDoc(), chunks))

	unwired := NewContextAnnotator(nil, testContextualConfig(), nil)
	require.Equal(t, chunks, unwired.Annotate(context.Background(), contextualDoc(), chunks))

	wired := NewContextAnnotator(&stubAnnotationProvider{replies: []string{"p"}}, testContextualConfig(), nil)
	require.Nil(t, wired.Annotate(context.Background(), contextualDoc(), nil))
}

func TestContextualAnnotatorDocContextKeepsHeadAndTail(t *testing.T) {
	annotator := NewContextAnnotator(&stubAnnotationProvider{}, testContextualConfig(), nil)
	long := strings.Repeat("头", 1500) + strings.Repeat("尾", 1500)

	excerpt := annotator.docContext(long)

	require.Len(t, []rune(excerpt), 2000+3, "limit runes plus the \n…\n joiner")
	require.Contains(t, excerpt, "…")
	require.True(t, strings.HasPrefix(excerpt, "头"))
	require.True(t, strings.HasSuffix(excerpt, "尾"))

	short := strings.TrimSpace("短文档")
	require.Equal(t, short, annotator.docContext(short))
}

func TestContextualAnnotatorPacesStartsAcrossWorkers(t *testing.T) {
	provider := &stubAnnotationProvider{replies: []string{"p1", "p2", "p3", "p4"}}
	cfg := testContextualConfig()
	cfg.Concurrency = 4
	cfg.RequestIntervalMS = 60
	annotator := NewContextAnnotator(provider, cfg, nil)
	chunks := []Chunk{{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}}

	start := time.Now()
	annotated := annotator.Annotate(context.Background(), contextualDoc(), chunks)
	elapsed := time.Since(start)

	for i, chunk := range annotated {
		require.True(t, strings.HasPrefix(chunk.Text, "p"), "chunk %d annotated", i)
	}
	require.GreaterOrEqual(t, elapsed, 3*60*time.Millisecond,
		"four paced starts across four workers need at least three intervals")
}

func TestContextualAnnotatorBoundedConcurrency(t *testing.T) {
	provider := &stubAnnotationProvider{
		replies:  []string{"p1", "p2", "p3", "p4", "p5", "p6"},
		blockFor: 40 * time.Millisecond,
	}
	cfg := testContextualConfig()
	cfg.Concurrency = 2
	annotator := NewContextAnnotator(provider, cfg, nil)
	chunks := make([]Chunk, 6)
	for i := range chunks {
		chunks[i] = Chunk{ChunkIndex: i, Text: "c"}
	}

	annotator.Annotate(context.Background(), contextualDoc(), chunks)

	require.Equal(t, 2, provider.peak, "in-flight annotation calls must never exceed concurrency")
}
