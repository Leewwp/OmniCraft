package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PRHandler struct {
	prSvc *service.PRService
}

func NewPRHandler(db *gorm.DB) *PRHandler {
	return &PRHandler{
		prSvc: service.NewPRService(
			repository.NewPRRepository(db),
			repository.NewVersionRepository(db),
			repository.NewContentRepository(db),
		),
	}
}

func (h *PRHandler) SubmitPR(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	var input service.SubmitPRInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	pr, err := h.prSvc.SubmitPR(input, callerID)
	if err != nil {
		switch err {
		case service.ErrContentNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "CONTENT_NOT_FOUND", "message": err.Error()})
		case service.ErrPRBlocked:
			c.JSON(http.StatusForbidden, gin.H{"code": "BLOCKED", "message": err.Error()})
		case service.ErrPRConflict:
			c.JSON(http.StatusConflict, gin.H{"code": "CONFLICT", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"pr": pr})
}

func (h *PRHandler) GetPR(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid pr id"})
		return
	}

	pr, err := h.prSvc.GetPR(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "pr not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pr": pr})
}

func (h *PRHandler) ListPRs(c *gin.Context) {
	contentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}

	prs, err := h.prSvc.ListPRs(contentID, c.Query("status"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"prs": prs})
}

func (h *PRHandler) AcceptPR(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid pr id"})
		return
	}

	if err := h.prSvc.AcceptPR(id, callerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pr accepted"})
}

func (h *PRHandler) RejectPR(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid pr id"})
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)

	if err := h.prSvc.RejectPR(id, callerID, body.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pr rejected"})
}

func (h *PRHandler) ManualMerge(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid pr id"})
		return
	}

	var body struct {
		MergedText string `json:"merged_text"`
	}
	_ = c.ShouldBindJSON(&body)

	version, err := h.prSvc.ManualMerge(id, callerID, body.MergedText)
	if err != nil {
		switch err {
		case service.ErrPRNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": err.Error()})
		case service.ErrPRForbidden:
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pr merged", "version": version})
}

func (h *PRHandler) BlockContributor(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	if err := h.prSvc.BlockContributor(callerID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "contributor blocked"})
}

func (h *PRHandler) UnblockContributor(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	if err := h.prSvc.UnblockContributor(callerID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "contributor unblocked"})
}
