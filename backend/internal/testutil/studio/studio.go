package studio

// Package studio is the test-side composition seam for the studio publish
// surface (#658). It mirrors, step for step, what the service container
// wires for the HTTP and MCP write channels: one container-semantics
// ReviewService (outbox + archive gate), one shared OSS presign service,
// one shared upload-grant store, and the full-featured ContentService both
// channels consume. Tests that used to reach these constructors through
// handler constructors compose from here instead. It lives in a subpackage
// (not testutil itself) because it imports repository/service: the parent
// testutil package is imported by repository/model tests and a seam there
// would close an import cycle in those test binaries. Production code must
// not import this package.

import (
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

type Stack struct {
	ReviewService  *service.ReviewService
	ContentService *service.ContentService
	OSS            *service.OSSService
	OSSErr         error
	UploadGrants   *service.UploadGrantService
}

func NewStack(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *Stack {
	reviewSvc := service.NewReviewService(db, rdb, cfg, service.NewReputationService(db))
	reviewSvc.SetOutboxRepository(repository.NewOutboxRepository(db))
	reviewSvc.SetArchiveScanGate(service.NewArchiveScanGate(db, cfg.Features.ArchiveMalwareScanEnabled))

	ossSvc, ossErr := service.NewOSSService(cfg)

	grantTTL := time.Duration(cfg.Feedback.UploadGrantTTLSec) * time.Second
	if grantTTL <= 0 {
		grantTTL = 5 * time.Minute
	}
	uploadGrants := service.NewUploadGrantService(rdb, grantTTL)

	contentRepo := repository.NewContentRepository(db)
	contentSvc := service.NewContentServiceWithOSS(contentRepo, reviewSvc, rdb, &cfg.Cache, ossSvc).
		WithUploadGrantService(uploadGrants).
		WithUploadedObjectVerifier(ossSvc).
		WithArchiveScanConfig(&cfg.ArchiveScan).
		WithArchiveScanGateEnabled(cfg.Features.ArchiveMalwareScanEnabled).
		WithImageDimensionsResolver(ossSvc).
		WithUploadConfig(&cfg.Upload)
	contentSvc.SetVersionService(service.NewVersionService(repository.NewVersionRepository(db), contentRepo))
	contentSvc.SetOutboxRepository(repository.NewOutboxRepository(db))
	contentSvc.SetQueueProducer(queue.NewNoopProducer())
	recSvc := service.NewRecommendationService(db, repository.NewEmbeddingRepository(db), contentRepo, contentSvc, rdb, &cfg.Recommendation)
	contentSvc.SetRecommendationService(recSvc)

	return &Stack{
		ReviewService:  reviewSvc,
		ContentService: contentSvc,
		OSS:            ossSvc,
		OSSErr:         ossErr,
		UploadGrants:   uploadGrants,
	}
}
