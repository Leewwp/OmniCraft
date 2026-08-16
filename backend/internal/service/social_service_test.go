package service

import (
	"bytes"
	"context"
	"database/sql/driver"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	sqlitedriver "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/repository"
)

var registerSQLiteNowOnce sync.Once

// registerSQLiteNow makes Postgres-flavored NOW() available on the sqlite
// driver so repository statements like EditComment's gorm.Expr("NOW()") run.
func registerSQLiteNow() {
	registerSQLiteNowOnce.Do(func() {
		_ = sqlitedriver.RegisterScalarFunction("NOW", 0, func(_ *sqlitedriver.FunctionContext, _ []driver.Value) (driver.Value, error) {
			return time.Now().Format("2006-01-02 15:04:05.999999999"), nil
		})
	})
}

func setupSocialServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	registerSQLiteNow()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user model: %v", err)
	}
	// model.Comment and model.Discussion carry defaults GORM renders as
	// `DEFAULT NOW()`, which is not valid SQLite DDL, so both tables are
	// created here with sqlite-compatible DDL.
	if err := db.Exec(`
		CREATE TABLE comments (
			id integer PRIMARY KEY AUTOINCREMENT,
			content_item_id integer,
			discussion_id integer,
			parent_id integer,
			author_id integer NOT NULL,
			target_type text,
			target_id integer,
			content text,
			body text NOT NULL,
			status text NOT NULL DEFAULT 'published',
			like_count integer NOT NULL DEFAULT 0,
			created_at datetime,
			updated_at datetime
		)`).Error; err != nil {
		t.Fatalf("create comments table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE discussions (
			id integer PRIMARY KEY AUTOINCREMENT,
			ip_id integer,
			content_item_id integer,
			author_id integer NOT NULL,
			title text NOT NULL,
			body text,
			status text NOT NULL DEFAULT 'published',
			is_pinned numeric NOT NULL DEFAULT 0,
			view_count integer NOT NULL DEFAULT 0,
			reply_count integer NOT NULL DEFAULT 0,
			last_active_at datetime NOT NULL DEFAULT (datetime('now')),
			created_at datetime,
			updated_at datetime
		)`).Error; err != nil {
		t.Fatalf("create discussions table: %v", err)
	}
	return db
}

func newTestSocialService(db *gorm.DB, mode string, reviewer TextReviewer) *SocialService {
	cfg := &config.Config{Server: config.ServerConfig{Mode: mode}}
	return NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		cfg,
		nil,
		reviewer,
	)
}

func seedTestSocialUser(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	u := model.User{Email: "author@example.com", Username: "author", Reputation: 10}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u.ID
}

func countSocialComments(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.Comment{}).Count(&n).Error; err != nil {
		t.Fatalf("count comments: %v", err)
	}
	return n
}

func countSocialDiscussions(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.Discussion{}).Count(&n).Error; err != nil {
		t.Fatalf("count discussions: %v", err)
	}
	return n
}

type fakeTextReviewer struct {
	result string
	err    error
	calls  []string
}

func (f *fakeTextReviewer) ReviewText(_ context.Context, text string) (string, error) {
	f.calls = append(f.calls, text)
	if f.err != nil {
		return "", f.err
	}
	return f.result, nil
}

func captureSocialLogs(t *testing.T, fn func()) []string {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)
	fn()
	return strings.Split(buf.String(), "\n")
}

func TestPostCommentRejectsBlockedTextAndDoesNotPersist(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)
	reviewer := &fakeTextReviewer{result: "block"}
	svc := newTestSocialService(db, "debug", reviewer)

	_, err := svc.PostComment(context.Background(), PostCommentInput{Body: "this comment is a violation"}, authorID)
	if !errors.Is(err, ErrTextBlocked) {
		t.Fatalf("PostComment() error = %v, want ErrTextBlocked", err)
	}
	if got := countSocialComments(t, db); got != 0 {
		t.Fatalf("comment count = %d, want 0 (blocked text must not be persisted)", got)
	}
	if len(reviewer.calls) != 1 || !strings.Contains(reviewer.calls[0], "violation") {
		t.Fatalf("moderation calls = %v, want one call with the comment body", reviewer.calls)
	}
}

func TestPostCommentAllowsPassAndReviewText(t *testing.T) {
	for _, result := range []string{"pass", "review"} {
		t.Run(result, func(t *testing.T) {
			db := setupSocialServiceTestDB(t)
			authorID := seedTestSocialUser(t, db)
			svc := newTestSocialService(db, "debug", &fakeTextReviewer{result: result})

			got, err := svc.PostComment(context.Background(), PostCommentInput{Body: "perfectly fine comment"}, authorID)
			if err != nil {
				t.Fatalf("PostComment() error = %v, want nil", err)
			}
			if got.Body != "perfectly fine comment" {
				t.Fatalf("comment body = %q, want %q", got.Body, "perfectly fine comment")
			}
			if n := countSocialComments(t, db); n != 1 {
				t.Fatalf("comment count = %d, want 1", n)
			}
		})
	}
}

func TestPostDiscussionRejectsBlockedTextAndDoesNotPersist(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)
	reviewer := &fakeTextReviewer{result: "block"}
	svc := newTestSocialService(db, "debug", reviewer)

	_, err := svc.PostDiscussion(context.Background(), PostDiscussionInput{Title: "Great mod", Body: "but contains violation"}, authorID)
	if !errors.Is(err, ErrTextBlocked) {
		t.Fatalf("PostDiscussion() error = %v, want ErrTextBlocked", err)
	}
	if got := countSocialDiscussions(t, db); got != 0 {
		t.Fatalf("discussion count = %d, want 0 (blocked text must not be persisted)", got)
	}
	if len(reviewer.calls) != 1 || !strings.Contains(reviewer.calls[0], "Great mod") || !strings.Contains(reviewer.calls[0], "violation") {
		t.Fatalf("moderation calls = %v, want one call covering title and body", reviewer.calls)
	}
}

func TestPostDiscussionAllowsPassAndReviewText(t *testing.T) {
	for _, result := range []string{"pass", "review"} {
		t.Run(result, func(t *testing.T) {
			db := setupSocialServiceTestDB(t)
			authorID := seedTestSocialUser(t, db)
			svc := newTestSocialService(db, "debug", &fakeTextReviewer{result: result})

			got, err := svc.PostDiscussion(context.Background(), PostDiscussionInput{Title: "Great mod", Body: "all good"}, authorID)
			if err != nil {
				t.Fatalf("PostDiscussion() error = %v, want nil", err)
			}
			if got.Title != "Great mod" || got.Body != "all good" {
				t.Fatalf("discussion = %q/%q, want %q/%q", got.Title, got.Body, "Great mod", "all good")
			}
			if n := countSocialDiscussions(t, db); n != 1 {
				t.Fatalf("discussion count = %d, want 1", n)
			}
		})
	}
}

func TestEditCommentRejectsBlockedTextAndKeepsOriginal(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)
	svc := newTestSocialService(db, "debug", &fakeTextReviewer{result: "pass"})

	created, err := svc.PostComment(context.Background(), PostCommentInput{Body: "original safe text"}, authorID)
	if err != nil {
		t.Fatalf("seed comment: %v", err)
	}

	blockingSvc := newTestSocialService(db, "debug", &fakeTextReviewer{result: "block"})
	if _, err := blockingSvc.EditComment(context.Background(), created.ID, authorID, "edited violation text"); !errors.Is(err, ErrTextBlocked) {
		t.Fatalf("EditComment() error = %v, want ErrTextBlocked", err)
	}

	var stored model.Comment
	if err := db.First(&stored, created.ID).Error; err != nil {
		t.Fatalf("reload comment: %v", err)
	}
	if stored.Body != "original safe text" {
		t.Fatalf("stored body = %q, want %q (blocked edit must not change original)", stored.Body, "original safe text")
	}
}

func TestEditCommentAllowsPassAndReviewText(t *testing.T) {
	for _, result := range []string{"pass", "review"} {
		t.Run(result, func(t *testing.T) {
			db := setupSocialServiceTestDB(t)
			authorID := seedTestSocialUser(t, db)
			svc := newTestSocialService(db, "debug", &fakeTextReviewer{result: result})

			created, err := svc.PostComment(context.Background(), PostCommentInput{Body: "original"}, authorID)
			if err != nil {
				t.Fatalf("seed comment: %v", err)
			}
			got, err := svc.EditComment(context.Background(), created.ID, authorID, "edited content")
			if err != nil {
				t.Fatalf("EditComment() error = %v, want nil", err)
			}
			if got.Body != "edited content" {
				t.Fatalf("returned body = %q, want %q", got.Body, "edited content")
			}
			var stored model.Comment
			if err := db.First(&stored, created.ID).Error; err != nil {
				t.Fatalf("reload comment: %v", err)
			}
			if stored.Body != "edited content" {
				t.Fatalf("stored body = %q, want %q", stored.Body, "edited content")
			}
		})
	}
}

func TestPostCommentFailClosedWhenModerationFailsInReleaseMode(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)
	svc := newTestSocialService(db, "release", &fakeTextReviewer{err: errors.New("green api error")})

_, err := svc.PostComment(context.Background(), PostCommentInput{Body: "some comment"}, authorID)
	if !errors.Is(err, ErrModerationUnavailable) {
		t.Fatalf("PostComment() error = %v, want ErrModerationUnavailable", err)
	}
	if got := countSocialComments(t, db); got != 0 {
		t.Fatalf("comment count = %d, want 0 (fail-closed must not persist)", got)
	}

	_, err = svc.PostDiscussion(context.Background(), PostDiscussionInput{Title: "some discussion"}, authorID)
	if !errors.Is(err, ErrModerationUnavailable) {
		t.Fatalf("PostDiscussion() error = %v, want ErrModerationUnavailable", err)
	}
	if got := countSocialDiscussions(t, db); got != 0 {
		t.Fatalf("discussion count = %d, want 0 (fail-closed must not persist)", got)
	}
}

func TestPostCommentFailClosedWhenReviewerMissingInReleaseMode(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)
	svc := newTestSocialService(db, "release", nil)

	if _, err := svc.PostComment(context.Background(), PostCommentInput{Body: "some comment"}, authorID); !errors.Is(err, ErrModerationUnavailable) {
		t.Fatalf("PostComment() error = %v, want ErrModerationUnavailable", err)
	}
	if got := countSocialComments(t, db); got != 0 {
		t.Fatalf("comment count = %d, want 0", got)
	}
}

func TestPostCommentFailOpenWhenGreenNotConfiguredInLocalMode(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)
	svc := newTestSocialService(db, "debug", &fakeTextReviewer{err: aliyun.ErrGreenNotConfigured})

	logs := captureSocialLogs(t, func() {
		got, err := svc.PostComment(context.Background(), PostCommentInput{Body: "comment with green disabled"}, authorID)
		if err != nil {
			t.Fatalf("PostComment() error = %v, want nil (fail-open)", err)
		}
		if got.Body != "comment with green disabled" {
			t.Fatalf("comment body = %q", got.Body)
		}
	})
	if n := countSocialComments(t, db); n != 1 {
		t.Fatalf("comment count = %d, want 1 (fail-open must persist)", n)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "fail_open") {
		t.Fatalf("logs do not record the fail-open policy: %s", joined)
	}
}

func TestModerationFailOpenWithUnconfiguredGreenRecordsLogAndMetric(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)

	metrics := observability.NewMetrics()
	observability.SetDefaultMetrics(metrics)
	defer observability.SetDefaultMetrics(nil)

	cfg := &config.Config{Server: config.ServerConfig{Mode: "debug"}}
	reviewSvc := NewReviewService(db, nil, cfg, nil)
	svc := NewSocialServiceWithRedis(
		repository.NewSocialRepository(db),
		repository.NewContentRepository(db),
		repository.NewUserRepository(db),
		cfg,
		nil,
		reviewSvc,
	)

	logs := captureSocialLogs(t, func() {
		if _, err := svc.PostComment(context.Background(), PostCommentInput{Body: "comment with green unconfigured"}, authorID); err != nil {
			t.Fatalf("PostComment() error = %v, want nil (fail-open)", err)
		}
	})
	if n := countSocialComments(t, db); n != 1 {
		t.Fatalf("comment count = %d, want 1 (fail-open must persist)", n)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "fail_open") {
		t.Fatalf("logs do not record the fail-open policy: %s", joined)
	}
	if got := countExternalMetric(t, metrics, "green", "failure"); got < 1 {
		t.Fatalf("green/failure external metric = %d, want >= 1", got)
	}
}

func TestPostCommentSkipsModerationForBlankText(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	authorID := seedTestSocialUser(t, db)
	reviewer := &fakeTextReviewer{result: "block"}
	svc := newTestSocialService(db, "release", reviewer)

	got, err := svc.PostComment(context.Background(), PostCommentInput{Body: "   "}, authorID)
	if err != nil {
		t.Fatalf("PostComment() error = %v, want nil (blank text skips moderation)", err)
	}
	if got.Body != "   " {
		t.Fatalf("comment body = %q", got.Body)
	}
	if len(reviewer.calls) != 0 {
		t.Fatalf("moderation calls = %v, want none for blank text", reviewer.calls)
	}
}

func TestReviewTextReturnsErrGreenNotConfiguredWhenUnconfigured(t *testing.T) {
	db := setupSocialServiceTestDB(t)
	cfg := &config.Config{Server: config.ServerConfig{Mode: "debug"}}
	svc := NewReviewService(db, nil, cfg, nil)

	if _, err := svc.ReviewText(context.Background(), "any text"); !errors.Is(err, aliyun.ErrGreenNotConfigured) {
		t.Fatalf("ReviewText() error = %v, want ErrGreenNotConfigured", err)
	}

	var nilSvc *ReviewService
	if _, err := nilSvc.ReviewText(context.Background(), "any text"); !errors.Is(err, aliyun.ErrGreenNotConfigured) {
		t.Fatalf("ReviewText() on nil receiver error = %v, want ErrGreenNotConfigured", err)
	}
}

func countExternalMetric(t *testing.T, m *observability.Metrics, dependency, result string) int {
	t.Helper()
	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "omnicraft_external_dependency_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, lp := range metric.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["dependency"] == dependency && labels["result"] == result {
				return int(metric.GetCounter().GetValue())
			}
		}
	}
	return 0
}
