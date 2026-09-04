package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrInsufficientQuestions = errors.New("not enough questions in pool")
	ErrAlreadyVoted          = errors.New("already voted on this case")
	ErrCaseNotFound          = errors.New("judge case not found")
	ErrNoQualification       = errors.New("no judge qualification for this content type")
	// T37（FIX-36a）：抽题会话缺失或过期——提交必须按会话题集评分，不得重新抽题。
	ErrExamSessionExpired = errors.New("exam session expired or missing")
	// T37（FIX-36a）：已持有有效资格再考拦截。
	ErrAlreadyQualified = errors.New("already qualified for this content type")
	// T38（FIX-36b）：理由投票守卫。
	ErrReasonSelfVote             = errors.New("cannot vote on your own reason")
	ErrJudgeQualificationRequired = errors.New("judge qualification required")
	ErrReasonVoteNotFound         = errors.New("reason target vote not found")
)

const examQuestionCount = 10
const examPassRate = 0.8

// examSessionTTL（T37）：抽题会话窗口，过期后提交返回「会话过期，请重新抽题」。
const examSessionTTL = 15 * time.Minute

type JudgeService struct {
	judgeRepo *repository.JudgeRepository
	reputSvc  *ReputationService
	cfg       *config.Config
	// 闭案回写 seam（FIX-10）：db/rdb/outbox/通知仅在装配后可用。
	db       *gorm.DB
	rdb      *redis.Client
	outbox   repository.OutboxWriter
	notifSvc *NotificationService
}

func NewJudgeService(judgeRepo *repository.JudgeRepository, reputSvc *ReputationService, cfg *config.Config) *JudgeService {
	return &JudgeService{judgeRepo: judgeRepo, reputSvc: reputSvc, cfg: cfg}
}

// SetNotificationService attaches the author-notification seam used when a
// closed judge case changes the content status (FIX-17a contract).
func (s *JudgeService) SetNotificationService(ns *NotificationService) {
	s.notifSvc = ns
}

// SetContentOutcomeWriter wires the write-back seam for closed judge cases
// (FIX-10): the content conditional update, its index event and the cache
// invalidation all run through these handles. Unwired (nil db) the outcome
// write-back is skipped, which keeps exam/qualification paths hermetic.
func (s *JudgeService) SetContentOutcomeWriter(db *gorm.DB, rdb *redis.Client, outbox repository.OutboxWriter) {
	s.db = db
	s.rdb = rdb
	s.outbox = outbox
}

// examSessionKey（T37）：抽题会话键，按 用户+内容类型 隔离。
func examSessionKey(userID int64, contentType string) string {
	return fmt.Sprintf("judge:exam:%d:%s", userID, contentType)
}

// GetExam 抽题并绑定考试会话（T37）：题集与 correct_key 映射写入 Redis（TTL 15min），
// SubmitExam 仅按该会话题集评分。已持有有效资格的用户拒绝再考。
func (s *JudgeService) GetExam(contentType string, userID int64) ([]model.JudgeQuestion, error) {
	qualified, err := s.judgeRepo.CheckQualification(userID, contentType)
	if err != nil {
		return nil, err
	}
	if qualified {
		return nil, ErrAlreadyQualified
	}

	count, err := s.judgeRepo.CountQuestions(contentType)
	if err != nil {
		return nil, err
	}
	if count < examQuestionCount {
		return nil, ErrInsufficientQuestions
	}
	questions, err := s.judgeRepo.GetRandomQuestions(contentType, examQuestionCount)
	if err != nil {
		return nil, err
	}

	// 会话绑定：question_id → correct_key（服务端持有，不下发答案字段）。
	if s.rdb != nil {
		session := make(map[int64]string, len(questions))
		for _, q := range questions {
			var data map[string]interface{}
			if err := json.Unmarshal(q.QuestionData, &data); err != nil {
				continue
			}
			if key, ok := data["correct_key"].(string); ok {
				session[q.ID] = key
			}
		}
		payload, err := json.Marshal(session)
		if err == nil {
			if err := s.rdb.Set(context.Background(), examSessionKey(userID, contentType), payload, examSessionTTL).Err(); err != nil {
				slog.Error("judge exam: failed to bind exam session", "user_id", userID, "content_type", contentType, "error", err)
			}
		} else {
			slog.Error("judge exam: failed to marshal exam session", "user_id", userID, "content_type", contentType, "error", err)
		}
	}
	return questions, nil
}

