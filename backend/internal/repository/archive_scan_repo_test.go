package repository

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
)

func TestArchiveScanRepositoryCreateJobTracksAttachmentState(t *testing.T) {
	db, modID, imageID := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	job, err := repo.CreateJob(context.Background(), modID, 3)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if job.Status != model.ScanStatusPending || job.AttachmentID != modID || job.ScanVersion != 3 {
		t.Fatalf("job = %#v, want pending job for attachment %d version 3", job, modID)
	}
	if job.Attempts != 0 {
		t.Fatalf("fresh job attempts = %d, want 0", job.Attempts)
	}

	att, err := repo.GetAttachmentScanState(context.Background(), modID)
	if err != nil {
		t.Fatalf("GetAttachmentScanState() error = %v", err)
	}
	if att.ScanStatus != model.ScanStatusPending || !att.ScanRequired || att.ScanVersion != 3 {
		t.Fatalf("attachment after CreateJob = (status=%s required=%v version=%d), want (pending true 3)",
			att.ScanStatus, att.ScanRequired, att.ScanVersion)
	}

	if _, err := repo.CreateJob(context.Background(), imageID, 1); !errors.Is(err, ErrArchiveScanNotScannable) {
		t.Fatalf("CreateJob on not_required attachment error = %v, want ErrArchiveScanNotScannable", err)
	}
	if _, err := repo.CreateJob(context.Background(), 9999, 1); !errors.Is(err, ErrArchiveScanNotFound) {
		t.Fatalf("CreateJob on missing attachment error = %v, want ErrArchiveScanNotFound", err)
	}
}

func TestArchiveScanRepositoryCurrentJobUniquenessPerAttachmentVersion(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	if _, err := repo.CreateJob(context.Background(), modID, 1); err != nil {
		t.Fatalf("first CreateJob() error = %v", err)
	}
	if _, err := repo.CreateJob(context.Background(), modID, 1); !errors.Is(err, ErrArchiveScanJobExists) {
		t.Fatalf("duplicate current job error = %v, want ErrArchiveScanJobExists", err)
	}

	// A different version is a different current-job key.
	if _, err := repo.CreateJob(context.Background(), modID, 2); err != nil {
		t.Fatalf("CreateJob for version 2 error = %v", err)
	}

	// Once the first job reaches a terminal state a new job for the same
	// version becomes possible (admin-initiated re-scan after failure).
	first, err := repo.GetCurrentJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("GetCurrentJob() error = %v", err)
	}
	if err := repo.StartScan(context.Background(), first.ID); err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}
	if err := repo.FinishClean(context.Background(), first.ID, "clamav-1.0", "sig-20260813", "sha256:abc"); err != nil {
		t.Fatalf("FinishClean() error = %v", err)
	}
	if _, err := repo.CreateJob(context.Background(), modID, 1); err != nil {
		t.Fatalf("CreateJob after terminal job error = %v", err)
	}
}

func TestArchiveScanRepositoryHappyPathEndsClean(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	job, err := repo.CreateJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.StartScan(context.Background(), job.ID); err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}
	started, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if started.Status != model.ScanStatusScanning || started.StartedAt == nil {
		t.Fatalf("job after StartScan = (status=%s started_at=%v), want scanning with started_at", started.Status, started.StartedAt)
	}

	if err := repo.FinishClean(context.Background(), job.ID, "clamav-1.2.1", "sig-20260813", "sha256:abc123"); err != nil {
		t.Fatalf("FinishClean() error = %v", err)
	}
	finished, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if finished.Status != model.ScanStatusClean || finished.FinishedAt == nil {
		t.Fatalf("job after FinishClean = (status=%s finished_at=%v), want clean with finished_at", finished.Status, finished.FinishedAt)
	}
	if finished.EngineVersion != "clamav-1.2.1" || finished.SignatureVersion != "sig-20260813" || finished.ObjectSHA256 != "sha256:abc123" {
		t.Fatalf("clean job evidence = %#v, want engine/signature/object hash recorded", finished)
	}

	att, err := repo.GetAttachmentScanState(context.Background(), modID)
	if err != nil {
		t.Fatalf("GetAttachmentScanState() error = %v", err)
	}
	if att.ScanStatus != model.ScanStatusClean || att.ScannedAt == nil || att.LastScanJobID == nil || *att.LastScanJobID != job.ID {
		t.Fatalf("attachment after clean = %#v, want clean with scanned_at and last_scan_job_id=%d", att, job.ID)
	}
}

