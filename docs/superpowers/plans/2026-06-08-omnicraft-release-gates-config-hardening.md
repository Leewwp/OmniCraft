# OmniCraft Release Gates And Config Hardening Implementation Plan

> ✅ **完成状态**: 本计划全部步骤已于 2026-06-09 执行完毕。执行记录见 `docs/superpowers/progress/2026-06-09-security-hardening-execution.md`。以下步骤仅保留原始计划结构作历史参考。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make unsafe production configuration impossible to start and keep deployment documentation aligned with the actual startup checks.

**Architecture:** Keep release validation centralized in `config.Config.ValidateRelease()` so startup, tests, and future deployment tooling share one gate. Add small validation helpers in `backend/config/config.go`, focused table tests in `backend/config/config_test.go`, and update deploy docs so operators do not follow stale instructions.

**Tech Stack:** Go config package, Viper/env overrides, Docker Compose deployment docs, Go unit tests.

---

## Scope And Mode

This plan is a security-hardening plan, not a historical `task.json` task and not a checked Beta roadmap item by itself. When implementing, first choose the active project mode per `AGENTS.md`; do not mark `task.json` or Beta checkboxes complete unless the maintainer explicitly converts this plan into one of those task sources.

This plan does not handle the Tauri desktop security chain.

## Current Repo Facts

- `backend/cmd/server/main.go` already calls `cfg.ValidateRelease()` immediately after `config.Load()` and before `database.Init(cfg)`, so this plan strengthens an existing startup gate rather than adding a new startup hook.
- `backend/config/config.go` currently rejects only release-mode `captcha.provider=bypass`, `smtp.mode=logger`, and the development JWT secret. It does not yet reject localhost origins, missing Redis password, missing OSS/Green/CAPTCHA/SMTP secrets, empty legal versions, or desktop deploy being enabled.
- `backend/config.yaml` intentionally contains development defaults such as `web.public_base_url: http://localhost:3000`, empty OSS secrets, `captcha.provider: bypass`, and `smtp.mode: logger`; those defaults must remain usable in debug mode and must be rejected in release mode.

## File Structure

- Modify: `backend/config/config.go`
  - Owns release-only validation and helper functions.
- Modify: `backend/config/config_test.go`
  - Owns table-driven validation tests for production blockers.
- Modify: `docs/deploy/production-config-template.md`
  - Owns production configuration guidance and known caveats.
- Modify: `docs/deploy/single-server-beta-runbook.md`
  - Owns single-server deployment commands and verification checklist.
- Modify: `.env.example`
  - Add placeholder names only when a release gate references an env var that is missing from `.env.example`. Do not add real values.

## Success Criteria

- `go test ./config -run ValidateRelease -count=1` passes.
- `go test ./...` passes.
- Starting backend in release mode with missing critical config fails before opening the HTTP listener.
- Starting backend in release mode with a fully valid config passes validation.
- Deploy docs no longer claim that `ValidateRelease()` is not called at startup.
- No real secrets are committed.

## Task 1: Add Release Validation Tests

**Files:**
- Modify: `backend/config/config_test.go`

- [ ] **Step 1: Write the release happy-path test**

Add a helper and a passing release baseline near the existing `ValidateRelease` tests:

