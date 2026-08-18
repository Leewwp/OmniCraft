package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/internal/model"
)

var ErrRagGenerationNotFound = errors.New("rag generation not found")

type RagGeneration struct {
	ContentID       int64
	IndexVersion    int
	ChunkingVersion int
	EmbeddingModel  string
}

type RagChunkRepository struct{ db *gorm.DB }

func NewRagChunkRepository(db *gorm.DB) *RagChunkRepository {
	return &RagChunkRepository{db: db}
}

// LatestPublishedVersion is the only content-version read seam used by the
// projection: unpublished content and inactive/non-latest versions never pass.
func (r *RagChunkRepository) LatestPublishedVersion(ctx context.Context, contentID int64) (*model.ContentVersion, error) {
	var version model.ContentVersion
	err := r.db.WithContext(ctx).Table("content_versions AS cv").
		Select("cv.*").
		Joins("JOIN content_items AS ci ON ci.id = cv.content_item_id").
		Where("ci.id = ? AND ci.status = ? AND ci.deleted_at IS NULL", contentID, "published").
		Where("cv.status = ? AND cv.is_latest = ?", "active", true).
		Order("cv.version_number DESC").
		First(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRagGenerationNotFound
	}
	return &version, err
}

// StageGeneration atomically replaces only the requested non-current
// generation. The current generation remains queryable if staging fails.
func (r *RagChunkRepository) StageGeneration(ctx context.Context, generation RagGeneration, chunks []model.RagChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.IndexProjectionStatus
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("content_id = ? AND index_version = ?", generation.ContentID, generation.IndexVersion).
			Take(&existing).Error
		if err == nil && existing.IsCurrent {
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		status := model.IndexProjectionStatus{
			ContentID: generation.ContentID, IndexVersion: generation.IndexVersion,
			ChunkingVersion: generation.ChunkingVersion, EmbeddingModel: generation.EmbeddingModel,
			State: "staging", ErrorSummary: "", IsCurrent: false,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "content_id"}, {Name: "index_version"}},
			DoUpdates: clause.AssignmentColumns([]string{"chunking_version", "embedding_model", "state", "error_summary"}),
		}).Create(&status).Error; err != nil {
			return err
		}
		if err := tx.Where("content_id = ? AND index_version = ?", generation.ContentID, generation.IndexVersion).Delete(&model.RagChunk{}).Error; err != nil {
			return err
		}
		staged := append([]model.RagChunk(nil), chunks...)
		for i := range staged {
			staged[i].ContentID = generation.ContentID
			staged[i].IndexVersion = generation.IndexVersion
			staged[i].ChunkingVersion = generation.ChunkingVersion
		}
		if len(staged) == 0 {
			return nil
		}
		return tx.Create(&staged).Error
	})
}

func (r *RagChunkRepository) MarkFailed(ctx context.Context, generation RagGeneration, summary string) error {
	summary = strings.Join(strings.Fields(summary), " ")
	summaryRunes := []rune(summary)
	if len(summaryRunes) > 500 {
		summary = string(summaryRunes[:500])
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var status model.IndexProjectionStatus
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("content_id = ? AND index_version = ?", generation.ContentID, generation.IndexVersion).
			Take(&status).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRagGenerationNotFound
			}
			return err
		}
		if status.IsCurrent {
			return nil
		}
		return tx.Model(&status).Updates(map[string]any{
			"state": "failed", "error_summary": summary, "is_current": false,
		}).Error
	})
}

func (r *RagChunkRepository) PromoteGeneration(ctx context.Context, generation RagGeneration) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var status model.IndexProjectionStatus
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("content_id = ? AND index_version = ?", generation.ContentID, generation.IndexVersion).
			First(&status).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRagGenerationNotFound
			}
			return err
		}
		matches := status.ChunkingVersion == generation.ChunkingVersion && status.EmbeddingModel == generation.EmbeddingModel
		if matches && status.State == "ready" && status.IsCurrent {
			return nil
		}
		if !matches || status.State != "staging" || status.IsCurrent {
			return ErrRagGenerationNotFound
		}
		if err := tx.Model(&model.IndexProjectionStatus{}).Where("content_id = ? AND is_current = ?", generation.ContentID, true).
			Update("is_current", false).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		return tx.Model(&status).Updates(map[string]any{
			"state": "ready", "error_summary": "", "last_indexed_at": now, "is_current": true,
		}).Error
	})
}

// ListCurrent returns only the ready generation matching every generation
// dimension. This prevents old chunks from leaking during rebuilds.
func (r *RagChunkRepository) ListCurrent(ctx context.Context, contentID int64, chunkingVersion int, embeddingModel string) ([]model.RagChunk, error) {
	var chunks []model.RagChunk
	err := r.db.WithContext(ctx).Table("rag_chunks AS rc").Select("rc.*").
		Joins(`JOIN index_projection_status AS ips
              ON ips.content_id = rc.content_id AND ips.index_version = rc.index_version`).
		Where("rc.content_id = ? AND rc.chunking_version = ?", contentID, chunkingVersion).
		Where("ips.is_current = ? AND ips.state = ? AND ips.chunking_version = ? AND ips.embedding_model = ?", true, "ready", chunkingVersion, embeddingModel).
		Order("rc.chunk_index ASC").Find(&chunks).Error
	return chunks, err
}
