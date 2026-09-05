package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type JudgeHandler struct {
	judgeSvc *service.JudgeService
	auditSvc *service.AdminAuditService
	db       *gorm.DB
}

// NewJudgeHandler consumes the container-wired JudgeService (#379/F-A001):
// the exam-session gate (T37) and the closed-case write-back/notification
// seams (FIX-10/17a) only work on the instance assembled in container.go
// (SetContentOutcomeWriter + SetNotificationService). Building a bare
// service here silently regressed every judge route — sessions never bound,
// submissions always 409, outcomes never written back.
func NewJudgeHandler(db *gorm.DB, judgeSvc *service.JudgeService, auditSvc *service.AdminAuditService) *JudgeHandler {
	return &JudgeHandler{
		judgeSvc: judgeSvc,
		auditSvc: auditSvc,
		db:       db,
	}
}

// sanitizeExamQuestion 构造考试题的对外形态（T37/F-100 allowlist）：只透出
// 题干与选项。question_data 其余字段一律不下发——调度器生成的题内嵌
// votes_approve/votes_reject 可机械推算正确答案，admin 手工题的 explanation
// 常含答案明文。题干兼容 prompt 与调度器的 question 两种字段名。
func sanitizeExamQuestion(q model.JudgeQuestion) gin.H {
	var data map[string]interface{}
	if err := json.Unmarshal(q.QuestionData, &data); err != nil {
		slog.Warn("judge question: failed to unmarshal question data", "question_id", q.ID, "error", err)
		data = map[string]interface{}{}
	}
	prompt := data["prompt"]
	if prompt == nil {
		prompt = data["question"]
	}
	return gin.H{
		"id":           q.ID,
		"content_type": q.ContentType,
		"question": gin.H{
			"prompt":  prompt,
			"options": data["options"],
		},
	}
}

func (h *JudgeHandler) GetExam(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		// T37：抽题会话按 用户+类型 绑定，匿名不可抽题。
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	category := c.Param("category")
	questions, err := h.judgeSvc.GetExam(category, callerID)
	if err != nil {
		if err == service.ErrAlreadyQualified {
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_QUALIFIED", "message": "you already hold the qualification for this content type"})
			return
		}
		if err == service.ErrInsufficientQuestions {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "INSUFFICIENT_QUESTIONS", "message": "not enough questions available for this category"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	sanitized := make([]gin.H, 0, len(questions))
	for _, q := range questions {
		sanitized = append(sanitized, sanitizeExamQuestion(q))
	}
	c.JSON(http.StatusOK, gin.H{"questions": sanitized})
}

func (h *JudgeHandler) SubmitExam(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.SubmitExamInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	record, passed, err := h.judgeSvc.SubmitExam(input, callerID)
	if err != nil {
		if err == service.ErrAlreadyQualified {
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_QUALIFIED", "message": "you already hold the qualification for this content type"})
			return
		}
		if err == service.ErrExamSessionExpired {
			// T37：会话缺失/TTL 过期——必须重新抽题，不得按新题集评分。
			c.JSON(http.StatusConflict, gin.H{"code": "EXAM_SESSION_EXPIRED", "message": "exam session expired, please draw questions again"})
			return
		}
		if err == service.ErrInsufficientQuestions {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "INSUFFICIENT_QUESTIONS", "message": "not enough questions available for this category"})
			return
		}
		response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"record": record, "passed": passed})
}

func (h *JudgeHandler) GetQueue(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	cases, total, err := h.judgeSvc.GetJudgeQueue(callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"cases": cases, "total": total, "page": page, "page_size": pageSize})
}

func (h *JudgeHandler) SubmitVote(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.SubmitVoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	if err := h.judgeSvc.SubmitVote(input, callerID); err != nil {
		if err == service.ErrAlreadyVoted {
			response.SafeErrorResponse(c, http.StatusConflict, "ALREADY_VOTED", err)
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "vote submitted"})
}

func (h *JudgeHandler) GetVerdictDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid case id"})
		return
	}
	judgeCase, votes, err := h.judgeSvc.GetVerdictDetail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "case not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": judgeCase, "votes": votes})
}

func (h *JudgeHandler) VoteReason(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	voteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid vote id"})
		return
	}
	var body struct {
		VoteType string `json:"vote_type" binding:"required,oneof=up down"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	// T38（FIX-36b）：守卫下沉 service——禁自赞（409）、判官资格校验（403）、
	// 重复投票幂等/反方向切换。
	if err := h.judgeSvc.VoteReason(voteID, callerID, body.VoteType); err != nil {
		switch err {
		case service.ErrReasonVoteNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "reason target vote not found"})
		case service.ErrReasonSelfVote:
			c.JSON(http.StatusConflict, gin.H{"code": "REASON_SELF_VOTE", "message": "cannot vote on your own reason"})
		case service.ErrJudgeQualificationRequired:
			c.JSON(http.StatusForbidden, gin.H{"code": "JUDGE_QUALIFICATION_REQUIRED", "message": "judge qualification required"})
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "reason vote submitted"})
}

func (h *JudgeHandler) CreateQuestions(c *gin.Context) {
	var questions []model.JudgeQuestion
	if err := c.ShouldBindJSON(&questions); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}
	adminID := middleware.GetUserID(c)
	traceID := c.GetString("trace_id")
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		for i := range questions {
			if err := tx.Create(&questions[i]).Error; err != nil {
				return err
			}
			if h.auditSvc == nil {
				continue
			}
			if err := h.auditSvc.RecordTx(c.Request.Context(), tx, service.RecordAdminAuditInput{
				AdminUserID: adminID,
				Action:      "judge_question_create",
				TargetType:  "judge_question",
				TargetID:    strconv.FormatInt(questions[i].ID, 10),
				TraceID:     traceID,
				Metadata:    map[string]any{"question_id": questions[i].ID, "content_type": questions[i].ContentType},
				Result:      "success",
			}); err != nil {
				return errAdminAuditWriteFailed
			}
		}
		return nil
	}); err != nil {
		if err == errAdminAuditWriteFailed {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "AUDIT_WRITE_FAILED", "message": "audit write failed"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"created": len(questions)})
}
