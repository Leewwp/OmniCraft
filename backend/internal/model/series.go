package model

import "time"

type ContentSeries struct {
	ID             int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	Title          string       `gorm:"size:200;not null" json:"title"`
	Description    string       `gorm:"type:text;not null;default:''" json:"description"`
	CoverContentID *int64       `gorm:"index" json:"cover_content_id,omitempty"`
	CoverContent   *ContentItem `gorm:"foreignKey:CoverContentID" json:"-"`
	OwnerID        int64        `gorm:"not null;index" json:"owner_id"`
	Owner          User         `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Zone           string       `gorm:"size:10;not null" json:"zone"`
	CreatedAt      time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ContentSeries) TableName() string { return "content_series" }

type ContentSeriesItem struct {
	ID            int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	SeriesID      int64         `gorm:"not null;uniqueIndex:idx_content_series_items_unique;index" json:"series_id"`
	Series        ContentSeries `gorm:"foreignKey:SeriesID" json:"-"`
	ContentItemID int64         `gorm:"not null;uniqueIndex:idx_content_series_items_unique;index" json:"content_item_id"`
	ContentItem   ContentItem   `gorm:"foreignKey:ContentItemID" json:"content,omitempty"`
	SortOrder     int           `gorm:"not null;default:0" json:"sort_order"`
	AddedAt       time.Time     `gorm:"autoCreateTime" json:"added_at"`
}

func (ContentSeriesItem) TableName() string { return "content_series_items" }