type ExamAnswer struct {
	QuestionID int64  `json:"question_id"`
	Answer     string `json:"answer"`
}

type SubmitExamInput struct {
	ContentType string       `json:"content_type" binding:"required"`
	Answers     []ExamAnswer `json:"answers" binding:"required"`
}

// SubmitExam 按抽题会话评分（T37）：total=服务端下发题数（会话题集长度）而非
// len(answers)——单题提交不再可过；会话缺失/过期一律拒绝，不重新抽题。
func (s *JudgeService) SubmitExam(input SubmitExamInput, userID int64) (*model.JudgeExamRecord, bool, error) {
	qualified, err := s.judgeRepo.CheckQualification(userID, input.ContentType)
	if err != nil {
		return nil, false, err
	}
	if qualified {
		return nil, false, ErrAlreadyQualified
	}

	if s.rdb == nil {
		return nil, false, ErrExamSessionExpired
	}
	key := examSessionKey(userID, input.ContentType)
	raw, err := s.rdb.Get(context.Background(), key).Result()
	if err != nil {
		// redis.Nil（未抽题或 TTL 过期）与其他读取错误都不得回退到重新抽题评分。
		return nil, false, ErrExamSessionExpired
	}
	var session map[int64]string
	if err := json.Unmarshal([]byte(raw), &session); err != nil || len(session) == 0 {
		return nil, false, ErrExamSessionExpired
	}

	correct := 0
	for _, a := range input.Answers {
		if expected, ok := session[a.QuestionID]; ok && expected == a.Answer {
			correct++
		}
	}
	total := len(session)
	passed := total > 0 && float64(correct)/float64(total) >= examPassRate

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

	// 会话一次性：提交即消费，防同一会话重复提交。
	if err := s.rdb.Del(context.Background(), key).Err(); err != nil {
		slog.Warn("judge exam: failed to consume exam session", "user_id", userID, "content_type", input.ContentType, "error", err)
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
		} else {
			s.applyCaseOutcome(input.CaseID, judgeCase.TargetID, newStatus)
			// T39（FIX-03）：闭案提交后对多数派在册判官 +1 准确率奖励（幂等）。
			s.awardMajorityAccuracy(input.CaseID, newStatus)
		}
	}

	if err := s.checkAndRevokeIfNeeded(judgeID); err != nil {
		slog.Error("failed to check and revoke judge", "judge_id", judgeID, "error", err)
	}
	return nil
}

// awardMajorityAccuracy（T39/FIX-03）：闭案提交后对与多数派一致的在册判官
// 发放 judge_accuracy +1 奖励。best-effort：失败仅记日志，不回滚投票与闭案。
// 幂等由 AwardJudgeAccuracy 内的 (user, reason, related_id) 去重保证。
func (s *JudgeService) awardMajorityAccuracy(caseID int64, newStatus string) {
	if s.reputSvc == nil {
		return
	}
	winningVote := "approve"
	if newStatus == "closed_reject" {
		winningVote = "reject"
	}
	votes, err := s.judgeRepo.ListVotesForCase(caseID)
	if err != nil {
		slog.Error("judge accuracy award: failed to list votes", "case_id", caseID, "error", err)
		return
	}
	for _, v := range votes {
		if v.Vote != winningVote {
			continue
		}
		quals, err := s.judgeRepo.GetUserQualifications(v.JudgeID)
		if err != nil {
			slog.Warn("judge accuracy award: failed to load qualifications", "case_id", caseID, "judge_id", v.JudgeID, "error", err)
			continue
		}
		if len(quals) == 0 {
			continue
		}
		if err := s.reputSvc.AwardJudgeAccuracy(v.JudgeID, caseID); err != nil {
			slog.Warn("judge accuracy award failed", "case_id", caseID, "judge_id", v.JudgeID, "error", err)
		}
	}
}

