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
var ErrRagPublishedVersionUnavailable = errors.New("published content version unavailable")

type RagGeneration struct {
	ContentID       int64
	IndexVersion    int
	ChunkingVersion int
	EmbeddingModel  string
}

// CitationTruth is the repository-owned projection of a viewer-visible RAG
// chunk used to rebuild server-owned Agent citations.
type CitationTruth struct {
	ContentID      int64  `gorm:"column:content_id"`
	ContentVersion int    `gorm:"column:content_version"`
	ChunkIndex     int    `gorm:"column:chunk_index"`
	ChunkKey       string `gorm:"column:chunk_key"`
	Text           string `gorm:"column:text"`
	Title          string `gorm:"column:title"`
	Zone           string `gorm:"column:zone"`
}

type CitationLookup struct {
	ContentID      int64
	ContentVersion int
	ChunkIndex     int
	ChunkKey       string
}

type RagChunkRepository struct{ db *gorm.DB }

func NewRagChunkRepository(db *gorm.DB) *RagChunkRepository {
	return &RagChunkRepository{db: db}
}

func (r *RagChunkRepository) WithDB(db *gorm.DB) *RagChunkRepository {
	return &RagChunkRepository{db: db}
}

// LoadVisibleCitationTruth enforces the same current-generation and
// viewer-visibility predicates used by retrieval before a citation is sent.
func (r *RagChunkRepository) LoadVisibleCitationTruth(ctx context.Context, viewerID int64, lookup CitationLookup) (CitationTruth, error) {
	var truth CitationTruth
	query := r.citationTruthQuery(ctx, viewerID).
		Where("rc.content_id = ? AND rc.content_version = ? AND rc.chunk_key = ? AND rc.chunk_index = ?", lookup.ContentID, lookup.ContentVersion, lookup.ChunkKey, lookup.ChunkIndex)
	return truth, query.Take(&truth).Error
}

// FirstVisibleCitationTruth returns the first current chunk for a visible
// content item so direct-detail tool results can gain server-owned provenance.
func (r *RagChunkRepository) FirstVisibleCitationTruth(ctx context.Context, viewerID, contentID int64) (CitationTruth, error) {
	var truth CitationTruth
	query := r.citationTruthQuery(ctx, viewerID).
		Where("rc.content_id = ?", contentID).
		Order("rc.chunk_index ASC")
	return truth, query.Take(&truth).Error
}

func (r *RagChunkRepository) citationTruthQuery(ctx context.Context, viewerID int64) *gorm.DB {
	query := r.db.WithContext(ctx).Table("rag_chunks AS rc").
		Select("rc.content_id, rc.content_version, rc.chunk_index, rc.chunk_key, rc.text, content_items.title, content_items.zone").
		Joins("JOIN content_items ON content_items.id = rc.content_id").
		Joins("JOIN content_versions ON content_versions.content_item_id = rc.content_id AND content_versions.version_number = rc.content_version AND content_versions.status = 'active' AND content_versions.is_latest = TRUE").
		Joins("JOIN index_projection_status ON index_projection_status.content_id = rc.content_id AND index_projection_status.index_version = rc.index_version AND index_projection_status.is_current = TRUE AND index_projection_status.state = 'ready' AND index_projection_status.chunking_version = rc.chunking_version")
	return ApplyContentVisibilityScope(query, viewerID)
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
		var published int64
		if countErr := r.db.WithContext(ctx).Table("content_items").
			Where("id = ? AND status = ? AND deleted_at IS NULL", contentID, "published").
			Count(&published).Error; countErr != nil {
			return nil, countErr
		}
		if published > 0 {
			return nil, ErrRagPublishedVersionUnavailable
		}
		return nil, ErrRagGenerationNotFound
	}
	return &version, err
}

// StageGeneration atomically replaces only the requested non-current
// generation. The current generation remains queryable if staging fails.
func (r *RagChunkRepository) StageGeneration(ctx context.Context, generation RagGeneration, chunks []model.RagChunk) error {
	return r.stageGeneration(ctx, generation, chunks, false)
}

// RestageGeneration replaces an already-current content projection inside the
// same global index generation. Callers must first prove current truth changed;
// ordinary at-least-once replay continues to use StageGeneration's no-op guard.
func (r *RagChunkRepository) RestageGeneration(ctx context.Context, generation RagGeneration, chunks []model.RagChunk) error {
	return r.stageGeneration(ctx, generation, chunks, true)
}

func (r *RagChunkRepository) stageGeneration(ctx context.Context, generation RagGeneration, chunks []model.RagChunk, replaceCurrent bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.IndexProjectionStatus
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("content_id = ? AND index_version = ?", generation.ContentID, generation.IndexVersion).
			Take(&existing).Error
		if err == nil && existing.IsCurrent && !replaceCurrent {
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
			DoUpdates: clause.AssignmentColumns([]string{"chunking_version", "embedding_model", "state", "error_summary", "is_current"}),
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

// DeleteProjection removes one content from a global generation after the
// external search projection has acknowledged its idempotent delete.
func (r *RagChunkRepository) DeleteProjection(ctx context.Context, contentID int64, indexVersion int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("content_id = ? AND index_version = ?", contentID, indexVersion).
			Delete(&model.RagChunk{}).Error; err != nil {
			return err
		}
		return tx.Where("content_id = ? AND index_version = ?", contentID, indexVersion).
			Delete(&model.IndexProjectionStatus{}).Error
	})
}

func (r *RagChunkRepository) ProjectionVersions(ctx context.Context) ([]int, error) {
	var versions []int
	err := r.db.WithContext(ctx).Table("index_projection_status").Distinct("index_version").
		Order("index_version ASC").Pluck("index_version", &versions).Error
	return versions, err
}

func (r *RagChunkRepository) DeleteAllProjections(ctx context.Context, contentID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("content_id = ?", contentID).Delete(&model.RagChunk{}).Error; err != nil {
			return err
		}
		return tx.Where("content_id = ?", contentID).Delete(&model.IndexProjectionStatus{}).Error
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

// IsCurrentFresh distinguishes a true at-least-once replay from metadata-only
// changes (for example a title update) that do not alter chunk identity.
func (r *RagChunkRepository) IsCurrentFresh(ctx context.Context, generation RagGeneration, truthUpdatedAt time.Time) (bool, error) {
	var status model.IndexProjectionStatus
	err := r.db.WithContext(ctx).
		Where("content_id = ? AND index_version = ? AND chunking_version = ? AND embedding_model = ? AND state = ? AND is_current = ?",
			generation.ContentID, generation.IndexVersion, generation.ChunkingVersion, generation.EmbeddingModel, "ready", true).
		Take(&status).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return status.LastIndexedAt != nil && !status.LastIndexedAt.Before(truthUpdatedAt), nil
}

func (r *RagChunkRepository) HasCurrentGeneration(ctx context.Context, generation RagGeneration) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.IndexProjectionStatus{}).
		Where("content_id = ? AND index_version = ? AND chunking_version = ? AND embedding_model = ? AND state = ? AND is_current = ?",
			generation.ContentID, generation.IndexVersion, generation.ChunkingVersion, generation.EmbeddingModel, "ready", true).
		Count(&count).Error
	return count == 1, err
}
