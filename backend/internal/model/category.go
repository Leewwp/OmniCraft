package model

import "time"

type Category struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Zone      string    `gorm:"size:20;not null" json:"zone"`
	Level     string    `gorm:"size:20;not null" json:"level"`
	ParentID  *int64    `json:"parent_id,omitempty"`
	NameI18n  JSONMap   `gorm:"type:jsonb;not null;default:'{}'" json:"name_i18n"`
	Slug      string    `gorm:"size:100;not null;uniqueIndex" json:"slug"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	IsActive  bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
