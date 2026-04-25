package scheduler

import (
	"encoding/json"
	"log"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"

	"gorm.io/gorm"
)

const judgeQuestionsPerType = 10

type JudgeQuestionSync struct {
	judgeRepo *repository.JudgeRepository
}

func NewJudgeQuestionSync(db *gorm.DB) *JudgeQuestionSync {
	return &JudgeQuestionSync{
		judgeRepo: repository.NewJudgeRepository(db),
	}
}

func (s *JudgeQuestionSync) Start() {
	go func() {
		s.Run()
		ticker := time.NewTicker(7 * 24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.Run()
		}
	}()
}

func (s *JudgeQuestionSync) Run() {
	log.Println("[JudgeQuestionSync] Starting weekly question sync...")

	types, err := s.judgeRepo.GetDistinctCaseTypes()
	if err != nil {
		log.Printf("[JudgeQuestionSync] Failed to get case types: %v", err)
		return
	}

	created := 0
	for _, contentType := range types {
		cases, err := s.judgeRepo.ListClosedCasesForQuestioning(contentType, judgeQuestionsPerType)
		if err != nil {
			log.Printf("[JudgeQuestionSync] Failed to get cases for %s: %v", contentType, err)
			continue
		}

		for _, jcase := range cases {
			correctKey := "B"
			if jcase.Status == "closed_reject" {
				correctKey = "A"
			}

			qData, _ := json.Marshal(map[string]interface{}{
				"question": "根据多数投票意见，以下内容是否违规？",
				"options": map[string]string{
					"A": "违规（应下架）",
					"B": "不违规（应保留）",
				},
				"correct_key":  correctKey,
				"votes_approve": jcase.VoteApprove,
				"votes_reject":  jcase.VoteReject,
			})

			caseID := jcase.ID
			q := &model.JudgeQuestion{
				ContentType:  contentType,
				SourceCaseID: &caseID,
				QuestionData: qData,
				IsActive:     true,
				CreatedBy:    "system",
			}
			if err := s.judgeRepo.CreateQuestion(q); err != nil {
				log.Printf("[JudgeQuestionSync] Failed to create question for case %d: %v", jcase.ID, err)
			} else {
				created++
			}
		}
	}

	log.Printf("[JudgeQuestionSync] Completed: created %d questions", created)
}
