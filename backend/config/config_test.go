package config

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// validateReleaseErrPrefix is the stable error prefix returned by
// Config.ValidateRelease(). ValidateRelease() returns a plain fmt.Errorf
// (not a structured AppError), so we cannot use errors.As to unwrap a code
// field. Instead, we assert on two things: (1) the stable prefix that
// identifies the error category, and (2) the field-path substring that
// identifies the specific validation rule. This is more robust than a
// single strings.Contains on an arbitrary human-readable substring because
// the prefix acts as a stable error-code-like identifier even though the
// production code does not yet use a structured error type.
const validateReleaseErrPrefix = "release mode configuration error"

func loadDefaultConfigForTest(t *testing.T) *Config {
	t.Helper()

	raw, err := os.ReadFile("../config.yaml")
	require.NoError(t, err)

	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(string(raw))))

	var cfg Config
	require.NoError(t, v.Unmarshal(&cfg))
	return &cfg
}

func TestValidateReleaseRejectsBypassCaptchaAndLoggerSMTP(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Mode = "release"
	cfg.Captcha.Provider = "bypass"
	cfg.SMTP.Mode = "logger"

	err := cfg.ValidateRelease()
	require.Error(t, err)
	// Assert stable error-category prefix (acts as structured code substitute).
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix),
		"error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
}

func TestValidateReleaseRejectsDefaultJWTSecret(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Mode = "release"
	cfg.Captcha.Provider = "aliyun_v2"
	cfg.SMTP.Mode = "smtp"
	cfg.JWT.Secret = "dev-secret-change-in-production"

	err := cfg.ValidateRelease()
	require.Error(t, err)
	// Assert stable error-category prefix (acts as structured code substitute).
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix),
		"error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
	require.ErrorContains(t, err, "jwt.secret")
}

