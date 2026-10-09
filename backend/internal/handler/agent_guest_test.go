package handler

// #854 guest agent HTTP seam: full middleware chain (total gate + device
// identity + per-IP cost bucket) over the guest endpoints, with a counting
// fake Provider asserting Provider calls stay zero on every refusal path.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/guestid"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/service"
)

// guestCountingProvider answers every stream round in one delta and counts
// the Provider invocations.
type guestCountingProvider struct {
	calls atomic.Int64
}

func (p *guestCountingProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "ok"}, nil
}

func (p *guestCountingProvider) ChatStream(_ context.Context, _ llm.ChatRequest, handler func(llm.ChatDelta) error) error {
	p.calls.Add(1)
	_ = handler(llm.ChatDelta{Content: "answer", Done: true})
	return nil
}

func (p *guestCountingProvider) GetEmbedding(context.Context, string) ([]float32, error) {
	return nil, nil
}

type guestFixture struct {
	router   *gin.Engine
	provider *guestCountingProvider
	mr       *miniredis.Miniredis
	db       *gorm.DB
	cfg      *config.Config
}

func guestHandlerFixture(t *testing.T, mutate func(*config.Config)) *guestFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_conversations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id BIGINT,
		context_type VARCHAR(50) NOT NULL DEFAULT '',
		context_id BIGINT,
		title VARCHAR(200),
		pinned_at DATETIME,
		is_guest BOOLEAN NOT NULL DEFAULT FALSE,
		guest_device_key VARCHAR(64),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create agent_conversations: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{Agent: config.AgentConfig{
		WebAgentEnabled: true, RateLimitPerMinute: 5, RateLimitPerDay: 50,
		MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, ConversationListLimit: 50,
		ConversationPageSize: 20, ChatContextTokenBudget: 100000, MaxToolCallsPerTurn: 8,
		CitationMaxCount: 5, MaxOutputTokens: 1200,
		Guest: config.AgentGuestConfig{
			MaxTotalTurns: 3, ConversationTTLDays: 7, MaxConcurrentTurns: 3,
			CookieSecret: "unit-test-guest-cookie-secret-0123456789abcdef", CookieMaxAgeHours: 8760,
		},
	}, RateLimit: config.RateLimitConfig{AgentWindowSec: 86400, AgentMinuteWindowSec: 60}}
	cfg.Features.GuestAgentEnabled = true
	if mutate != nil {
		mutate(cfg)
	}

	provider := &guestCountingProvider{}
	agentSvc := service.NewAgentService(provider, nil, nil, nil, db, cfg)
	guestHandler := NewAgentGuestHandler(db, cfg, agentSvc, middleware.NewGuestQuotaReserver(rdb, cfg))

	router := gin.New()
	guest := router.Group("/api/v1/agent/guest", middleware.GuestAgentAccess(cfg, rdb))
	{
		guest.GET("/models", guestHandler.ListModels)
		guest.GET("/quota", guestHandler.Quota)
		guest.GET("/conversations", guestHandler.ListConversations)
		guest.GET("/conversations/:id", guestHandler.GetConversationMessages)
		guest.POST("/chat/stream", middleware.GuestAgentCostLimit(rdb, cfg, time.Now), guestHandler.ChatStream)
	}
	return &guestFixture{router: router, provider: provider, mr: mr, db: db, cfg: cfg}
}

// guestCookieFor bootstraps a device identity via the quota read (the same
// flow the workspace uses) and returns the cookie value.
func guestCookieFor(t *testing.T, f *guestFixture) string {
	t.Helper()
	rec := guestRequest(f.router, http.MethodGet, "/api/v1/agent/guest/quota", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap quota: status %d body %s", rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if strings.Contains(cookie.Name, "omnicraft_guest") {
			return cookie.Value
		}
	}
	t.Fatal("bootstrap did not set the device cookie")
	return ""
}

