package rag

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestChunkerCompletesRealTokenizerLargeSmoke(t *testing.T) {
	text := strings.Repeat("a", 1024*1024)
	c := NewChunker(ChunkerConfig{MaxTokens: 512, OverlapTokens: 48, ChunkingVersion: 1})

	chunks, err := c.Chunk(SourceDocument{ContentID: 1, ContentVersion: 1, Text: text})
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)
	require.Equal(t, 0, chunks[0].SourceStart)
	require.Equal(t, utf8.RuneCountInString(text), chunks[len(chunks)-1].SourceEnd)
	for i := 1; i < len(chunks); i++ {
		require.Greater(t, chunks[i].SourceStart, chunks[i-1].SourceStart)
		require.Less(t, chunks[i].SourceStart, chunks[i-1].SourceEnd)
	}
}

func TestChunkerExaminesOnlyBoundedWindowsWhileSplitting(t *testing.T) {
	text := strings.Repeat("x", 10*1024*1024)
	c := NewChunker(ChunkerConfig{MaxTokens: 512, OverlapTokens: 48, ChunkingVersion: 1})
	var examinedRunes int
	c.tokenCounter = func(input string) int {
		count := utf8.RuneCountInString(input)
		examinedRunes += count
		return count
	}

	chunks, err := c.Chunk(SourceDocument{ContentID: 1, ContentVersion: 1, Text: text})
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)
	require.Less(t, examinedRunes, utf8.RuneCountInString(text)*32,
		"tokenization work must stay bounded by local windows, not the full remaining document")
}

func TestChunkerCachesTokenizerInitializationAcrossConcurrentConstructors(t *testing.T) {
	const count = 16
	encodings := make(chan any, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			encodings <- NewChunker(ChunkerConfig{MaxTokens: 8, OverlapTokens: 1, ChunkingVersion: 1}).encoding
		}()
	}
	wg.Wait()
	close(encodings)

	var first any
	for encoding := range encodings {
		require.NotNil(t, encoding)
		if first == nil {
			first = encoding
			continue
		}
		require.Same(t, first, encoding)
	}
}

func TestChunkerUsesEmbeddedTokenizerOffline(t *testing.T) {
	t.Setenv("TIKTOKEN_CACHE_DIR", filepath.Join(t.TempDir(), "must-stay-empty"))
	c := NewChunker(ChunkerConfig{MaxTokens: 8, OverlapTokens: 1, ChunkingVersion: 1})
	chunks, err := c.Chunk(SourceDocument{ContentID: 1, ContentVersion: 1, Text: "offline tokenizer"})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
}

func TestChunkerRejectsUnsupportedTokenizerEncoding(t *testing.T) {
	c := NewChunker(ChunkerConfig{MaxTokens: 8, OverlapTokens: 1, ChunkingVersion: 1, TokenizerEncoding: "o200k_base"})
	_, err := c.Chunk(SourceDocument{ContentID: 1, ContentVersion: 1, Text: "unsupported tokenizer"})
	require.ErrorContains(t, err, "unsupported tokenizer encoding")
}

func TestChunkerHeadingParagraphAndStableIdentity(t *testing.T) {
	c := NewChunker(ChunkerConfig{MaxTokens: 32, OverlapTokens: 4, ChunkingVersion: 1})
	doc := SourceDocument{ContentID: 42, ContentVersion: 3, Title: "Guide", Text: "# Intro\n\n你好，world。\n\n## Details\n\nThis is a paragraph."}
	chunks, err := c.Chunk(doc)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	require.Equal(t, "Intro", chunks[0].Heading)
	require.LessOrEqual(t, chunks[0].SourceStart, chunks[0].SourceEnd)
	require.Contains(t, chunks[0].Text, "Intro\n")
	require.Len(t, chunks[0].ChunkKey, 64)
	require.Equal(t, "Intro > Details", chunks[len(chunks)-1].Heading)
	again, err := c.Chunk(doc)
	require.NoError(t, err)
	require.Equal(t, chunks, again)
}

func TestChunkerPrefixesPlainParagraphWithContentTitle(t *testing.T) {
	c := NewChunker(ChunkerConfig{MaxTokens: 32, OverlapTokens: 4, ChunkingVersion: 1})
	chunks, err := c.Chunk(SourceDocument{ContentID: 2, ContentVersion: 1, Title: "Content title", Text: "plain body"})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Empty(t, chunks[0].Heading)
	require.Equal(t, "Content title\nplain body", chunks[0].Text)
	require.Equal(t, "plain body", string([]rune("plain body")[chunks[0].SourceStart:chunks[0].SourceEnd]))
}

func TestChunkerHardSplitsAndUnicodeSpans(t *testing.T) {
	c := NewChunker(ChunkerConfig{MaxTokens: 4, OverlapTokens: 1, ChunkingVersion: 2})
	doc := SourceDocument{ContentID: 1, ContentVersion: 1, Text: "你好世界，这是一个很长的段落，用于验证分块。"}
	chunks, err := c.Chunk(doc)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)
	for i, chunk := range chunks {
		require.LessOrEqual(t, chunk.SourceEnd, len([]rune(doc.Text)))
		require.GreaterOrEqual(t, chunk.SourceStart, 0)
		require.Less(t, chunk.SourceStart, chunk.SourceEnd)
		if i > 0 {
			require.Less(t, chunk.SourceStart, chunks[i-1].SourceEnd, "adjacent hard splits must overlap")
		}
	}
}

func TestChunkerMergesAdjacentParagraphsWithinBudget(t *testing.T) {
	c := NewChunker(ChunkerConfig{MaxTokens: 64, OverlapTokens: 4, ChunkingVersion: 1})
	chunks, err := c.Chunk(SourceDocument{ContentID: 9, ContentVersion: 1, Text: "# Head\n\nshort one\n\nshort two"})
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Equal(t, "Head", chunks[0].Heading)
}
