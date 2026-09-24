package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// #669：错误信封收口的 wire 兼容窄 helper 契约——
// CodeOnly / CaptchaError 必须逐字节保持历史点位响应体，
// 且与 Error 一样使用 AbortWithStatusJSON（中止语义见 #669 账本：
// 非测试代码无 IsAborted 消费方）。

func runWithRecorder(t *testing.T, send func(c *gin.Context) bool) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	send(c)
	return w, c.IsAborted()
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not a JSON object: %v (raw=%s)", err, w.Body.String())
	}
	return body
}

func TestCodeOnlyPreservesHistoricalWire(t *testing.T) {
	w, aborted := runWithRecorder(t, func(c *gin.Context) bool {
		CodeOnly(c, http.StatusBadRequest, "INVALID_ID")
		return true
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	body := decodeBody(t, w)
	if len(body) != 1 || body["code"] != "INVALID_ID" {
		t.Fatalf("code-only body must be exactly {code}: got %v (raw=%s)", body, w.Body.String())
	}
	if !aborted {
		t.Fatal("CodeOnly must abort the context (AbortWithStatusJSON semantics)")
	}
}

func TestCaptchaErrorPreservesHistoricalWire(t *testing.T) {
	w, aborted := runWithRecorder(t, func(c *gin.Context) bool {
		CaptchaError(c, http.StatusBadRequest, "CAPTCHA_FAILED", "captcha verification failed")
		return true
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	body := decodeBody(t, w)
	if len(body) != 3 ||
		body["code"] != "CAPTCHA_FAILED" ||
		body["message"] != "captcha verification failed" ||
		body["captcha_result"] != false {
		t.Fatalf("captcha error body must be {code,message,captcha_result:false}: got %v (raw=%s)", body, w.Body.String())
	}
	if !aborted {
		t.Fatal("CaptchaError must abort the context")
	}
}

func TestErrorWireUnchangedForPlainEnvelope(t *testing.T) {
	// 迁移主体形态：{code,message} 两字段 + Abort——与历史 c.JSON 仅发送语义不同。
	w, aborted := runWithRecorder(t, func(c *gin.Context) bool {
		Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return true
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	body := decodeBody(t, w)
	if len(body) != 2 || body["code"] != "INVALID_CREDENTIALS" || body["message"] != "invalid email or password" {
		t.Fatalf("plain error body must be exactly {code,message}: got %v (raw=%s)", body, w.Body.String())
	}
	if !aborted {
		t.Fatal("Error must abort the context")
	}
}

func TestErrorWithDetailsWire(t *testing.T) {
	// details 形态（既有 helper，非 439 迁移面；此处锁 wire 供完整路由对照）。
	w, _ := runWithRecorder(t, func(c *gin.Context) bool {
		ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request", []string{"field"})
		return true
	})
	body := decodeBody(t, w)
	if len(body) != 3 || body["details"] == nil {
		t.Fatalf("details body must be {code,message,details}: got %v (raw=%s)", body, w.Body.String())
	}
}