```go
func validReleaseConfigForTest() *Config {
	return &Config{
		Server: ServerConfig{Mode: "release", Port: "8080", ShutdownTimeout: 15},
		Web: WebConfig{PublicBaseURL: "https://app.example.com"},
		Security: SecurityConfig{AllowedOrigins: []string{"https://app.example.com"}},
		Database: DatabaseConfig{DSN: "host=db port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=require"},
		Redis: RedisConfig{Addr: "redis:6379", Password: "redis-secret", DB: 0},
		JWT: JWTConfig{Secret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		OSS: OSSConfig{
			Endpoint: "https://oss-cn-hangzhou.aliyuncs.com",
			AccessKeyID: "oss-key-id",
			AccessKeySecret: "oss-key-secret",
			BucketName: "private-bucket",
			Domain: "https://private-bucket.oss-cn-hangzhou.aliyuncs.com",
			DownloadURLTTL: 300,
		},
		Green: GreenConfig{
			AccessKeyID: "green-key-id",
			AccessKeySecret: "green-key-secret",
			Region: "cn-shanghai",
			CallbackURL: "https://api.example.com/api/v1/internal/ai-callback",
			CallbackAllowedIPs: []string{"203.0.113.10"},
		},
		Features: FeaturesConfig{
			PaymentEnabled: false,
			CreatorSupportEnabled: false,
			DesktopDeployEnabled: false,
		},
		Captcha: CaptchaConfig{
			Provider: "aliyun_v2",
			Prefix: "captcha-prefix",
			SceneID: "captcha-scene",
			Region: "cn",
			AccessKeyID: "captcha-key-id",
			AccessKeySecret: "captcha-key-secret",
		},
		SMTP: SMTPConfig{
			Mode: "smtp",
			Host: "smtp.example.com",
			Port: 587,
			User: "mailer@example.com",
			Password: "smtp-secret",
			FromAddress: "noreply@example.com",
		},
		Legal: LegalConfig{
			CurrentTermsVersion: "2026-06-08",
			CurrentPrivacyVersion: "2026-06-08",
		},
		Client: ClientConfig{
			DownloadEnabled: false,
			DownloadURL: "",
			LatestVersion: "",
		},
		Agent: AgentConfig{
			WebAgentEnabled: false,
			RateLimitPerDay: 50,
			UploadAssistMaxFileMB: 10,
			MaxUserMessageChars: 4000,
			ChatMaxContextMsgs: 10,
		},
		RateLimit: RateLimitConfig{
			Enabled: true,
			NormalPerMinute: 100,
			UploadPerHour: 200,
		},
	}
}

func TestValidateReleaseAcceptsCompleteProductionConfig(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	cfg := validReleaseConfigForTest()
	if err := cfg.ValidateRelease(); err != nil {
		t.Fatalf("ValidateRelease() unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run the focused test**

Run:

```powershell
cd backend
go test ./config -run ValidateRelease -count=1
```

Expected: PASS. This test documents the complete valid release baseline; the red phase for this task comes from the negative cases in Step 3.

- [ ] **Step 3: Write failing negative tests**

Add a table test:

```go
func TestValidateReleaseRejectsIncompleteProductionConfig(t *testing.T) {
	cases := []struct {
		name string
		mutate func(*Config)
		want string
	}{
		{"localhost public base url", func(c *Config) { c.Web.PublicBaseURL = "http://localhost:3000" }, "web.public_base_url"},
		{"missing allowed origins", func(c *Config) { c.Security.AllowedOrigins = nil }, "security.allowed_origins"},
		{"wildcard allowed origin", func(c *Config) { c.Security.AllowedOrigins = []string{"*"} }, "wildcard"},
		{"localhost allowed origin", func(c *Config) { c.Security.AllowedOrigins = []string{"http://localhost:3000"} }, "localhost"},
		{"missing redis password", func(c *Config) { c.Redis.Password = "" }, "redis.password"},
		{"missing oss endpoint", func(c *Config) { c.OSS.Endpoint = "" }, "oss.endpoint"},
		{"missing oss secret", func(c *Config) { c.OSS.AccessKeySecret = "" }, "oss.access_key_secret"},
		{"missing green callback url", func(c *Config) { c.Green.CallbackURL = "" }, "green.callback_url"},
		{"missing green callback allowed ips", func(c *Config) { c.Green.CallbackAllowedIPs = nil }, "green.callback_allowed_ips"},
		{"missing captcha public fields", func(c *Config) { c.Captcha.Prefix = "" }, "captcha.prefix"},
		{"missing captcha secret", func(c *Config) { c.Captcha.AccessKeySecret = "" }, "captcha.access_key_secret"},
		{"missing smtp host", func(c *Config) { c.SMTP.Host = "" }, "smtp.host"},
		{"missing smtp password", func(c *Config) { c.SMTP.Password = "" }, "smtp.password"},
		{"missing terms version", func(c *Config) { c.Legal.CurrentTermsVersion = "" }, "legal.current_terms_version"},
		{"missing privacy version", func(c *Config) { c.Legal.CurrentPrivacyVersion = "" }, "legal.current_privacy_version"},
		{"desktop deploy enabled", func(c *Config) { c.Features.DesktopDeployEnabled = true }, "desktop_deploy_enabled"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			tc.mutate(cfg)
			err := cfg.ValidateRelease()
			if err == nil {
				t.Fatal("ValidateRelease() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateRelease() error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateReleaseRejectsMissingLLMKeyEncryptionSecret(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "")
	cfg := validReleaseConfigForTest()
	err := cfg.ValidateRelease()
	if err == nil {
		t.Fatal("ValidateRelease() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "LLM_KEY_ENCRYPTION_SECRET") {
		t.Fatalf("ValidateRelease() error = %q, want LLM_KEY_ENCRYPTION_SECRET", err.Error())
	}
}
```

Add `strings` to the test imports. `backend/config/config_test.go` already imports `strings`; keep that import and do not duplicate it.

- [ ] **Step 4: Run the failing tests**

Run:

```powershell
cd backend
go test ./config -run ValidateRelease -count=1
```

Expected: FAIL on several cases because `ValidateRelease()` currently checks only captcha provider, SMTP mode, and JWT secret.

## Task 2: Implement Strict Release Validation

**Files:**
- Modify: `backend/config/config.go`

- [ ] **Step 1: Add validation helpers**

Add helper functions below `ValidateRelease()` or above it:

```go
func requireNonEmpty(errs *[]string, field, value string) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, field+" is required in release mode")
	}
}

func isLocalURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "http://localhost") ||
		strings.HasPrefix(lower, "https://localhost") ||
		strings.HasPrefix(lower, "http://127.0.0.1") ||
		strings.HasPrefix(lower, "https://127.0.0.1")
}

