package repository

import (
	"fmt"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type SearchRepository struct {
	db *gorm.DB
}

func NewSearchRepository(db *gorm.DB) *SearchRepository {
	return &SearchRepository{db: db}
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

func (r *SearchRepository) SearchSuggestions(prefix string, limit int) ([]SearchSuggestion, error) {
	if limit <= 0 {
		limit = 10
	}
	var results []SearchSuggestion

	rows, err := r.db.Raw(`
		SELECT text, score FROM (
			SELECT name AS text, usage_count AS score FROM tags WHERE name ILIKE ? AND status = 'active'
			UNION ALL
			SELECT ci.title AS text, ci.view_count AS score
			FROM content_items ci
			WHERE ci.title ILIKE ? AND ci.status = 'published' AND ci.deleted_at IS NULL
				AND ci.author_id NOT IN (SELECT id FROM users WHERE is_banned = true OR deleted_at IS NOT NULL)
				AND (ci.ip_id IS NULL OR ci.ip_id NOT IN (SELECT id FROM ips WHERE status = ?))
				AND ci.is_public = true
		) s
		ORDER BY score DESC
		LIMIT ?
	`, prefix+"%", prefix+"%", "banned", limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s SearchSuggestion
		if err := rows.Scan(&s.Text, &s.Score); err != nil {
			continue
		}
		results = append(results, s)
	}
	return results, nil
}

func (r *SearchRepository) SearchContents(query string, zone, category, contentType string, tagFilters []string, sort string, page, pageSize int, viewerID int64) ([]ContentSearchResult, int64, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	if query != "" {
		return r.searchContentsWithQuery(query, zone, category, contentType, tagFilters, page, pageSize, offset, viewerID)
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

func (r *SearchRepository) searchContentsWithQuery(query, zone, category, contentType string, tagFilters []string, page, pageSize, offset int, viewerID int64) ([]ContentSearchResult, int64, error) {
	tsQuery := toTSQuery(query)
	ilikePattern := "%" + query + "%"

	visibilityClause, visibilityArgs := ContentVisibilitySQL(viewerID)

	filterClause := ""
	args := []interface{}{}

	if zone != "" {
		filterClause += " AND content_items.zone = ?"
		args = append(args, zone)
	}
	if category != "" {
		filterClause += " AND content_items.category = ?"
		args = append(args, category)
	}
	if contentType != "" {
		filterClause += " AND content_items.content_type = ?"
		args = append(args, contentType)
	}
	if len(tagFilters) > 0 {
		filterClause += " AND EXISTS (SELECT 1 FROM content_tags ct WHERE ct.content_item_id = content_items.id AND ct.tag IN (?))"
		args = append(args, tagFilters)
	}

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM content_items
		WHERE %s%s
		AND (
			content_items.search_vector @@ to_tsquery('simple', ?)
			OR content_items.title ILIKE ?
			OR EXISTS (SELECT 1 FROM content_tags ct2 WHERE ct2.content_item_id = content_items.id AND ct2.tag ILIKE ?)
		)`, visibilityClause, filterClause)

	countArgs := append([]interface{}{}, visibilityArgs...)
	countArgs = append(countArgs, args...)
	countArgs = append(countArgs, tsQuery, ilikePattern, ilikePattern)

	var total int64
	if err := r.db.Raw(countSQL, countArgs...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	dataArgs := []interface{}{tsQuery, query}
	dataArgs = append(dataArgs, visibilityArgs...)
	dataArgs = append(dataArgs, args...)
	dataArgs = append(dataArgs, tsQuery, ilikePattern, ilikePattern, pageSize, offset)

	dataSQL := fmt.Sprintf(`SELECT content_items.*,
		COALESCE(ts_rank_cd(content_items.search_vector, to_tsquery('simple', ?)), 0) AS score,
		ts_headline('simple', COALESCE(content_items.title, '') || ' ' || COALESCE(content_items.description, ''),
			phraseto_tsquery('simple', ?), 'MaxWords=35,MinWords=10,ShortWord=3,MaxFragments=3,FragmentDelimiter=...') AS headline
		FROM content_items
		WHERE %s%s
		AND (
			content_items.search_vector @@ to_tsquery('simple', ?)
			OR content_items.title ILIKE ?
			OR EXISTS (SELECT 1 FROM content_tags ct2 WHERE ct2.content_item_id = content_items.id AND ct2.tag ILIKE ?)
		)
		ORDER BY score DESC
		LIMIT ? OFFSET ?`, visibilityClause, filterClause)

	var results []ContentSearchResult
	if err := r.db.Raw(dataSQL, dataArgs...).Scan(&results).Error; err != nil {
		return nil, 0, err
	}

	r.hydrateAuthors(results)

	return results, total, nil
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
	result := ""
	for i, w := range words {
		if i > 0 {
			result += " & "
		}
		result += w + ":*"
	}
	if result == "" {
		result = query + ":*"
	}
	return result
}

func splitAndNormalize(query string) []string {
	var words []string
	current := ""
	for _, r := range query {
		if r == ' ' || r == '&' || r == '|' || r == '!' || r == '(' || r == ')' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		words = append(words, current)
	}
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
