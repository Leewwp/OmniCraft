package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	OSS        OSSConfig        `mapstructure:"oss"`
	Green      GreenConfig      `mapstructure:"green"`
	Features   FeaturesConfig   `mapstructure:"features"`
	Limits     LimitsConfig     `mapstructure:"limits"`
	Reputation ReputationConfig `mapstructure:"reputation"`
	Judge      JudgeConfig      `mapstructure:"judge"`
	Social     SocialConfig     `mapstructure:"social"`
	Upload     UploadConfig     `mapstructure:"upload"`
	Agent      AgentConfig      `mapstructure:"agent"`
	Cache      CacheConfig      `mapstructure:"cache"`
	RateLimit  RateLimitConfig  `mapstructure:"rate_limit"`
}

type AgentConfig struct {
	WebAgentEnabled       bool   `mapstructure:"web_agent_enabled"`
	LLMProvider           string `mapstructure:"llm_provider"`
	LLMModel              string `mapstructure:"llm_model"`
	LLMAPIBase            string `mapstructure:"llm_api_base"`
	LLMAPIKey             string `mapstructure:"llm_api_key"`
	EmbeddingModel        string `mapstructure:"embedding_model"`
	EmbeddingDimensions   int    `mapstructure:"embedding_dimensions"`
	RateLimitPerDay       int    `mapstructure:"rate_limit_per_day"`
	UploadAssistMaxFileMB int    `mapstructure:"upload_assist_max_file_mb"`
	HMACSecret            string `mapstructure:"hmac_secret"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	DSN     string `mapstructure:"dsn"`
	ReadDSN string `mapstructure:"read_dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret          string `mapstructure:"secret"`
	AccessTokenTTL  int    `mapstructure:"access_token_ttl"`
	RefreshTokenTTL int    `mapstructure:"refresh_token_ttl"`
}

type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	BucketName      string `mapstructure:"bucket_name"`
	Domain          string `mapstructure:"domain"`
}

type GreenConfig struct {
	AccessKeyID        string   `mapstructure:"access_key_id"`
	AccessKeySecret    string   `mapstructure:"access_key_secret"`
	Region             string   `mapstructure:"region"`
	CallbackURL        string   `mapstructure:"callback_url"`
	CallbackAllowedIPs []string `mapstructure:"callback_allowed_ips"`
}

type FeaturesConfig struct {
	PaymentEnabled        bool `mapstructure:"payment_enabled"`
	CreatorSupportEnabled bool `mapstructure:"creator_support_enabled"`
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

type CacheConfig struct {
	ContentListTTL         int `mapstructure:"content_list_ttl"`
	ContentDetailTTL       int `mapstructure:"content_detail_ttl"`
	IPListTTL              int `mapstructure:"ip_list_ttl"`
	IPDetailTTL            int `mapstructure:"ip_detail_ttl"`
	ViewCountFlushInterval int `mapstructure:"view_count_flush_interval"`
}

type RateLimitConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	NormalPerMinute int  `mapstructure:"normal_per_minute"`
	UploadPerHour   int  `mapstructure:"upload_per_hour"`
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
		log.Printf("Warning: config file not found, using defaults/env vars: %v", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}

	overrideFromEnv(cfg)

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
			log.Printf("Warning: failed loading env file %s: %v", candidate, err)
			continue
		}
		log.Printf("Loaded environment from %s", candidate)
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
}