func requireHTTPSURL(errs *[]string, field, raw string) {
	requireNonEmpty(errs, field, raw)
	if strings.TrimSpace(raw) == "" {
		return
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "https://") {
		*errs = append(*errs, field+" must use https in release mode")
	}
	if isLocalURL(raw) {
		*errs = append(*errs, field+" must not use localhost in release mode")
	}
}

func requireAllowedOrigins(errs *[]string, origins []string) {
	if len(origins) == 0 {
		*errs = append(*errs, "security.allowed_origins is required in release mode")
		return
	}
	for _, origin := range origins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			*errs = append(*errs, "security.allowed_origins must not contain empty origins")
			continue
		}
		if trimmed == "*" {
			*errs = append(*errs, "security.allowed_origins must not contain wildcard origins in release mode")
		}
		if !strings.HasPrefix(strings.ToLower(trimmed), "https://") {
			*errs = append(*errs, "security.allowed_origins entries must use https in release mode")
		}
		if isLocalURL(trimmed) {
			*errs = append(*errs, "security.allowed_origins must not contain localhost origins in release mode")
		}
	}
}
```

- [ ] **Step 2: Expand `ValidateRelease()`**

Replace the body after `if c.Server.Mode != "release" { return nil }` with explicit checks:

```go
var errs []string

requireHTTPSURL(&errs, "web.public_base_url", c.Web.PublicBaseURL)
requireAllowedOrigins(&errs, c.Security.AllowedOrigins)

requireNonEmpty(&errs, "database.dsn", c.Database.DSN)
if strings.Contains(strings.ToLower(c.Database.DSN), "password=omnicraft") {
	errs = append(errs, "database.dsn must not use the default development password in release mode")
}
requireNonEmpty(&errs, "redis.addr", c.Redis.Addr)
requireNonEmpty(&errs, "redis.password", c.Redis.Password)

if strings.TrimSpace(c.JWT.Secret) == "" || c.JWT.Secret == "dev-secret-change-in-production" || len(c.JWT.Secret) < 32 {
	errs = append(errs, "jwt.secret must be a production secret of at least 32 characters in release mode")
}

requireHTTPSURL(&errs, "oss.endpoint", c.OSS.Endpoint)
requireNonEmpty(&errs, "oss.access_key_id", c.OSS.AccessKeyID)
requireNonEmpty(&errs, "oss.access_key_secret", c.OSS.AccessKeySecret)
requireNonEmpty(&errs, "oss.bucket_name", c.OSS.BucketName)
requireHTTPSURL(&errs, "oss.domain", c.OSS.Domain)
if c.OSS.DownloadURLTTL <= 0 || c.OSS.DownloadURLTTL > 3600 {
	errs = append(errs, "oss.download_url_ttl_sec must be between 1 and 3600 in release mode")
}