func TestArchiveScanRepositoryBlockedFlowAndManualReview(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	job, err := repo.CreateJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.StartScan(context.Background(), job.ID); err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}
	if err := repo.Block(context.Background(), job.ID, "EICAR-Test-Signature", "quarantine/2026/08/13/a.zip"); err != nil {
		t.Fatalf("Block() error = %v", err)
	}
	blocked, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if blocked.Status != model.ScanStatusBlocked || blocked.DetectionName != "EICAR-Test-Signature" || blocked.QuarantineKey != "quarantine/2026/08/13/a.zip" || blocked.FinishedAt == nil {
		t.Fatalf("blocked job = %#v, want blocked with detection and quarantine key", blocked)
	}
	att, err := repo.GetAttachmentScanState(context.Background(), modID)
	if err != nil {
		t.Fatalf("GetAttachmentScanState() error = %v", err)
	}
	if att.ScanStatus != model.ScanStatusBlocked {
		t.Fatalf("attachment after Block = %q, want blocked", att.ScanStatus)
	}

	// manual_review is only reachable from blocked.
	if err := repo.StartManualReview(context.Background(), job.ID); err != nil {
		t.Fatalf("StartManualReview() error = %v", err)
	}
	review, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if review.Status != model.ScanStatusManualReview {
		t.Fatalf("job after StartManualReview = %q, want manual_review", review.Status)
	}

	// False positive: admin resolves to clean.
	if err := repo.ResolveManualReview(context.Background(), job.ID, model.ScanStatusClean); err != nil {
		t.Fatalf("ResolveManualReview(clean) error = %v", err)
	}
	released, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if released.Status != model.ScanStatusClean || released.FinishedAt == nil {
		t.Fatalf("released job = %#v, want clean with finished_at", released)
	}
	att, err = repo.GetAttachmentScanState(context.Background(), modID)
	if err != nil {
		t.Fatalf("GetAttachmentScanState() error = %v", err)
	}
	if att.ScanStatus != model.ScanStatusClean || att.LastScanJobID == nil || *att.LastScanJobID != job.ID {
		t.Fatalf("attachment after review release = %#v, want clean with last_scan_job_id", att)
	}
}

func TestArchiveScanRepositoryManualReviewCanConfirmBlocked(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	job := mustStartScannableJob(t, repo, modID)
	if err := repo.Block(context.Background(), job.ID, "EICAR-Test-Signature", "quarantine/2026/08/13/a.zip"); err != nil {
		t.Fatalf("Block() error = %v", err)
	}
	if err := repo.StartManualReview(context.Background(), job.ID); err != nil {
		t.Fatalf("StartManualReview() error = %v", err)
	}
	if err := repo.ResolveManualReview(context.Background(), job.ID, model.ScanStatusBlocked); err != nil {
		t.Fatalf("ResolveManualReview(blocked) error = %v", err)
	}
	confirmed, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if confirmed.Status != model.ScanStatusBlocked || confirmed.DetectionName != "EICAR-Test-Signature" {
		t.Fatalf("confirmed job = %#v, want blocked with detection preserved", confirmed)
	}
	att, err := repo.GetAttachmentScanState(context.Background(), modID)
	if err != nil {
		t.Fatalf("GetAttachmentScanState() error = %v", err)
	}
	if att.ScanStatus != model.ScanStatusBlocked {
		t.Fatalf("attachment after review confirmation = %q, want blocked", att.ScanStatus)
	}
}

