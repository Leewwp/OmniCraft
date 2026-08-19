package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/clamav"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

type fakeArchiveScanRepository struct {
	job         model.ArchiveScanJob
	attachment  model.ContentAttachment
	attempts    []model.ArchiveScanAttempt
	transitions []string
	failCount   int
	failBackoff time.Duration
	retryErr    error
}

func (r *fakeArchiveScanRepository) GetJob(context.Context, int64) (*model.ArchiveScanJob, error) {
	job := r.job
	return &job, nil
}

func (r *fakeArchiveScanRepository) GetAttachmentScanState(context.Context, int64) (*model.ContentAttachment, error) {
	attachment := r.attachment
	return &attachment, nil
}

func (r *fakeArchiveScanRepository) StartScan(context.Context, int64) error {
	r.transitions = append(r.transitions, "start")
	r.job.Status = model.ScanStatusScanning
	return nil
}

func (r *fakeArchiveScanRepository) FinishClean(_ context.Context, _ int64, engine, signatures, objectSHA256 string) error {
	r.transitions = append(r.transitions, "clean")
	r.job.Status = model.ScanStatusClean
	r.job.EngineVersion = engine
	r.job.SignatureVersion = signatures
	r.job.ObjectSHA256 = objectSHA256
	return nil
}

func (r *fakeArchiveScanRepository) Block(_ context.Context, _ int64, _ string, quarantineKey, objectSHA256 string) error {
	r.transitions = append(r.transitions, "blocked")
	r.job.Status = model.ScanStatusBlocked
	r.job.QuarantineKey = quarantineKey
	r.job.ObjectSHA256 = objectSHA256
	return nil
}

func (r *fakeArchiveScanRepository) Fail(context.Context, int64, string) error {
	r.transitions = append(r.transitions, "failed")
	r.failCount++
	r.job.Status = model.ScanStatusFailed
	if r.failBackoff > 0 {
		next := time.Now().Add(r.failBackoff)
		r.job.NextAttemptAt = &next
	} else {
		next := time.Now()
		r.job.NextAttemptAt = &next
	}
	return nil
}

func (r *fakeArchiveScanRepository) Retry(context.Context, int64) error {
	r.transitions = append(r.transitions, "retry")
	if r.retryErr != nil {
		return r.retryErr
	}
	r.job.Status = model.ScanStatusPending
	r.job.NextAttemptAt = nil
	return nil
}

func (r *fakeArchiveScanRepository) AppendAttempt(_ context.Context, attempt *model.ArchiveScanAttempt) error {
	r.attempts = append(r.attempts, *attempt)
	return nil
}

type fakeArchiveObjectStore struct {
	data        string
	steps       []string
	deleteErrs  []error
	deleteCalls int
}

func (s *fakeArchiveObjectStore) Open(string) (io.ReadCloser, error) {
	s.steps = append(s.steps, "open")
	return io.NopCloser(strings.NewReader(s.data)), nil
}

func (s *fakeArchiveObjectStore) Copy(source, target string) error {
	s.steps = append(s.steps, "copy:"+source+":"+target)
	return nil
}

func (s *fakeArchiveObjectStore) Delete(key string) error {
	s.steps = append(s.steps, "delete:"+key)
	if s.deleteCalls < len(s.deleteErrs) {
		err := s.deleteErrs[s.deleteCalls]
		s.deleteCalls++
		return err
	}
	s.deleteCalls++
	return nil
}

func (s *fakeArchiveObjectStore) Exists(string) (bool, error) { return false, nil }

type fakeArchiveScanner struct {
	results []clamav.Result
	errors  []error
	index   int
}

func (s *fakeArchiveScanner) Ping(context.Context) error { return nil }

func (s *fakeArchiveScanner) Version(context.Context) (clamav.Version, error) {
	return clamav.Version{Engine: "clamav-test", Signatures: "sig-test"}, nil
}

func (s *fakeArchiveScanner) Scan(_ context.Context, source io.Reader) (clamav.Result, error) {
	// A real clamd adapter consumes the complete stream before returning; keep
	// the fake honest so the worker's digest assertion exercises that contract.
	if source != nil {
		_, _ = io.Copy(io.Discard, source)
	}
	index := s.index
	s.index++
	if index < len(s.errors) && s.errors[index] != nil {
		return clamav.Result{}, s.errors[index]
	}
	if index >= len(s.results) {
		return clamav.Result{}, errors.New("fake scanner result missing")
	}
	return s.results[index], nil
}

func archiveScanMessage(jobID int64) queue.Message {
	return queue.Message{ID: "1-0", Group: "omnicraft-archive-scan", Payload: []byte(fmt.Sprintf(`{"job_id":%d}`, jobID))}
}

