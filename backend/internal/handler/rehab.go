package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RehabHandler struct {
	rehabSvc *service.RehabService
}

func NewRehabHandler(db *gorm.DB) *RehabHandler {
	return &RehabHandler{
		rehabSvc: service.NewRehabService(db),
	}
}

func (h *RehabHandler) ListCourses(c *gin.Context) {
	userID := middleware.GetUserID(c)
	locale := c.DefaultQuery("locale", "zh")
	courses, err := h.rehabSvc.GetAvailableCourses(userID, locale)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"courses": courses})
}

func (h *RehabHandler) GetCourse(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid course id"})
		return
	}
	locale := c.DefaultQuery("locale", "zh")
	course, err := h.rehabSvc.GetCourseDetail(id, locale)
	if err != nil {
		if err == service.ErrCourseNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "COURSE_NOT_FOUND", "message": "course not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"course": course})
}

func (h *RehabHandler) CompleteCourse(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid course id"})
		return
	}
	userID := middleware.GetUserID(c)
	if err := h.rehabSvc.CompleteCourse(userID, id); err != nil {
		switch err {
		case service.ErrCourseNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "COURSE_NOT_FOUND", "message": "course not found"})
		case service.ErrAlreadyCompleted:
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_COMPLETED", "message": "course already completed"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "course completed"})
}

func (h *RehabHandler) GetMyProgress(c *gin.Context) {
	userID := middleware.GetUserID(c)
	courses, err := h.rehabSvc.GetMyProgress(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"courses": courses})
}
