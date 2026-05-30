package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DiscussionHandler struct {
	discRepo   *repository.DiscussionRepository
	socialRepo *repository.SocialRepository
	ipRepo     *repository.IPRepository
}

func NewDiscussionHandler(db *gorm.DB) *DiscussionHandler {
	return &DiscussionHandler{
		discRepo:   repository.NewDiscussionRepository(db),
		socialRepo: repository.NewSocialRepository(db),
		ipRepo:     repository.NewIPRepository(db),
	}
}

func (h *DiscussionHandler) ListDiscussions(c *gin.Context) {
	ipID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	sort := c.DefaultQuery("sort", "latest_reply")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	discussions, total, err := h.discRepo.ListByIP(ipID, sort, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"discussions": discussions, "total": total})
}

func (h *DiscussionHandler) CreateDiscussion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	ipID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var body struct {
		Title string `json:"title" binding:"required"`
		Body  string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	d := &model.Discussion{
		IPID:     &ipID,
		AuthorID: callerID,
		Title:    body.Title,
		Body:     body.Body,
	}
	if err := h.discRepo.Create(d); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"discussion": d})
}

func (h *DiscussionHandler) GetDiscussion(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	d, err := h.discRepo.GetByID(id)
	if err != nil || d == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "discussion not found"})
		return
	}

	comments, total, err := h.socialRepo.ListCommentsByTarget("discussion", id, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"discussion": d, "comments": comments, "total": total})
}

func (h *DiscussionHandler) ReplyToDiscussion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	discID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var body struct {
		Content  string `json:"content" binding:"required"`
		ParentID *int64 `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	comment := &model.Comment{
		DiscussionID: &discID,
		TargetType:   "discussion",
		TargetID:     discID,
		AuthorID:     callerID,
		Body:         body.Content,
		ParentID:     body.ParentID,
	}
	if err := h.socialRepo.CreateComment(comment); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.discRepo.IncrementReplyCount(discID)
	c.JSON(http.StatusCreated, gin.H{"comment": comment})
}

func (h *DiscussionHandler) PinDiscussion(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	d, err := h.discRepo.GetByID(id)
	if err != nil || d == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "discussion not found"})
		return
	}

	var ipIDVal int64
	if d.IPID != nil {
		ipIDVal = *d.IPID
	}
	ip, _ := h.ipRepo.FindByID(ipIDVal)
	if ip == nil || (ip.CreatorID == nil || *ip.CreatorID != callerID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN"})
		return
	}

	var body struct {
		Pinned bool `json:"pinned"`
	}
	c.ShouldBindJSON(&body)
	if err := h.discRepo.Pin(id, body.Pinned); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *DiscussionHandler) SearchDiscussions(c *gin.Context) {
	ipID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	keyword := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	discussions, err := h.discRepo.SearchByKeyword(ipID, keyword, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"discussions": discussions})
}

func (h *DiscussionHandler) ListByUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	discussions, total, err := h.discRepo.ListByUser(userID, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"discussions": discussions, "total": total})
}
