package seed

import (
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
)

var (
	seedAPIKeyID1 = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	seedAPIKeyID2 = uuid.MustParse("00000000-0000-0000-0000-000000000011")
)

// seedAPIKeys creates sample API keys for the developer user.
// Uses FirstOrCreate keyed on key_hash for idempotency.
// NOTE: These are DEVELOPMENT-ONLY keys. Do NOT use in production.
func seedAPIKeys(db *gorm.DB, devUserID uuid.UUID) error {
	keys := []model.APIKey{
		{
			ID:        seedAPIKeyID1,
			UserID:    devUserID,
			Name:      "Dev Primary Key",
			KeyHash:   hashAPIKey("sk_test_dev_primary_key_001"),
			KeyPrefix: "sk_test_dev_",
			IsActive:  true,
			RateLimit: 1000,
		},
		{
			ID:        seedAPIKeyID2,
			UserID:    devUserID,
			Name:      "Dev Secondary Key",
			KeyHash:   hashAPIKey("sk_test_dev_secondary_key_002"),
			KeyPrefix: "sk_test_dev_",
			IsActive:  true,
			RateLimit: 500,
		},
	}

	for _, key := range keys {
		if err := db.Where("key_hash = ?", key.KeyHash).FirstOrCreate(&key).Error; err != nil {
			return err
		}
		slog.Info("seed api key", "name", key.Name, "prefix", key.KeyPrefix)
	}

	return nil
}
