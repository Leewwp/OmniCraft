package model

import "time"

type Collection struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64      `gorm:"not null;index" json:"user_id"`
	User        User       `gorm:"foreignKey:UserID" json:"-"`
	Title       string     `gorm:"size:200;not null" json:"title"`
	Description string     `gorm:"type:text;not null;default:''" json:"description"`
	Zone        string     `gorm:"size:10;not null;index" json:"zone"`
	IsDefault   bool       `gorm:"not null;default:false" json:"is_default"`
	IsPublic    bool       `gorm:"not null;default:false" json:"is_public"`
	SortOrder   int        `gorm:"not null;default:0" json:"sort_order"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Collection) TableName() string { return "collections" }

type CollectionItem struct {
	ID            int64       `gorm:"primaryKey;autoIncrement" json:"id"`
	CollectionID  int64       `gorm:"not null;uniqueIndex:idx_collection_items_unique;index" json:"collection_id"`
	Collection    Collection  `gorm:"foreignKey:CollectionID" json:"-"`
	ContentItemID int64       `gorm:"not null;uniqueIndex:idx_collection_items_unique;index" json:"content_item_id"`
	ContentItem   ContentItem `gorm:"foreignKey:ContentItemID" json:"content_item,omitempty"`
	Note          string      `gorm:"type:text;not null;default:''" json:"note"`
	AddedAt       time.Time   `gorm:"autoCreateTime" json:"added_at"`
}

func (CollectionItem) TableName() string { return "collection_items" }
