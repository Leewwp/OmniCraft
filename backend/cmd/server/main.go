package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/pkg/database"
	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/pkg/recovery"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/pkg/scheduler"
	"omnicraft/backend/internal/router"
	"omnicraft/backend/internal/service"
)

func main() {
	cfg := config.Load()

	logger, err := observability.NewLogger(*cfg)
	if err != nil {
		slog.Error("invalid observability configuration", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)

	if err := cfg.ValidateRelease(); err != nil {
		logger.Error("invalid release configuration", "error", err)
		os.Exit(1)
	}

	ipHasher, err := observability.NewIPHasher(cfg.Observability)
	if err != nil {
		logger.Error("invalid client IP hasher configuration", "error", err)
		os.Exit(1)
	}

	gin.SetMode(cfg.Server.Mode)

	db := database.Init(cfg)
	rdb := redisclient.Init(cfg)

	metrics := observability.NewMetrics()
	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("failed to access underlying database handle", "error", err)
		os.Exit(1)
	}
	metrics.Registry.MustRegister(observability.NewDatabaseCollector(sqlDB))
	if rdb != nil {
		metrics.Registry.MustRegister(observability.NewRedisClientCollector(rdb))
	}
	observability.SetDefaultMetrics(metrics)
	queue.SetMetricsHooks(observability.SetDefaultQueueBacklog, observability.IncDefaultWorkerFailures)

	ctr := container.NewContainer(db, rdb, cfg)

	scheduler.NewJudgeQuestionSync(db).Start()
	scheduler.NewTagUsageSync(db).Start()
	scheduler.NewViewCountSync(ctr.ContentService, &cfg.Cache).Start()
	scheduler.NewDownloadCountSync(ctr.ContentService, &cfg.Cache).Start()
	browseHistoryCleanup := scheduler.NewBrowseHistoryCleanup(db, &cfg.BrowseHistory)
	browseHistoryCleanup.Start()
	collabInviteExpiry := scheduler.NewCollabInviteExpiry(db, &cfg.Collaboration)
	collabInviteExpiry.Start()

	hotRankSvc := service.NewHotRankService(ctr.ContentService, &cfg.Recommendation).
		WithRecommendationService(ctr.RecommendationSvc).
		WithIPStatsService(ctr.IPStatsService)
	recovery.GoSafe(func() {
		hotRankSvc.Run()
	})

	ready := buildReadinessCheck(cfg, sqlDB, rdb)
	obsServer := observability.NewServer(metrics.Registry, ready, time.Duration(cfg.Observability.ReadHeaderTimeoutSec)*time.Second)
	go func() {
		if err := obsServer.Serve(":" + cfg.Observability.MetricsPort); err != nil {
			logger.Error("observability server failed", "error", err)
			os.Exit(1)
		}
	}()

	r := gin.New()
	if len(cfg.Security.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.Security.TrustedProxies); err != nil {
			logger.Error("invalid trusted proxies", "error", err)
			os.Exit(1)
		}
	} else if cfg.Server.Mode == "release" {
		_ = r.SetTrustedProxies(nil)
	}
	bodyLimit := resolveJSONBodyLimit(cfg)
	r.Use(middleware.RequestID())
	r.Use(middleware.Metrics(metrics))
	r.Use(middleware.Logger(logger, ipHasher))
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.BodyLimit(bodyLimit))
	r.Use(middleware.CSRF(cfg))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RateLimit(rdb, &cfg.RateLimit))
	r.Use(middleware.PanicRecovery(logger, metrics))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "omnicraft-backend"})
	})

	v1 := r.Group("/api/v1")
	router.RegisterRoutes(v1, cfg, ctr)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		logger.Info("OmniCraft backend starting", "port", cfg.Server.Port, "mode", cfg.Server.Mode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	browseHistoryCleanup.Stop()
	collabInviteExpiry.Stop()

	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeout) * time.Second
	if shutdownTimeout <= 0 {
		shutdownTimeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	if rdb != nil {
		rdb.Close()
	}

	logger.Info("Server exited")
}

// buildReadinessCheck returns a dependency-aware readiness probe. The probe
// checks PostgreSQL and Redis with bounded timeouts and reports a single
// opaque error; connection details never leave the process.
func buildReadinessCheck(cfg *config.Config, sqlDB *sql.DB, rdb *redis.Client) func() error {
	return func() error {
		dbTimeout := time.Duration(cfg.Observability.Readiness.DBTimeoutSec) * time.Second
		if dbTimeout <= 0 {
			return errors.New("readiness timeout is not configured")
		}
		redisTimeout := time.Duration(cfg.Observability.Readiness.RedisTimeoutSec) * time.Second
		if redisTimeout <= 0 {
			return errors.New("readiness timeout is not configured")
		}

		dbCtx, dbCancel := context.WithTimeout(context.Background(), dbTimeout)
		defer dbCancel()
		if err := sqlDB.PingContext(dbCtx); err != nil {
			return errors.New("dependency unavailable")
		}

		if rdb != nil {
			redisCtx, redisCancel := context.WithTimeout(context.Background(), redisTimeout)
			defer redisCancel()
			if err := rdb.Ping(redisCtx).Err(); err != nil {
				return errors.New("dependency unavailable")
			}
		}
		return nil
	}
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