func validReleaseConfigForTest() *Config {
	return &Config{
		Server:   ServerConfig{Mode: "release", Port: "8080", ShutdownTimeout: 15, ReadTimeout: 30, WriteTimeout: 60, IdleTimeout: 120},
		Web:      WebConfig{PublicBaseURL: "https://app.omnicraft.prod"},
		Security: SecurityConfig{AllowedOrigins: []string{"https://app.omnicraft.prod"}},
		Database: DatabaseConfig{
			DSN: "host=db.omnicraft.prod port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=verify-full",
		},
		Redis: RedisConfig{Addr: "redis:6379", Password: "redis-secret", DB: 0},
		JWT: JWTConfig{
			Secret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
		OSS: OSSConfig{
			Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
			AccessKeyID:     "oss-key-id",
			AccessKeySecret: "oss-key-secret",
			BucketName:      "private-bucket",
			Domain:          "https://private-bucket.oss-cn-hangzhou.aliyuncs.com",
			DownloadURLTTL:  300,
		},
		Green: GreenConfig{
			AccessKeyID:        "green-key-id",
			AccessKeySecret:    "green-key-secret",
			Region:             "cn-shanghai",
			CallbackURL:        "https://api.omnicraft.prod/api/v1/internal/ai-callback",
			CallbackAllowedIPs: []string{"203.0.113.10"},
		},
		Features: FeaturesConfig{
			PaymentEnabled:        false,
			CreatorSupportEnabled: false,
			DesktopDeployEnabled:  false,
		},
		Captcha: CaptchaConfig{
			Provider:        "aliyun_v2",
			Prefix:          "captcha-prefix",
			SceneID:         "captcha-scene",
			Region:          "cn",
			AccessKeyID:     "captcha-key-id",
			AccessKeySecret: "captcha-key-secret",
		},
		SMTP: SMTPConfig{
			Mode:        "smtp",
			Host:        "smtp.omnicraft.prod",
			Port:        587,
			User:        "mailer@omnicraft.prod",
			Password:    "smtp-secret",
			FromAddress: "noreply@omnicraft.prod",
		},
		Legal: LegalConfig{
			CurrentTermsVersion:   "2026-06-08",
			CurrentPrivacyVersion: "2026-06-08",
		},
		Client: ClientConfig{
			DownloadEnabled: false,
			DownloadURL:     "",
			LatestVersion:   "",
		},
		Agent: AgentConfig{
			WebAgentEnabled:       false,
			RateLimitPerDay:       50,
			UploadAssistMaxFileMB: 10,
			MaxUserMessageChars:   4000,
			ChatMaxContextMsgs:    10,
		},
		RateLimit: RateLimitConfig{
			Enabled:         true,
			NormalPerMinute: 100,
			UploadPerHour:   200,
		},
		Observability: ObservabilityConfig{
			MetricsPort:          "9091",
			LogLevel:             "info",
			LogIPHashSecret:      "observability-ip-hash-secret-value",
			LogIPKeyID:           "current",
			ReadHeaderTimeoutSec: 5,
			Readiness:            ReadinessConfig{DBTimeoutSec: 3, RedisTimeoutSec: 3},
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

func TestValidateReleaseRejectsIncompleteProductionConfig(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
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
			// Assert the stable error-category prefix first; this acts as a
			// structured error-code substitute since ValidateRelease returns
			// plain fmt.Errorf, not a typed error with a Code field.
			if !strings.HasPrefix(err.Error(), validateReleaseErrPrefix) {
				t.Fatalf("ValidateRelease() error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
			}
			// Then assert the specific field-path identifier is present.
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
	// Assert stable error-category prefix (acts as structured code substitute).
	if !strings.HasPrefix(err.Error(), validateReleaseErrPrefix) {
		t.Fatalf("ValidateRelease() error = %q, want prefix %q", err.Error(), validateReleaseErrPrefix)
	}
	if !strings.Contains(err.Error(), "LLM_KEY_ENCRYPTION_SECRET") {
		t.Fatalf("ValidateRelease() error = %q, want substring LLM_KEY_ENCRYPTION_SECRET", err.Error())
	}
}

func TestDefaultConfigDeclaresCreatorSupportDisabled(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	require.False(t, cfg.Features.CreatorSupportEnabled)
}

func TestDefaultConfigHasAbuseControlLimits(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Positive(t, cfg.RateLimit.NormalPerMinute)
	require.Positive(t, cfg.RateLimit.UploadPerHour)
	require.Positive(t, cfg.RateLimit.CredentialPerMinute)
	require.Positive(t, cfg.RateLimit.SearchPerMinute)
	require.Positive(t, cfg.RateLimit.MaxJSONBodyBytes)
	require.Positive(t, cfg.RateLimit.MaxQueryChars)
	require.Positive(t, cfg.RateLimit.MaxSearchPage)
}

func TestDefaultConfigJSONBodyLimitAllowsTextUploads(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)
	minBodyBytes := int64(cfg.Limits.TextMaxMB) * 1024 * 1024
	require.GreaterOrEqual(t, cfg.RateLimit.MaxJSONBodyBytes, minBodyBytes)
}

func TestBrowseHistoryConfig(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Equal(t, 7, cfg.BrowseHistory.RetentionDays)
	require.Equal(t, "03:00", cfg.BrowseHistory.CleanupTime)
}

func TestApplyTestModeReplacesNormalDatabaseConfigurationAfterOverrides(t *testing.T) {
	t.Setenv("OMNICRAFT_TEST_MODE", "1")
	t.Setenv("OMNICRAFT_TEST_DB_DSN", "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft_test_cross_stack sslmode=disable")
	t.Setenv("OMNICRAFT_TEST_REDIS_DB", "15")

	cfg := &Config{
		Database: DatabaseConfig{DSN: "host=db.omnicraft.prod dbname=production", ReadDSN: "host=replica.omnicraft.prod dbname=production"},
		Redis:    RedisConfig{Addr: "127.0.0.1:6379", DB: 0},
	}

	require.NoError(t, applyTestMode(cfg))
	require.Equal(t, "host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft_test_cross_stack sslmode=disable", cfg.Database.DSN)
	require.Empty(t, cfg.Database.ReadDSN)
	require.Equal(t, 15, cfg.Redis.DB)
}

func TestApplyTestModeRejectsUnsafeTestConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		redisDB  string
		wantPart string
	}{
		{"remote database", "host=db.omnicraft.prod dbname=omnicraft_test_cross_stack", "15", "loopback"},
		{"normal database name", "host=127.0.0.1 dbname=omnicraft", "15", "omnicraft_test_"},
		{"redis db zero", "host=127.0.0.1 dbname=omnicraft_test_cross_stack", "0", "non-zero"},
		{"invalid redis db", "host=127.0.0.1 dbname=omnicraft_test_cross_stack", "not-a-number", "valid integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OMNICRAFT_TEST_MODE", "1")
			t.Setenv("OMNICRAFT_TEST_DB_DSN", tt.dsn)
			t.Setenv("OMNICRAFT_TEST_REDIS_DB", tt.redisDB)

			err := applyTestMode(&Config{Redis: RedisConfig{Addr: "127.0.0.1:6379"}})
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantPart)
		})
	}
}

