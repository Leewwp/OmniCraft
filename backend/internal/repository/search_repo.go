package repository

import (
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

func (r *SearchRepository) SearchSuggestions(prefix string, limit int) ([]SearchSuggestion, error) {
	if limit <= 0 {
		limit = 10
	}
	var results []SearchSuggestion

	rows, err := r.db.Raw(`
		SELECT text, score FROM (
			SELECT name AS text, usage_count AS score FROM tags WHERE name ILIKE ? AND status = 'active'
			UNION ALL
			SELECT title AS text, view_count AS score FROM content_items WHERE title ILIKE ? AND status = 'published'
		) s
		ORDER BY score DESC
		LIMIT ?
	`, prefix+"%", prefix+"%", limit).Rows()
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
