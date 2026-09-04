package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SocialHandler struct {
	socialSvc     *service.SocialService
	db            *gorm.DB
	displaySigner *service.DisplayURLSigner
}

func NewSocialHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *SocialHandler {
	h := NewSocialHandlerWithService(service.NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		cfg,
		rdb,
		service.NewReviewService(db, rdb, cfg, nil),
	), db)
	h.SetDisplayURLSigner(service.NewDisplayURLSigner(cfg))
	return h
}

func NewSocialHandlerWithService(socialSvc *service.SocialService, db *gorm.DB) *SocialHandler {
	return &SocialHandler{socialSvc: socialSvc, db: db}
}

// SetDisplayURLSigner wires display URL signing for comment/discussion
// author avatars and linked IP covers (B-002).
func (h *SocialHandler) SetDisplayURLSigner(signer *service.DisplayURLSigner) {
	h.displaySigner = signer
}

func (h *SocialHandler) PostComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.PostCommentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	comment, err := h.socialSvc.PostComment(c.Request.Context(), input, callerID)
	if err != nil {
		if err == service.ErrLowReputation {
			response.Forbidden(c, "reputation score too low to perform this action")
			return
		}
		if err == service.ErrTextBlocked {
			response.Error(c, http.StatusUnprocessableEntity, "CONTENT_BLOCKED", "content was rejected by content moderation")
			return
		}
		if err == service.ErrModerationUnavailable {
			response.Error(c, http.StatusServiceUnavailable, "MODERATION_UNAVAILABLE", "content moderation is temporarily unavailable, please try again later")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return
	}
	h.displaySigner.DecorateComment(comment)
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *SocialHandler) DeleteComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid comment id"})
		return
	}
	// 错误风格与 EditComment 对齐（FIX-31b/F-093）：404/403 专用码，不再落
	// 400 "ERROR" 通配透传底层错误。
	if err := h.socialSvc.DeleteComment(id, callerID); err != nil {
		if err == service.ErrCommentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "comment not found"})
			return
		}
		if err == service.ErrCommentForbidden {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "not comment author"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *SocialHandler) EditComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid comment id"})
		return
	}
	var body struct {
		Body string `json:"body" binding:"required,min=1,max=5000"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	comment, err := h.socialSvc.EditComment(c.Request.Context(), id, callerID, body.Body)
	if err != nil {
		if err == service.ErrCommentNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "comment not found"})
			return
		}
		if err == service.ErrCommentForbidden {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "not comment author"})
			return
		}
		if err == service.ErrTextBlocked {
			response.Error(c, http.StatusUnprocessableEntity, "CONTENT_BLOCKED", "content was rejected by content moderation")
			return
		}
		if err == service.ErrModerationUnavailable {
			response.Error(c, http.StatusServiceUnavailable, "MODERATION_UNAVAILABLE", "content moderation is temporarily unavailable, please try again later")
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": "database error"})
		return
	}
	h.displaySigner.DecorateComment(comment)
	c.JSON(http.StatusOK, gin.H{"comment": comment})
}

func (h *SocialHandler) ListComments(c *gin.Context) {
	contentIDStr := c.Query("content_item_id")
	if contentIDStr == "" {
		contentIDStr = c.Query("content_id")
	}
	contentID, err := strconv.ParseInt(contentIDStr, 10, 64)
	if err != nil || contentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "content_item_id required"})
		return
	}
	var parentID *int64
	if p := c.Query("parent_id"); p != "" {
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			parentID = &v
		}
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	comments, total, err := h.socialSvc.ListComments(contentID, parentID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateComments(comments)
	c.JSON(http.StatusOK, gin.H{"comments": comments, "total": total, "page": page, "page_size": pageSize})
}

func (h *SocialHandler) ListDiscussions(c *gin.Context) {
	var ipID, contentID *int64
	if v, err := strconv.ParseInt(c.Query("ip_id"), 10, 64); err == nil {
		ipID = &v
	}
	if v, err := strconv.ParseInt(c.Query("content_id"), 10, 64); err == nil {
		contentID = &v
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	discussions, total, err := h.socialSvc.ListDiscussions(ipID, contentID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateDiscussions(discussions)
	c.JSON(http.StatusOK, gin.H{"discussions": discussions, "total": total, "page": page, "page_size": pageSize})
}

func (h *SocialHandler) PostDiscussion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.PostDiscussionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	d, err := h.socialSvc.PostDiscussion(c.Request.Context(), input, callerID)
	if err != nil {
		if err == service.ErrLowReputation {
			response.Forbidden(c, "reputation score too low to perform this action")
			return
		}
		if err == service.ErrTextBlocked {
			response.Error(c, http.StatusUnprocessableEntity, "CONTENT_BLOCKED", "content was rejected by content moderation")
			return
		}
		if err == service.ErrModerationUnavailable {
			response.Error(c, http.StatusServiceUnavailable, "MODERATION_UNAVAILABLE", "content moderation is temporarily unavailable, please try again later")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return
	}
	h.displaySigner.DecorateDiscussion(d)
	c.JSON(http.StatusCreated, gin.H{"discussion": d})
}

func (h *SocialHandler) GetDiscussion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid discussion id"})
		return
	}
	d, err := h.socialSvc.GetDiscussion(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "discussion not found"})
		return
	}
	h.displaySigner.DecorateDiscussion(d)
	c.JSON(http.StatusOK, gin.H{"discussion": d})
}

func (h *SocialHandler) React(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	var input service.ReactInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	action, err := h.socialSvc.React(input, callerID)
	if err != nil {
		if err == service.ErrLowReputation {
			response.Forbidden(c, "reputation score too low to perform this action")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	counts, viewerReaction, err := reactionSnapshot(h.db, callerID, input.TargetType, input.TargetID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"action": action, "counts": counts, "viewer_reaction": viewerReaction})
}

func (h *SocialHandler) ListReactions(c *gin.Context) {
	targetType := c.Query("target_type")
	targetIDStr := c.Query("target_id")
	if targetType == "" || targetIDStr == "" {
		response.ValidationError(c, "target_type and target_id are required")
		return
	}
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		response.ValidationError(c, "invalid target_id")
		return
	}

	if targetType != "content" && targetType != "comment" {
		response.ValidationError(c, "invalid target_type")
		return
	}
	counts, viewerReaction, err := reactionSnapshot(h.db, middleware.GetUserID(c), targetType, targetID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"counts": counts, "viewer_reaction": viewerReaction})
}

func reactionSnapshot(db *gorm.DB, userID int64, targetType string, targetID int64) (gin.H, *string, error) {
	var rows []struct {
		Reaction string
		Count    int64
	}
	if err := db.Model(&model.Reaction{}).Select("reaction, COUNT(*) AS count").
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Group("reaction").Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	counts := gin.H{"like": int64(0), "dislike": int64(0)}
	for _, row := range rows {
		if row.Reaction == "like" || row.Reaction == "dislike" {
			counts[row.Reaction] = row.Count
		}
	}
	viewerReaction, err := repository.NewSocialRepository(db).GetViewerReaction(userID, targetType, targetID)
	if err != nil {
		return nil, nil, err
	}
	return counts, viewerReaction, nil
}

func (h *SocialHandler) ReportContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	contentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	var body struct {
		Reason string `json:"reason" binding:"required"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	if err := h.socialSvc.Report("content", contentID, callerID, body.Reason, body.Detail); err != nil {
		if err == service.ErrAlreadyReported {
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_REPORTED", "message": "you have already reported this content"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "reported"})
}

func (h *SocialHandler) ReportComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	commentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid comment id"})
		return
	}
	var body struct {
		Reason string `json:"reason" binding:"required"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request parameters")
		return
	}
	if err := h.socialSvc.Report("comment", commentID, callerID, body.Reason, body.Detail); err != nil {
		if err == service.ErrAlreadyReported {
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_REPORTED", "message": "you have already reported this comment"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "reported"})
}

// ListMyReports returns the caller's own reports with handling status and
// action-taken notes (FIX-28a). The reporter_id filter is derived from the
// auth context so other users' reports can never leak.
func (h *SocialHandler) ListMyReports(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	searchRepo := repository.NewSearchRepository(h.db)
	reports, total, err := searchRepo.ListReportsByReporter(callerID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	if reports == nil {
		reports = []model.Report{}
	}
	c.JSON(http.StatusOK, gin.H{
		"reports":   reports,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
