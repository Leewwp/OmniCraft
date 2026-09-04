package handler

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PRHandler struct {
	prSvc *service.PRService
}

func NewPRHandler(db *gorm.DB) *PRHandler {
	return NewPRHandlerWithService(service.NewPRService(
		repository.NewPRRepository(db),
		repository.NewVersionRepository(db),
		repository.NewContentRepository(db),
	))
}

func NewPRHandlerWithService(prSvc *service.PRService) *PRHandler {
	return &PRHandler{prSvc: prSvc}
}

func (h *PRHandler) SubmitPR(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	var input service.SubmitPRInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}

	pr, err := h.prSvc.SubmitPR(input, callerID)
	if err != nil {
		switch err {
		case service.ErrContentNotFound:
			response.SafeErrorResponse(c, http.StatusNotFound, "CONTENT_NOT_FOUND", err)
		case service.ErrPRBlocked:
			response.SafeErrorResponse(c, http.StatusForbidden, "BLOCKED", err)
		case service.ErrPRConflict:
			response.Conflict(c, "resource conflict")
		case service.ErrPRBaseInvalid:
			response.Error(c, http.StatusBadRequest, "INVALID_BASE_VERSION", "base version does not belong to the content")
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
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

	// participant gate: content author / PR submitter / admin (FIX-21④).
	isAdmin := middleware.IsAdmin(c)

	pr, err := h.prSvc.GetPRForViewer(id, middleware.GetUserID(c), isAdmin)
	if err != nil {
		switch err {
		case service.ErrPRNotFound, service.ErrContentNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "pr not found"})
		case service.ErrPRForbidden:
			response.SafeErrorResponse(c, http.StatusForbidden, "FORBIDDEN", err)
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		}
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	prs, total, err := h.prSvc.ListPRsPaged(contentID, c.Query("status"), page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	c.JSON(http.StatusOK, gin.H{
		"prs":         prs,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

func (h *PRHandler) AcceptPR(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid pr id"})
		return
	}

	if err := h.prSvc.AcceptPR(id, callerID); err != nil {
		switch err {
		case service.ErrPRNotFound, service.ErrContentNotFound:
			response.SafeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err)
		case service.ErrPRForbidden:
			response.SafeErrorResponse(c, http.StatusForbidden, "FORBIDDEN", err)
		case service.ErrPRInvalidState:
			response.Conflict(c, "pr already resolved")
		default:
			response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		}
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
	if err := c.ShouldBindJSON(&body); err != nil {
		slog.Warn("reject pr: bind json failed, using defaults", "error", err)
	}

	if err := h.prSvc.RejectPR(id, callerID, body.Reason); err != nil {
		switch err {
		case service.ErrPRNotFound, service.ErrContentNotFound:
			response.SafeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err)
		case service.ErrPRForbidden:
			response.SafeErrorResponse(c, http.StatusForbidden, "FORBIDDEN", err)
		case service.ErrPRInvalidState:
			response.Conflict(c, "pr already resolved")
		default:
			response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
		}
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
	if err := c.ShouldBindJSON(&body); err != nil {
		slog.Warn("manual merge: bind json failed, using defaults", "error", err)
	}

	version, err := h.prSvc.ManualMerge(id, callerID, body.MergedText)
	if err != nil {
		switch err {
		case service.ErrPRNotFound, service.ErrContentNotFound:
			response.SafeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err)
		case service.ErrPRForbidden:
			response.SafeErrorResponse(c, http.StatusForbidden, "FORBIDDEN", err)
		case service.ErrPRInvalidState:
			response.Conflict(c, "pr already resolved")
		case service.ErrPRMergeTextMissing:
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "merged text required")
		default:
			response.SafeErrorResponse(c, http.StatusBadRequest, "ERROR", err)
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "contributor unblocked"})
}
