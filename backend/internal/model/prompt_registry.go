package model

import "time"

// Prompt registry persistence (SP-21 T5, map #549): versioned prompt
// templates plus production/staging label pointers. Rows are immutable —
// a new version is a new row; the label pointer is the only mutable thing.

type PromptRegistry struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	Name      string    `gorm:"size:100;not null;uniqueIndex:uq_prompt_registry_name_version,priority:1" json:"name"`
	Version   int       `gorm:"not null;uniqueIndex:uq_prompt_registry_name_version,priority:2" json:"version"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	// RequiredPlaceholders snapshots the slot's validation contract onto the
	// row so audits can reproduce what was checked at save time.
	RequiredPlaceholders JSONB  `gorm:"type:jsonb;not null;default:'[]'" json:"required_placeholders"`
	CreatedBy            *int64 `json:"created_by,omitempty"`
}

func (PromptRegistry) TableName() string { return "prompt_registry" }

// PromptLabel is a named pointer (production/staging) from a prompt name to
// a registry version; runtime resolution follows the production pointer.
type PromptLabel struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Name      string    `gorm:"size:100;not null;uniqueIndex:uq_prompt_labels_name_label,priority:1" json:"name"`
	Label     string    `gorm:"size:32;not null;uniqueIndex:uq_prompt_labels_name_label,priority:2" json:"label"`
	Version   int       `gorm:"not null" json:"version"`
}

func (PromptLabel) TableName() string { return "prompt_labels" }
