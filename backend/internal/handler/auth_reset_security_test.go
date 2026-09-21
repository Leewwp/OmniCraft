package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"omnicraft/backend/internal/model"
)

// FR-01（中-5）：重置成功后，攻击者已持有的旧 refresh token 必须立即失效。
// 行为链：登录拿到 refresh → 重置密码 → 旧 refresh 换发必须 401。
func TestResetPasswordRevokesExistingRefreshTokens(t *testing.T) {
	r, _, db, rdb, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	user := insertCookieTestUser(t, db, "resetrevoke@test.com", "resetrevokeuser", "password123")

	csrfToken := fetchCSRFToken(t, r)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"resetrevoke@test.com","password":"password123"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-CSRF-Token", csrfToken)
	loginReq.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d body=%s", loginRec.Code, loginRec.Body.String())
	}

	var oldRefresh string
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "refresh_token" {
			oldRefresh = c.Value
		}
	}
	if oldRefresh == "" {
		t.Fatal("login must set refresh_token cookie")
	}

	rawToken := "reset-token-revokes-old-sessions"
	sum := sha256.Sum256([]byte(rawToken))
	digest := hex.EncodeToString(sum[:])
	ctx := context.Background()
	if err := rdb.Set(ctx, "reset:password:"+digest, user.ID, 0).Err(); err != nil {
		t.Fatalf("seed reset token: %v", err)
	}
	if err := rdb.Set(ctx, "reset:password:user:"+strconv.FormatInt(user.ID, 10), digest, 0).Err(); err != nil {
		t.Fatalf("seed reset digest: %v", err)
	}

	resetCSRF := fetchCSRFToken(t, r)
	body := `{"token":"` + rawToken + `","new_password":"newpassword123"}`
	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	resetReq.Header.Set("Content-Type", "application/json")
	resetReq.Header.Set("X-CSRF-Token", resetCSRF)
	resetReq.AddCookie(&http.Cookie{Name: "csrf-token", Value: resetCSRF})
	resetRec := httptest.NewRecorder()
	r.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset password: expected 200, got %d body=%s", resetRec.Code, resetRec.Body.String())
	}

	refreshCSRF := fetchCSRFToken(t, r)
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshReq.Header.Set("X-CSRF-Token", refreshCSRF)
	refreshReq.AddCookie(&http.Cookie{Name: "csrf-token", Value: refreshCSRF})
	refreshReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: oldRefresh})
	refreshRec := httptest.NewRecorder()
	r.ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusUnauthorized {
		t.Fatalf("old refresh token must be revoked after password reset, got %d body=%s", refreshRec.Code, refreshRec.Body.String())
	}
}

// FR-01（低-17）：73+ 字节新密码必须得到 400，且不得把 password_hash 写空。
// 行为链：注入合法 reset token → 超长新密码 → 400 + 旧哈希保留。
func TestResetPasswordRejectsOverlongPassword(t *testing.T) {
	r, _, db, rdb, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	user := insertCookieTestUser(t, db, "resetlong@test.com", "resetlonguser", "password123")

	rawToken := "reset-token-overlong-password"
	sum := sha256.Sum256([]byte(rawToken))
	digest := hex.EncodeToString(sum[:])
	ctx := context.Background()
	if err := rdb.Set(ctx, "reset:password:"+digest, user.ID, 0).Err(); err != nil {
		t.Fatalf("seed reset token: %v", err)
	}
	if err := rdb.Set(ctx, "reset:password:user:"+strconv.FormatInt(user.ID, 10), digest, 0).Err(); err != nil {
		t.Fatalf("seed reset digest: %v", err)
	}

	resetCSRF := fetchCSRFToken(t, r)
	overlong := strings.Repeat("a", 73)
	body := `{"token":"` + rawToken + `","new_password":"` + overlong + `"}`
	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	resetReq.Header.Set("Content-Type", "application/json")
	resetReq.Header.Set("X-CSRF-Token", resetCSRF)
	resetReq.AddCookie(&http.Cookie{Name: "csrf-token", Value: resetCSRF})
	resetRec := httptest.NewRecorder()
	r.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusBadRequest {
		t.Fatalf("overlong new password must return 400, got %d body=%s", resetRec.Code, resetRec.Body.String())
	}

	var reloaded model.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.PasswordHash == "" {
		t.Fatal("password_hash must never be written as empty string")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(reloaded.PasswordHash), []byte("password123")); err != nil {
		t.Fatalf("original password must remain intact after rejected reset: %v", err)
	}
}
