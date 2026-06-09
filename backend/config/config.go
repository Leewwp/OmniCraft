package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"omnicraft/backend/internal/pkg/queue"
)

type Config struct {
	Server         ServerConfig         `mapstructure:"server"`
	Web            WebConfig            `mapstructure:"web"`
	Database       DatabaseConfig       `mapstructure:"database"`
	Redis          RedisConfig          `mapstructure:"redis"`
	JWT            JWTConfig            `mapstructure:"jwt"`
	OSS            OSSConfig            `mapstructure:"oss"`
	Green          GreenConfig          `mapstructure:"green"`
	Security       SecurityConfig       `mapstructure:"security"`
	Features       FeaturesConfig       `mapstructure:"features"`
	Limits         LimitsConfig         `mapstructure:"limits"`
	Reputation     ReputationConfig     `mapstructure:"reputation"`
	Judge          JudgeConfig          `mapstructure:"judge"`
	Social         SocialConfig         `mapstructure:"social"`
	Upload         UploadConfig         `mapstructure:"upload"`
	Publish        PublishConfig        `mapstructure:"publish"`
	Agent          AgentConfig          `mapstructure:"agent"`
	SMTP           SMTPConfig           `mapstructure:"smtp"`
	Captcha        CaptchaConfig        `mapstructure:"captcha"`
	Verification   VerificationConfig   `mapstructure:"verification"`
	Legal          LegalConfig          `mapstructure:"legal"`
	Client         ClientConfig         `mapstructure:"client"`
	Feedback       FeedbackConfig       `mapstructure:"feedback"`
	Cache          CacheConfig          `mapstructure:"cache"`
	RateLimit      RateLimitConfig      `mapstructure:"rate_limit"`
	Recommendation RecommendationConfig `mapstructure:"recommendation"`
	Queue          queue.QueueConfig    `mapstructure:"queue"`
}

type ServerConfig struct {
	Port            string `mapstructure:"port"`
	Mode            string `mapstructure:"mode"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
}

type SecurityConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}

type DatabaseConfig struct {
	DSN     string `mapstructure:"dsn" json:"-"`
	ReadDSN string `mapstructure:"read_dsn" json:"-"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr" json:"-"`
	Password string `mapstructure:"password" json:"-"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret          string `mapstructure:"secret" json:"-"`
	AccessTokenTTL  int    `mapstructure:"access_token_ttl"`
	RefreshTokenTTL int    `mapstructure:"refresh_token_ttl"`
}

type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id" json:"-"`
	AccessKeySecret string `mapstructure:"access_key_secret" json:"-"`
	BucketName      string `mapstructure:"bucket_name"`
	Domain          string `mapstructure:"domain"`
	DownloadURLTTL  int    `mapstructure:"download_url_ttl_sec"`
}

type GreenConfig struct {
	AccessKeyID        string   `mapstructure:"access_key_id" json:"-"`
	AccessKeySecret    string   `mapstructure:"access_key_secret" json:"-"`
	Region             string   `mapstructure:"region"`
	CallbackURL        string   `mapstructure:"callback_url" json:"-"`
	CallbackAllowedIPs []string `mapstructure:"callback_allowed_ips" json:"-"`
}

type FeaturesConfig struct {
	PaymentEnabled        bool `mapstructure:"payment_enabled"`
	CreatorSupportEnabled bool `mapstructure:"creator_support_enabled"`
	DesktopDeployEnabled  bool `mapstructure:"desktop_deploy_enabled"`
}

type LimitsConfig struct {
	VideoMaxMB      int `mapstructure:"video_max_mb"`
	VideoMaxSec     int `mapstructure:"video_max_sec"`
	ImageMaxMB      int `mapstructure:"image_max_mb"`
	TextMaxMB       int `mapstructure:"text_max_mb"`
	ModMaxMB        int `mapstructure:"mod_max_mb"`
	SheetMusicMaxMB int `mapstructure:"sheet_music_max_mb"`
}

