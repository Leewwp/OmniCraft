package redisclient

import (
	"context"
	"log/slog"
	"os"

	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
)

var Client *redis.Client

func Init(cfg *config.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}

	Client = client
	slog.Info("Redis connected successfully")
	return client
}