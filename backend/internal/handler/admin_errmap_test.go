package handler

// admin.go 端点的 sentinel→HTTP 契约表（#925 从手写 if 链迁到 endpointErrMap
// 声明）。期望值全部为迁移前手写字面量（= wire 的逐字节锁定）；每份 map
// 一张表，规则数锁历史值防漂移。审计事务错误的 500 AUDIT_WRITE_FAILED 是
// 各 tx map 的首条规则（原 respondAuditTxError 承接，错误只包
// errAdminAuditWriteFailed 一个 sentinel，与其余规则判交集为空，规则序与
// 迁移前分支序对任意实际错误值等价）。auditFailed 是纯副作用分支（只写
// 审计不发响应），留在 handler 内先于 write() 执行；唯 ReplayDLQEntry 的
// 按分支审计元数据走实例 map 的规则级 before（dlqReplayErrMap）。

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/worker"
)

// wrappedAuditTxError mirrors withAuditTx's audit-write failure shape:
// only errAdminAuditWriteFailed is in the %w chain.
func wrappedAuditTxError() error {
	return fmt.Errorf("%w: %v", errAdminAuditWriteFailed, errors.New("audit blocked"))
}

// TestBroadcastErrMap_RuleContract pins BroadcastNotification rules (order =
// pre-#925 switch order). ErrBroadcastValidation 命中的是服务层返回的
// *BroadcastValidationError（自定义 Is 匹配 sentinel），规则化后同判。
func TestBroadcastErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "idempotency key required", err: service.ErrIdempotencyKeyRequired, status: http.StatusBadRequest, code: "IDEMPOTENCY_KEY_REQUIRED", message: "Idempotency-Key header is required"},
		{name: "idempotency key reused", err: service.ErrIdempotencyKeyReused, status: http.StatusConflict, code: "IDEMPOTENCY_KEY_REUSED", message: "idempotency key was already used with a different payload"},
		{name: "broadcast validation", err: service.ErrBroadcastValidation, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "invalid request parameters"},
	}
	if len(cases) != 3 {
		t.Fatalf("broadcast contract table must keep the historical 3 rules, got %d", len(cases))
	}
	if len(broadcastErrMap.rules) != 3 {
		t.Fatalf("broadcast map must keep exactly 3 rules, got %d", len(broadcastErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, broadcastErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
		})
	}
	// 迁移前的实际错误形态：*BroadcastValidationError（errors.Is 经自定义
	// Is 方法命中 sentinel），与裸 sentinel 同应答。
	t.Run("broadcast validation typed error", func(t *testing.T) {
		typed := &service.BroadcastValidationError{Code: "VALIDATION_ERROR", Fields: []string{"title"}}
		assertErrMapResponse(t, broadcastErrMap, typed, http.StatusBadRequest, envelope("VALIDATION_ERROR", "invalid request parameters"))
		wrapped := fmt.Errorf("broadcast: rejected: %w", typed)
		assertErrMapResponse(t, broadcastErrMap, wrapped, http.StatusBadRequest, envelope("VALIDATION_ERROR", "invalid request parameters"))
	})
}

// TestIPDecisionErrMap_RuleContract pins the shared ApproveIP / RejectIP tx
// contract: audit write failure -> 500, unknown (incl. ErrIPNotFound via tx
// translation) -> Safe 400 ERROR.
func TestIPDecisionErrMap_RuleContract(t *testing.T) {
	if len(ipDecisionErrMap.rules) != 1 {
		t.Fatalf("ip decision map must keep exactly 1 rule, got %d", len(ipDecisionErrMap.rules))
	}
	assertErrMapResponse(t, ipDecisionErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, ipDecisionErrMap, service.ErrIPNotFound, http.StatusBadRequest,
		envelope("ERROR", "an error occurred, please try again later"))
	assertErrMapResponse(t, ipDecisionErrMap, errors.New("ip tx boom"), http.StatusBadRequest,
		envelope("ERROR", "an error occurred, please try again later"))
}

