package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/handler"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/database"
	"omnicraft/backend/internal/pkg/llm"
	"omnicraft/backend/internal/pkg/recovery"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/pkg/scheduler"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func main() {
	cfg := config.Load()

	gin.SetMode(cfg.Server.Mode)

	db := database.Init(cfg)
	rdb := redisclient.Init(cfg)

	scheduler.NewJudgeQuestionSync(db).Start()
	scheduler.NewTagUsageSync(db).Start()

	contentRepo := repository.NewContentRepository(db)
	viewCountSvc := service.NewContentServiceWithCache(contentRepo, nil, rdb, &cfg.Cache)
	scheduler.NewViewCountSync(viewCountSvc, &cfg.Cache).Start()

	embeddingRepo := repository.NewEmbeddingRepository(db)
	recSvc := service.NewRecommendationService(db, embeddingRepo, contentRepo, viewCountSvc, rdb, &cfg.Recommendation)
	embedProv := llm.NewProvider(cfg)

	ipStatsSvc := service.NewIPStatsService(db, rdb)

	hotRankSvc := service.NewHotRankService(viewCountSvc, &cfg.Recommendation).
		WithRecommendationService(recSvc).
		WithEmbeddingProvider(embedProv).
		WithIPStatsService(ipStatsSvc)
recovery.GoSafe(func() {
		hotRankSvc.Run()
	})

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RateLimit(rdb, &cfg.RateLimit))
	r.Use(middleware.PanicRecovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "omnicraft-backend"})
	})

	v1 := r.Group("/api/v1")
	handler.RegisterRoutes(v1, cfg, db, rdb)

	slog.Info("OmniCraft backend starting", "port", cfg.Server.Port, "mode", cfg.Server.Mode)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
