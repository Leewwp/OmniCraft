package handler

// #658 PR-3 测试组合 seam（handler 测试包内，绕开 testutil↔handler 的
// import 环）：镜像 NewAdminHandler/NewUserHandler 迁移前的构造面，
// 行为断言零改动，仅构造路径经此组装。

import (
	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/testutil/studio"
	"omnicraft/backend/internal/worker"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func newAdminHandlerForTest(db *gorm.DB, cfg *config.Config, rdb *redis.Client, auditSvc *service.AdminAuditService) *AdminHandler {
	var dlqWorker *worker.DLQWorker
	if rdb != nil {
		dlqWorker = worker.NewDLQWorker(rdb)
	}
	return NewAdminHandler(db, cfg, rdb, auditSvc, AdminDeps{
		IPAdminSvc:    service.NewIPServiceWithInvalidation(repository.NewIPRepository(db), rdb),
		UserRepo:      repository.NewUserRepository(db),
		ContentRepo:   repository.NewContentRepository(db),
		SocialRepo:    repository.NewSocialRepository(db),
		LLMConfigSvc:  service.NewLLMConfigService(repository.NewLLMConfigRepository(db), cfg),
		DLQWorker:     dlqWorker,
		DisplaySigner: service.NewDisplayURLSigner(cfg),
	})
}

func newUserHandlerForTest(db *gorm.DB, authSvc *service.AuthService, rdb *redis.Client, cfg *config.Config, reviewers ...avatarReviewer) *UserHandler {
	return NewUserHandler(
		repository.NewUserRepository(db), service.NewReputationService(db), repository.NewContentRepository(db), repository.NewFollowRepository(db),
		authSvc, rdb, cfg, reviewers...,
	)
}

// newContentHandlerForTest mirrors the pre-#658 NewContentHandler(db, cfg,
// rdb) construction face over the container-shaped deps; tests keep their
// behavior assertions untouched.
func newContentHandlerForTest(db *gorm.DB, cfg *config.Config, rdb *redis.Client) (*ContentHandler, *studio.Stack) {
	st := studio.NewStack(db, cfg, rdb)
	return NewContentHandler(cfg, ContentHandlerDeps{
		ContentService: st.ContentService,
		OSS:            st.OSS,
		OSSErr:         st.OSSErr,
		UploadGrants:   st.UploadGrants,
		ContentRepo:    repository.NewContentRepository(db),
		JudgeRepo:      repository.NewJudgeRepository(db),
		FollowRepo:     repository.NewFollowRepository(db),
		SeriesSvc:      service.NewSeriesService(repository.NewSeriesRepository(db)),
		BrowseHistory:  repository.NewBrowseHistoryRepository(db),
		CollectionRepo: repository.NewCollectionRepository(db),
		ArchiveGate:    service.NewArchiveScanGate(db, cfg.Features.ArchiveMalwareScanEnabled),
		DisplaySigner:  service.NewDisplayURLSigner(cfg),
	}), st
}

// newIPHandlerForTest mirrors the pre-#658 cached IP handler construction.
func newIPHandlerForTest(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *IPHandler {
	return NewIPHandlerWithCache(
		service.NewIPServiceWithReview(repository.NewIPRepository(db), rdb, &cfg.Cache, service.NewReviewService(db, rdb, cfg, nil)),
		repository.NewContentRepository(db),
		repository.NewDiscussionRepository(db),
		service.NewDisplayURLSigner(cfg),
		cfg,
	)
}
