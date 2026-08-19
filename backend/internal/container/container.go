package container

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/captcha"
	"omnicraft/backend/internal/pkg/events"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/mail"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
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
	OpenSearchRepo    *repository.OpenSearchRepository
	HybridRetriever   *ragservice.HybridRetriever

	// Services
	AuthService         *service.AuthService
	VerificationService *service.VerificationService
	ContentService      *service.ContentService
	IPService           *service.IPService
	SocialService       *service.SocialService
	ReputationService   *service.ReputationService
	ReviewService       *service.ReviewService
	JudgeService        *service.JudgeService
	RecommendationSvc   *service.RecommendationService
	StatsService        *service.StatsService
	IPStatsService      *service.IPStatsService
	AgentService        *service.AgentService
	NotificationService *service.NotificationService
	PRService           *service.PRService
	VersionService      *service.VersionService
	SearchService       *service.SearchService
	FeedbackService     *service.FeedbackService
	AdminAuditService   *service.AdminAuditService
	CollabInviteService *service.CollabInviteService
	CaptchaVerifier     captcha.CaptchaVerifier
	CaptchaProvider     captcha.CaptchaVerifier
	CaptchaTickets      *captcha.TicketStore
	RAGProjection       *ragservice.Projection
}

func NewContainer(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *ServiceContainer {
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

	// Services
	c.AuthService = service.NewAuthService(c.UserRepo, rdb, cfg)

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
	c.JudgeService = service.NewJudgeService(c.JudgeRepo, c.ReputationService, cfg)
	c.ContentService = service.NewContentServiceWithOSS(c.ContentRepo, c.ReviewService, rdb, &cfg.Cache, nil)
	c.ContentService.SetOutboxRepository(c.OutboxRepo)
	c.SocialService = service.NewSocialServiceWithRedis(c.SocialRepo, c.ContentRepo, c.UserRepo, cfg, rdb, c.ReviewService)
	c.StatsService = service.NewStatsService(db, rdb)
	c.IPStatsService = service.NewIPStatsService(db, rdb)
	c.NotificationService = service.NewNotificationService(c.NotificationRepo)
	c.PRService = service.NewPRService(c.PRRepo, c.VersionRepo, c.ContentRepo)
	c.VersionService = service.NewVersionService(c.VersionRepo, c.ContentRepo)
	c.SearchService = service.NewSearchService(c.SearchRepo, rdb)

	uploadGrantTTL := 300
	if cfg.Feedback.UploadGrantTTLSec > 0 {
		uploadGrantTTL = cfg.Feedback.UploadGrantTTLSec
	}
	feedbackOSSSigner, err := service.NewOSSService(cfg)
	if err != nil {
		slog.Warn("Feedback screenshot OSS presign is unavailable", "error", err)
	}
	c.FeedbackService = service.NewFeedbackService(c.FeedbackRepo, c.UserRepo, rdb, c.CaptchaVerifier, uploadGrantTTL, feedbackOSSSigner)
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

	// Create AgentService for worker use
	provider := llm.NewProvider(cfg)
	greenClient := aliyun.NewGreenClient(cfg.Green.AccessKeyID, cfg.Green.AccessKeySecret, cfg.Green.Region)
	c.AgentService = service.NewAgentService(provider, c.EmbeddingRepo, c.ContentRepo, greenClient, db, cfg)
	c.AgentService.SetSearchRepository(c.SearchRepo)
	c.AgentService.SetQueueProducer(c.QueueProducer)
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
	c.HybridRetriever = ragservice.NewHybridRetriever(
		ragservice.NewOpenSearchKeywordRetriever(c.OpenSearchRepo),
		ragservice.NewPostgresKeywordRetriever(c.SearchRepo),
		ragservice.NewPostgresVectorRetriever(c.EmbeddingRepo, cfg.RAG.Index.EmbeddingModel),
		provider,
		ragservice.NewDatabaseVisibilityFilter(db),
		cfg.RAG.Hybrid,
	)
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

	// Wire notification service
	c.SocialService.SetNotificationService(c.NotificationService)
	c.PRService.SetNotificationService(c.NotificationService)

	// Wire agent service with queue producer
	c.AgentService.SetQueueProducer(c.QueueProducer)

	return c
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
	indexerWorker := worker.NewIndexerWorker(c.DB, c.AgentService, repository.NewEmbeddingRepository(c.DB), nil)
	if c.Cfg.Features.RAGHybridEnabled {
		indexerWorker = worker.NewIndexerWorker(c.DB, c.AgentService, repository.NewEmbeddingRepository(c.DB), c.RAGProjection)
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
