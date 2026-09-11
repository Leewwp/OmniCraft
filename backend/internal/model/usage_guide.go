package model

import "time"

// ContentUsageGuide is the content-level specifics layer of the usage-guide
// model (SP-16 #447 / spec D4): author-confirmed guidance persisted per
// content and locale. Requirements/Steps are JSON arrays of strings stored
// as jsonb text; Notes is free Markdown.
type ContentUsageGuide struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentItemID int64     `gorm:"column:content_id;not null;uniqueIndex:uq_content_usage_guides_content_locale,priority:1" json:"content_id"`
	Locale        string    `gorm:"size:10;not null;uniqueIndex:uq_content_usage_guides_content_locale,priority:2" json:"locale"`
	Requirements  string    `gorm:"type:jsonb;not null;default:[]" json:"requirements"`
	Steps         string    `gorm:"type:jsonb;not null;default:[]" json:"steps"`
	Notes         string    `gorm:"type:text;not null;default:''" json:"notes"`
	Source        string    `gorm:"size:20;not null;default:author" json:"source"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ContentUsageGuide) TableName() string { return "content_usage_guides" }

// Usage guide source vocabulary: author | llm_assisted.
const (
	UsageGuideSourceAuthor      = "author"
	UsageGuideSourceLLMAssisted = "llm_assisted"
)
