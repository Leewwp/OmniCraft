package handler

import (
	"errors"
	"net/http"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DiscussionHandler struct {
	discRepo      *repository.DiscussionRepository
	socialRepo    *repository.SocialRepository
	ipRepo        *repository.IPRepository
	socialSvc     *service.SocialService
	displaySigner *service.DisplayURLSigner
	cfg           *config.Config
}

// NewDiscussionHandler takes the shared SocialService so discussion creation
// and replies go through the same reputation gate, Green text moderation and
// owner notification as /social routes (T12/FIX-18 route duality fix).
// NewDiscussionHandler receives the container-owned repositories plus the
// shared SocialService (#658 PR-3).
func NewDiscussionHandler(discRepo *repository.DiscussionRepository, socialRepo *repository.SocialRepository, ipRepo *repository.IPRepository, socialSvc *service.SocialService) *DiscussionHandler {
	return &DiscussionHandler{
		discRepo:   discRepo,
		socialRepo: socialRepo,
		ipRepo:     ipRepo,
		socialSvc:  socialSvc,
	}
}

// SetConfig wires runtime config for the hot-sort decay parameter (#290).
func (h *DiscussionHandler) SetConfig(cfg *config.Config) {
	h.cfg = cfg
}

func (h *DiscussionHandler) hotDecayHours() float64 {
	if h == nil || h.cfg == nil {
		return 72
	}
	return h.cfg.Discussion.EffectiveHotDecayHours()
}

// SetDisplayURLSigner wires display URL signing for discussion/comment author
// avatars and linked IP covers (B-002).
func (h *DiscussionHandler) SetDisplayURLSigner(signer *service.DisplayURLSigner) {
	h.displaySigner = signer
}

func (h *DiscussionHandler) ListDiscussions(c *gin.Context) {
	ipID, ok := pathID(c, "id", "invalid ip id")
	if !ok {
		return
	}
	if !h.ipDiscussionsVisible(c, ipID) {
		response.Error(c, http.StatusNotFound, "IP_NOT_FOUND", "ip not found")
		return
	}
	sort := c.DefaultQuery("sort", "latest_reply")
	query := c.Query("q")
	page, pageSize := pageQuery(c, 20)

	decay := h.hotDecayHours()
	discussions, total, err := h.discRepo.ListByIP(ipID, sort, query, decay, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateDiscussions(discussions)
	c.JSON(http.StatusOK, gin.H{"discussions": discussions, "total": total})
}

func (h *DiscussionHandler) CreateDiscussion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	ipID, ok := pathID(c, "id", "invalid ip id")
	if !ok {
		return
	}

	var body struct {
		Title string `json:"title" binding:"required"`
		Body  string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	// T12（FIX-18）：改走 SocialService.PostDiscussion——信誉门 + Green 审核
	// 与 /social/discussions 同一套治理（此前直写 repo 绕过双门）。
	d, err := h.socialSvc.PostDiscussion(c.Request.Context(), service.PostDiscussionInput{
		IPID:  &ipID,
		Title: body.Title,
		Body:  body.Body,
	}, callerID)
	if err != nil {
		h.respondSocialServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"discussion": d})
}

func (h *DiscussionHandler) GetDiscussion(c *gin.Context) {
	id, ok := pathID(c, "id", "invalid discussion id")
	if !ok {
		return
	}
	page, pageSize := pageQuery(c, 20)

	d, err := h.discRepo.GetByID(id)
	if err != nil || d == nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "discussion not found")
		return
	}
	// T12/F-106 顺带收口：未发布（under_review/hidden）讨论不透出详情；
	// admin 置顶走 PinDiscussion（不经此读路径），不受影响。
	if d.Status != "published" {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "discussion not found")
		return
	}
	// run-1 审计 #4：published 讨论还须过父可见性闸——挂非公开内容/未过审
	// IP 下的讨论不可按 ID 透出（与列表路径 ContentVisibilitySQL/
	// ipDiscussionsVisible 同口径）。
	if !discussionParentsVisible(c, h.socialRepo.DB(), d) {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "discussion not found")
		return
	}

	comments, total, err := h.socialRepo.ListCommentsByTarget("discussion", id, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	// T46（FIX-29b）：随顶层一页带回其子回复（两级展示），total 仍计顶层。
	parentIDs := make([]int64, 0, len(comments))
	for _, comment := range comments {
		parentIDs = append(parentIDs, comment.ID)
	}
	children, err := h.socialRepo.ListCommentsByParentIDs(parentIDs)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	comments = append(comments, children...)
	h.displaySigner.DecorateDiscussion(d)
	h.displaySigner.DecorateComments(comments)
	c.JSON(http.StatusOK, gin.H{"discussion": d, "comments": comments, "total": total})
}

