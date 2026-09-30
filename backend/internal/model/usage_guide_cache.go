package model

import "time"

// ContentUsageGuideCache is the #728 auto-generation cache: one row per
// (content, locale) holding the last COMPLETE auto-generated guide. It is
// deliberately separate from ContentUsageGuide (author-confirmed rows) so
// automatic results never mix into author data.
type ContentUsageGuideCache struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentItemID    int64     `gorm:"column:content_id;not null;uniqueIndex:uq_usage_guide_cache_content_locale,priority:1" json:"content_id"`
	Locale           string    `gorm:"size:8;not null;uniqueIndex:uq_usage_guide_cache_content_locale,priority:2" json:"locale"`
	PromptVersion    int       `gorm:"not null" json:"prompt_version"`
	InputFingerprint string    `gorm:"size:64;not null" json:"input_fingerprint"`
	ContentUpdatedAt time.Time `gorm:"not null" json:"content_updated_at"`
	GuideMarkdown    string    `gorm:"type:text;not null" json:"guide_markdown"`
	Source           string    `gorm:"size:16;not null;default:auto" json:"source"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ContentUsageGuideCache) TableName() string { return "content_usage_guide_cache" }

// UsageGuideCacheSourceAuto marks rows written by the automatic pipeline.
const UsageGuideCacheSourceAuto = "auto"
