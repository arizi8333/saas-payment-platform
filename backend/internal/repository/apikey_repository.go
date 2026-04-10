package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// APIKeyRepository defines the interface for API key database operations.
type APIKeyRepository interface {
	// Create inserts a new API key record into the database.
	Create(ctx context.Context, apiKey *model.APIKey) error
	// FindByKeyHash retrieves an API key by its hash. Returns not found error if absent.
	FindByKeyHash(ctx context.Context, keyHash string) (*model.APIKey, error)
	// FindByUserID retrieves paginated API keys for a user. Returns items and total count.
	FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.APIKey, int64, error)
	// FindByID retrieves an API key by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.APIKey, error)
	// Update persists changes to an existing API key record.
	Update(ctx context.Context, apiKey *model.APIKey) error
	// UpdateLastUsed updates only the last_used_at field to the current time.
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}

// apiKeyRepository implements APIKeyRepository using GORM.
type apiKeyRepository struct {
	db *gorm.DB
}

// NewAPIKeyRepository creates a new APIKeyRepository backed by the given GORM DB.
func NewAPIKeyRepository(db *gorm.DB) APIKeyRepository {
	return &apiKeyRepository{db: db}
}

// Create inserts a new API key record.
func (r *apiKeyRepository) Create(ctx context.Context, apiKey *model.APIKey) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apierror.NewConflict("api key already exists")
		}
		return apierror.NewInternalError("failed to create api key")
	}

	return nil
}

// FindByKeyHash looks up an API key by its hash value.
// GORM automatically filters soft-deleted records.
func (r *apiKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*model.APIKey, error) {
	db := database.GetDB(ctx, r.db)

	var apiKey model.APIKey
	if err := db.Preload("User").Where("key_hash = ?", keyHash).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("api key not found")
		}
		return nil, apierror.NewInternalError("failed to find api key by hash")
	}

	return &apiKey, nil
}

// FindByUserID retrieves paginated API keys belonging to a user.
// Returns the items for the requested page and the total count of matching records.
func (r *apiKeyRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.APIKey, int64, error) {
	db := database.GetDB(ctx, r.db)

	var total int64
	if err := db.Model(&model.APIKey{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to count api keys")
	}

	var apiKeys []model.APIKey
	offset := (page - 1) * pageSize
	if err := db.Where("user_id = ?", userID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&apiKeys).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to find api keys by user id")
	}

	return apiKeys, total, nil
}

// FindByID looks up an API key by its UUID primary key.
// GORM automatically filters soft-deleted records.
func (r *apiKeyRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.APIKey, error) {
	db := database.GetDB(ctx, r.db)

	var apiKey model.APIKey
	if err := db.Where("id = ?", id).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("api key not found")
		}
		return nil, apierror.NewInternalError("failed to find api key by id")
	}

	return &apiKey, nil
}

// Update saves all fields of the given API key record.
func (r *apiKeyRepository) Update(ctx context.Context, apiKey *model.APIKey) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Save(apiKey).Error; err != nil {
		return apierror.NewInternalError("failed to update api key")
	}

	return nil
}

// UpdateLastUsed updates only the last_used_at field to the current time.
func (r *apiKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	db := database.GetDB(ctx, r.db)

	result := db.Model(&model.APIKey{}).Where("id = ?", id).Update("last_used_at", time.Now())
	if result.Error != nil {
		return apierror.NewInternalError("failed to update last used")
	}
	if result.RowsAffected == 0 {
		return apierror.NewNotFound("api key not found")
	}

	return nil
}
