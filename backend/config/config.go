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

// greenSeedFactoryTemplate is the historical factory template value that used
// to ship in the deployment templates (.env.production.example and the deploy
// docs). It has been public in git history since it was committed, so a
// deployment that keeps it lets anyone who knows the literal (plus the
// account UID) forge /internal/ai-callback checksums. Release validation
// rejects it outright; publishing the literal as a blacklist constant adds no
// new exposure. Security audit 2026-09-11 F-04.
const greenSeedFactoryTemplate = "eGvqrYixTEzFRDUToSd1lgy3plgaMJDqr0X5Ji7P4TY"

const RAGEmbeddingDimensions = 1536

// RAG retrieval defaults mirror backend/config.yaml. They are used only as a
// defensive value for direct repository callers that omit a limit; production
// startup validates the configured values when hybrid retrieval is enabled.
const (
	RAGDefaultBM25TopK   = 200
	RAGDefaultVectorTopK = 200
	RAGDefaultRRFK       = 60
	RAGDefaultFinalTopK  = 10
)

// ValidGreenSeed reports whether seed matches the green.seed format contract
// ([A-Za-z0-9_], 1-64 chars). This is the single definition shared by the
// release config gate, the Green request builder and the release preflight;
// callers apply their own empty-trim and placeholder guards around it.
func ValidGreenSeed(seed string) bool {
	return greenSeedFormat.MatchString(seed)
}

type Config struct {
	Server     ServerConfig     `mapstructure:"server" json:"server"`
	Web        WebConfig        `mapstructure:"web" json:"web"`
	Database   DatabaseConfig   `mapstructure:"database" json:"database"`
	Redis      RedisConfig      `mapstructure:"redis" json:"redis"`
	JWT        JWTConfig        `mapstructure:"jwt" json:"jwt"`
	OSS        OSSConfig        `mapstructure:"oss" json:"oss"`
	Green      GreenConfig      `mapstructure:"green" json:"green"`
	Security   SecurityConfig   `mapstructure:"security" json:"security"`
	Features   FeaturesConfig   `mapstructure:"features" json:"features"`
	Limits     LimitsConfig     `mapstructure:"limits" json:"limits"`
	Reputation ReputationConfig `mapstructure:"reputation" json:"reputation"`
	Judge      JudgeConfig      `mapstructure:"judge" json:"judge"`
	IPProposal IPProposalConfig `mapstructure:"ip_proposal" json:"ip_proposal"`
	// IPCategories is the IP category allowlist (SP-19 G1-3). Kept in sync
	// with the frontend single source frontend/lib/ip-categories.ts; IP
	// creation validates category against it. Extending = frontend constant +
	// this list + ipCategory.* i18n keys in one small PR (GLOSSARY "IP 分类").
	IPCategories   []string             `mapstructure:"ip_categories" json:"ip_categories"`
	Discussion     DiscussionConfig     `mapstructure:"discussion" json:"discussion"`
	Social         SocialConfig         `mapstructure:"social" json:"social"`
	Collaboration  CollaborationConfig  `mapstructure:"collaboration" json:"collaboration"`
	BrowseHistory  BrowseHistoryConfig  `mapstructure:"browse_history" json:"browse_history"`
	Upload         UploadConfig         `mapstructure:"upload" json:"upload"`
	Publish        PublishConfig        `mapstructure:"publish" json:"publish"`
	ArchiveScan    ArchiveScanConfig    `mapstructure:"archive_scan" json:"archive_scan"`
	Agent          AgentConfig          `mapstructure:"agent" json:"agent"`
	AgentAccess    AgentAccessConfig    `mapstructure:"agent_access" json:"agent_access"`
	RAG            RAGConfig            `mapstructure:"rag" json:"rag"`
	SMTP           SMTPConfig           `mapstructure:"smtp" json:"smtp"`
	Captcha        CaptchaConfig        `mapstructure:"captcha" json:"captcha"`
	Verification   VerificationConfig   `mapstructure:"verification" json:"verification"`
	Legal          LegalConfig          `mapstructure:"legal" json:"legal"`
	Client         ClientConfig         `mapstructure:"client" json:"client"`
	Feedback       FeedbackConfig       `mapstructure:"feedback" json:"feedback"`
	Cache          CacheConfig          `mapstructure:"cache" json:"cache"`
	RateLimit      RateLimitConfig      `mapstructure:"rate_limit" json:"rate_limit"`
	Recommendation RecommendationConfig `mapstructure:"recommendation" json:"recommendation"`
	Queue          queue.QueueConfig    `mapstructure:"queue" json:"queue"`
	Relay          RelayConfig          `mapstructure:"relay" json:"relay"`
	Worker         WorkerConfig         `mapstructure:"worker" json:"worker"`
	Observability  ObservabilityConfig  `mapstructure:"observability" json:"observability"`
	Resilience     ResilienceConfig     `mapstructure:"resilience" json:"resilience"`
}

// ResilienceConfig carries the shared failure-isolation tunables
// (SP-24 R5/R6). The breaker section feeds every guarded external dependency
// mount: rerank chain, image API, MCP servers, OpenSearch lexical channel.
// The aux_cache and llm_concurrency sections gate the SP-24 R6 auxiliary
// call caches and per-provider chat concurrency; both default off.
type ResilienceConfig struct {
	Breaker        BreakerConfig        `mapstructure:"breaker" json:"breaker"`
	AuxCache       AuxCacheConfig       `mapstructure:"aux_cache" json:"aux_cache"`
	LLMConcurrency LLMConcurrencyConfig `mapstructure:"llm_concurrency" json:"llm_concurrency"`
}

// BreakerConfig mirrors the polyu three-state blueprint: N consecutive
// failures open the circuit for open_timeout_sec, then a single HALF_OPEN
// probe permit is handed out (CAS; stale probe results cannot mis-close).
// Non-positive values fall back to the construction defaults (2 / 30s).
type BreakerConfig struct {
	FailureThreshold int `mapstructure:"failure_threshold" json:"failure_threshold"`
	OpenTimeoutSec   int `mapstructure:"open_timeout_sec" json:"open_timeout_sec"`
}

// AuxCacheConfig gates the SP-24 R6 Redis TTL caches for auxiliary LLM
// calls: auto conversation title (keyed by the hashed first user message)
// and query expansion terms (keyed by the hashed query). The main Q&A chain
// is never cached. Each item defaults off — zero behavior change until an
// operator flips it.
type AuxCacheConfig struct {
	Title    AuxCacheItemConfig `mapstructure:"title" json:"title"`
	Expander AuxCacheItemConfig `mapstructure:"expander" json:"expander"`
}

// AuxCacheItemConfig is one cache switch plus its TTL. Validation requires
// ttl_sec > 0 on an enabled item, so the shipped defaults double as the
// recommended first-on values.
type AuxCacheItemConfig struct {
	Enabled bool `mapstructure:"enabled" json:"enabled"`
	TTLSec  int  `mapstructure:"ttl_sec" json:"ttl_sec"`
}

// LLMConcurrencyConfig gates the SP-24 R6 per-provider chat concurrency
// semaphore (queueing never fails; only slow queue waits are logged).
// Providers under the same vendor name share one slot pool. Default off.
type LLMConcurrencyConfig struct {
	Enabled        bool `mapstructure:"enabled" json:"enabled"`
	MaxPerProvider int  `mapstructure:"max_per_provider" json:"max_per_provider"`
}

// RelayConfig carries the outbox relay loop tuning (issue #200): batch size
// bounds how many due outbox events one relay run claims and the poll interval
// bounds delivery latency. The relay runs only in the standalone worker
// process (ADR 0005, cmd/worker), so the values are server-side only and never
// reach the public config response. All limits are read from config, never
// hardcoded.
type RelayConfig struct {
	BatchSize       int `mapstructure:"batch_size" json:"batch_size"`
	PollIntervalSec int `mapstructure:"poll_interval_sec" json:"poll_interval_sec"`
}

// AgentAccessConfig configures the external-agent PAT identity (SP-16 #450,
// spec D2). Values are read from config.yaml, never hardcoded (Key Rule 6).
type AgentAccessConfig struct {
	MaxTokensPerUser int `mapstructure:"max_tokens_per_user" json:"max_tokens_per_user"`
}

// Relay tuning bounds guard the relay loop against nonsense configuration in
// release mode (issue #200).
const (
	RelayMinBatchSize       = 1
	RelayMaxBatchSize       = 10000
	RelayMinPollIntervalSec = 1
	RelayMaxPollIntervalSec = 3600
)

