package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// setupAdminConfigRouter wires an isolated sqlite-backed admin config router.
// The override file is pointed into a temp dir so PATCH tests never leak
// config_override.yaml into the working tree (T26 FIX-33).
func setupAdminConfigRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	t.Helper()
	t.Setenv("CONFIG_OVERRIDE_PATH", filepath.Join(t.TempDir(), "config_override.yaml"))

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{}
	cfg.JWT.Secret = "admin-config-contract-secret"
	cfg.Judge.MinVotesRequired = 20
	cfg.Judge.PassThreshold = 0.6
	cfg.Judge.ExamPassRate = 0.8
	cfg.Judge.ErrorRateRevoke = 0.5
	cfg.Judge.ErrorRateWindow = 10
	cfg.Reputation.QualityContentThreshold = 10
	cfg.Reputation.QualityCommentThreshold = 5
	cfg.Reputation.RepeatViolationExtraPenalty = -1

	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := NewAdminHandler(db, cfg, nil, auditSvc)

	router := gin.New()
	admin := router.Group("/api/v1/admin")
	admin.GET("/config", adminHandler.GetConfig)
	admin.PATCH("/config", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		c.Set("trace_id", "trace-admin-config-contract")
		adminHandler.PatchConfig(c)
	})
	return router, db, cfg
}

func adminConfigRequest(t *testing.T, router *gin.Engine, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, "/api/v1/admin/config", reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(rec, req)
	return rec
}

// T26 FIX-33: GET /admin/config must serialize nested config sections in
// snake_case. Without json tags on the config structs the response leaks
// PascalCase keys and the admin UI silently shows default values; a naive
// "open then save" then persists fabricated data (e.g. extra_penalty -1 → +1).
func TestAdminGetConfigReturnsSnakeCaseJSON(t *testing.T) {
	router, _, _ := setupAdminConfigRouter(t)

	rec := adminConfigRequest(t, router, http.MethodGet, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Config map[string]map[string]any `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}

	judge := payload.Config["judge"]
	if judge == nil {
		t.Fatalf("missing judge section: %s", rec.Body.String())
	}
	if judge["pass_threshold"] != 0.6 {
		t.Fatalf("judge.pass_threshold = %#v, want 0.6 (snake_case contract)", judge["pass_threshold"])
	}
	if judge["min_votes_required"] != float64(20) {
		t.Fatalf("judge.min_votes_required = %#v, want 20", judge["min_votes_required"])
	}

	reputation := payload.Config["reputation"]
	if reputation == nil {
		t.Fatalf("missing reputation section: %s", rec.Body.String())
	}
	if reputation["quality_content_threshold"] != float64(10) {
		t.Fatalf("reputation.quality_content_threshold = %#v, want 10", reputation["quality_content_threshold"])
	}
	if reputation["repeat_violation_extra_penalty"] != float64(-1) {
		t.Fatalf("reputation.repeat_violation_extra_penalty = %#v, want -1", reputation["repeat_violation_extra_penalty"])
	}

	// PascalCase leak would mean the UI cannot read the real values.
	body := rec.Body.String()
	for _, leaked := range []string{`"PassThreshold"`, `"MinVotesRequired"`, `"QualityContentThreshold"`, `"RepeatViolationExtraPenalty"`} {
		if bytes.Contains([]byte(body), []byte(leaked)) {
			t.Fatalf("response leaks PascalCase key %s: %s", leaked, body)
		}
	}
}

// T26 FIX-33: PatchConfig must apply judge section fields instead of
// returning 200 while silently dropping them ("fake success").
func TestAdminPatchConfigJudgeFieldsTakeEffect(t *testing.T) {
	router, _, cfg := setupAdminConfigRouter(t)

	rec := adminConfigRequest(t, router, http.MethodPatch, `{
		"judge": {
			"min_votes_required": 25,
			"pass_threshold": 0.7,
			"exam_pass_rate": 0.9,
			"error_rate_revoke": 0.4,
			"error_rate_window": 15
		}
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	if cfg.Judge.MinVotesRequired != 25 {
		t.Fatalf("judge.min_votes_required = %d, want 25 (patch must take effect)", cfg.Judge.MinVotesRequired)
	}
	if cfg.Judge.PassThreshold != 0.7 {
		t.Fatalf("judge.pass_threshold = %v, want 0.7", cfg.Judge.PassThreshold)
	}
	if cfg.Judge.ExamPassRate != 0.9 {
		t.Fatalf("judge.exam_pass_rate = %v, want 0.9", cfg.Judge.ExamPassRate)
	}
	if cfg.Judge.ErrorRateRevoke != 0.4 {
		t.Fatalf("judge.error_rate_revoke = %v, want 0.4", cfg.Judge.ErrorRateRevoke)
	}
	if cfg.Judge.ErrorRateWindow != 15 {
		t.Fatalf("judge.error_rate_window = %d, want 15", cfg.Judge.ErrorRateWindow)
	}

	// The response echoes the live config; it must reflect the patched values
	// in snake_case so the UI does not snap back to stale display data.
	var payload struct {
		Config map[string]map[string]any `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
	}
	if payload.Config["judge"]["min_votes_required"] != float64(25) {
		t.Fatalf("response judge.min_votes_required = %#v, want 25", payload.Config["judge"]["min_votes_required"])
	}
}

// T26 FIX-33: PatchConfig must apply reputation quality_* thresholds, which
// gate quality-content/quality-comment reputation awards.
func TestAdminPatchConfigReputationQualityFieldsTakeEffect(t *testing.T) {
	router, _, cfg := setupAdminConfigRouter(t)

	rec := adminConfigRequest(t, router, http.MethodPatch, `{
		"reputation": {
			"quality_content_threshold": 12,
			"quality_comment_threshold": 7
		}
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	if cfg.Reputation.QualityContentThreshold != 12 {
		t.Fatalf("reputation.quality_content_threshold = %d, want 12", cfg.Reputation.QualityContentThreshold)
	}
	if cfg.Reputation.QualityCommentThreshold != 7 {
		t.Fatalf("reputation.quality_comment_threshold = %d, want 7", cfg.Reputation.QualityCommentThreshold)
	}
}