type ReputationConfig struct {
	QualityContentThreshold     int `mapstructure:"quality_content_threshold"`
	QualityCommentThreshold     int `mapstructure:"quality_comment_threshold"`
	MinScoreForInteraction      int `mapstructure:"min_score_for_interaction"`
	RepeatViolationWindowDays   int `mapstructure:"repeat_violation_window_days"`
	RepeatViolationThreshold    int `mapstructure:"repeat_violation_threshold"`
	RepeatViolationExtraPenalty int `mapstructure:"repeat_violation_extra_penalty"`
}

type JudgeConfig struct {
	MinVotesRequired int     `mapstructure:"min_votes_required"`
	PassThreshold    float64 `mapstructure:"pass_threshold"`
	ExamPassRate     float64 `mapstructure:"exam_pass_rate"`
	ErrorRateRevoke  float64 `mapstructure:"error_rate_revoke"`
	ErrorRateWindow  int     `mapstructure:"error_rate_window"`
}

type SocialConfig struct {
	ReportAutoHideRate   float64 `mapstructure:"report_auto_hide_rate"`
	CommentFoldThreshold float64 `mapstructure:"comment_fold_threshold"`
}

type UploadConfig struct {
	SheetMusicExtensions []string `mapstructure:"sheet_music_extensions"`
}

type PublishConfig struct {
	RequireReview     bool     `mapstructure:"require_review"`
	MaxDailyPosts     int      `mapstructure:"max_daily_posts"`
	FreezeOnViolation bool     `mapstructure:"freeze_on_violation"`
	TypeOrderOriginal []string `mapstructure:"type_order_original"`
	TypeOrderFanwork  []string `mapstructure:"type_order_fanwork"`
}

type AgentConfig struct {
	WebAgentEnabled       bool   `mapstructure:"web_agent_enabled"`
	LLMProvider           string `mapstructure:"llm_provider"`
	LLMModel              string `mapstructure:"llm_model"`
	LLMAPIBase            string `mapstructure:"llm_api_base"`
	LLMAPIKey             string `mapstructure:"llm_api_key" json:"-"`
	EmbeddingModel        string `mapstructure:"embedding_model"`
	EmbeddingDimensions   int    `mapstructure:"embedding_dimensions"`
	RateLimitPerDay       int    `mapstructure:"rate_limit_per_day"`
	UploadAssistMaxFileMB int    `mapstructure:"upload_assist_max_file_mb"`
	HMACSecret            string `mapstructure:"hmac_secret" json:"-"`
	MaxUserMessageChars   int    `mapstructure:"max_user_message_chars"`
	ChatMaxContextMsgs    int    `mapstructure:"chat_max_context_messages"`
}

type CaptchaConfig struct {
	Provider        string `mapstructure:"provider"`
	Prefix          string `mapstructure:"prefix"`
	SceneID         string `mapstructure:"scene_id"`
	Region          string `mapstructure:"region"`
	AccessKeyID     string `mapstructure:"access_key_id" json:"-"`
	AccessKeySecret string `mapstructure:"access_key_secret" json:"-"`
	TicketTTLSec    int    `mapstructure:"ticket_ttl_sec"`
}

type ClientConfig struct {
	DownloadEnabled bool   `mapstructure:"download_enabled"`
	DownloadURL     string `mapstructure:"download_url"`
	LatestVersion   string `mapstructure:"latest_version"`
}

type FeedbackConfig struct {
	UploadGrantTTLSec int `mapstructure:"upload_grant_ttl_sec"`
}

type WebConfig struct {
	PublicBaseURL string `mapstructure:"public_base_url"`
}

type SMTPConfig struct {
	Mode        string `mapstructure:"mode"`
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password" json:"-"`
	FromAddress string `mapstructure:"from_address"`
}

