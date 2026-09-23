package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/agentmcp"
	"omnicraft/backend/internal/mcpserver"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/observability/agenttrace"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/auxcache"
	"omnicraft/backend/internal/pkg/breaker"
	"omnicraft/backend/internal/pkg/captcha"
	"omnicraft/backend/internal/pkg/clamav"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/mail"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/service/promptregistry"
	ragservice "omnicraft/backend/internal/service/rag"
	"omnicraft/backend/internal/worker"
)

type ServiceContainer struct {
	DB  *gorm.DB
	RDB *redis.Client
	Cfg *config.Config

	// Queue
	QueueBroker   queue.Broker
	QueueProducer queue.Producer

	// Repositories
	UserRepo          *repository.UserRepository
	ContentRepo       *repository.ContentRepository
	IPRepo            *repository.IPRepository
	SocialRepo        *repository.SocialRepository
	FollowRepo        *repository.FollowRepository
	JudgeRepo         *repository.JudgeRepository
	TagRepo           *repository.TagRepository
	CategoryRepo      *repository.CategoryRepository
	PRRepo            *repository.PRRepository
	VersionRepo       *repository.VersionRepository
	AppealRepo        *repository.AppealRepository
	NotificationRepo  *repository.NotificationRepository
	BrowseHistoryRepo *repository.BrowseHistoryRepository
	DiscussionRepo    *repository.DiscussionRepository
	MessageRepo       *repository.MessageRepository
	RehabRepo         *repository.RehabRepository
	EmbeddingRepo     *repository.EmbeddingRepository
	LLMConfigRepo     *repository.LLMConfigRepository
	SearchRepo        *repository.SearchRepository
	FeedbackRepo      *repository.FeedbackRepository
	AdminAuditRepo    *repository.AdminAuditRepository
	OutboxRepo        *repository.OutboxRepository
	ArchiveScanRepo   *repository.ArchiveScanRepository
	OpenSearchRepo    *repository.OpenSearchRepository
	HybridRetriever   *ragservice.HybridRetriever
	// AgentTraceWriter batches agent trace run/node upserts off the request
	// path (SP-21 T1, map #549); nil-safe, gated by
	// observability.agent_trace.enabled.
	AgentTraceRepo   *repository.AgentTraceRepository
	AgentTraceWriter *agenttrace.Writer
	// RagEvaluationRepo owns the golden-set / eval-run tables (SP-22 E5
	// admin evals surface; the rag-eval grid runner records through it too).
	RagEvaluationRepo *repository.RagEvaluationRepository

	// Services
	AuthService         *service.AuthService
	VerificationService *service.VerificationService
	ContentService      *service.ContentService
	// StudioContentService is the full-featured content service the HTTP
	// studio routes and the MCP write channel share (#658): upload grants,
	// object verification, archive scanning, versions, recommendation and
	// the transactional outbox. ContentService above stays the read-only
	// scheduler copy (recommendation base, no upload surface).
	StudioContentService *service.ContentService
	// OSSService is the single shared presign service for studio uploads,
	// MCP write tools and feedback screenshots (#658 收拢：原 3 份); nil
	// with OSSInitErr set keeps each surface's fail-open behavior.
	OSSService          *service.OSSService
	OSSInitErr          error
	UploadGrants        *service.UploadGrantService
	IPService           *service.IPService
	SocialService       *service.SocialService
	ReputationService   *service.ReputationService
	ReviewService       *service.ReviewService
	JudgeService        *service.JudgeService
	RecommendationSvc   *service.RecommendationService
	StatsService        *service.StatsService
	IPStatsService      *service.IPStatsService
	AgentService        *service.AgentService
	AgentTokenService   *service.AgentAccessTokenService
	NotificationService *service.NotificationService
	PRService           *service.PRService
	VersionService      *service.VersionService
	UsageGuideService   *service.UsageGuideService
	MCPHandler          http.Handler
	SearchService       *service.SearchService
	IPProposalService   *service.IPProposalService
	FeedbackService     *service.FeedbackService
	AdminAuditService   *service.AdminAuditService
	CollabInviteService *service.CollabInviteService
	CaptchaVerifier     captcha.CaptchaVerifier
	CaptchaProvider     captcha.CaptchaVerifier
	CaptchaTickets      *captcha.TicketStore
	RAGProjection       *ragservice.Projection
	ArchiveObjectStore  worker.ArchiveScanObjectStore
	ArchiveScanner      worker.ArchiveScanner
	// PromptRegistryService resolves versioned prompt templates over the
	// compiled-in builtins (SP-21 T5); shared by server and worker so admin
	// label moves are hot in both processes.
	PromptRegistryRepo    *repository.PromptRegistryRepository
	PromptRegistryService *promptregistry.PromptResolver
	// DisplayURLSigner issues short-lived signed GET URLs for display media
	// at the API serialization boundary (B-002); nil-safe passthrough when
	// OSS is not configured.
	DisplayURLSigner *service.DisplayURLSigner
}

