package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// InvoiceRepository defines the interface for invoice database operations.
type InvoiceRepository interface {
	// Create inserts a new invoice record into the database.
	Create(ctx context.Context, invoice *model.Invoice) error
	// FindByID retrieves an invoice by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.Invoice, error)
	// FindByUserID retrieves paginated invoices for a user. Returns items, total count, and error.
	FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Invoice, int64, error)
	// FindBySubscriptionID retrieves the most recent invoice for a subscription. Returns not found error if absent.
	FindBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) (*model.Invoice, error)
	// UpdateStatus updates the status of an invoice. Sets paid_at if status is "paid".
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.InvoiceStatus) error
	// GenerateInvoiceNumber generates a unique invoice number with format "INV-YYYYMMDD-XXXXX".
	GenerateInvoiceNumber(ctx context.Context) (string, error)
}

// invoiceRepository implements InvoiceRepository using GORM.
type invoiceRepository struct {
	db *gorm.DB
}

// NewInvoiceRepository creates a new InvoiceRepository backed by the given GORM DB.
func NewInvoiceRepository(db *gorm.DB) InvoiceRepository {
	return &invoiceRepository{db: db}
}

// Create inserts a new invoice. Returns a conflict error if the invoice_number already exists.
func (r *invoiceRepository) Create(ctx context.Context, invoice *model.Invoice) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(invoice).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apierror.NewConflict("invoice with this number already exists")
		}
		return apierror.NewInternalError("failed to create invoice")
	}

	return nil
}

// FindByID looks up an invoice by its UUID primary key.
// GORM automatically filters soft-deleted records.
func (r *invoiceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Invoice, error) {
	db := database.GetDB(ctx, r.db)

	var invoice model.Invoice
	if err := db.Where("id = ?", id).First(&invoice).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("invoice not found")
		}
		return nil, apierror.NewInternalError("failed to find invoice by id")
	}

	return &invoice, nil
}

// FindByUserID retrieves paginated invoices belonging to a user, ordered by created_at DESC.
// Returns the items for the requested page and the total count of matching records.
func (r *invoiceRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Invoice, int64, error) {
	db := database.GetDB(ctx, r.db)

	var total int64
	if err := db.Model(&model.Invoice{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to count invoices")
	}

	var invoices []model.Invoice
	offset := (page - 1) * pageSize
	if err := db.Where("user_id = ?", userID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&invoices).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to find invoices by user id")
	}

	return invoices, total, nil
}

// FindBySubscriptionID retrieves the most recent invoice for a subscription, ordered by created_at DESC.
func (r *invoiceRepository) FindBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) (*model.Invoice, error) {
	db := database.GetDB(ctx, r.db)

	var invoice model.Invoice
	if err := db.Where("subscription_id = ?", subscriptionID).
		Order("created_at DESC").
		First(&invoice).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("invoice not found for subscription")
		}
		return nil, apierror.NewInternalError("failed to find invoice by subscription id")
	}

	return &invoice, nil
}

// UpdateStatus updates only the status field of an invoice.
// If the new status is "paid", paid_at is also set to the current time.
func (r *invoiceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.InvoiceStatus) error {
	db := database.GetDB(ctx, r.db)

	updates := map[string]interface{}{
		"status": status,
	}
	if status == model.InvoicePaid {
		now := time.Now()
		updates["paid_at"] = &now
	}

	result := db.Model(&model.Invoice{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return apierror.NewInternalError("failed to update invoice status")
	}
	if result.RowsAffected == 0 {
		return apierror.NewNotFound("invoice not found")
	}

	return nil
}

// GenerateInvoiceNumber generates a unique invoice number with format "INV-YYYYMMDD-XXXXX".
// It counts existing invoices for today and increments the sequence number.
func (r *invoiceRepository) GenerateInvoiceNumber(ctx context.Context) (string, error) {
	db := database.GetDB(ctx, r.db)

	dateStr := time.Now().Format("20060102")
	prefix := fmt.Sprintf("INV-%s-", dateStr)

	var count int64
	if err := db.Model(&model.Invoice{}).
		Where("invoice_number LIKE ?", prefix+"%").
		Count(&count).Error; err != nil {
		return "", apierror.NewInternalError("failed to generate invoice number")
	}

	invoiceNumber := fmt.Sprintf("INV-%s-%05d", dateStr, count+1)
	return invoiceNumber, nil
}
