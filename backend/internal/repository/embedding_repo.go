package repository

import (
	"context"
	"fmt"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type EmbeddingRepository struct {
	db *gorm.DB
}

func NewEmbeddingRepository(db *gorm.DB) *EmbeddingRepository {
	return &EmbeddingRepository{db: db}
}

type EmbeddingSearchResult struct {
	ContentItemID int64   `gorm:"column:content_item_id"`
	Score         float64 `gorm:"column:score"`
}

type ChunkEmbeddingSearchResult struct {
	model.RagChunk
	Score float64 `gorm:"column:score"`
}

func (r *EmbeddingRepository) UpsertEmbedding(contentItemID int64, embedding []float32) error {
	return r.db.Exec(`
		INSERT INTO content_embeddings (content_item_id, embedding, embedded_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (content_item_id)
		DO UPDATE SET embedding = EXCLUDED.embedding, embedded_at = NOW()
	`, contentItemID, pgVector(embedding)).Error
}

// DeleteByContentIDTx removes every embedding row of one content inside the
// caller's transaction. The indexer worker calls it for content.banned and
// content.deleted events, in the same transaction as the inbox completion
// record.
func (r *EmbeddingRepository) DeleteByContentIDTx(tx *gorm.DB, contentItemID int64) error {
	return tx.Exec(`DELETE FROM content_embeddings WHERE content_item_id = ?`, contentItemID).Error
}

func (r *EmbeddingRepository) VectorSearch(embedding []float32, topK int) ([]EmbeddingSearchResult, error) {
	var results []EmbeddingSearchResult
	err := r.db.Raw(`
		SELECT content_item_id, 1 - (embedding <=> ?) AS score
		FROM content_embeddings
		ORDER BY score DESC
		LIMIT ?
	`, pgVector(embedding), topK).Scan(&results).Error
	return results, err
}

// VectorSearchChunks reads the current chunk embedding projection and repeats
// the complete viewer visibility predicate in SQL. The final retriever check
// still revalidates the returned chunk against current PostgreSQL truth.
func (r *EmbeddingRepository) VectorSearchChunks(ctx context.Context, embedding []float32, topK int, viewerID int64, embeddingModel string) ([]ChunkEmbeddingSearchResult, error) {
	if topK <= 0 {
		topK = config.RAGDefaultVectorTopK
	}
	var results []ChunkEmbeddingSearchResult
	err := r.db.WithContext(ctx).Raw(`
		SELECT rc.*,
		       1 - (ce.embedding <=> ?) AS score
		FROM chunk_embeddings AS ce
		JOIN rag_chunks AS rc ON rc.id = ce.chunk_id
		JOIN content_items AS ci ON ci.id = rc.content_id
		JOIN index_projection_status AS ips
		  ON ips.content_id = rc.content_id
		 AND ips.index_version = rc.index_version
		 AND ips.is_current = TRUE
		 AND ips.state = 'ready'
		 AND ips.chunking_version = rc.chunking_version
		 AND ips.embedding_model = ce.embedding_model
		WHERE ce.embedding_model = ?
		  AND ci.status = 'published'
		  AND ci.deleted_at IS NULL
		  AND NOT EXISTS (
			  SELECT 1 FROM users AS author
			  WHERE author.id = ci.author_id
				AND (author.is_banned = TRUE OR author.deleted_at IS NOT NULL)
		  )
		  AND (ci.ip_id IS NULL OR NOT EXISTS (
			  SELECT 1 FROM ips AS ip WHERE ip.id = ci.ip_id AND ip.status = 'banned'
		  ))
		  AND (ci.is_public = TRUE OR ci.author_id = ?)
		ORDER BY score DESC, rc.chunk_key ASC
		LIMIT ?
	`, pgVector(embedding), embeddingModel, viewerID, topK).Scan(&results).Error
	return results, err
}

func pgVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	b := make([]byte, 0, len(v)*8)
	b = append(b, '[')
	for i, f := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, []byte(formatFloat32(f))...)
	}
	b = append(b, ']')
	return string(b)
}

func formatFloat32(f float32) string {
	return fmt.Sprintf("%g", f)
}
