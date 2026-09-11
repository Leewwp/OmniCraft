package handler

// T40（FIX-36d）：判官内容预览的读豁免——持对应类型有效资格的判官可读
// under_review 内容（审案预览）；banned 照旧 404；类型不符/匿名不可读。

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
)

func setupT40VisibilityHandler(t *testing.T) *ContentHandler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// judge_qualifications 的 now() 列默认值在 sqlite 不可用，统一走 ephemeral postgres。
	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}, &model.JudgeQualification{}))
	cfg := &config.Config{}
	cfg.JWT.Secret = "t40-visibility-secret"
	return NewContentHandler(db, cfg, nil)
}

func seedT40Qualification(t *testing.T, h *ContentHandler, userID int64, contentType string) {
	t.Helper()
	require.NoError(t, h.judgeRepo.CreateQualification(userID, contentType))
}

func t40ViewerCtx(viewerID int64) *gin.Context {
	c, _ := gin.CreateTestContext(nil)
	if viewerID != 0 {
		c.Set(middleware.UserIDKey, viewerID)
	}
	return c
}

func t40Item(status, contentType string) *model.ContentItem {
	return &model.ContentItem{
		ID: 700, Title: "T40 preview", AuthorID: 9,
		Zone: "original", Category: "game", ContentType: contentType,
		Status: status, IsPublic: true, Author: model.User{ID: 9},
	}
}

func TestT40QualifiedJudgeReadsUnderReview(t *testing.T) {
	h := setupT40VisibilityHandler(t)
	seedT40Qualification(t, h, 4001, "article")

	require.True(t, h.contentVisibleToViewer(t40Item("under_review", "article"), t40ViewerCtx(4001)),
		"持 article 资格的判官可读 under_review 内容")
}

func TestT40UnqualifiedOrWrongTypeCannotReadUnderReview(t *testing.T) {
	h := setupT40VisibilityHandler(t)
	seedT40Qualification(t, h, 4002, "article")

	require.False(t, h.contentVisibleToViewer(t40Item("under_review", "image"), t40ViewerCtx(4002)),
		"类型不符的判官不可读")
	require.False(t, h.contentVisibleToViewer(t40Item("under_review", "article"), t40ViewerCtx(4003)),
		"无资格用户不可读")
	require.False(t, h.contentVisibleToViewer(t40Item("under_review", "article"), t40ViewerCtx(0)),
		"匿名不可读")
}

func TestT40JudgeExemptDoesNotLeakBanned(t *testing.T) {
	h := setupT40VisibilityHandler(t)
	seedT40Qualification(t, h, 4004, "article")

	require.False(t, h.contentVisibleToViewer(t40Item("banned", "article"), t40ViewerCtx(4004)),
		"banned 内容对判官仍不可见")
}