func TestArchiveScanRepositoryIllegalTransitionsAreRejected(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	job, err := repo.CreateJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.StartScan(context.Background(), job.ID); err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}
	if err := repo.FinishClean(context.Background(), job.ID, "clamav-1.2.1", "sig-20260813", "sha256:abc"); err != nil {
		t.Fatalf("FinishClean() error = %v", err)
	}

	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"StartScan on clean job", func() error { return repo.StartScan(context.Background(), job.ID) }},
		{"FinishClean on clean job", func() error { return repo.FinishClean(context.Background(), job.ID, "e", "s", "h") }},
		{"Block on clean job", func() error { return repo.Block(context.Background(), job.ID, "d", "q") }},
		{"Fail on clean job", func() error { return repo.Fail(context.Background(), job.ID, "ENGINE_ERR") }},
		{"Retry on clean job", func() error { return repo.Retry(context.Background(), job.ID) }},
		{"StartManualReview on clean job", func() error { return repo.StartManualReview(context.Background(), job.ID) }},
		{"ResolveManualReview on clean job", func() error { return repo.ResolveManualReview(context.Background(), job.ID, model.ScanStatusClean) }},
	} {
		if err := tc.run(); !errors.Is(err, ErrArchiveScanIllegalState) {
			t.Fatalf("%s error = %v, want ErrArchiveScanIllegalState", tc.name, err)
		}
	}

	// A pending job can only go to scanning: fail/block/clean/manual_review
	// from pending are all illegal.
	pending, err := repo.CreateJob(context.Background(), modID, 2)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.Block(context.Background(), pending.ID, "d", "q"); !errors.Is(err, ErrArchiveScanIllegalState) {
		t.Fatalf("Block on pending job error = %v, want ErrArchiveScanIllegalState", err)
	}
	if err := repo.Fail(context.Background(), pending.ID, "ENGINE_ERR"); !errors.Is(err, ErrArchiveScanIllegalState) {
		t.Fatalf("Fail on pending job error = %v, want ErrArchiveScanIllegalState", err)
	}
	if err := repo.ResolveManualReview(context.Background(), pending.ID, model.ScanStatusClean); !errors.Is(err, ErrArchiveScanIllegalState) {
		t.Fatalf("ResolveManualReview on pending job error = %v, want ErrArchiveScanIllegalState", err)
	}

	// Blocked can only go to manual_review, never straight to clean.
	if err := repo.StartScan(context.Background(), pending.ID); err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}
	if err := repo.Block(context.Background(), pending.ID, "d", "q"); err != nil {
		t.Fatalf("Block() error = %v", err)
	}
	if err := repo.ResolveManualReview(context.Background(), pending.ID, model.ScanStatusClean); !errors.Is(err, ErrArchiveScanIllegalState) {
		t.Fatalf("ResolveManualReview on blocked job error = %v, want ErrArchiveScanIllegalState", err)
	}

	if err := repo.StartScan(context.Background(), 9999); !errors.Is(err, ErrArchiveScanNotFound) {
		t.Fatalf("StartScan on missing job error = %v, want ErrArchiveScanNotFound", err)
	}
}

func TestArchiveScanRepositoryBoundedRetryKeepsFailedOnExhaustion(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	policy := ArchiveScanRetryPolicy{Backoff: []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}}
	repo := NewArchiveScanRepository(db, policy)

	job, err := repo.CreateJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.StartScan(context.Background(), job.ID); err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}

	// scanning -> failed -> pending (retry) -> scanning -> ... three retries.
	for retry := 0; retry < 3; retry++ {
		if err := repo.Fail(context.Background(), job.ID, "ENGINE_UNREACHABLE"); err != nil {
			t.Fatalf("Fail(retry %d) error = %v", retry, err)
		}
		failed, err := repo.GetJob(context.Background(), job.ID)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}
		if failed.Status != model.ScanStatusFailed {
			t.Fatalf("job after Fail(retry %d) = %q, want failed", retry, failed.Status)
		}
		if failed.NextAttemptAt == nil {
			t.Fatalf("retry %d: next_attempt_at must be scheduled while the retry budget lasts", retry)
		}
		expectedDelay := policy.Backoff[retry]
		if delay := failed.NextAttemptAt.Sub(time.Now()); delay < expectedDelay-time.Minute {
			t.Fatalf("retry %d next attempt delay = %v, want >= %v (backoff schedule)", retry, delay, expectedDelay)
		}
		if err := repo.Retry(context.Background(), job.ID); err != nil {
			t.Fatalf("Retry(retry %d) error = %v", retry, err)
		}
		pending, err := repo.GetJob(context.Background(), job.ID)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}
		if pending.Status != model.ScanStatusPending || pending.Attempts != retry+1 {
			t.Fatalf("job after Retry(retry %d) = (status=%s attempts=%d), want pending attempts=%d", retry, pending.Status, pending.Attempts, retry+1)
		}
		if err := repo.StartScan(context.Background(), job.ID); err != nil {
			t.Fatalf("StartScan(retry %d) error = %v", retry, err)
		}
	}

	// Fourth failure exceeds the 3-entry backoff schedule: stays failed, no
	// next_attempt_at scheduled, retry refused.
	if err := repo.Fail(context.Background(), job.ID, "ENGINE_UNREACHABLE"); err != nil {
		t.Fatalf("final Fail() error = %v", err)
	}
	final, err := repo.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if final.Status != model.ScanStatusFailed || final.NextAttemptAt != nil {
		t.Fatalf("exhausted job = (status=%s next_attempt_at=%v), want failed without next_attempt_at", final.Status, final.NextAttemptAt)
	}
	if err := repo.Retry(context.Background(), job.ID); !errors.Is(err, ErrArchiveScanRetryExhausted) {
		t.Fatalf("Retry on exhausted job error = %v, want ErrArchiveScanRetryExhausted", err)
	}
	// Retry exhaustion never auto-releases: the attachment stays failed.
	att, err := repo.GetAttachmentScanState(context.Background(), modID)
	if err != nil {
		t.Fatalf("GetAttachmentScanState() error = %v", err)
	}
	if att.ScanStatus != model.ScanStatusFailed {
		t.Fatalf("attachment after retry exhaustion = %q, want failed (no auto-release)", att.ScanStatus)
	}
}

