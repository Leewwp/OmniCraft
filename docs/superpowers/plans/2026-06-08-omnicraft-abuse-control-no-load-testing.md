# OmniCraft Abuse Control Hardening Implementation Plan

> ✅ **完成状态**: 本计划全部步骤已于 2026-06-09 执行完毕。执行记录见 `docs/superpowers/progress/2026-06-09-security-hardening-execution.md`。以下步骤仅保留原始计划结构作历史参考。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce robot, crawler, brute-force, and high-cost request abuse without running load or stress tests in this task.

**Architecture:** Add deterministic application-layer controls first: bounded request sizes, capped query parameters, credential limits by IP plus account key, stricter behavior for expensive endpoints, and proxy trust configuration for reliable client IP extraction. Keep load testing out of scope; this plan prepares the code so a later k6/vegeta task has meaningful controls to verify.

**Tech Stack:** Go/Gin middleware, Redis rate limiting, backend config, Next.js error handling, nginx deployment config, Go unit tests.

---

## Scope And Mode

This plan explicitly excludes pressure/load testing. Do not run k6, vegeta, Locust, or destructive traffic tests here.

Do not update `task.json` or Beta roadmap checkboxes unless this plan is separately converted into a tracked task.

## Current Repo Facts

- `backend/internal/middleware/ratelimit.go` currently provides global IP limits, upload limits, credential limits, and comment edit limits. Credential limits are fixed at 5/minute and IP-only.
- `backend/internal/middleware/body_limit.go`, `backend/internal/middleware/ratelimit_test.go`, and `backend/internal/handler/search_abuse_test.go` do not exist yet.
- `backend/internal/handler/search.go` currently parses `limit`, `page`, and `q` directly and forwards them to `SearchService` without config-based caps.
- `backend/internal/handler/routes.go` registers search endpoints without route-specific Redis limits.
- `backend/cmd/server/main.go` mounts middleware in this order: request ID, logger, CORS, CSRF, security headers, global rate limit, panic recovery. Body limits and trusted proxy configuration are not wired yet.
- `frontend/lib/error-handler.ts` handles `401`, `403`, and `5xx`, but does not yet normalize `429` rate-limit errors.

## File Structure

- Modify: `backend/config/config.go`
  - Adds the exact `RateLimitConfig` and `SecurityConfig` fields listed in this plan.
- Modify: `backend/config.yaml`
  - Adds conservative default values.
- Modify: `backend/internal/middleware/ratelimit.go`
  - Owns IP, credential, upload, comment-edit, and reusable Redis limit helpers.
- Create: `backend/internal/middleware/body_limit.go`
  - Owns request body size limits for JSON/form endpoints.
- Create: `backend/internal/middleware/body_limit_test.go`
  - Owns request body limit middleware tests.
- Modify: `backend/cmd/server/main.go`
  - Wires trusted proxy and body-limit middleware before routes.
- Modify: `backend/internal/handler/search.go`
  - Caps search query length and `limit`.
- Modify: `backend/internal/handler/tag.go`
  - Caps tag search query length if not already capped.
- Modify: `backend/internal/handler/routes.go`
  - Applies route-specific limiters to expensive public endpoints.
- Create: `backend/internal/middleware/ratelimit_test.go`
  - Owns limiter behavior tests.
- Create: `backend/internal/handler/search_abuse_test.go`
  - Owns query cap and limit cap tests.
- Modify: `frontend/lib/error-handler.ts`
  - Normalizes 429 user-facing copy without exposing backend internals.
- Modify: `docs/deploy/nginx.omnicraft.single-server.conf`
  - Keeps proxy limits aligned with app-layer limits.

## Success Criteria

- Search and suggestion endpoints reject or clamp unreasonably large `limit`, `page_size`, and query inputs.
- Credential attempts are limited by both IP and normalized account key, not only IP.
- Release mode has deterministic behavior when Redis is unavailable for credential, upload, and agent limits.
- Gin trusts only configured proxies for `ClientIP()`.
- JSON request body size is capped before handlers parse it.
- `go test ./internal/middleware ./internal/handler -run "RateLimit|Search|Abuse|BodyLimit" -count=1` passes.
- `go test ./...`, `go build ./...`, and `go vet ./...` pass.

## Task 1: Add Abuse-Control Config Tests

**Files:**
- Modify: `backend/config/config_test.go`

- [ ] **Step 1: Add test coverage for default config fields**

Add a test near existing config tests:

