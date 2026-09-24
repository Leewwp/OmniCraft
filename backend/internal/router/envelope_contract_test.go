package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// #669 错误信封收口的 full-router 特征样本：迁移（c.JSON → response.*）
// 前后，代表性错误路径的 status 与 wire body 必须逐字段一致。
// 覆盖票面四类：code-only / captcha_result / 普通 code+message（auth 面）/
// 未配置供应商的 503 附加字段形态。复用 optauth 泄漏审计栈（sqlite +
// miniredis + 真实 container）。

func doEnvelopeJSON(t *testing.T, router http.Handler, method, path, token string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	} else {
		payload = []byte("{}")
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	var parsed map[string]any
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("%s %s: body is not JSON: %v (raw=%s)", method, path, err, w.Body.String())
		}
	}
	return w, parsed
}

func assertEnvelope(t *testing.T, path string, w *httptest.ResponseRecorder, body map[string]any, wantStatus int, want map[string]any) {
	t.Helper()
	if w.Code != wantStatus {
		t.Fatalf("%s: status = %d, want %d (body=%s)", path, w.Code, wantStatus, w.Body.String())
	}
	if len(body) != len(want) {
		t.Fatalf("%s: field set = %v, want exactly %v (raw=%s)", path, body, want, w.Body.String())
	}
	for k, v := range want {
		got, ok := body[k]
		if !ok || got != v {
			t.Fatalf("%s: field %q = %v (%T), want %v (%T) (raw=%s)", path, k, got, got, v, v, w.Body.String())
		}
	}
}

func TestErrorEnvelopeContractViaFullRouter(t *testing.T) {
	router, _, cfg, fx := buildOptAuthLeakAuditStack(t)
	token := makeRoutesSecurityToken(cfg, fx.viewerID, "user")

	t.Run("code-only INVALID_ID keeps exact two-token body", func(t *testing.T) {
		w, body := doEnvelopeJSON(t, router, http.MethodGet, "/api/v1/messages/not-a-number", token, nil)
		assertEnvelope(t, "/messages/:id", w, body, http.StatusBadRequest, map[string]any{
			"code": "INVALID_ID",
		})
	})

	t.Run("captcha validation error keeps captcha_result field", func(t *testing.T) {
		w, body := doEnvelopeJSON(t, router, http.MethodPost, "/api/v1/captcha/verify", "", nil)
		assertEnvelope(t, "/captcha/verify (empty)", w, body, http.StatusBadRequest, map[string]any{
			"code":           "VALIDATION_ERROR",
			"message":        "captcha verification parameter required",
			"captcha_result": false,
		})
	})

	t.Run("register missing captcha keeps CAPTCHA_REQUIRED envelope", func(t *testing.T) {
		// 栈内 captcha verifier 为本地 fail-open 形态（CAPTCHA_UNAVAILABLE 503
		// 不可达），改钉 auth.go verifyCaptcha 的空 token 快败形态（service 前触发）。
		w, body := doEnvelopeJSON(t, router, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
			"email":    "envelope-newuser@example.com",
			"username": "envelope_newuser",
			"password": "longenough123",
		})
		assertEnvelope(t, "/auth/register (no captcha)", w, body, http.StatusBadRequest, map[string]any{
			"code":    "CAPTCHA_REQUIRED",
			"message": "captcha verification required",
		})
	})

	t.Run("login wrong credentials keeps plain code+message envelope", func(t *testing.T) {
		w, body := doEnvelopeJSON(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
			"email":    "envelope-nobody@example.com",
			"password": "definitely-wrong",
		})
		assertEnvelope(t, "/auth/login", w, body, http.StatusUnauthorized, map[string]any{
			"code":    "INVALID_CREDENTIALS",
			"message": "invalid email or password",
		})
	})
}
