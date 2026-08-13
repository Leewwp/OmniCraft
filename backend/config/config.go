package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"omnicraft/backend/internal/pkg/queue"
)

var (
	greenSeedFormat = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)
	greenUIDFormat  = regexp.MustCompile(`^\d+$`)
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
	Collaboration  CollaborationConfig  `mapstructure:"collaboration"`
	BrowseHistory  BrowseHistoryConfig  `mapstructure:"browse_history"`
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
	Observability  ObservabilityConfig  `mapstructure:"observability"`
}

type ObservabilityConfig struct {
	MetricsPort          string              `mapstructure:"metrics_port"`
	LogLevel             string              `mapstructure:"log_level"`
	LogIPHashSecret      string              `mapstructure:"log_ip_hash_secret" json:"-"`
	LogIPKeyID           string              `mapstructure:"log_ip_key_id"`
	ReadHeaderTimeoutSec int                 `mapstructure:"read_header_timeout_sec"`
	IPKeyRotation        IPKeyRotationConfig `mapstructure:"ip_key_rotation"`
	Readiness            ReadinessConfig     `mapstructure:"readiness"`
}

// IPKeyRotationConfig limits the previous IP-hash key to an explicit
// rotation window so past hashes can be correlated only while rotating.
type IPKeyRotationConfig struct {
	PreviousSecret string `mapstructure:"previous_secret" json:"-"`
	PreviousKeyID  string `mapstructure:"previous_key_id"`
	ActiveFrom     string `mapstructure:"active_from"`
	ActiveUntil    string `mapstructure:"active_until"`
}

type ReadinessConfig struct {
	DBTimeoutSec    int `mapstructure:"db_timeout_sec"`
	RedisTimeoutSec int `mapstructure:"redis_timeout_sec"`
}