// WorkerConfig controls the standalone worker process (cmd/worker) only: the
// API server never starts asynchronous consumers or the outbox relay (ADR
// 0005, issue #138). Concurrency is the number of consumer goroutines per
// topic; messages of a topic are distributed across them by the consumer
// group semantics.
type WorkerConfig struct {
	Enabled     bool   `mapstructure:"enabled" json:"enabled"`
	Concurrency int    `mapstructure:"concurrency" json:"concurrency"`
	ServiceName string `mapstructure:"service_name" json:"service_name"`
}

type ObservabilityConfig struct {
	MetricsPort          string              `mapstructure:"metrics_port" json:"metrics_port"`
	LogLevel             string              `mapstructure:"log_level" json:"log_level"`
	LogIPHashSecret      string              `mapstructure:"log_ip_hash_secret" json:"-"`
	LogIPKeyID           string              `mapstructure:"log_ip_key_id" json:"log_ip_key_id"`
	ReadHeaderTimeoutSec int                 `mapstructure:"read_header_timeout_sec" json:"read_header_timeout_sec"`
	IPKeyRotation        IPKeyRotationConfig `mapstructure:"ip_key_rotation" json:"ip_key_rotation"`
	Readiness            ReadinessConfig     `mapstructure:"readiness" json:"readiness"`
	Tracing              TracingConfig       `mapstructure:"tracing" json:"tracing"`
	AgentTrace           AgentTraceConfig    `mapstructure:"agent_trace" json:"agent_trace"`
}

type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled" json:"enabled"`
	Endpoint    string  `mapstructure:"endpoint" json:"endpoint"`
	SampleRatio float64 `mapstructure:"sample_ratio" json:"sample_ratio"`
	Backend     string  `mapstructure:"backend" json:"backend"`
	ServiceName string  `mapstructure:"service_name" json:"service_name"`
}

// AgentTraceConfig governs the self-built agent trace persistence (SP-21 T1,
// map #549): agent_trace_runs/agent_trace_nodes rows are written by an async
// batch writer off the request path. A full channel drops records and counts
// them; trace persistence must never block or fail an agent turn.
type AgentTraceConfig struct {
	Enabled         bool    `mapstructure:"enabled" json:"enabled"`
	SampleRatio     float64 `mapstructure:"sample_ratio" json:"sample_ratio"`
	ChannelSize     int     `mapstructure:"channel_size" json:"channel_size"`
	FlushIntervalMs int     `mapstructure:"flush_interval_ms" json:"flush_interval_ms"`
	FlushBatchSize  int     `mapstructure:"flush_batch_size" json:"flush_batch_size"`
	DigestMaxRunes  int     `mapstructure:"digest_max_runes" json:"digest_max_runes"`
	KeepFullPrompt  bool    `mapstructure:"keep_full_prompt" json:"keep_full_prompt"`
	RetentionDays   int     `mapstructure:"retention_days" json:"retention_days"`
}

// IPKeyRotationConfig limits the previous IP-hash key to an explicit
// rotation window so past hashes can be correlated only while rotating.
type IPKeyRotationConfig struct {
	PreviousSecret string `mapstructure:"previous_secret" json:"-"`
	PreviousKeyID  string `mapstructure:"previous_key_id" json:"previous_key_id"`
	ActiveFrom     string `mapstructure:"active_from" json:"active_from"`
	ActiveUntil    string `mapstructure:"active_until" json:"active_until"`
}

type ReadinessConfig struct {
	DBTimeoutSec    int `mapstructure:"db_timeout_sec" json:"db_timeout_sec"`
	RedisTimeoutSec int `mapstructure:"redis_timeout_sec" json:"redis_timeout_sec"`
}

type ServerConfig struct {
	Port            string `mapstructure:"port" json:"port"`
	Mode            string `mapstructure:"mode" json:"mode"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout" json:"shutdown_timeout"`
	ReadTimeout     int    `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout" json:"write_timeout"`
	IdleTimeout     int    `mapstructure:"idle_timeout" json:"idle_timeout"`
}

type SecurityConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins" json:"allowed_origins"`
	TrustedProxies []string `mapstructure:"trusted_proxies" json:"trusted_proxies"`
}

type DatabaseConfig struct {
	DSN     string `mapstructure:"dsn" json:"-"`
	ReadDSN string `mapstructure:"read_dsn" json:"-"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr" json:"-"`
	Password string `mapstructure:"password" json:"-"`
	DB       int    `mapstructure:"db" json:"db"`
}

type JWTConfig struct {
	Secret          string `mapstructure:"secret" json:"-"`
	AccessTokenTTL  int    `mapstructure:"access_token_ttl" json:"access_token_ttl"`
	RefreshTokenTTL int    `mapstructure:"refresh_token_ttl" json:"refresh_token_ttl"`
}

type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint" json:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id" json:"-"`
	AccessKeySecret string `mapstructure:"access_key_secret" json:"-"`
	BucketName      string `mapstructure:"bucket_name" json:"bucket_name"`
	Domain          string `mapstructure:"domain" json:"domain"`
	DownloadURLTTL  int    `mapstructure:"download_url_ttl_sec" json:"download_url_ttl_sec"`
	// DisplayURLTTL bounds the signed GET URLs issued for display media
	// (covers, avatars, gallery attachments) at the API serialization
	// boundary. Falls back to the architecture §6.2 1h budget and must stay
	// above the Redis display cache TTL (300s) so cached rows never outlive
	// their re-issued signatures.
	DisplayURLTTL int `mapstructure:"display_url_ttl_sec" json:"display_url_ttl_sec"`
}

type GreenConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id" json:"-"`
	AccessKeySecret string `mapstructure:"access_key_secret" json:"-"`
	Region          string `mapstructure:"region" json:"region"`
	CallbackURL     string `mapstructure:"callback_url" json:"-"`
	// Seed is the callback signature seed (green.seed): release-required, [A-Za-z0-9_], max 64 chars.
	Seed string `mapstructure:"seed" json:"-"`
	// UID is the Aliyun main account UID (green.uid): release-required, digits only (console account info, not RAM UID).
	UID string `mapstructure:"uid" json:"-"`
}

type FeaturesConfig struct {
	PaymentEnabled            bool `mapstructure:"payment_enabled" json:"payment_enabled"`
	CreatorSupportEnabled     bool `mapstructure:"creator_support_enabled" json:"creator_support_enabled"`
	DesktopDeployEnabled      bool `mapstructure:"desktop_deploy_enabled" json:"desktop_deploy_enabled"`
	ArchiveMalwareScanEnabled bool `mapstructure:"archive_malware_scan_enabled" json:"archive_malware_scan_enabled"`
	RAGHybridEnabled          bool `mapstructure:"rag_hybrid_enabled" json:"rag_hybrid_enabled"`
	// RAGQueryExpansionEnabled and RAGRerankEnabled gate the A-03 retrieval
	// upgrades; defaults stay off until A-04 ablation decides them.
	RAGQueryExpansionEnabled bool `mapstructure:"rag_query_expansion_enabled" json:"rag_query_expansion_enabled"`
	RAGRerankEnabled         bool `mapstructure:"rag_rerank_enabled" json:"rag_rerank_enabled"`
}

type RAGConfig struct {
	Chunking   RAGChunkingConfig   `mapstructure:"chunking" json:"chunking"`
	Index      RAGIndexConfig      `mapstructure:"index" json:"index"`
	Hybrid     RAGHybridConfig     `mapstructure:"hybrid" json:"hybrid"`
	Rerank     RAGRerankConfig     `mapstructure:"rerank" json:"rerank"`
	Refusal    RAGRefusalConfig    `mapstructure:"refusal" json:"refusal"`
	Contextual RAGContextualConfig `mapstructure:"contextual" json:"contextual"`
}

type RAGChunkingConfig struct {
	MaxTokens         int    `mapstructure:"max_tokens" json:"max_tokens"`
	OverlapTokens     int    `mapstructure:"overlap_tokens" json:"overlap_tokens"`
	ChunkingVersion   int    `mapstructure:"version" json:"version"`
	TokenizerEncoding string `mapstructure:"tokenizer_encoding" json:"tokenizer_encoding"`
}

type RAGIndexConfig struct {
	URL                   string `mapstructure:"url" json:"url"`
	GenerationStart       int    `mapstructure:"generation_start" json:"generation_start"`
	EmbeddingModel        string `mapstructure:"embedding_model" json:"embedding_model"`
	HealthPollIntervalSec int    `mapstructure:"health_poll_interval_sec" json:"health_poll_interval_sec"`
	TimeoutSec            int    `mapstructure:"timeout_sec" json:"timeout_sec"`
	AuditTimeoutSec       int    `mapstructure:"audit_timeout_sec" json:"audit_timeout_sec"`
	LockCleanupTimeoutSec int    `mapstructure:"lock_cleanup_timeout_sec" json:"lock_cleanup_timeout_sec"`
	ErrorBodyMaxBytes     int    `mapstructure:"error_body_max_bytes" json:"error_body_max_bytes"`
	ResponseBodyMaxBytes  int    `mapstructure:"response_body_max_bytes" json:"response_body_max_bytes"`
}

