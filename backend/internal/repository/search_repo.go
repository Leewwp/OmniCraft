package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type SearchRepository struct {
	db *gorm.DB

	// ftsConfigOnce caches the text-search configuration selection (A-03):
	// pg_jieba's jiebacfg when the extension is installed (041/042 maintain
	// the stored search_vector with it), the built-in 'simple' otherwise.
	ftsConfigOnce sync.Once
	ftsConfig     string
}

func NewSearchRepository(db *gorm.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

// ftsQueryConfig returns the allowlisted text-search configuration used for
// every tsquery/ts_rank call: "jiebacfg" when pg_jieba is present, "simple"
// otherwise (mirroring the 041/042 trigger fallback). The value only ever
// comes from this allowlist, never from user input, so interpolating it into
// SQL is safe; detection failure degrades to the historical 'simple'.
func (r *SearchRepository) ftsQueryConfig() string {
	r.ftsConfigOnce.Do(func() {
		r.ftsConfig = "simple"
		if r.db == nil || r.db.Dialector.Name() == "sqlite" {
			return
		}
		var exists bool
		if err := r.db.Raw("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_jieba')").Scan(&exists).Error; err != nil {
			return
		}
		if exists {
			r.ftsConfig = "jiebacfg"
		}
	})
	return r.ftsConfig
}

type SearchSuggestion struct {
	Text  string `json:"text"`
	Score int64  `json:"score"`
}

type ContentSearchResult struct {
	model.ContentItem
	Score    float64 `json:"score"`
	Headline string  `json:"headline,omitempty"`
}

type RAGChunkSearchResult struct {
	model.RagChunk
	Title string  `gorm:"column:title"`
	Score float64 `gorm:"column:score"`
}

// SearchRAGChunks is the PostgreSQL keyword fallback for the hybrid retriever.
// It reads only the current ready chunk generation and repeats the complete
// viewer visibility predicate before ranking.
func (r *SearchRepository) SearchRAGChunks(ctx context.Context, query string, topK int, viewerID int64) ([]RAGChunkSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if topK <= 0 {
		topK = config.RAGDefaultBM25TopK
	}
	pattern := "%" + query + "%"
	cfg := r.ftsQueryConfig()
	var results []RAGChunkSearchResult
	// A-03: the lexical fallback consumes the 041-maintained
	// content_items.search_vector (pg_jieba when installed) for matching and
	// primary ranking instead of recomputing a runtime 'simple' vector; the
	// chunk-level rank orders chunks inside one content item. The config
	// token is allowlist-only (ftsQueryConfig), never user input.
	err := r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT rc.*, ci.title,
		       ts_rank_cd(ci.search_vector, plainto_tsquery('%s', ?)) AS content_rank,
		       ts_rank_cd(to_tsvector('%[1]s', concat_ws(' ', rc.heading, rc.text)), plainto_tsquery('%[1]s', ?)) AS score
		FROM rag_chunks AS rc
		JOIN content_items AS ci ON ci.id = rc.content_id
		JOIN index_projection_status AS ips
		  ON ips.content_id = rc.content_id
		 AND ips.index_version = rc.index_version
		 AND ips.is_current = TRUE
		 AND ips.state = 'ready'
		WHERE ci.status = 'published'
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
		  AND (
			  ci.search_vector @@ plainto_tsquery('%[1]s', ?)
			  OR ci.title ILIKE ?
			  OR rc.heading ILIKE ?
			  OR rc.text ILIKE ?
		  )
		ORDER BY content_rank DESC, score DESC, rc.chunk_key ASC
		LIMIT ?
	`, cfg), query, query, viewerID, query, pattern, pattern, pattern, topK).Scan(&results).Error
	return results, err
}

func (r *SearchRepository) SearchSuggestions(prefix string, limit int, viewerID int64) ([]SearchSuggestion, error) {
	if limit <= 0 {
		limit = 10
	}
	var results []SearchSuggestion
	// Contains matching (T21/FIX-39b): a prefix-only pattern cannot surface
	// Chinese substrings ("镜头" inside "我的镜头笔记") because users rarely
	// type from the first character. Cost note: '%x%' defeats btree indexes;
	// at the current content scale the sequential scan stays negligible and a
	// pg_trgm GIN index is the escape hatch if suggestions ever slow down.
	likePattern := "%" + prefix + "%"

	var sql string
	if r.db.Dialector.Name() == "sqlite" {
		sql = `
			SELECT text, score FROM (
				SELECT name AS text, usage_count AS score FROM tags WHERE name LIKE ?
				UNION ALL
				SELECT content_items.title AS text, content_items.view_count AS score
				FROM content_items
				WHERE content_items.title LIKE ?
				AND content_items.status = ?
				AND content_items.deleted_at IS NULL
				AND content_items.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL)
				AND (content_items.ip_id IS NULL OR content_items.ip_id NOT IN (SELECT id FROM ips WHERE status = ?))
				AND (content_items.is_public = ? OR content_items.author_id = ?)
			) s
			ORDER BY score DESC
			LIMIT ?
		`
	} else {
		sql = `
			SELECT text, score FROM (
				SELECT name AS text, usage_count AS score FROM tags WHERE name ILIKE ?
				UNION ALL
				SELECT content_items.title AS text, content_items.view_count AS score
				FROM content_items
				WHERE content_items.title ILIKE ?
				AND content_items.status = ?
				AND content_items.deleted_at IS NULL
				AND content_items.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL)
				AND (content_items.ip_id IS NULL OR content_items.ip_id NOT IN (SELECT id FROM ips WHERE status = ?))
				AND (content_items.is_public = ? OR content_items.author_id = ?)
			) s
			ORDER BY score DESC
			LIMIT ?
		`
	}

	if err := r.db.Raw(sql, likePattern, likePattern, "published", "banned", true, viewerID, limit).Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

// ResolveTrendingContents maps hot-rank members (content IDs) to their titles
// under the public visibility scope (FIX-06): unpublished, deleted, private or
// banned-author entries are dropped so they never reach the discovery surface.
func (r *SearchRepository) ResolveTrendingContents(ctx context.Context, ids []int64) (map[int64]string, error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	var rows []struct {
		ID    int64  `gorm:"column:id"`
		Title string `gorm:"column:title"`
	}
	if err := ApplyContentVisibilityScope(r.db.WithContext(ctx).Model(&model.ContentItem{}), 0).
		Select("content_items.id, content_items.title").
		Where("content_items.id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	titles := make(map[int64]string, len(rows))
	for _, row := range rows {
		titles[row.ID] = row.Title
	}
	return titles, nil
}

func (r *SearchRepository) SearchContents(query string, zone, category, contentType string, tagFilters []string, sort, timeRange string, page, pageSize int, viewerID int64) ([]ContentSearchResult, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	if query != "" {
		return r.searchContentsWithQuery(query, zone, category, contentType, tagFilters, timeRange, page, pageSize, offset, viewerID)
	}

	baseQuery := ApplyContentVisibilityScope(r.db.Model(&model.ContentItem{}), viewerID)

	if zone != "" {
		baseQuery = baseQuery.Where("content_items.zone = ?", zone)
	}
	if category != "" {
		baseQuery = baseQuery.Where("content_items.category = ?", category)
	}
	if contentType != "" {
		baseQuery = baseQuery.Where("content_items.content_type = ?", contentType)
	}
	if since, ok := searchTimeRangeSince(timeRange); ok {
		baseQuery = baseQuery.Where("content_items.created_at >= ?", since)
	}
	if len(tagFilters) > 0 {
		baseQuery = baseQuery.
			Joins("JOIN content_tags ct ON ct.content_item_id = content_items.id").
			Where("ct.tag IN ?", tagFilters).
			Group("content_items.id")
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var results []ContentSearchResult
	switch sort {
	case "most_views":
		baseQuery = baseQuery.Order("content_items.view_count DESC")
	case "best_rated":
		baseQuery = baseQuery.Order("(CASE WHEN content_items.like_count + content_items.dislike_count >= 5 THEN CAST(content_items.like_count AS float) / NULLIF(CAST(content_items.like_count AS float) + CAST(content_items.dislike_count AS float), 0) ELSE 0 END) DESC")
	case "newest":
		baseQuery = baseQuery.Order("content_items.created_at DESC")
	case "hot":
		baseQuery = baseQuery.Order("content_items.like_count DESC, content_items.view_count DESC")
	default:
		baseQuery = baseQuery.Order("content_items.created_at DESC")
	}

	if err := baseQuery.Preload("Author").Offset(offset).Limit(pageSize).Find(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *SearchRepository) searchContentsWithQuery(query, zone, category, contentType string, tagFilters []string, timeRange string, page, pageSize, offset int, viewerID int64) ([]ContentSearchResult, int64, error) {
	if r.db.Dialector.Name() == "sqlite" {
		return r.searchContentsWithQueryLike(query, zone, category, contentType, tagFilters, timeRange, pageSize, offset, viewerID)
	}

	tsQuery := toTSQuery(query)
	ilikePattern := "%" + query + "%"

	// Build base query with visibility scope and filters
	q := ApplyContentVisibilityScope(r.db.Model(&model.ContentItem{}), viewerID)

	if zone != "" {
		q = q.Where("content_items.zone = ?", zone)
	}
	if category != "" {
		q = q.Where("content_items.category = ?", category)
	}
	if contentType != "" {
		q = q.Where("content_items.content_type = ?", contentType)
	}
	if since, ok := searchTimeRangeSince(timeRange); ok {
		q = q.Where("content_items.created_at >= ?", since)
	}
	if len(tagFilters) > 0 {
		q = q.Where(`
			EXISTS (SELECT 1 FROM content_tags ct WHERE ct.content_item_id = content_items.id AND ct.tag IN ?)
		`, tagFilters)
	}

	// Full-text search conditions; the text-search config follows the
	// jiebacfg/simple selection of the 041-maintained search_vector (A-03).
	cfg := r.ftsQueryConfig()
	searchCond := fmt.Sprintf(`
		content_items.search_vector @@ to_tsquery('%s', ?)
		OR content_items.title ILIKE ?
		OR EXISTS (SELECT 1 FROM content_tags ct2 WHERE ct2.content_item_id = content_items.id AND ct2.tag ILIKE ?)
	`, cfg)

	// Count
	countQuery := q.Where(searchCond, tsQuery, ilikePattern, ilikePattern)
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Data query with score and headline
	var results []ContentSearchResult
	if err := q.Select(fmt.Sprintf(`
		content_items.*,
		COALESCE(ts_rank_cd(content_items.search_vector, to_tsquery('%s', ?)), 0) AS score,
		ts_headline('%[1]s', COALESCE(content_items.title, '') || ' ' || COALESCE(content_items.description, ''),
			phraseto_tsquery('%[1]s', ?), 'MaxWords=35,MinWords=10,ShortWord=3,MaxFragments=3,FragmentDelimiter=...') AS headline
	`, cfg), tsQuery, query).
		Where(searchCond, tsQuery, ilikePattern, ilikePattern).
		Order("score DESC").
		Offset(offset).Limit(pageSize).
		Find(&results).Error; err != nil {
		return nil, 0, err
	}

	r.hydrateAuthors(results)

	return results, total, nil
}

func (r *SearchRepository) searchContentsWithQueryLike(query, zone, category, contentType string, tagFilters []string, timeRange string, pageSize, offset int, viewerID int64) ([]ContentSearchResult, int64, error) {
	likePattern := "%" + query + "%"
	matchCond := `(LOWER(content_items.title) LIKE LOWER(?) OR EXISTS (SELECT 1 FROM content_tags ct2 WHERE ct2.content_item_id = content_items.id AND LOWER(ct2.tag) LIKE LOWER(?)))`

	// Build base query with visibility scope and filters
	q := ApplyContentVisibilityScope(r.db.Model(&model.ContentItem{}), viewerID)

	if zone != "" {
		q = q.Where("content_items.zone = ?", zone)
	}
	if category != "" {
		q = q.Where("content_items.category = ?", category)
	}
	if contentType != "" {
		q = q.Where("content_items.content_type = ?", contentType)
	}
	if since, ok := searchTimeRangeSince(timeRange); ok {
		q = q.Where("content_items.created_at >= ?", since)
	}
	if len(tagFilters) > 0 {
		q = q.Where(`
			EXISTS (SELECT 1 FROM content_tags ct WHERE ct.content_item_id = content_items.id AND ct.tag IN ?)
		`, tagFilters)
	}

	// Count
	countQuery := q.Where(matchCond, likePattern, likePattern)
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Data
	var results []ContentSearchResult
	if err := q.Select("content_items.*, 1 AS score, content_items.title AS headline").
		Where(matchCond, likePattern, likePattern).
		Order("content_items.created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&results).Error; err != nil {
		return nil, 0, err
	}

	r.hydrateAuthors(results)

	return results, total, nil
}

func searchTimeRangeSince(timeRange string) (time.Time, bool) {
	now := time.Now()
	switch timeRange {
	case "", "all":
		return time.Time{}, false
	case "day":
		return now.AddDate(0, 0, -1), true
	case "week":
		return now.AddDate(0, 0, -7), true
	case "month":
		return now.AddDate(0, -1, 0), true
	case "year":
		return now.AddDate(-1, 0, 0), true
	default:
		return time.Time{}, false
	}
}

func (r *SearchRepository) hydrateAuthors(results []ContentSearchResult) {
	authorIDs := make([]int64, 0, len(results))
	for _, res := range results {
		if res.AuthorID != 0 {
			authorIDs = append(authorIDs, res.AuthorID)
		}
	}
	if len(authorIDs) == 0 {
		return
	}
	var authors []model.User
	r.db.Where("id IN ?", authorIDs).Find(&authors)
	authorMap := make(map[int64]model.User, len(authors))
	for _, a := range authors {
		authorMap[a.ID] = a
	}
	for i := range results {
		if a, ok := authorMap[results[i].AuthorID]; ok {
			results[i].Author = a
		}
	}
}

func (r *SearchRepository) SearchIPs(query string, category string, page, pageSize int) ([]model.IP, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	baseQuery := r.db.Model(&model.IP{}).
		Where("ips.status = ?", "approved")

	if category != "" {
		baseQuery = baseQuery.Where("ips.category = ?", category)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var ips []model.IP
	if query != "" {
		tsQuery := toTSQuery(query)
		ilikePattern := "%" + query + "%"
		if err := r.db.Raw(`
			SELECT * FROM ips
			WHERE ips.status = 'approved'
			AND (? = '' OR ips.category = ?)
			AND (ips.search_vector @@ to_tsquery('simple', ?) OR ips.name ILIKE ?)
			ORDER BY ts_rank_cd(ips.search_vector, to_tsquery('simple', ?)) DESC
			LIMIT ? OFFSET ?
		`, category, category, tsQuery, ilikePattern, tsQuery, pageSize, offset).Scan(&ips).Error; err != nil {
			return nil, 0, err
		}
	} else {
		q := r.db.Where("ips.status = ?", "approved")
		if category != "" {
			q = q.Where("ips.category = ?", category)
		}
		if err := q.Order("ips.created_at DESC").Offset(offset).Limit(pageSize).Find(&ips).Error; err != nil {
			return nil, 0, err
		}
	}

	return ips, total, nil
}

func toTSQuery(query string) string {
	words := splitAndNormalize(query)
	if len(words) == 0 {
		// Punctuation-only input must not fall back to `query + ":*"`: any
		// tsquery boundary character in it is a syntax error for to_tsquery
		// (#319). The empty tsquery only disables the FTS arm (NOTICE, no
		// error, @@ is always false); the ILIKE fallbacks still apply.
		return ""
	}
	parts := make([]string, 0, len(words))
	for _, w := range words {
		parts = append(parts, w+":*")
	}
	return strings.Join(parts, " & ")
}

// splitAndNormalize tokenizes a user query into runs of letters/digits.
// Everything else — whitespace, tsquery operators (&|!()<->'), and any
// half/full-width punctuation such as : ：·《》 — is a token boundary. The
// text-search parser treats those characters as lexeme separators too, so
// splitting here is what keeps the hand-built to_tsquery input operator-safe
// (#319: a colon glued to a word produced "PIECE::**"-style syntax errors).
func splitAndNormalize(query string) []string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return words
}

func (r *SearchRepository) SearchUsers(query string, page, pageSize int) ([]UserSearchResult, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	baseQuery := r.db.Model(&model.User{}).
		Where("is_banned = ?", false).
		Where("deleted_at IS NULL")

	if query != "" {
		likeQuery := "%" + query + "%"
		baseQuery = baseQuery.Where("username ILIKE ?", likeQuery)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("search users count: %w", err)
	}

	var results []UserSearchResult
	if err := baseQuery.
		Select("id, username, COALESCE(avatar_url, '') as avatar_url, reputation, role").
		Order("username ASC").
		Offset(offset).Limit(pageSize).
		Find(&results).Error; err != nil {
		return nil, 0, fmt.Errorf("search users: %w", err)
	}

	return results, total, nil
}

func (r *SearchRepository) ListReports(status, targetType string, page, pageSize int) ([]model.Report, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	baseQuery := r.db.Model(&model.Report{})
	if status != "" {
		baseQuery = baseQuery.Where("status = ?", status)
	}
	if targetType != "" {
		baseQuery = baseQuery.Where("target_type = ?", targetType)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var reports []model.Report
	if err := baseQuery.Preload("Reporter").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&reports).Error; err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

// ListReportsByReporter returns a reporter's own reports (FIX-28a): the
// reporter_id filter is mandatory, so the me-endpoint cannot leak other
// users' reports.
func (r *SearchRepository) ListReportsByReporter(reporterID int64, page, pageSize int) ([]model.Report, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	baseQuery := r.db.Model(&model.Report{}).Where("reporter_id = ?", reporterID)

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var reports []model.Report
	if err := baseQuery.
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&reports).Error; err != nil {
		return nil, 0, err
	}

	return reports, total, nil
}

func (r *SearchRepository) UpdateReportStatus(id int64, status, actionTaken string) error {
	return r.db.Model(&model.Report{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"action_taken": actionTaken,
	}).Error
}

type ReportStats struct {
	TotalCount     int64 `json:"total_count"`
	PendingCount   int64 `json:"pending_count"`
	ResolvedCount  int64 `json:"resolved_count"`
	ContentReports int64 `json:"content_reports"`
	CommentReports int64 `json:"comment_reports"`
}

func (r *SearchRepository) GetReportStats() (*ReportStats, error) {
	var stats ReportStats
	if err := r.db.Model(&model.Report{}).Count(&stats.TotalCount).Error; err != nil {
		return nil, err
	}
	r.db.Model(&model.Report{}).Where("status = ?", "pending").Count(&stats.PendingCount)
	r.db.Model(&model.Report{}).Where("status = ?", "resolved").Count(&stats.ResolvedCount)
	r.db.Model(&model.Report{}).Where("target_type = ?", "content").Count(&stats.ContentReports)
	r.db.Model(&model.Report{}).Where("target_type = ?", "comment").Count(&stats.CommentReports)
	return &stats, nil
}

type UserSearchResult struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	AvatarURL  string `json:"avatar_url,omitempty"`
	Reputation int    `json:"reputation"`
	Role       string `json:"role"`
}
