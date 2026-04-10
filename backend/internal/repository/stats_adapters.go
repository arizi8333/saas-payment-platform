package repository

import (
	"context"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
	"gorm.io/gorm"
)

// --- Transaction Stats Adapter ---

// TransactionStatsAdapter wraps a GORM DB to satisfy service.TransactionStatsRepository.
type TransactionStatsAdapter struct {
	db *gorm.DB
}

// NewTransactionStatsAdapter creates a new TransactionStatsAdapter.
func NewTransactionStatsAdapter(db *gorm.DB) *TransactionStatsAdapter {
	return &TransactionStatsAdapter{db: db}
}

// CountByStatus returns the count of transactions with the given status.
func (a *TransactionStatsAdapter) CountByStatus(ctx context.Context, status model.TransactionStatus) (int64, error) {
	db := database.GetDB(ctx, a.db)
	var count int64
	if err := db.Model(&model.Transaction{}).Where("status = ?", status).Count(&count).Error; err != nil {
		return 0, apierror.NewInternalError("failed to count transactions by status")
	}
	return count, nil
}

// CountAll returns the total count of transactions.
func (a *TransactionStatsAdapter) CountAll(ctx context.Context) (int64, error) {
	db := database.GetDB(ctx, a.db)
	var count int64
	if err := db.Model(&model.Transaction{}).Count(&count).Error; err != nil {
		return 0, apierror.NewInternalError("failed to count all transactions")
	}
	return count, nil
}


// --- User Stats Adapter ---

// UserStatsAdapter wraps a GORM DB to satisfy service.UserStatsRepository.
type UserStatsAdapter struct {
	db *gorm.DB
}

// NewUserStatsAdapter creates a new UserStatsAdapter.
func NewUserStatsAdapter(db *gorm.DB) *UserStatsAdapter {
	return &UserStatsAdapter{db: db}
}

// CountActive returns the count of active users (is_active = true, not soft-deleted).
func (a *UserStatsAdapter) CountActive(ctx context.Context) (int64, error) {
	db := database.GetDB(ctx, a.db)
	var count int64
	if err := db.Model(&model.User{}).Where("is_active = ?", true).Count(&count).Error; err != nil {
		return 0, apierror.NewInternalError("failed to count active users")
	}
	return count, nil
}

// --- Webhook Delivery Stats Adapter ---

// WebhookDeliveryStatsAdapter wraps a GORM DB to satisfy service.WebhookDeliveryStatsRepository.
type WebhookDeliveryStatsAdapter struct {
	db *gorm.DB
}

// NewWebhookDeliveryStatsAdapter creates a new WebhookDeliveryStatsAdapter.
func NewWebhookDeliveryStatsAdapter(db *gorm.DB) *WebhookDeliveryStatsAdapter {
	return &WebhookDeliveryStatsAdapter{db: db}
}

// CountByStatus returns the count of webhook deliveries with the given status.
func (a *WebhookDeliveryStatsAdapter) CountByStatus(ctx context.Context, status model.DeliveryStatus) (int64, error) {
	db := database.GetDB(ctx, a.db)
	var count int64
	if err := db.Model(&model.WebhookDelivery{}).Where("status = ?", status).Count(&count).Error; err != nil {
		return 0, apierror.NewInternalError("failed to count webhook deliveries by status")
	}
	return count, nil
}

// CountAll returns the total count of webhook deliveries.
func (a *WebhookDeliveryStatsAdapter) CountAll(ctx context.Context) (int64, error) {
	db := database.GetDB(ctx, a.db)
	var count int64
	if err := db.Model(&model.WebhookDelivery{}).Count(&count).Error; err != nil {
		return 0, apierror.NewInternalError("failed to count all webhook deliveries")
	}
	return count, nil
}
