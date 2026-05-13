package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type JudgeHandler struct {
	judgeSvc *service.JudgeService
	judgeRepo *repository.JudgeRepository
}

func NewJudgeHandler(db *gorm.DB, cfg *config.Config) *JudgeHandler {
	judgeRepo := repository.NewJudgeRepository(db)
	reputSvc := service.NewReputationService(db)
	return &JudgeHandler{
		judgeSvc:  service.NewJudgeService(judgeRepo, reputSvc, cfg),
		judgeRepo: judgeRepo,
	}
}

func (h *JudgeHandler) GetExam(c *gin.Context) {
	category := c.Param("category")
	questions, err := h.judgeSvc.GetExam(category)
	if err != nil {
		if err == service.ErrInsufficientQuestions {
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "INSUFFICIENT_QUESTIONS", "message": "not enough questions available for this category"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	sanitized := make([]gin.H, 0, len(questions))
	for _, q := range questions {
		var data map[string]interface{}
		if err := json.Unmarshal(q.QuestionData, &data); err != nil {
			slog.Warn("judge question: failed to unmarshal question data", "question_id", q.ID, "error", err)
			data = map[string]interface{}{}
		}
		delete(data, "correct_key")
		sanitized = append(sanitized, gin.H{
			"id":           q.ID,
			"content_type": q.ContentType,
			"question":     data,
		})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	record, passed, err := h.judgeSvc.SubmitExam(input, callerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	if err := h.judgeSvc.SubmitVote(input, callerID); err != nil {
		if err == service.ErrAlreadyVoted {
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_VOTED", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	rv := model.JudgeReasonVote{
		ReasonOwnerVoteID: voteID,
		VoterID:           callerID,
		VoteType:          body.VoteType,
	}
	if err := h.judgeRepo.CreateReasonVote(&rv); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": "ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "reason vote submitted"})
}

func (h *JudgeHandler) CreateQuestions(c *gin.Context) {
	var questions []model.JudgeQuestion
	if err := c.ShouldBindJSON(&questions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	for i := range questions {
		if err := h.judgeRepo.CreateQuestion(&questions[i]); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
			return
		}
	}
	c.JSON(http.StatusCreated, gin.H{"created": len(questions)})
}
