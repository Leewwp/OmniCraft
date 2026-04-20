package model

import (
	"time"

	"github.com/lib/pq"
)

type Tag struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Category   string    `gorm:"size:50;not null;default:''" json:"category"`
	UsageCount int       `gorm:"not null;default:0" json:"usage_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TagSuggestion struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentItemID int64     `gorm:"not null;index" json:"content_item_id"`
	UserID        int64     `gorm:"not null" json:"user_id"`
	Tag           string    `gorm:"size:100;not null" json:"tag"`
	Action        string    `gorm:"size:10;not null;check:action IN ('add','remove')" json:"action"`
	Status        string    `gorm:"size:20;not null;default:'pending'" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type TagGroup struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64          `gorm:"not null;index" json:"user_id"`
	Name      string         `gorm:"size:100;not null" json:"name"`
	Tags      pq.StringArray `gorm:"type:text[];not null;default:'{}'" json:"tags"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type SavedSearch struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"not null;index" json:"user_id"`
	Name      string    `gorm:"size:200;not null" json:"name"`
	Config    JSONMap   `gorm:"type:jsonb;not null;default:'{}'" json:"config"`
	CreatedAt time.Time `json:"created_at"`
}
