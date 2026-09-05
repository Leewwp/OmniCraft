package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// #381：已轮换的旧 refresh token 在 grace 窗口内重放（并发换发 / 硬导航打断错过
// Set-Cookie 的标签页）必须返回 200 并重设同一新 cookie——绝不允许 401 + clear-cookie
// 把赢家刚轮换出的会话一并抹掉。
func TestRefreshReplayOfRotatedTokenReturns200AndRotatedCookie(t *testing.T) {
	r, _, db, _, mr := setupAuthCookieTestRouter(t)
	defer mr.Close()
	insertCookieTestUser(t, db, "replay-race@test.com", "replayraceuser", "password123")

	csrfToken := fetchCSRFToken(t, r)

	doLogin := func() string {
		t.Helper()
		body := `{"email":"replay-race@test.com","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("login: expected 200, got %d body=%s", w.Code, w.Body.String())
		}
		var refreshCookie string
		for _, c := range w.Result().Cookies() {
			if c.Name == "refresh_token" || c.Name == "__Host-refresh_token" {
				refreshCookie = c.Value
			}
		}
		if refreshCookie == "" {
			t.Fatal("login must set refresh cookie")
		}
		return refreshCookie
	}

	doRefresh := func(token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: token})
		req.AddCookie(&http.Cookie{Name: "csrf-token", Value: csrfToken})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 只取「设置型」refresh cookie（MaxAge>0）；清退型（MaxAge=-1）不算轮换成功
	rotatedCookie := func(t *testing.T, w *httptest.ResponseRecorder) string {
		t.Helper()
		var value string
		for _, c := range w.Result().Cookies() {
			if (c.Name == "refresh_token" || c.Name == "__Host-refresh_token") && c.MaxAge > 0 {
				value = c.Value
			}
		}
		return value
	}

	r1 := doLogin()

	w1 := doRefresh(r1)
	if w1.Code != http.StatusOK {
		t.Fatalf("refresh#1: expected 200, got %d body=%s", w1.Code, w1.Body.String())
	}
	r2 := rotatedCookie(t, w1)
	if r2 == "" || r2 == r1 {
		t.Fatalf("refresh#1 must rotate to a new cookie value, got %q", r2)
	}

	w2 := doRefresh(r1)
	if w2.Code != http.StatusOK {
		t.Fatalf("stale replay within grace: expected 200, got %d body=%s", w2.Code, w2.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &body); err != nil {
		t.Fatalf("replay body json: %v", err)
	}
	if tokens, ok := body["tokens"].(map[string]interface{}); !ok || tokens["access_token"] == "" {
		t.Fatal("replay response must contain access_token")
	}
	if got := rotatedCookie(t, w2); got != r2 {
		t.Fatalf("replay must re-set the same rotated cookie %q, got %q", r2, got)
	}
}
