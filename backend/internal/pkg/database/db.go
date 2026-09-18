package database

import (
	"log"
	"log/slog"
	"os"
	"time"

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
	// GORM's logger.Default writes to os.Stdout, which is the JSON-RPC
	// stream for stdio MCP subprocesses (mcp-docserver) — SQL logs there
	// corrupt the protocol. Every GORM log line goes to stderr instead,
	// matching the slog stderr discipline.
	gormLogger := logger.New(
		log.New(os.Stderr, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{
		Logger:                 gormLogger,
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