type VerificationConfig struct {
	EmailTTLSec           int `mapstructure:"email_ttl_sec"`
	ResetTTLSec           int `mapstructure:"reset_ttl_sec"`
	ResendCooldownSec     int `mapstructure:"resend_cooldown_sec"`
	LoginCaptchaThreshold int `mapstructure:"login_captcha_threshold"`
	PasswordMinLength     int `mapstructure:"password_min_length"`
	RegisterPendingTTLSec int `mapstructure:"register_pending_ttl_sec"`
}

type LegalConfig struct {
	CurrentTermsVersion   string `mapstructure:"current_terms_version"`
	CurrentPrivacyVersion string `mapstructure:"current_privacy_version"`
}

type CacheConfig struct {
	ContentListTTL         int `mapstructure:"content_list_ttl"`
	ContentDetailTTL       int `mapstructure:"content_detail_ttl"`
	IPListTTL              int `mapstructure:"ip_list_ttl"`
	IPDetailTTL            int `mapstructure:"ip_detail_ttl"`
	ViewCountFlushInterval int `mapstructure:"view_count_flush_interval"`
	HotRankZSetTTL         int `mapstructure:"hot_rank_zset_ttl"`
	UserStatusTTL          int `mapstructure:"user_status_ttl"`
	TagCacheTTL            int `mapstructure:"tag_cache_ttl"`
	EmailVerifyTTL         int `mapstructure:"email_verify_ttl"`
	PasswordResetTTL       int `mapstructure:"password_reset_ttl"`
	PublishFreezeTTL       int `mapstructure:"publish_freeze_ttl"`
}

type RateLimitConfig struct {
	Enabled             bool  `mapstructure:"enabled"`
	NormalPerMinute     int   `mapstructure:"normal_per_minute"`
	UploadPerHour       int   `mapstructure:"upload_per_hour"`
	NormalWindowSec     int   `mapstructure:"normal_window_sec"`
	UploadWindowSec     int   `mapstructure:"upload_window_sec"`
	AgentWindowSec      int   `mapstructure:"agent_window_sec"`
	CredentialPerMinute int   `mapstructure:"credential_per_minute"`
	SearchPerMinute     int   `mapstructure:"search_per_minute"`
	MaxJSONBodyBytes    int64 `mapstructure:"max_json_body_bytes"`
	MaxQueryChars       int   `mapstructure:"max_query_chars"`
	MaxSearchLimit      int   `mapstructure:"max_search_limit"`
}

type RecommendationConfig struct {
	Enabled                      bool    `mapstructure:"enabled"`
	HotDecayHours                float64 `mapstructure:"hot_decay_hours"`
	PersonalizationWeight        float64 `mapstructure:"personalization_weight"`
	MinInteractionForPersonalize int     `mapstructure:"min_interaction_for_personalize"`
	EmbeddingTopk                int     `mapstructure:"embedding_topk"`
	TrendingWindowDays           int     `mapstructure:"trending_window_days"`
	RefreshIntervalH             int     `mapstructure:"refresh_interval_h"`
	RankIntervalMin              int     `mapstructure:"rank_interval_min"`
	EmbeddingMultiplier          int     `mapstructure:"embedding_multiplier"`
}

var Cfg *Config

func Load() *Config {
	loadDotEnvFiles()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		slog.Warn("config file not found, using defaults/env vars", "error", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		slog.Error("Failed to unmarshal config", "error", err)
		os.Exit(1)
	}

	overrideFromEnv(cfg)

	OverridePath := "data/config_override.yaml"
	if v := os.Getenv("CONFIG_OVERRIDE_PATH"); v != "" {
		OverridePath = v
	}
	LoadOverride(cfg, OverridePath)

	Cfg = cfg
	return cfg
}

func loadDotEnvFiles() {
	candidates := []string{".env", filepath.Join("..", ".env")}
	for _, candidate := range candidates {
		if err := godotenv.Overload(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			slog.Warn("failed loading env file", "file", candidate, "error", err)
			continue
		}
		slog.Info("Loaded environment", "file", candidate)
		return
	}
}

