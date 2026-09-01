package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// FIX-31 首项（T04）：FollowIP 曾向 user_id=0 发幽灵广播通知（无收件人，
// 物化为演示噪音）。修复后关注 IP 不得产生任何通知行；关注用户的真人通知保留。
func TestFollowIPDoesNotEmitGhostNotification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:followipghost?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Notification{}, &model.IP{}, &model.Follow{}))

	follower := &model.User{Email: "f@t.local", Username: "follower", PasswordHash: "x", Reputation: 10}
	require.NoError(t, db.Create(follower).Error)

	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db))
	h := NewFollowHandler(db)
	h.SetNotificationService(notifSvc)

	r := gin.New()
	r.POST("/ips/:id/follow", func(c *gin.Context) { c.Set("userID", follower.ID); c.Next() }, h.FollowIP)

	req := httptest.NewRequest(http.MethodPost, "/ips/210/follow", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// the Noop-producer path writes synchronously enough for a short poll; the
	// assertion is that no ghost row EVER appears.
	var count int64
	for i := 0; i < 10; i++ {
		require.NoError(t, db.Model(&model.Notification{}).Count(&count).Error)
		if count > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Equal(t, int64(0), count, "FollowIP must not create notifications (was a user_id=0 broadcast)")
}

func TestFollowUserStillNotifiesTheRealTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:followuserreal?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Notification{}, &model.Follow{}))

	follower := &model.User{Email: "f2@t.local", Username: "follower2", PasswordHash: "x", Reputation: 10}
	target := &model.User{Email: "t2@t.local", Username: "target2", PasswordHash: "x", Reputation: 10}
	require.NoError(t, db.Create(follower).Error)
	require.NoError(t, db.Create(target).Error)

	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db))
	h := NewFollowHandler(db)
	h.SetNotificationService(notifSvc)

	r := gin.New()
	r.POST("/users/:id/follow", func(c *gin.Context) { c.Set("userID", follower.ID); c.Next() }, h.FollowUser)

	req := httptest.NewRequest(http.MethodPost, "/users/"+itoa64(target.ID)+"/follow", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var notifs []model.Notification
	for i := 0; i < 10 && len(notifs) == 0; i++ {
		require.NoError(t, db.Find(&notifs).Error)
		time.Sleep(100 * time.Millisecond)
	}
	require.Len(t, notifs, 1, "real follow notification must still land")
	require.Equal(t, target.ID, notifs[0].UserID)
}
