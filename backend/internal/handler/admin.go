package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminHandler struct {
	ipSvc      *service.IPService
	contentRepo *repository.ContentRepository
	userRepo    *repository.UserRepository
	socialRepo  *repository.SocialRepository
	cfg        *config.Config
}

func NewAdminHandler(db *gorm.DB, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		ipSvc:       service.NewIPService(repository.NewIPRepository(db)),
		contentRepo: repository.NewContentRepository(db),
		userRepo:    repository.NewUserRepository(db),
		socialRepo:  repository.NewSocialRepository(db),
		cfg:         cfg,
	}
}

func (h *AdminHandler) ListPendingIPs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	ips, total, err := h.ipSvc.ListIPs(repository.ListIPsFilter{
		Status:   "pending",
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ips": ips, "total": total})
}

func (h *AdminHandler) ApproveIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	if err := h.ipSvc.ApproveIP(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ip approved"})
}

func (h *AdminHandler) RejectIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}
	if err := h.ipSvc.RejectIP(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ip rejected"})
}

func (h *AdminHandler) ListUnderReviewContents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	contentSvc := service.NewContentService(h.contentRepo)
	contents, total, err := contentSvc.ListContents(repository.ListContentsFilter{
		Status:   "under_review",
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contents": contents, "total": total})
}

func (h *AdminHandler) BanContent(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid content id"})
		return
	}
	if err := h.contentRepo.UpdateContent(id, map[string]interface{}{"status": "banned"}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "content banned"})
}

func (h *AdminHandler) BanUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := h.userRepo.UpdateFields(id, map[string]interface{}{"is_banned": true, "ban_reason": body.Reason}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user banned"})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var users []map[string]interface{}
	var total int64
	h.userRepo.DB().Model(&struct{ ID uint }{}).Table("users").Count(&total)
	h.userRepo.DB().Table("users").Select("id, username, email, role, is_banned, reputation, created_at").
		Offset((page-1)*pageSize).Limit(pageSize).Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total})
}

func (h *AdminHandler) ListAppeals(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var appeals []map[string]interface{}
	var total int64
	h.userRepo.DB().Model(&struct{ ID uint }{}).Table("appeals").Where("status = ?", "pending").Count(&total)
	h.userRepo.DB().Table("appeals").Where("status = ?", "pending").
		Offset((page-1)*pageSize).Limit(pageSize).Find(&appeals)
	c.JSON(http.StatusOK, gin.H{"appeals": appeals, "total": total})
}

func (h *AdminHandler) ResolveAppeal(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid appeal id"})
		return
	}
	var body struct {
		Status        string `json:"status" binding:"required,oneof=approved rejected"`
		AdminResponse string `json:"admin_response"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}
	updates := map[string]interface{}{
		"status":         body.Status,
		"admin_response": body.AdminResponse,
		"resolved_at":    gorm.Expr("NOW()"),
	}
	if err := h.userRepo.DB().Table("appeals").Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "appeal resolved"})
}

func (h *AdminHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"config": h.cfg})
}