func TestArchiveScanWorkerCleanPathHashesAndAudits(t *testing.T) {
	repo := testArchiveScanRepository()
	objects := &fakeArchiveObjectStore{data: "clean archive"}
	scanner := &fakeArchiveScanner{results: []clamav.Result{{Status: clamav.StatusClean}}}
	worker := NewArchiveScanWorker(repo, objects, scanner, time.Second)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repo.job.Status != model.ScanStatusClean || len(repo.attempts) != 1 || repo.attempts[0].Result != model.ScanAttemptResultClean {
		t.Fatalf("repo state = %+v attempts = %+v", repo.job, repo.attempts)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256([]byte("clean archive")))
	if repo.job.ObjectSHA256 != wantHash {
		t.Fatalf("object hash = %q, want %q", repo.job.ObjectSHA256, wantHash)
	}
}

func TestArchiveScanWorkerNotifiesReviewPublicationAfterClean(t *testing.T) {
	repo := testArchiveScanRepository()
	objects := &fakeArchiveObjectStore{data: "clean archive"}
	scanner := &fakeArchiveScanner{results: []clamav.Result{{Status: clamav.StatusClean}}}
	notifier := &fakeArchiveScanCompletionNotifier{}
	worker := NewArchiveScanWorkerWithDBAndNotifier(repo, objects, scanner, time.Second, nil, notifier)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if notifier.attachmentID != repo.attachment.ID || notifier.calls != 1 {
		t.Fatalf("completion notification = calls %d attachment %d, want one for %d", notifier.calls, notifier.attachmentID, repo.attachment.ID)
	}
}

type fakeArchiveScanCompletionNotifier struct {
	calls        int
	attachmentID int64
}

func (n *fakeArchiveScanCompletionNotifier) ArchiveScanClean(_ context.Context, attachmentID int64) error {
	n.calls++
	n.attachmentID = attachmentID
	return nil
}

func TestArchiveScanWorkerQuarantinesBeforeDeletingBlockedObject(t *testing.T) {
	repo := testArchiveScanRepository()
	objects := &fakeArchiveObjectStore{data: "infected archive"}
	scanner := &fakeArchiveScanner{results: []clamav.Result{{Status: clamav.StatusBlocked, DetectionName: "Eicar-Test-Signature"}}}
	worker := NewArchiveScanWorker(repo, objects, scanner, time.Second)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(objects.steps) != 3 || !strings.HasPrefix(objects.steps[1], "copy:") || !strings.HasPrefix(objects.steps[2], "delete:") {
		t.Fatalf("object steps = %v, want open, copy, delete", objects.steps)
	}
	if repo.job.Status != model.ScanStatusBlocked || repo.attempts[0].Result != model.ScanAttemptResultBlocked {
		t.Fatalf("repo state = %+v attempts = %+v", repo.job, repo.attempts)
	}
	wantHash := fmt.Sprintf("%x", sha256.Sum256([]byte("infected archive")))
	if repo.job.ObjectSHA256 != wantHash {
		t.Fatalf("blocked object hash = %q, want %q", repo.job.ObjectSHA256, wantHash)
	}
}

func TestArchiveScanWorkerRetriesBlockedCleanupAfterDeleteFailure(t *testing.T) {
	repo := testArchiveScanRepository()
	objects := &fakeArchiveObjectStore{
		data:       "infected archive",
		deleteErrs: []error{errors.New("raw OSS delete failure")},
	}
	scanner := &fakeArchiveScanner{results: []clamav.Result{{Status: clamav.StatusBlocked, DetectionName: "Eicar-Test-Signature"}}}
	worker := NewArchiveScanWorker(repo, objects, scanner, time.Second)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err == nil {
		t.Fatal("first Handle() error = nil, want blocked cleanup failure to be retried")
	}
	if repo.job.Status != model.ScanStatusBlocked || len(repo.attempts) != 1 {
		t.Fatalf("after delete failure repo state = %+v attempts = %+v, want blocked with one audit", repo.job, repo.attempts)
	}

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("second Handle() error = %v, want cleanup retry to succeed", err)
	}
	if scanner.index != 1 {
		t.Fatalf("scanner calls = %d, want no rescan after blocked transition", scanner.index)
	}
	if got := strings.Join(objects.steps, ","); !strings.Contains(got, "open,copy:") || strings.Count(got, "delete:") != 2 {
		t.Fatalf("object steps = %v, want one quarantine copy and two cleanup attempts", objects.steps)
	}
}

func TestArchiveScanWorkerRetriesClamdFailureWithoutExposingRawError(t *testing.T) {
	repo := testArchiveScanRepository()
	objects := &fakeArchiveObjectStore{data: "eventually clean"}
	scanner := &fakeArchiveScanner{
		errors:  []error{errors.New("raw clamd connection refused")},
		results: []clamav.Result{{}, {Status: clamav.StatusClean}},
	}
	worker := NewArchiveScanWorker(repo, objects, scanner, time.Second)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if repo.job.Status != model.ScanStatusClean || repo.failCount != 1 || len(repo.attempts) != 2 {
		t.Fatalf("repo state = %+v failures = %d attempts = %+v", repo.job, repo.failCount, repo.attempts)
	}
	if repo.attempts[0].ErrorCode == "raw clamd connection refused" || repo.attempts[0].ErrorCode == "" {
		t.Fatalf("error audit code = %q, want sanitized non-empty code", repo.attempts[0].ErrorCode)
	}
}

