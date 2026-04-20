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

type IPHandler struct {
	ipSvc *service.IPService
}

func NewIPHandler(db *gorm.DB) *IPHandler {
	return &IPHandler{
		ipSvc: service.NewIPService(repository.NewIPRepository(db)),
	}
}

func (h *IPHandler) ListIPs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := repository.ListIPsFilter{
		Search:   c.Query("q"),
		Category: c.Query("category"),
		Page:     page,
		PageSize: pageSize,
	}

	ips, total, err := h.ipSvc.ListIPs(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ips":       ips,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *IPHandler) CreateIP(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "login required"})
		return
	}

	var input service.CreateIPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	ip, err := h.ipSvc.CreateIP(input, callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"ip": ip})
}

func (h *IPHandler) GetIP(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}

	ip, err := h.ipSvc.GetIP(id)
	if err != nil {
		if err == service.ErrIPNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "IP_NOT_FOUND", "message": "ip not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ip": ip})
}

func (h *IPHandler) GetIPContents(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
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

	_ = id
	_ = page
	_ = pageSize
	c.JSON(http.StatusOK, gin.H{"contents": []interface{}{}, "total": 0, "page": page, "page_size": pageSize})
}

func (h *IPHandler) GetIPDiscussions(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid ip id"})
		return
	}

	_ = id
	c.JSON(http.StatusOK, gin.H{"discussions": []interface{}{}, "total": 0})
}
