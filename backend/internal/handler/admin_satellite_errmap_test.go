package handler

// admin 卫星四文件（admin_feedback / admin_archive_scan / admin_prompt /
// admin_evals）端点的 sentinel→HTTP 契约表（#929 从手写链迁到 endpointErrMap
// 声明）。期望值全部为迁移前手写字面量（= wire 的逐字节锁定）；每份 map
// 一张表，规则数锁历史值防漂移。Patch/Reply 的审计写失败 500
// AUDIT_WRITE_FAILED 是两张 tx map 的首条规则（迁移前 if 前置分支，错误
// 只包 errAdminAuditWriteFailed 一个 sentinel）；GetFeedback 与两个 notify
// 兜底分支的「slog.Error 先于 response.Error」由 fallbackLog 承接，顺序探
// 针断言副作用顺序。archiveScanErrMap 的规则序锁迁移前 switch case 序
// （errors.Join 组合错误按表序首命中，与 switch 首个 case 命中等价）。

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// TestAdminFeedbackGetErrMap_RuleContract pins GetFeedback 的 1 规则 + 兜底
// 合同：NotFound → 404 固定文案；其余错误先落兜底日志（迁移前
// slog.Error "failed to get feedback ticket for admin"）再回 500 固定文案。
func TestAdminFeedbackGetErrMap_RuleContract(t *testing.T) {
	if len(adminFeedbackGetErrMap.rules) != 1 {
		t.Fatalf("admin feedback get map must keep exactly 1 rule, got %d", len(adminFeedbackGetErrMap.rules))
	}
	assertErrMapResponse(t, adminFeedbackGetErrMap, service.ErrFeedbackTicketNotFound, http.StatusNotFound,
		envelope("NOT_FOUND", "Feedback ticket not found"))

	boom := errors.New("feedback repo: dial tcp 10.9.9.9:5432: boom")
	c, rec, probe := newErrMapLogProbe(t)
	adminFeedbackGetErrMap.write(c, boom)
	assertErrMapRecorder(t, rec, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "Failed to get feedback ticket"))
	if len(probe.lines) != 1 {
		t.Fatalf("fallback log lines = %d, want 1: %v", len(probe.lines), probe.lines)
	}
	for _, want := range []string{"failed to get feedback ticket for admin", "dial tcp 10.9.9.9"} {
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
}

// TestAdminFeedbackPatchErrMap_RuleContract pins PatchFeedback 审计事务错误
// 的 5 规则合同（审计写失败首条 + 迁移前 switch 四 case 序）与固定兜底文
// 案 "Failed to update feedback ticket"（无兜底日志——迁移前 default 分支
// 只发响应）。
func TestAdminFeedbackPatchErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "audit write failed", err: wrappedAuditTxError(), status: http.StatusInternalServerError, code: "AUDIT_WRITE_FAILED", message: "audit write failed"},
		{name: "ticket not found", err: service.ErrFeedbackTicketNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "Feedback ticket not found"},
		{name: "invalid status", err: service.ErrFeedbackInvalidStatus, status: http.StatusBadRequest, code: "INVALID_STATUS", message: "Invalid status value"},
		{name: "invalid priority", err: service.ErrFeedbackInvalidPriority, status: http.StatusBadRequest, code: "INVALID_PRIORITY", message: "Invalid priority value"},
		{name: "delivery failed", err: service.ErrFeedbackDeliveryFailed, status: http.StatusBadGateway, code: "FEEDBACK_DELIVERY_FAILED", message: "Feedback update delivery failed; please retry"},
	}
	if len(cases) != 5 {
		t.Fatalf("patch contract table must keep the historical 5 rules, got %d", len(cases))
	}
	if len(adminFeedbackPatchErrMap.rules) != 5 {
		t.Fatalf("admin feedback patch map must keep exactly 5 rules, got %d", len(adminFeedbackPatchErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, adminFeedbackPatchErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
		})
	}
	t.Run("tx error wrapped", func(t *testing.T) {
		wrapped := fmt.Errorf("patch ticket tx: %w", service.ErrFeedbackInvalidStatus)
		assertErrMapResponse(t, adminFeedbackPatchErrMap, wrapped, http.StatusBadRequest,
			envelope("INVALID_STATUS", "Invalid status value"))
	})

	boom := errors.New("patch tx boom")
	c, rec, probe := newErrMapLogProbe(t)
	adminFeedbackPatchErrMap.write(c, boom)
	assertErrMapRecorder(t, rec, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "Failed to update feedback ticket"))
	if len(probe.lines) != 0 {
		t.Fatalf("patch fallback must not log (pre-migration default branch had none), got %v", probe.lines)
	}
}