type RAGHybridConfig struct {
	BM25TopK   int `mapstructure:"bm25_topk" json:"bm25_topk"`
	VectorTopK int `mapstructure:"vector_topk" json:"vector_topk"`
	RRFK       int `mapstructure:"rrf_k" json:"rrf_k"`
	FinalTopK  int `mapstructure:"final_topk" json:"final_topk"`
	// KeywordSource selects the lexical primary: "postgres" (canonical
	// pg_jieba path, default) or "opensearch" (optional accelerator for
	// full-infra stacks). The other backend, when wired, serves as fallback.
	KeywordSource string `mapstructure:"keyword_source" json:"keyword_source"`
}

// RAGRerankConfig carries the A-03 rerank chain: a primary provider
// (DashScope qwen3-rerank by default) with an optional SiliconFlow fallback.
// API keys are env-injected (RAG_RERANK_API_KEY / RAG_RERANK_FALLBACK_API_KEY
// with DASHSCOPE_API_KEY as the primary-side shared-key fallback) and stay
// out of config files.
type RAGRerankConfig struct {
	Provider         string `mapstructure:"provider" json:"provider"`
	Model            string `mapstructure:"model" json:"model"`
	APIBase          string `mapstructure:"api_base" json:"api_base"`
	APIKey           string `mapstructure:"api_key" json:"-"`
	FallbackProvider string `mapstructure:"fallback_provider" json:"fallback_provider"`
	FallbackModel    string `mapstructure:"fallback_model" json:"fallback_model"`
	FallbackAPIBase  string `mapstructure:"fallback_api_base" json:"fallback_api_base"`
	FallbackAPIKey   string `mapstructure:"fallback_api_key" json:"-"`
	InputTopK        int    `mapstructure:"input_topk" json:"input_topk"`
	TimeoutSec       int    `mapstructure:"timeout_sec" json:"timeout_sec"`
}

// RAGRefusalConfig carries the SP-24 R3 refusal boundary knobs. The citation
// revalidation gate itself is untouched (revalidateCitations semantics never
// change): min_surviving_citations only moves the answer-side boundary of how
// many revalidation-surviving citations a grounded answer needs to keep its
// text (1 = the historical zero-citation no_evidence boundary).
// min_top_relevance_score is the reserved similarity-floor refusal: when > 0,
// a retrieval whose top rerank relevance score sits below the floor returns
// no candidates, which flows into the deterministic no_evidence refusal.
type RAGRefusalConfig struct {
	MinSurvivingCitations int     `mapstructure:"min_surviving_citations" json:"min_surviving_citations"`
	MinTopRelevanceScore  float64 `mapstructure:"min_top_relevance_score" json:"min_top_relevance_score"`
}

// RAGContextualConfig carries the SP-24 R4 contextual-retrieval pilot
// (#575): at ingestion every chunk gets a short LLM-written situation
// prefix prepended to its stored text — the embedding input, the lexical
// index and the citation surface all read that one text column, so a
// single prepend upgrades every retrieval path (Anthropic contextual
// retrieval). Annotation is fail-open: any per-chunk failure keeps the
// original chunk text, and enabled=false (the shipped default) leaves
// ingestion byte-identical. The API key is env-only
// (RAG_CONTEXTUAL_API_KEY, falling back to the SP-20 registry convention
// AGENT_MODEL_DEEPSEEK_API_KEY when the provider is deepseek) and never
// lives in config files.
type RAGContextualConfig struct {
	Enabled           bool   `mapstructure:"enabled" json:"enabled"`
	Provider          string `mapstructure:"provider" json:"provider"`
	Model             string `mapstructure:"model" json:"model"`
	APIBase           string `mapstructure:"api_base" json:"api_base"`
	APIKey            string `mapstructure:"api_key" json:"-"`
	MaxPrefixTokens   int    `mapstructure:"max_prefix_tokens" json:"max_prefix_tokens"`
	DocContextChars   int    `mapstructure:"doc_context_chars" json:"doc_context_chars"`
	Concurrency       int    `mapstructure:"concurrency" json:"concurrency"`
	RequestIntervalMS int    `mapstructure:"request_interval_ms" json:"request_interval_ms"`
	TimeoutSec        int    `mapstructure:"timeout_sec" json:"timeout_sec"`
	MaxRetries        int    `mapstructure:"max_retries" json:"max_retries"`
}

// ArchiveScanConfig carries the archive malware scanning quotas, timeout and
// retry policy (archive malware scanning design §4/§6). All limits are read
// from config, never hardcoded. This ticket (S01) only builds the skeleton;
// the worker (S03) and gates (S04) consume it.
type ArchiveScanConfig struct {
	MaxUploadSizeMB        int    `mapstructure:"max_upload_size_mb" json:"max_upload_size_mb"`
	MaxZipEntries          int    `mapstructure:"max_zip_entries" json:"max_zip_entries"`
	MaxEntryUncompressedMB int    `mapstructure:"max_entry_uncompressed_mb" json:"max_entry_uncompressed_mb"`
	MaxTotalUncompressedMB int    `mapstructure:"max_total_uncompressed_mb" json:"max_total_uncompressed_mb"`
	MaxRecursionDepth      int    `mapstructure:"max_recursion_depth" json:"max_recursion_depth"`
	ScanTimeoutSec         int    `mapstructure:"scan_timeout_sec" json:"scan_timeout_sec"`
	ClamdAddress           string `mapstructure:"clamd_address" json:"clamd_address"`
	RetryBackoffSec        []int  `mapstructure:"retry_backoff_sec" json:"retry_backoff_sec"`
	URLTTLSec              int    `mapstructure:"url_ttl_sec" json:"url_ttl_sec"`
}

type LimitsConfig struct {
	VideoMaxMB      int `mapstructure:"video_max_mb" json:"video_max_mb"`
	VideoMaxSec     int `mapstructure:"video_max_sec" json:"video_max_sec"`
	ImageMaxMB      int `mapstructure:"image_max_mb" json:"image_max_mb"`
	TextMaxMB       int `mapstructure:"text_max_mb" json:"text_max_mb"`
	ModMaxMB        int `mapstructure:"mod_max_mb" json:"mod_max_mb"`
	SheetMusicMaxMB int `mapstructure:"sheet_music_max_mb" json:"sheet_music_max_mb"`
	// DMMaxLength caps a direct-message text in runes; must stay aligned with
	// the frontend MAX_DM_LENGTH (2000) so the UI constraint is server-enforced.
	DMMaxLength int `mapstructure:"dm_max_length" json:"dm_max_length"`
}

type ReputationConfig struct {
	QualityContentThreshold     int `mapstructure:"quality_content_threshold" json:"quality_content_threshold"`
	QualityCommentThreshold     int `mapstructure:"quality_comment_threshold" json:"quality_comment_threshold"`
	MinScoreForInteraction      int `mapstructure:"min_score_for_interaction" json:"min_score_for_interaction"`
	RepeatViolationWindowDays   int `mapstructure:"repeat_violation_window_days" json:"repeat_violation_window_days"`
	RepeatViolationThreshold    int `mapstructure:"repeat_violation_threshold" json:"repeat_violation_threshold"`
	RepeatViolationExtraPenalty int `mapstructure:"repeat_violation_extra_penalty" json:"repeat_violation_extra_penalty"`

	// Score values (positive = award, negative = penalty).
	// Zero means "use the hardcoded default in reputation_service.go".
	ScoreQualityContent     int `mapstructure:"score_quality_content" json:"score_quality_content"`
	ScorePRMerged           int `mapstructure:"score_pr_merged" json:"score_pr_merged"`
	ScoreQualityComment     int `mapstructure:"score_quality_comment" json:"score_quality_comment"`
	ScoreTagRecognized      int `mapstructure:"score_tag_recognized" json:"score_tag_recognized"`
	ScoreJudgeAccuracy      int `mapstructure:"score_judge_accuracy" json:"score_judge_accuracy"`
	ScoreRehabCourse        int `mapstructure:"score_rehab_course" json:"score_rehab_course"`
	ScoreValidReport        int `mapstructure:"score_valid_report" json:"score_valid_report"`
	ScoreMaliciousContent   int `mapstructure:"score_malicious_content" json:"score_malicious_content"`
	ScoreMaliciousPR        int `mapstructure:"score_malicious_pr" json:"score_malicious_pr"`
	ScoreMaliciousComment   int `mapstructure:"score_malicious_comment" json:"score_malicious_comment"`
	ScoreMaliciousReport    int `mapstructure:"score_malicious_report" json:"score_malicious_report"`
	ScoreMaliciousTagReport int `mapstructure:"score_malicious_tag_report" json:"score_malicious_tag_report"`
	ScoreJudgeError         int `mapstructure:"score_judge_error" json:"score_judge_error"`
}

