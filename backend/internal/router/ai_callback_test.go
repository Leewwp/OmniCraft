package router

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/mail"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

const (
	testCallbackUID  = "1234567890123456"
	testCallbackSeed = "test_seed_abc123"
)

// TestAICallbackValidChecksumAppliesResult covers the happy path of the
// Aliyun scan-result callback contract: a form request signed with the
// correct SHA256(uid + seed + content) checksum is accepted (200) and the
// target content transitions according to the strictest suggestion.
func TestAICallbackValidChecksumAppliesResult(t *testing.T) {
	cases := []struct {
		name        string
		suggestions []string
		startStatus string
		wantStatus  string
	}{
		{name: "pass publishes", suggestions: []string{"pass"}, startStatus: "under_review", wantStatus: "published"},
		{name: "block bans", suggestions: []string{"block"}, startStatus: "under_review", wantStatus: "banned"},
		{name: "review flags", suggestions: []string{"review"}, startStatus: "pending", wantStatus: "under_review"},
		{name: "violation normalizes to block", suggestions: []string{"violation"}, startStatus: "under_review", wantStatus: "banned"},
		{name: "strictest suggestion wins", suggestions: []string{"pass", "block"}, startStatus: "under_review", wantStatus: "banned"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, _, db, cleanup := buildAICallbackRouter(t, queue.NewNoopProducer(), tc.startStatus)
			defer cleanup()

			content := callbackContentJSON("content:1", "task-"+strings.ReplaceAll(tc.name, " ", "-"), tc.suggestions...)
			rec := postCallback(t, router, checksumOf(testCallbackUID, testCallbackSeed, content), content)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			if got := contentStatus(t, db); got != tc.wantStatus {
				t.Fatalf("content status = %q, want %q", got, tc.wantStatus)
			}
			if n := countAIReviewRecords(t, db); n != 1 {
				t.Fatalf("ai_review_records = %d, want 1", n)
			}
		})
	}
}

// TestAICallbackRejectsForgedOrTamperedRequests covers the authentication
// failure paths: any checksum mismatch (wrong checksum, wrong seed, wrong
// uid, tampered content) and missing green configuration must yield 403 with
// no side effects.
func TestAICallbackRejectsForgedOrTamperedRequests(t *testing.T) {
	start := "under_review"
	tests := []struct {
		name string
		sign func(uid, seed, content string) (checksum, body string)
	}{
		{name: "wrong checksum", sign: func(uid, seed, content string) (string, string) {
			return strings.Repeat("0", 64), content
		}},
		{name: "wrong seed", sign: func(uid, seed, content string) (string, string) {
			return checksumOf(uid, "attacker_seed", content), content
		}},
		{name: "wrong uid", sign: func(uid, seed, content string) (string, string) {
			return checksumOf("999", seed, content), content
		}},
		{name: "tampered content", sign: func(uid, seed, content string) (string, string) {
			return checksumOf(uid, seed, content), strings.ReplaceAll(content, `"suggestion":"block"`, `"suggestion":"pass"`)
		}},
		{name: "missing checksum", sign: func(uid, seed, content string) (string, string) {
			return "", content
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router, _, db, cleanup := buildAICallbackRouter(t, queue.NewNoopProducer(), start)
			defer cleanup()

			content := callbackContentJSON("content:1", "task-forged", "block")
			checksum, body := tc.sign(testCallbackUID, testCallbackSeed, content)
			rec := postCallback(t, router, checksum, body)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "FORBIDDEN") {
				t.Fatalf("body = %s, want FORBIDDEN", rec.Body.String())
			}
			assertNoCallbackSideEffects(t, db, start)
		})
	}

	t.Run("missing green config fails closed", func(t *testing.T) {
		router, cfg, db, cleanup := buildAICallbackRouter(t, queue.NewNoopProducer(), start)
		defer cleanup()
		cfg.Green.Seed = ""
		cfg.Green.UID = ""

		content := callbackContentJSON("content:1", "task-noconfig", "block")
		rec := postCallback(t, router, checksumOf("", "", content), content)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
		}
		assertNoCallbackSideEffects(t, db, start)
	})
}