type ServerConfig struct {
	Port            string `mapstructure:"port"`
	Mode            string `mapstructure:"mode"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
	ReadTimeout     int    `mapstructure:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout"`
	IdleTimeout     int    `mapstructure:"idle_timeout"`
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
	AccessKeyID     string `mapstructure:"access_key_id" json:"-"`
	AccessKeySecret string `mapstructure:"access_key_secret" json:"-"`
	Region          string `mapstructure:"region"`
	CallbackURL     string `mapstructure:"callback_url" json:"-"`
	// Seed is the callback signature seed (green.seed): release-required, [A-Za-z0-9_], max 64 chars.
	Seed string `mapstructure:"seed" json:"-"`
	// UID is the Aliyun main account UID (green.uid): release-required, digits only (console account info, not RAM UID).
	UID string `mapstructure:"uid" json:"-"`
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

	// Score values (positive = award, negative = penalty).
	// Zero means "use the hardcoded default in reputation_service.go".
	ScoreQualityContent     int `mapstructure:"score_quality_content"`
	ScorePRMerged           int `mapstructure:"score_pr_merged"`
	ScoreQualityComment     int `mapstructure:"score_quality_comment"`
	ScoreTagRecognized      int `mapstructure:"score_tag_recognized"`
	ScoreJudgeAccuracy      int `mapstructure:"score_judge_accuracy"`
	ScoreRehabCourse        int `mapstructure:"score_rehab_course"`
	ScoreValidReport        int `mapstructure:"score_valid_report"`
	ScoreMaliciousContent   int `mapstructure:"score_malicious_content"`
	ScoreMaliciousPR        int `mapstructure:"score_malicious_pr"`
	ScoreMaliciousComment   int `mapstructure:"score_malicious_comment"`
	ScoreMaliciousReport    int `mapstructure:"score_malicious_report"`
	ScoreMaliciousTagReport int `mapstructure:"score_malicious_tag_report"`
	ScoreJudgeError         int `mapstructure:"score_judge_error"`
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

// CollaborationConfig carries server-only collaboration invite limits.
// Only MaxInviteesPerPublish is exposed through the public config endpoint;
// daily limits, expiry and contributor capacity must never reach the client.
type CollaborationConfig struct {
	InviteDailyLimit       int `mapstructure:"invite_daily_limit"`
	InviteExpireDays       int `mapstructure:"invite_expire_days"`
	MaxInviteesPerPublish  int `mapstructure:"max_invitees_per_publish"`
	MaxContributorsPerItem int `mapstructure:"max_contributors_per_item"`
}

type BrowseHistoryConfig struct {
	RetentionDays int    `mapstructure:"retention_days"`
	CleanupTime   string `mapstructure:"cleanup_time"`
}

type UploadConfig struct {
	SheetMusicExtensions []string `mapstructure:"sheet_music_extensions"`
	// Media set (media gallery) size bounds for newly published image/video
	// content. Zero means "use the specification default" so tests and
	// minimal configs keep working.
	ImageGalleryMinItems int `mapstructure:"image_gallery_min_items"`
	ImageGalleryMaxItems int `mapstructure:"image_gallery_max_items"`
	VideoGalleryMinItems int `mapstructure:"video_gallery_min_items"`
	VideoGalleryMaxItems int `mapstructure:"video_gallery_max_items"`
}

// NormalizedGalleryLimits fills only omitted media-gallery limits with the
// specification defaults. It is shared by the publish validator and the
// public-config response so clients and the backend enforce the same bounds.
func (u UploadConfig) NormalizedGalleryLimits() UploadConfig {
	if u.ImageGalleryMinItems == 0 {
		u.ImageGalleryMinItems = 2
	}
	if u.ImageGalleryMaxItems == 0 {
		u.ImageGalleryMaxItems = 9
	}
	if u.VideoGalleryMinItems == 0 {
		u.VideoGalleryMinItems = 1
	}
	if u.VideoGalleryMaxItems == 0 {
		u.VideoGalleryMaxItems = 3
	}
	return u
}

func (u UploadConfig) ValidateGalleryLimits() error {
	if u.ImageGalleryMinItems < 0 || u.ImageGalleryMaxItems < 0 {
		return fmt.Errorf("image gallery limits must not be negative")
	}
	if u.VideoGalleryMinItems < 0 || u.VideoGalleryMaxItems < 0 {
		return fmt.Errorf("video gallery limits must not be negative")
	}
	n := u.NormalizedGalleryLimits()
	if n.ImageGalleryMinItems > n.ImageGalleryMaxItems {
		return fmt.Errorf("image gallery min_items must not exceed max_items")
	}
	if n.VideoGalleryMinItems > n.VideoGalleryMaxItems {
		return fmt.Errorf("video gallery min_items must not exceed max_items")
	}
	return nil
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
	RateLimitPerMinute    int    `mapstructure:"rate_limit_per_minute"`
	MaxToolCallsPerTurn   int    `mapstructure:"max_tool_calls_per_turn"`
	MaxOutputTokens       int    `mapstructure:"max_output_tokens"`
	ProviderTimeoutSec    int    `mapstructure:"provider_timeout_sec"`
	ProviderMaxRetries    int    `mapstructure:"provider_max_retries"`
	CitationMaxCount      int    `mapstructure:"citation_max_count"`
	UploadAssistMaxFileMB int    `mapstructure:"upload_assist_max_file_mb"`
	HMACSecret            string `mapstructure:"hmac_secret" json:"-"`
	MaxUserMessageChars   int    `mapstructure:"max_user_message_chars"`
	ChatMaxContextMsgs    int    `mapstructure:"chat_max_context_messages"`
	ConversationListLimit int    `mapstructure:"conversation_list_limit"`
	ConversationPageSize  int    `mapstructure:"conversation_page_size"`
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
	Enabled              bool  `mapstructure:"enabled"`
	NormalPerMinute      int   `mapstructure:"normal_per_minute"`
	UploadPerHour        int   `mapstructure:"upload_per_hour"`
	NormalWindowSec      int   `mapstructure:"normal_window_sec"`
	UploadWindowSec      int   `mapstructure:"upload_window_sec"`
	AgentWindowSec       int   `mapstructure:"agent_window_sec"`
	AgentMinuteWindowSec int   `mapstructure:"agent_minute_window_sec"`
	CredentialPerMinute  int   `mapstructure:"credential_per_minute"`
	SearchPerMinute      int   `mapstructure:"search_per_minute"`
	MaxJSONBodyBytes     int64 `mapstructure:"max_json_body_bytes"`
	MaxQueryChars        int   `mapstructure:"max_query_chars"`
	MaxSearchLimit       int   `mapstructure:"max_search_limit"`
	MaxSearchPage        int   `mapstructure:"max_search_page"`
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
	// Do not enable viper.AutomaticEnv here: with SetEnvKeyReplacer it maps a
	// top-level section key (e.g. "agent") to an env var of the same name
	// (AGENT) and a generic value such as AGENT=1 silently shadows the entire
	// YAML section. All runtime overrides flow through OverrideFromEnv below.

	if err := viper.ReadInConfig(); err != nil {
		slog.Warn("config file not found, using defaults/env vars", "error", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		slog.Error("Failed to unmarshal config", "error", err)
		os.Exit(1)
	}

	OverridePath := "data/config_override.yaml"
	if v := os.Getenv("CONFIG_OVERRIDE_PATH"); v != "" {
		OverridePath = v
	}
	LoadOverride(cfg, OverridePath)
	// Explicit environment variables are the final runtime authority. In
	// particular, deployment overrides written by the admin UI must not turn
	// off an explicitly enabled Agent or replace its provider credentials.
	OverrideFromEnv(cfg)
	if err := applyTestMode(cfg); err != nil {
		slog.Error("invalid test mode configuration", "error", err)
		os.Exit(1)
	}

	Cfg = cfg
	return cfg
}

func applyTestMode(cfg *Config) error {
	if os.Getenv("OMNICRAFT_TEST_MODE") != "1" {
		return nil
	}

	// Read replicas must never be used by tests, even if a normal config
	// source supplied one before test-mode validation fails.
	cfg.Database.ReadDSN = ""

	dsn := strings.TrimSpace(os.Getenv("OMNICRAFT_TEST_DB_DSN"))
	if dsn == "" {
		return fmt.Errorf("OMNICRAFT_TEST_DB_DSN is required when OMNICRAFT_TEST_MODE=1")
	}
	host, database, err := testDatabaseDSNHostAndName(dsn)
	if err != nil {
		return fmt.Errorf("invalid OMNICRAFT_TEST_DB_DSN: %w", err)
	}
	if !isLoopbackDatabaseHost(host) {
		return fmt.Errorf("OMNICRAFT_TEST_DB_DSN host %q must be loopback", host)
	}
	if !strings.HasPrefix(database, "omnicraft_test_") || len(database) == len("omnicraft_test_") {
		return fmt.Errorf("OMNICRAFT_TEST_DB_DSN database %q must use the omnicraft_test_ prefix", database)
	}
	if err := validateTestRedisAddr(cfg.Redis.Addr); err != nil {
		return err
	}

	redisDBRaw := strings.TrimSpace(os.Getenv("OMNICRAFT_TEST_REDIS_DB"))
	redisDB, err := strconv.Atoi(redisDBRaw)
	if err != nil || redisDB <= 0 {
		return fmt.Errorf("OMNICRAFT_TEST_REDIS_DB must be a valid integer and non-zero")
	}

	cfg.Database.DSN = dsn
	cfg.Redis.DB = redisDB
	return nil
}

func testDatabaseDSNHostAndName(dsn string) (string, string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "", "", err
		}
		return parsed.Hostname(), strings.TrimPrefix(parsed.EscapedPath(), "/"), nil
	}

	values := make(map[string]string)
	for _, part := range strings.Fields(dsn) {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			values[strings.ToLower(key)] = strings.Trim(value, "'")
		}
	}
	return values["host"], values["dbname"], nil
}

