package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func setupFeedbackHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.FeedbackTicket{}, &model.FeedbackAttachment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	feedbackRepo := repository.NewFeedbackRepository(db)
	userRepo := repository.NewUserRepository(db)
	feedbackSvc := service.NewFeedbackService(feedbackRepo, userRepo, nil, nil, 300)
	feedbackHandler := NewFeedbackHandler(feedbackSvc)

	r := gin.New()
	r.POST("/feedback", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(0))
		feedbackHandler.SubmitTicket(c)
	})
	return r, db
}

func TestSubmitTicketTreatsUserIDZeroAsAnonymous(t *testing.T) {
	r, db := setupFeedbackHandlerTest(t)

	payload := map[string]any{
		"contact_email": "anon@example.com",
		"category":      "web_bug",
		"title":         "Anonymous feedback",
		"description":   "Anonymous feedback should not write user_id zero.",
		"captcha_token": "bypass-token",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var ticket model.FeedbackTicket
	if err := db.First(&ticket).Error; err != nil {
		t.Fatalf("load ticket: %v", err)
	}
	if ticket.UserID != nil {
		t.Fatalf("anonymous ticket user_id = %d, want NULL", *ticket.UserID)
	}
}