// TestAICallbackDuplicateTaskIDIsIdempotent covers the Aliyun retry
// semantics: a duplicate callback for the same taskId returns 200 and does
// not record a second ai_review_records row (the service-level idempotency
// short-circuit from #105).
func TestAICallbackDuplicateTaskIDIsIdempotent(t *testing.T) {
	router, _, db, cleanup := buildAICallbackRouter(t, queue.NewNoopProducer(), "under_review")
	defer cleanup()

	content := callbackContentJSON("content:1", "task-duplicate", "block")
	for i := 0; i < 2; i++ {
		rec := postCallback(t, router, checksumOf(testCallbackUID, testCallbackSeed, content), content)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want 200; body = %s", i+1, rec.Code, rec.Body.String())
		}
	}
	if n := countAIReviewRecords(t, db); n != 1 {
		t.Fatalf("ai_review_records = %d, want 1 (duplicate callback must not re-record)", n)
	}
}

// TestAICallbackRejectsUnsupportedOrMalformedDataID covers the dataId
// resolver: only the supported content:<id> namespace is accepted; unknown
// types (including the future ip:<id> namespace), malformed separators and
// non-numeric ids are rejected with an explicit 400 and no side effects.
func TestAICallbackRejectsUnsupportedOrMalformedDataID(t *testing.T) {
	start := "under_review"
	for _, dataID := range []string{"bogus:1", "ip:1", "content:", "content:abc", "no-separator", "content:0", "content:-3"} {
		t.Run(dataID, func(t *testing.T) {
			router, _, db, cleanup := buildAICallbackRouter(t, queue.NewNoopProducer(), start)
			defer cleanup()

			content := callbackContentJSON(dataID, "task-bad-dataid", "block")
			rec := postCallback(t, router, checksumOf(testCallbackUID, testCallbackSeed, content), content)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			assertNoCallbackSideEffects(t, db, start)
		})
	}
}

// TestAICallbackWithQueueProducerPublishesWithoutSideEffects covers the
// queue branch: with a real producer the callback is published to the
// content.review topic with the worker-compatible payload (including
// provider_task_id for #108 end-to-end verification) and is NOT applied
// synchronously (the worker is not running in this test).
func TestAICallbackWithQueueProducerPublishesWithoutSideEffects(t *testing.T) {
	producer := &recordingProducer{}
	router, _, db, cleanup := buildAICallbackRouter(t, producer, "under_review")
	defer cleanup()

	content := callbackContentJSON("content:1", "task-queued", "block")
	rec := postCallback(t, router, checksumOf(testCallbackUID, testCallbackSeed, content), content)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	// The publish runs in a background goroutine (recovery.GoSafe) after the
	// 200 ACK, so wait for it to land before asserting the payload.
	deadline := time.Now().Add(2 * time.Second)
	for {
		producer.mu.Lock()
		got := producer.topic
		producer.mu.Unlock()
		if got != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("callback payload was not published within 2s")
		}
		time.Sleep(10 * time.Millisecond)
	}

	producer.mu.Lock()
	defer producer.mu.Unlock()
	if producer.topic != "content.review" {
		t.Fatalf("published topic = %q, want content.review", producer.topic)
	}
	var payload struct {
		Action         string                 `json:"action"`
		TargetType     string                 `json:"target_type"`
		TargetID       int64                  `json:"target_id"`
		Result         string                 `json:"result"`
		RawResponse    map[string]interface{} `json:"raw_response"`
		ProviderTaskID string                 `json:"provider_task_id"`
	}
	if err := json.Unmarshal(producer.payload, &payload); err != nil {
		t.Fatalf("unmarshal published payload: %v", err)
	}
	if payload.Action != "process_ai_callback" ||
		payload.TargetType != "content" ||
		payload.TargetID != 1 ||
		payload.Result != "block" ||
		payload.ProviderTaskID != "task-queued" {
		t.Fatalf("published payload = %+v, want process_ai_callback/content/1/block/task-queued", payload)
	}
	if got := contentStatus(t, db); got != "under_review" {
		t.Fatalf("content status = %q, want unchanged under_review (queued, not applied)", got)
	}
}

