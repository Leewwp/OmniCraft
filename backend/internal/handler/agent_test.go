package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

func TestAgentConversationEndpointsRespectFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentConversation{}, &model.AgentMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	handler := NewAgentHandler(db, &config.Config{Agent: config.AgentConfig{WebAgentEnabled: false}})
	router := gin.New()
	router.GET("/agent/conversations", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ListConversations(c)
	})
	router.GET("/agent/conversations/:id", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.GetConversationMessages(c)
	})

	for _, path := range []string{"/agent/conversations", "/agent/conversations/1"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d, want 503; body = %s", path, rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); !strings.Contains(body, "FEATURE_DISABLED") {
			t.Fatalf("%s body missing FEATURE_DISABLED: %s", path, body)
		}
	}
}