type JudgeConfig struct {
	MinVotesRequired int     `mapstructure:"min_votes_required" json:"min_votes_required"`
	PassThreshold    float64 `mapstructure:"pass_threshold" json:"pass_threshold"`
	ExamPassRate     float64 `mapstructure:"exam_pass_rate" json:"exam_pass_rate"`
	ErrorRateRevoke  float64 `mapstructure:"error_rate_revoke" json:"error_rate_revoke"`
	ErrorRateWindow  int     `mapstructure:"error_rate_window" json:"error_rate_window"`
}

// IPProposalConfig carries the collaborative-governance proposal thresholds
// (#290). Deliberately independent from JudgeConfig: proposals govern IP
// profile edits by followers, judges arbitrate content violations.
type IPProposalConfig struct {
	MinVotes      int     `mapstructure:"min_votes" json:"min_votes"`
	PassThreshold float64 `mapstructure:"pass_threshold" json:"pass_threshold"`
	DeadlineDays  int     `mapstructure:"deadline_days" json:"deadline_days"`
}

// DiscussionConfig carries discussion-list tuning. HotDecayHours feeds the
// reply-count/age decay expression for the "hot" sort (#290).
type DiscussionConfig struct {
	HotDecayHours float64 `mapstructure:"hot_decay_hours" json:"hot_decay_hours"`
}

func (c IPProposalConfig) EffectiveMinVotes() int {
	if c.MinVotes <= 0 {
		return 10
	}
	return c.MinVotes
}

func (c IPProposalConfig) EffectivePassThreshold() float64 {
	if c.PassThreshold <= 0 || c.PassThreshold > 1 {
		return 0.6
	}
	return c.PassThreshold
}

func (c IPProposalConfig) EffectiveDeadlineDays() int {
	if c.DeadlineDays <= 0 {
		return 7
	}
	return c.DeadlineDays
}

func (c DiscussionConfig) EffectiveHotDecayHours() float64 {
	if c.HotDecayHours <= 0 {
		return 72
	}
	return c.HotDecayHours
}

type SocialConfig struct {
	ReportAutoHideRate   float64 `mapstructure:"report_auto_hide_rate" json:"report_auto_hide_rate"`
	CommentFoldThreshold float64 `mapstructure:"comment_fold_threshold" json:"comment_fold_threshold"`
}

// CollaborationConfig carries server-only collaboration invite limits.
// Only MaxInviteesPerPublish is exposed through the public config endpoint;
// daily limits, expiry and contributor capacity must never reach the client.
type CollaborationConfig struct {
	InviteDailyLimit       int `mapstructure:"invite_daily_limit" json:"invite_daily_limit"`
	InviteExpireDays       int `mapstructure:"invite_expire_days" json:"invite_expire_days"`
	MaxInviteesPerPublish  int `mapstructure:"max_invitees_per_publish" json:"max_invitees_per_publish"`
	MaxContributorsPerItem int `mapstructure:"max_contributors_per_item" json:"max_contributors_per_item"`
}

type BrowseHistoryConfig struct {
	RetentionDays int    `mapstructure:"retention_days" json:"retention_days"`
	CleanupTime   string `mapstructure:"cleanup_time" json:"cleanup_time"`
}

type UploadConfig struct {
	SheetMusicExtensions []string `mapstructure:"sheet_music_extensions" json:"sheet_music_extensions"`
	// Media set (media gallery) size bounds for newly published image/video
	// content. Zero means "use the specification default" so tests and
	// minimal configs keep working.
	ImageGalleryMinItems int `mapstructure:"image_gallery_min_items" json:"image_gallery_min_items"`
	ImageGalleryMaxItems int `mapstructure:"image_gallery_max_items" json:"image_gallery_max_items"`
	VideoGalleryMinItems int `mapstructure:"video_gallery_min_items" json:"video_gallery_min_items"`
	VideoGalleryMaxItems int `mapstructure:"video_gallery_max_items" json:"video_gallery_max_items"`
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
	RequireReview     bool     `mapstructure:"require_review" json:"require_review"`
	MaxDailyPosts     int      `mapstructure:"max_daily_posts" json:"max_daily_posts"`
	FreezeOnViolation bool     `mapstructure:"freeze_on_violation" json:"freeze_on_violation"`
	TypeOrderOriginal []string `mapstructure:"type_order_original" json:"type_order_original"`
	TypeOrderFanwork  []string `mapstructure:"type_order_fanwork" json:"type_order_fanwork"`
}

type AgentConfig struct {
	WebAgentEnabled bool   `mapstructure:"web_agent_enabled" json:"web_agent_enabled"`
	LLMProvider     string `mapstructure:"llm_provider" json:"llm_provider"`
	LLMModel        string `mapstructure:"llm_model" json:"llm_model"`
	LLMAPIBase      string `mapstructure:"llm_api_base" json:"llm_api_base"`
	LLMAPIKey       string `mapstructure:"llm_api_key" json:"-"`
	EmbeddingModel  string `mapstructure:"embedding_model" json:"embedding_model"`
	// EmbeddingProvider routes embeddings to a different adapter than chat
	// (canonical profile: minimax chat + openai_compat DashScope embeddings).
	// Empty means "follow llm_provider" (single-provider wiring).
	EmbeddingProvider     string `mapstructure:"embedding_provider" json:"embedding_provider"`
	EmbeddingAPIBase      string `mapstructure:"embedding_api_base" json:"embedding_api_base"`
	EmbeddingGroupID      string `mapstructure:"embedding_group_id" json:"-"`
	EmbeddingAPIKey       string `mapstructure:"embedding_api_key" json:"-"`
	EmbeddingDimensions   int    `mapstructure:"embedding_dimensions" json:"embedding_dimensions"`
	RateLimitPerDay       int    `mapstructure:"rate_limit_per_day" json:"rate_limit_per_day"`
	RateLimitPerMinute    int    `mapstructure:"rate_limit_per_minute" json:"rate_limit_per_minute"`
	MaxToolCallsPerTurn   int    `mapstructure:"max_tool_calls_per_turn" json:"max_tool_calls_per_turn"`
	MaxOutputTokens       int    `mapstructure:"max_output_tokens" json:"max_output_tokens"`
	ProviderTimeoutSec    int    `mapstructure:"provider_timeout_sec" json:"provider_timeout_sec"`
	ProviderMaxRetries    int    `mapstructure:"provider_max_retries" json:"provider_max_retries"`
	CitationMaxCount      int    `mapstructure:"citation_max_count" json:"citation_max_count"`
	UploadAssistMaxFileMB int    `mapstructure:"upload_assist_max_file_mb" json:"upload_assist_max_file_mb"`
	HMACSecret            string `mapstructure:"hmac_secret" json:"-"`
	MaxUserMessageChars   int    `mapstructure:"max_user_message_chars" json:"max_user_message_chars"`
	ChatMaxContextMsgs    int    `mapstructure:"chat_max_context_messages" json:"chat_max_context_messages"`
	ConversationListLimit int    `mapstructure:"conversation_list_limit" json:"conversation_list_limit"`
	ConversationPageSize  int    `mapstructure:"conversation_page_size" json:"conversation_page_size"`
	// ChatContextTokenBudget caps the server-side assembled conversation
	// history (estimated tokens; CJK-heavy so rune count is a safe upper
	// bound). The system prompt is always included outside this budget.
	ChatContextTokenBudget int `mapstructure:"chat_context_token_budget" json:"chat_context_token_budget"`
	// ChitchatShortcutEnabled gates the rule-layer chitchat shortcut (SP-15
	// A1): an exact-match greeting consumes no LLM call and replays a
	// server-owned template with answer_kind=conversational.
	ChitchatShortcutEnabled bool `mapstructure:"chitchat_shortcut_enabled" json:"chitchat_shortcut_enabled"`
	// ChitchatPatterns is the exact-match keyword table for the shortcut.
	// Matching is trim + full/half-width fold + case fold, ≤8 runes, whole
	// message equal to one pattern (no substring matches).
	ChitchatPatterns []string `mapstructure:"chitchat_patterns" json:"chitchat_patterns"`
	// ConversationalMaxRunes is the deterministic guardrail for the
	// model-routed conversational lane (SP-15 A2): a zero-tool, zero-citation
	// reply is kept only while it stays within this many runes; anything
	// longer falls back to no_evidence. Zero disables the lane entirely
	// (fail-closed: a missing key can never loosen the citation guard).
	ConversationalMaxRunes int `mapstructure:"conversational_max_runes" json:"conversational_max_runes"`
	// Models is the SP-20 (#545) incremental model supply registry. The
	// primary stays on the single llm_provider wiring above (zero migration);
	// each entry with an empty api_key is silently unregistered (not
	// selectable, never in the routing chain).
	Models []AgentModelConfig `mapstructure:"models" json:"models"`
	// Routing drives the SP-20 chain: primary empty = the llm_provider
	// synthetic entry goes first; fallbacks list registered model ids in
	// order; retry_on accepts provider_error / blank_answer.
	Routing AgentRoutingConfig `mapstructure:"routing" json:"routing"`
	// Guardrails carries the SP-23 M4 five-piece protections that are
	// server-side: untrusted-tool-result fencing, image-URL domain
	// allowlist, and the conversation-level budgets.
	Guardrails AgentGuardrailsConfig `mapstructure:"guardrails" json:"guardrails"`
	// MCP drives the external-tool bridge (SP-23 M3, #568): configured
	// servers are exposed to the agent as mcp_<server>_<tool> tools over
	// stdio subprocess transports. Default off (gray rollout per server).
	MCP AgentMCPConfig `mapstructure:"mcp" json:"mcp"`
	// Image drives the generate_image tool (SP-23 M1, #566): a CogView-4
	// OpenAI-compatible images endpoint whose results are re-uploaded to the
	// platform's own OSS bucket (external image URLs never surface), plus a
	// per-conversation generation budget. enabled=false or a missing
	// AGENT_IMAGE_API_KEY keeps the tool out of the model's tool list.
	Image AgentImageConfig `mapstructure:"image" json:"image"`
}

