package model

import "time"

type ContentItem struct {
	ID             int64      `gorm:"primaryKey;autoIncrement"`
	Title          string     `gorm:"size:500;not null"`
	AuthorID       int64      `gorm:"not null;index"`
	Author         User       `gorm:"foreignKey:AuthorID"`
	Zone           string     `gorm:"size:10;not null"`
	IPID           *int64     `gorm:"index"`
	IP             *IP        `gorm:"foreignKey:IPID"`
	Category       string     `gorm:"size:50"`
	ContentType    string     `gorm:"size:20;not null"`
	CoverImageURL  string     `gorm:"type:text"`
	Status         string     `gorm:"size:20;not null;default:pending"`
	ViewCount      int64      `gorm:"not null;default:0"`
	LikeCount      int        `gorm:"not null;default:0"`
	DislikeCount   int        `gorm:"not null;default:0"`
	IsPublic       bool       `gorm:"not null;default:true"`
	AllowCopy      bool       `gorm:"not null;default:true"`
	AgentEnabled   bool       `gorm:"not null;default:false"`
	IsPaid         bool       `gorm:"not null;default:false"`
	Price          float64    `gorm:"type:numeric(10,2);default:0"`
	CreatedAt      time.Time  `gorm:"autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime"`
}

func (ContentItem) TableName() string { return "content_items" }

type ContentAttachment struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`
	ContentItemID int64        `gorm:"not null;index"`
	ContentItem   ContentItem  `gorm:"foreignKey:ContentItemID"`
	FileType      string       `gorm:"size:30;not null"`
	OSSKey        string       `gorm:"type:text;not null"`
	FileSize      *int64
	MimeType      string `gorm:"size:100"`
	DurationSec   *int
	Width         *int
	Height        *int
	IsPrimary     bool      `gorm:"default:true"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

func (ContentAttachment) TableName() string { return "content_attachments" }

type ContentTag struct {
	ContentItemID int64  `gorm:"primaryKey"`
	Tag           string `gorm:"size:50;primaryKey"`
}

func (ContentTag) TableName() string { return "content_tags" }