func TestArchiveScanRepositoryAttemptAuditIsAppendOnly(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	job, err := repo.CreateJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	payloads := []model.ArchiveScanAttempt{
		{ScanJobID: job.ID, Result: model.ScanAttemptResultClean, DurationMs: 42, EngineVersion: "clamav-1.2.1", SignatureVersion: "sig-20260813"},
		{ScanJobID: job.ID, Result: model.ScanAttemptResultBlocked, DurationMs: 87, DetectionName: "EICAR-Test-Signature"},
		{ScanJobID: job.ID, Result: model.ScanAttemptResultError, DurationMs: 15, ErrorCode: "ENGINE_UNREACHABLE"},
	}
	for i := range payloads {
		if err := repo.AppendAttempt(context.Background(), &payloads[i]); err != nil {
			t.Fatalf("AppendAttempt(%d) error = %v", i, err)
		}
	}

	attempts, err := repo.ListAttemptsByJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("ListAttemptsByJob() error = %v", err)
	}
	if len(attempts) != len(payloads) {
		t.Fatalf("attempts = %d rows, want %d", len(attempts), len(payloads))
	}
	for i, got := range attempts {
		if got.AttemptNo != i+1 {
			t.Fatalf("attempt[%d].attempt_no = %d, want %d (monotonic audit numbering)", i, got.AttemptNo, i+1)
		}
		if got.Result != payloads[i].Result || got.DurationMs != payloads[i].DurationMs {
			t.Fatalf("attempt[%d] = %#v, want payload %#v (append-only rows must round-trip untouched)", i, got, payloads[i])
		}
		if got.EngineVersion != payloads[i].EngineVersion || got.SignatureVersion != payloads[i].SignatureVersion ||
			got.DetectionName != payloads[i].DetectionName || got.ErrorCode != payloads[i].ErrorCode {
			t.Fatalf("attempt[%d] evidence = %#v, want payload %#v", i, got, payloads[i])
		}
	}

	// The repository exposes no update or delete path: the only mutation is
	// append. A direct database-level tamper check guards against silent
	// drift of the immutable contract.
	var tampered bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'archive_scan_attempts'
			  AND column_name IN ('updated_at', 'deleted_at')
		)
	`).Scan(&tampered).Error; err != nil {
		t.Fatalf("inspect attempt audit schema: %v", err)
	}
	if tampered {
		t.Fatal("archive_scan_attempts must stay append-only: no updated_at/deleted_at columns")
	}

	if err := repo.AppendAttempt(context.Background(), &model.ArchiveScanAttempt{ScanJobID: 9999, Result: model.ScanAttemptResultClean, DurationMs: 1}); !errors.Is(err, ErrArchiveScanNotFound) {
		t.Fatalf("AppendAttempt on missing job error = %v, want ErrArchiveScanNotFound", err)
	}
}

func TestArchiveScanRepositoryConcurrentCreateJobAllowsSingleCurrentJob(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	const workers = 8
	var wg sync.WaitGroup
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.CreateJob(context.Background(), modID, 1)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	created, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			created++
		case errors.Is(err, ErrArchiveScanJobExists):
			conflicts++
		default:
			t.Fatalf("concurrent CreateJob error = %v", err)
		}
	}
	if created != 1 || conflicts != workers-1 {
		t.Fatalf("concurrent CreateJob = (created=%d conflicts=%d), want (1, %d)", created, conflicts, workers-1)
	}
	current, err := repo.GetCurrentJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("GetCurrentJob() error = %v", err)
	}
	if current.Status != model.ScanStatusPending {
		t.Fatalf("surviving current job = %q, want pending", current.Status)
	}
}

func TestArchiveScanRepositoryQueries(t *testing.T) {
	db, modID, _ := setupArchiveScanRepositoryDB(t)
	repo := NewArchiveScanRepository(db, ArchiveScanRetryPolicy{})

	first, err := repo.CreateJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	second, err := repo.CreateJob(context.Background(), modID, 2)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	current, err := repo.GetCurrentJob(context.Background(), modID, 1)
	if err != nil {
		t.Fatalf("GetCurrentJob() error = %v", err)
	}
	if current.ID != first.ID {
		t.Fatalf("GetCurrentJob(v1) = job %d, want %d", current.ID, first.ID)
	}
	if _, err := repo.GetCurrentJob(context.Background(), modID, 7); !errors.Is(err, ErrArchiveScanNotFound) {
		t.Fatalf("GetCurrentJob(v7) error = %v, want ErrArchiveScanNotFound", err)
	}

	jobs, err := repo.ListJobsByAttachment(context.Background(), modID)
	if err != nil {
		t.Fatalf("ListJobsByAttachment() error = %v", err)
	}
	if len(jobs) != 2 || jobs[0].ID != second.ID || jobs[1].ID != first.ID {
		t.Fatalf("jobs = %#v, want newest-first [%d %d]", jobs, second.ID, first.ID)
	}

	if _, err := repo.GetJob(context.Background(), 9999); !errors.Is(err, ErrArchiveScanNotFound) {
		t.Fatalf("GetJob(missing) error = %v, want ErrArchiveScanNotFound", err)
	}
}

func mustStartScannableJob(t *testing.T, repo *ArchiveScanRepository, attachmentID int64) *model.ArchiveScanJob {
	t.Helper()
	job, err := repo.CreateJob(context.Background(), attachmentID, 1)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.StartScan(context.Background(), job.ID); err != nil {
		t.Fatalf("StartScan() error = %v", err)
	}
	return job
}

func setupArchiveScanRepositoryDB(t *testing.T) (*gorm.DB, int64, int64) {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	if err := db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL
		);
		CREATE TABLE content_items (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			zone VARCHAR(10) NOT NULL,
			content_type VARCHAR(20) NOT NULL,
			cover_image_url TEXT,
			status VARCHAR(20) NOT NULL DEFAULT 'pending'
		);
		CREATE TABLE content_attachments (
			id BIGSERIAL PRIMARY KEY,
			content_item_id BIGINT NOT NULL REFERENCES content_items(id) ON DELETE CASCADE,
			file_type VARCHAR(30) NOT NULL,
			oss_key TEXT NOT NULL,
			file_size BIGINT,
			mime_type VARCHAR(100),
			duration_sec INT,
			width INT,
			height INT,
			is_primary BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO users (id, email, password_hash, username)
		VALUES (1, 'archive-owner@example.test', 'hash', 'archive-owner');
		INSERT INTO content_items (id, title, author_id, zone, content_type)
		VALUES (1, 'archive item', 1, 'original', 'mod');
		INSERT INTO content_attachments (content_item_id, file_type, oss_key)
		VALUES (1, 'mod', 'uploads/1/mod/a.zip');
		INSERT INTO content_attachments (content_item_id, file_type, oss_key)
		VALUES (1, 'image', 'uploads/1/image/a.png');
	`).Error; err != nil {
		t.Fatalf("create archive scan repository base tables: %v", err)
	}

	migration := filepath.Join("..", "..", "migrations", "072_archive_malware_scanning.sql")
	testutil.ApplyMigrationFile(t, db, migration)

	var modID, imageID int64
	if err := db.Raw(`SELECT id FROM content_attachments WHERE file_type = 'mod'`).Scan(&modID).Error; err != nil {
		t.Fatalf("load mod attachment: %v", err)
	}
	if err := db.Raw(`SELECT id FROM content_attachments WHERE file_type = 'image'`).Scan(&imageID).Error; err != nil {
		t.Fatalf("load image attachment: %v", err)
	}
	return db, modID, imageID
}
