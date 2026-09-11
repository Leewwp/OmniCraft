package rag

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	baseService "omnicraft/backend/internal/service"
)

var ErrProjectionUnavailable = errors.New("content projection unavailable")

const projectionAdvisoryLockKey int64 = 0x4f4d4e49524147
const projectionContentLockBase int64 = 0x5241470000000000

var projectionUnlockAll = func(ctx context.Context, bound *gorm.DB) error {
	return bound.WithContext(ctx).Exec("SELECT pg_advisory_unlock_all()").Error
}

var projectionDiscardPhysicalConnection = func(conn *sql.Conn) error {
	return conn.Raw(func(any) error { return driver.ErrBadConn })
}

const ragIndexPrefix = "omnicraft-rag-v"
const ragReadAlias = "omnicraft-rag-read"

type ProjectionConfig struct {
	IndexVersion          int
	EmbeddingModel        string
	EmbeddingDimensions   int
	LockCleanupTimeoutSec int
}

type ContentVersionLoader interface {
	LoadLatestPublishedContent(ctx context.Context, contentID int64) (int, string, error)
}

type SearchDocument = repository.SearchDocument

type ChunkEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type SearchProjection interface {
	UpsertContent(ctx context.Context, index string, documents []SearchDocument) error
	PruneStaleContent(ctx context.Context, index string, contentID int64, keepIDs []string) error
	DeleteContent(ctx context.Context, index string, contentID int64) error
}

type SearchIndexManager interface {
	SearchProjection
	CreateIndex(ctx context.Context, index string) error
	ValidateIndex(ctx context.Context, index string, expectedDocuments int64) error
	SwapAlias(ctx context.Context, alias string, index string) error
	RemoveAlias(ctx context.Context, alias string) error
	AliasTarget(ctx context.Context, alias string) (string, error)
	ListIndexes(ctx context.Context, prefix string) ([]string, error)
	DeleteIndex(ctx context.Context, index string) error
}

type Projection struct {
	db       *gorm.DB
	chunks   *repository.RagChunkRepository
	chunker  *Chunker
	embedder ChunkEmbedder
	search   SearchProjection
	versions ContentVersionLoader
	config   ProjectionConfig
}

func NewProjection(db *gorm.DB, chunker *Chunker, embedder ChunkEmbedder, search SearchProjection, config ProjectionConfig) *Projection {
	return NewProjectionWithVersionLoader(
		db, chunker, embedder, search,
		baseService.NewVersionService(repository.NewVersionRepository(db), repository.NewContentRepository(db)),
		config,
	)
}

func NewProjectionWithVersionLoader(db *gorm.DB, chunker *Chunker, embedder ChunkEmbedder, search SearchProjection, versions ContentVersionLoader, config ProjectionConfig) *Projection {
	return &Projection{
		db: db, chunks: repository.NewRagChunkRepository(db), chunker: chunker,
		embedder: embedder, search: search, versions: versions, config: config,
	}
}

func (p *Projection) SyncContent(ctx context.Context, contentID int64) error {
	return p.withProjectionConnection(ctx, true, contentID, func(scoped *Projection) error {
		if checker, ok := scoped.search.(interface{ Health(context.Context) error }); ok {
			if err := checker.Health(ctx); err != nil {
				return ErrProjectionUnavailable
			}
		}
		indexVersion, err := scoped.activeIndexVersion(ctx)
		if err != nil {
			return err
		}
		return scoped.syncContent(ctx, contentID, indexVersion, true)
	})
}

func (p *Projection) withDB(db *gorm.DB) *Projection {
	clone := *p
	scoped := db.Session(&gorm.Session{})
	clone.db = scoped
	clone.chunks = p.chunks.WithDB(scoped)
	if versions, ok := p.versions.(*baseService.VersionService); ok {
		clone.versions = versions.WithDB(scoped)
	}
	return &clone
}

func (p *Projection) activeIndexVersion(ctx context.Context) (int, error) {
	if manager, ok := p.search.(SearchIndexManager); ok {
		target, exists, err := p.resolveAliasTarget(ctx, manager)
		if err != nil {
			return 0, err
		}
		if exists {
			version, valid := p.parseIndexVersion(target)
			if !valid {
				return 0, ErrProjectionUnavailable
			}
			return version, nil
		}
	}
	var current int
	if err := p.db.WithContext(ctx).Table("index_projection_status").Select("COALESCE(MAX(index_version), 0)").
		Where("is_current = ?", true).Scan(&current).Error; err != nil {
		return 0, fmt.Errorf("read current projection generation: %w", err)
	}
	if current > 0 {
		return current, nil
	}
	return p.config.IndexVersion, nil
}