// TestAICallbackBlockThenPassKeepsBanned covers the A1 terminal-state guard
// at the HTTP boundary (the service-level equivalent ships with #105): once a
// block result bans a content item, a later pass result for a different task
// is recorded but must never resurrect the content.
func TestAICallbackBlockThenPassKeepsBanned(t *testing.T) {
	router, _, db, cleanup := buildAICallbackRouter(t, queue.NewNoopProducer(), "under_review")
	defer cleanup()

	blockContent := callbackContentJSON("content:1", "task-block-then-pass", "block")
	rec := postCallback(t, router, checksumOf(testCallbackUID, testCallbackSeed, blockContent), blockContent)
	if rec.Code != http.StatusOK {
		t.Fatalf("block callback status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if got := contentStatus(t, db); got != "banned" {
		t.Fatalf("content status after block = %q, want banned", got)
	}

	passContent := callbackContentJSON("content:1", "task-pass-after-block", "pass")
	rec = postCallback(t, router, checksumOf(testCallbackUID, testCallbackSeed, passContent), passContent)
	if rec.Code != http.StatusOK {
		t.Fatalf("pass callback status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if got := contentStatus(t, db); got != "banned" {
		t.Fatalf("content status after pass = %q, want banned (banned is the AI channel terminal state)", got)
	}
	if n := countAIReviewRecords(t, db); n != 2 {
		t.Fatalf("ai_review_records = %d, want 2 (each distinct taskId is recorded; the pass must not flip the status)", n)
	}
}

// buildAICallbackRouter wires a router with the real ReviewService, sqlite
// storage and the given queue producer, plus one author user (id 1) and one
// content item (id 1) in the given start status.
func buildAICallbackRouter(t *testing.T, producer queue.Producer, contentStartStatus string) (*gin.Engine, *config.Config, *gorm.DB, func()) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		mr.Close()
		t.Fatalf("sqlite: %v", err)
	}
	for _, m := range []interface{}{&model.User{}, &model.ContentItem{}, &model.AIReviewRecord{}, &model.ReputationLog{}, &model.JudgeCase{}} {
		if err := db.AutoMigrate(m); err != nil {
			_ = rdb.Close()
			mr.Close()
			t.Fatalf("migrate: %v", err)
		}
	}
	// Migration 068's unique index is what the record insert's ON CONFLICT
	// clause binds to; sqlite AutoMigrate does not declare it, so create it
	// here to mirror the production schema.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_review_records_provider_task
		ON ai_review_records (provider, provider_task_id)`).Error; err != nil {
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("create provider task unique index: %v", err)
	}

	verifiedAt := time.Now()
	user := model.User{
		ID:              1,
		Email:           "ai-callback-author@example.com",
		PasswordHash:    "hash",
		Username:        "ai-callback-author",
		Role:            "user",
		Reputation:      10,
		EmailVerifiedAt: &verifiedAt,
	}
	if err := db.Create(&user).Error; err != nil {
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("create author: %v", err)
	}
	content := model.ContentItem{
		ID:          1,
		Title:       "ai-callback-content",
		AuthorID:    1,
		Zone:        "original",
		ContentType: "craft",
		Status:      contentStartStatus,
	}
	if err := db.Create(&content).Error; err != nil {
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("create content: %v", err)
	}

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:          "route-security-secret",
			AccessTokenTTL:  7200,
			RefreshTokenTTL: 604800,
		},
		Server: config.ServerConfig{
			Mode: "debug",
		},
		RateLimit: config.RateLimitConfig{
			Enabled:              true,
			CredentialPerMinute:  1,
			NormalWindowSec:      60,
			AgentWindowSec:       60,
			AgentMinuteWindowSec: 60,
		},
		Agent: config.AgentConfig{
			WebAgentEnabled:     true,
			RateLimitPerDay:     1,
			RateLimitPerMinute:  5,
			MaxUserMessageChars: 4000,
			ChatMaxContextMsgs:  10,
			MaxToolCallsPerTurn: 8,
			MaxOutputTokens:     1200,
			CitationMaxCount:    5,
		},
		Reputation: config.ReputationConfig{
			MinScoreForInteraction:      3,
			RepeatViolationWindowDays:   7,
			RepeatViolationThreshold:    1,
			RepeatViolationExtraPenalty: -1,
		},
		Cache: config.CacheConfig{
			UserStatusTTL:    300,
			PublishFreezeTTL: 604800,
		},
		Judge: config.JudgeConfig{
			MinVotesRequired: 3,
		},
		Green: config.GreenConfig{
			Seed: testCallbackSeed,
			UID:  testCallbackUID,
		},
	}

	userRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(userRepo, rdb, cfg)
	verificationSvc := service.NewVerificationService(userRepo, rdb, mail.NewLoggerSender(), cfg)
	ctr := &container.ServiceContainer{
		DB:                  db,
		RDB:                 rdb,
		Cfg:                 cfg,
		UserRepo:            userRepo,
		AuthService:         authSvc,
		VerificationService: verificationSvc,
		QueueProducer:       producer,
		AgentService: service.NewAgentService(
			&routeFakeAgentProvider{},
			nil,
			repository.NewContentRepository(db),
			nil,
			db,
			cfg,
		),
	}

	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterRoutes(v1, cfg, ctr)

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}
	return router, cfg, db, cleanup
}

// callbackContentJSON builds the content payload of an Aliyun scan-result
// callback for the given dataId, taskId and suggestions.
func callbackContentJSON(dataID, taskID string, suggestions ...string) string {
	results := make([]map[string]string, 0, len(suggestions))
	for _, s := range suggestions {
		results = append(results, map[string]string{"scene": "porn", "label": "callback-test", "suggestion": s})
	}
	body, _ := json.Marshal(map[string]interface{}{
		"dataId":  dataID,
		"taskId":  taskID,
		"results": results,
		"code":    200,
		"message": "OK",
	})
	return string(body)
}

// checksumOf computes SHA256(uid + seed + content) as a lowercase hex
// string, exactly as the Aliyun callback signing rule defines it.
func checksumOf(uid, seed, content string) string {
	sum := sha256.Sum256([]byte(uid + seed + content))
	return hex.EncodeToString(sum[:])
}

func postCallback(t *testing.T, router *gin.Engine, checksum, content string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("checksum", checksum)
	form.Set("content", content)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/ai-callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func contentStatus(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var c model.ContentItem
	if err := db.First(&c, 1).Error; err != nil {
		t.Fatalf("load content: %v", err)
	}
	return c.Status
}

func countAIReviewRecords(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.AIReviewRecord{}).Count(&n).Error; err != nil {
		t.Fatalf("count ai_review_records: %v", err)
	}
	return n
}

func assertNoCallbackSideEffects(t *testing.T, db *gorm.DB, wantStatus string) {
	t.Helper()
	if got := contentStatus(t, db); got != wantStatus {
		t.Fatalf("content status = %q, want %q (no side effects)", got, wantStatus)
	}
	if n := countAIReviewRecords(t, db); n != 0 {
		t.Fatalf("ai_review_records = %d, want 0 (no side effects)", n)
	}
}

// recordingProducer captures the last published topic and payload so the
// queue branch can be asserted without a running worker.
type recordingProducer struct {
	mu      sync.Mutex
	topic   string
	payload []byte
}

func (p *recordingProducer) Publish(_ context.Context, topic string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.topic = topic
	p.payload = append([]byte(nil), payload...)
	return nil
}
