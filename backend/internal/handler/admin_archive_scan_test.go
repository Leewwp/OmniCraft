package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func TestAdminArchiveScanManualReviewRequiresReasonAndWritesAudit(t *testing.T) {
	db, repo, job := setupAdminArchiveScan(t)
	audit := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	h := NewAdminArchiveScanHandler(db, repo, audit, &fakeArchiveRestoreStore{existsKeys: map[string]bool{"quarantine/archive-scan/1/1/1": true}})
	router := gin.New()
	router.POST("/admin/archive-scan-jobs/:id/manual-review", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(42))
		c.Set(middleware.UserRoleKey, "admin")
		h.StartManualReview(c)
	})

	missingReason := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/manual-review", bytes.NewBufferString(`{"reason":"  "}`))
	router.ServeHTTP(missingReason, request)
	if missingReason.Code != http.StatusBadRequest || !strings.Contains(missingReason.Body.String(), "REVIEW_REASON_REQUIRED") {
		t.Fatalf("missing reason response = %d %s, want 400 REVIEW_REASON_REQUIRED", missingReason.Code, missingReason.Body.String())
	}

	accepted := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/manual-review", bytes.NewBufferString(`{"reason":"false positive fixture"}`))
	router.ServeHTTP(accepted, request)
	if accepted.Code != http.StatusOK {
		t.Fatalf("manual review response = %d %s, want 200", accepted.Code, accepted.Body.String())
	}
	var updated model.ArchiveScanJob
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatalf("load updated job: %v", err)
	}
	if updated.Status != model.ScanStatusManualReview {
		t.Fatalf("job status = %q, want manual_review", updated.Status)
	}
	var auditLog model.AdminAuditLog
	if err := db.Where("action = ? AND target_id = ?", "archive_scan_manual_review", strconv.FormatInt(job.ID, 10)).First(&auditLog).Error; err != nil {
		t.Fatalf("load manual-review audit: %v", err)
	}
	if auditLog.Metadata["reason"] != "false positive fixture" {
		t.Fatalf("audit metadata = %#v, want review reason", auditLog.Metadata)
	}
}

func TestAdminArchiveScanManualReviewWaitsForQuarantineCleanup(t *testing.T) {
	db, repo, job := setupAdminArchiveScan(t)
	store := &fakeArchiveRestoreStore{exists: true}
	h := NewAdminArchiveScanHandler(db, repo, service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db), store)
	router := gin.New()
	router.POST("/admin/archive-scan-jobs/:id/manual-review", h.StartManualReview)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/manual-review", bytes.NewBufferString(`{"reason":"false positive fixture"}`))
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "ARCHIVE_QUARANTINE_CLEANUP_PENDING") {
		t.Fatalf("manual review response = %d %s, want 409 ARCHIVE_QUARANTINE_CLEANUP_PENDING", recorder.Code, recorder.Body.String())
	}
	updated, err := repo.GetJob(t.Context(), job.ID)
	if err != nil {
		t.Fatalf("load blocked job: %v", err)
	}
	if updated.Status != model.ScanStatusBlocked {
		t.Fatalf("job status = %q, want blocked", updated.Status)
	}
}