func TestArchiveScanWorkerOwnsArchiveBackoffInsteadOfQueueRetryBudget(t *testing.T) {
	repo := testArchiveScanRepository()
	repo.failBackoff = 25 * time.Millisecond
	objects := &fakeArchiveObjectStore{data: "eventually clean"}
	scanner := &fakeArchiveScanner{
		errors:  []error{errors.New("raw clamd connection refused")},
		results: []clamav.Result{{}, {Status: clamav.StatusClean}},
	}
	worker := NewArchiveScanWorker(repo, objects, scanner, time.Second)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("Handle() error = %v, want archive retry to stay inside one queue delivery", err)
	}
	if repo.job.Status != model.ScanStatusClean || repo.failCount != 1 {
		t.Fatalf("repo state = %+v failures = %d, want one scheduled failure then clean", repo.job, repo.failCount)
	}
	if got := strings.Join(repo.transitions, ","); got != "start,failed,retry,start,clean" {
		t.Fatalf("transitions = %q, want archive-owned backoff sequence", got)
	}
}

func TestArchiveScanWorkerAcknowledgesExhaustedArchiveRetry(t *testing.T) {
	repo := testArchiveScanRepository()
	repo.retryErr = repository.ErrArchiveScanRetryExhausted
	objects := &fakeArchiveObjectStore{data: "permanently unavailable"}
	scanner := &fakeArchiveScanner{errors: []error{errors.New("raw clamd connection refused")}}
	worker := NewArchiveScanWorker(repo, objects, scanner, time.Second)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("Handle() error = %v, want terminal failed job to be acknowledged", err)
	}
	if repo.job.Status != model.ScanStatusFailed || repo.failCount != 1 {
		t.Fatalf("repo state = %+v failures = %d, want terminal failed state", repo.job, repo.failCount)
	}
}

func TestArchiveScanWorkerMarksEnvelopeConsumedAfterTerminalSideEffect(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))

	envelope, err := events.NewArchiveScanEnvelope(3, 7, "", "")
	if err != nil {
		t.Fatalf("NewArchiveScanEnvelope() error = %v", err)
	}
	envelope.EventID = 99
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal archive envelope: %v", err)
	}
	repo := testArchiveScanRepository()
	objects := &fakeArchiveObjectStore{data: "clean archive"}
	scanner := &fakeArchiveScanner{results: []clamav.Result{{Status: clamav.StatusClean}}}
	worker := NewArchiveScanWorkerWithDB(repo, objects, scanner, time.Second, db)
	msg := queue.Message{ID: "1-0", Group: ArchiveScanConsumerGroup, Payload: payload}

	if err := worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := worker.Handle(context.Background(), msg); err != nil {
		t.Fatalf("duplicate Handle() error = %v", err)
	}
	var count int64
	if err := db.Model(&model.InboxConsumer{}).Where("consumer_group = ? AND event_id = ?", ArchiveScanConsumerGroup, envelope.EventID).Count(&count).Error; err != nil {
		t.Fatalf("count archive inbox rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("archive inbox rows = %d, want 1", count)
	}
}

func TestArchiveScanWorkerRejectsMalformedMessage(t *testing.T) {
	worker := NewArchiveScanWorker(&fakeArchiveScanRepository{}, &fakeArchiveObjectStore{}, &fakeArchiveScanner{}, time.Second)
	if err := worker.Handle(context.Background(), queue.Message{Payload: []byte(`{"job_id":0}`)}); err == nil {
		t.Fatal("Handle() error = nil, want malformed job id error")
	}
}

func TestArchiveScanWorkerPreservesConfiguredTimeout(t *testing.T) {
	worker := NewArchiveScanWorker(&fakeArchiveScanRepository{}, &fakeArchiveObjectStore{}, &fakeArchiveScanner{}, 0)
	if worker.timeout != 0 {
		t.Fatalf("timeout = %s, want configured zero value", worker.timeout)
	}
}

func TestArchiveScanWorkerDuplicateTerminalDeliveryIsIdempotent(t *testing.T) {
	repo := testArchiveScanRepository()
	objects := &fakeArchiveObjectStore{data: "clean archive"}
	scanner := &fakeArchiveScanner{results: []clamav.Result{{Status: clamav.StatusClean}}}
	worker := NewArchiveScanWorker(repo, objects, scanner, time.Second)

	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := worker.Handle(context.Background(), archiveScanMessage(7)); err != nil {
		t.Fatalf("duplicate Handle() error = %v", err)
	}
	if len(repo.attempts) != 1 || len(objects.steps) != 1 {
		t.Fatalf("duplicate delivery repeated side effects: attempts=%d object_steps=%v", len(repo.attempts), objects.steps)
	}
}

func testArchiveScanRepository() *fakeArchiveScanRepository {
	return &fakeArchiveScanRepository{
		job: model.ArchiveScanJob{ID: 7, AttachmentID: 3, ScanVersion: 2, Status: model.ScanStatusPending},
		attachment: model.ContentAttachment{
			ID: 3, OSSKey: "mods/3/archive.zip", ScanRequired: true,
			ScanStatus: model.ScanStatusPending, ScanVersion: 2,
		},
	}
}
