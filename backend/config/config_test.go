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
		Server:   ServerConfig{Mode: "release", Port: "8080", ShutdownTimeout: 15},
		Web:      WebConfig{PublicBaseURL: "https://app.example.com"},
		Security: SecurityConfig{AllowedOrigins: []string{"https://app.example.com"}},
		Database: DatabaseConfig{
			DSN: "host=db port=5432 user=omnicraft password=secret dbname=omnicraft sslmode=require",
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
			CallbackURL:        "https://api.example.com/api/v1/internal/ai-callback",
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
			Host:        "smtp.example.com",
			Port:        587,
			User:        "mailer@example.com",
			Password:    "smtp-secret",
			FromAddress: "noreply@example.com",
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
