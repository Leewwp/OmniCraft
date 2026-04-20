package model

import "time"

type JudgeQuestion struct {
	ID           int64     `gorm:"primaryKey;autoIncrement"`
	ContentType  string    `gorm:"size:50;not null"`
	SourceCaseID *int64
	QuestionData []byte    `gorm:"type:jsonb;not null"`
	IsActive     bool      `gorm:"not null;default:true"`
	CreatedBy    string    `gorm:"size:20;not null;default:admin"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (JudgeQuestion) TableName() string { return "judge_questions" }

type JudgeCase struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	TargetType  string     `gorm:"size:20;not null"`
	TargetID    int64      `gorm:"not null"`
	Status      string     `gorm:"size:20;not null;default:open;index"`
	VoteApprove int        `gorm:"not null;default:0"`
	VoteReject  int        `gorm:"not null;default:0"`
	MinVotes    int        `gorm:"not null;default:20"`
	ClosedAt    *time.Time
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
}

func (JudgeCase) TableName() string { return "judge_cases" }

type JudgeVote struct {
	ID       int64     `gorm:"primaryKey;autoIncrement"`
	CaseID   int64     `gorm:"not null;uniqueIndex:idx_judge_votes_unique"`
	Case     JudgeCase `gorm:"foreignKey:CaseID"`
	JudgeID  int64     `gorm:"not null;uniqueIndex:idx_judge_votes_unique"`
	Judge    User      `gorm:"foreignKey:JudgeID"`
	Vote     string    `gorm:"size:10;not null"`
	Reason   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (JudgeVote) TableName() string { return "judge_votes" }

type JudgeReasonVote struct {
	ID                  int64     `gorm:"primaryKey;autoIncrement"`
	ReasonOwnerVoteID   int64     `gorm:"not null;uniqueIndex:idx_reason_votes_unique;index"`
	ReasonOwnerVote     JudgeVote `gorm:"foreignKey:ReasonOwnerVoteID"`
	VoterID             int64     `gorm:"not null;uniqueIndex:idx_reason_votes_unique"`
	Voter               User      `gorm:"foreignKey:VoterID"`
	VoteType            string    `gorm:"size:10;not null"`
	CreatedAt           time.Time `gorm:"autoCreateTime"`
}

func (JudgeReasonVote) TableName() string { return "judge_reason_votes" }

type JudgeExamRecord struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	UserID      int64     `gorm:"not null;index"`
	User        User      `gorm:"foreignKey:UserID"`
	ContentType string    `gorm:"size:50;not null"`
	Score       int       `gorm:"not null"`
	Total       int       `gorm:"not null"`
	Passed      bool      `gorm:"not null"`
	TakenAt     time.Time `gorm:"autoCreateTime"`
}

func (JudgeExamRecord) TableName() string { return "judge_exam_records" }
