// Package seed provides idempotent seed data for development environments.
// This data MUST NOT be used in production.
// All seed operations use FirstOrCreate to ensure idempotency.
package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
)

// Run executes all seed data operations. Safe to run multiple times (idempotent).
func Run(db *gorm.DB) error {
	slog.Info("seeding development data...")

	users, err := seedUsers(db)
	if err != nil {
		return fmt.Errorf("seed users: %w", err)
	}

	if err := seedAPIKeys(db, users.devUser.ID); err != nil {
		return fmt.Errorf("seed api keys: %w", err)
	}

	if err := seedTransactions(db, users.devUser.ID); err != nil {
		return fmt.Errorf("seed transactions: %w", err)
	}

	slog.Info("development seed data applied successfully")
	return nil
}

type seededUsers struct {
	adminUser *model.User
	devUser   *model.User
}

func hashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("failed to hash seed password: %v", err))
	}
	return string(hash)
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
