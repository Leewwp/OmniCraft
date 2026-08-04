package handler

import (
	"net/http"
	"strconv"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type RehabHandler struct {
	rehabSvc *service.RehabService
}

func NewRehabHandler(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *RehabHandler {
	return &RehabHandler{
		rehabSvc: service.NewRehabService(db, service.NewRuntimeStatusCache(rdb, cfg)),
	}
}

func (h *RehabHandler) ListCourses(c *gin.Context) {
	userID := middleware.GetUserID(c)
	locale := c.DefaultQuery("locale", "zh")
	courses, err := h.rehabSvc.GetAvailableCourses(userID, locale)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
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
	reputation, err := h.rehabSvc.CompleteCourse(userID, id)
	if err != nil {
		switch err {
		case service.ErrCourseNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "COURSE_NOT_FOUND", "message": "course not found"})
		case service.ErrAlreadyCompleted:
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_COMPLETED", "message": "course already completed"})
		case service.ErrCourseNotStarted:
			c.JSON(http.StatusBadRequest, gin.H{"code": "COURSE_NOT_STARTED", "message": "course has not been started"})
		case service.ErrReadingTooShort:
			c.JSON(http.StatusTooEarly, gin.H{"code": "READING_TIME_TOO_SHORT", "message": "minimum reading time has not elapsed"})
		case service.ErrStatusCacheUnavailable:
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": service.DenialReasonAuthStatusUnavailable, "message": "account status is temporarily unavailable"})
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "course completed", "reputation": reputation})
}

func (h *RehabHandler) StartCourse(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid course id"})
		return
	}
	userID := middleware.GetUserID(c)
	if err := h.rehabSvc.StartCourse(userID, id); err != nil {
		switch err {
		case service.ErrCourseNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "COURSE_NOT_FOUND", "message": "course not found"})
		case service.ErrAlreadyCompleted:
			c.JSON(http.StatusConflict, gin.H{"code": "ALREADY_COMPLETED", "message": "course already completed"})
		default:
			response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "course started"})
}

func (h *RehabHandler) GetMyProgress(c *gin.Context) {
	userID := middleware.GetUserID(c)
	courses, err := h.rehabSvc.GetMyProgress(userID)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"courses": courses})
}