func isLoopbackDatabaseHost(host string) bool {
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(trimmed, "localhost") {
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func validateTestRedisAddr(raw string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("redis.addr %q must be a valid loopback host:port in test mode", raw)
	}
	if !isLoopbackDatabaseHost(host) {
		return fmt.Errorf("redis.addr host %q must be loopback in test mode", host)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber <= 0 || portNumber > 65535 {
		return fmt.Errorf("redis.addr port %q must be valid in test mode", port)
	}
	return nil
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

func OverrideFromEnv(cfg *Config) {
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
	if v := os.Getenv("GREEN_SEED"); v != "" {
		cfg.Green.Seed = v
	}
	if v := os.Getenv("GREEN_UID"); v != "" {
		cfg.Green.UID = v
	}
	if v := os.Getenv("AGENT_LLM_API_KEY"); v != "" {
		cfg.Agent.LLMAPIKey = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("AGENT_WEB_AGENT_ENABLED"); v != "" {
		cfg.Agent.WebAgentEnabled = v == "1" || strings.EqualFold(v, "true")
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
	if v := os.Getenv("AGENT_EMBEDDING_MODEL"); v != "" {
		cfg.Agent.EmbeddingModel = v
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
	if v := os.Getenv("LOG_IP_HASH_SECRET"); v != "" {
		cfg.Observability.LogIPHashSecret = v
	}
	if v := os.Getenv("LOG_IP_KEY_ID"); v != "" {
		cfg.Observability.LogIPKeyID = v
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
		"rate_limit_per_minute":     c.Agent.RateLimitPerMinute,
		"max_tool_calls_per_turn":   c.Agent.MaxToolCallsPerTurn,
		"max_output_tokens":         c.Agent.MaxOutputTokens,
		"provider_timeout_sec":      c.Agent.ProviderTimeoutSec,
		"provider_max_retries":      c.Agent.ProviderMaxRetries,
		"citation_max_count":        c.Agent.CitationMaxCount,
		"upload_assist_max_file_mb": c.Agent.UploadAssistMaxFileMB,
	})
	v.Set("judge", c.Judge)
	v.Set("recommendation", c.Recommendation)
	v.Set("cache", c.Cache)
	v.Set("rate_limit", c.RateLimit)
	return v.WriteConfigAs(path)
}

// LoadOverride merges the override file onto base field-by-field: only keys
// that actually appear in the override file are applied. Whole-struct
// unmarshal of a partially present section would rely on mapstructure decode
// semantics to leave absent siblings untouched; decoding per top-level section
// makes that guarantee explicit and independent of the viper/mapstructure
// version in use (#127).
func LoadOverride(base *Config, path string) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return
	}
	sections := make(map[string]bool)
	for _, key := range v.AllKeys() {
		sections[strings.Split(key, ".")[0]] = true
	}
	if len(sections) == 0 {
		return
	}
	val := reflect.ValueOf(base).Elem()
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag, ok := field.Tag.Lookup("mapstructure")
		if !ok {
			continue
		}
		section := strings.Split(tag, ",")[0]
		if section == "" || !sections[section] {
			continue
		}
		if err := v.UnmarshalKey(section, val.Field(i).Addr().Interface()); err != nil {
			slog.Warn("failed to merge config override section", "section", section, "error", err)
		}
	}
}

func requireNonEmpty(errs *[]string, field, value string) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, field+" is required in release mode")
	}
}