// AgentGuardrailsConfig holds every M4 limit (config-driven, Key Rule 6).
type AgentGuardrailsConfig struct {
	// FenceExternalToolResults wraps MCP/image tool results in explicit
	// "data, not instructions" boundary markers before they re-enter the
	// model conversation (OWASP LLM01 / tool-poisoning mitigation).
	FenceExternalToolResults bool `mapstructure:"fence_external_tool_results" json:"fence_external_tool_results"`
	// ImageURLAllowHosts is the image-URL allowlist for model output: any
	// image-looking URL outside these hosts (and the OSS signing host) is
	// replaced by a placeholder (M4 piece 2).
	ImageURLAllowHosts []string `mapstructure:"image_url_allow_hosts" json:"image_url_allow_hosts"`
	// SessionToolCallLimit caps total tool calls per conversation across
	// turns (persisted #538 steps + live turn); exceeding returns a stable
	// degradation code instead of executing (M4 piece 3).
	SessionToolCallLimit int `mapstructure:"session_tool_call_limit" json:"session_tool_call_limit"`
	// SessionToolTurnLimit caps how many turns of one conversation may run
	// tools at all — the conversation-level hard roof above the per-turn
	// tool_limit (M4 piece 4).
	SessionToolTurnLimit int `mapstructure:"session_tool_turn_limit" json:"session_tool_turn_limit"`
}

// AgentMCPConfig carries the MCP client bridge switches: every limit is
// config-driven; servers launch lazily as stdio subprocesses.
type AgentMCPConfig struct {
	Enabled        bool `mapstructure:"enabled" json:"enabled"`
	CallTimeoutSec int  `mapstructure:"call_timeout_sec" json:"call_timeout_sec"`
	ResultMaxBytes int  `mapstructure:"result_max_bytes" json:"result_max_bytes"`
	// ExternalAnswerMaxRunes is the conversational-lane guardrail for turns
	// whose only tools were external ones (MCP / image generation): their
	// answers cite workspace data rather than RAG chunks, so the strict
	// citation gate would otherwise clear every substantive answer to
	// no_evidence. Zero disables the lane.
	ExternalAnswerMaxRunes int                    `mapstructure:"external_answer_max_runes" json:"external_answer_max_runes"`
	Servers                []AgentMCPServerConfig `mapstructure:"servers" json:"servers"`
}

// AgentMCPServerConfig is one stdio MCP server subprocess.
type AgentMCPServerConfig struct {
	ID      string            `mapstructure:"id" json:"id"`
	Command string            `mapstructure:"command" json:"command"`
	Args    []string          `mapstructure:"args" json:"args"`
	Env     map[string]string `mapstructure:"env" json:"env"`
	// Tools is an optional allowlist; empty exposes every advertised tool.
	Tools []string `mapstructure:"tools" json:"tools"`
}

// AgentImageConfig carries every generate_image limit (Key Rule 6: limits
// come from config, never code). APIKey is env-only (AGENT_IMAGE_API_KEY).
type AgentImageConfig struct {
	Enabled           bool     `mapstructure:"enabled" json:"enabled"`
	Provider          string   `mapstructure:"provider" json:"provider"`
	Model             string   `mapstructure:"model" json:"model"`
	APIBase           string   `mapstructure:"api_base" json:"api_base"`
	APIKey            string   `mapstructure:"api_key" json:"-"`
	SizeDefault       string   `mapstructure:"size_default" json:"size_default"`
	SizeOptions       []string `mapstructure:"size_options" json:"size_options"`
	MaxImageBytes     int      `mapstructure:"max_image_bytes" json:"max_image_bytes"`
	TimeoutSec        int      `mapstructure:"timeout_sec" json:"timeout_sec"`
	SessionImageLimit int      `mapstructure:"session_image_limit" json:"session_image_limit"`
	// PricePerImageCNY feeds the trace cost estimate (SP-21 T7 rate-table
	// linkage): cost = generated images x this flat rate.
	PricePerImageCNY float64 `mapstructure:"price_per_image_cny" json:"price_per_image_cny"`
}

// ImageConfigured reports whether the tool may be offered to the model: the
// switch is on AND an endpoint key exists. Fail-closed — a missing key can
// never surface a half-working tool.
func (a AgentImageConfig) ImageConfigured() bool {
	return a.Enabled && strings.TrimSpace(a.APIKey) != ""
}

// AgentModelConfig is one incremental chat model registry entry.
type AgentModelConfig struct {
	ID          string `mapstructure:"id" json:"id"`
	Provider    string `mapstructure:"provider" json:"provider"`
	Model       string `mapstructure:"model" json:"model"`
	APIBase     string `mapstructure:"api_base" json:"api_base"`
	APIKey      string `mapstructure:"api_key" json:"-"`
	DisplayName string `mapstructure:"display_name" json:"display_name"`
	// CostInPerMTokens / CostOutPerMTokens are CNY per 1M tokens for the
	// cost ledger (SP-21 T7). Zero = unknown rate: the model's tokens are
	// still aggregated but never priced ("unknown 不估算").
	CostInPerMTokens  float64 `mapstructure:"cost_in_per_m_tokens" json:"cost_in_per_m_tokens"`
	CostOutPerMTokens float64 `mapstructure:"cost_out_per_m_tokens" json:"cost_out_per_m_tokens"`
}

// AgentRoutingConfig is the SP-20 routing chain configuration.
type AgentRoutingConfig struct {
	Primary   string   `mapstructure:"primary" json:"primary"`
	Fallbacks []string `mapstructure:"fallbacks" json:"fallbacks"`
	RetryOn   []string `mapstructure:"retry_on" json:"retry_on"`
}