// NewContainer builds the single source of truth for the dependency graph
// and validates its own wiring before returning (#658): any missing
// REQUIRED dependency fails construction with the complete list.
func NewContainer(db *gorm.DB, rdb *redis.Client, cfg *config.Config) (*ServiceContainer, error) {
	c := &ServiceContainer{
		DB:  db,
		RDB: rdb,
		Cfg: cfg,
	}

	// Queue setup
	if cfg.Queue.Enabled && rdb != nil {
		broker := queue.NewRedisStreamBroker(rdb, &cfg.Queue)
		c.QueueBroker = broker
		c.QueueProducer = broker
	} else {
		c.QueueBroker = nil
		c.QueueProducer = queue.NewNoopProducer()
	}

	// Repositories
	c.UserRepo = repository.NewUserRepository(db)
	c.ContentRepo = repository.NewContentRepository(db)
	c.IPRepo = repository.NewIPRepository(db)
	c.SocialRepo = repository.NewSocialRepository(db)
	c.FollowRepo = repository.NewFollowRepository(db)
	c.JudgeRepo = repository.NewJudgeRepository(db)
	c.TagRepo = repository.NewTagRepository(db)
	c.CategoryRepo = repository.NewCategoryRepository(db)
	c.PRRepo = repository.NewPRRepository(db)
	c.VersionRepo = repository.NewVersionRepository(db)
	c.AppealRepo = repository.NewAppealRepository(db)
	c.NotificationRepo = repository.NewNotificationRepository(db)
	c.BrowseHistoryRepo = repository.NewBrowseHistoryRepository(db)
	c.DiscussionRepo = repository.NewDiscussionRepository(db)
	c.MessageRepo = repository.NewMessageRepository(db)
	c.RehabRepo = repository.NewRehabRepository(db)
	c.EmbeddingRepo = repository.NewEmbeddingRepository(db)
	c.LLMConfigRepo = repository.NewLLMConfigRepository(db)
	c.SearchRepo = repository.NewSearchRepository(db)
	c.FeedbackRepo = repository.NewFeedbackRepository(db)
	c.AdminAuditRepo = repository.NewAdminAuditRepository(db)
	c.OutboxRepo = repository.NewOutboxRepository(db)
	backoff := make([]time.Duration, 0, len(cfg.ArchiveScan.RetryBackoffSec))
	for _, seconds := range cfg.ArchiveScan.RetryBackoffSec {
		if seconds >= 0 {
			backoff = append(backoff, time.Duration(seconds)*time.Second)
		}
	}
	c.ArchiveScanRepo = repository.NewArchiveScanRepositoryWithOutbox(db, repository.ArchiveScanRetryPolicy{Backoff: backoff}, c.OutboxRepo)
	// SP-21 T1: async trace persistence. Start/Stop are owned by the
	// server/worker mains so shutdown ordering (flush before redis close)
	// stays explicit; recording call sites arrive with T2.
	c.AgentTraceRepo = repository.NewAgentTraceRepository(db)
	c.RagEvaluationRepo = repository.NewRagEvaluationRepository(db)
	c.AgentTraceWriter = agenttrace.NewWriter(c.AgentTraceRepo, agenttrace.Options{
		Enabled:        cfg.Observability.AgentTrace.Enabled,
		SampleRatio:    cfg.Observability.AgentTrace.SampleRatio,
		ChannelSize:    cfg.Observability.AgentTrace.ChannelSize,
		FlushInterval:  time.Duration(cfg.Observability.AgentTrace.FlushIntervalMs) * time.Millisecond,
		FlushBatchSize: cfg.Observability.AgentTrace.FlushBatchSize,
		DigestMaxRunes: cfg.Observability.AgentTrace.DigestMaxRunes,
		KeepFullPrompt: cfg.Observability.AgentTrace.KeepFullPrompt,
		RetentionDays:  cfg.Observability.AgentTrace.RetentionDays,
	})
	if cfg.Features.ArchiveMalwareScanEnabled {
		ossClient, ossErr := aliyun.NewOSSClient(cfg.OSS.Endpoint, cfg.OSS.AccessKeyID, cfg.OSS.AccessKeySecret, cfg.OSS.BucketName)
		if ossErr != nil {
			slog.Error("archive scan OSS object store is unavailable", "error", ossErr)
		} else {
			c.ArchiveObjectStore = ossClient
		}
		c.ArchiveScanner = clamav.NewClient(cfg.ArchiveScan.ClamdAddress, time.Duration(cfg.ArchiveScan.ScanTimeoutSec)*time.Second)
	}

	// Services
	c.AuthService = service.NewAuthService(c.UserRepo, rdb, cfg)
	c.DisplayURLSigner = service.NewDisplayURLSigner(cfg)

	// One shared OSS presign instance serves the studio upload surface, the
	// MCP write channel and feedback screenshots (#658 收拢：原 3 份).
	c.OSSService, c.OSSInitErr = service.NewOSSService(cfg)
	if c.OSSInitErr != nil {
		slog.Warn("OSS presign is unavailable", "error", c.OSSInitErr)
	}
	// One shared per-user upload-grant store (same redis keys for web studio
	// uploads and MCP writes; TTL contract unchanged).
	grantTTL := 5 * time.Minute
	if cfg.Feedback.UploadGrantTTLSec > 0 {
		grantTTL = time.Duration(cfg.Feedback.UploadGrantTTLSec) * time.Second
	}
	c.UploadGrants = service.NewUploadGrantService(rdb, grantTTL)

	var mailSender mail.MailSender
	var feedbackMailSender service.FeedbackMailSender
	if cfg.SMTP.Mode == "smtp" {
		sender := mail.NewSMTPSender(cfg.SMTP)
		mailSender = sender
		feedbackMailSender = sender
	} else {
		sender := mail.NewLoggerSender()
		mailSender = sender
		feedbackMailSender = sender
	}

	captchaProvider := captcha.NewCaptchaVerifier(cfg.Captcha)
	captchaTickets := captcha.NewTicketStore(rdb, cfg.Captcha.TicketTTLSec)
	c.CaptchaProvider = captchaProvider
	c.CaptchaTickets = captchaTickets
	c.CaptchaVerifier = captcha.NewTicketAwareVerifier(cfg.Captcha.Provider, captchaProvider, captchaTickets)
	c.VerificationService = service.NewVerificationService(c.UserRepo, rdb, mailSender, cfg)

	c.IPService = service.NewIPService(c.IPRepo)
	c.ReputationService = service.NewReputationService(db)
	c.ReviewService = service.NewReviewService(db, rdb, cfg, c.ReputationService)
	c.ReviewService.SetOutboxRepository(c.OutboxRepo)
	c.ReviewService.SetArchiveScanGate(service.NewArchiveScanGate(db, cfg.Features.ArchiveMalwareScanEnabled))
	c.JudgeService = service.NewJudgeService(c.JudgeRepo, c.ReputationService, cfg)
	c.ContentService = service.NewContentServiceWithOSS(c.ContentRepo, c.ReviewService, rdb, &cfg.Cache, nil).
		WithArchiveScanConfig(&cfg.ArchiveScan)
	c.ContentService.SetOutboxRepository(c.OutboxRepo)
	c.SocialService = service.NewSocialServiceWithRedis(c.SocialRepo, c.ContentRepo, c.UserRepo, cfg, rdb, c.ReviewService)
	c.StatsService = service.NewStatsService(db, rdb)
	c.IPStatsService = service.NewIPStatsService(db, rdb)
	c.NotificationService = service.NewNotificationService(c.NotificationRepo)
	// AI 审核结果与 IP 级联下架通知作者（FIX-17a）。
	c.ReviewService.SetNotificationService(c.NotificationService)
	// 判官闭案回写内容状态 + 作者通知（FIX-10）。
	c.JudgeService.SetNotificationService(c.NotificationService)
	c.JudgeService.SetContentOutcomeWriter(db, rdb, c.OutboxRepo)
	c.PRService = service.NewPRService(c.PRRepo, c.VersionRepo, c.ContentRepo)
	// merge 事务（版本+正文+索引事件）/ 缓存失效 / +3 信誉分（FIX-21）。
	c.PRService.SetMergeSupport(rdb, c.OutboxRepo, c.ReputationService)
	c.VersionService = service.NewVersionService(c.VersionRepo, c.ContentRepo)
	// FIX-42: 发布事务内建初始版本 v1（full=description）。
	c.ContentService.SetVersionService(c.VersionService)
	c.SearchService = service.NewSearchService(c.SearchRepo, rdb)
	c.IPProposalService = service.NewIPProposalService(
		c.IPRepo, c.UserRepo, c.FollowRepo,
		repository.NewIPProposalRepository(db), rdb, cfg,
	)
	c.IPProposalService.SetNotifier(c.NotificationService.Notify)
	c.IPProposalService.SetReviewService(c.ReviewService)

	uploadGrantTTL := 300
	if cfg.Feedback.UploadGrantTTLSec > 0 {
		uploadGrantTTL = cfg.Feedback.UploadGrantTTLSec
	}
	c.FeedbackService = service.NewFeedbackService(c.FeedbackRepo, c.UserRepo, rdb, c.CaptchaVerifier, uploadGrantTTL, c.OSSService)
	c.FeedbackService.SetNotificationService(c.NotificationService)
	c.FeedbackService.SetFeedbackMailSender(feedbackMailSender)
	c.FeedbackService.SetReviewService(c.ReviewService)
	c.FeedbackService.SetConfig(cfg)
	c.AdminAuditService = service.NewAdminAuditService(c.AdminAuditRepo, db)
	c.NotificationService.SetAdminAuditService(c.AdminAuditService)
	c.CollabInviteService = service.NewCollabInviteService(
		c.ContentRepo,
		repository.NewCollabInviteRepository(db),
		c.MessageRepo,
		c.UserRepo,
		rdb,
		cfg,
	)

	// Wire recommendation into content service
	c.RecommendationSvc = service.NewRecommendationService(db, c.EmbeddingRepo, c.ContentRepo, c.ContentService, rdb, &cfg.Recommendation)
	c.ContentService.SetRecommendationService(c.RecommendationSvc)

	// Wire queue producer into services
	c.ContentService.SetQueueProducer(c.QueueProducer)
	c.IPService.SetQueueProducer(c.QueueProducer)
	c.NotificationService.SetQueueProducer(c.QueueProducer)

	// #658 内容服务收敛 ×2：调度器只读份（上方 c.ContentService）+ 这份
	// HTTP/MCP 共用全功能份。MCP 写路径由此补上事务性发件箱——外部 Agent
	// 写入的内容与 HTTP 发布产生相同的索引事件（本批唯一预期行为变更）。
	c.StudioContentService = service.NewContentServiceWithOSS(c.ContentRepo, c.ReviewService, rdb, &cfg.Cache, c.OSSService).
		WithUploadGrantService(c.UploadGrants).
		WithUploadedObjectVerifier(c.OSSService).
		WithArchiveScanConfig(&cfg.ArchiveScan).
		WithArchiveScanGateEnabled(cfg.Features.ArchiveMalwareScanEnabled).
		WithImageDimensionsResolver(c.OSSService).
		WithUploadConfig(&cfg.Upload)
	c.StudioContentService.SetVersionService(c.VersionService)
	c.StudioContentService.SetOutboxRepository(c.OutboxRepo)
	c.StudioContentService.SetQueueProducer(c.QueueProducer)
	c.StudioContentService.SetArchiveScanRepository(c.ArchiveScanRepo, cfg.Features.ArchiveMalwareScanEnabled)
	c.StudioContentService.SetRecommendationService(c.RecommendationSvc)

	// Usage-guide merged view (SP-16 #447): constructed before AgentService
	// so the in-site agent can read structured guides first.
	c.UsageGuideService = service.NewUsageGuideService(
		repository.NewUsageGuideRepository(db),
		c.ContentRepo,
	)

	// MCP server surface (SP-16 #449/#451): the four anonymous read-only
	// tools close over the same repos as the public REST surface; the
	// PAT-scoped write tools consume the shared StudioContentService and
	// OSS/grant instances so external agents hit byte-identical validation
	// and event semantics (#658).
	c.MCPHandler = mcpserver.NewHandler(mcpserver.Deps{
		DB:            db,
		SearchRepo:    c.SearchRepo,
		ContentRepo:   c.ContentRepo,
		CategoryRepo:  c.CategoryRepo,
		GuideSvc:      c.UsageGuideService,
		DisplaySigner: c.DisplayURLSigner,
		Cfg:           cfg,
		ContentSvc:    c.StudioContentService,
		// SuggestPublishMetadata reuses the in-chat upload-assist LLM;
		// AgentService is constructed below, hence the late-bound closure.
		SuggestPublishMetadata: func(ctx context.Context, title, description, filename, contentType string) (*service.UploadAssistResult, error) {
			return c.AgentService.UploadAssist(ctx, 0, title, description, filename, contentType)
		},
		// IssueUploadURL mirrors POST /contents/oss-token: presign then
		// register the grant the later create call consumes.
		IssueUploadURL: func(ctx context.Context, req service.PresignUploadRequest, userID int64) (*service.PresignUploadResponse, error) {
			resp, err := c.OSSService.GeneratePresignUploadURL(ctx, req, userID)
			if err != nil {
				return nil, err
			}
			grant, err := c.UploadGrants.Issue(ctx, service.UploadGrant{
				UserID:   userID,
				Purpose:  "content",
				OSSKey:   resp.OSSKey,
				FileType: req.FileType,
				MimeType: req.MimeType,
				FileSize: req.FileSize,
			})
			if err != nil {
				return nil, err
			}
			resp.GrantID = grant.ID
			return resp, nil
		},
		// The shared per-user hourly upload window (same redis keys as the
		// web studio uploads).
		ConsumeUploadQuota: func(ctx context.Context, userID int64) error {
			return middleware.ConsumeUploadQuota(ctx, rdb, &cfg.RateLimit, userID)
		},
	})

	// Create AgentService for worker use
	provider := llm.NewProvider(cfg)
	greenClient := aliyun.NewGreenClient(cfg.Green.AccessKeyID, cfg.Green.AccessKeySecret, cfg.Green.Region)
	// SP-21 T5: prompt registry — built before AgentService so every prompt
	// consumer shares one resolver (cache + invalidation). Seeding is
	// idempotent (ON CONFLICT DO NOTHING); a missing registry table or a
	// failed seed degrades to builtins with a log, never blocks startup.
	c.PromptRegistryRepo = repository.NewPromptRegistryRepository(db)
	c.PromptRegistryService = promptregistry.NewPromptResolver(c.PromptRegistryRepo)
	if err := promptregistry.SeedV1(context.Background(), c.PromptRegistryRepo); err != nil {
		slog.Warn("prompt registry v1 seed failed; builtins stay active", "error", err)
	}
	// #610: code-shipped version bumps (agent_system v2 image-tool guidance).
	// Ships once per fresh version row; later boots never move an
	// admin-managed label. Failure degrades to the current label, not startup.
	if err := promptregistry.SeedUpgrades(context.Background(), c.PromptRegistryRepo); err != nil {
		slog.Warn("prompt registry upgrade seed failed; current labels stay active", "error", err)
	}
	c.AgentService = service.NewAgentService(provider, c.EmbeddingRepo, c.ContentRepo, greenClient, db, cfg)
	// SP-23 M3: MCP client bridge — inert unless agent.mcp.enabled; dead
	// server subprocesses degrade to "no tools from that server".
	c.AgentService.SetMCPBridge(agentmcp.NewGuardedBridge(agentmcp.New(cfg.Agent.MCP), c.newBreaker(cfg, "mcp")))
	// SP-23 M1: generate_image wiring — fail-closed on every seam (switch,
	// key, OSS store); an unconfigured image endpoint never surfaces the
	// tool to the model. A dedicated OSS client keeps image availability
	// independent of the archive-scan feature switch.
	if cfg.Agent.Image.ImageConfigured() {
		if ossClient, ossErr := aliyun.NewOSSClient(cfg.OSS.Endpoint, cfg.OSS.AccessKeyID, cfg.OSS.AccessKeySecret, cfg.OSS.BucketName); ossErr != nil {
			slog.Error("generate_image disabled: OSS store unavailable", "error", ossErr)
		} else {
			c.AgentService.SetImageTool(cfg.Agent.Image, llm.NewGuardedImageGenerator(llm.NewCogViewClient(
				cfg.Agent.Image.APIBase, cfg.Agent.Image.APIKey, cfg.Agent.Image.Model,
			), c.newBreaker(cfg, "image")), &ossAgentImageStore{client: ossClient})
		}
	}
	c.AgentTokenService = service.NewAgentAccessTokenService(
		repository.NewAgentAccessTokenRepository(db), cfg)
	c.AgentService.SetSearchRepository(c.SearchRepo)
	c.AgentService.SetUsageGuideService(c.UsageGuideService)
	c.AgentService.SetQueueProducer(c.QueueProducer)
	c.AgentService.SetPromptResolver(c.PromptRegistryService)
	// SP-21 T2: turn instrumentation rides the shared async writer.
	c.AgentService.SetTraceWriter(c.AgentTraceWriter)
	// SP-24 R6: auto-title content-hash cache (disabled config = bypass).
	c.AgentService.SetTitleCache(c.newAuxCache(cfg.Resilience.AuxCache.Title, "title", rdb))
	opensearchTimeout := time.Duration(cfg.RAG.Index.TimeoutSec) * time.Second
	c.OpenSearchRepo = repository.NewOpenSearchRepositoryWithLimits(
		cfg.RAG.Index.URL,
		&http.Client{Timeout: opensearchTimeout},
		repository.OpenSearchResponseLimits{
			ErrorBodyMaxBytes:     int64(cfg.RAG.Index.ErrorBodyMaxBytes),
			ResponseBodyMaxBytes:  int64(cfg.RAG.Index.ResponseBodyMaxBytes),
			HealthPollIntervalSec: cfg.RAG.Index.HealthPollIntervalSec,
		},
	)
	// Lexical primary follows rag.hybrid.keyword_source: the pg_jieba
	// Postgres retriever is the canonical path (viewer-scoped queries); the
	// OpenSearch retriever serves as the optional fallback. Full-infra stacks
	// may invert the pair explicitly.
	var keywordPrimary ragservice.KeywordRetriever = ragservice.NewPostgresKeywordRetriever(c.SearchRepo)
	var keywordFallback ragservice.KeywordRetriever = ragservice.NewGuardedKeywordRetriever(
		ragservice.NewOpenSearchKeywordRetriever(c.OpenSearchRepo), c.newBreaker(cfg, "opensearch"))
	if strings.EqualFold(strings.TrimSpace(cfg.RAG.Hybrid.KeywordSource), "opensearch") {
		keywordPrimary, keywordFallback = keywordFallback, keywordPrimary
	}
	c.HybridRetriever = ragservice.NewHybridRetriever(
		keywordPrimary,
		keywordFallback,
		ragservice.NewPostgresVectorRetriever(c.EmbeddingRepo, cfg.RAG.Index.EmbeddingModel),
		provider,
		ragservice.NewDatabaseVisibilityFilter(db),
		cfg.RAG.Hybrid,
	)
	// A-03 retrieval upgrades, each behind its feature flag; defaults stay
	// off until the A-04 ablation decides them.
	if cfg.Features.RAGQueryExpansionEnabled {
		expander := ragservice.NewLLMQueryExpander(provider)
		expander.SetPromptResolver(c.PromptRegistryService)
		// SP-24 R6: query-hash TTL cache (disabled config = bypass cache).
		expander.SetResultCache(c.newAuxCache(cfg.Resilience.AuxCache.Expander, "expander", rdb))
		c.HybridRetriever.SetQueryExpander(expander)
	}
	if cfg.Features.RAGRerankEnabled {
		if reranker, inputTopK := llm.NewRerankerFromConfig(cfg.RAG.Rerank); reranker != nil {
			c.HybridRetriever.SetReranker(llm.NewGuardedReranker(reranker, c.newBreaker(cfg, "rerank")), inputTopK)
			// SP-24 R3: the similarity-floor refusal only carries meaning
			// next to a wired reranker (relevance scores live on that path).
			c.HybridRetriever.SetMinTopRelevance(cfg.RAG.Refusal.MinTopRelevanceScore)
		}
	}
	if cfg.Features.RAGHybridEnabled {
		c.AgentService.SetContentRetriever(&agentRAGRetriever{retriever: c.HybridRetriever})
	}
	c.RAGProjection = ragservice.NewProjectionWithVersionLoader(
		db,
		ragservice.NewChunker(ragservice.ChunkerConfig{
			MaxTokens: cfg.RAG.Chunking.MaxTokens, OverlapTokens: cfg.RAG.Chunking.OverlapTokens,
			ChunkingVersion: cfg.RAG.Chunking.ChunkingVersion, TokenizerEncoding: cfg.RAG.Chunking.TokenizerEncoding,
		}),
		ragservice.NewProviderChunkEmbedder(provider),
		c.OpenSearchRepo,
		c.VersionService,
		ragservice.ProjectionConfig{
			IndexVersion: cfg.RAG.Index.GenerationStart, EmbeddingModel: cfg.RAG.Index.EmbeddingModel,
			EmbeddingDimensions:   cfg.Agent.EmbeddingDimensions,
			LockCleanupTimeoutSec: cfg.RAG.Index.LockCleanupTimeoutSec,
		},
	)
	// SP-24 R4 contextual retrieval: the annotation provider is separate from
	// the agent chat provider (cheap one-shot ingestion calls, DeepSeek by
	// default). An enabled-but-keyless deployment logs once and keeps
	// ingesting unannotated — fail-open, never a gate.
	if cfg.RAG.Contextual.Enabled {
		if strings.TrimSpace(cfg.RAG.Contextual.APIKey) == "" {
			slog.Warn("rag.contextual.enabled but no API key resolved (RAG_CONTEXTUAL_API_KEY / AGENT_MODEL_DEEPSEEK_API_KEY); ingesting unannotated",
				"provider", cfg.RAG.Contextual.Provider, "model", cfg.RAG.Contextual.Model)
		} else {
			annotationProvider := llm.NewProviderFromConfig(
				cfg.RAG.Contextual.Provider, cfg.RAG.Contextual.APIKey, cfg.RAG.Contextual.APIBase, cfg.RAG.Contextual.Model, "",
				llm.WithTimeout(time.Duration(cfg.RAG.Contextual.TimeoutSec)*time.Second),
				llm.WithMaxRetries(cfg.RAG.Contextual.MaxRetries),
			)
			c.RAGProjection.SetContextAnnotator(ragservice.NewContextAnnotator(
				annotationProvider, cfg.RAG.Contextual, c.PromptRegistryService,
			))
		}
	}

	// Wire notification service
	c.SocialService.SetNotificationService(c.NotificationService)
	// 举报 auto-hide 触发众裁（FIX-11）：复用 AI 审核路径的幂等建案。
	c.SocialService.SetJudgeCaseEnsurer(c.ReviewService)
	c.PRService.SetNotificationService(c.NotificationService)

	return c, c.ValidateWiring()
}

