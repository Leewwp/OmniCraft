package scheduler

import (
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
)

type BrowseHistoryCleanup struct {
	db      *gorm.DB
	cfg     *config.BrowseHistoryConfig
	repo    *repository.BrowseHistoryRepository
	now     func() time.Time
	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
}

type browseHistoryCleanupRunResult struct {
	Deleted        int64
	AcquiredLeader bool
}

func NewBrowseHistoryCleanup(db *gorm.DB, cfg *config.BrowseHistoryConfig) *BrowseHistoryCleanup {
	var repo *repository.BrowseHistoryRepository
	if db != nil {
		repo = repository.NewBrowseHistoryRepository(db)
	}
	return &BrowseHistoryCleanup{
		db:   db,
		cfg:  cfg,
		repo: repo,
		now:  time.Now,
	}
}

func (s *BrowseHistoryCleanup) Start() {
	s.mu.Lock()
	s.stopped = false
	s.mu.Unlock()
	s.scheduleNext()
}

func (s *BrowseHistoryCleanup) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *BrowseHistoryCleanup) runOnce() (int64, error) {
	result, err := s.runOnceWithStatus()
	return result.Deleted, err
}

func (s *BrowseHistoryCleanup) runOnceWithStatus() (browseHistoryCleanupRunResult, error) {
	if s.repo == nil {
		return browseHistoryCleanupRunResult{}, nil
	}
	now := s.now().In(s.location())
	deleted, acquired, err := s.repo.DeleteExpiredIfLeader(s.retentionDays(), now)
	result := browseHistoryCleanupRunResult{Deleted: deleted, AcquiredLeader: acquired}
	if err != nil {
		slog.Error("[BrowseHistoryCleanup] cleanup failed", "error", err)
		return result, err
	}
	if !acquired {
		slog.Info("[BrowseHistoryCleanup] cleanup skipped", "reason", "leader lock not acquired")
		return result, nil
	}
	slog.Info("[BrowseHistoryCleanup] cleanup completed", "deleted", deleted)
	return result, nil
}

func (s *BrowseHistoryCleanup) scheduleNext() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	now := s.now().In(s.location())
	next := s.nextRun(now)
	delay := next.Sub(now)
	s.scheduleAfter(delay, func() {
		_, _ = s.runOnce()
		s.scheduleNext()
	})
	slog.Info("[BrowseHistoryCleanup] scheduled", "next_run", next)
}

func (s *BrowseHistoryCleanup) scheduleAfter(delay time.Duration, callback func()) {
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

func (s *BrowseHistoryCleanup) nextRun(now time.Time) time.Time {
	loc := s.location()
	localNow := now.In(loc)
	hour, minute := s.cleanupHourMinute()
	next := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, loc)
	if !next.After(localNow) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func (s *BrowseHistoryCleanup) cleanupHourMinute() (int, int) {
	cleanupTime := "03:00"
	if s.cfg != nil && s.cfg.CleanupTime != "" {
		cleanupTime = s.cfg.CleanupTime
	}
	parsed, err := time.Parse("15:04", cleanupTime)
	if err != nil {
		slog.Warn("[BrowseHistoryCleanup] invalid cleanup_time, using fallback", "cleanup_time", cleanupTime)
		return 3, 0
	}
	return parsed.Hour(), parsed.Minute()
}

func (s *BrowseHistoryCleanup) retentionDays() int {
	if s.cfg != nil && s.cfg.RetentionDays > 0 {
		return s.cfg.RetentionDays
	}
	return 7
}

func (s *BrowseHistoryCleanup) location() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		slog.Warn("[BrowseHistoryCleanup] failed to load Asia/Shanghai, using fixed UTC+8", "error", err)
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}
