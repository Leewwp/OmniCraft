package handler

// errmap 引擎与 API 层合同测试（端点契约表在各自 handler 侧测试文件，
// 如 content_errmap_test.go / feedback_errmap_test.go）。本文件只用临时
// 构造的小 map 验证写法语义：message 固定文案 vs SafeErrorResponse 分
// 流、fmt.Errorf %w 包装、多 sentinel 组合按表序首命中、兜底三件套
// （Safe / fallbackMessage / fallbackLog）与规则级 before 的副作用顺
// 序（副作用先于响应）。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var (
	errEngineSentinelA = errors.New("ENGINE_A")
	errEngineSentinelB = errors.New("ENGINE_B")
)

// envelope builds the expected JSON body literal for assertErrMapResponse.
func envelope(code, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

// assertErrMapResponse drives one errmap write through a fresh gin context
// and asserts the exact status plus the complete JSON envelope.
func assertErrMapResponse(t *testing.T, m endpointErrMap, err error, wantStatus int, wantBody map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	m.write(c, err)

	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if len(body) != len(wantBody) {
		t.Fatalf("body = %v, want exactly the envelope %v (no extra fields)", body, wantBody)
	}
	for key, want := range wantBody {
		if got := body[key]; got != want {
			t.Fatalf("body[%q] = %#v, want %#v; full body = %v", key, got, want, body)
		}
	}
}

// assertErrMapRecorder asserts the exact status plus the complete JSON
// envelope on an already-written recorder (no re-drive, so a probe-installed
// default logger observes exactly one write).
func assertErrMapRecorder(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantBody map[string]any) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if len(body) != len(wantBody) {
		t.Fatalf("body = %v, want exactly the envelope %v (no extra fields)", body, wantBody)
	}
	for key, want := range wantBody {
		if got := body[key]; got != want {
			t.Fatalf("body[%q] = %#v, want %#v; full body = %v", key, got, want, body)
		}
	}
}

// TestErrMap_RuleMessageSplit：message 非空走 response.Error 固定文案，
// 空 message 走 SafeErrorResponse 按 code 取安全文案；%w 包装下同规则
// 生效。
func TestErrMap_RuleMessageSplit(t *testing.T) {
	m := endpointErrMap{
		rules: []errorRule{
			{sentinel: errEngineSentinelA, status: http.StatusBadRequest, code: "FIXED_TEXT", message: "fixed safe text"},
			{sentinel: errEngineSentinelB, status: http.StatusForbidden, code: "SAFE_BRANCH"},
		},
		fallback: fallbackInternal,
	}

	assertErrMapResponse(t, m, errEngineSentinelA, http.StatusBadRequest,
		envelope("FIXED_TEXT", "fixed safe text"))
	assertErrMapResponse(t, m, errEngineSentinelB, http.StatusForbidden,
		envelope("SAFE_BRANCH", "an unexpected error occurred, please try again later"))

	wrapped := fmt.Errorf("engine wrap: %w", errEngineSentinelA)
	assertErrMapResponse(t, m, wrapped, http.StatusBadRequest,
		envelope("FIXED_TEXT", "fixed safe text"))
}

// TestErrMap_MultiMatchKeepsRuleOrder：组合错误同时命中多条规则时按表
// 序首个生效——包装文本中的顺序不参与决策。
func TestErrMap_MultiMatchKeepsRuleOrder(t *testing.T) {
	m := endpointErrMap{
		rules: []errorRule{
			{sentinel: errEngineSentinelB, status: http.StatusConflict, code: "SECOND_RULE"},
			{sentinel: errEngineSentinelA, status: http.StatusBadRequest, code: "FIRST_RULE"},
		},
		fallback: fallbackInternal,
	}

	combined := fmt.Errorf("combined failure: %w; %w", errEngineSentinelA, errEngineSentinelB)
	assertErrMapResponse(t, m, combined, http.StatusConflict,
		envelope("SECOND_RULE", "an unexpected error occurred, please try again later"))
}

// TestErrMap_FallbackPresets：未知错误（无 sentinel 命中）按兜底声明分
// 流——预设引用（fallbackInternal/fallbackDB）落 SafeErrorResponse 固
// 定安全文案，且不泄露内部原因。
func TestErrMap_FallbackPresets(t *testing.T) {
	leaky := errors.New("sql: no such table: content_items; dsn=postgres://omnicraft:secret@10.0.0.1:5432/db")

	internal := endpointErrMap{fallback: fallbackInternal}
	assertErrMapResponse(t, internal, leaky, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "an unexpected error occurred, please try again later"))

	db := endpointErrMap{fallback: fallbackDB}
	assertErrMapResponse(t, db, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))

	gin.SetMode(gin.TestMode)
	for _, m := range []endpointErrMap{internal, db} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		m.write(c, leaky)
		for _, internal := range []string{"secret", "10.0.0.1", "dsn", "no such table", "postgres://"} {
			if strings.Contains(rec.Body.String(), internal) {
				t.Fatalf("fallback leaked internal detail %q: %s", internal, rec.Body.String())
			}
		}
	}
}