func (p *Projection) syncContent(ctx context.Context, contentID int64, indexVersion int, promote bool) error {
	var content projectionContent
	if err := p.db.WithContext(ctx).Table("content_items").
		Select("id, title, zone, content_type, category, ip_id, status, updated_at").
		Where("id = ? AND status = ? AND deleted_at IS NULL", contentID, "published").
		Take(&content).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return p.removeContent(ctx, contentID, indexVersion)
		}
		return fmt.Errorf("load published content projection: %w", err)
	}
	if p.versions == nil {
		return ErrProjectionUnavailable
	}
	contentVersion, contentText, err := p.versions.LoadLatestPublishedContent(ctx, contentID)
	if err != nil {
		if errors.Is(err, baseService.ErrVersionNotFound) {
			return ErrProjectionUnavailable
		}
		return err
	}
	var tags pq.StringArray
	if err := p.db.WithContext(ctx).Table("content_tags").Where("content_item_id = ?", contentID).
		Order("tag ASC").Pluck("tag", &tags).Error; err != nil {
		return fmt.Errorf("load content projection tags: %w", err)
	}

	chunked, err := p.chunker.Chunk(SourceDocument{
		ContentID: contentID, ContentVersion: contentVersion,
		Title: content.Title, Text: contentText,
	})
	if err != nil {
		return fmt.Errorf("chunk content projection: %w", err)
	}
	generation := repository.RagGeneration{
		ContentID: contentID, IndexVersion: indexVersion,
		ChunkingVersion: chunkedVersion(chunked), EmbeddingModel: p.config.EmbeddingModel,
	}
	chunks := make([]model.RagChunk, len(chunked))
	for i, chunk := range chunked {
		chunks[i] = model.RagChunk{
			ContentID: contentID, IndexVersion: generation.IndexVersion, ChunkingVersion: generation.ChunkingVersion,
			ContentVersion: contentVersion, ChunkIndex: chunk.ChunkIndex,
			ChunkKey: chunk.ChunkKey, Heading: chunk.Heading, Text: chunk.Text,
			SourceStart: chunk.SourceStart, SourceEnd: chunk.SourceEnd,
			Zone: content.Zone, ContentType: content.ContentType, Category: content.Category,
			IP: content.IP, Tags: tags,
		}
	}
	current, err := p.chunks.ListCurrent(ctx, contentID, generation.ChunkingVersion, generation.EmbeddingModel)
	if err != nil {
		return fmt.Errorf("load current content projection: %w", err)
	}
	if promote && sameProjectedChunks(current, chunks, generation.IndexVersion) {
		fresh, freshErr := p.chunks.IsCurrentFresh(ctx, generation, content.UpdatedAt)
		if freshErr != nil {
			return fmt.Errorf("check current content projection freshness: %w", freshErr)
		}
		if fresh {
			return nil
		}
	}
	refreshingCurrent, err := p.chunks.HasCurrentGeneration(ctx, generation)
	if err != nil {
		return fmt.Errorf("check current content projection: %w", err)
	}
	staged := chunks
	if !refreshingCurrent {
		if err := p.chunks.StageGeneration(ctx, generation, chunks); err != nil {
			return fmt.Errorf("stage content projection: %w", err)
		}
		if err := p.db.WithContext(ctx).Where("content_id = ? AND index_version = ?", contentID, indexVersion).
			Order("chunk_index ASC").Find(&staged).Error; err != nil {
			return fmt.Errorf("load staged content projection: %w", err)
		}
	}
	texts := make([]string, len(staged))
	for i := range staged {
		texts[i] = staged[i].Text
	}
	vectors, err := p.embedder.Embed(ctx, texts)
	if err != nil {
		return p.failSync(ctx, generation, refreshingCurrent, "embedding provider unavailable")
	}
	if len(vectors) != len(staged) {
		return p.failSync(ctx, generation, refreshingCurrent, "embedding provider returned invalid output")
	}
	if err := p.validateVectorDimensions(vectors); err != nil {
		return p.failSync(ctx, generation, refreshingCurrent, "embedding provider returned invalid output")
	}
	if !refreshingCurrent {
		if err := p.storeEmbeddings(ctx, staged, vectors); err != nil {
			return p.failGeneration(ctx, generation, "embedding storage unavailable")
		}
	}

	documents := projectionDocuments(staged, contentID, p.config.EmbeddingModel, content)
	index := fmt.Sprintf("%s%d", ragIndexPrefix, indexVersion)
	if manager, ok := p.search.(SearchIndexManager); ok {
		if err := manager.CreateIndex(ctx, index); err != nil {
			return p.failSync(ctx, generation, refreshingCurrent, "search projection unavailable")
		}
	}
	if len(documents) > 0 {
		if err := p.search.UpsertContent(ctx, index, documents); err != nil {
			return p.failSync(ctx, generation, refreshingCurrent, "search projection unavailable")
		}
	}
	if refreshingCurrent {
		if err := p.replaceCurrentProjection(ctx, generation, staged, vectors); err != nil {
			return fmt.Errorf("replace current content projection: %w", err)
		}
	}
	if err := p.search.PruneStaleContent(ctx, index, contentID, documentIDsForProjection(documents)); err != nil {
		return p.failSync(ctx, generation, refreshingCurrent, "search projection unavailable")
	}
	if manager, ok := p.search.(SearchIndexManager); ok {
		_, exists, aliasErr := p.resolveAliasTarget(ctx, manager)
		if aliasErr != nil {
			return p.failSync(ctx, generation, refreshingCurrent, "search projection alias unavailable")
		}
		if !exists {
			if err := manager.SwapAlias(ctx, ragReadAlias, index); err != nil {
				resolved, resolveErr := manager.AliasTarget(ctx, ragReadAlias)
				if resolveErr != nil || resolved != index {
					return p.failSync(ctx, generation, refreshingCurrent, "search projection alias unavailable")
				}
			}
		}
	}
	if promote {
		if refreshingCurrent {
			if err := p.markCurrentIndexed(ctx, generation); err != nil {
				return fmt.Errorf("mark current content projection indexed: %w", err)
			}
		} else if err := p.chunks.PromoteGeneration(ctx, generation); err != nil {
			return fmt.Errorf("promote content projection: %w", err)
		}
	}
	return nil
}

