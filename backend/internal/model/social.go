package model

import "time"

type Discussion struct {
	ID            int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	IPID          *int64       `gorm:"index" json:"ip_id,omitempty"`
	IP            *IP          `gorm:"foreignKey:IPID" json:"ip,omitempty"`
	ContentItemID *int64       `json:"content_item_id,omitempty"`
	ContentItem   *ContentItem `gorm:"foreignKey:ContentItemID" json:"-"`
	AuthorID      int64        `gorm:"not null;index" json:"author_id"`
	Author        User         `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Title         string       `gorm:"size:500;not null" json:"title"`
	Body          string       `gorm:"type:text" json:"body,omitempty"`
	Status        string       `gorm:"size:20;not null;default:published" json:"status"`
	IsPinned      bool         `gorm:"not null;default:false" json:"is_pinned"`
	ViewCount     int64        `gorm:"not null;default:0" json:"view_count"`
	ReplyCount    int          `gorm:"not null;default:0" json:"reply_count"`
	LastActiveAt  time.Time    `gorm:"not null;default:NOW()" json:"last_active_at"`
	CreatedAt     time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Discussion) TableName() string { return "discussions" }

type Comment struct {
	ID            int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentItemID *int64       `gorm:"index" json:"content_item_id,omitempty"`
	ContentItem   *ContentItem `gorm:"foreignKey:ContentItemID" json:"-"`
	DiscussionID  *int64       `gorm:"index" json:"discussion_id,omitempty"`
	Discussion    *Discussion  `gorm:"foreignKey:DiscussionID" json:"-"`
	ParentID      *int64       `json:"parent_id,omitempty"`
	Parent        *Comment     `gorm:"foreignKey:ParentID" json:"-"`
	AuthorID      int64        `gorm:"not null;index" json:"author_id"`
	Author        User         `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	TargetType    string       `gorm:"size:20;index" json:"target_type,omitempty"`
	TargetID      int64        `gorm:"index" json:"target_id"`
	Content       string       `gorm:"type:text" json:"content,omitempty"`
	Body          string       `gorm:"type:text;not null" json:"body"`
	Status        string       `gorm:"size:20;not null;default:published" json:"status"`
	LikeCount     int          `gorm:"not null;default:0" json:"like_count"`
	/* T47（FIX-29c）：赞/踩展示计数由仓储层从 reactions 聚合回填——
	   comments.like_count 冗余列从未被维护，禁止直接读列。 */
	DislikeCount int       `gorm:"-" json:"dislike_count"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Comment) TableName() string { return "comments" }

type Reaction struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"not null;uniqueIndex:idx_reactions_unique" json:"user_id"`
	User       User      `gorm:"foreignKey:UserID" json:"-"`
	TargetType string    `gorm:"size:20;not null;uniqueIndex:idx_reactions_unique" json:"target_type"`
	TargetID   int64     `gorm:"not null;uniqueIndex:idx_reactions_unique" json:"target_id"`
	Reaction   string    `gorm:"size:10;not null" json:"reaction"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Reaction) TableName() string { return "reactions" }

type BrowseHistory struct {
	ID            int64       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        int64       `gorm:"not null;uniqueIndex:idx_browse_unique;index:idx_browse_history_user" json:"user_id"`
	ContentItemID int64       `gorm:"not null;uniqueIndex:idx_browse_unique" json:"content_item_id"`
	User          User        `gorm:"foreignKey:UserID" json:"-"`
	ContentItem   ContentItem `gorm:"foreignKey:ContentItemID" json:"-"`
	ViewedAt      time.Time   `gorm:"autoCreateTime" json:"viewed_at"`
}

func (BrowseHistory) TableName() string { return "browse_history" }

type Follow struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FollowerID int64     `gorm:"not null;uniqueIndex:idx_follows_unique;index" json:"follower_id"`
	Follower   User      `gorm:"foreignKey:FollowerID" json:"-"`
	TargetType string    `gorm:"size:20;not null;uniqueIndex:idx_follows_unique" json:"target_type"`
	TargetID   int64     `gorm:"not null;uniqueIndex:idx_follows_unique" json:"target_id"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Follow) TableName() string { return "follows" }

type Appeal struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         int64      `gorm:"not null;index" json:"user_id"`
	User           User       `gorm:"foreignKey:UserID" json:"-"`
	TargetType     string     `gorm:"size:20;not null" json:"target_type"`
	TargetID       int64      `gorm:"not null" json:"target_id"`
	Reason         string     `gorm:"type:text;not null" json:"reason"`
	Status         string     `gorm:"size:20;not null;default:pending;index" json:"status"`
	AdminResponse  string     `gorm:"type:text" json:"admin_response,omitempty"`
	ResolvedBy     *int64     `json:"resolved_by,omitempty"`
	ResolvedByUser *User      `gorm:"foreignKey:ResolvedBy" json:"-"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (Appeal) TableName() string { return "appeals" }
