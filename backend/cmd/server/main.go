package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/handler"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/pkg/database"
	"omnicraft/backend/internal/pkg/recovery"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/pkg/scheduler"
	"omnicraft/backend/internal/service"
)

func main() {
	cfg := config.Load()
	if err := cfg.ValidateRelease(); err != nil {
		slog.Error("invalid release configuration", "error", err)
		os.Exit(1)
	}

	gin.SetMode(cfg.Server.Mode)

	db := database.Init(cfg)
	rdb := redisclient.Init(cfg)

	ctr := container.NewContainer(db, rdb, cfg)

	scheduler.NewJudgeQuestionSync(db).Start()
	scheduler.NewTagUsageSync(db).Start()
	scheduler.NewViewCountSync(ctr.ContentService, &cfg.Cache).Start()
	scheduler.NewDownloadCountSync(ctr.ContentService, &cfg.Cache).Start()
	browseHistoryCleanup := scheduler.NewBrowseHistoryCleanup(db, &cfg.BrowseHistory)
	browseHistoryCleanup.Start()

	hotRankSvc := service.NewHotRankService(ctr.ContentService, &cfg.Recommendation).
		WithRecommendationService(ctr.RecommendationSvc).
		WithIPStatsService(ctr.IPStatsService)
	recovery.GoSafe(func() {
		hotRankSvc.Run()
	})

	// Start queue workers if enabled
	stopWorkers := ctr.StartWorkers(context.Background())

	r := gin.New()
	if len(cfg.Security.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.Security.TrustedProxies); err != nil {
			slog.Error("invalid trusted proxies", "error", err)
			os.Exit(1)
		}
	} else if cfg.Server.Mode == "release" {
		_ = r.SetTrustedProxies(nil)
	}
	bodyLimit := resolveJSONBodyLimit(cfg)
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.BodyLimit(bodyLimit))
	r.Use(middleware.CSRF(cfg))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RateLimit(rdb, &cfg.RateLimit))
	r.Use(middleware.PanicRecovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "omnicraft-backend"})
	})

	v1 := r.Group("/api/v1")
	handler.RegisterRoutes(v1, cfg, ctr)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("OmniCraft backend starting", "port", cfg.Server.Port, "mode", cfg.Server.Mode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	stopWorkers()
	browseHistoryCleanup.Stop()

	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeout) * time.Second
	if shutdownTimeout <= 0 {
		shutdownTimeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}
	if rdb != nil {
		rdb.Close()
	}

	slog.Info("Server exited")
}

func resolveJSONBodyLimit(cfg *config.Config) int64 {
	const fallbackTextLimitBytes int64 = 10 * 1024 * 1024
	if cfg == nil {
		return fallbackTextLimitBytes
	}
	bodyLimit := cfg.RateLimit.MaxJSONBodyBytes
	textLimit := int64(cfg.Limits.TextMaxMB) * 1024 * 1024
	if textLimit <= 0 {
		textLimit = fallbackTextLimitBytes
	}
	if bodyLimit < textLimit {
		return textLimit
	}
	return bodyLimit
}