func documentIDsForProjection(documents []SearchDocument) []string {
	ids := make([]string, len(documents))
	for i := range documents {
		ids[i] = documents[i].ID
	}
	return ids
}

func projectionDocuments(chunks []model.RagChunk, contentID int64, embeddingModel string, content projectionContent) []SearchDocument {
	documents := make([]SearchDocument, len(chunks))
	for i, chunk := range chunks {
		documents[i] = SearchDocument{
			ID: chunk.ChunkKey, ChunkKey: chunk.ChunkKey, ContentID: contentID,
			ContentVersion: chunk.ContentVersion, ChunkIndex: chunk.ChunkIndex, ChunkingVersion: chunk.ChunkingVersion,
			IndexVersion: chunk.IndexVersion, EmbeddingModel: embeddingModel,
			Title: content.Title, Heading: chunk.Heading, Text: chunk.Text,
			SourceStart: chunk.SourceStart, SourceEnd: chunk.SourceEnd,
			Zone: chunk.Zone, ContentType: chunk.ContentType, Category: chunk.Category,
			IP: chunk.IP, Tags: append([]string(nil), chunk.Tags...), Status: content.Status,
		}
	}
	return documents
}

func (p *Projection) failSync(ctx context.Context, generation repository.RagGeneration, refreshingCurrent bool, summary string) error {
	if refreshingCurrent {
		return ErrProjectionUnavailable
	}
	return p.failGeneration(ctx, generation, summary)
}

func (p *Projection) failGeneration(ctx context.Context, generation repository.RagGeneration, summary string) error {
	if err := p.chunks.MarkFailed(ctx, generation, summary); err != nil {
		return fmt.Errorf("%w: persist failure state", ErrProjectionUnavailable)
	}
	return ErrProjectionUnavailable
}

func (p *Projection) removeContent(ctx context.Context, contentID int64, indexVersion int) error {
	versions, err := p.chunks.ProjectionVersions(ctx)
	if err != nil {
		return fmt.Errorf("list search projection generations: %w", err)
	}
	seen := make(map[int]struct{}, len(versions)+1)
	versions = append(versions, indexVersion)
	if manager, ok := p.search.(SearchIndexManager); ok {
		indexes, listErr := manager.ListIndexes(ctx, ragIndexPrefix)
		if listErr != nil {
			return fmt.Errorf("list search projection indexes: %w", listErr)
		}
		for _, index := range indexes {
			if version, valid := p.parseIndexVersion(index); valid {
				versions = append(versions, version)
			}
		}
	}
	for _, version := range versions {
		if version <= 0 {
			continue
		}
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}
		index := fmt.Sprintf("%s%d", ragIndexPrefix, version)
		if err := p.search.DeleteContent(ctx, index, contentID); err != nil {
			return fmt.Errorf("delete search content projection: %w", err)
		}
	}
	if err := p.chunks.DeleteAllProjections(ctx, contentID); err != nil {
		return fmt.Errorf("delete postgres content projection: %w", err)
	}
	return nil
}