requireNonEmpty(&errs, "green.access_key_id", c.Green.AccessKeyID)
requireNonEmpty(&errs, "green.access_key_secret", c.Green.AccessKeySecret)
requireNonEmpty(&errs, "green.region", c.Green.Region)
requireHTTPSURL(&errs, "green.callback_url", c.Green.CallbackURL)
if len(c.Green.CallbackAllowedIPs) == 0 {
	errs = append(errs, "green.callback_allowed_ips is required in release mode")
}

if c.Captcha.Provider == "bypass" || strings.TrimSpace(c.Captcha.Provider) == "" {
	errs = append(errs, "captcha.provider must not be 'bypass' in release mode; use 'aliyun_v2'")
}
requireNonEmpty(&errs, "captcha.prefix", c.Captcha.Prefix)
requireNonEmpty(&errs, "captcha.scene_id", c.Captcha.SceneID)
requireNonEmpty(&errs, "captcha.access_key_id", c.Captcha.AccessKeyID)
requireNonEmpty(&errs, "captcha.access_key_secret", c.Captcha.AccessKeySecret)

if c.SMTP.Mode == "logger" || strings.TrimSpace(c.SMTP.Mode) == "" {
	errs = append(errs, "smtp.mode must not be 'logger' in release mode; use 'smtp'")
}
requireNonEmpty(&errs, "smtp.host", c.SMTP.Host)
requireNonEmpty(&errs, "smtp.user", c.SMTP.User)
requireNonEmpty(&errs, "smtp.password", c.SMTP.Password)
requireNonEmpty(&errs, "smtp.from_address", c.SMTP.FromAddress)

requireNonEmpty(&errs, "legal.current_terms_version", c.Legal.CurrentTermsVersion)
requireNonEmpty(&errs, "legal.current_privacy_version", c.Legal.CurrentPrivacyVersion)

if c.Features.DesktopDeployEnabled {
	errs = append(errs, "features.desktop_deploy_enabled must remain false until desktop security gates are complete")
}

if c.Agent.WebAgentEnabled {
	requireNonEmpty(&errs, "agent.llm_api_key", c.Agent.LLMAPIKey)
	if c.Agent.RateLimitPerDay <= 0 {
		errs = append(errs, "agent.rate_limit_per_day must be positive when web agent is enabled")
	}
}

requireNonEmpty(&errs, "LLM_KEY_ENCRYPTION_SECRET", os.Getenv("LLM_KEY_ENCRYPTION_SECRET"))

if c.RateLimit.Enabled && c.RateLimit.NormalPerMinute <= 0 {
	errs = append(errs, "rate_limit.normal_per_minute must be positive when rate limiting is enabled")
}

if len(errs) > 0 {
	return fmt.Errorf("release mode configuration error: %s", strings.Join(errs, "; "))
}
return nil
```

Keep existing imports. `fmt` and `strings` are already imported in this file.

- [ ] **Step 3: Run focused tests**

Run:

```powershell
cd backend
go test ./config -run ValidateRelease -count=1
```

Expected: PASS.

## Task 3: Verify Startup Uses The Gate Before External Initialization

**Files:**
- Read: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/main_test.go`

- [ ] **Step 1: Confirm startup calls validation**

Inspect:

```powershell
rg -n "ValidateRelease" backend/cmd/server/main.go backend/cmd/server/main_test.go
```

Expected: `main.go` calls `cfg.ValidateRelease()` before database initialization and listener startup.

- [ ] **Step 2: Replace the existing source-level regression test**

`backend/cmd/server/main_test.go` already contains `TestMainCallsValidateReleaseAfterLoadingConfig`. Replace that test with the following single test so there are not two overlapping source-order tests:

