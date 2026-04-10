package main

import (
	"log/slog"
	"os"

	"github.com/saas-payment-platform/backend/internal/config"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/pkg/logger"
	"github.com/saas-payment-platform/backend/migrations/seed"
)

func main() {
	logger.Setup()

	slog.Info("starting seed data process")

	cfg := config.Load()

	db, err := database.NewPostgresDB(&cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	if err := seed.Run(db); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}

	slog.Info("seed data completed successfully")
}