func (p *Projection) Rebuild(ctx context.Context) error {
	return p.withProjectionConnection(ctx, false, 0, func(scoped *Projection) error {
		return scoped.rebuild(ctx)
	})
}

func (p *Projection) rebuild(ctx context.Context) error {
	manager, ok := p.search.(SearchIndexManager)
	if !ok {
		return ErrProjectionUnavailable
	}
	if checker, ok := p.search.(interface{ Health(context.Context) error }); ok {
		if err := checker.Health(ctx); err != nil {
			return ErrProjectionUnavailable
		}
	}
	previousVersion, err := p.currentProjectionVersion(ctx)
	if err != nil {
		return err
	}
	reconciled, reconciledVersion, err := p.reconcileAlias(ctx, manager)
	if err != nil {
		return err
	}
	if reconciled {
		previousIndex := ""
		if previousVersion > 0 && previousVersion != reconciledVersion {
			previousIndex = fmt.Sprintf("%s%d", ragIndexPrefix, previousVersion)
		}
		return p.retainRebuildGenerations(ctx, manager, reconciledVersion, previousIndex)
	}
	var highest int
	if err := p.db.WithContext(ctx).Table("index_projection_status").
		Select("COALESCE(MAX(index_version), 0)").Scan(&highest).Error; err != nil {
		return fmt.Errorf("read projection generation: %w", err)
	}
	if highest < p.config.IndexVersion {
		highest = p.config.IndexVersion
	}
	aliasTarget, aliasExists, aliasErr := p.resolveAliasTarget(ctx, manager)
	if aliasErr != nil {
		return aliasErr
	}
	if aliasExists {
		if aliasVersion, ok := p.parseIndexVersion(aliasTarget); !ok {
			return ErrProjectionUnavailable
		} else if aliasVersion > highest {
			highest = aliasVersion
		}
	}
	targetVersion := highest + 1
	targetIndex := fmt.Sprintf("%s%d", ragIndexPrefix, targetVersion)
	previousIndex := aliasTarget
	if targetIndex == previousIndex {
		return ErrProjectionUnavailable
	}
	if err := manager.DeleteIndex(ctx, targetIndex); err != nil {
		return ErrProjectionUnavailable
	}
	if err := manager.CreateIndex(ctx, targetIndex); err != nil {
		return ErrProjectionUnavailable
	}
	var contentIDs []int64
	if err := p.db.WithContext(ctx).Table("content_items").Where("status = ? AND deleted_at IS NULL", "published").
		Order("id ASC").Pluck("id", &contentIDs).Error; err != nil {
		return fmt.Errorf("list published rebuild contents: %w", err)
	}
	for _, contentID := range contentIDs {
		if err := p.syncContent(ctx, contentID, targetVersion, false); err != nil {
			return p.failRebuild(ctx, targetVersion, "rebuild content projection unavailable")
		}
	}
	var expectedDocuments int64
	if err := p.db.WithContext(ctx).Model(&model.RagChunk{}).Where("index_version = ?", targetVersion).
		Count(&expectedDocuments).Error; err != nil {
		return p.failRebuild(ctx, targetVersion, "rebuild validation unavailable")
	}
	if err := manager.ValidateIndex(ctx, targetIndex, expectedDocuments); err != nil {
		return p.failRebuild(ctx, targetVersion, "rebuild validation unavailable")
	}
	if err := manager.SwapAlias(ctx, ragReadAlias, targetIndex); err != nil {
		resolved, resolveErr := manager.AliasTarget(ctx, ragReadAlias)
		if resolveErr != nil || resolved != targetIndex {
			return p.failRebuild(ctx, targetVersion, "rebuild alias swap unavailable")
		}
	}
	if err := p.promoteRebuild(ctx, targetVersion, int64(len(contentIDs))); err != nil {
		if rollbackErr := p.restoreAliasAfterPromotionFailure(ctx, manager, previousIndex); rollbackErr != nil {
			return fmt.Errorf("promote rebuilt projection: %w; restore previous alias: %v", err, rollbackErr)
		}
		return fmt.Errorf("promote rebuilt projection generation: %w", err)
	}
	if err := p.retainRebuildGenerations(ctx, manager, targetVersion, previousIndex); err != nil {
		return err
	}
	return nil
}