// TestAdminFeedbackPatchNotifyErrMap_RuleContract pins PatchFeedback 通知分
// 支合同：ErrFeedbackDeliveryFailed → 502；其余错误先落兜底日志（迁移前
// slog.Error "failed to notify patch ticket"）再回 500 固定文案。
func TestAdminFeedbackPatchNotifyErrMap_RuleContract(t *testing.T) {
	if len(adminFeedbackPatchNotifyErrMap.rules) != 1 {
		t.Fatalf("patch notify map must keep exactly 1 rule, got %d", len(adminFeedbackPatchNotifyErrMap.rules))
	}
	assertErrMapResponse(t, adminFeedbackPatchNotifyErrMap, service.ErrFeedbackDeliveryFailed, http.StatusBadGateway,
		envelope("FEEDBACK_DELIVERY_FAILED", "Feedback update delivery failed; please retry"))

	boom := errors.New("smtp: connection refused")
	c, rec, probe := newErrMapLogProbe(t)
	adminFeedbackPatchNotifyErrMap.write(c, boom)
	assertErrMapRecorder(t, rec, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "Failed to update feedback ticket"))
	if len(probe.lines) != 1 {
		t.Fatalf("fallback log lines = %d, want 1: %v", len(probe.lines), probe.lines)
	}
	if !strings.Contains(probe.lines[0], "failed to notify patch ticket") || !strings.Contains(probe.lines[0], "connection refused") {
		t.Fatalf("fallback log %q missing fixed log text or raw error", probe.lines[0])
	}
	if probe.codeAtLastLog != http.StatusOK {
		t.Fatalf("fallback log fired after response was written (codeAtLastLog = %d, want 200 = unwritten)", probe.codeAtLastLog)
	}
}

// TestAdminFeedbackReplyErrMap_RuleContract pins ReplyFeedback 审计事务错误
// 的 5 规则合同（审计写失败首条 + 迁移前 switch 四 case 序）与固定兜底文
// 案 "Failed to create reply"（无兜底日志）。
func TestAdminFeedbackReplyErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "audit write failed", err: wrappedAuditTxError(), status: http.StatusInternalServerError, code: "AUDIT_WRITE_FAILED", message: "audit write failed"},
		{name: "ticket not found", err: service.ErrFeedbackTicketNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "Feedback ticket not found"},
		{name: "body required", err: service.ErrFeedbackBodyRequired, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Reply body is required"},
		{name: "body too long", err: service.ErrFeedbackBodyTooLong, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "Reply body must not exceed 5000 characters"},
		{name: "delivery failed", err: service.ErrFeedbackDeliveryFailed, status: http.StatusBadGateway, code: "FEEDBACK_DELIVERY_FAILED", message: "Feedback reply delivery failed; please retry"},
	}
	if len(cases) != 5 {
		t.Fatalf("reply contract table must keep the historical 5 rules, got %d", len(cases))
	}
	if len(adminFeedbackReplyErrMap.rules) != 5 {
		t.Fatalf("admin feedback reply map must keep exactly 5 rules, got %d", len(adminFeedbackReplyErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, adminFeedbackReplyErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
		})
	}
	t.Run("tx error wrapped", func(t *testing.T) {
		wrapped := fmt.Errorf("admin reply tx: %w", service.ErrFeedbackBodyTooLong)
		assertErrMapResponse(t, adminFeedbackReplyErrMap, wrapped, http.StatusBadRequest,
			envelope("VALIDATION_ERROR", "Reply body must not exceed 5000 characters"))
	})

	boom := errors.New("reply tx boom")
	c, rec, probe := newErrMapLogProbe(t)
	adminFeedbackReplyErrMap.write(c, boom)
	assertErrMapRecorder(t, rec, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "Failed to create reply"))
	if len(probe.lines) != 0 {
		t.Fatalf("reply fallback must not log (pre-migration default branch had none), got %v", probe.lines)
	}
}

