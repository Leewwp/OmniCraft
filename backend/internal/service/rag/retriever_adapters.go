package rag

import (
	"context"
	"sort"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

type openSearchChunkSearcher interface {
	Search(ctx context.Context, query string, topK int) ([]repository.SearchResult, error)
}

type OpenSearchKeywordRetriever struct {
	searcher openSearchChunkSearcher
}

func NewOpenSearchKeywordRetriever(searcher openSearchChunkSearcher) *OpenSearchKeywordRetriever {
	return &OpenSearchKeywordRetriever{searcher: searcher}
}

func (r *OpenSearchKeywordRetriever) Search(ctx context.Context, query string, topK int, _ int64) ([]RetrievalCandidate, error) {
	results, err := r.searcher.Search(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	candidates := make([]RetrievalCandidate, 0, len(results))
	for _, result := range results {
		candidates = append(candidates, candidateFromDocument(result.Document))
	}
	return candidates, nil
}

type PostgresKeywordRetriever struct {
	searcher postgresKeywordSearcher
}

type postgresKeywordSearcher interface {
	SearchContents(query string, zone, category, contentType string, tagFilters []string, sort, timeRange string, page, pageSize int, viewerID int64) ([]repository.ContentSearchResult, int64, error)
	SearchRAGChunks(ctx context.Context, query string, topK int, viewerID int64) ([]repository.RAGChunkSearchResult, error)
}

func NewPostgresKeywordRetriever(searcher postgresKeywordSearcher) *PostgresKeywordRetriever {
	return &PostgresKeywordRetriever{searcher: searcher}
}

func (r *PostgresKeywordRetriever) Search(ctx context.Context, query string, topK int, viewerID int64) ([]RetrievalCandidate, error) {
	pageSize := topK
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 50
	}
	contentResults := make([]repository.ContentSearchResult, 0, pageSize)
	for page := 1; len(contentResults) < topK || topK <= 0; page++ {
		results, total, err := r.searcher.SearchContents(query, "", "", "", nil, "", "", page, pageSize, viewerID)
		if err != nil {
			return nil, err
		}
		contentResults = append(contentResults, results...)
		if len(results) < pageSize || len(contentResults) >= int(total) || (topK > 0 && len(contentResults) >= topK) {
			break
		}
	}
	if len(contentResults) == 0 {
		return nil, nil
	}

	contentRank := make(map[int64]int, len(contentResults))
	for rank, result := range contentResults {
		contentRank[result.ID] = rank
	}
	results, err := r.searcher.SearchRAGChunks(ctx, query, topK, viewerID)
	if err != nil {
		return nil, err
	}
	candidates := make([]RetrievalCandidate, 0, len(results))
	for _, result := range results {
		if _, ok := contentRank[result.ContentID]; !ok {
			continue
		}
		candidate := candidateFromChunk(result.RagChunk)
		candidate.Title = result.Title
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return contentRank[candidates[i].ContentID] < contentRank[candidates[j].ContentID]
	})
	return candidates, nil
}

type PostgresVectorRetriever struct {
	searcher       *repository.EmbeddingRepository
	embeddingModel string
}

func NewPostgresVectorRetriever(searcher *repository.EmbeddingRepository, embeddingModel string) *PostgresVectorRetriever {
	return &PostgresVectorRetriever{searcher: searcher, embeddingModel: embeddingModel}
}