```go
func TestMainCallsValidateReleaseAfterLoadAndBeforeExternalInit(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(src)
	loadIdx := strings.Index(text, "cfg := config.Load()")
	validateIdx := strings.Index(text, "cfg.ValidateRelease()")
	dbIdx := strings.Index(text, "database.Init")
	if loadIdx < 0 {
		t.Fatal("main must load config")
	}
	if validateIdx < 0 {
		t.Fatal("main must call cfg.ValidateRelease()")
	}
	if validateIdx < loadIdx {
		t.Fatal("ValidateRelease must run after config load")
	}
	if dbIdx < 0 {
		t.Fatal("main must initialize database")
	}
	if validateIdx > dbIdx {
		t.Fatal("ValidateRelease must run before database initialization")
	}
	redisIdx := strings.Index(text, "redisclient.Init")
	if redisIdx < 0 {
		t.Fatal("main must initialize redis")
	}
	if validateIdx > redisIdx {
		t.Fatal("ValidateRelease must run before redis initialization")
	}
}
```

- [ ] **Step 3: Run server tests**

Run:

```powershell
cd backend
go test ./cmd/server -count=1
```

Expected: PASS.

## Task 4: Align Deployment Documentation

**Files:**
- Modify: `docs/deploy/production-config-template.md`
- Modify: `docs/deploy/single-server-beta-runbook.md`
- Modify: `.env.example` only when Step 4 finds a missing placeholder.

- [ ] **Step 1: Update stale caveats**

In `docs/deploy/production-config-template.md`, remove or replace any caveat claiming `ValidateRelease()` is not called at startup.

Replacement text:

```markdown
- `ValidateRelease()` runs during backend startup. A release-mode backend must fail
  fast when required production inputs are missing or unsafe.
```

- [ ] **Step 2: Add a release gate checklist**

Add the following checklist section to both deploy docs. If a checklist with the same heading already exists, update the existing checklist in place instead of adding a duplicate section:

```markdown
## Release Gate Checklist

- `server.mode: "release"` is set through `CONFIG_OVERRIDE_PATH`.
- `web.public_base_url` and all `security.allowed_origins` entries use HTTPS production domains.
- `JWT_SECRET`, `REDIS_PASSWORD`, `POSTGRES_PASSWORD`, `LLM_KEY_ENCRYPTION_SECRET`, OSS keys, Green keys, CAPTCHA keys, and SMTP password are real secrets stored outside Git.
- `captcha.provider` is not `bypass`.
- `smtp.mode` is not `logger`.
- `legal.current_terms_version` and `legal.current_privacy_version` are non-empty.
- `features.desktop_deploy_enabled` remains `false`.
- `features.payment_enabled` remains `false` unless payment is separately approved.
```

- [ ] **Step 3: Verify referenced files exist**

Run:

```powershell
Test-Path docs/deploy/nginx.omnicraft.single-server.conf
Test-Path docs/deploy/docker-compose.single-server.yml
```

Expected: both return `True`.

- [ ] **Step 4: Verify `.env.example` placeholders**

Run:

```powershell
rg -n "LLM_KEY_ENCRYPTION_SECRET|CAPTCHA_ACCESS_KEY_ID|CAPTCHA_ACCESS_KEY_SECRET|SMTP_PASSWORD" .env.example
```

Expected: all four placeholder names are present. If any are missing, add placeholder-only entries to `.env.example`:

```dotenv
LLM_KEY_ENCRYPTION_SECRET=
CAPTCHA_ACCESS_KEY_ID=
CAPTCHA_ACCESS_KEY_SECRET=
SMTP_PASSWORD=
```

Do not add real credentials or environment-specific production values.

## Task 5: Full Verification And Commit

**Files:**
- All files changed in this plan.

- [ ] **Step 1: Run backend checks**

Run:

```powershell
cd backend
go test ./...
go build ./...
go vet ./...
```

Expected: all PASS.

- [ ] **Step 2: Check for accidental secrets**

Run:

```powershell
git diff -- backend/config/config.go backend/config/config_test.go docs/deploy/production-config-template.md docs/deploy/single-server-beta-runbook.md .env.example
```

Expected: only placeholders and validation code; no real credentials.

- [ ] **Step 3: Commit exact files only**

Run:

```powershell
git add backend/config/config.go backend/config/config_test.go docs/deploy/production-config-template.md docs/deploy/single-server-beta-runbook.md
git commit -m "Security: harden release configuration gates"
```

If `.env.example` was changed because a referenced env var placeholder was missing, include it explicitly in `git add`. Otherwise do not stage `.env.example`.
