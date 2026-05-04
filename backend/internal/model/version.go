package model

import "time"

type ContentVersion struct {
	ID              int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentItemID   int64           `gorm:"not null;index" json:"content_item_id"`
	ContentItem     ContentItem     `gorm:"foreignKey:ContentItemID" json:"-"`
	ParentVersionID *int64          `json:"parent_version_id,omitempty"`
	ParentVersion   *ContentVersion `gorm:"foreignKey:ParentVersionID" json:"-"`
	AuthorID        int64           `gorm:"not null" json:"author_id"`
	Author          User            `gorm:"foreignKey:AuthorID" json:"-"`
	VersionNumber   int             `gorm:"not null" json:"version_number"`
	StorageType     string          `gorm:"size:10;not null" json:"storage_type"`
	StorageKey      string          `gorm:"type:text" json:"storage_key,omitempty"`
	DiffSummary     string          `gorm:"type:text" json:"diff_summary,omitempty"`
	Status          string          `gorm:"size:20;not null;default:active" json:"status"`
	IsLatest        bool            `gorm:"not null;default:false" json:"is_latest"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

func (ContentVersion) TableName() string { return "content_versions" }

type PullRequest struct {
	ID                int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentItemID     int64           `gorm:"not null;index:idx_pr_content" json:"content_item_id"`
	ContentItem       ContentItem     `gorm:"foreignKey:ContentItemID" json:"-"`
	SubmitterID       int64           `gorm:"not null;index" json:"submitter_id"`
	Submitter         User            `gorm:"foreignKey:SubmitterID" json:"-"`
	BaseVersionID     int64           `gorm:"not null" json:"base_version_id"`
	BaseVersion       ContentVersion  `gorm:"foreignKey:BaseVersionID" json:"-"`
	ProposedVersionID *int64          `json:"proposed_version_id,omitempty"`
	ProposedVersion   *ContentVersion `gorm:"foreignKey:ProposedVersionID" json:"-"`
	Status            string          `gorm:"size:20;not null;default:open" json:"status"`
	Message           string          `gorm:"type:text" json:"message,omitempty"`
	RejectReason      string          `gorm:"type:text" json:"reject_reason,omitempty"`
	ResolvedAt        *time.Time      `json:"resolved_at,omitempty"`
	CreatedAt         time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

func (PullRequest) TableName() string { return "pull_requests" }

type ContentContributor struct {
	ContentItemID int64     `gorm:"primaryKey" json:"content_item_id"`
	UserID        int64     `gorm:"primaryKey" json:"user_id"`
	ContentItem   ContentItem `gorm:"foreignKey:ContentItemID" json:"-"`
	User          User      `gorm:"foreignKey:UserID" json:"-"`
	PRCount       int       `gorm:"not null;default:1" json:"pr_count"`
	FirstAt       time.Time `gorm:"autoCreateTime" json:"first_at"`
}

func (ContentContributor) TableName() string { return "content_contributors" }

type AuthorBlocklist struct {
	AuthorID  int64     `gorm:"primaryKey" json:"author_id"`
	BlockedID int64     `gorm:"primaryKey" json:"blocked_id"`
	Author    User      `gorm:"foreignKey:AuthorID" json:"-"`
	Blocked   User      `gorm:"foreignKey:BlockedID" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AuthorBlocklist) TableName() string { return "author_blocklist" }