func TestAdminArchiveScanResolveRejectsIllegalOutcomeAndResolvesClean(t *testing.T) {
	db, repo, job := setupAdminArchiveScan(t)
	if err := repo.StartManualReview(t.Context(), job.ID); err != nil {
		t.Fatalf("start review fixture: %v", err)
	}
	notifier := &fakeArchiveScanCompletionNotifier{}
	h := NewAdminArchiveScanHandler(db, repo, service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db), &fakeArchiveRestoreStore{})
	h.SetArchiveScanCompletionNotifier(notifier)
	router := gin.New()
	router.POST("/admin/archive-scan-jobs/:id/resolve", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(42))
		c.Set(middleware.UserRoleKey, "admin")
		h.ResolveManualReview(c)
	})

	invalid := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/resolve", bytes.NewBufferString(`{"outcome":"published","reason":"bad outcome"}`))
	router.ServeHTTP(invalid, request)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "INVALID_OUTCOME") {
		t.Fatalf("invalid outcome response = %d %s, want 400 INVALID_OUTCOME", invalid.Code, invalid.Body.String())
	}

	resolved := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/resolve", bytes.NewBufferString(`{"outcome":"clean","reason":"reviewed as false positive"}`))
	router.ServeHTTP(resolved, request)
	if resolved.Code != http.StatusOK {
		t.Fatalf("resolve response = %d %s, want 200", resolved.Code, resolved.Body.String())
	}
	var updated model.ArchiveScanJob
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatalf("load resolved job: %v", err)
	}
	if updated.Status != model.ScanStatusClean {
		t.Fatalf("resolved status = %q, want clean", updated.Status)
	}
	if notifier.calls != 1 || notifier.attachmentID != job.AttachmentID {
		t.Fatalf("completion notification = calls %d attachment %d, want one for %d", notifier.calls, notifier.attachmentID, job.AttachmentID)
	}
}

func TestAdminArchiveScanResolveCompensatesRestoreWhenAuditFails(t *testing.T) {
	db, repo, job := setupAdminArchiveScan(t)
	if err := repo.StartManualReview(t.Context(), job.ID); err != nil {
		t.Fatalf("start review fixture: %v", err)
	}
	store := &fakeArchiveRestoreStore{}
	h := NewAdminArchiveScanHandler(db, repo, failingArchiveScanAudit{}, store)
	router := gin.New()
	router.POST("/admin/archive-scan-jobs/:id/resolve", h.ResolveManualReview)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/resolve", bytes.NewBufferString(`{"outcome":"clean","reason":"reviewed as false positive"}`))
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("resolve response = %d %s, want 500", recorder.Code, recorder.Body.String())
	}
	if store.deleteCalls != 1 {
		t.Fatalf("restore compensation delete calls = %d, want 1", store.deleteCalls)
	}
	updated, err := repo.GetJob(t.Context(), job.ID)
	if err != nil {
		t.Fatalf("load review job: %v", err)
	}
	if updated.Status != model.ScanStatusManualReview {
		t.Fatalf("job status = %q, want manual_review after rollback", updated.Status)
	}
}

func TestAdminArchiveScanResolveReportsCompensationFailure(t *testing.T) {
	db, repo, job := setupAdminArchiveScan(t)
	if err := repo.StartManualReview(t.Context(), job.ID); err != nil {
		t.Fatalf("start review fixture: %v", err)
	}
	store := &fakeArchiveRestoreStore{compensationErr: errors.New("restore rollback unavailable")}
	h := NewAdminArchiveScanHandler(db, repo, failingArchiveScanAudit{}, store)
	router := gin.New()
	router.POST("/admin/archive-scan-jobs/:id/resolve", h.ResolveManualReview)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/resolve", bytes.NewBufferString(`{"outcome":"clean","reason":"reviewed as false positive"}`))
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "ARCHIVE_RESTORE_ROLLBACK_FAILED") {
		t.Fatalf("resolve response = %d %s, want 500 ARCHIVE_RESTORE_ROLLBACK_FAILED", recorder.Code, recorder.Body.String())
	}
}

func TestAdminArchiveScanRetryFailedJob(t *testing.T) {
	db, repo, job := setupAdminArchiveScan(t)
	if err := db.Model(&model.ArchiveScanJob{}).Where("id = ?", job.ID).Updates(map[string]interface{}{
		"status":          model.ScanStatusFailed,
		"next_attempt_at": time.Now().Add(time.Minute),
	}).Error; err != nil {
		t.Fatalf("fail fixture: %v", err)
	}
	if err := db.Model(&model.ContentAttachment{}).Where("id = ?", job.AttachmentID).Update("scan_status", model.ScanStatusFailed).Error; err != nil {
		t.Fatalf("fail attachment fixture: %v", err)
	}
	h := NewAdminArchiveScanHandler(db, repo, service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db))
	router := gin.New()
	router.POST("/admin/archive-scan-jobs/:id/retry", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(42))
		c.Set(middleware.UserRoleKey, "admin")
		h.Retry(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/retry", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("retry response = %d %s, want 200", recorder.Code, recorder.Body.String())
	}
	updated, err := repo.GetJob(t.Context(), job.ID)
	if err != nil {
		t.Fatalf("load retried job: %v", err)
	}
	if updated.Status != model.ScanStatusPending {
		t.Fatalf("retried status = %q, want pending", updated.Status)
	}
}