func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("DB_READ_DSN"); v != "" {
		cfg.Database.ReadDSN = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = n
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("OSS_ENDPOINT"); v != "" {
		cfg.OSS.Endpoint = v
	}
	if v := os.Getenv("OSS_ACCESS_KEY_ID"); v != "" {
		cfg.OSS.AccessKeyID = v
	}
	if v := os.Getenv("OSS_ACCESS_KEY_SECRET"); v != "" {
		cfg.OSS.AccessKeySecret = v
	}
	if v := os.Getenv("OSS_BUCKET_NAME"); v != "" {
		cfg.OSS.BucketName = v
	}
	if v := os.Getenv("OSS_DOMAIN"); v != "" {
		cfg.OSS.Domain = v
	}
	if v := os.Getenv("GREEN_ACCESS_KEY_ID"); v != "" {
		cfg.Green.AccessKeyID = v
	}
	if v := os.Getenv("GREEN_ACCESS_KEY_SECRET"); v != "" {
		cfg.Green.AccessKeySecret = v
	}
	if v := os.Getenv("GREEN_REGION"); v != "" {
		cfg.Green.Region = v
	}
	if v := os.Getenv("GREEN_CALLBACK_URL"); v != "" {
		cfg.Green.CallbackURL = v
	}
	if v := os.Getenv("GREEN_CALLBACK_ALLOWED_IPS"); v != "" {
		parts := strings.Split(v, ",")
		ips := make([]string, 0, len(parts))
		for _, part := range parts {
			ip := strings.TrimSpace(part)
			if ip != "" {
				ips = append(ips, ip)
			}
		}
		if len(ips) > 0 {
			cfg.Green.CallbackAllowedIPs = ips
		}
	}
	if v := os.Getenv("AGENT_LLM_API_KEY"); v != "" {
		cfg.Agent.LLMAPIKey = v
	}
	if v := os.Getenv("AGENT_LLM_PROVIDER"); v != "" {
		cfg.Agent.LLMProvider = v
	}
	if v := os.Getenv("AGENT_LLM_MODEL"); v != "" {
		cfg.Agent.LLMModel = v
	}
	if v := os.Getenv("AGENT_LLM_API_BASE"); v != "" {
		cfg.Agent.LLMAPIBase = v
	}
	if v := os.Getenv("AGENT_HMAC_SECRET"); v != "" {
		cfg.Agent.HMACSecret = v
	}
	if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
		cfg.Security.AllowedOrigins = strings.Split(v, ",")
	}
	if v := os.Getenv("CAPTCHA_ACCESS_KEY_ID"); v != "" {
		cfg.Captcha.AccessKeyID = v
	}
	if v := os.Getenv("CAPTCHA_ACCESS_KEY_SECRET"); v != "" {
		cfg.Captcha.AccessKeySecret = v
	}
	if v := os.Getenv("CAPTCHA_TICKET_TTL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Captcha.TicketTTLSec = n
		}
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		cfg.SMTP.Password = v
	}
}

func (c *Config) SaveOverride(path string) error {
	v := viper.New()
	v.Set("features", c.Features)
	v.Set("limits", c.Limits)
	v.Set("reputation", c.Reputation)
	v.Set("social", c.Social)
	v.Set("agent", map[string]interface{}{
		"web_agent_enabled":         c.Agent.WebAgentEnabled,
		"rate_limit_per_day":        c.Agent.RateLimitPerDay,
		"upload_assist_max_file_mb": c.Agent.UploadAssistMaxFileMB,
	})
	v.Set("judge", c.Judge)
	v.Set("recommendation", c.Recommendation)
	v.Set("cache", c.Cache)
	v.Set("rate_limit", c.RateLimit)
	return v.WriteConfigAs(path)
}

func LoadOverride(base *Config, path string) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return
	}
	if err := v.Unmarshal(base); err != nil {
		slog.Warn("failed to merge config override", "error", err)
	}
}

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

func (c *Config) ValidateRelease() error {
	if c.Server.Mode != "release" {
		return nil
	}
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
}
