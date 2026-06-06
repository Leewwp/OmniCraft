package container

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/captcha"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/mail"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
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
	IPStatsService      *service.IPStatsService
	AgentService        *service.AgentService
	NotificationService *service.NotificationService
	PRService           *service.PRService
	SearchService       *service.SearchService
	FeedbackService     *service.FeedbackService
	AdminAuditService   *service.AdminAuditService
	CaptchaVerifier     captcha.CaptchaVerifier
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

	captchaVerifier := captcha.NewCaptchaVerifier(cfg.Captcha)
	c.CaptchaVerifier = captchaVerifier
	c.VerificationService = service.NewVerificationService(c.UserRepo, rdb, mailSender, cfg)

	c.IPService = service.NewIPService(c.IPRepo)
	c.ReputationService = service.NewReputationService(db)
	c.ReviewService = service.NewReviewService(db, rdb, cfg, c.ReputationService)
	c.JudgeService = service.NewJudgeService(c.JudgeRepo, c.ReputationService, cfg)
	c.ContentService = service.NewContentServiceWithOSS(c.ContentRepo, c.ReviewService, rdb, &cfg.Cache, nil)
	c.SocialService = service.NewSocialServiceWithRedis(c.SocialRepo, c.ContentRepo, c.UserRepo, cfg, rdb)
	c.IPStatsService = service.NewIPStatsService(db, rdb)
	c.NotificationService = service.NewNotificationService(c.NotificationRepo)
	c.PRService = service.NewPRService(c.PRRepo, c.VersionRepo, c.ContentRepo)
	c.SearchService = service.NewSearchService(c.SearchRepo, rdb)

	uploadGrantTTL := 300
	if cfg.Feedback.UploadGrantTTLSec > 0 {
		uploadGrantTTL = cfg.Feedback.UploadGrantTTLSec
	}
	feedbackOSSSigner, err := service.NewOSSService(cfg)
	if err != nil {
		slog.Warn("Feedback screenshot OSS presign is unavailable", "error", err)
	}
	c.FeedbackService = service.NewFeedbackService(c.FeedbackRepo, c.UserRepo, rdb, captchaVerifier, uploadGrantTTL, feedbackOSSSigner)
	c.FeedbackService.SetNotificationService(c.NotificationService)
	c.FeedbackService.SetFeedbackMailSender(feedbackMailSender)
	c.AdminAuditService = service.NewAdminAuditService(c.AdminAuditRepo, db)

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
	c.AgentService.SetQueueProducer(c.QueueProducer)

	// Wire notification service
	c.SocialService.SetNotificationService(c.NotificationService)
	c.PRService.SetNotificationService(c.NotificationService)

	// Wire agent service with queue producer
	c.AgentService.SetQueueProducer(c.QueueProducer)

	return c
}

// StartWorkers starts all queue consumers when queue is enabled.
// Returns a stop function that should be called on shutdown.
func (c *ServiceContainer) StartWorkers(ctx context.Context) func() {
	if c.QueueBroker == nil || !c.Cfg.Queue.Enabled {
		return func() {}
	}

	mgr := worker.NewWorkerManager(c.QueueBroker)

	mgr.Register("content.review", "omnicraft-content-review", worker.NewReviewWorker(c.ReviewService).Handle)
	mgr.Register("ip.review", "omnicraft-ip-review", worker.NewReviewWorker(c.ReviewService).Handle)
	mgr.Register("notification.create", "omnicraft-notification", worker.NewNotificationWorker(c.NotificationRepo).Handle)
	mgr.Register("count.download", "omnicraft-count", worker.NewCountWorker(c.RDB).Handle)
	mgr.Register("content.embedding", "omnicraft-embedding", worker.NewEmbeddingWorker(c.AgentService).Handle)

	if err := mgr.Start(ctx); err != nil {
		slog.Error("Failed to start workers", "error", err)
	}

	return func() {
		mgr.Stop()
	}
}