// applyCaseOutcome lets a closed judge case take effect on its target
// content (FIX-10). closed_approve conditionally restores the content to
// published (the under_review guard can never overwrite an admin/AI terminal
// banned verdict) and re-emits the index event; closed_reject bans it with a
// judge_verdict reason and skips the reputation penalty — penalty semantics
// belong to the AI/admin channels. Both paths invalidate the public content
// cache and notify the author through the FIX-17a contract. Best-effort after
// the vote/case commit: a write-back failure is logged but never rolls back
// the vote, and the conditional update keeps replays idempotent.
func (s *JudgeService) applyCaseOutcome(caseID, contentID int64, outcome string) {
	if s.db == nil {
		return
	}
	var content model.ContentItem
	if err := s.db.First(&content, contentID).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Error("judge case outcome: failed to load content", "case_id", caseID, "content_id", contentID, "error", err)
		}
		return
	}

	var status, topic, reason string
	switch outcome {
	case "closed_approve":
		status, topic = "published", events.TopicContentPublished
	case "closed_reject":
		status, topic, reason = "banned", events.TopicContentBanned, "judge_verdict: 不违规比例未达 60%"
	default:
		return
	}

	updates := map[string]interface{}{"status": status}
	if status == "banned" {
		updates["ban_reason"] = reason
	}
	applied := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ContentItem{}).
			Where("id = ? AND status = ?", contentID, "under_review").
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		applied = res.RowsAffected > 0
		if !applied || s.outbox == nil {
			return nil
		}
		return EmitContentStatusEventTx(context.Background(), tx, s.outbox, topic, &content, status)
	})
	if err != nil {
		slog.Error("judge case outcome: failed to write back content status", "case_id", caseID, "content_id", contentID, "error", err)
		return
	}
	InvalidateContentCaches(s.rdb, contentID)
	if applied && s.notifSvc != nil {
		notifyReason := ""
		if status == "banned" {
			notifyReason = reason
		}
		s.notifSvc.NotifyContentStatus(content.AuthorID, content.ID, content.Title, status, notifyReason, 0)
	}
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

	// T39（FIX-03）：outcome 阈值与闭案口径同源（judge.pass_threshold，缺省 0.6），
	// 避免调整闭案阈值时撤权仍按旧口径误判。
	threshold := s.cfg.Judge.PassThreshold
	if threshold == 0 {
		threshold = 0.6
	}

	wrong := 0
	for _, v := range votes {
		judgeCase, err := s.judgeRepo.FindCase(v.CaseID)
		if err != nil || judgeCase == nil || !strings.HasPrefix(judgeCase.Status, "closed_") {
			// T39（FIX-03）：结案终态是 closed_approve/closed_reject——
			// 旧词表（== "closed"）恒假导致撤权为死代码。
			continue
		}
		totalVotes := judgeCase.VoteApprove + judgeCase.VoteReject
		if totalVotes == 0 {
			continue
		}
		outcome := "approve"
		if float64(judgeCase.VoteApprove)/float64(totalVotes) < threshold {
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

func (s *JudgeService) GetVerdictDetail(caseID int64) (*model.JudgeCase, []repository.VerdictVoteItem, error) {
	judgeCase, err := s.judgeRepo.FindCase(caseID)
	if err != nil || judgeCase == nil {
		return nil, nil, ErrCaseNotFound
	}
	votes, err := s.judgeRepo.ListVerdictVotes(caseID)
	return judgeCase, votes, err
}

// VoteReason（T38/FIX-36b）：理由投票守卫——禁自赞（owner==caller），
// 投票人须持任一有效判官资格；重复投票幂等，反方向切换。
func (s *JudgeService) VoteReason(voteID int64, voterID int64, voteType string) error {
	vote, err := s.judgeRepo.GetVoteByID(voteID)
	if err != nil {
		return ErrReasonVoteNotFound
	}
	if vote.JudgeID == voterID {
		return ErrReasonSelfVote
	}
	quals, err := s.judgeRepo.GetUserQualifications(voterID)
	if err != nil {
		return err
	}
	if len(quals) == 0 {
		return ErrJudgeQualificationRequired
	}
	return s.judgeRepo.UpsertReasonVote(voteID, voterID, voteType)
}
