package handler

import (
	"context"
	"log/slog"

	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/service"
)

// recordAudit 写一条 admin 审计记录并做降级收尾（体检 A6/#949）：调用点的
// 业务变更已经成功，审计行只是事后留痕，所以写失败时不上抛成客户端错
// 误，而是 slog.Error 带全上下文（action/target/admin）+ 失败计数 metric。
// recorder 为 nil（未接线）时保持原有跳过语义。
func recordAudit(ctx context.Context, recorder AdminRAGAuditRecorder, entry service.RecordAdminAuditInput) {
	if recorder == nil {
		return
	}
	if err := recorder.Record(ctx, entry); err != nil {
		slog.Error("admin audit record write failed",
			"action", entry.Action,
			"target_id", entry.TargetID,
			"admin_user_id", entry.AdminUserID,
			"error", err,
		)
		observability.IncDefaultAuditRecordFailure()
	}
}
