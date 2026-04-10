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

// TransactionRepository defines the interface for transaction database operations.
type TransactionRepository interface {
	// Create inserts a new transaction record into the database.
	Create(ctx context.Context, tx *model.Transaction) error
	// FindByID retrieves a transaction by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	// FindByUserID retrieves paginated transactions for a user. Returns items, total count, and error.
	FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Transaction, int64, error)
	// FindByExternalID retrieves a transaction by its external ID. Returns not found error if absent.
	FindByExternalID(ctx context.Context, externalID string) (*model.Transaction, error)
	// UpdateStatus updates only the status field of a transaction. Sets paid_at if status is "success".
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.TransactionStatus) error
}

// transactionRepository implements TransactionRepository using GORM.
type transactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new TransactionRepository backed by the given GORM DB.
func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Create inserts a new transaction. Returns a conflict error if the external_id already exists.
func (r *transactionRepository) Create(ctx context.Context, tx *model.Transaction) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(tx).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apierror.NewConflict("transaction with this external_id already exists")
		}
		return apierror.NewInternalError("failed to create transaction")
	}

	return nil
}

// FindByID looks up a transaction by its UUID primary key.
func (r *transactionRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	db := database.GetDB(ctx, r.db)

	var transaction model.Transaction
	if err := db.Where("id = ?", id).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("transaction not found")
		}
		return nil, apierror.NewInternalError("failed to find transaction by id")
	}

	return &transaction, nil
}

// FindByUserID retrieves paginated transactions belonging to a user, ordered by created_at DESC.
// Returns the items for the requested page and the total count of matching records.
func (r *transactionRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Transaction, int64, error) {
	db := database.GetDB(ctx, r.db)

	var total int64
	if err := db.Model(&model.Transaction{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to count transactions")
	}

	var transactions []model.Transaction
	offset := (page - 1) * pageSize
	if err := db.Where("user_id = ?", userID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&transactions).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to find transactions by user id")
	}

	return transactions, total, nil
}

// FindByExternalID looks up a transaction by its external ID.
func (r *transactionRepository) FindByExternalID(ctx context.Context, externalID string) (*model.Transaction, error) {
	db := database.GetDB(ctx, r.db)

	var transaction model.Transaction
	if err := db.Where("external_id = ?", externalID).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("transaction not found")
		}
		return nil, apierror.NewInternalError("failed to find transaction by external id")
	}

	return &transaction, nil
}

// UpdateStatus updates only the status field of a transaction.
// If the new status is "success", paid_at is also set to the current time.
func (r *transactionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TransactionStatus) error {
	db := database.GetDB(ctx, r.db)

	updates := map[string]interface{}{
		"status": status,
	}
	if status == model.TxStatusSuccess {
		now := time.Now()
		updates["paid_at"] = &now
	}

	result := db.Model(&model.Transaction{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return apierror.NewInternalError("failed to update transaction status")
	}
	if result.RowsAffected == 0 {
		return apierror.NewNotFound("transaction not found")
	}

	return nil
}