func guestRequest(router *gin.Engine, method, path, cookie, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "omnicraft_guest", Value: cookie})
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGuestQuotaBootstrapAndRemaining(t *testing.T) {
	f := guestHandlerFixture(t, nil)
	cookie := guestCookieFor(t, f)

	rec := guestRequest(f.router, http.MethodGet, "/api/v1/agent/guest/quota", cookie, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("quota: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Remaining int  `json:"remaining"`
		MaxTurns  int  `json:"max_turns"`
		Exhausted bool `json:"exhausted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Remaining != 3 || out.MaxTurns != 3 || out.Exhausted {
		t.Fatalf("quota = %+v, want 3/3 not exhausted", out)
	}
	if f.provider.calls.Load() != 0 {
		t.Fatalf("quota reads must never reach the provider, calls=%d", f.provider.calls.Load())
	}
}

func TestGuestThreeTurnsThenExhausted(t *testing.T) {
	f := guestHandlerFixture(t, nil)
	cookie := guestCookieFor(t, f)

	for turn := 1; turn <= 3; turn++ {
		rec := guestRequest(f.router, http.MethodPost, "/api/v1/agent/guest/chat/stream", cookie, `{"message":"q"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("turn %d: %d %s", turn, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "event:done") {
			t.Fatalf("turn %d must stream a done event, body: %s", turn, rec.Body.String())
		}
		quota := guestRequest(f.router, http.MethodGet, "/api/v1/agent/guest/quota", cookie, "")
		var out struct {
			Remaining int `json:"remaining"`
		}
		_ = json.Unmarshal(quota.Body.Bytes(), &out)
		if out.Remaining != 3-turn {
			t.Fatalf("after turn %d remaining = %d, want %d", turn, out.Remaining, 3-turn)
		}
	}
	if f.provider.calls.Load() != 3 {
		t.Fatalf("provider calls = %d, want 3", f.provider.calls.Load())
	}

	// The 4th turn — UI or direct HTTP — is refused before any Provider work.
	rec := guestRequest(f.router, http.MethodPost, "/api/v1/agent/guest/chat/stream", cookie, `{"message":"q4"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th turn status = %d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "GUEST_QUOTA_EXHAUSTED") {
		t.Fatalf("4th turn body = %s", rec.Body.String())
	}
	if f.provider.calls.Load() != 3 {
		t.Fatalf("exhausted turn must not reach the provider, calls=%d", f.provider.calls.Load())
	}
}

func TestGuestGenerationWithoutDeviceCookieRefused(t *testing.T) {
	f := guestHandlerFixture(t, nil)
	rec := guestRequest(f.router, http.MethodPost, "/api/v1/agent/guest/chat/stream", "", `{"message":"q"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "GUEST_DEVICE_REQUIRED") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if f.provider.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want 0", f.provider.calls.Load())
	}
}

func TestGuestForeignConversationScopedOwnerOnly(t *testing.T) {
	f := guestHandlerFixture(t, nil)
	cookieA := guestCookieFor(t, f)
	cookieB := guestCookieFor(t, f)

	// First turn creates a conversation for device A.
	rec := guestRequest(f.router, http.MethodPost, "/api/v1/agent/guest/chat/stream", cookieA, `{"message":"q"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("first turn: %d %s", rec.Code, rec.Body.String())
	}
	convID := int64(0)
	for _, block := range strings.Split(rec.Body.String(), "\n\n") {
		if !strings.Contains(block, "event:start") {
			continue
		}
		dataLine := strings.SplitN(block, "data:", 2)[1]
		var start struct {
			ConversationID int64 `json:"conversation_id"`
		}
		if err := json.Unmarshal([]byte(dataLine), &start); err != nil {
			t.Fatalf("parse start: %v", err)
		}
		convID = start.ConversationID
	}
	if convID == 0 {
		t.Fatalf("start event missing, body: %s", rec.Body.String())
	}

	// Foreign device: owner-scoped 404 on read and on continuation.
	for _, attempt := range []struct {
		method, path, body string
	}{
		{http.MethodGet, fmt.Sprintf("/api/v1/agent/guest/conversations/%d", convID), ""},
		{http.MethodPost, "/api/v1/agent/guest/chat/stream", fmt.Sprintf(`{"message":"hi","conversation_id":%d}`, convID)},
	} {
		rec := guestRequest(f.router, attempt.method, attempt.path, cookieB, attempt.body)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s with foreign cookie: %d, want 404 (%s)", attempt.method, attempt.path, rec.Code, rec.Body.String())
		}
	}
	// Own device: 200 on read.
	rec = guestRequest(f.router, http.MethodGet, fmt.Sprintf("/api/v1/agent/guest/conversations/%d", convID), cookieA, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("own conversation read: %d %s", rec.Code, rec.Body.String())
	}
	if f.provider.calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", f.provider.calls.Load())
	}
}

func TestGuestExpiredConversationRefusedReadWrite(t *testing.T) {
	f := guestHandlerFixture(t, nil)
	cookie := guestCookieFor(t, f)

	deviceKey, err := verifyGuestDevice(f.cfg, cookie)
	if err != nil {
		t.Fatalf("resolve device key: %v", err)
	}
	expiredAt := time.Now().Add(-8 * 24 * time.Hour)
	guestConv := &model.AgentGuestConversation{
		IsGuest: true, GuestDeviceKey: deviceKey,
		CreatedAt: expiredAt, UpdatedAt: expiredAt,
	}
	if err := f.db.Create(guestConv).Error; err != nil {
		t.Fatalf("seed expired conversation: %v", err)
	}

	// Read: 410 GONE (distinct from the owner-scoped 404).
	rec := guestRequest(f.router, http.MethodGet, fmt.Sprintf("/api/v1/agent/guest/conversations/%d", guestConv.ID), cookie, "")
	if rec.Code != http.StatusGone {
		t.Fatalf("expired read: %d %s, want 410", rec.Code, rec.Body.String())
	}
	// Continuation: 410 before the reservation — the budget stays intact.
	rec = guestRequest(f.router, http.MethodPost, "/api/v1/agent/guest/chat/stream", cookie,
		fmt.Sprintf(`{"message":"q","conversation_id":%d}`, guestConv.ID))
	if rec.Code != http.StatusGone {
		t.Fatalf("expired continuation: %d %s, want 410", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AGENT_CONVERSATION_EXPIRED") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	quota := guestRequest(f.router, http.MethodGet, "/api/v1/agent/guest/quota", cookie, "")
	if !strings.Contains(quota.Body.String(), `"remaining":3`) {
		t.Fatalf("expired continuation must not consume: %s", quota.Body.String())
	}
	if f.provider.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want 0", f.provider.calls.Load())
	}
}

func TestGuestRedisOutageFailsClosedOnGeneration(t *testing.T) {
	f := guestHandlerFixture(t, nil)
	cookie := guestCookieFor(t, f)
	f.mr.Close()

	rec := guestRequest(f.router, http.MethodPost, "/api/v1/agent/guest/chat/stream", cookie, `{"message":"q"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("outage status = %d (%s), want 503", rec.Code, rec.Body.String())
	}
	if f.provider.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want 0", f.provider.calls.Load())
	}
}

func TestGuestGateClosedBlocksEveryEndpoint(t *testing.T) {
	f := guestHandlerFixture(t, func(c *config.Config) {
		c.Features.GuestAgentEnabled = false
	})
	for _, attempt := range []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/agent/guest/quota", ""},
		{http.MethodGet, "/api/v1/agent/guest/conversations", ""},
		{http.MethodPost, "/api/v1/agent/guest/chat/stream", `{"message":"q"}`},
	} {
		rec := guestRequest(f.router, attempt.method, attempt.path, "", attempt.body)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s: %d, want 403 GUEST_AGENT_DISABLED", attempt.method, attempt.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "GUEST_AGENT_DISABLED") {
			t.Fatalf("body = %s", rec.Body.String())
		}
	}
	if f.provider.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want 0", f.provider.calls.Load())
	}
}

func TestGuestWebAgentGateClosedAlsoBlocks(t *testing.T) {
	f := guestHandlerFixture(t, func(c *config.Config) {
		c.Agent.WebAgentEnabled = false
	})
	rec := guestRequest(f.router, http.MethodPost, "/api/v1/agent/guest/chat/stream", "", `{"message":"q"}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "GUEST_AGENT_DISABLED") {
		t.Fatalf("web agent off: %d %s", rec.Code, rec.Body.String())
	}
	if f.provider.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, want 0", f.provider.calls.Load())
	}
}

// verifyGuestDevice resolves the device key of a minted cookie (test helper
// mirroring the middleware's verification).
func verifyGuestDevice(cfg *config.Config, cookieValue string) (string, error) {
	return guestid.Verify(cfg.Agent.Guest.CookieSecret, cookieValue)
}