func (h *DiscussionHandler) ReplyToDiscussion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	discID, ok := pathID(c, "id", "invalid discussion id")
	if !ok {
		return
	}

	// 前端 ReplyList 发 content（service 输入是 body）：保留本地 binding 做
	// 字段映射（T12 前提①），否则全站讨论回复 400。
	var body struct {
		Content  string `json:"content" binding:"required"`
		ParentID *int64 `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	// T12（FIX-18）：改走 SocialService.PostComment——信誉门 + Green 审核 +
	// 楼主通知一并复用；reply_count/last_active_at 由 service 的 discussion
	// 分支统一递增（一条评论只计一次，handler 不再手动 IncrementReplyCount）。
	comment, err := h.socialSvc.PostComment(c.Request.Context(), service.PostCommentInput{
		DiscussionID: &discID,
		ParentID:     body.ParentID,
		Body:         body.Content,
	}, callerID)
	if err != nil {
		h.respondSocialServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

// respondSocialServiceError maps SocialService domain errors to the same HTTP
// contract SocialHandler uses, so both discussion write paths (/ips and
// /social) answer with identical status codes and error codes.
func (h *DiscussionHandler) respondSocialServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrLowReputation):
		response.Forbidden(c, "reputation score too low to perform this action")
	case errors.Is(err, service.ErrTextBlocked):
		response.Error(c, http.StatusUnprocessableEntity, "CONTENT_BLOCKED", "content was rejected by content moderation")
	case errors.Is(err, service.ErrModerationUnavailable):
		response.Error(c, http.StatusServiceUnavailable, "MODERATION_UNAVAILABLE", "content moderation is temporarily unavailable, please try again later")
	case errors.Is(err, service.ErrDiscussionTargetInvalid):
		// run-1 审计 #4：挂非公开父/未知目标 → 404（与 /social 路径同口径）。
		response.Error(c, http.StatusNotFound, "DISCUSSION_TARGET_INVALID", "discussion target not found or not visible")
	default:
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
	}
}

// PinDiscussion is admin-only (#290): the pin right moved from the IP creator
// to system administrators so community self-governance cannot be abused by a
// single point. The AdminRequired middleware enforces the role.
func (h *DiscussionHandler) PinDiscussion(c *gin.Context) {
	id, ok := pathID(c, "id", "invalid discussion id")
	if !ok {
		return
	}

	d, err := h.discRepo.GetByID(id)
	if err != nil || d == nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "discussion not found")
		return
	}

	var body struct {
		Pinned bool `json:"pinned"`
	}
	c.ShouldBindJSON(&body)
	if err := h.discRepo.Pin(id, body.Pinned); err != nil {
		response.CodeOnly(c, http.StatusInternalServerError, "DB_ERROR")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *DiscussionHandler) SearchDiscussions(c *gin.Context) {
	ipID, ok := pathID(c, "id", "invalid ip id")
	if !ok {
		return
	}
	if !h.ipDiscussionsVisible(c, ipID) {
		response.Error(c, http.StatusNotFound, "IP_NOT_FOUND", "ip not found")
		return
	}
	keyword := c.Query("q")
	page, pageSize := pageQuery(c, 20)

	discussions, err := h.discRepo.SearchByKeyword(ipID, keyword, page, pageSize)
	if err != nil {
		response.CodeOnly(c, http.StatusInternalServerError, "DB_ERROR")
		return
	}
	h.displaySigner.DecorateDiscussions(discussions)
	c.JSON(http.StatusOK, gin.H{"discussions": discussions})
}

func (h *DiscussionHandler) ListByUser(c *gin.Context) {
	userID, ok := pathID(c, "id", "invalid user id")
	if !ok {
		return
	}
	page, pageSize := pageQuery(c, 20)

	discussions, total, err := h.discRepo.ListByUser(userID, page, pageSize, middleware.GetUserID(c))
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateDiscussions(discussions)
	c.JSON(http.StatusOK, gin.H{"discussions": discussions, "total": total})
}

// ipDiscussionsVisible gates the ip-scoped discussion surfaces behind IP
// visibility (#446 / SP-16 P0): discussions of a non-approved IP are only
// for its creator (and admins).
func (h *DiscussionHandler) ipDiscussionsVisible(c *gin.Context, ipID int64) bool {
	ip, err := h.ipRepo.FindByID(ipID)
	if err != nil || ip == nil {
		return false
	}
	return ipVisibleToViewer(c, ip)
}

// discussionParentsVisible gates a discussion detail behind its parent
// visibility (run-1 audit #4): a published discussion hanging under a
// non-public content item or a non-approved IP must not surface by id. The
// content side reuses ContentVisibleToViewer (author-or-public, same scope
// as the list paths); the IP side reuses ipVisibleToViewer (approved, or
// creator, or admin — same as the ip-scoped discussion surfaces).
func discussionParentsVisible(c *gin.Context, db *gorm.DB, d *model.Discussion) bool {
	if d == nil {
		return false
	}
	viewerID := middleware.GetUserID(c)
	if d.ContentItemID != nil {
		var content model.ContentItem
		if err := db.First(&content, *d.ContentItemID).Error; err != nil {
			return false
		}
		if !repository.ContentVisibleToViewer(db, &content, viewerID) {
			return false
		}
	}
	if d.IPID != nil {
		var ip model.IP
		if err := db.First(&ip, *d.IPID).Error; err != nil {
			return false
		}
		if !ipVisibleToViewer(c, &ip) {
			return false
		}
	}
	return true
}