// TestAdminFeedbackReplyNotifyErrMap_RuleContract pins ReplyFeedback 通知分
// 支合同：ErrFeedbackDeliveryFailed → 502；其余错误先落兜底日志（迁移前
// slog.Error "failed to notify admin reply"）再回 500 固定文案。
func TestAdminFeedbackReplyNotifyErrMap_RuleContract(t *testing.T) {
	if len(adminFeedbackReplyNotifyErrMap.rules) != 1 {
		t.Fatalf("reply notify map must keep exactly 1 rule, got %d", len(adminFeedbackReplyNotifyErrMap.rules))
	}
	assertErrMapResponse(t, adminFeedbackReplyNotifyErrMap, service.ErrFeedbackDeliveryFailed, http.StatusBadGateway,
		envelope("FEEDBACK_DELIVERY_FAILED", "Feedback reply delivery failed; please retry"))

	boom := errors.New("smtp: dial timeout")
	c, rec, probe := newErrMapLogProbe(t)
	adminFeedbackReplyNotifyErrMap.write(c, boom)
	assertErrMapRecorder(t, rec, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "Failed to create reply"))
	if len(probe.lines) != 1 {
		t.Fatalf("fallback log lines = %d, want 1: %v", len(probe.lines), probe.lines)
	}
	if !strings.Contains(probe.lines[0], "failed to notify admin reply") || !strings.Contains(probe.lines[0], "dial timeout") {
		t.Fatalf("fallback log %q missing fixed log text or raw error", probe.lines[0])
	}
	if probe.codeAtLastLog != http.StatusOK {
		t.Fatalf("fallback log fired after response was written (codeAtLastLog = %d, want 200 = unwritten)", probe.codeAtLastLog)
	}
}

