package model

import (
	"time"
)

type User struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Email           string    `gorm:"uniqueIndex;not null;size:255" json:"email"`
	PasswordHash    string    `gorm:"not null;size:255" json:"-"`
	Username        string    `gorm:"uniqueIndex;not null;size:64" json:"username"`
	AvatarURL       string    `gorm:"size:2048" json:"avatar_url"`
	Bio             string    `gorm:"type:text" json:"bio"`
	Reputation      int       `gorm:"not null;default:10" json:"reputation"`
	PreferredLocale string    `gorm:"not null;default:'zh-CN';size:10" json:"preferred_locale"`
	Role            string    `gorm:"not null;default:'user';size:20" json:"role"`
	IsBanned        bool      `gorm:"not null;default:false" json:"is_banned"`
	BanReason       string    `gorm:"type:text" json:"ban_reason,omitempty"`
	SupportInfo     []byte    `gorm:"type:jsonb;default:'{}'" json:"support_info,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

type JudgeQualification struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint       `gorm:"not null;index" json:"user_id"`
	ContentType string     `gorm:"not null;size:50" json:"content_type"`
	QualifiedAt time.Time  `gorm:"not null;default:now()" json:"qualified_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	IsActive    bool       `gorm:"not null;default:true" json:"is_active"`
}

func (JudgeQualification) TableName() string {
	return "judge_qualifications"
}

type ReputationLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Delta     int       `gorm:"not null" json:"delta"`
	Reason    string    `gorm:"not null;size:100" json:"reason"`
	RelatedID *uint     `json:"related_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (ReputationLog) TableName() string {
	return "reputation_logs"
}

type OAuthAccount struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Provider    string    `gorm:"not null;size:20" json:"provider"`
	ProviderUID string    `gorm:"not null;size:255" json:"provider_uid"`
	AccessToken string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

func (OAuthAccount) TableName() string {
	return "oauth_accounts"
}
