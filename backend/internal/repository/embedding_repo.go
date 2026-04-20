package repository

import (
	"fmt"

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

func (r *EmbeddingRepository) UpsertEmbedding(contentItemID int64, embedding []float32) error {
	return r.db.Exec(`
		INSERT INTO content_embeddings (content_item_id, embedding, embedded_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (content_item_id)
		DO UPDATE SET embedding = EXCLUDED.embedding, embedded_at = NOW()
	`, contentItemID, pgVector(embedding)).Error
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