func TestAdminArchiveScanRouteRequiresAdminRole(t *testing.T) {
	db, repo, job := setupAdminArchiveScan(t)
	h := NewAdminArchiveScanHandler(db, repo, nil)
	router := gin.New()
	router.POST("/admin/archive-scan-jobs/:id/manual-review", middleware.AdminRequired(), h.StartManualReview)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/archive-scan-jobs/"+strconv.FormatInt(job.ID, 10)+"/manual-review", bytes.NewBufferString(`{"reason":"not reached"}`))
	request = request.WithContext(request.Context())
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("non-admin response = %d %s, want 403", recorder.Code, recorder.Body.String())
	}
}

func setupAdminArchiveScan(t *testing.T) (*gorm.DB, *repository.ArchiveScanRepository, model.ArchiveScanJob) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.ContentAttachment{}, &model.ArchiveScanJob{}, &model.ArchiveScanAttempt{}, &model.AdminAuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	content := model.ContentItem{Title: "blocked mod", AuthorID: 1, Zone: "fanwork", ContentType: "mod", Status: "pending"}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}
	attachment := model.ContentAttachment{ContentItemID: content.ID, FileType: "mod", OSSKey: "uploads/mod.zip", ScanStatus: model.ScanStatusNotRequired}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	repo := repository.NewArchiveScanRepository(db, repository.ArchiveScanRetryPolicy{})
	job, err := repo.CreateJob(t.Context(), attachment.ID, 1)
	if err != nil {
		t.Fatalf("create scan job: %v", err)
	}
	if err := repo.StartScan(t.Context(), job.ID); err != nil {
		t.Fatalf("start scan: %v", err)
	}
	objectSHA256 := fmt.Sprintf("%x", sha256.Sum256([]byte("restored archive")))
	if err := repo.Block(t.Context(), job.ID, "EICAR-Test-Signature", "quarantine/archive-scan/1/1/1", objectSHA256); err != nil {
		t.Fatalf("block scan: %v", err)
	}
	return db, repo, *job
}

type fakeArchiveRestoreStore struct {
	exists          bool
	existsKeys      map[string]bool
	copyCalls       int
	deleteCalls     int
	compensationErr error
}

func (s *fakeArchiveRestoreStore) Copy(source, target string) error {
	s.copyCalls++
	if s.compensationErr != nil && strings.HasPrefix(target, "quarantine/") {
		return s.compensationErr
	}
	return nil
}

func (s *fakeArchiveRestoreStore) Open(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("restored archive")), nil
}

func (s *fakeArchiveRestoreStore) Delete(string) error {
	s.deleteCalls++
	return nil
}

func (s *fakeArchiveRestoreStore) Exists(key string) (bool, error) {
	if s.existsKeys != nil {
		return s.existsKeys[key], nil
	}
	return s.exists, nil
}

type failingArchiveScanAudit struct{}

type fakeArchiveScanCompletionNotifier struct {
	calls        int
	attachmentID int64
}

func (n *fakeArchiveScanCompletionNotifier) ArchiveScanClean(_ context.Context, attachmentID int64) error {
	n.calls++
	n.attachmentID = attachmentID
	return nil
}

func (failingArchiveScanAudit) Record(context.Context, service.RecordAdminAuditInput) error {
	return errors.New("audit unavailable")
}

func (failingArchiveScanAudit) RecordTx(context.Context, *gorm.DB, service.RecordAdminAuditInput) error {
	return errors.New("audit unavailable")
}
