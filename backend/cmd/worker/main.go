package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/observability"
	"omnicraft/backend/internal/pkg/database"
	redisclient "omnicraft/backend/internal/pkg/redis"
)

// main is the standalone worker process (ADR 0005, issue #138): it is the
// only place async consumers and the outbox relay run. The API server never
// starts workers; stopping this process pauses consumption without touching
// the REST/SSE surface, and events keep accumulating in the outbox/streams.
func main() {
	cfg := config.Load()

	logger, err := observability.NewLogger(*cfg)
	if err != nil {
		slog.Error("invalid observability configuration", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)

	tracerProvider, err := observability.NewTracerProvider(context.Background(), observability.TracingConfig{
		Enabled: cfg.Observability.Tracing.Enabled, Endpoint: cfg.Observability.Tracing.Endpoint,
		SampleRatio: cfg.Observability.Tracing.SampleRatio, Backend: cfg.Observability.Tracing.Backend,
	})
	if err != nil {
		logger.Error("invalid tracing configuration", "error", err)
		os.Exit(1)
	}
	shutdownTracing := observability.InstallTracerProvider(tracerProvider)
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			logger.Warn("trace provider shutdown failed", "error", err)
		}
	}()

	if !cfg.Worker.Enabled {
		logger.Info("worker is disabled in config, exiting")
		return
	}

	db := database.Init(cfg)
	rdb := redisclient.Init(cfg)

	ctr := container.NewContainer(db, rdb, cfg)
	stopWorkers := ctr.StartWorkers(context.Background())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down worker...")

	stopWorkers()
	rdb.Close()

	logger.Info("Worker exited")
}