func (p *Projection) restoreAliasAfterPromotionFailure(ctx context.Context, manager SearchIndexManager, previousIndex string) error {
	if previousIndex == "" {
		return manager.RemoveAlias(ctx, ragReadAlias)
	}
	if err := manager.SwapAlias(ctx, ragReadAlias, previousIndex); err != nil {
		resolved, resolveErr := manager.AliasTarget(ctx, ragReadAlias)
		if resolveErr == nil && resolved == previousIndex {
			return nil
		}
		return err
	}
	return nil
}

func (p *Projection) retainRebuildGenerations(ctx context.Context, manager SearchIndexManager, targetVersion int, previousIndex string) error {
	previousVersion, previousValid := p.parseIndexVersion(previousIndex)
	if previousIndex != "" && !previousValid {
		return ErrProjectionUnavailable
	}
	var registeredVersions []int
	if err := p.db.WithContext(ctx).Table("index_projection_status").Distinct("index_version").
		Order("index_version ASC").Pluck("index_version", &registeredVersions).Error; err != nil {
		return fmt.Errorf("list stale projection generations: %w", err)
	}
	indexes, err := manager.ListIndexes(ctx, ragIndexPrefix)
	if err != nil {
		return ErrProjectionUnavailable
	}
	staleSet := make(map[int]struct{}, len(registeredVersions)+len(indexes))
	for _, version := range registeredVersions {
		staleSet[version] = struct{}{}
	}
	for _, index := range indexes {
		if version, valid := p.parseIndexVersion(index); valid {
			staleSet[version] = struct{}{}
		}
	}
	delete(staleSet, targetVersion)
	if previousValid {
		delete(staleSet, previousVersion)
	}
	staleVersions := make([]int, 0, len(staleSet))
	for version := range staleSet {
		staleVersions = append(staleVersions, version)
	}
	sort.Ints(staleVersions)
	for _, version := range staleVersions {
		index := fmt.Sprintf("%s%d", ragIndexPrefix, version)
		if err := manager.DeleteIndex(ctx, index); err != nil {
			return ErrProjectionUnavailable
		}
		if err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("index_version = ?", version).Delete(&model.RagChunk{}).Error; err != nil {
				return err
			}
			return tx.Where("index_version = ?", version).Delete(&model.IndexProjectionStatus{}).Error
		}); err != nil {
			return fmt.Errorf("delete stale projection generation: %w", err)
		}
	}
	return nil
}

func (p *Projection) withProjectionConnection(ctx context.Context, shared bool, contentID int64, operation func(*Projection) error) error {
	return p.db.WithContext(ctx).Connection(func(bound *gorm.DB) (resultErr error) {
		conn, ok := bound.Statement.ConnPool.(*sql.Conn)
		if !ok {
			return errors.New("projection connection binding unavailable")
		}
		lockErr := acquireProjectionSessionLocks(bound, shared, contentID)
		if lockErr != nil {
			if cleanupErr := cleanupProjectionSessionLocks(ctx, bound, conn, p.config.LockCleanupTimeoutSec); cleanupErr != nil {
				slog.Error("projection lock connection discarded", "phase", "acquire")
			}
			return lockErr
		}
		defer func() {
			panicValue := recover()
			if cleanupErr := cleanupProjectionSessionLocks(ctx, bound, conn, p.config.LockCleanupTimeoutSec); cleanupErr != nil {
				slog.Error("projection lock connection discarded", "phase", "release")
				if resultErr == nil {
					resultErr = ErrProjectionUnavailable
				}
			}
			if panicValue != nil {
				slog.Error("projection operation panicked")
				resultErr = ErrProjectionUnavailable
			}
		}()
		resultErr = operation(p.withDB(bound))
		return resultErr
	})
}