```go
func TestDefaultConfigHasAbuseControlLimits(t *testing.T) {
	raw, err := os.ReadFile("../config.yaml")
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		"normal_per_minute",
		"upload_per_hour",
		"credential_per_minute",
		"search_per_minute",
		"max_json_body_bytes",
		"max_query_chars",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config.yaml must declare %s", want)
		}
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```powershell
cd backend
go test ./config -run AbuseControl -count=1
```

Expected: FAIL until new fields are added.

## Task 2: Add Config Fields And Defaults

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`

- [ ] **Step 1: Extend `RateLimitConfig`**

Add fields:

```go
type RateLimitConfig struct {
	Enabled             bool `mapstructure:"enabled"`
	NormalPerMinute     int  `mapstructure:"normal_per_minute"`
	UploadPerHour       int  `mapstructure:"upload_per_hour"`
	NormalWindowSec     int  `mapstructure:"normal_window_sec"`
	UploadWindowSec     int  `mapstructure:"upload_window_sec"`
	AgentWindowSec      int  `mapstructure:"agent_window_sec"`
	CredentialPerMinute int  `mapstructure:"credential_per_minute"`
	SearchPerMinute     int  `mapstructure:"search_per_minute"`
	MaxJSONBodyBytes    int64 `mapstructure:"max_json_body_bytes"`
	MaxQueryChars       int  `mapstructure:"max_query_chars"`
	MaxSearchLimit      int  `mapstructure:"max_search_limit"`
}
```

- [ ] **Step 2: Add config defaults**

In `backend/config.yaml`, update `rate_limit`:

```yaml
rate_limit:
  enabled: true
  normal_per_minute: 100
  upload_per_hour: 200
  credential_per_minute: 5
  search_per_minute: 30
  max_json_body_bytes: 1048576
  max_query_chars: 120
  max_search_limit: 50
```

Keep existing fields if present. Do not remove `normal_window_sec`, `upload_window_sec`, or `agent_window_sec` if another branch already added them.

- [ ] **Step 3: Persist new fields in admin override**

Inspect `Config.SaveOverride` and confirm it already calls:

```go
v.Set("rate_limit", c.RateLimit)
```

If that exact call still exists, make no additional persistence changes. Do not expose the new fields in the admin UI in this plan.

- [ ] **Step 4: Re-run config test**

Run:

```powershell
cd backend
go test ./config -run AbuseControl -count=1
```

Expected: PASS.

## Task 3: Add Request Body Limit Middleware