func TestApplyTestModeRequiresLoopbackRedisWithValidPort(t *testing.T) {
	tests := []struct {
		name      string
		redisAddr string
		wantErr   bool
	}{
		{"localhost", "localhost:6379", false},
		{"ipv4 loopback", "127.0.0.1:6379", false},
		{"ipv6 loopback", "[::1]:6379", false},
		{"remote host", "redis.omnicraft.prod:6379", true},
		{"empty address", "", true},
		{"missing port", "127.0.0.1", true},
		{"invalid port", "127.0.0.1:not-a-port", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OMNICRAFT_TEST_MODE", "1")
			t.Setenv("OMNICRAFT_TEST_DB_DSN", "host=127.0.0.1 dbname=omnicraft_test_cross_stack")
			t.Setenv("OMNICRAFT_TEST_REDIS_DB", "15")

			err := applyTestMode(&Config{Redis: RedisConfig{Addr: tt.redisAddr}})
			if (err != nil) != tt.wantErr {
				t.Fatalf("applyTestMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyTestModeRequiresExplicitTestInputs(t *testing.T) {
	t.Setenv("OMNICRAFT_TEST_MODE", "1")
	t.Setenv("OMNICRAFT_TEST_DB_DSN", "")
	t.Setenv("OMNICRAFT_TEST_REDIS_DB", "")

	err := applyTestMode(&Config{})
	require.ErrorContains(t, err, "OMNICRAFT_TEST_DB_DSN")
}

func TestDefaultConfigDeclaresObservabilitySection(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.Equal(t, "9091", cfg.Observability.MetricsPort)
	require.Equal(t, "info", cfg.Observability.LogLevel)
	require.NotEmpty(t, cfg.Observability.LogIPKeyID)
	require.Positive(t, cfg.Observability.Readiness.DBTimeoutSec)
	require.Positive(t, cfg.Observability.Readiness.RedisTimeoutSec)
}

func TestValidateReleaseRequiresIPHashSecretAndKeyID(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = ""
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "observability.log_ip_hash_secret")

	cfg = validReleaseConfigForTest()
	cfg.Observability.LogIPKeyID = ""
	err = cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "observability.log_ip_key_id")
}

func TestValidateReleaseRejectsIncompleteIPKeyRotation(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation.PreviousKeyID = "previous"

	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "observability.ip_key_rotation")
}

func TestValidateReleaseRejectsMalformedRotationWindow(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation = IPKeyRotationConfig{
		PreviousSecret: "previous-secret-value",
		PreviousKeyID:  "previous",
		ActiveFrom:     "not-a-timestamp",
		ActiveUntil:    "2026-12-31T23:59:59Z",
	}

	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "ip_key_rotation")
}

