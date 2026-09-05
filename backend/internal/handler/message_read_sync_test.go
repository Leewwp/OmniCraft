package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// T36（FIX-30b）私信状态可信：
// ①读会话（ListMessages → UpdateLastRead）必须同事务把该会话对应 message
//   通知标记已读——铃铛未读与会话未读同步（双通道一致）；
// ②通知 body 必须是摘要「你有一条新私信」，不得泄露私信全文。
// 红线：SendWithColdStartGuard 语义不变（本文件不放宽、cold start 用例回归）。

func postMessageForReadSync(t *testing.T, router *gin.Engine, senderID, recipientID int64, text string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{"recipient_id": recipientID, "text": text})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(senderID, 10))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func listMessagesAs(t *testing.T, router *gin.Engine, userID, convID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/"+strconv.FormatInt(convID, 10), nil)
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(userID, 10))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func waitMessageNotification(t *testing.T, db *gorm.DB, recipientID int64) model.Notification {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var n model.Notification
	for time.Now().Before(deadline) {
		err := db.Where("user_id = ? AND type = ? AND target_type = ?", recipientID, "message", "message").
			Order("id DESC").First(&n).Error
		if err == nil {
			return n
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("message notification for user %d did not land before deadline", recipientID)
	return n
}

// ①已读联动：接收方打开会话后，该会话 message 通知应被标记已读。
func TestListMessagesMarksMessageNotificationsRead(t *testing.T) {
	router, db := setupMessageRouterWithOptions(t, nil, nil, true)

	rec := postMessageForReadSync(t, router, 1, 2, "请查收")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	n := waitMessageNotification(t, db, 2)
	require.False(t, n.IsRead, "fresh message notification must start unread")

	var conv model.Conversation
	require.NoError(t, db.First(&conv).Error)

	rec = listMessagesAs(t, router, 2, conv.ID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var stored model.Notification
	require.NoError(t, db.First(&stored, n.ID).Error)
	require.True(t, stored.IsRead, "reading the conversation must mark its message notifications read (bell ⇆ chat sync)")
}

// ②摘要通知：通知 body 不得携带私信全文。
func TestMessageNotificationBodyIsSummaryNotFullText(t *testing.T) {
	router, db := setupMessageRouterWithOptions(t, nil, nil, true)

	rec := postMessageForReadSync(t, router, 1, 2, "私信全文机密内容XYZ")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	n := waitMessageNotification(t, db, 2)
	require.NotNil(t, n.Body)
	require.NotContains(t, *n.Body, "机密内容XYZ", "notification body must not leak the DM full text")
	require.Equal(t, "你有一条新私信", *n.Body, "notification body should be the summary copy")
}
