package scheduler

import (
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
)

// CollabInviteExpiry periodically expires pending collaboration invites older
// than collaboration.invite_expire_days. Only one replica performs a sweep:
// the update runs inside a transaction that first takes a PostgreSQL advisory
// leader lock, so replicas that lose the race record a skipped run and
// reschedule normally.
type CollabInviteExpiry struct {
	db      *gorm.DB
	cfg     *config.CollaborationConfig
	repo    *repository.CollabInviteRepository
	now     func() time.Time
	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
}

type collabInviteExpiryRunResult struct {
	Expired        int64
	AcquiredLeader bool
}

// collabInviteExpiryInterval is the self-rescheduling sweep cadence. The
// sweep itself is idempotent and leader-gated, so a fixed daily cadence is
// sufficient.
const collabInviteExpiryInterval = 24 * time.Hour

func NewCollabInviteExpiry(db *gorm.DB, cfg *config.CollaborationConfig) *CollabInviteExpiry {
	var repo *repository.CollabInviteRepository
	if db != nil {
		repo = repository.NewCollabInviteRepository(db)
	}
	return &CollabInviteExpiry{
		db:   db,
		cfg:  cfg,
		repo: repo,
		now:  time.Now,
	}
}

func (s *CollabInviteExpiry) Start() {
	s.mu.Lock()
	s.stopped = false
	s.mu.Unlock()
	s.scheduleNext()
}

// Stop cancels the pending timer callback. It is idempotent.
func (s *CollabInviteExpiry) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *CollabInviteExpiry) runOnce() (int64, error) {
	result, err := s.runOnceWithStatus()
	return result.Expired, err
}

func (s *CollabInviteExpiry) runOnceWithStatus() (collabInviteExpiryRunResult, error) {
	if s.repo == nil {
		return collabInviteExpiryRunResult{}, nil
	}
	now := s.now().In(s.location())
	expired, acquired, err := s.repo.ExpirePendingIfLeader(s.expireDays(), now)
	result := collabInviteExpiryRunResult{Expired: expired, AcquiredLeader: acquired}
	if err != nil {
		slog.Error("[CollabInviteExpiry] sweep failed", "error", err)
		return result, err
	}
	if !acquired {
		slog.Info("[CollabInviteExpiry] sweep skipped", "reason", "leader lock not acquired")
		return result, nil
	}
	slog.Info("[CollabInviteExpiry] sweep completed", "expired", expired)
	return result, nil
}

func (s *CollabInviteExpiry) scheduleNext() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.scheduleAfter(collabInviteExpiryInterval, func() {
		_, _ = s.runOnce()
		s.scheduleNext()
	})
}

func (s *CollabInviteExpiry) scheduleAfter(delay time.Duration, callback func()) {
	if delay < 0 {
		delay = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(delay, callback)
}

func (s *CollabInviteExpiry) expireDays() int {
	if s.cfg != nil && s.cfg.InviteExpireDays > 0 {
		return s.cfg.InviteExpireDays
	}
	return 7
}

func (s *CollabInviteExpiry) location() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		slog.Warn("[CollabInviteExpiry] failed to load Asia/Shanghai, using fixed UTC+8", "error", err)
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}