// ValidateWiring asserts every REQUIRED dependency of the composition root
// resolved non-nil (#658). Wiring drift previously failed silently through
// nil-tolerant Set* seams; now a missing REQUIRED dependency fails startup
// with the COMPLETE list in one boot instead of one deploy per finding.
//
// OPTIONAL by design and therefore not asserted: QueueBroker (nil when the
// queue is disabled; QueueProducer falls back to the no-op producer),
// OSSService (unconfigured deployments keep per-surface fail-open 503s with
// OSSInitErr), ArchiveObjectStore/ArchiveScanner (gated behind
// features.archive_malware_scan_enabled).
func (c *ServiceContainer) ValidateWiring() error {
	required := []struct {
		name string
		val  any
	}{
		{"db", c.DB},
		{"redis", c.RDB},
		{"config", c.Cfg},
		{"queue.producer", c.QueueProducer},
		{"repo.user", c.UserRepo},
		{"repo.content", c.ContentRepo},
		{"repo.ip", c.IPRepo},
		{"repo.social", c.SocialRepo},
		{"repo.follow", c.FollowRepo},
		{"repo.judge", c.JudgeRepo},
		{"repo.tag", c.TagRepo},
		{"repo.category", c.CategoryRepo},
		{"repo.pr", c.PRRepo},
		{"repo.version", c.VersionRepo},
		{"repo.appeal", c.AppealRepo},
		{"repo.notification", c.NotificationRepo},
		{"repo.browse_history", c.BrowseHistoryRepo},
		{"repo.discussion", c.DiscussionRepo},
		{"repo.message", c.MessageRepo},
		{"repo.rehab", c.RehabRepo},
		{"repo.embedding", c.EmbeddingRepo},
		{"repo.llm_config", c.LLMConfigRepo},
		{"repo.search", c.SearchRepo},
		{"repo.feedback", c.FeedbackRepo},
		{"repo.admin_audit", c.AdminAuditRepo},
		{"repo.outbox", c.OutboxRepo},
		{"repo.archive_scan", c.ArchiveScanRepo},
		{"repo.agent_trace", c.AgentTraceRepo},
		{"repo.rag_evaluation", c.RagEvaluationRepo},
		{"agenttrace.writer", c.AgentTraceWriter},
		{"service.auth", c.AuthService},
		{"service.verification", c.VerificationService},
		{"service.content", c.ContentService},
		{"service.studio_content", c.StudioContentService},
		{"service.ip", c.IPService},
		{"service.social", c.SocialService},
		{"service.reputation", c.ReputationService},
		{"service.review", c.ReviewService},
		{"service.judge", c.JudgeService},
		{"service.recommendation", c.RecommendationSvc},
		{"service.stats", c.StatsService},
		{"service.ip_stats", c.IPStatsService},
		{"service.agent", c.AgentService},
		{"service.agent_token", c.AgentTokenService},
		{"service.notification", c.NotificationService},
		{"service.pr", c.PRService},
		{"service.version", c.VersionService},
		{"service.usage_guide", c.UsageGuideService},
		{"service.search", c.SearchService},
		{"service.ip_proposal", c.IPProposalService},
		{"service.feedback", c.FeedbackService},
		{"service.admin_audit", c.AdminAuditService},
		{"service.collab_invite", c.CollabInviteService},
		{"service.prompt_registry", c.PromptRegistryService},
		{"upload.grants", c.UploadGrants},
		{"captcha.verifier", c.CaptchaVerifier},
		{"captcha.provider", c.CaptchaProvider},
		{"captcha.tickets", c.CaptchaTickets},
		{"display.signer", c.DisplayURLSigner},
		{"mcp.handler", c.MCPHandler},
		{"opensearch.repo", c.OpenSearchRepo},
		{"hybrid.retriever", c.HybridRetriever},
		{"rag.projection", c.RAGProjection},
	}
	var missing []string
	for _, dep := range required {
		if isNilValue(dep.val) {
			missing = append(missing, dep.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("composition root wiring incomplete; missing REQUIRED dependencies: %s", strings.Join(missing, ", "))
	}
	return nil
}

// isNilValue reports whether an any-wrapped dependency is nil, unwrapping
// interfaces so a typed-nil pointer inside an interface field also counts.
func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	case reflect.Interface:
		return rv.IsNil() || isNilValue(rv.Elem().Interface())
	default:
		return false
	}
}

