package repository

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// WebhookEndpointRepository defines the interface for webhook endpoint database operations.
type WebhookEndpointRepository interface {
	// Create inserts a new webhook endpoint record into the database.
	Create(ctx context.Context, endpoint *model.WebhookEndpoint) error
	// FindByUserID retrieves paginated webhook endpoints for a user. Returns items, total count, and error.
	FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.WebhookEndpoint, int64, error)
	// FindByID retrieves a webhook endpoint by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.WebhookEndpoint, error)
	// Update persists changes to an existing webhook endpoint record.
	Update(ctx context.Context, endpoint *model.WebhookEndpoint) error
	// Delete soft-deletes a webhook endpoint by ID.
	Delete(ctx context.Context, id uuid.UUID) error
	// FindActiveByUserIDAndEvent retrieves active endpoints for a user that subscribe to a given event type.
	FindActiveByUserIDAndEvent(ctx context.Context, userID uuid.UUID, eventType string) ([]model.WebhookEndpoint, error)
}

// WebhookDeliveryRepository defines the interface for webhook delivery database operations.
type WebhookDeliveryRepository interface {
	// Create inserts a new webhook delivery record into the database.
	Create(ctx context.Context, delivery *model.WebhookDelivery) error
	// FindPendingDeliveries retrieves deliveries with status "pending" and next_retry_at <= now (or null).
	FindPendingDeliveries(ctx context.Context, limit int) ([]model.WebhookDelivery, error)
	// UpdateStatus updates the status of a delivery. Sets delivered_at if status is "delivered".
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.DeliveryStatus, responseCode *int, responseBody *string) error
	// IncrementRetry increments retry_count and sets next_retry_at based on exponential backoff.
	IncrementRetry(ctx context.Context, id uuid.UUID) error
}

// webhookEndpointRepository implements WebhookEndpointRepository using GORM.
type webhookEndpointRepository struct {
	db *gorm.DB
}

// NewWebhookEndpointRepository creates a new WebhookEndpointRepository backed by the given GORM DB.
func NewWebhookEndpointRepository(db *gorm.DB) WebhookEndpointRepository {
	return &webhookEndpointRepository{db: db}
}

// Create inserts a new webhook endpoint.
func (r *webhookEndpointRepository) Create(ctx context.Context, endpoint *model.WebhookEndpoint) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(endpoint).Error; err != nil {
		return apierror.NewInternalError("failed to create webhook endpoint")
	}

	return nil
}

// FindByUserID retrieves paginated webhook endpoints belonging to a user, ordered by created_at DESC.
// GORM automatically filters soft-deleted records.
func (r *webhookEndpointRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.WebhookEndpoint, int64, error) {
	db := database.GetDB(ctx, r.db)

	var total int64
	if err := db.Model(&model.WebhookEndpoint{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to count webhook endpoints")
	}

	var endpoints []model.WebhookEndpoint
	offset := (page - 1) * pageSize
	if err := db.Where("user_id = ?", userID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&endpoints).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to find webhook endpoints by user id")
	}

	return endpoints, total, nil
}

// FindByID looks up a webhook endpoint by its UUID primary key.
// GORM automatically filters soft-deleted records.
func (r *webhookEndpointRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.WebhookEndpoint, error) {
	db := database.GetDB(ctx, r.db)

	var endpoint model.WebhookEndpoint
	if err := db.Where("id = ?", id).First(&endpoint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("webhook endpoint not found")
		}
		return nil, apierror.NewInternalError("failed to find webhook endpoint by id")
	}

	return &endpoint, nil
}

// Update saves all fields of the given webhook endpoint record.
func (r *webhookEndpointRepository) Update(ctx context.Context, endpoint *model.WebhookEndpoint) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Save(endpoint).Error; err != nil {
		return apierror.NewInternalError("failed to update webhook endpoint")
	}

	return nil
}

// Delete soft-deletes a webhook endpoint by ID.
func (r *webhookEndpointRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := database.GetDB(ctx, r.db)

	result := db.Delete(&model.WebhookEndpoint{}, "id = ?", id)
	if result.Error != nil {
		return apierror.NewInternalError("failed to delete webhook endpoint")
	}
	if result.RowsAffected == 0 {
		return apierror.NewNotFound("webhook endpoint not found")
	}

	return nil
}

