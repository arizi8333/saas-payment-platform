package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/saas-payment-platform/backend/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewPostgresDB creates a new GORM database connection with connection pooling
// configured from the provided DatabaseConfig.
func NewPostgresDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	slog.Info("connecting to PostgreSQL",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.DBName,
		"ssl_mode", cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)

	slog.Info("PostgreSQL connection pool configured",
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
		"max_lifetime", cfg.MaxLifetime,
	)

	return db, nil
}

// PostgresHealthCheck pings the PostgreSQL database and returns an error if unreachable.
func PostgresHealthCheck(db *gorm.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("postgres health check failed: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres health check failed: %w", err)
	}

	return nil
}