// StartWorkers starts all queue consumers and the outbox relay. It is the
// single entry point of the standalone worker process (cmd/worker): the API
// server never starts asynchronous consumers (ADR 0005) and no
// worker.external=false fallback exists. Returns a stop function that
// gracefully drains the consumers and the relay.
func (c *ServiceContainer) StartWorkers(ctx context.Context) func() {
	if c.QueueBroker == nil {
		slog.Warn("worker: queue broker is nil (queue disabled or redis absent), skipping worker startup")
		return func() {}
	}

	mgr := worker.NewWorkerManager(c.QueueBroker)
	concurrency := c.Cfg.Worker.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	reviewWorker := worker.NewReviewWorker(c.ReviewService, c.DB)
	notificationWorker := worker.NewNotificationWorker(c.NotificationRepo, c.DB)
	countWorker := worker.NewCountWorker(c.RDB, c.DB)
	embeddingWorker := worker.NewEmbeddingWorker(c.AgentService, c.DB)
	// #658 收拢：worker 复用容器的 EmbeddingRepo（原两处自建）。
	indexerWorker := worker.NewIndexerWorker(c.DB, c.AgentService, c.EmbeddingRepo, nil)
	if c.Cfg.Features.RAGHybridEnabled {
		indexerWorker = worker.NewIndexerWorker(c.DB, c.AgentService, c.EmbeddingRepo, c.RAGProjection)
	}

	subscriptions := []struct {
		topic   string
		group   string
		handler queue.Handler
	}{
		{"content.review", "omnicraft-content-review", reviewWorker.Handle},
		{"ip.review", "omnicraft-ip-review", reviewWorker.Handle},
		{"notification.create", "omnicraft-notification", notificationWorker.Handle},
		{"count.download", "omnicraft-count", countWorker.Handle},
		{"content.embedding", "omnicraft-embedding", embeddingWorker.Handle},
		{events.TopicContentPublished, "omnicraft-indexer", indexerWorker.Handle},
		{events.TopicContentUpdated, "omnicraft-indexer", indexerWorker.Handle},
		{events.TopicContentBanned, "omnicraft-indexer", indexerWorker.Handle},
		{events.TopicContentDeleted, "omnicraft-indexer", indexerWorker.Handle},
	}
	if c.Cfg.Features.ArchiveMalwareScanEnabled && c.ArchiveObjectStore != nil && c.ArchiveScanner != nil {
		archiveScanWorker := worker.NewArchiveScanWorkerWithDBAndNotifier(
			c.ArchiveScanRepo,
			c.ArchiveObjectStore,
			c.ArchiveScanner,
			time.Duration(c.Cfg.ArchiveScan.ScanTimeoutSec)*time.Second,
			c.DB,
			c.ReviewService,
		)
		subscriptions = append(subscriptions, struct {
			topic   string
			group   string
			handler queue.Handler
		}{worker.ArchiveScanTopic, worker.ArchiveScanConsumerGroup, archiveScanWorker.Handle})
	} else if c.Cfg.Features.ArchiveMalwareScanEnabled {
		slog.Error("archive scan worker is disabled because its dependencies are unavailable")
	}
	for _, sub := range subscriptions {
		// Concurrency is one consumer goroutine per topic per unit; the Redis
		// consumer-group semantics distribute messages across them (each
		// message is delivered to exactly one consumer).
		for i := 0; i < concurrency; i++ {
			mgr.Register(sub.topic, sub.group, sub.handler)
		}
	}

	if err := mgr.Start(ctx); err != nil {
		slog.Error("Failed to start workers", "error", err)
	}

	relayCtx, relayCancel := context.WithCancel(ctx)
	recovery.GoSafe(func() {
		relay := worker.NewRelayWorker(
			service.NewRelayService(c.OutboxRepo, c.QueueProducer, c.Cfg.Relay.BatchSize, &c.Cfg.Queue),
			time.Duration(c.Cfg.Relay.PollIntervalSec)*time.Second,
		)
		if err := relay.Start(relayCtx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("relay worker stopped unexpectedly", "error", err)
		}
	})

	return func() {
		relayCancel()
		mgr.Stop()
	}
}