// FindActiveByUserIDAndEvent retrieves active (non-deleted, is_active=true) endpoints
// for a user. Event filtering is done at the service layer since events is stored as JSONB.
func (r *webhookEndpointRepository) FindActiveByUserIDAndEvent(ctx context.Context, userID uuid.UUID, eventType string) ([]model.WebhookEndpoint, error) {
	db := database.GetDB(ctx, r.db)

	var endpoints []model.WebhookEndpoint
	if err := db.Where("user_id = ? AND is_active = ?", userID, true).
		Find(&endpoints).Error; err != nil {
		return nil, apierror.NewInternalError("failed to find active webhook endpoints")
	}

	return endpoints, nil
}

// webhookDeliveryRepository implements WebhookDeliveryRepository using GORM.
type webhookDeliveryRepository struct {
	db *gorm.DB
}

// NewWebhookDeliveryRepository creates a new WebhookDeliveryRepository backed by the given GORM DB.
func NewWebhookDeliveryRepository(db *gorm.DB) WebhookDeliveryRepository {
	return &webhookDeliveryRepository{db: db}
}

// Create inserts a new webhook delivery record.
func (r *webhookDeliveryRepository) Create(ctx context.Context, delivery *model.WebhookDelivery) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(delivery).Error; err != nil {
		return apierror.NewInternalError("failed to create webhook delivery")
	}

	return nil
}

// FindPendingDeliveries retrieves deliveries with status "pending" where
// next_retry_at is null (first attempt) or next_retry_at <= now (ready for retry).
func (r *webhookDeliveryRepository) FindPendingDeliveries(ctx context.Context, limit int) ([]model.WebhookDelivery, error) {
	db := database.GetDB(ctx, r.db)

	var deliveries []model.WebhookDelivery
	now := time.Now()
	if err := db.Where("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", model.DeliveryPending, now).
		Order("created_at ASC").
		Limit(limit).
		Find(&deliveries).Error; err != nil {
		return nil, apierror.NewInternalError("failed to find pending webhook deliveries")
	}

	return deliveries, nil
}

// UpdateStatus updates the status of a delivery record.
// If the new status is "delivered", delivered_at is set to the current time.
func (r *webhookDeliveryRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.DeliveryStatus, responseCode *int, responseBody *string) error {
	db := database.GetDB(ctx, r.db)

	updates := map[string]interface{}{
		"status":        status,
		"response_code": responseCode,
		"response_body": responseBody,
	}
	if status == model.DeliveryDelivered {
		now := time.Now()
		updates["delivered_at"] = &now
	}

	result := db.Model(&model.WebhookDelivery{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return apierror.NewInternalError("failed to update webhook delivery status")
	}
	if result.RowsAffected == 0 {
		return apierror.NewNotFound("webhook delivery not found")
	}

	return nil
}

// IncrementRetry increments retry_count and sets next_retry_at using exponential backoff.
// Backoff formula: 2^retry_count seconds (1s, 2s, 4s, 8s, 16s, ...).
func (r *webhookDeliveryRepository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	db := database.GetDB(ctx, r.db)

	// First fetch the current retry_count to calculate backoff.
	var delivery model.WebhookDelivery
	if err := db.Where("id = ?", id).First(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apierror.NewNotFound("webhook delivery not found")
		}
		return apierror.NewInternalError("failed to find webhook delivery")
	}

	newRetryCount := delivery.RetryCount + 1
	backoffSeconds := math.Pow(2, float64(delivery.RetryCount)) // 1s, 2s, 4s, 8s, 16s
	nextRetry := time.Now().Add(time.Duration(backoffSeconds) * time.Second)

	result := db.Model(&model.WebhookDelivery{}).Where("id = ?", id).Updates(map[string]interface{}{
		"retry_count":   newRetryCount,
		"next_retry_at": &nextRetry,
	})
	if result.Error != nil {
		return apierror.NewInternalError("failed to increment webhook delivery retry")
	}

	return nil
}
