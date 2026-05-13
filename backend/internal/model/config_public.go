package model

import "omnicraft/backend/config"

type PublicConfig struct {
	Features       config.FeaturesConfig       `json:"features"`
	Limits         config.LimitsConfig         `json:"limits"`
	Reputation     config.ReputationConfig     `json:"reputation"`
	Judge          config.JudgeConfig           `json:"judge"`
	Social         config.SocialConfig          `json:"social"`
	Agent          PublicAgentConfig            `json:"agent"`
	Upload         config.UploadConfig          `json:"upload"`
	Cache          config.CacheConfig            `json:"cache"`
	RateLimit      config.RateLimitConfig       `json:"rate_limit"`
	Recommendation config.RecommendationConfig  `json:"recommendation"`
}

type PublicAgentConfig struct {
	WebAgentEnabled       bool `json:"web_agent_enabled"`
	RateLimitPerDay       int  `json:"rate_limit_per_day"`
	UploadAssistMaxFileMB int  `json:"upload_assist_max_file_mb"`
}

type ConfigRedactStatus struct {
	JWTSecretConfigured   bool `json:"jwt_secret_configured"`
	OSSKeyConfigured      bool `json:"oss_key_configured"`
	GreenKeyConfigured    bool `json:"green_key_configured"`
	LLMApiKeyConfigured   bool `json:"llm_api_key_configured"`
	HMACSecretConfigured  bool `json:"hmac_secret_configured"`
	DatabaseConfigured    bool `json:"database_configured"`
	RedisConfigured       bool `json:"redis_configured"`
}