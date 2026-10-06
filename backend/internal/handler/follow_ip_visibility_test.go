package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// run-1 审计 #6：FollowIP 的目标校验（FollowTargetStatus）此前对任意存在
// IP 放行，为不可见 IP 供给投票资格前置（follow 是提案投票门槛）。修复后
// 非 approved IP 仅其创建者可关注——隐藏 IP 不再确认存在。

func setupFollowIPVisibilityRouter(t *testing.T, dbName string) (*gin.Engine, *gorm.DB, int64, int64, int64, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.Follow{}))

	creator := &model.User{Email: "fv-creator@t.local", Username: "fvcreator", PasswordHash: "x", Reputation: 10}
	outsider := &model.User{Email: "fv-outsider@t.local", Username: "fvoutsider", PasswordHash: "x", Reputation: 10}
	require.NoError(t, db.Create(creator).Error)
	require.NoError(t, db.Create(outsider).Error)

	require.NoError(t, db.Create(&model.IP{Name: "fv-pending", Slug: "fv-pending", Category: "game", Status: "pending", CreatorID: &creator.ID}).Error)
	require.NoError(t, db.Create(&model.IP{Name: "fv-approved", Slug: "fv-approved", Category: "game", Status: "approved", CreatorID: &creator.ID}).Error)

	var pendingID, approvedID int64
	require.NoError(t, db.Model(&model.IP{}).Where("slug = ?", "fv-pending").Pluck("id", &pendingID).Error)
	require.NoError(t, db.Model(&model.IP{}).Where("slug = ?", "fv-approved").Pluck("id", &approvedID).Error)

	h := NewFollowHandler(repository.NewFollowRepository(db))
	r := gin.New()
	register := func(path string, userID int64) {
		r.POST(path, func(c *gin.Context) {
			c.Set("userID", userID)
			c.Next()
		}, h.FollowIP)
	}
	register("/pending/:id", outsider.ID)
	register("/pending-creator/:id", creator.ID)
	register("/approved/:id", outsider.ID)
	return r, db, outsider.ID, creator.ID, pendingID, approvedID
}

func TestFollowIPHiddenIPNotConfirmedForNonCreator(t *testing.T) {
	r, db, outsiderID, creatorID, pendingID, _ := setupFollowIPVisibilityRouter(t, "followipvis-hidden")

	req := httptest.NewRequest(http.MethodPost, "/pending/"+itoa64(pendingID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "IP_NOT_FOUND")

	var followCount int64
	require.NoError(t, db.Model(&model.Follow{}).Where("follower_id = ?", outsiderID).Count(&followCount).Error)
	require.Equal(t, int64(0), followCount, "no follow row may land on a hidden ip")

	// 创建者保留 studio 自助路径（自己的 pending IP 可关注）。
	req = httptest.NewRequest(http.MethodPost, "/pending-creator/"+itoa64(pendingID), nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_ = creatorID
}

func TestFollowIPApprovedIPOpenToEveryone(t *testing.T) {
	r, _, _, _, _, approvedID := setupFollowIPVisibilityRouter(t, "followipvis-approved")

	req := httptest.NewRequest(http.MethodPost, "/approved/"+itoa64(approvedID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
