package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/handler"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/database"
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

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RateLimit(rdb, &cfg.RateLimit))
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "omnicraft-backend"})
	})

	v1 := r.Group("/api/v1")
	handler.RegisterRoutes(v1, cfg, db, rdb)

	log.Printf("OmniCraft backend starting on port %s (mode: %s)", cfg.Server.Port, cfg.Server.Mode)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
