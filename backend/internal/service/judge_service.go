package service

import (
	"encoding/json"
	"errors"
	"log/slog"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrInsufficientQuestions = errors.New("not enough questions in pool")
	ErrAlreadyVoted          = errors.New("already voted on this case")
	ErrCaseNotFound          = errors.New("judge case not found")
	ErrNoQualification       = errors.New("no judge qualification for this content type")
)

const examQuestionCount = 10
const examPassRate = 0.8

type JudgeService struct {
	judgeRepo *repository.JudgeRepository
	reputSvc  *ReputationService
	cfg       *config.Config
}

func NewJudgeService(judgeRepo *repository.JudgeRepository, reputSvc *ReputationService, cfg *config.Config) *JudgeService {
	return &JudgeService{judgeRepo: judgeRepo, reputSvc: reputSvc, cfg: cfg}
}

func (s *JudgeService) GetExam(contentType string) ([]model.JudgeQuestion, error) {
	count, err := s.judgeRepo.CountQuestions(contentType)
	if err != nil {
		return nil, err
	}
	if count < examQuestionCount {
		return nil, ErrInsufficientQuestions
	}
	return s.judgeRepo.GetRandomQuestions(contentType, examQuestionCount)
}

type ExamAnswer struct {
	QuestionID int64  `json:"question_id"`
	Answer     string `json:"answer"`
}

type SubmitExamInput struct {
	ContentType string       `json:"content_type" binding:"required"`
	Answers     []ExamAnswer `json:"answers" binding:"required"`
}

func (s *JudgeService) SubmitExam(input SubmitExamInput, userID int64) (*model.JudgeExamRecord, bool, error) {
	questions, err := s.judgeRepo.GetRandomQuestions(input.ContentType, examQuestionCount)
	if err != nil || len(questions) == 0 {
		return nil, false, ErrInsufficientQuestions
	}

	questionMap := make(map[int64]string)
	for _, q := range questions {
		var data map[string]interface{}
		if err := json.Unmarshal(q.QuestionData, &data); err == nil {
			if correctKey, ok := data["correct_key"].(string); ok {
				questionMap[int64(q.ID)] = correctKey
			}
		}
	}

	correct := 0
	for _, a := range input.Answers {
		if expected, ok := questionMap[a.QuestionID]; ok && expected == a.Answer {
			correct++
		}
	}

	total := len(input.Answers)
	passed := float64(correct)/float64(total) >= examPassRate

	record := &model.JudgeExamRecord{
		UserID:      userID,
		ContentType: input.ContentType,
		Score:       correct,
		Total:       total,
		Passed:      passed,
	}
	if err := s.judgeRepo.CreateExamRecord(record); err != nil {
		return nil, false, err
	}

	if passed {
		if err := s.judgeRepo.CreateQualification(userID, input.ContentType); err != nil {
			slog.Error("failed to create judge qualification", "user_id", userID, "content_type", input.ContentType, "error", err)
		}
	}

	return record, passed, nil
}

type SubmitVoteInput struct {
	CaseID int64  `json:"case_id" binding:"required"`
	Vote   string `json:"vote" binding:"required,oneof=approve reject"`
	Reason string `json:"reason"`
}

func (s *JudgeService) GetJudgeQueue(userID int64, page, pageSize int) ([]model.JudgeCase, int64, error) {
	quals, err := s.judgeRepo.GetUserQualifications(userID)
	if err != nil {
		return nil, 0, err
	}

	types := make([]string, 0, len(quals))
	for _, q := range quals {
		types = append(types, q.ContentType)
	}
	if len(types) == 0 {
		return []model.JudgeCase{}, 0, nil
	}

	return s.judgeRepo.ListOpenCases(types, page, pageSize)
}

func (s *JudgeService) SubmitVote(input SubmitVoteInput, judgeID int64) error {
	voted, err := s.judgeRepo.HasVoted(input.CaseID, judgeID)
	if err != nil {
		return err
	}
	if voted {
		return ErrAlreadyVoted
	}

	vote := &model.JudgeVote{
		CaseID:  input.CaseID,
		JudgeID: judgeID,
		Vote:    input.Vote,
		Reason:  input.Reason,
	}
	if err := s.judgeRepo.CreateVote(vote); err != nil {
		return err
	}

	approve, reject, _ := s.judgeRepo.GetVoteStats(input.CaseID)
	judgeCase, err := s.judgeRepo.FindCase(input.CaseID)
	if err != nil || judgeCase == nil {
		return nil
	}

	total := approve + reject
	if total >= int64(judgeCase.MinVotes) {
		approveRatio := float64(approve) / float64(total)
		threshold := s.cfg.Judge.PassThreshold
		if threshold == 0 {
			threshold = 0.6
		}
		var newStatus string
		if approveRatio >= threshold {
			newStatus = "closed_approve"
		} else {
			newStatus = "closed_reject"
		}
		if err := s.judgeRepo.CloseCase(input.CaseID, newStatus, int(approve), int(reject)); err != nil {
			slog.Error("failed to close judge case", "case_id", input.CaseID, "error", err)
		}
	}

	if err := s.checkAndRevokeIfNeeded(judgeID); err != nil {
		slog.Error("failed to check and revoke judge", "judge_id", judgeID, "error", err)
	}
	return nil
}

func (s *JudgeService) checkAndRevokeIfNeeded(judgeID int64) error {
	window := s.cfg.Judge.ErrorRateWindow
	if window <= 10 {
		window = 10
	}
	votes, err := s.judgeRepo.GetRecentVoteHistory(judgeID, window)
	if err != nil || len(votes) <= 10 {
		return nil
	}

	wrong := 0
	for _, v := range votes {
		judgeCase, err := s.judgeRepo.FindCase(v.CaseID)
		if err != nil || judgeCase == nil || judgeCase.Status != "closed" {
			continue
		}
		totalVotes := judgeCase.VoteApprove + judgeCase.VoteReject
		if totalVotes == 0 {
			continue
		}
		outcome := "approve"
		if float64(judgeCase.VoteApprove)/float64(totalVotes) < 0.6 {
			outcome = "reject"
		}
		if v.Vote != outcome {
			wrong++
		}
	}

	if len(votes) > 0 && float64(wrong)/float64(len(votes)) > 0.5 {
		if err := s.judgeRepo.RevokeQualifications(judgeID); err != nil {
			return err
		}
		if s.reputSvc != nil {
			_ = s.reputSvc.AddReputation(judgeID, -1, "judge_error_rate", nil)
		}
	}

	return nil
}

func (s *JudgeService) GetVerdictDetail(caseID int64) (*model.JudgeCase, []model.JudgeVote, error) {
	judgeCase, err := s.judgeRepo.FindCase(caseID)
	if err != nil || judgeCase == nil {
		return nil, nil, ErrCaseNotFound
	}
	votes, err := s.judgeRepo.ListVotesForCase(caseID)
	return judgeCase, votes, err
}
