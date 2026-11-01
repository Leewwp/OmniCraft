package handler

// feedback 端点的 sentinel→HTTP 契约表（#922 从手写 switch 链迁到
// endpointErrMap 声明）。期望值全部为手写字面量（=迁移前 switch 分支的
// 逐字节 wire）；三份 map 各一张表（SubmitTicket 10 规则 / PresignUpload
// 6 规则 / GetTicket 2 规则），规则数锁历史值防漂移。兜底行为（固定文
// 案 + 兜底日志 + 日志先于响应）是迁移的核心风险面，单独用顺序探针断
// 言副作用顺序。

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"omnicraft/backend/internal/service"
)

// TestSubmitTicketErrMap_RuleContract pins every SubmitTicket rule (order =
// pre-#922 switch branch order); the wrapped variant proves errors.Is
// reaches the same rule through fmt.Errorf %w chains.
func TestSubmitTicketErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "invalid category", err: service.ErrFeedbackInvalidCategory, status: http.StatusBadRequest, code: "INVALID_CATEGORY", message: "Invalid feedback category"},
		{name: "title and description required", err: service.ErrFeedbackTitleAndDescriptionReq, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Title and description are required"},
		{name: "title too long", err: service.ErrFeedbackTitleTooLong, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Title must not exceed 160 characters"},
		{name: "contact email required", err: service.ErrFeedbackContactEmailRequired, status: http.StatusBadRequest, code: "CONTACT_EMAIL_REQUIRED", message: "Contact email is required for anonymous submissions"},
		{name: "captcha required", err: service.ErrFeedbackCaptchaRequired, status: http.StatusBadRequest, code: "CAPTCHA_REQUIRED", message: "Captcha verification is required for anonymous submissions"},
		{name: "captcha failed", err: service.ErrFeedbackCaptchaFailed, status: http.StatusBadRequest, code: "CAPTCHA_FAILED", message: "Captcha verification failed"},
		{name: "upload grant invalid", err: service.ErrFeedbackUploadGrantInvalid, status: http.StatusBadRequest, code: "UPLOAD_GRANT_INVALID", message: "Screenshot upload grant is invalid or has already been used"},
		{name: "attachment blocked", err: service.ErrFeedbackAttachmentBlocked, status: http.StatusBadRequest, code: "ATTACHMENT_BLOCKED", message: "Attachment image was rejected by content moderation"},
		{name: "attachment moderation unavailable", err: service.ErrFeedbackAttachmentModerationUnavailable, status: http.StatusServiceUnavailable, code: "ATTACHMENT_MODERATION_UNAVAILABLE", message: "Attachment image moderation is temporarily unavailable, please retry without attachments"},
		{name: "upload grant unavailable", err: service.ErrUploadGrantUnavailable, status: http.StatusServiceUnavailable, code: "UPLOAD_GRANT_UNAVAILABLE", message: "Screenshot upload grants are temporarily unavailable"},
	}
	if len(cases) != 10 {
		t.Fatalf("submit ticket contract table must keep the historical 10 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, submitTicketErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("submit ticket: moderation pipeline failed: %w", tt.err)
			assertErrMapResponse(t, submitTicketErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestPresignUploadErrMap_RuleContract pins every PresignUpload rule (order
// = pre-#922 branch order, including ErrOSSNotConfigured which used to be a
// nested errors.Is inside the default branch).
func TestPresignUploadErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "captcha required", err: service.ErrFeedbackCaptchaRequired, status: http.StatusBadRequest, code: "CAPTCHA_REQUIRED", message: "Captcha verification is required for anonymous uploads"},
		{name: "captcha failed", err: service.ErrFeedbackCaptchaFailed, status: http.StatusBadRequest, code: "CAPTCHA_FAILED", message: "Captcha verification failed"},
		{name: "invalid mime type", err: service.ErrFeedbackInvalidMimeType, status: http.StatusBadRequest, code: "INVALID_MIME_TYPE", message: "Only image uploads are supported for feedback screenshots"},
		{name: "file too large", err: service.ErrFeedbackFileTooLarge, status: http.StatusBadRequest, code: "FILE_TOO_LARGE", message: "Screenshot must be smaller than 20MB"},
		{name: "upload grant unavailable", err: service.ErrUploadGrantUnavailable, status: http.StatusServiceUnavailable, code: "UPLOAD_GRANT_UNAVAILABLE", message: "Screenshot upload grants are temporarily unavailable"},
		{name: "oss not configured", err: service.ErrOSSNotConfigured, status: http.StatusServiceUnavailable, code: "OSS_NOT_CONFIGURED", message: "OSS upload is not configured"},
	}
	if len(cases) != 6 {
		t.Fatalf("presign upload contract table must keep the historical 6 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, presignUploadErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("presign upload: signer unavailable: %w", tt.err)
			assertErrMapResponse(t, presignUploadErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestGetTicketErrMap_RuleContract pins every GetTicket rule (order =
// pre-#922 branch order).
func TestGetTicketErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "ticket not found", err: service.ErrFeedbackTicketNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "Feedback ticket not found"},
		{name: "forbidden", err: service.ErrFeedbackForbidden, status: http.StatusForbidden, code: "FORBIDDEN", message: "You can only view your own tickets"},
	}
	if len(cases) != 2 {
		t.Fatalf("get ticket contract table must keep the historical 2 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, getTicketErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("get ticket: repo failure: %w", tt.err)
			assertErrMapResponse(t, getTicketErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestFeedbackErrMap_FallbackContract：三份 map 的未知错误兜底保留历史
// 固定文案（fallbackMessage），且兜底日志（fallbackLog）先于响应写出
// （顺序探针：日志发生时响应 Code 仍为 200 初值）、日志面可见原始错误
// 而响应面不可见——与迁移前「slog.Error 后 response.Error」逐行同序。
func TestFeedbackErrMap_FallbackContract(t *testing.T) {
	maps := []struct {
		name        string
		m           endpointErrMap
		wantMessage string
		wantLog     string
	}{
		{name: "submit ticket", m: submitTicketErrMap, wantMessage: "Failed to submit feedback", wantLog: "failed to submit feedback"},
		{name: "presign upload", m: presignUploadErrMap, wantMessage: "Failed to generate upload grant", wantLog: "failed to generate upload grant"},
		{name: "get ticket", m: getTicketErrMap, wantMessage: "Failed to get feedback ticket", wantLog: "failed to get feedback ticket"},
	}
	for _, tt := range maps {
		t.Run(tt.name, func(t *testing.T) {
			boom := errors.New("feedback repo: dial tcp 10.9.9.9:5432: boom")
			c, rec, probe := newErrMapLogProbe(t)
			tt.m.write(c, boom)

			assertErrMapRecorder(t, rec, http.StatusInternalServerError, envelope("INTERNAL_ERROR", tt.wantMessage))
			if len(probe.lines) != 1 {
				t.Fatalf("fallback log lines = %d, want 1: %v", len(probe.lines), probe.lines)
			}
			for _, want := range []string{tt.wantLog, "dial tcp 10.9.9.9"} {
				if !strings.Contains(probe.lines[0], want) {
					t.Fatalf("fallback log %q missing %q", probe.lines[0], want)
				}
			}
			if strings.Contains(rec.Body.String(), "dial tcp") {
				t.Fatalf("fallback response leaked internal error: %s", rec.Body.String())
			}
			if probe.codeAtLastLog != http.StatusOK {
				t.Fatalf("fallback log fired after response was written (codeAtLastLog = %d, want 200 = unwritten)", probe.codeAtLastLog)
			}
		})
	}
}
