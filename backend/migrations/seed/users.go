package seed

import (
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
)

// Fixed UUIDs for development seed data — ensures idempotency across runs.
var (
	adminUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	devUserID   = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

// seedUsers creates an admin user and a sample developer user.
// Uses FirstOrCreate keyed on email for idempotency.
// NOTE: These are DEVELOPMENT-ONLY credentials. Do NOT use in production.
func seedUsers(db *gorm.DB) (*seededUsers, error) {
	// Development-only passwords — hashed with bcrypt immediately.
	adminHash := hashPassword("admin1234dev")
	devHash := hashPassword("dev1234pass")

	admin := model.User{
		ID:           adminUserID,
		Email:        "admin@dev.local",
		PasswordHash: adminHash,
		FullName:     "Dev Admin",
		Role:         model.RoleAdmin,
		IsActive:     true,
	}

	dev := model.User{
		ID:           devUserID,
		Email:        "developer@dev.local",
		PasswordHash: devHash,
		FullName:     "Sample Developer",
		Role:         model.RoleDeveloper,
		IsActive:     true,
	}

	if err := db.Where("email = ?", admin.Email).FirstOrCreate(&admin).Error; err != nil {
		return nil, err
	}
	slog.Info("seed user", "email", admin.Email, "role", admin.Role)

	if err := db.Where("email = ?", dev.Email).FirstOrCreate(&dev).Error; err != nil {
		return nil, err
	}
	slog.Info("seed user", "email", dev.Email, "role", dev.Role)

	return &seededUsers{adminUser: &admin, devUser: &dev}, nil
}