func TestValidateReleaseRejectsReversedRotationWindow(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation = IPKeyRotationConfig{
		PreviousSecret: "previous-secret-value",
		PreviousKeyID:  "previous",
		ActiveFrom:     "2026-12-31T23:59:59Z",
		ActiveUntil:    "2026-01-01T00:00:00Z",
	}

	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "ip_key_rotation")
}

func TestValidateReleaseAcceptsValidRotationWindow(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")

	cfg := validReleaseConfigForTest()
	cfg.Observability.LogIPHashSecret = "current-secret-value"
	cfg.Observability.LogIPKeyID = "current"
	cfg.Observability.IPKeyRotation = IPKeyRotationConfig{
		PreviousSecret: "previous-secret-value",
		PreviousKeyID:  "previous",
		ActiveFrom:     "2026-01-01T00:00:00Z",
		ActiveUntil:    "2026-12-31T23:59:59Z",
	}

	require.NoError(t, cfg.ValidateRelease())
}

func TestValidateReleaseRejectsPlaceholderValues(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"angle placeholder in jwt secret", func(c *Config) { c.JWT.Secret = "<openssl-rand-base64-64>" }, "jwt.secret"},
		{"change-me placeholder in redis password", func(c *Config) { c.Redis.Password = "CHANGE_ME" }, "redis.password"},
		{"example domain in public base url", func(c *Config) { c.Web.PublicBaseURL = "https://app.example.com" }, "web.public_base_url"},
		{"placeholder in smtp host", func(c *Config) { c.SMTP.Host = "<smtp-host>" }, "smtp.host"},
		{"placeholder in oss bucket", func(c *Config) { c.OSS.BucketName = "your-bucket-name" }, "oss.bucket_name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validReleaseConfigForTest()
			tc.mutate(cfg)
			err := cfg.ValidateRelease()
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestValidateReleaseRequiresVerifyFullDatabaseTLS(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Database.DSN = "host=db.omnicraft.prod port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=disable"
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "sslmode")

	cfg = validReleaseConfigForTest()
	cfg.Database.DSN = "host=db.omnicraft.prod port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=require"
	err = cfg.ValidateRelease()
	require.Error(t, err)
	require.ErrorContains(t, err, "verify-full")
}

func TestValidateReleaseAllowsPrivateDbHostTLSNegotiation(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "pgbouncer")

	cfg := validReleaseConfigForTest()
	cfg.Database.DSN = "host=pgbouncer port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=disable"
	require.NoError(t, cfg.ValidateRelease())
}

func TestValidateReleaseRejectsCallbackAllowedIPPlaceholders(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("OMNICRAFT_PRIVATE_DB_HOSTS", "")

	cfg := validReleaseConfigForTest()
	cfg.Green.CallbackAllowedIPs = []string{"<comma-separated-ip-list>"}
	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), validateReleaseErrPrefix))
	require.ErrorContains(t, err, "green.callback_allowed_ips")
}

func TestApplyTestModeLeavesNormalConfigurationUnchanged(t *testing.T) {
	t.Setenv("OMNICRAFT_TEST_MODE", "")
	t.Setenv("OMNICRAFT_TEST_DB_DSN", "host=127.0.0.1 dbname=omnicraft_test_ignored")
	t.Setenv("OMNICRAFT_TEST_REDIS_DB", "15")

	cfg := &Config{Database: DatabaseConfig{DSN: "host=db.omnicraft.prod dbname=production", ReadDSN: "host=replica.omnicraft.prod dbname=production"}, Redis: RedisConfig{DB: 0}}
	require.NoError(t, applyTestMode(cfg))
	require.Equal(t, "host=db.omnicraft.prod dbname=production", cfg.Database.DSN)
	require.Equal(t, "host=replica.omnicraft.prod dbname=production", cfg.Database.ReadDSN)
	require.Equal(t, 0, cfg.Redis.DB)
}
