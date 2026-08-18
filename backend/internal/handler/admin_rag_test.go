package handler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type recordingRAGRebuilder struct {
	calls  int
	err    error
	onCall func()
}

type recordingAdminRAGAudit struct {
	entries       []service.RecordAdminAuditInput
	contextErrors []error
	failOn        map[string]error
}

func (a *recordingAdminRAGAudit) Record(ctx context.Context, entry service.RecordAdminAuditInput) error {
	a.entries = append(a.entries, entry)
	a.contextErrors = append(a.contextErrors, ctx.Err())
	return a.failOn[entry.Result]
}

func (r *recordingRAGRebuilder) Rebuild(context.Context) error {
	r.calls++
	if r.onCall != nil {
		r.onCall()
	}
	return r.err
}

func TestAdminRAGRebuildReturnsFeatureDisabledWithoutRunning(t *testing.T) {
	router, db, token, rebuilder := setupAdminRAGRouter(t, false)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rag/rebuild", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "FEATURE_DISABLED")
	require.Zero(t, rebuilder.calls)
	var audit model.AdminAuditLog
	require.NoError(t, db.Where("action = ?", "rag_rebuild").Take(&audit).Error)
	require.Equal(t, "failed", audit.Result)
}

func TestAdminRAGRebuildRequiresAuthentication(t *testing.T) {
	router, db, _, rebuilder := setupAdminRAGRouter(t, true)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/admin/rag/rebuild", nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Zero(t, rebuilder.calls)
	var audits int64
	require.NoError(t, db.Model(&model.AdminAuditLog{}).Count(&audits).Error)
	require.Zero(t, audits)
}

func TestAdminRAGRebuildRejectsAuthenticatedNonAdmin(t *testing.T) {
	router, db, _, rebuilder := setupAdminRAGRouter(t, true)
	member := model.User{Email: "rag-member@example.com", Username: "rag-member", PasswordHash: "hash", Role: "user"}
	require.NoError(t, db.Create(&member).Error)
	cfg := &config.Config{}
	cfg.JWT.Secret = "rag-rebuild-contract-secret"
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rag/rebuild", nil)
	request.Header.Set("Authorization", "Bearer "+mustToken(t, cfg, member.ID, member.Role))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Zero(t, rebuilder.calls)
	var audits int64
	require.NoError(t, db.Model(&model.AdminAuditLog{}).Count(&audits).Error)
	require.Zero(t, audits)
}

func TestAdminRAGRebuildSuccessIsAudited(t *testing.T) {
	router, db, token, rebuilder := setupAdminRAGRouter(t, true)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rag/rebuild", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, rebuilder.calls)
	var audits []model.AdminAuditLog
	require.NoError(t, db.Where("action = ?", "rag_rebuild").Order("id ASC").Find(&audits).Error)
	require.Equal(t, []string{"started", "success"}, []string{audits[0].Result, audits[1].Result})
	require.NotEmpty(t, audits[0].TargetID)
	require.Equal(t, audits[0].TargetID, audits[1].TargetID)
	require.Equal(t, audits[0].TargetID, audits[0].Metadata["operation_id"])
}

func TestAdminRAGRebuildFailureIsSafeAndAudited(t *testing.T) {
	router, db, token, rebuilder := setupAdminRAGRouter(t, true)
	rebuilder.err = errors.New("opensearch credential must-not-leak")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rag/rebuild", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "RAG_REBUILD_UNAVAILABLE")
	require.NotContains(t, recorder.Body.String(), "must-not-leak")
	var audits []model.AdminAuditLog
	require.NoError(t, db.Where("action = ?", "rag_rebuild").Order("id ASC").Find(&audits).Error)
	require.Equal(t, []string{"started", "failed"}, []string{audits[0].Result, audits[1].Result})
	require.Equal(t, audits[0].TargetID, audits[1].TargetID)
	require.Equal(t, "RAG_REBUILD_UNAVAILABLE", audits[1].Metadata["error_code"])
}

func TestAdminRAGRebuildFailsClosedWhenStartedAuditCannotPersist(t *testing.T) {
	audit := &recordingAdminRAGAudit{failOn: map[string]error{"started": errors.New("audit unavailable")}}
	router, _, token, rebuilder := setupAdminRAGRouterWithAudit(t, true, audit)
	response := performAdminRAGRebuild(router, token)
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Zero(t, rebuilder.calls)
	require.Len(t, audit.entries, 1)
	require.Equal(t, "started", audit.entries[0].Result)
}