// newBreaker builds one guarded-dependency circuit breaker (SP-24 R5, polyu
// three-state blueprint): every transition lands in a WARN log line and the
// omnicraft_breaker_state gauge; an open circuit makes the mount skip the
// dependency and walk its existing fallback chain unchanged.
func (c *ServiceContainer) newBreaker(cfg *config.Config, name string) *breaker.Breaker {
	return breaker.New(name, breaker.Config{
		FailureThreshold: cfg.Resilience.Breaker.FailureThreshold,
		OpenTimeout:      time.Duration(cfg.Resilience.Breaker.OpenTimeoutSec) * time.Second,
	}, func(dep string, from, to breaker.State) {
		slog.Warn("[breaker] state change", "dependency", dep, "from", from.String(), "to", to.String())
		observability.SetDefaultBreakerState(dep, float64(to))
	})
}

// newAuxCache builds one SP-24 R6 auxiliary-call cache. A disabled item or a
// missing redis client yields a permanently bypassing cache, so call sites
// never branch.
func (c *ServiceContainer) newAuxCache(item config.AuxCacheItemConfig, name string, rdb *redis.Client) *auxcache.Cache {
	ttl := time.Duration(0)
	if item.Enabled {
		ttl = time.Duration(item.TTLSec) * time.Second
	}
	return auxcache.New(name, "omnicraft:auxcache:"+name+":", ttl, rdb)
}