// isPlaceholderValue detects template/placeholder tokens that must never reach
// production configuration. The check is deliberately conservative: angle
// brackets, explicit placeholder phrases and example domains are all rejected.
func isPlaceholderValue(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, token := range []string{"<", ">", "change_me", "replace_me", "placeholder", "your-", "example.com", "example.org"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// requireReleaseValue is requireNonEmpty plus placeholder rejection.
func requireReleaseValue(errs *[]string, field, value string) {
	requireNonEmpty(errs, field, value)
	if strings.TrimSpace(value) != "" && isPlaceholderValue(value) {
		*errs = append(*errs, field+" must not contain placeholders in release mode")
	}
}

// databaseDSNValue extracts a single key from a libpq-style keyword/value DSN
// or a postgres:// URL. Returns "" when the key is absent or unparsable.
func databaseDSNValue(dsn, key string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return ""
		}
		return u.Query().Get(key)
	}
	for _, part := range strings.Fields(dsn) {
		k, v, ok := strings.Cut(part, "=")
		if ok && strings.EqualFold(k, key) {
			return strings.Trim(v, "'")
		}
	}
	return ""
}

// validateDatabaseTLSPolicy requires sslmode=verify-full in release mode.
// Only hosts explicitly listed in OMNICRAFT_PRIVATE_DB_HOSTS (an unexposed
// internal PgBouncer behind the network perimeter) may use TLS negotiation
// because the connection never leaves the private network.
func validateDatabaseTLSPolicy(errs *[]string, dsn string) {
	sslmode := strings.ToLower(strings.TrimSpace(databaseDSNValue(dsn, "sslmode")))
	if sslmode == "" {
		sslmode = "prefer" // libpq default when the key is absent
	}
	if sslmode == "verify-full" {
		return
	}
	host := strings.ToLower(strings.TrimSpace(databaseDSNValue(dsn, "host")))
	privateHosts := make(map[string]bool)
	for _, h := range strings.Split(os.Getenv("OMNICRAFT_PRIVATE_DB_HOSTS"), ",") {
		privateHosts[strings.ToLower(strings.TrimSpace(h))] = true
	}
	if host != "" && privateHosts[host] {
		return
	}
	*errs = append(*errs, fmt.Sprintf(
		"database.dsn sslmode must be verify-full in release mode (host %q is not an approved private-network exception)", host))
}

