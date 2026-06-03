package model

import "time"

type AIReviewRecord struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetType  string    `gorm:"size:20;not null" json:"target_type"`
	TargetID    int64     `gorm:"not null" json:"target_id"`
	Provider    string    `gorm:"size:50;not null;default:aliyun" json:"provider"`
	Result      string    `gorm:"size:20;not null" json:"result"`
	RawResponse []byte    `gorm:"type:jsonb" json:"raw_response,omitempty"`
	ScannedAt   time.Time `gorm:"autoCreateTime" json:"scanned_at"`
}

func (AIReviewRecord) TableName() string { return "ai_review_records" }

type Report struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ReporterID  int64     `gorm:"not null;index" json:"reporter_id"`
	Reporter    User      `gorm:"foreignKey:ReporterID" json:"-"`
	TargetType  string    `gorm:"size:20;not null" json:"target_type"`
	TargetID    int64     `gorm:"not null" json:"target_id"`
	Reason      string    `gorm:"size:100;not null" json:"reason"`
	Detail      string    `gorm:"type:text" json:"detail,omitempty"`
	Status      string    `gorm:"size:20;not null;default:pending;index" json:"status"`
	ActionTaken string    `gorm:"type:text" json:"action_taken,omitempty"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Report) TableName() string { return "reports" }