// TestArchiveScanErrMap_RuleContract pins respondArchiveScanError（迁移前
// switch 函数）的 9 规则 + 兜底合同；规则序锁迁移前 case 序，errors.Join
// 组合错误（restore 补偿失败、审计写失败、完成通知失败的实际错误形态）
// 按表序首命中，与 switch 首个 case 命中逐字节等价。
func TestArchiveScanErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "restore rollback failed", err: errArchiveRestoreRollbackFailed, status: http.StatusInternalServerError, code: "ARCHIVE_RESTORE_ROLLBACK_FAILED", message: "archive restore rollback failed"},
		{name: "audit write failed", err: errAdminAuditWriteFailed, status: http.StatusInternalServerError, code: "AUDIT_WRITE_FAILED", message: "audit write failed"},
		{name: "audit unavailable", err: errAdminAuditUnavailable, status: http.StatusServiceUnavailable, code: "AUDIT_UNAVAILABLE", message: "audit service is unavailable"},
		{name: "scan not found", err: repository.ErrArchiveScanNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "archive scan job not found"},
		{name: "illegal state", err: repository.ErrArchiveScanIllegalState, status: http.StatusConflict, code: "ARCHIVE_SCAN_ILLEGAL_STATE", message: "archive scan state transition is not allowed"},
		{name: "retry exhausted", err: repository.ErrArchiveScanRetryExhausted, status: http.StatusConflict, code: "ARCHIVE_SCAN_RETRY_EXHAUSTED", message: "archive scan retry budget is exhausted"},
		{name: "restore failed", err: errArchiveRestoreFailed, status: http.StatusBadGateway, code: "ARCHIVE_RESTORE_FAILED", message: "archive restore failed"},
		{name: "restore target exists", err: errArchiveRestoreTargetExists, status: http.StatusConflict, code: "ARCHIVE_RESTORE_TARGET_EXISTS", message: "archive restore target already exists"},
		{name: "completion failed", err: errArchiveScanCompletionFailed, status: http.StatusServiceUnavailable, code: "ARCHIVE_SCAN_COMPLETION_FAILED", message: "archive scan completion could not be applied"},
	}
	if len(cases) != 9 {
		t.Fatalf("archive scan contract table must keep the historical 9 rules, got %d", len(cases))
	}
	if len(archiveScanErrMap.rules) != 9 {
		t.Fatalf("archive scan map must keep exactly 9 rules, got %d", len(archiveScanErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, archiveScanErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
		})
	}

	// 迁移前的实际错误形态：errors.Join 组合（resolveCleanManualReview 的
	// 补偿失败 / 审计写失败、完成通知失败）与 %w 包装——表序首命中。
	t.Run("joined compensation failure hits rollback rule first", func(t *testing.T) {
		joined := errors.Join(errArchiveRestoreTargetExists, errArchiveRestoreRollbackFailed)
		assertErrMapResponse(t, archiveScanErrMap, joined, http.StatusInternalServerError,
			envelope("ARCHIVE_RESTORE_ROLLBACK_FAILED", "archive restore rollback failed"))
	})
	t.Run("joined restore failure with rollback hits rollback rule first", func(t *testing.T) {
		joined := errors.Join(errArchiveRestoreFailed, errArchiveRestoreRollbackFailed)
		assertErrMapResponse(t, archiveScanErrMap, joined, http.StatusInternalServerError,
			envelope("ARCHIVE_RESTORE_ROLLBACK_FAILED", "archive restore rollback failed"))
	})
	t.Run("joined audit write failure", func(t *testing.T) {
		joined := errors.Join(errAdminAuditWriteFailed, errors.New("audit record rejected"))
		assertErrMapResponse(t, archiveScanErrMap, joined, http.StatusInternalServerError,
			envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	})
	t.Run("joined completion failure", func(t *testing.T) {
		joined := errors.Join(errArchiveScanCompletionFailed, errors.New("notifier unavailable"))
		assertErrMapResponse(t, archiveScanErrMap, joined, http.StatusServiceUnavailable,
			envelope("ARCHIVE_SCAN_COMPLETION_FAILED", "archive scan completion could not be applied"))
	})
	t.Run("wrapped sentinel", func(t *testing.T) {
		wrapped := fmt.Errorf("retry job: %w", repository.ErrArchiveScanRetryExhausted)
		assertErrMapResponse(t, archiveScanErrMap, wrapped, http.StatusConflict,
			envelope("ARCHIVE_SCAN_RETRY_EXHAUSTED", "archive scan retry budget is exhausted"))
	})

	// 兜底：DB_ERROR 固定文案（fallbackMessage），无兜底日志。
	boom := errors.New("sql: dial tcp 10.9.9.9:5432: boom")
	c, rec, probe := newErrMapLogProbe(t)
	archiveScanErrMap.write(c, boom)
	assertErrMapRecorder(t, rec, http.StatusInternalServerError,
		envelope("DB_ERROR", "archive scan operation failed"))
	if strings.Contains(rec.Body.String(), "dial tcp") {
		t.Fatalf("fallback response leaked internal error: %s", rec.Body.String())
	}
	if len(probe.lines) != 0 {
		t.Fatalf("archive scan fallback must not log (pre-migration default branch had none), got %v", probe.lines)
	}
}

