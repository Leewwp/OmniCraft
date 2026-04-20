package model

import "time"

type LLMConfig struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigName   string    `gorm:"size:100;not null" json:"config_name"`
	ProviderType string    `gorm:"size:50;not null" json:"provider_type"`
	APIBase      string    `gorm:"size:500" json:"api_base"`
	Model        string    `gorm:"size:100;not null" json:"model"`
	APIKeyEnc    string    `gorm:"type:text" json:"-"`
	IsActive     bool      `gorm:"not null;default:false" json:"is_active"`
	ExtraParams  JSONMap   `gorm:"type:jsonb;not null;default:'{}'" json:"extra_params"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
