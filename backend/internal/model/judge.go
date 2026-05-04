package model

import "time"

type JudgeQuestion struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentType  string    `gorm:"size:50;not null" json:"content_type"`
	SourceCaseID *int64    `json:"source_case_id,omitempty"`
	QuestionData []byte    `gorm:"type:jsonb;not null" json:"question_data"`
	IsActive     bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedBy    string    `gorm:"size:20;not null;default:admin" json:"created_by"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (JudgeQuestion) TableName() string { return "judge_questions" }

type JudgeCase struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetType  string     `gorm:"size:20;not null" json:"target_type"`
	TargetID    int64      `gorm:"not null" json:"target_id"`
	Status      string     `gorm:"size:20;not null;default:open;index" json:"status"`
	VoteApprove int        `gorm:"not null;default:0" json:"vote_approve"`
	VoteReject  int        `gorm:"not null;default:0" json:"vote_reject"`
	MinVotes    int        `gorm:"not null;default:20" json:"min_votes"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (JudgeCase) TableName() string { return "judge_cases" }

type JudgeVote struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CaseID    int64     `gorm:"not null;uniqueIndex:idx_judge_votes_unique" json:"case_id"`
	Case      JudgeCase `gorm:"foreignKey:CaseID" json:"-"`
	JudgeID   int64     `gorm:"not null;uniqueIndex:idx_judge_votes_unique" json:"judge_id"`
	Judge     User      `gorm:"foreignKey:JudgeID" json:"-"`
	Vote      string    `gorm:"size:10;not null" json:"vote"`
	Reason    string    `gorm:"type:text" json:"reason,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (JudgeVote) TableName() string { return "judge_votes" }

type JudgeReasonVote struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ReasonOwnerVoteID int64     `gorm:"not null;uniqueIndex:idx_reason_votes_unique;index" json:"reason_owner_vote_id"`
	ReasonOwnerVote   JudgeVote `gorm:"foreignKey:ReasonOwnerVoteID" json:"-"`
	VoterID           int64     `gorm:"not null;uniqueIndex:idx_reason_votes_unique" json:"voter_id"`
	Voter             User      `gorm:"foreignKey:VoterID" json:"-"`
	VoteType          string    `gorm:"size:10;not null" json:"vote_type"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (JudgeReasonVote) TableName() string { return "judge_reason_votes" }

type JudgeExamRecord struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64     `gorm:"not null;index" json:"user_id"`
	User        User      `gorm:"foreignKey:UserID" json:"-"`
	ContentType string    `gorm:"size:50;not null" json:"content_type"`
	Score       int       `gorm:"not null" json:"score"`
	Total       int       `gorm:"not null" json:"total"`
	Passed      bool      `gorm:"not null" json:"passed"`
	TakenAt     time.Time `gorm:"autoCreateTime" json:"taken_at"`
}

func (JudgeExamRecord) TableName() string { return "judge_exam_records" }