// TestAdminPromptErrMaps_RuleContract pins 三份 prompt 注册表面合同：
// next-version 兜底 map 无规则（ErrPromptNotFound 的放行是调用点控制流谓
// 词，不在 map 面）；CreateVersion 的版本冲突 409；SetLabel 的版本缺失
// 404。三份兜底均为 500 DB_ERROR 固定文案。
func TestAdminPromptErrMaps_RuleContract(t *testing.T) {
	if len(promptNextVersionErrMap.rules) != 0 {
		t.Fatalf("prompt next version map must stay rule-less, got %d rules", len(promptNextVersionErrMap.rules))
	}
	if len(promptCreateVersionErrMap.rules) != 1 {
		t.Fatalf("prompt create version map must keep exactly 1 rule, got %d", len(promptCreateVersionErrMap.rules))
	}
	if len(promptSetLabelErrMap.rules) != 1 {
		t.Fatalf("prompt set label map must keep exactly 1 rule, got %d", len(promptSetLabelErrMap.rules))
	}

	for _, err := range []error{
		errors.New("sql: dial tcp boom"),
		// 谓词层面不会送入（LatestVersion 的 ErrPromptNotFound 走放行续
		// 行）；若送入同样 500——行为面只由调用点谓词决定。
		repository.ErrPromptNotFound,
	} {
		assertErrMapResponse(t, promptNextVersionErrMap, err, http.StatusInternalServerError,
			envelope("DB_ERROR", "failed to resolve next version"))
	}

	assertErrMapResponse(t, promptCreateVersionErrMap, repository.ErrPromptVersionExists, http.StatusConflict,
		envelope("PROMPT_VERSION_EXISTS", "prompt version already exists, retry to re-resolve next version"))
	wrapped := fmt.Errorf("create version: unique violation: %w", repository.ErrPromptVersionExists)
	assertErrMapResponse(t, promptCreateVersionErrMap, wrapped, http.StatusConflict,
		envelope("PROMPT_VERSION_EXISTS", "prompt version already exists, retry to re-resolve next version"))
	assertErrMapResponse(t, promptCreateVersionErrMap, errors.New("insert boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to create prompt version"))

	assertErrMapResponse(t, promptSetLabelErrMap, repository.ErrPromptNotFound, http.StatusNotFound,
		envelope("PROMPT_VERSION_NOT_FOUND", "prompt version not found"))
	assertErrMapResponse(t, promptSetLabelErrMap, errors.New("label upsert boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to move prompt label"))
}

// TestAdminEvalErrMaps_RuleContract pins 两份 eval 诊断面合同：草稿存在性
// 预检兜底 map 无规则（ErrRagEvalNotFound 的放行是调用点控制流谓词）；
// DeleteDraft 的草稿缺失 404。兜底均为 500 DB_ERROR 固定文案。
func TestAdminEvalErrMaps_RuleContract(t *testing.T) {
	if len(evalDraftPrecheckErrMap.rules) != 0 {
		t.Fatalf("eval draft precheck map must stay rule-less, got %d rules", len(evalDraftPrecheckErrMap.rules))
	}
	if len(evalDeleteDraftErrMap.rules) != 1 {
		t.Fatalf("eval delete draft map must keep exactly 1 rule, got %d", len(evalDeleteDraftErrMap.rules))
	}

	for _, err := range []error{
		errors.New("sql: dial tcp boom"),
		// 谓词层面不会送入（GetGoldenCaseByKey 的 ErrRagEvalNotFound 走
		// 放行续行）；若送入同样 500——行为面只由调用点谓词决定。
		repository.ErrRagEvalNotFound,
	} {
		assertErrMapResponse(t, evalDraftPrecheckErrMap, err, http.StatusInternalServerError,
			envelope("DB_ERROR", "failed to check existing draft"))
	}

	assertErrMapResponse(t, evalDeleteDraftErrMap, repository.ErrRagEvalNotFound, http.StatusNotFound,
		envelope("DRAFT_NOT_FOUND", "golden draft not found"))
	wrapped := fmt.Errorf("delete draft: %w", repository.ErrRagEvalNotFound)
	assertErrMapResponse(t, evalDeleteDraftErrMap, wrapped, http.StatusNotFound,
		envelope("DRAFT_NOT_FOUND", "golden draft not found"))
	assertErrMapResponse(t, evalDeleteDraftErrMap, errors.New("delete boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to delete golden draft"))
}