func TestAdminRAGRebuildFailsClosedWithoutAuditService(t *testing.T) {
	cfg := &config.Config{}
	cfg.Features.RAGHybridEnabled = true
	rebuilder := &recordingRAGRebuilder{}
	handler := NewAdminRAGHandler(cfg, rebuilder, nil)
	router := gin.New()
	router.POST("/api/v1/admin/rag/rebuild", handler.Rebuild)

	response := performAdminRAGRebuild(router, "unused")
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.Zero(t, rebuilder.calls)
}

func TestAdminRAGRebuildSuccessAuditDegradationDoesNotInviteRepeat(t *testing.T) {
	audit := &recordingAdminRAGAudit{failOn: map[string]error{"success": errors.New("audit unavailable")}}
	router, _, token, rebuilder := setupAdminRAGRouterWithAudit(t, true, audit)
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	response := performAdminRAGRebuild(router, token)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, 1, rebuilder.calls)
	require.Contains(t, logs.String(), "rag rebuild result audit degraded")
	require.Contains(t, logs.String(), audit.entries[0].TargetID)
}

func TestAdminRAGRebuildFailureAuditDegradationKeepsSafeFailure(t *testing.T) {
	audit := &recordingAdminRAGAudit{failOn: map[string]error{"failed": errors.New("audit unavailable")}}
	router, _, token, rebuilder := setupAdminRAGRouterWithAudit(t, true, audit)
	rebuilder.err = errors.New("provider secret must-not-leak")
	response := performAdminRAGRebuild(router, token)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.NotContains(t, response.Body.String(), "must-not-leak")
	require.Equal(t, []string{"started", "failed"}, []string{audit.entries[0].Result, audit.entries[1].Result})
}

func TestAdminRAGRebuildTerminalAuditSurvivesRequestCancellation(t *testing.T) {
	for _, result := range []string{"success", "failed"} {
		t.Run(result, func(t *testing.T) {
			audit := &recordingAdminRAGAudit{}
			router, _, token, rebuilder := setupAdminRAGRouterWithAudit(t, true, audit)
			ctx, cancel := context.WithCancel(context.Background())
			rebuilder.onCall = cancel
			if result == "failed" {
				rebuilder.err = errors.New("rebuild unavailable")
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rag/rebuild", nil).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, []string{"started", result}, []string{audit.entries[0].Result, audit.entries[1].Result})
			require.NoError(t, audit.contextErrors[0])
			require.NoError(t, audit.contextErrors[1], "terminal audit must detach from client cancellation")
		})
	}
}

func performAdminRAGRebuild(router *gin.Engine, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/rag/rebuild", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)
	return recorder
}

func setupAdminRAGRouter(t *testing.T, enabled bool) (*gin.Engine, *gorm.DB, string, *recordingRAGRebuilder) {
	return setupAdminRAGRouterWithAudit(t, enabled, nil)
}

func setupAdminRAGRouterWithAudit(t *testing.T, enabled bool, customAudit AdminRAGAuditRecorder) (*gin.Engine, *gorm.DB, string, *recordingRAGRebuilder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AdminAuditLog{}))
	cfg := &config.Config{}
	cfg.JWT.Secret = "rag-rebuild-contract-secret"
	cfg.Features.RAGHybridEnabled = enabled
	cfg.RAG.Index.AuditTimeoutSec = 2
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	if customAudit == nil {
		customAudit = auditSvc
	}
	rebuilder := &recordingRAGRebuilder{}
	handler := NewAdminRAGHandler(cfg, rebuilder, customAudit)
	admin := model.User{Email: "rag-admin@example.com", Username: "rag-admin", PasswordHash: "hash", Role: "admin"}
	require.NoError(t, db.Create(&admin).Error)
	token := mustToken(t, cfg, admin.ID, admin.Role)
	router := gin.New()
	group := router.Group("/api/v1/admin", middleware.AuthRequired(cfg, nil, db), middleware.AdminRequired())
	group.POST("/rag/rebuild", handler.Rebuild)
	return router, db, token, rebuilder
}