**Files:**
- Create: `backend/internal/middleware/body_limit.go`
- Create: `backend/internal/middleware/body_limit_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write failing middleware tests**

Create `backend/internal/middleware/body_limit_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBodyLimitRejectsOversizedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(8))
	r.POST("/x", func(c *gin.Context) {
		var body map[string]string
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_BODY"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"payload":"too-long"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 body=%s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run failing test**

Run:

```powershell
cd backend
go test ./internal/middleware -run BodyLimit -count=1
```

Expected: FAIL because `BodyLimit` does not exist.

- [ ] **Step 3: Implement middleware**

Create `backend/internal/middleware/body_limit.go`:

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"code":    "REQUEST_BODY_TOO_LARGE",
				"message": "request body is too large",
			})
			c.Abort()
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
```

- [ ] **Step 4: Wire middleware**

In `backend/cmd/server/main.go`, mount it before `CSRF` and route registration:

```go
bodyLimit := cfg.RateLimit.MaxJSONBodyBytes
if bodyLimit <= 0 {
	bodyLimit = 1 << 20
}
r.Use(middleware.BodyLimit(bodyLimit))
```

Suggested order:

```go
r.Use(middleware.RequestID())
r.Use(middleware.Logger())
r.Use(middleware.CORS(cfg))
r.Use(middleware.BodyLimit(bodyLimit))
r.Use(middleware.CSRF(cfg))
r.Use(middleware.SecurityHeaders())
r.Use(middleware.RateLimit(rdb, &cfg.RateLimit))
r.Use(middleware.PanicRecovery())
```

- [ ] **Step 5: Run tests**

Run:

```powershell
cd backend
go test ./internal/middleware -run BodyLimit -count=1
```

Expected: PASS.

## Task 4: Harden Credential Rate Limiting

**Files:**
- Modify: `backend/internal/middleware/ratelimit.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/middleware/ratelimit_test.go`

- [ ] **Step 1: Write failing tests for account-key limiter**

Create `backend/internal/middleware/ratelimit_test.go` if it does not exist. Add this complete test for account-key limiting:

```go
func TestCredentialRateLimitUsesAccountKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cfg := &config.RateLimitConfig{
		Enabled:             true,
		CredentialPerMinute: 2,
		NormalWindowSec:     60,
	}
	r := gin.New()
	r.POST("/login", CredentialRateLimit(rdb, cfg), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"User@Example.com","password":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:12345", i+10)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if i < 2 && w.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 body=%s", i+1, w.Code, w.Body.String())
		}
		if i == 2 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("third request status = %d, want 429 body=%s", w.Code, w.Body.String())
		}
		if i == 2 && !strings.Contains(w.Body.String(), "CREDENTIAL_RATE_LIMIT_EXCEEDED") {
			t.Fatalf("third response body = %s, want CREDENTIAL_RATE_LIMIT_EXCEEDED", w.Body.String())
		}
	}
}
```

Add imports used by this test: `fmt`, `net/http`, `net/http/httptest`, `strings`, `testing`, `github.com/alicebob/miniredis/v2`, `github.com/gin-gonic/gin`, `github.com/redis/go-redis/v9`, and `omnicraft/backend/config`.

- [ ] **Step 2: Run failing test**

Run:

```powershell
cd backend
go test ./internal/middleware -run CredentialRateLimit -count=1
```

Expected: FAIL because the current limiter is IP-only.

- [ ] **Step 3: Add a credential key extractor**

In `ratelimit.go`, add:

```go
func credentialAccountKey(c *gin.Context) string {
	var body struct {
		Email string `json:"email"`
	}
	if c.Request.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	normalized := strings.ToLower(strings.TrimSpace(body.Email))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
```

Add imports: `bytes`, `crypto/sha256`, `encoding/hex`, `encoding/json`, `io`, `strings`.

- [ ] **Step 4: Update `CredentialRateLimit`**

Use both keys:

```go
limit := cfg.CredentialPerMinute
if limit <= 0 {
	limit = 5
}
keys := []string{
	fmt.Sprintf("ratelimit:credential:ip:%s:%d", ip, window),
}
if accountKey := credentialAccountKey(c); accountKey != "" {
	keys = append(keys, fmt.Sprintf("ratelimit:credential:acct:%s:%d", accountKey, window))
}
for _, key := range keys {
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "RATE_LIMIT_UNAVAILABLE", "message": "rate limit temporarily unavailable"})
		c.Abort()
		return
	}
	if count == 1 {
		rdb.Expire(ctx, key, credWindowTTL)
	}
	if int(count) > limit {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": "CREDENTIAL_RATE_LIMIT_EXCEEDED", "message": "too many credential attempts, please try again later"})
		c.Abort()
		return
	}
}
```

Credential endpoints must fail closed when Redis is unavailable. General public read limits must keep their existing fail-open behavior unless a route explicitly passes `failClosed=true`.

- [ ] **Step 5: Run tests**

Run:

```powershell
cd backend
go test ./internal/middleware -run CredentialRateLimit -count=1
```

Expected: PASS.

## Task 5: Cap Search And Suggestion Inputs

**Files:**
- Modify: `backend/internal/handler/search.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/handler/tag.go`
- Create: `backend/internal/handler/search_abuse_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `backend/internal/handler/search_abuse_test.go` with package `handler`. Use pure handler/helper tests, not database-backed tests:

- `rejectLongQuery` writes status `400` and JSON code `QUERY_TOO_LONG` when query length is 121 runes and max is 120.
- `clampLimit("100000", 20, 50)` returns `50`.
- `clampLimit("0", 20, 50)` returns `20`.
- `clampLimit("not-a-number", 20, 50)` returns `20`.

Do not create a fake `SearchService` or sqlite schema for these cap tests; the cap helpers are sufficient and keep the test isolated.

- [ ] **Step 2: Run failing tests**

Run:

```powershell
cd backend
go test ./internal/handler -run "Search.*Abuse|Search.*Limit" -count=1
```

Expected: FAIL until caps are implemented.

- [ ] **Step 3: Add normalization helpers**

`SearchHandler` currently only stores `searchSvc`, so first add config access:

```go
type SearchHandler struct {
	searchSvc *service.SearchService
	cfg       *config.Config
}

func NewSearchHandler(searchSvc *service.SearchService, cfg *config.Config) *SearchHandler {
	return &SearchHandler{searchSvc: searchSvc, cfg: cfg}
}
```

Update `routes.go` to call:

```go
searchHandler := NewSearchHandler(service.NewSearchService(repository.NewSearchRepository(db), rdb), cfg)
```

Then in `search.go`, add:

```go
const defaultMaxQueryChars = 120
const defaultMaxSearchLimit = 50

func clampLimit(raw string, def, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if max <= 0 {
		max = defaultMaxSearchLimit
	}
	if n > max {
		return max
	}
	return n
}

func rejectLongQuery(c *gin.Context, q string, max int) bool {
	if max <= 0 {
		max = defaultMaxQueryChars
	}
	if len([]rune(q)) > max {
		c.JSON(http.StatusBadRequest, gin.H{"code": "QUERY_TOO_LONG", "message": "search query is too long"})
		return true
	}
	return false
}
```

- [ ] **Step 4: Use helpers in handlers**

Apply to:

```go
Suggestions
Trending
SearchContents
SearchUsers
```

Use `cfg.RateLimit.MaxQueryChars` and `cfg.RateLimit.MaxSearchLimit`.

- [ ] **Step 5: Cap tag search input**

Inspect `backend/internal/handler/tag.go`:

```powershell
rg -n "SearchTags|q :=|limit" backend/internal/handler/tag.go
```

`SearchTags` currently accepts `q` and has no `limit` parameter. Add a config field to `TagHandler`, pass it from routes, and cap query length only.

Update `TagHandler`:

```go
type TagHandler struct {
	tagSvc        *service.TagService
	maxQueryChars int
}
```

Update the constructor signature:

```go
func NewTagHandler(db *gorm.DB, rdb *redis.Client, cacheCfg *config.CacheConfig, maxQueryChars int) *TagHandler
```

Update `routes.go`:

```go
tagHandler := NewTagHandler(db, rdb, &cfg.Cache, cfg.RateLimit.MaxQueryChars)
```

Then in `SearchTags`, after the empty query check:

```go
if rejectLongQuery(c, q, h.maxQueryChars) {
	return
}
```

Keep the existing response shape unchanged. Do not add a new `limit` query parameter to tag search in this plan.

- [ ] **Step 6: Run tests**

Run:

```powershell
cd backend
go test ./internal/handler -run "Search.*Abuse|Search.*Limit" -count=1
```

Expected: PASS.

## Task 6: Apply Search-Specific Rate Limits

**Files:**
- Modify: `backend/internal/middleware/ratelimit.go`
- Modify: `backend/internal/handler/routes.go`
- Modify: `backend/internal/middleware/ratelimit_test.go`

- [ ] **Step 1: Add reusable named limiter**

Implement:

```go
func RedisFixedWindowLimit(rdb *redis.Client, keyPrefix string, limit int, window time.Duration, failClosed bool) gin.HandlerFunc
```

Behavior:

- If `rdb == nil` and `failClosed == true`, return 503.
- If `rdb == nil` and `failClosed == false`, continue.
- Increment `keyPrefix:<clientIP>:<windowID>`.
- Use `limit <= 0` as "limiter disabled" and continue.
- Return 429 with `RATE_LIMIT_EXCEEDED` when over limit.

- [ ] **Step 2: Wire search limiters**

In `routes.go`, apply a search limiter to:

```go
v1.GET("/search/suggestions", optAuth, searchLimiter, searchHandler.Suggestions)
v1.GET("/search/trending", optAuth, searchLimiter, searchHandler.Trending)
v1.GET("/contents/search", optAuth, searchLimiter, searchHandler.SearchContents)
users.GET("/search", optAuth, searchLimiter, searchHandler.SearchUsers)
```

Use:

```go
searchLimiter := middleware.RedisFixedWindowLimit(
	rdb,
	"ratelimit:search",
	cfg.RateLimit.SearchPerMinute,
	time.Minute,
	false,
)
```

Add `time` to `routes.go` imports.

- [ ] **Step 3: Run route tests**

Run:

```powershell
cd backend
go test ./internal/handler -run Routes -count=1
go test ./internal/middleware -run RedisFixedWindowLimit -count=1
```

Expected: PASS.

## Task 7: Configure Trusted Proxies For Reliable Client IPs

**Files:**
- Modify: `backend/config/config.go`
- Modify: `backend/config.yaml`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/main_test.go`

- [ ] **Step 1: Add config field**

Add to `SecurityConfig`:

```go
TrustedProxies []string `mapstructure:"trusted_proxies"`
```

In `backend/config.yaml`:

```yaml
security:
  allowed_origins:
    - "http://localhost:3000"
  trusted_proxies:
    - "127.0.0.1"
```

For the development default in `backend/config.yaml`, use only:

```yaml
trusted_proxies:
  - "127.0.0.1"
