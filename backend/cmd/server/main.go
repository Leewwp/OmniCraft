package main

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/handler"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/database"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/pkg/scheduler"
	"omnicraft/backend/internal/service"
)

func main() {
	cfg := config.Load()

	gin.SetMode(cfg.Server.Mode)

	db := database.Init(cfg)
	rdb := redisclient.Init(cfg)

	ctr := container.NewContainer(db, rdb, cfg)

	scheduler.NewJudgeQuestionSync(db).Start()
	scheduler.NewTagUsageSync(db).Start()
	scheduler.NewViewCountSync(ctr.ContentService, &cfg.Cache).Start()

	hotRankSvc := service.NewHotRankService(ctr.ContentService, &cfg.Recommendation).
		WithRecommendationService(ctr.RecommendationSvc).
		WithIPStatsService(ctr.IPStatsService)
	recovery.GoSafe(func() {
		hotRankSvc.Run()
	})

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.CSRF(cfg))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RateLimit(rdb, &cfg.RateLimit))
	r.Use(middleware.PanicRecovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "omnicraft-backend"})
	})

	v1 := r.Group("/api/v1")
	handler.RegisterRoutes(v1, cfg, ctr)

	slog.Info("OmniCraft backend starting", "port", cfg.Server.Port, "mode", cfg.Server.Mode)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