// TestContentErrMaps_RuleContract pins BanContent / RestoreContent 前置存
// 在性校验（contentPrecheckErrMap）与审计事务错误（contentModerationErrMap）
// 两份合同：NotFound / 并发缺失同 404 文案，审计写失败 500，未知错误 Safe
// 500 DB_ERROR。
func TestContentErrMaps_RuleContract(t *testing.T) {
	if len(contentPrecheckErrMap.rules) != 1 {
		t.Fatalf("content precheck map must keep exactly 1 rule, got %d", len(contentPrecheckErrMap.rules))
	}
	if len(contentModerationErrMap.rules) != 2 {
		t.Fatalf("content moderation map must keep exactly 2 rules, got %d", len(contentModerationErrMap.rules))
	}

	assertErrMapResponse(t, contentPrecheckErrMap, gorm.ErrRecordNotFound, http.StatusNotFound,
		envelope("CONTENT_NOT_FOUND", "content not found"))
	assertErrMapResponse(t, contentPrecheckErrMap, errors.New("sql: dial tcp boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))

	assertErrMapResponse(t, contentModerationErrMap, errAdminTargetMissing, http.StatusNotFound,
		envelope("CONTENT_NOT_FOUND", "content not found"))
	assertErrMapResponse(t, contentModerationErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, contentModerationErrMap, errors.New("content tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestUserModerationErrMaps_RuleContract pins BanUser / UnbanUser 前置存在
// 性校验与两张 tx map 的固定兜底文案（"failed to load/ban/unban user"，
// 迁移前固定 message 分支的逐字节承接）。
func TestUserModerationErrMaps_RuleContract(t *testing.T) {
	if len(userPrecheckErrMap.rules) != 1 {
		t.Fatalf("user precheck map must keep exactly 1 rule, got %d", len(userPrecheckErrMap.rules))
	}
	if len(banUserErrMap.rules) != 2 {
		t.Fatalf("ban user map must keep exactly 2 rules, got %d", len(banUserErrMap.rules))
	}
	if len(unbanUserErrMap.rules) != 2 {
		t.Fatalf("unban user map must keep exactly 2 rules, got %d", len(unbanUserErrMap.rules))
	}

	assertErrMapResponse(t, userPrecheckErrMap, gorm.ErrRecordNotFound, http.StatusNotFound,
		envelope("USER_NOT_FOUND", "user not found"))
	assertErrMapResponse(t, userPrecheckErrMap, errors.New("sql: dial tcp boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to load user"))

	assertErrMapResponse(t, banUserErrMap, errAdminTargetMissing, http.StatusNotFound,
		envelope("USER_NOT_FOUND", "user not found"))
	assertErrMapResponse(t, banUserErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, banUserErrMap, errors.New("ban user tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to ban user"))

	assertErrMapResponse(t, unbanUserErrMap, errAdminTargetMissing, http.StatusNotFound,
		envelope("USER_NOT_FOUND", "user not found"))
	assertErrMapResponse(t, unbanUserErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, unbanUserErrMap, errors.New("unban user tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to unban user"))
}

// TestResolveAppealErrMap_RuleContract pins ResolveAppeal tx 合同：目标不可
// 恢复 409、审计写失败 500、未知 Safe 500 DB_ERROR。
func TestResolveAppealErrMap_RuleContract(t *testing.T) {
	if len(resolveAppealErrMap.rules) != 2 {
		t.Fatalf("resolve appeal map must keep exactly 2 rules, got %d", len(resolveAppealErrMap.rules))
	}
	assertErrMapResponse(t, resolveAppealErrMap, errAppealTargetGone, http.StatusConflict,
		envelope("APPEAL_TARGET_GONE", "appeal target is no longer available"))
	assertErrMapResponse(t, resolveAppealErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, resolveAppealErrMap, errors.New("appeal tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestLLMConfigErrMaps_RuleContract pins 四份 LLM 配置面合同；密钥链缺失
// （LLM_KEY_ENCRYPTION_UNAVAILABLE）与 DeleteLLMConfig 的 404 兜底是迁移前
// Safe / 固定分支的逐字节承接（Update/Delete 的原 `==` 比较在服务层裸返
// sentinel 下与 errors.Is 等价）。
func TestLLMConfigErrMaps_RuleContract(t *testing.T) {
	if len(llmConfigCreateErrMap.rules) != 2 {
		t.Fatalf("llm create map must keep exactly 2 rules, got %d", len(llmConfigCreateErrMap.rules))
	}
	if len(llmConfigUpdateErrMap.rules) != 3 {
		t.Fatalf("llm update map must keep exactly 3 rules, got %d", len(llmConfigUpdateErrMap.rules))
	}
	if len(llmConfigDeleteErrMap.rules) != 2 {
		t.Fatalf("llm delete map must keep exactly 2 rules, got %d", len(llmConfigDeleteErrMap.rules))
	}
	if len(llmConfigActivateErrMap.rules) != 2 {
		t.Fatalf("llm activate map must keep exactly 2 rules, got %d", len(llmConfigActivateErrMap.rules))
	}

	assertErrMapResponse(t, llmConfigCreateErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, llmConfigCreateErrMap, service.ErrLLMKeyEncryptionSecretMissing, http.StatusInternalServerError,
		envelope("LLM_KEY_ENCRYPTION_UNAVAILABLE", "an unexpected error occurred, please try again later"))
	assertErrMapResponse(t, llmConfigCreateErrMap, errors.New("llm create tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))

	assertErrMapResponse(t, llmConfigUpdateErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, llmConfigUpdateErrMap, service.ErrLLMKeyEncryptionSecretMissing, http.StatusInternalServerError,
		envelope("LLM_KEY_ENCRYPTION_UNAVAILABLE", "an unexpected error occurred, please try again later"))
	assertErrMapResponse(t, llmConfigUpdateErrMap, service.ErrConfigNotFound, http.StatusNotFound,
		envelope("CONFIG_NOT_FOUND", "config not found"))
	assertErrMapResponse(t, llmConfigUpdateErrMap, errors.New("llm update tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))

	assertErrMapResponse(t, llmConfigDeleteErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, llmConfigDeleteErrMap, repository.ErrActiveConfigCannotDelete, http.StatusConflict,
		envelope("ACTIVE_CONFIG", "cannot delete active config"))
	assertErrMapResponse(t, llmConfigDeleteErrMap, errors.New("llm delete tx boom"), http.StatusNotFound,
		envelope("CONFIG_NOT_FOUND", "config not found"))

	assertErrMapResponse(t, llmConfigActivateErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, llmConfigActivateErrMap, service.ErrConfigNotFound, http.StatusNotFound,
		envelope("CONFIG_NOT_FOUND", "config not found"))
	assertErrMapResponse(t, llmConfigActivateErrMap, errors.New("llm activate tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "failed to activate config"))
}

// TestReportErrMaps_RuleContract pins ResolveReport 前置存在性校验与 tx 合
// 同（REPORT_NOT_FOUND 双位点同文案）。
func TestReportErrMaps_RuleContract(t *testing.T) {
	if len(reportPrecheckErrMap.rules) != 1 {
		t.Fatalf("report precheck map must keep exactly 1 rule, got %d", len(reportPrecheckErrMap.rules))
	}
	if len(reportResolveErrMap.rules) != 2 {
		t.Fatalf("report resolve map must keep exactly 2 rules, got %d", len(reportResolveErrMap.rules))
	}

	assertErrMapResponse(t, reportPrecheckErrMap, gorm.ErrRecordNotFound, http.StatusNotFound,
		envelope("REPORT_NOT_FOUND", "report not found"))
	assertErrMapResponse(t, reportPrecheckErrMap, errors.New("sql: dial tcp boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))

	assertErrMapResponse(t, reportResolveErrMap, errAdminTargetMissing, http.StatusNotFound,
		envelope("REPORT_NOT_FOUND", "report not found"))
	assertErrMapResponse(t, reportResolveErrMap, wrappedAuditTxError(), http.StatusInternalServerError,
		envelope("AUDIT_WRITE_FAILED", "audit write failed"))
	assertErrMapResponse(t, reportResolveErrMap, errors.New("report tx boom"), http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// newDLQReplayMapHarness builds an AdminHandler with a real audit service so
// instance-map side effects (auditFailed) are observable in sqlite.
func newDLQReplayMapHarness(t *testing.T) (*AdminHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	return newAdminHandlerForTest(db, &config.Config{}, nil, auditSvc), db
}

// TestDLQReplayErrMap_RuleContract pins the instance map: NotFound rule fires
// its rule-level before (failed audit with error_code DLQ_ENTRY_NOT_FOUND)
// then the 404 body; unknown errors fall to the Safe 500 REPLAY_FAILED
// fallback without the rule before（fallback 审计由 handler 的纯副作用分
// 支承接，见 TestReplayDLQEntryReplayFailedAudits）。
func TestDLQReplayErrMap_RuleContract(t *testing.T) {
	handler, db := newDLQReplayMapHarness(t)
	if handler.dlqReplayErrMap.rules == nil || len(handler.dlqReplayErrMap.rules) != 1 {
		t.Fatalf("dlq replay instance map must keep exactly 1 rule, got %d", len(handler.dlqReplayErrMap.rules))
	}

	newCtx := func(entryID string) (*gin.Context, *httptest.ResponseRecorder) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/queue/dlq/"+entryID+"/replay", nil)
		c.Params = gin.Params{{Key: "id", Value: entryID}}
		c.Set(middleware.UserIDKey, int64(99))
		c.Set("trace_id", "trace-dlq-map-test")
		return c, rec
	}

	// NotFound 分支：规则级 before 写失败审计（error_code 按分支）→ 404。
	c, rec := newCtx("1234567890-0")
	handler.dlqReplayErrMap.write(c, worker.ErrDLQEntryNotFound)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "DLQ_ENTRY_NOT_FOUND") || !strings.Contains(rec.Body.String(), "dlq entry not found") {
		t.Fatalf("body = %s, want DLQ_ENTRY_NOT_FOUND envelope", rec.Body.String())
	}
	var logs []model.AdminAuditLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("not-found branch audit rows = %d, want 1", len(logs))
	}
	if logs[0].Result != "failed" || logs[0].Action != "dlq_replay" || logs[0].TargetID != "1234567890-0" {
		t.Fatalf("audit row = %#v, want failed dlq_replay on 1234567890-0", logs[0])
	}
	if logs[0].Metadata["error_code"] != "DLQ_ENTRY_NOT_FOUND" {
		t.Fatalf("audit error_code = %v, want DLQ_ENTRY_NOT_FOUND", logs[0].Metadata["error_code"])
	}

	// 未知错误分支：实例 map 兜底不发规则 before，500 REPLAY_FAILED（Safe
	// 通用文案，REPLAY_FAILED 无安全文案映射）。
	c2, rec2 := newCtx("1234567890-1")
	handler.dlqReplayErrMap.write(c2, errors.New("redis: connection closed"))
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), "REPLAY_FAILED") {
		t.Fatalf("body = %s, want REPLAY_FAILED", rec2.Body.String())
	}
	if strings.Contains(rec2.Body.String(), "connection closed") {
		t.Fatalf("fallback leaked internal error: %s", rec2.Body.String())
	}
	// 兜底路径规则 before 不触发：仍只有第一分支的 1 行审计。
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("fallback branch must not fire rule before, audit rows = %d, want 1", len(logs))
	}
}

// TestReplayDLQEntryReplayFailedAudits pins the handler's pure side-effect
// branch: a non-NotFound replay failure records the failed audit with
// error_code REPLAY_FAILED before the unified write（迁移前 auditFailed 分
// 支的逐字节等价，含顺序：审计先于响应由 write 语义与引擎 before 探针保
// 证）。触发方式 = 关闭 miniredis 使 Replay 产生非 NotFound 错误。
func TestReplayDLQEntryReplayFailedAudits(t *testing.T) {
	router, db, rdb, adminToken, _ := setupReplayRouter(t)

	entryID := "1111111111-0"
	rec := httptest.NewRecorder()
	// Close the redis before the call so Replay fails with a non-NotFound error.
	rdb.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/queue/dlq/"+entryID+"/replay", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "REPLAY_FAILED") {
		t.Fatalf("body = %s, want REPLAY_FAILED", rec.Body.String())
	}

	var logs []model.AdminAuditLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("replay failure audit rows = %d, want 1", len(logs))
	}
	if logs[0].Result != "failed" || logs[0].Action != "dlq_replay" || logs[0].TargetID != entryID {
		t.Fatalf("audit row = %#v, want failed dlq_replay on %s", logs[0], entryID)
	}
	if logs[0].Metadata["error_code"] != "REPLAY_FAILED" {
		t.Fatalf("audit error_code = %v, want REPLAY_FAILED", logs[0].Metadata["error_code"])
	}
}