func (r *PostgresVectorRetriever) Search(ctx context.Context, embedding []float32, topK int, viewerID int64) ([]RetrievalCandidate, error) {
	results, err := r.searcher.VectorSearchChunks(ctx, embedding, topK, viewerID, r.embeddingModel)
	if err != nil {
		return nil, err
	}
	candidates := make([]RetrievalCandidate, 0, len(results))
	for _, result := range results {
		candidate := candidateFromChunk(result.RagChunk)
		candidate.EmbeddingModel = r.embeddingModel
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

type DatabaseVisibilityFilter struct {
	db *gorm.DB
}

func NewDatabaseVisibilityFilter(db *gorm.DB) *DatabaseVisibilityFilter {
	return &DatabaseVisibilityFilter{db: db}
}

type visibleChunk struct {
	ChunkKey        string `gorm:"column:chunk_key"`
	ContentID       int64  `gorm:"column:content_id"`
	ContentVersion  int    `gorm:"column:content_version"`
	ChunkingVersion int    `gorm:"column:chunking_version"`
	IndexVersion    int    `gorm:"column:index_version"`
	EmbeddingModel  string `gorm:"column:embedding_model"`
}

func (f *DatabaseVisibilityFilter) FilterVisible(ctx context.Context, viewerID int64, candidates []RetrievalCandidate) ([]RetrievalCandidate, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ChunkKey != "" {
			keys = append(keys, candidate.ChunkKey)
		}
	}
	if len(keys) == 0 {
		return nil, nil
	}
	var rows []visibleChunk
	query := f.db.WithContext(ctx).Table("rag_chunks").
		Select("rag_chunks.chunk_key, rag_chunks.content_id, rag_chunks.content_version, rag_chunks.chunking_version, rag_chunks.index_version, index_projection_status.embedding_model").
		Joins("JOIN content_items ON content_items.id = rag_chunks.content_id").
		Joins("JOIN content_versions ON content_versions.content_item_id = rag_chunks.content_id AND content_versions.version_number = rag_chunks.content_version AND content_versions.status = 'active' AND content_versions.is_latest = TRUE").
		Joins("JOIN index_projection_status ON index_projection_status.content_id = rag_chunks.content_id AND index_projection_status.index_version = rag_chunks.index_version").
		Where("rag_chunks.chunk_key IN ?", keys).
		Where("index_projection_status.is_current = TRUE AND index_projection_status.state = 'ready'").
		Where("index_projection_status.chunking_version = rag_chunks.chunking_version")
	query = repository.ApplyContentVisibilityScope(query, viewerID)
	err := query.
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	valid := make(map[string]visibleChunk, len(rows))
	for _, row := range rows {
		valid[row.ChunkKey] = row
	}
	result := make([]RetrievalCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		row, ok := valid[candidate.ChunkKey]
		if !ok || row.ContentID != candidate.ContentID || row.ContentVersion != candidate.ContentVersion || row.ChunkingVersion != candidate.ChunkingVersion || row.IndexVersion != candidate.IndexVersion {
			continue
		}
		if candidate.EmbeddingModel != "" && row.EmbeddingModel != candidate.EmbeddingModel {
			continue
		}
		result = append(result, candidate)
	}
	return result, nil
}

func candidateFromDocument(document repository.SearchDocument) RetrievalCandidate {
	return RetrievalCandidate{
		ChunkKey:        document.ChunkKey,
		ContentID:       document.ContentID,
		ContentVersion:  document.ContentVersion,
		ChunkingVersion: document.ChunkingVersion,
		IndexVersion:    document.IndexVersion,
		EmbeddingModel:  document.EmbeddingModel,
		Title:           document.Title,
		Heading:         document.Heading,
		Text:            document.Text,
		SourceStart:     document.SourceStart,
		SourceEnd:       document.SourceEnd,
		Zone:            document.Zone,
		ContentType:     document.ContentType,
		Category:        document.Category,
		IP:              document.IP,
		Tags:            append([]string(nil), document.Tags...),
	}
}

func candidateFromChunk(chunk model.RagChunk) RetrievalCandidate {
	return RetrievalCandidate{
		ChunkKey:        chunk.ChunkKey,
		ContentID:       chunk.ContentID,
		ContentVersion:  chunk.ContentVersion,
		ChunkingVersion: chunk.ChunkingVersion,
		IndexVersion:    chunk.IndexVersion,
		Title:           chunk.Heading,
		Heading:         chunk.Heading,
		Text:            chunk.Text,
		SourceStart:     chunk.SourceStart,
		SourceEnd:       chunk.SourceEnd,
		Zone:            chunk.Zone,
		ContentType:     chunk.ContentType,
		Category:        chunk.Category,
		IP:              chunk.IP,
		Tags:            append([]string(nil), chunk.Tags...),
	}
}
