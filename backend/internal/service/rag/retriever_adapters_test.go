package rag

import (
	"context"
	"testing"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

type recordingPostgresKeywordSearcher struct {
	contentSearchCalls int
	chunkSearchCalls   int
}

func (s *recordingPostgresKeywordSearcher) SearchContents(string, string, string, string, []string, string, string, int, int, int64) ([]repository.ContentSearchResult, int64, error) {
	s.contentSearchCalls++
	return []repository.ContentSearchResult{{ContentItem: model.ContentItem{ID: 2}, Score: 1}}, 1, nil
}

func (s *recordingPostgresKeywordSearcher) SearchRAGChunks(context.Context, string, int, int64) ([]repository.RAGChunkSearchResult, error) {
	s.chunkSearchCalls++
	return []repository.RAGChunkSearchResult{{RagChunk: model.RagChunk{ChunkKey: "chunk-2", ContentID: 2}, Title: "content 2"}}, nil
}

func TestPostgresKeywordRetrieverUsesViewerAwareContentSearchContract(t *testing.T) {
	searcher := &recordingPostgresKeywordSearcher{}
	retriever := NewPostgresKeywordRetriever(searcher)

	got, err := retriever.Search(context.Background(), "query", 10, 7)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if searcher.contentSearchCalls != 1 {
		t.Fatalf("SearchContents calls = %d, want 1", searcher.contentSearchCalls)
	}
	if searcher.chunkSearchCalls != 1 {
		t.Fatalf("SearchRAGChunks calls = %d, want 1", searcher.chunkSearchCalls)
	}
	if len(got) != 1 || got[0].ChunkKey != "chunk-2" {
		t.Fatalf("candidates = %#v, want chunk-2", got)
	}
}
