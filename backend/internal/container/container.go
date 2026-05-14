package container

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type ServiceContainer struct {
	DB  *gorm.DB
	RDB *redis.Client
	Cfg *config.Config

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

	// Services
	AuthService         *service.AuthService
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
}

func NewContainer(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *ServiceContainer {
	c := &ServiceContainer{
		DB:  db,
		RDB: rdb,
		Cfg: cfg,
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

	// Services
	c.AuthService = service.NewAuthService(c.UserRepo, rdb, cfg)
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

	// Wire recommendation into content service
	c.RecommendationSvc = service.NewRecommendationService(db, c.EmbeddingRepo, c.ContentRepo, c.ContentService, rdb, &cfg.Recommendation)
	c.ContentService.SetRecommendationService(c.RecommendationSvc)

	// Wire notification service
	c.SocialService.SetNotificationService(c.NotificationService)
	c.PRService.SetNotificationService(c.NotificationService)

	return c
}