func acquireProjectionSessionLocks(db *gorm.DB, shared bool, contentID int64) error {
	var lockErr error
	if shared {
		lockErr = db.Exec("SELECT pg_advisory_lock_shared(?)", projectionAdvisoryLockKey).Error
	} else {
		lockErr = db.Exec("SELECT pg_advisory_lock(?)", projectionAdvisoryLockKey).Error
	}
	if lockErr != nil {
		return fmt.Errorf("acquire projection advisory lock: %w", lockErr)
	}
	if shared {
		contentLockKey := projectionContentLockBase ^ contentID
		if err := db.Exec("SELECT pg_advisory_lock(?)", contentLockKey).Error; err != nil {
			return fmt.Errorf("acquire content projection lock: %w", err)
		}
	}
	return nil
}

func cleanupProjectionSessionLocks(requestCtx context.Context, bound *gorm.DB, conn *sql.Conn, timeoutSec int) error {
	cleanupCtx := context.WithoutCancel(requestCtx)
	cancel := func() {}
	if timeoutSec > 0 {
		cleanupCtx, cancel = context.WithTimeout(cleanupCtx, time.Duration(timeoutSec)*time.Second)
	}
	defer cancel()
	if err := projectionUnlockAll(cleanupCtx, bound); err == nil {
		return nil
	}
	// An ambiguous unlock must never return a possibly locked physical session
	// to database/sql. ErrBadConn forces the pool to discard that session.
	if err := projectionDiscardPhysicalConnection(conn); err != nil {
		slog.Error("projection physical connection discard failed", "error", err)
	}
	return ErrProjectionUnavailable
}

func (p *Projection) promoteRebuild(ctx context.Context, targetVersion int, expectedContents int64) error {
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.IndexProjectionStatus{}).Where("is_current = ?", true).
			Update("is_current", false).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		result := tx.Model(&model.IndexProjectionStatus{}).Where("index_version = ? AND state IN ?", targetVersion, []string{"staging", "ready"}).
			Updates(map[string]any{"state": "ready", "is_current": true, "last_indexed_at": now, "error_summary": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != expectedContents {
			return ErrProjectionUnavailable
		}
		return nil
	})
}

func (p *Projection) reconcileAlias(ctx context.Context, manager SearchIndexManager) (bool, int, error) {
	target, exists, err := p.resolveAliasTarget(ctx, manager)
	if err != nil {
		return false, 0, err
	}
	if !exists {
		return false, 0, nil
	}
	version, ok := p.parseIndexVersion(target)
	if !ok {
		return false, 0, ErrProjectionUnavailable
	}
	var publishedIDs []int64
	if err := p.db.WithContext(ctx).Table("content_items").Where("status = ? AND deleted_at IS NULL", "published").
		Order("id ASC").Pluck("id", &publishedIDs).Error; err != nil {
		return false, 0, fmt.Errorf("list published rebuild contents: %w", err)
	}
	if len(publishedIDs) == 0 {
		return false, 0, nil
	}
	complete, allCurrent, err := p.generationState(ctx, version, false)
	if err != nil {
		return false, 0, err
	}
	if complete {
		if allCurrent {
			return false, 0, nil
		}
		if err := p.promoteRebuild(ctx, version, int64(len(publishedIDs))); err != nil {
			return false, 0, fmt.Errorf("reconcile alias projection generation: %w", err)
		}
		return true, version, nil
	}

	// An alias can survive a process crash or an operator-side partial write.
	// Restore the last complete PostgreSQL generation before starting a fresh
	// rebuild, so readers never remain pointed at an incomplete index.
	fallbackVersion, err := p.currentProjectionVersion(ctx)
	if err != nil {
		return false, 0, err
	}
	if fallbackVersion <= 0 || fallbackVersion == version {
		if err := manager.RemoveAlias(ctx, ragReadAlias); err != nil {
			return false, 0, ErrProjectionUnavailable
		}
		return false, 0, nil
	}
	fallbackComplete, fallbackCurrent, err := p.generationState(ctx, fallbackVersion, true)
	if err != nil {
		return false, 0, err
	}
	if !fallbackComplete || !fallbackCurrent {
		return false, 0, ErrProjectionUnavailable
	}
	fallbackIndex := fmt.Sprintf("%s%d", ragIndexPrefix, fallbackVersion)
	if err := manager.SwapAlias(ctx, ragReadAlias, fallbackIndex); err != nil {
		resolved, resolveErr := manager.AliasTarget(ctx, ragReadAlias)
		if resolveErr != nil || resolved != fallbackIndex {
			return false, 0, ErrProjectionUnavailable
		}
	}
	return false, 0, nil
}