// modelEnvKey normalizes a model id for its credential env var suffix.
func modelEnvKey(id string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(id) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

type CaptchaConfig struct {
	Provider        string `mapstructure:"provider" json:"provider"`
	Prefix          string `mapstructure:"prefix" json:"prefix"`
	SceneID         string `mapstructure:"scene_id" json:"scene_id"`
	Region          string `mapstructure:"region" json:"region"`
	AccessKeyID     string `mapstructure:"access_key_id" json:"-"`
	AccessKeySecret string `mapstructure:"access_key_secret" json:"-"`
	TicketTTLSec    int    `mapstructure:"ticket_ttl_sec" json:"ticket_ttl_sec"`
}

type ClientConfig struct {
	DownloadEnabled bool   `mapstructure:"download_enabled" json:"download_enabled"`
	DownloadURL     string `mapstructure:"download_url" json:"download_url"`
	LatestVersion   string `mapstructure:"latest_version" json:"latest_version"`
}

type FeedbackConfig struct {
	UploadGrantTTLSec int `mapstructure:"upload_grant_ttl_sec" json:"upload_grant_ttl_sec"`
}

type WebConfig struct {
	PublicBaseURL string `mapstructure:"public_base_url" json:"public_base_url"`
}

type SMTPConfig struct {
	Mode        string `mapstructure:"mode" json:"mode"`
	Host        string `mapstructure:"host" json:"host"`
	Port        int    `mapstructure:"port" json:"port"`
	User        string `mapstructure:"user" json:"user"`
	Password    string `mapstructure:"password" json:"-"`
	FromAddress string `mapstructure:"from_address" json:"from_address"`
}

type VerificationConfig struct {
	EmailTTLSec           int `mapstructure:"email_ttl_sec" json:"email_ttl_sec"`
	ResetTTLSec           int `mapstructure:"reset_ttl_sec" json:"reset_ttl_sec"`
	ResendCooldownSec     int `mapstructure:"resend_cooldown_sec" json:"resend_cooldown_sec"`
	LoginCaptchaThreshold int `mapstructure:"login_captcha_threshold" json:"login_captcha_threshold"`
	PasswordMinLength     int `mapstructure:"password_min_length" json:"password_min_length"`
	RegisterPendingTTLSec int `mapstructure:"register_pending_ttl_sec" json:"register_pending_ttl_sec"`
}

type LegalConfig struct {
	CurrentTermsVersion   string `mapstructure:"current_terms_version" json:"current_terms_version"`
	CurrentPrivacyVersion string `mapstructure:"current_privacy_version" json:"current_privacy_version"`
}

type CacheConfig struct {
	ContentListTTL         int `mapstructure:"content_list_ttl" json:"content_list_ttl"`
	ContentDetailTTL       int `mapstructure:"content_detail_ttl" json:"content_detail_ttl"`
	IPListTTL              int `mapstructure:"ip_list_ttl" json:"ip_list_ttl"`
	IPDetailTTL            int `mapstructure:"ip_detail_ttl" json:"ip_detail_ttl"`
	ViewCountFlushInterval int `mapstructure:"view_count_flush_interval" json:"view_count_flush_interval"`
	HotRankZSetTTL         int `mapstructure:"hot_rank_zset_ttl" json:"hot_rank_zset_ttl"`
	UserStatusTTL          int `mapstructure:"user_status_ttl" json:"user_status_ttl"`
	TagCacheTTL            int `mapstructure:"tag_cache_ttl" json:"tag_cache_ttl"`
	EmailVerifyTTL         int `mapstructure:"email_verify_ttl" json:"email_verify_ttl"`
	PasswordResetTTL       int `mapstructure:"password_reset_ttl" json:"password_reset_ttl"`
	PublishFreezeTTL       int `mapstructure:"publish_freeze_ttl" json:"publish_freeze_ttl"`
}

type RateLimitConfig struct {
	Enabled              bool  `mapstructure:"enabled" json:"enabled"`
	NormalPerMinute      int   `mapstructure:"normal_per_minute" json:"normal_per_minute"`
	UploadPerHour        int   `mapstructure:"upload_per_hour" json:"upload_per_hour"`
	NormalWindowSec      int   `mapstructure:"normal_window_sec" json:"normal_window_sec"`
	UploadWindowSec      int   `mapstructure:"upload_window_sec" json:"upload_window_sec"`
	AgentWindowSec       int   `mapstructure:"agent_window_sec" json:"agent_window_sec"`
	AgentMinuteWindowSec int   `mapstructure:"agent_minute_window_sec" json:"agent_minute_window_sec"`
	CredentialPerMinute  int   `mapstructure:"credential_per_minute" json:"credential_per_minute"`
	SearchPerMinute      int   `mapstructure:"search_per_minute" json:"search_per_minute"`
	PATPerMinute         int   `mapstructure:"pat_per_minute" json:"pat_per_minute"`
	PATWindowSec         int   `mapstructure:"pat_window_sec" json:"pat_window_sec"`
	MCPPerMinute         int   `mapstructure:"mcp_per_minute" json:"mcp_per_minute"`
	AICallbackPerMinute  int   `mapstructure:"ai_callback_per_minute" json:"ai_callback_per_minute"`
	MaxJSONBodyBytes     int64 `mapstructure:"max_json_body_bytes" json:"max_json_body_bytes"`
	MaxQueryChars        int   `mapstructure:"max_query_chars" json:"max_query_chars"`
	MaxSearchLimit       int   `mapstructure:"max_search_limit" json:"max_search_limit"`
	MaxSearchPage        int   `mapstructure:"max_search_page" json:"max_search_page"`
}

type RecommendationConfig struct {
	Enabled                      bool    `mapstructure:"enabled" json:"enabled"`
	HotDecayHours                float64 `mapstructure:"hot_decay_hours" json:"hot_decay_hours"`
	PersonalizationWeight        float64 `mapstructure:"personalization_weight" json:"personalization_weight"`
	MinInteractionForPersonalize int     `mapstructure:"min_interaction_for_personalize" json:"min_interaction_for_personalize"`
	EmbeddingTopk                int     `mapstructure:"embedding_topk" json:"embedding_topk"`
	TrendingWindowDays           int     `mapstructure:"trending_window_days" json:"trending_window_days"`
	RefreshIntervalH             int     `mapstructure:"refresh_interval_h" json:"refresh_interval_h"`
	RankIntervalMin              int     `mapstructure:"rank_interval_min" json:"rank_interval_min"`
	EmbeddingMultiplier          int     `mapstructure:"embedding_multiplier" json:"embedding_multiplier"`
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
	// Commands are commonly started from backend/, while the canonical local
	// env file lives at the repository root. Prefer that file when config.yaml
	// is present in the current directory so a stale backend/.env cannot shadow
	// the user's current credentials. Explicitly exported variables still win.
	candidates := []string{".env", filepath.Join("..", ".env")}
	if _, err := os.Stat("config.yaml"); err == nil {
		candidates = []string{filepath.Join("..", ".env"), ".env"}
	}
	for _, candidate := range candidates {
		if err := godotenv.Load(candidate); err != nil {
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
	// SP-20 (#545): per-entry model credentials follow the
	// AGENT_MODEL_<ID>_API_KEY convention (id upper-cased, non-alnum → _).
	for i := range cfg.Agent.Models {
		if cfg.Agent.Models[i].ID == "" {
			continue
		}
		envKey := "AGENT_MODEL_" + modelEnvKey(cfg.Agent.Models[i].ID) + "_API_KEY"
		if v := os.Getenv(envKey); v != "" {
			cfg.Agent.Models[i].APIKey = v
		}
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
		// Hybrid RAG stores the embedding model in both agent and index
		// configuration. Keep the common single-variable setup coherent unless
		// the caller explicitly supplies the index value below.
		if strings.TrimSpace(os.Getenv("RAG_INDEX_EMBEDDING_MODEL")) == "" {
			cfg.RAG.Index.EmbeddingModel = v
		}
	}
	if v := os.Getenv("AGENT_EMBEDDING_PROVIDER"); v != "" {
		cfg.Agent.EmbeddingProvider = v
	}
	if v := os.Getenv("AGENT_EMBEDDING_API_BASE"); v != "" {
		cfg.Agent.EmbeddingAPIBase = v
	}
	if v := os.Getenv("AGENT_EMBEDDING_API_KEY"); v != "" {
		cfg.Agent.EmbeddingAPIKey = v
	}
	if v := os.Getenv("AGENT_IMAGE_API_KEY"); v != "" {
		cfg.Agent.Image.APIKey = v
	}
	if v := os.Getenv("AGENT_IMAGE_API_BASE"); v != "" {
		cfg.Agent.Image.APIBase = v
	}
	if v := os.Getenv("AGENT_IMAGE_MODEL"); v != "" {
		cfg.Agent.Image.Model = v
	}
	if v := os.Getenv("AGENT_EMBEDDING_GROUP_ID"); v != "" {
		cfg.Agent.EmbeddingGroupID = v
	}
	if v := os.Getenv("RAG_INDEX_EMBEDDING_MODEL"); v != "" {
		cfg.RAG.Index.EmbeddingModel = v
	}
	if v := os.Getenv("RAG_RERANK_API_KEY"); v != "" {
		cfg.RAG.Rerank.APIKey = v
	} else if v := os.Getenv("DASHSCOPE_API_KEY"); v != "" {
		// Shared-key fallback: rerank defaults to the DashScope provider, so
		// the Bailian master key from the root .env applies unless a dedicated
		// rerank key is configured.
		cfg.RAG.Rerank.APIKey = v
	}
	if v := os.Getenv("RAG_RERANK_FALLBACK_API_KEY"); v != "" {
		cfg.RAG.Rerank.FallbackAPIKey = v
	}
	if v := os.Getenv("RAG_RERANK_API_BASE"); v != "" {
		cfg.RAG.Rerank.APIBase = v
	}
	if v := os.Getenv("RAG_RERANK_FALLBACK_API_BASE"); v != "" {
		cfg.RAG.Rerank.FallbackAPIBase = v
	}
	// SP-24 R4 contextual annotation key: dedicated var first, then the
	// model-registry convention so an existing deepseek deployment needs no
	// new secret plumbing.
	if v := os.Getenv("RAG_CONTEXTUAL_API_KEY"); v != "" {
		cfg.RAG.Contextual.APIKey = v
	} else if v := os.Getenv("AGENT_MODEL_DEEPSEEK_API_KEY"); v != "" && strings.EqualFold(strings.TrimSpace(cfg.RAG.Contextual.Provider), "deepseek") {
		cfg.RAG.Contextual.APIKey = v
	}
	// SP-24 R3 refusal boundary normalization: an omitted or non-positive
	// min_surviving_citations means the shipped default (the historical
	// zero-citation no_evidence boundary), so configs predating the knob —
	// and override files that only pin other axes — stay valid. Release
	// validation still rejects negative values explicitly.
	if cfg.RAG.Refusal.MinSurvivingCitations < 1 {
		cfg.RAG.Refusal.MinSurvivingCitations = 1
	}
	if v := os.Getenv("RAG_INDEX_URL"); v != "" {
		cfg.RAG.Index.URL = v
	}
	if v := os.Getenv("RAG_HYBRID_FINAL_TOPK"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RAG.Hybrid.FinalTopK = n
		}
	}
	if v := os.Getenv("CLAMD_ADDRESS"); v != "" {
		cfg.ArchiveScan.ClamdAddress = v
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
	if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		cfg.Observability.Tracing.Endpoint = v
	}
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		cfg.Observability.Tracing.ServiceName = v
		cfg.Worker.ServiceName = v
	}
	if v := os.Getenv("OBSERVABILITY_TRACING_ENABLED"); v != "" {
		cfg.Observability.Tracing.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); v != "" {
		if ratio, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Observability.Tracing.SampleRatio = ratio
		}
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
	if seed := strings.TrimSpace(c.Green.Seed); seed != "" && seed == greenSeedFactoryTemplate {
		errs = append(errs, "green.seed must not be the factory template value in release mode; generate a deploy-time value (see the runbook secret-generation step)")
	}
	if seed := strings.TrimSpace(c.Green.Seed); seed != "" && !greenSeedFormat.MatchString(seed) {
		errs = append(errs, "green.seed must be 1-64 characters of [A-Za-z0-9_] in release mode")
	}
	if uid := strings.TrimSpace(c.Green.UID); uid != "" && !greenUIDFormat.MatchString(uid) {
		errs = append(errs, "green.uid must be the numeric Aliyun main account UID (digits only) in release mode")
	}

	switch c.Captcha.Provider {
	case "aliyun_v2":
		// the only production-capable provider
	case "bypass", "":
		errs = append(errs, "captcha.provider must not be 'bypass' in release mode; use 'aliyun_v2'")
	default:
		// SP-25 低-29：拼错的 provider 此前过 release 校验、运行时静默回退
		// bypass（fail-open）——release 模式直接硬失败。
		errs = append(errs, "captcha.provider must be 'aliyun_v2' in release mode (unknown provider: "+c.Captcha.Provider+")")
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
	if c.Features.ArchiveMalwareScanEnabled {
		if c.ArchiveScan.MaxUploadSizeMB <= 0 {
			errs = append(errs, "archive_scan.max_upload_size_mb must be positive when archive malware scanning is enabled")
		}
		if c.ArchiveScan.MaxZipEntries <= 0 {
			errs = append(errs, "archive_scan.max_zip_entries must be positive when archive malware scanning is enabled")
		}
		if c.ArchiveScan.MaxEntryUncompressedMB <= 0 {
			errs = append(errs, "archive_scan.max_entry_uncompressed_mb must be positive when archive malware scanning is enabled")
		}
		if c.ArchiveScan.MaxTotalUncompressedMB <= 0 {
			errs = append(errs, "archive_scan.max_total_uncompressed_mb must be positive when archive malware scanning is enabled")
		}
		if c.ArchiveScan.MaxRecursionDepth <= 0 {
			errs = append(errs, "archive_scan.max_recursion_depth must be positive when archive malware scanning is enabled")
		}
		if c.ArchiveScan.ScanTimeoutSec <= 0 {
			errs = append(errs, "archive_scan.scan_timeout_sec must be positive when archive malware scanning is enabled")
		}
		if strings.TrimSpace(c.ArchiveScan.ClamdAddress) == "" {
			errs = append(errs, "archive_scan.clamd_address must be configured when archive malware scanning is enabled")
		}
		if len(c.ArchiveScan.RetryBackoffSec) == 0 {
			errs = append(errs, "archive_scan.retry_backoff_sec must not be empty when archive malware scanning is enabled")
		}
		if c.ArchiveScan.URLTTLSec <= 0 {
			errs = append(errs, "archive_scan.url_ttl_sec must be positive when archive malware scanning is enabled")
		}
	}
	if c.Features.RAGHybridEnabled {
		if c.RAG.Chunking.MaxTokens <= 0 {
			errs = append(errs, "rag.chunking.max_tokens must be positive when RAG hybrid search is enabled")
		}
		if c.RAG.Chunking.OverlapTokens < 0 || c.RAG.Chunking.OverlapTokens >= c.RAG.Chunking.MaxTokens {
			errs = append(errs, "rag.chunking.overlap_tokens must be non-negative and less than max_tokens when RAG hybrid search is enabled")
		}
		if c.RAG.Chunking.ChunkingVersion <= 0 {
			errs = append(errs, "rag.chunking.version must be positive when RAG hybrid search is enabled")
		}
		if c.RAG.Chunking.TokenizerEncoding != "cl100k_base" {
			errs = append(errs, "rag.chunking.tokenizer_encoding must be cl100k_base when RAG hybrid search is enabled")
		}
		indexURL, err := url.Parse(strings.TrimSpace(c.RAG.Index.URL))
		if err != nil || indexURL.Host == "" || (indexURL.Scheme != "http" && indexURL.Scheme != "https") {
			errs = append(errs, "rag.index.url must be an absolute http(s) URL when RAG hybrid search is enabled")
		}
		requireNonEmpty(&errs, "rag.index.embedding_model", c.RAG.Index.EmbeddingModel)
		if c.RAG.Index.EmbeddingModel != c.Agent.EmbeddingModel {
			errs = append(errs, "rag.index.embedding_model must match agent.embedding_model when RAG hybrid search is enabled")
		}
		requirePositiveInt(&errs, "rag.index.generation_start", c.RAG.Index.GenerationStart)
		if c.Agent.EmbeddingDimensions != RAGEmbeddingDimensions {
			errs = append(errs, fmt.Sprintf("agent.embedding_dimensions must be %d when RAG hybrid search is enabled", RAGEmbeddingDimensions))
		}
		requirePositiveInt(&errs, "rag.index.health_poll_interval_sec", c.RAG.Index.HealthPollIntervalSec)
		requirePositiveInt(&errs, "rag.index.timeout_sec", c.RAG.Index.TimeoutSec)
		requirePositiveInt(&errs, "rag.index.audit_timeout_sec", c.RAG.Index.AuditTimeoutSec)
		requirePositiveInt(&errs, "rag.index.lock_cleanup_timeout_sec", c.RAG.Index.LockCleanupTimeoutSec)
		requirePositiveInt(&errs, "rag.index.error_body_max_bytes", c.RAG.Index.ErrorBodyMaxBytes)
		requirePositiveInt(&errs, "rag.index.response_body_max_bytes", c.RAG.Index.ResponseBodyMaxBytes)
		requirePositiveInt(&errs, "rag.hybrid.bm25_topk", c.RAG.Hybrid.BM25TopK)
		requirePositiveInt(&errs, "rag.hybrid.vector_topk", c.RAG.Hybrid.VectorTopK)
		requirePositiveInt(&errs, "rag.hybrid.rrf_k", c.RAG.Hybrid.RRFK)
		requirePositiveInt(&errs, "rag.hybrid.final_topk", c.RAG.Hybrid.FinalTopK)
		switch strings.ToLower(strings.TrimSpace(c.RAG.Hybrid.KeywordSource)) {
		case "", "postgres", "opensearch":
		default:
			errs = append(errs, "rag.hybrid.keyword_source must be postgres or opensearch")
		}
	}

	// SP-24 R3 refusal boundary knobs: the citation-count boundary is an
	// answer-classification knob (applies regardless of hybrid), the
	// similarity floor only has meaning on the rerank path. A non-positive
	// min_surviving_citations is normalized to 1 at load (configs predating
	// the knob stay valid), so validation only rejects explicit negatives.
	if c.Resilience.Breaker.FailureThreshold < 0 || c.Resilience.Breaker.OpenTimeoutSec < 0 {
		errs = append(errs, "resilience.breaker values must not be negative (0 falls back to the 2-failure/30s defaults)")
	}
	// SP-24 R6 aux-call caches: an enabled cache needs a positive TTL; the
	// shipped TTL defaults are the recommended first-on values.
	if item := c.Resilience.AuxCache.Title; item.Enabled && item.TTLSec <= 0 {
		errs = append(errs, "resilience.aux_cache.title.ttl_sec must be positive when the cache is enabled")
	}
	if item := c.Resilience.AuxCache.Expander; item.Enabled && item.TTLSec <= 0 {
		errs = append(errs, "resilience.aux_cache.expander.ttl_sec must be positive when the cache is enabled")
	}
	// SP-24 R6 per-provider chat concurrency: an enabled throttle needs a
	// usable slot count.
	if conc := c.Resilience.LLMConcurrency; conc.Enabled && conc.MaxPerProvider <= 0 {
		errs = append(errs, "resilience.llm_concurrency.max_per_provider must be >= 1 when the throttle is enabled")
	}
	if c.RAG.Refusal.MinSurvivingCitations < 0 {
		errs = append(errs, "rag.refusal.min_surviving_citations must not be negative")
	}
	if c.RAG.Refusal.MinTopRelevanceScore < 0 || c.RAG.Refusal.MinTopRelevanceScore > 1 {
		errs = append(errs, "rag.refusal.min_top_relevance_score must be within [0,1] (0 disables the similarity floor)")
	}
	if c.RAG.Refusal.MinTopRelevanceScore > 0 && !(c.Features.RAGHybridEnabled && c.Features.RAGRerankEnabled) {
		errs = append(errs, "rag.refusal.min_top_relevance_score requires features.rag_hybrid_enabled and features.rag_rerank_enabled (relevance scores exist only on the rerank path)")
	}
	// SP-24 R4 contextual retrieval: annotation rides the projection (hybrid
	// ingestion), so the switch requires hybrid and a fully specified
	// annotation provider. The API key is deliberately not validated here —
	// it arrives via env at runtime; an enabled-but-keyless deployment logs
	// a warning and ingests unannotated (fail-open, never a hard gate).
	if c.RAG.Contextual.Enabled {
		if !c.Features.RAGHybridEnabled {
			errs = append(errs, "rag.contextual.enabled requires features.rag_hybrid_enabled (annotation rides the content projection)")
		}
		if strings.TrimSpace(c.RAG.Contextual.Provider) == "" || strings.TrimSpace(c.RAG.Contextual.Model) == "" {
			errs = append(errs, "rag.contextual.provider and rag.contextual.model are required when rag.contextual.enabled")
		}
		// Upper bound aligns with the annotator's fixed 200-token output
		// cap: a configured 300–500 could never be delivered.
		if c.RAG.Contextual.MaxPrefixTokens < 20 || c.RAG.Contextual.MaxPrefixTokens > 200 {
			errs = append(errs, "rag.contextual.max_prefix_tokens must be within 20..200")
		}
		if c.RAG.Contextual.DocContextChars < 200 {
			errs = append(errs, "rag.contextual.doc_context_chars must be >= 200")
		}
		if c.RAG.Contextual.Concurrency < 1 || c.RAG.Contextual.Concurrency > 16 {
			errs = append(errs, "rag.contextual.concurrency must be within 1..16")
		}
		if c.RAG.Contextual.RequestIntervalMS < 0 {
			errs = append(errs, "rag.contextual.request_interval_ms must be non-negative")
		}
		if c.RAG.Contextual.TimeoutSec <= 0 {
			errs = append(errs, "rag.contextual.timeout_sec must be positive")
		}
		if c.RAG.Contextual.MaxRetries < 0 {
			errs = append(errs, "rag.contextual.max_retries must be non-negative")
		}
	}

	if c.Relay.BatchSize < RelayMinBatchSize || c.Relay.BatchSize > RelayMaxBatchSize {
		errs = append(errs, fmt.Sprintf("relay.batch_size must be between %d and %d in release mode", RelayMinBatchSize, RelayMaxBatchSize))
	}
	if c.Relay.PollIntervalSec < RelayMinPollIntervalSec || c.Relay.PollIntervalSec > RelayMaxPollIntervalSec {
		errs = append(errs, fmt.Sprintf("relay.poll_interval_sec must be between %d and %d in release mode", RelayMinPollIntervalSec, RelayMaxPollIntervalSec))
	}

	if c.Agent.WebAgentEnabled {
		requireNonEmpty(&errs, "agent.llm_api_key", c.Agent.LLMAPIKey)
		if p := strings.TrimSpace(c.Agent.EmbeddingProvider); p != "" &&
			!strings.EqualFold(p, strings.TrimSpace(c.Agent.LLMProvider)) &&
			strings.TrimSpace(c.Agent.EmbeddingAPIKey) == "" {
			errs = append(errs, "agent.embedding_provider differs from agent.llm_provider: agent.embedding_api_key is required (cross-vendor embedding must not borrow the chat credential)")
		}
		requirePositiveInt(&errs, "agent.rate_limit_per_day", c.Agent.RateLimitPerDay)
		requirePositiveInt(&errs, "agent.rate_limit_per_minute", c.Agent.RateLimitPerMinute)
		requirePositiveInt(&errs, "agent.max_tool_calls_per_turn", c.Agent.MaxToolCallsPerTurn)
		requirePositiveInt(&errs, "agent.max_output_tokens", c.Agent.MaxOutputTokens)
		requirePositiveInt(&errs, "agent.provider_timeout_sec", c.Agent.ProviderTimeoutSec)
		if c.Agent.Image.ImageConfigured() {
			requirePositiveInt(&errs, "agent.image.timeout_sec", c.Agent.Image.TimeoutSec)
			requirePositiveInt(&errs, "agent.image.session_image_limit", c.Agent.Image.SessionImageLimit)
			requirePositiveInt(&errs, "agent.image.max_image_bytes", c.Agent.Image.MaxImageBytes)
			if strings.TrimSpace(c.Agent.Image.Model) == "" || strings.TrimSpace(c.Agent.Image.APIBase) == "" {
				errs = append(errs, "agent.image.model and agent.image.api_base are required when image generation is enabled")
			}
		}
		requirePositiveInt(&errs, "agent.citation_max_count", c.Agent.CitationMaxCount)
		requirePositiveInt(&errs, "agent.max_user_message_chars", c.Agent.MaxUserMessageChars)
		requirePositiveInt(&errs, "agent.chat_max_context_messages", c.Agent.ChatMaxContextMsgs)
		requirePositiveInt(&errs, "agent.conversation_list_limit", c.Agent.ConversationListLimit)
		requirePositiveInt(&errs, "agent.conversation_page_size", c.Agent.ConversationPageSize)
		requirePositiveInt(&errs, "agent.chat_context_token_budget", c.Agent.ChatContextTokenBudget)
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
	if c.Observability.Tracing.SampleRatio < 0 || c.Observability.Tracing.SampleRatio > 1 {
		errs = append(errs, "observability.tracing.sample_ratio must be between 0 and 1 in release mode")
	}
	if c.Observability.Tracing.Enabled {
		if strings.TrimSpace(c.Observability.Tracing.Endpoint) == "" {
			errs = append(errs, "observability.tracing.endpoint is required when tracing is enabled")
		}
		if strings.TrimSpace(c.Observability.Tracing.Backend) != "jaeger" {
			errs = append(errs, "observability.tracing.backend must be jaeger")
		}
	}
	if c.Observability.AgentTrace.Enabled {
		at := c.Observability.AgentTrace
		if at.SampleRatio < 0 || at.SampleRatio > 1 {
			errs = append(errs, "observability.agent_trace.sample_ratio must be between 0 and 1")
		}
		requirePositiveInt(&errs, "observability.agent_trace.channel_size", at.ChannelSize)
		requirePositiveInt(&errs, "observability.agent_trace.flush_interval_ms", at.FlushIntervalMs)
		requirePositiveInt(&errs, "observability.agent_trace.flush_batch_size", at.FlushBatchSize)
		requirePositiveInt(&errs, "observability.agent_trace.retention_days", at.RetentionDays)
		if at.DigestMaxRunes < 0 {
			errs = append(errs, "observability.agent_trace.digest_max_runes must not be negative")
		}
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
