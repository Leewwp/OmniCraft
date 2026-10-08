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

// NewSocialHandler is the test-composition constructor; production routes
// compose from the container-owned SocialService via
// NewSocialHandlerWithService. The ReviewService arrives as a parameter
// (#658)：handler 层不再自建第二份审核服务，测试侧经 testutil seam 供给。
func NewSocialHandler(db *gorm.DB, cfg *config.Config, rdb *redis.Client, reviewSvc *service.ReviewService) *SocialHandler {
	h := NewSocialHandlerWithService(service.NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		cfg,
		rdb,
		reviewSvc,
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
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
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
	id, ok := pathID(c, "id", "invalid comment id")
	if !ok {
		return
	}
	// 错误风格与 EditComment 对齐（FIX-31b/F-093）：404/403 专用码，不再落
	// 400 "ERROR" 通配透传底层错误。
	if err := h.socialSvc.DeleteComment(id, callerID); err != nil {
		if err == service.ErrCommentNotFound {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "comment not found")
			return
		}
		if err == service.ErrCommentForbidden {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "not comment author")
			return
		}
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *SocialHandler) EditComment(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, ok := pathID(c, "id", "invalid comment id")
	if !ok {
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
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "comment not found")
			return
		}
		if err == service.ErrCommentForbidden {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", "not comment author")
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
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "database error")
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
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "content_item_id required")
		return
	}
	var parentID *int64
	if p := c.Query("parent_id"); p != "" {
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			parentID = &v
		}
	}
	page, pageSize := pageQuery(c, 20)
	comments, total, err := h.socialSvc.ListComments(contentID, parentID, page, pageSize, middleware.GetUserID(c))
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
	page, pageSize := pageQuery(c, 20)
	discussions, total, err := h.socialSvc.ListDiscussions(ipID, contentID, page, pageSize, middleware.GetUserID(c))
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
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
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
		// run-1 审计 #4：挂非公开父/未知目标 → 404（不确认目标存在性）。
		if err == service.ErrDiscussionTargetInvalid {
			response.Error(c, http.StatusNotFound, "DISCUSSION_TARGET_INVALID", "discussion target not found or not visible")
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
		return
	}
	h.displaySigner.DecorateDiscussion(d)
	c.JSON(http.StatusCreated, gin.H{"discussion": d})
}

func (h *SocialHandler) GetDiscussion(c *gin.Context) {
	id, ok := pathID(c, "id", "invalid discussion id")
	if !ok {
		return
	}
	d, err := h.socialSvc.GetDiscussion(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "discussion not found")
		return
	}
	// #446/SP-16 P0：与 /discussions/:id 同口径（T12/F-106）——未发布
	// （under_review/hidden）讨论不透出详情，此前 social 路径漏了这层门。
	if d.Status != "published" {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "discussion not found")
		return
	}
	// run-1 审计 #4：social 详情路径补父可见性闸（与 /discussions/:id 同款）。
	if !discussionParentsVisible(c, h.db, d) {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "discussion not found")
		return
	}
	h.displaySigner.DecorateDiscussion(d)
	c.JSON(http.StatusOK, gin.H{"discussion": d})
}

func (h *SocialHandler) React(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
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

// ListMyComments returns the caller's own comments filtered by status
// (#845/A3：申诉「近期事件」选择框的数据源——本人被隐藏评论此前无任何
// 查看途径)。author_id 恒取 auth 上下文、status 白名单仅 hidden，只读
// 端点无越权面；每行携带关联父对象标题（内容/讨论）便于申诉人辨认。
func (h *SocialHandler) ListMyComments(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}
	status := c.Query("status")
	if status == "" {
		status = "hidden"
	}
	if status != "hidden" {
		response.ValidationError(c, "status must be hidden")
		return
	}
	page, pageSize := pageQuery(c, 20)

	comments, total, err := repository.NewSocialRepository(h.db).ListCommentsByAuthorStatus(callerID, status, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	// 批量回填父对象标题（content_items / discussions），父可能已被软删，
	// 故按 id 裸查标题不套可见性 scope——受众是评论作者本人（发评时父必然
	// 对其可见），且 id 全部来自本人评论行，无枚举面。
	contentIDs := make([]int64, 0, len(comments))
	discussionIDs := make([]int64, 0, len(comments))
	for i := range comments {
		if comments[i].ContentItemID != nil {
			contentIDs = append(contentIDs, *comments[i].ContentItemID)
		}
		if comments[i].DiscussionID != nil {
			discussionIDs = append(discussionIDs, *comments[i].DiscussionID)
		}
	}
	contentTitles := map[int64]string{}
	if len(contentIDs) > 0 {
		var rows []struct {
			ID    int64
			Title string
		}
		if err := h.db.Table("content_items").Select("id, title").Where("id IN ?", contentIDs).Scan(&rows).Error; err != nil {
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
			return
		}
		for _, row := range rows {
			contentTitles[row.ID] = row.Title
		}
	}
	discussionTitles := map[int64]string{}
	if len(discussionIDs) > 0 {
		var rows []struct {
			ID    int64
			Title string
		}
		if err := h.db.Table("discussions").Select("id, title").Where("id IN ?", discussionIDs).Scan(&rows).Error; err != nil {
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
			return
		}
		for _, row := range rows {
			discussionTitles[row.ID] = row.Title
		}
	}

	payloads := make([]gin.H, 0, len(comments))
	for i := range comments {
		parentTitle := ""
		if comments[i].ContentItemID != nil {
			parentTitle = contentTitles[*comments[i].ContentItemID]
		} else if comments[i].DiscussionID != nil {
			parentTitle = discussionTitles[*comments[i].DiscussionID]
		}
		payloads = append(payloads, gin.H{
			"id":            comments[i].ID,
			"body":          comments[i].Body,
			"content_title": parentTitle,
			"status":        comments[i].Status,
			"created_at":    comments[i].CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"comments":  payloads,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *SocialHandler) ReportContent(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}
	contentID, ok := pathID(c, "id", "invalid content id")
	if !ok {
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
			response.Error(c, http.StatusConflict, "ALREADY_REPORTED", "you have already reported this content")
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
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}
	commentID, ok := pathID(c, "id", "invalid comment id")
	if !ok {
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
			response.Error(c, http.StatusConflict, "ALREADY_REPORTED", "you have already reported this comment")
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
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "login required")
		return
	}
	page, pageSize := pageQuery(c, 20)

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