func (p *Projection) generationState(ctx context.Context, version int, requireCurrent bool) (complete bool, allCurrent bool, err error) {
	var publishedIDs []int64
	if err := p.db.WithContext(ctx).Table("content_items").Where("status = ? AND deleted_at IS NULL", "published").
		Order("id ASC").Pluck("id", &publishedIDs).Error; err != nil {
		return false, false, fmt.Errorf("list published rebuild contents: %w", err)
	}
	var statuses []model.IndexProjectionStatus
	if err := p.db.WithContext(ctx).Where("index_version = ?", version).Order("content_id ASC").Find(&statuses).Error; err != nil {
		return false, false, fmt.Errorf("load alias projection generation: %w", err)
	}
	if len(statuses) != len(publishedIDs) {
		return false, false, nil
	}
	allCurrent = true
	for i, status := range statuses {
		validState := status.State == "staging" || status.State == "ready"
		if requireCurrent {
			validState = status.State == "ready" && status.IsCurrent
		}
		if status.ContentID != publishedIDs[i] || !validState ||
			status.ChunkingVersion != p.configuredChunkingVersion() || status.EmbeddingModel != p.config.EmbeddingModel {
			return false, false, nil
		}
		var chunkCount, embeddingCount int64
		if err := p.db.WithContext(ctx).Model(&model.RagChunk{}).
			Where("content_id = ? AND index_version = ?", status.ContentID, version).Count(&chunkCount).Error; err != nil {
			return false, false, fmt.Errorf("count alias projection chunks: %w", err)
		}
		if err := p.db.WithContext(ctx).Table("chunk_embeddings AS ce").
			Joins("JOIN rag_chunks AS rc ON rc.id = ce.chunk_id").
			Where("rc.content_id = ? AND rc.index_version = ? AND ce.embedding_model = ?", status.ContentID, version, p.config.EmbeddingModel).
			Count(&embeddingCount).Error; err != nil {
			return false, false, fmt.Errorf("count alias projection embeddings: %w", err)
		}
		if embeddingCount != chunkCount {
			return false, false, nil
		}
		allCurrent = allCurrent && status.IsCurrent && status.State == "ready"
	}
	return true, allCurrent, nil
}

func (p *Projection) currentProjectionVersion(ctx context.Context) (int, error) {
	var version int
	if err := p.db.WithContext(ctx).Table("index_projection_status").
		Select("COALESCE(MAX(index_version), 0)").Where("is_current = ?", true).Scan(&version).Error; err != nil {
		return 0, fmt.Errorf("read current projection generation: %w", err)
	}
	return version, nil
}

func (p *Projection) parseIndexVersion(index string) (int, bool) {
	if !strings.HasPrefix(index, ragIndexPrefix) {
		return 0, false
	}
	version, err := strconv.Atoi(strings.TrimPrefix(index, ragIndexPrefix))
	return version, err == nil && version > 0 && index == fmt.Sprintf("%s%d", ragIndexPrefix, version)
}

func (p *Projection) resolveAliasTarget(ctx context.Context, manager SearchIndexManager) (string, bool, error) {
	target, err := manager.AliasTarget(ctx, ragReadAlias)
	if errors.Is(err, repository.ErrOpenSearchAliasNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, ErrProjectionUnavailable
	}
	if target == "" {
		return "", false, nil
	}
	return target, true, nil
}

func (p *Projection) configuredChunkingVersion() int { return p.chunker.config.ChunkingVersion }

func (p *Projection) failRebuild(ctx context.Context, indexVersion int, summary string) error {
	if err := p.db.WithContext(ctx).Model(&model.IndexProjectionStatus{}).
		Where("index_version = ? AND is_current = ?", indexVersion, false).
		Updates(map[string]any{"state": "failed", "error_summary": summary}).Error; err != nil {
		return fmt.Errorf("%w: persist rebuild failure state", ErrProjectionUnavailable)
	}
	return ErrProjectionUnavailable
}

func sameProjectedChunks(current, expected []model.RagChunk, indexVersion int) bool {
	if len(current) != len(expected) {
		return false
	}
	for i := range current {
		left, right := current[i], expected[i]
		if left.IndexVersion != indexVersion || left.ContentVersion != right.ContentVersion ||
			left.ChunkIndex != right.ChunkIndex || left.ChunkKey != right.ChunkKey ||
			left.Heading != right.Heading || left.Text != right.Text ||
			left.SourceStart != right.SourceStart || left.SourceEnd != right.SourceEnd ||
			left.Zone != right.Zone || left.ContentType != right.ContentType ||
			!equalStringPointers(left.Category, right.Category) || !equalInt64Pointers(left.IP, right.IP) ||
			!equalStrings(left.Tags, right.Tags) {
			return false
		}
	}
	return true
}