func requirePositiveInt(errs *[]string, field string, value int) {
	if value <= 0 {
		*errs = append(*errs, field+" must be positive when web agent is enabled")
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
	requireReleaseValue(errs, field, raw)
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
		if isPlaceholderValue(trimmed) {
			*errs = append(*errs, "security.allowed_origins must not contain placeholder origins in release mode")
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
	if err := c.Upload.ValidateGalleryLimits(); err != nil {
		errs = append(errs, "upload."+err.Error())
	}

	requireHTTPSURL(&errs, "web.public_base_url", c.Web.PublicBaseURL)
	requireAllowedOrigins(&errs, c.Security.AllowedOrigins)

	requireNonEmpty(&errs, "database.dsn", c.Database.DSN)
	if strings.Contains(strings.ToLower(c.Database.DSN), "password=omnicraft") {
		errs = append(errs, "database.dsn must not use the default development password in release mode")
	}
	validateDatabaseTLSPolicy(&errs, c.Database.DSN)
	requireNonEmpty(&errs, "redis.addr", c.Redis.Addr)
	requireReleaseValue(&errs, "redis.password", c.Redis.Password)

	if strings.TrimSpace(c.JWT.Secret) == "" || c.JWT.Secret == "dev-secret-change-in-production" || len(c.JWT.Secret) < 32 || isPlaceholderValue(c.JWT.Secret) {
		errs = append(errs, "jwt.secret must be a production secret of at least 32 characters in release mode")
	}

	requireHTTPSURL(&errs, "oss.endpoint", c.OSS.Endpoint)
	requireReleaseValue(&errs, "oss.access_key_id", c.OSS.AccessKeyID)
	requireReleaseValue(&errs, "oss.access_key_secret", c.OSS.AccessKeySecret)
	requireReleaseValue(&errs, "oss.bucket_name", c.OSS.BucketName)
	requireHTTPSURL(&errs, "oss.domain", c.OSS.Domain)
	if c.OSS.DownloadURLTTL <= 0 || c.OSS.DownloadURLTTL > 3600 {
		errs = append(errs, "oss.download_url_ttl_sec must be between 1 and 3600 in release mode")
	}

	requireReleaseValue(&errs, "green.access_key_id", c.Green.AccessKeyID)
	requireReleaseValue(&errs, "green.access_key_secret", c.Green.AccessKeySecret)
	requireReleaseValue(&errs, "green.region", c.Green.Region)
	requireHTTPSURL(&errs, "green.callback_url", c.Green.CallbackURL)
	requireReleaseValue(&errs, "green.seed", c.Green.Seed)
	requireReleaseValue(&errs, "green.uid", c.Green.UID)
	if seed := strings.TrimSpace(c.Green.Seed); seed != "" && !greenSeedFormat.MatchString(seed) {
		errs = append(errs, "green.seed must be 1-64 characters of [A-Za-z0-9_] in release mode")
	}
	if uid := strings.TrimSpace(c.Green.UID); uid != "" && !greenUIDFormat.MatchString(uid) {
		errs = append(errs, "green.uid must be the numeric Aliyun main account UID (digits only) in release mode")
	}

	if c.Captcha.Provider == "bypass" || strings.TrimSpace(c.Captcha.Provider) == "" {
		errs = append(errs, "captcha.provider must not be 'bypass' in release mode; use 'aliyun_v2'")
	}
	requireReleaseValue(&errs, "captcha.prefix", c.Captcha.Prefix)
	requireReleaseValue(&errs, "captcha.scene_id", c.Captcha.SceneID)
	requireReleaseValue(&errs, "captcha.access_key_id", c.Captcha.AccessKeyID)
	requireReleaseValue(&errs, "captcha.access_key_secret", c.Captcha.AccessKeySecret)

	if c.SMTP.Mode == "logger" || strings.TrimSpace(c.SMTP.Mode) == "" {
		errs = append(errs, "smtp.mode must not be 'logger' in release mode; use 'smtp'")
	}
	requireReleaseValue(&errs, "smtp.host", c.SMTP.Host)
	requireReleaseValue(&errs, "smtp.user", c.SMTP.User)
	requireReleaseValue(&errs, "smtp.password", c.SMTP.Password)
	requireReleaseValue(&errs, "smtp.from_address", c.SMTP.FromAddress)

	requireReleaseValue(&errs, "legal.current_terms_version", c.Legal.CurrentTermsVersion)
	requireReleaseValue(&errs, "legal.current_privacy_version", c.Legal.CurrentPrivacyVersion)

	if c.Features.DesktopDeployEnabled {
		errs = append(errs, "features.desktop_deploy_enabled must remain false until desktop security gates are complete")
	}
	if c.Client.DownloadEnabled {
		errs = append(errs, "client.download_enabled must remain false in the Web-only release scope")
	}

	if c.Agent.WebAgentEnabled {
		requireNonEmpty(&errs, "agent.llm_api_key", c.Agent.LLMAPIKey)
		requirePositiveInt(&errs, "agent.rate_limit_per_day", c.Agent.RateLimitPerDay)
		requirePositiveInt(&errs, "agent.rate_limit_per_minute", c.Agent.RateLimitPerMinute)
		requirePositiveInt(&errs, "agent.max_tool_calls_per_turn", c.Agent.MaxToolCallsPerTurn)
		requirePositiveInt(&errs, "agent.max_output_tokens", c.Agent.MaxOutputTokens)
		requirePositiveInt(&errs, "agent.provider_timeout_sec", c.Agent.ProviderTimeoutSec)
		requirePositiveInt(&errs, "agent.citation_max_count", c.Agent.CitationMaxCount)
		requirePositiveInt(&errs, "agent.max_user_message_chars", c.Agent.MaxUserMessageChars)
		requirePositiveInt(&errs, "agent.chat_max_context_messages", c.Agent.ChatMaxContextMsgs)
		requirePositiveInt(&errs, "agent.conversation_list_limit", c.Agent.ConversationListLimit)
		requirePositiveInt(&errs, "agent.conversation_page_size", c.Agent.ConversationPageSize)
		requirePositiveInt(&errs, "rate_limit.agent_window_sec", c.RateLimit.AgentWindowSec)
		requirePositiveInt(&errs, "rate_limit.agent_minute_window_sec", c.RateLimit.AgentMinuteWindowSec)
		if c.Agent.ProviderMaxRetries < 0 {
			errs = append(errs, "agent.provider_max_retries must not be negative when web agent is enabled")
		}
	}

	requireNonEmpty(&errs, "LLM_KEY_ENCRYPTION_SECRET", os.Getenv("LLM_KEY_ENCRYPTION_SECRET"))

	if c.RateLimit.Enabled && c.RateLimit.NormalPerMinute <= 0 {
		errs = append(errs, "rate_limit.normal_per_minute must be positive when rate limiting is enabled")
	}

	if strings.TrimSpace(c.Observability.MetricsPort) == "" {
		errs = append(errs, "observability.metrics_port is required in release mode")
	}
	if strings.TrimSpace(c.Observability.LogIPHashSecret) == "" {
		errs = append(errs, "observability.log_ip_hash_secret is required in release mode (no raw IP may ever be logged)")
	}
	if strings.TrimSpace(c.Observability.LogIPKeyID) == "" {
		errs = append(errs, "observability.log_ip_key_id is required in release mode")
	}
	switch strings.ToLower(strings.TrimSpace(c.Observability.LogLevel)) {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, "observability.log_level must be one of debug, info, warn, error in release mode")
	}
	if c.Observability.Readiness.DBTimeoutSec <= 0 || c.Observability.Readiness.RedisTimeoutSec <= 0 {
		errs = append(errs, "observability.readiness timeouts must be positive in release mode")
	}
	if c.Observability.ReadHeaderTimeoutSec <= 0 {
		errs = append(errs, "observability.read_header_timeout_sec must be positive in release mode")
	}
	if c.Server.ReadTimeout <= 0 || c.Server.WriteTimeout <= 0 || c.Server.IdleTimeout <= 0 {
		errs = append(errs, "server HTTP timeouts must be positive in release mode")
	}
	if err := validateIPKeyRotation(&errs, c.Observability.IPKeyRotation); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("release mode configuration error: %s", strings.Join(errs, "; "))
	}
	return nil
}

// validateIPKeyRotation requires either a complete rotation block or none at
// all. A rotation window must be parseable and strictly increasing.
func validateIPKeyRotation(errs *[]string, rot IPKeyRotationConfig) error {
	set := 0
	for _, field := range []string{rot.PreviousSecret, rot.PreviousKeyID, rot.ActiveFrom, rot.ActiveUntil} {
		if strings.TrimSpace(field) != "" {
			set++
		}
	}
	if set == 0 {
		return nil
	}
	if set != 4 {
		return fmt.Errorf("observability.ip_key_rotation must define previous_secret, previous_key_id, active_from and active_until together")
	}
	from, err := time.Parse(time.RFC3339, rot.ActiveFrom)
	if err != nil {
		return fmt.Errorf("observability.ip_key_rotation.active_from must be RFC3339: %w", err)
	}
	until, err := time.Parse(time.RFC3339, rot.ActiveUntil)
	if err != nil {
		return fmt.Errorf("observability.ip_key_rotation.active_until must be RFC3339: %w", err)
	}
	if !until.After(from) {
		return fmt.Errorf("observability.ip_key_rotation.active_until must be after active_from")
	}
	return nil
}
