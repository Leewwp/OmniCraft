package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// failNextQuery makes every SELECT through the shared gorm.DB fail with a
// scripted error while writes (Updates) keep succeeding, so a handler test can
// pin the readback-after-write error path deterministically.
func failNextQuery(t *testing.T, db *gorm.DB, err error) {
	t.Helper()
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_readback", func(tx *gorm.DB) {
		tx.AddError(err)
	}))
}

// 体检 A11（#949）：UpdateUser 成功落库后的回读 FindByID 失败不得吞错——
// 空对象直传投影会 panic，且客户端拿到的是无信封 500。期望：统一 500
// DB_ERROR 信封，且变更本身已持久化（回读失败不等于更新失败）。
func TestUpdateUserReadbackFailureReturnsDBError(t *testing.T) {
	router, db := setupUserUpdateTest(t, "debug", "https://cdn.example.test", nil)
	userID := createUserUpdateTestUser(t, db)

	rec := patchUserJSON(t, router, userID, `{"bio":"readback probe"}`)
	require.Equal(t, http.StatusOK, rec.Code, "sanity: the update succeeds before the fault is injected")

	failNextQuery(t, db, errors.New("readback boom"))

	rec = patchUserJSON(t, router, userID, `{"bio":"readback probe 2"}`)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "DB_ERROR", body["code"])
	require.NotContains(t, rec.Body.String(), "readback boom", "raw DB error must not reach the client")

	require.NoError(t, db.Callback().Query().Remove("test:fail_readback"))
	var stored model.User
	require.NoError(t, db.Where("id = ?", userID).First(&stored).Error)
	require.Equal(t, "readback probe 2", stored.Bio, "the update itself must persist when only the readback fails")
}

// 体检 A11（#949）：publicUser 对 nil 入参返回安全空壳而非 panic——任何上游
// 回读路径的残留 nil 都可能命中投影，防线在投影函数自身。
func TestPublicUserNilReturnsEmptyShell(t *testing.T) {
	h := &UserHandler{}
	resp := h.publicUser(nil)
	require.NotNil(t, resp)
	require.Empty(t, resp, "the nil shell must not leak any projection field")
}