func equalStringPointers(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func equalInt64Pointers(left, right *int64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

type projectionContent struct {
	ID          int64
	Title       string
	Zone        string
	ContentType string
	Category    *string
	IP          *int64 `gorm:"column:ip_id"`
	Status      string
	UpdatedAt   time.Time
}

func chunkedVersion(chunks []Chunk) int {
	if len(chunks) == 0 {
		return 1
	}
	return chunks[0].ChunkingVersion
}

func (p *Projection) storeEmbeddings(ctx context.Context, chunks []model.RagChunk, vectors [][]float32) error {
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, chunk := range chunks {
			if err := tx.Exec(`INSERT INTO chunk_embeddings (chunk_id, embedding, embedding_model, embedded_at)
				VALUES (?, ?, ?, ?)
				ON CONFLICT (chunk_id, embedding_model)
				DO UPDATE SET embedding = EXCLUDED.embedding, embedded_at = EXCLUDED.embedded_at`,
				chunk.ID, formatVector(vectors[i]), p.config.EmbeddingModel, time.Now().UTC()).Error; err != nil {
				return fmt.Errorf("store chunk embedding: %w", err)
			}
		}
		return nil
	})
}

func (p *Projection) replaceCurrentProjection(ctx context.Context, generation repository.RagGeneration, chunks []model.RagChunk, vectors [][]float32) error {
	if len(chunks) != len(vectors) {
		return ErrProjectionUnavailable
	}
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var status model.IndexProjectionStatus
		if err := tx.Where("content_id = ? AND index_version = ? AND chunking_version = ? AND embedding_model = ? AND state = ? AND is_current = ?",
			generation.ContentID, generation.IndexVersion, generation.ChunkingVersion, generation.EmbeddingModel, "ready", true).
			Take(&status).Error; err != nil {
			return err
		}
		if err := tx.Where("content_id = ? AND index_version = ?", generation.ContentID, generation.IndexVersion).
			Delete(&model.RagChunk{}).Error; err != nil {
			return err
		}
		replacement := append([]model.RagChunk(nil), chunks...)
		for i := range replacement {
			replacement[i].ID = 0
			replacement[i].ContentID = generation.ContentID
			replacement[i].IndexVersion = generation.IndexVersion
			replacement[i].ChunkingVersion = generation.ChunkingVersion
		}
		if len(replacement) > 0 {
			if err := tx.Create(&replacement).Error; err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		for i, chunk := range replacement {
			if err := tx.Exec(`INSERT INTO chunk_embeddings (chunk_id, embedding, embedding_model, embedded_at)
				VALUES (?, ?, ?, ?)`, chunk.ID, formatVector(vectors[i]), generation.EmbeddingModel, now).Error; err != nil {
				return err
			}
		}
		return tx.Model(&status).Updates(map[string]any{
			"last_indexed_at": nil,
			"error_summary":   "",
		}).Error
	})
}

func (p *Projection) validateVectorDimensions(vectors [][]float32) error {
	if len(vectors) == 0 {
		return nil
	}
	expected := p.config.EmbeddingDimensions
	if expected <= 0 {
		expected = len(vectors[0])
	}
	if expected <= 0 {
		return errors.New("embed content projection: vector dimension must be positive")
	}
	for _, vector := range vectors {
		if len(vector) != expected {
			return fmt.Errorf("embed content projection: vector dimension must be %d", expected)
		}
	}
	return nil
}

func (p *Projection) markCurrentIndexed(ctx context.Context, generation repository.RagGeneration) error {
	result := p.db.WithContext(ctx).Model(&model.IndexProjectionStatus{}).
		Where("content_id = ? AND index_version = ? AND chunking_version = ? AND embedding_model = ? AND state = ? AND is_current = ?",
			generation.ContentID, generation.IndexVersion, generation.ChunkingVersion, generation.EmbeddingModel, "ready", true).
		Updates(map[string]any{"last_indexed_at": time.Now().UTC(), "error_summary": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return repository.ErrRagGenerationNotFound
	}
	return nil
}

func formatVector(vector []float32) string {
	values := make([]string, len(vector))
	for i, value := range vector {
		values[i] = fmt.Sprintf("%g", value)
	}
	return "[" + strings.Join(values, ",") + "]"
}