```

Do not guess a production Docker subnet in code. Add production proxy values only in `docs/deploy/nginx.omnicraft.single-server.conf` comments or deployment docs if the operator knows the subnet.

- [ ] **Step 2: Wire Gin trusted proxies**

In `main.go`, after `r := gin.New()`:

```go
if len(cfg.Security.TrustedProxies) > 0 {
	if err := r.SetTrustedProxies(cfg.Security.TrustedProxies); err != nil {
		slog.Error("invalid trusted proxies", "error", err)
		os.Exit(1)
	}
} else if cfg.Server.Mode == "release" {
	_ = r.SetTrustedProxies(nil)
}
```

- [ ] **Step 3: Add source-level order test**

In `main_test.go`, assert `SetTrustedProxies` appears before `RegisterRoutes`.

- [ ] **Step 4: Run tests**

Run:

```powershell
cd backend
go test ./cmd/server ./config -count=1
```

Expected: PASS.

## Task 8: Frontend 429 Copy Cleanup

**Files:**
- Modify: `frontend/lib/error-handler.ts`

- [ ] **Step 1: Inspect current error handler**

Run:

```powershell
rg -n "429|RATE_LIMIT|Server busy|ApiRequestError" frontend/lib frontend/components frontend/app
```

- [ ] **Step 2: Normalize rate-limit copy**

`frontend/lib/error-handler.ts` is the central handler. Add this branch after the existing `403` branch and before the `5xx` branch:

```ts
if (error.status === 429) {
  options?.toast?.("warning", "Too many requests. Please try again later.");
  return;
}
```

Do not edit `frontend/messages/zh.json` or `frontend/messages/en.json` in this backend-focused plan. This central utility already contains English strings; keep style consistent with the existing file.

- [ ] **Step 3: Run frontend checks**

Run:

```powershell
cd frontend
npm run lint
npm run test
```

Expected: PASS.

## Task 9: Align Nginx Template Without Running Load Tests

**Files:**
- Modify: `docs/deploy/nginx.omnicraft.single-server.conf`

- [ ] **Step 1: Inspect current single-server nginx template**

Run:

```powershell
Get-Content docs/deploy/nginx.omnicraft.single-server.conf
```

- [ ] **Step 2: Ensure route-level zones exist**

Add these directives near the top of `docs/deploy/nginx.omnicraft.single-server.conf`, before any `server { ... }` block:

```nginx
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=30r/s;
limit_req_zone $binary_remote_addr zone=auth_limit:10m rate=5r/s;
limit_req_zone $binary_remote_addr zone=search_limit:10m rate=10r/s;
limit_conn_zone $binary_remote_addr zone=conn_limit:10m;
```

In the `api.leeppp.online` server block, add explicit `location` blocks before the catch-all `location /`:

```nginx
location ~ ^/api/v1/auth/(login|register|forgot-password)$ {
    limit_req zone=auth_limit burst=10 nodelay;
    proxy_pass http://backend:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

location ~ ^/api/v1/(search/suggestions|search/trending|contents/search|users/search)$ {
    limit_req zone=search_limit burst=20 nodelay;
    proxy_pass http://backend:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Keep the existing catch-all `location /` for all other API routes.

- [ ] **Step 3: Syntax check template manually**

Do not start production traffic. Run this command only if Docker is already available:

```powershell
docker run --rm -v ${PWD}/docs/deploy/nginx.omnicraft.single-server.conf:/etc/nginx/conf.d/default.conf:ro nginx:1.27-alpine nginx -t
```

Expected: `syntax is ok` and `test is successful`.

## Task 10: Full Verification And Commit

**Files:**
- All files changed in this plan.

- [ ] **Step 1: Backend verification**

Run:

```powershell
cd backend
go test ./...
go build ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 2: Frontend verification if frontend changed**

Run:

```powershell
cd frontend
npm run lint
npm run test
```

Expected: PASS.

- [ ] **Step 3: Confirm no load test artifacts**

Run:

```powershell
git status --short
rg -n "k6|vegeta|locust" .
```

Expected: no new load-test scripts or generated traffic artifacts from this task. If `rg` finds old references in existing docs, do not edit them; confirm no new files or diffs were created for load testing.

- [ ] **Step 4: Commit exact files**

Run:

```powershell
git add backend/config/config.go backend/config/config_test.go backend/config.yaml backend/internal/middleware/ratelimit.go backend/internal/middleware/ratelimit_test.go backend/internal/middleware/body_limit.go backend/internal/middleware/body_limit_test.go backend/cmd/server/main.go backend/cmd/server/main_test.go backend/internal/handler/search.go backend/internal/handler/search_abuse_test.go backend/internal/handler/tag.go backend/internal/handler/routes.go frontend/lib/error-handler.ts docs/deploy/nginx.omnicraft.single-server.conf
git commit -m "Security: harden abuse controls"
```

Do not stage unrelated frontend files or translation files.
