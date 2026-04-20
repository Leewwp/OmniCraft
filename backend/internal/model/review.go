package model

import "time"

type AIReviewRecord struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	TargetType  string    `gorm:"size:20;not null"`
	TargetID    int64     `gorm:"not null"`
	Provider    string    `gorm:"size:50;not null;default:aliyun"`
	Result      string    `gorm:"size:20;not null"`
	RawResponse []byte    `gorm:"type:jsonb"`
	ScannedAt   time.Time `gorm:"autoCreateTime"`
}

func (AIReviewRecord) TableName() string { return "ai_review_records" }

type Report struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	ReporterID int64     `gorm:"not null;index"`
	Reporter   User      `gorm:"foreignKey:ReporterID"`
	TargetType string    `gorm:"size:20;not null"`
	TargetID   int64     `gorm:"not null"`
	Reason     string    `gorm:"size:100;not null"`
	Detail     string    `gorm:"type:text"`
	Status     string    `gorm:"size:20;not null;default:pending;index"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

func (Report) TableName() string { return "reports" }