// TestErrMap_FallbackMessage：兜底 message 非空时走 response.Error 固定
// 文案（迁移前手写固定兜底分支的逐字节承接），Safe 语义被覆盖且同样
// 不泄露内部原因；已知 sentinel 仍走规则，不受兜底 message 影响。
func TestErrMap_FallbackMessage(t *testing.T) {
	leaky := errors.New("submit failed: dial tcp 10.0.0.1:5432: secret")
	m := endpointErrMap{
		rules: []errorRule{
			{sentinel: errEngineSentinelA, status: http.StatusBadRequest, code: "KNOWN"},
		},
		fallback: errFallback{
			status:  fallbackInternal.status,
			code:    fallbackInternal.code,
			message: "Failed to submit feedback",
		},
	}

	assertErrMapResponse(t, m, leaky, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "Failed to submit feedback"))
	assertErrMapResponse(t, m, errEngineSentinelA, http.StatusBadRequest,
		envelope("KNOWN", "an unexpected error occurred, please try again later"))
}

// logProbe captures slog output (lines) and the response recorder status at
// the moment the last log record was handled (codeAtLastLog), to prove
// log-before-response ordering inside errmap writes.
type logProbe struct {
	lines         []string
	rec           *httptest.ResponseRecorder
	codeAtLastLog int
}

func (h *logProbe) Enabled(ctx context.Context, l slog.Level) bool { return true }

func (h *logProbe) Handle(ctx context.Context, r slog.Record) error {
	h.codeAtLastLog = h.rec.Code
	var buf bytes.Buffer
	if err := slog.NewTextHandler(&buf, nil).Handle(ctx, r.Clone()); err != nil {
		return err
	}
	h.lines = append(h.lines, strings.TrimRight(buf.String(), "\n"))
	return nil
}

func (h *logProbe) WithAttrs(attrs []slog.Attr) slog.Handler { return h }

func (h *logProbe) WithGroup(name string) slog.Handler { return h }

// newErrMapLogProbe builds a gin test context plus recorder wired to a
// logProbe installed as the default slog logger for the test's lifetime.
func newErrMapLogProbe(t *testing.T) (*gin.Context, *httptest.ResponseRecorder, *logProbe) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	probe := &logProbe{rec: rec}
	old := slog.Default()
	slog.SetDefault(slog.New(probe))
	t.Cleanup(func() { slog.SetDefault(old) })
	return c, rec, probe
}

// TestErrMap_FallbackLog：兜底 log 非空时引擎在兜底响应前落
// slog.Error(log, "error", err)——日志先于响应（顺序探针：日志发生时
// 响应尚未写出，Code 仍为 200 初值）；日志面可见原始错误，响应面不可
// 见。
func TestErrMap_FallbackLog(t *testing.T) {
	boom := errors.New("engine boom: db gone")
	m := endpointErrMap{
		fallback: errFallback{
			status:  fallbackInternal.status,
			code:    fallbackInternal.code,
			message: "Failed to submit feedback",
			log:     "failed to submit feedback",
		},
	}

	c, rec, probe := newErrMapLogProbe(t)
	m.write(c, boom)

	if len(probe.lines) != 1 {
		t.Fatalf("fallback log lines = %d, want 1: %v", len(probe.lines), probe.lines)
	}
	for _, want := range []string{"failed to submit feedback", "engine boom: db gone"} {
		if !strings.Contains(probe.lines[0], want) {
			t.Fatalf("fallback log %q missing %q", probe.lines[0], want)
		}
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "db gone") {
		t.Fatalf("fallback response leaked internal error: %s", rec.Body.String())
	}
	if probe.codeAtLastLog != http.StatusOK {
		t.Fatalf("fallback log fired after response was written (codeAtLastLog = %d, want 200 = unwritten)", probe.codeAtLastLog)
	}
}

// TestErrMap_RuleBeforeHook：规则级 before 在该规则的响应前执行且只对
// 命中规则生效（兜底路径不触发 before）——副作用在前、响应在后。
func TestErrMap_RuleBeforeHook(t *testing.T) {
	var events []string
	m := endpointErrMap{
		rules: []errorRule{
			{
				sentinel: errEngineSentinelA,
				status:   http.StatusConflict,
				code:     "WITH_BEFORE",
				before: func(c *gin.Context, err error) {
					events = append(events, "before:"+err.Error())
				},
			},
		},
		fallback: fallbackInternal,
	}

	assertErrMapResponse(t, m, errEngineSentinelA, http.StatusConflict,
		envelope("WITH_BEFORE", "an unexpected error occurred, please try again later"))
	if len(events) != 1 || events[0] != "before:"+errEngineSentinelA.Error() {
		t.Fatalf("before hook events = %v, want exactly [before:%s] fired before response", events, errEngineSentinelA.Error())
	}

	events = nil
	assertErrMapResponse(t, m, errors.New("unknown"), http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "an unexpected error occurred, please try again later"))
	if len(events) != 0 {
		t.Fatalf("fallback path must not fire rule before hook, got %v", events)
	}
}

// TestErrMap_PresetBytes：预设即文档——固定 (status, code, 空 message,
// 空 log) 组合不可漂移；扩展字段只能由端点声明叠加。
func TestErrMap_PresetBytes(t *testing.T) {
	if fallbackInternal.status != http.StatusInternalServerError || fallbackInternal.code != "INTERNAL_ERROR" ||
		fallbackInternal.message != "" || fallbackInternal.log != "" {
		t.Fatalf("fallbackInternal drifted: %+v", fallbackInternal)
	}
	if fallbackDB.status != http.StatusInternalServerError || fallbackDB.code != "DB_ERROR" ||
		fallbackDB.message != "" || fallbackDB.log != "" {
		t.Fatalf("fallbackDB drifted: %+v", fallbackDB)
	}
}
