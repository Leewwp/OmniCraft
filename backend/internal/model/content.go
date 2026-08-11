package model

import "time"

type ContentItem struct {
	ID               int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	Title            string       `gorm:"size:500;not null" json:"title"`
	Description      string       `gorm:"type:text" json:"description,omitempty"`
	AuthorID         int64        `gorm:"not null;index" json:"author_id"`
	Author           User         `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Zone             string       `gorm:"size:10;not null" json:"zone"`
	IPID             *int64       `gorm:"index" json:"ip_id,omitempty"`
	IP               *IP          `gorm:"foreignKey:IPID" json:"ip,omitempty"`
	SourceOriginalID *int64       `gorm:"index" json:"source_original_id,omitempty"`
	SourceOriginal   *ContentItem `gorm:"foreignKey:SourceOriginalID" json:"source_original,omitempty"`
	SourceFanworkID  *int64       `gorm:"index" json:"source_fanwork_id,omitempty"`
	SourceFanwork    *ContentItem `gorm:"foreignKey:SourceFanworkID" json:"source_fanwork,omitempty"`
	Category         string       `gorm:"size:50" json:"category,omitempty"`
	ContentType      string       `gorm:"size:20;not null" json:"content_type"`
	CoverImageURL    string       `gorm:"type:text" json:"cover_image_url,omitempty"`
	CoverWidth       *int         `gorm:"column:cover_width" json:"cover_width,omitempty"`
	CoverHeight      *int         `gorm:"column:cover_height" json:"cover_height,omitempty"`
	Status           string       `gorm:"size:20;not null;default:pending" json:"status"`
	ViewCount        int64        `gorm:"not null;default:0" json:"view_count"`
	LikeCount        int          `gorm:"not null;default:0" json:"like_count"`
	DislikeCount     int          `gorm:"not null;default:0" json:"dislike_count"`
	IsPublic         bool         `gorm:"not null;default:true" json:"is_public"`
	AllowCopy        bool         `gorm:"not null;default:true" json:"allow_copy"`
	AgentEnabled     bool         `gorm:"not null;default:false" json:"agent_enabled"`
	IsPaid           bool         `gorm:"not null;default:false" json:"is_paid"`
	Price            float64      `gorm:"type:numeric(10,2);default:0" json:"price"`
	DownloadCount    int          `gorm:"not null;default:0" json:"download_count"`
	DeletedAt        *time.Time   `gorm:"index" json:"deleted_at,omitempty"`
	CreatedAt        time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ContentItem) TableName() string { return "content_items" }

type ContentAttachment struct {
	ID            int64       `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentItemID int64       `gorm:"not null;index" json:"content_item_id"`
	ContentItem   ContentItem `gorm:"foreignKey:ContentItemID" json:"-"`
	FileType      string      `gorm:"size:30;not null" json:"file_type"`
	OSSKey        string      `gorm:"type:text;not null" json:"oss_key"`
	FileSize      *int64      `json:"file_size,omitempty"`
	MimeType      string      `gorm:"size:100" json:"mime_type,omitempty"`
	DurationSec   *int        `json:"duration_sec,omitempty"`
	Width         *int        `json:"width,omitempty"`
	Height        *int        `json:"height,omitempty"`
	SortOrder     *int        `gorm:"column:sort_order" json:"sort_order,omitempty"`
	// IsPrimary marks the single derived cover entry for media content (image =
	// sort_order 0 item; video = none, the poster is the cover). Pointer so new
	// media rows can explicitly persist false; legacy non-media rows keep the
	// column default true.
	IsPrimary *bool     `gorm:"default:true" json:"is_primary"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ContentAttachment) TableName() string { return "content_attachments" }

type ContentTag struct {
	ContentItemID int64  `gorm:"primaryKey" json:"content_item_id"`
	Tag           string `gorm:"size:50;primaryKey" json:"tag"`
}

func (ContentTag) TableName() string { return "content_tags" }
