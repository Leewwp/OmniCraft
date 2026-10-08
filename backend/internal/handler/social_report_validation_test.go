package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// #844：举报 reason ≤100 runes 在绑定层 400（VALIDATION_ERROR），不再打穿为
// DB varchar(100) 500。validator 的 max 对字符串按 rune 计数，与 PostgreSQL
// varchar(100) 字符语义一致——101 个三字节汉字同样被拦。
func TestReportContentRejectsOverlongReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Report{}))

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.POST("/contents/:id/report", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ReportContent(c)
	})

	body := `{"reason":"` + strings.Repeat("超", 101) + `"}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/contents/10/report", strings.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "VALIDATION_ERROR")
}

func TestReportCommentRejectsOverlongReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Report{}))

	handler := NewSocialHandlerWithService(nil, db)
	router := gin.New()
	router.POST("/social/comments/:id/report", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ReportComment(c)
	})

	body := `{"reason":"` + strings.Repeat("x", 101) + `"}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/social/comments/20/report", strings.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "VALIDATION_ERROR")
}

// 恰好 100 runes 的 reason 必须仍可提交（边界不误伤）。
func TestReportCommentAcceptsExactHundredRunes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Report{}))

	socialSvc := service.NewSocialService(
		repository.NewSocialRepository(db), nil, nil, nil, nil,
	)
	handler := NewSocialHandlerWithService(socialSvc, db)
	router := gin.New()
	router.POST("/social/comments/:id/report", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		handler.ReportComment(c)
	})

	body := `{"reason":"` + strings.Repeat("举", 100) + `"}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/social/comments/20/report", strings.NewReader(body)))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var stored model.Report
	require.NoError(t, db.Where("reporter_id = ? AND target_type = ?", int64(1), "comment").First(&stored).Error)
	require.Equal(t, 100, len([]rune(stored.Reason)))
}
