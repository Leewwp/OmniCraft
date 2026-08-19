package service

import (
	"context"
	"fmt"
	"log/slog"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"time"

	"gorm.io/gorm"
)

var auditMetadataAllowlist = map[string][]string{
	"content_ban":           {"content_id", "reason", "author_id"},
	"content_restore":       {"content_id", "reason"},
	"user_ban":              {"target_user_id", "reason"},
	"user_unban":            {"target_user_id", "reason"},
	"ip_approve":            {"ip_id", "decision"},
	"ip_reject":             {"ip_id", "decision"},
	"appeal_resolve":        {"appeal_id", "decision", "reason"},
	"report_resolve":        {"report_id", "decision", "reason"},
	"config_patch":          {"field", "old_value_masked", "new_value_masked"},
	"category_create":       {"name", "slug", "display_order"},
	"category_update":       {"category_id", "name", "slug", "display_order"},
	"category_delete":       {"category_id", "name"},
	"category_reorder":      {"order"},
	"llm_config_create":     {"provider", "model"},
	"llm_config_update":     {"config_id", "provider", "model"},
	"llm_config_delete":     {"config_id", "provider", "model"},
	"llm_config_activate":   {"config_id"},
	"llm_config_test":       {"config_id"},
	"judge_question_create": {"question_id", "content_type"},
	"feedback_reply":        {"ticket_id", "is_internal_note"},
	"feedback_close":        {"ticket_id", "reason"},
	"feedback_reopen":       {"ticket_id"},
	"feedback_priority":     {"ticket_id", "priority"},
	"feedback_assign":       {"ticket_id", "assignee_admin_id"},
	"broadcast_notification": {
		"recipient_count",
		"title_length",
		"body_length",
		"filter",
		"validation_error_code",
		"validation_fields",
		"error_code",
		"key_fingerprint",
		"replayed",
	},
	"dlq_replay":                  {"original_topic", "original_id", "error_code"},
	"rag_rebuild":                 {"error_code", "operation_id"},
	"archive_scan_manual_review":  {"job_id", "reason"},
	"archive_scan_review_resolve": {"job_id", "outcome", "reason"},
	"archive_scan_view":           {"job_id"},
	"archive_scan_retry":          {"job_id"},
}

var sensitiveKeyPatterns = []string{
	"token", "cookie", "password", "api_key", "secret", "grant",
	"access_key", "private_key", "authorization", "header",
}

type RecordAdminAuditInput struct {
	AdminUserID int64
	Action      string
	TargetType  string
	TargetID    string
	TraceID     string
	Metadata    map[string]interface{}
	Result      string
}

type AdminAuditFilter struct {
	Action      string
	AdminUserID int64
	From        *time.Time
	To          *time.Time
	Page        int
	PageSize    int
}

type AdminAuditService struct {
	repo *repository.AdminAuditRepository
	db   *gorm.DB
}

func NewAdminAuditService(repo *repository.AdminAuditRepository, db *gorm.DB) *AdminAuditService {
	return &AdminAuditService{repo: repo, db: db}
}

func (s *AdminAuditService) Record(ctx context.Context, entry RecordAdminAuditInput) error {
	filtered := filterMetadata(entry.Action, entry.Metadata)
	log := &model.AdminAuditLog{
		AdminUserID: entry.AdminUserID,
		Action:      entry.Action,
		TargetType:  entry.TargetType,
		TargetID:    entry.TargetID,
		TraceID:     entry.TraceID,
		Metadata:    filtered,
		Result:      entry.Result,
	}
	if err := s.db.WithContext(ctx).Create(log).Error; err != nil {
		slog.Error("audit write failed",
			slog.String("action", entry.Action),
			slog.String("target_type", entry.TargetType),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("AUDIT_WRITE_FAILED: %w", err)
	}
	return nil
}

func (s *AdminAuditService) RecordTx(ctx context.Context, tx *gorm.DB, entry RecordAdminAuditInput) error {
	filtered := filterMetadata(entry.Action, entry.Metadata)
	log := &model.AdminAuditLog{
		AdminUserID: entry.AdminUserID,
		Action:      entry.Action,
		TargetType:  entry.TargetType,
		TargetID:    entry.TargetID,
		TraceID:     entry.TraceID,
		Metadata:    filtered,
		Result:      entry.Result,
	}
	if err := tx.WithContext(ctx).Create(log).Error; err != nil {
		slog.Error("audit write in tx failed",
			slog.String("action", entry.Action),
			slog.String("target_type", entry.TargetType),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("AUDIT_WRITE_FAILED: %w", err)
	}
	return nil
}

func (s *AdminAuditService) List(ctx context.Context, filter AdminAuditFilter) ([]model.AdminAuditLog, int64, error) {
	repoFilter := repository.AdminAuditFilter{
		Action:      filter.Action,
		AdminUserID: filter.AdminUserID,
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		From:        filter.From,
		To:          filter.To,
	}
	return s.repo.List(repoFilter)
}

func filterMetadata(action string, raw map[string]interface{}) model.JSONMap {
	if raw == nil {
		return model.JSONMap{}
	}

	allowed, hasAllowlist := auditMetadataAllowlist[action]
	filtered := make(model.JSONMap)

	if hasAllowlist {
		allowedSet := make(map[string]struct{}, len(allowed))
		for _, k := range allowed {
			allowedSet[k] = struct{}{}
		}
		for k, v := range raw {
			if _, ok := allowedSet[k]; ok && !isSensitiveKey(k) {
				filtered[k] = v
			}
		}
	} else {
		for k, v := range raw {
			if !isSensitiveKey(k) {
				filtered[k] = v
			}
		}
	}

	return filtered
}

func isSensitiveKey(key string) bool {
	for _, pattern := range sensitiveKeyPatterns {
		if containsIgnoreCase(key, pattern) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	slen := len(s)
	sublen := len(substr)
	if sublen > slen {
		return false
	}
	for i := 0; i <= slen-sublen; i++ {
		match := true
		for j := 0; j < sublen; j++ {
			sc := s[i+j]
			pc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if pc >= 'A' && pc <= 'Z' {
				pc += 32
			}
			if sc != pc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
