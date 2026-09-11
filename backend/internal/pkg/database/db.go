package database

import (
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
	"gorm.io/plugin/opentelemetry/tracing"

	"omnicraft/backend/config"
)

var DB *gorm.DB

func Init(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Info),
		PrepareStmt:            false,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	if err := db.Use(tracing.NewPlugin(
		tracing.WithTracerProvider(otel.GetTracerProvider()),
		tracing.WithoutQueryVariables(),
	)); err != nil {
		slog.Error("Failed to configure database tracing", "error", err)
		os.Exit(1)
	}

	if cfg.Database.ReadDSN != "" {
		if err := db.Use(dbresolver.Register(dbresolver.Config{
			Replicas: []gorm.Dialector{postgres.Open(cfg.Database.ReadDSN)},
			Policy:   dbresolver.RandomPolicy{},
		})); err != nil {
			slog.Warn("failed to configure read replica", "error", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("Failed to get underlying sql.DB", "error", err)
		os.Exit(1)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	DB = db
	slog.Info("Database connected successfully")
	return db
}
