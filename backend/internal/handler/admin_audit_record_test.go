package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/service/promptregistry"
)

// erroringAdminRAGAudit is the AdminRAGAuditRecorder double for the audit
// degradation contract: it records every entry and returns a scripted error
// (nil error = success path).
type erroringAdminRAGAudit struct {
	err     error
	entries []service.RecordAdminAuditInput
}

func (a *erroringAdminRAGAudit) Record(_ context.Context, entry service.RecordAdminAuditInput) error {
	a.entries = append(a.entries, entry)
	return a.err
}

// installAuditFailureMetrics installs a fresh process-wide metrics registry for
// the duration of one test and resets the default to nil afterwards.
func installAuditFailureMetrics(t *testing.T) *observability.Metrics {
	t.Helper()
	metrics := observability.NewMetrics()
	observability.SetDefaultMetrics(metrics)
	t.Cleanup(func() { observability.SetDefaultMetrics(nil) })
	return metrics
}

// auditFailureCounterValue reads omnicraft_audit_record_failures_total off the
// gathered registry (0 when the family has not been recorded yet).
func auditFailureCounterValue(t *testing.T, metrics *observability.Metrics) float64 {
	t.Helper()
	families, err := metrics.Registry.Gather()
	require.NoError(t, err)
	for _, mf := range families {
		if mf.GetName() == "omnicraft_audit_record_failures_total" {
			require.Len(t, mf.GetMetric(), 1)
			return mf.GetMetric()[0].GetCounter().GetValue()
		}
	}
	return 0
}

func TestRecordAuditLogsAndCountsRecorderFailure(t *testing.T) {
	metrics := installAuditFailureMetrics(t)
	recorder := &erroringAdminRAGAudit{err: errors.New("audit sink unavailable")}
	entry := service.RecordAdminAuditInput{
		AdminUserID: 7,
		Action:      "prompt.version.create",
		TargetType:  "prompt",
		TargetID:    "conversation_title_prompt",
		Result:      "success",
	}

	logs := captureSlog(t, func() {
		recordAudit(context.Background(), recorder, entry)
	})
	joined := strings.Join(logs, "\n")

	require.Equal(t, float64(1), auditFailureCounterValue(t, metrics),
		"a failed audit write must bump the failure counter exactly once")
	require.Contains(t, joined, "admin audit record write failed")
	require.Contains(t, joined, "action=prompt.version.create")
	require.Contains(t, joined, "target_id=conversation_title_prompt")
	require.Contains(t, joined, "admin_user_id=7")
	require.Contains(t, joined, "audit sink unavailable")
	require.Equal(t, []service.RecordAdminAuditInput{entry}, recorder.entries)
}

func TestRecordAuditNilRecorderAndSuccessStayUncounted(t *testing.T) {
	metrics := installAuditFailureMetrics(t)
	ok := &erroringAdminRAGAudit{}

	logs := captureSlog(t, func() {
		recordAudit(context.Background(), nil, service.RecordAdminAuditInput{Action: "eval.golden_draft.delete"})
		recordAudit(context.Background(), ok, service.RecordAdminAuditInput{Action: "prompt.label.set"})
	})

	require.Equal(t, float64(0), auditFailureCounterValue(t, metrics))
	require.NotContains(t, strings.Join(logs, "\n"), "admin audit record write failed")
	require.Len(t, ok.entries, 1, "the recorder must still be called when wired")
}

// TestAdminPromptCreateVersionDegradesWhenAuditFails pins the call-site
// contract of the helper: the audited mutation already succeeded, so an
// audit-sink outage must degrade to log + counter, never fail the endpoint.
func TestAdminPromptCreateVersionDegradesWhenAuditFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := installAuditFailureMetrics(t)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AdminAuditLog{}, &model.PromptRegistry{}, &model.PromptLabel{}))
	repo := repository.NewPromptRegistryRepository(db)
	require.NoError(t, promptregistry.SeedV1(t.Context(), repo))

	recorder := &erroringAdminRAGAudit{err: errors.New("audit sink unavailable")}
	h := NewAdminPromptHandler(repo, promptregistry.NewPromptResolver(repo), recorder)
	router := gin.New()
	router.POST("/api/v1/admin/prompts/:name/versions", h.CreateVersion)

	var rec *httptest.ResponseRecorder
	logs := captureSlog(t, func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/prompts/conversation_title_prompt/versions",
			strings.NewReader(`{"content":"生成标题：{{first_user_message}}"}`))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	})

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, float64(1), auditFailureCounterValue(t, metrics))
	require.Contains(t, strings.Join(logs, "\n"), "action=prompt.version.create")
}
