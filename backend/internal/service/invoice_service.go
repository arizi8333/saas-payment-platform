package service

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/repository"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// InvoiceService defines the interface for invoice business logic operations.
type InvoiceService interface {
	// CreateInvoice creates a new invoice linked to a Transaction or Subscription.
	CreateInvoice(ctx context.Context, req *CreateInvoiceInput) (*dto.InvoiceResponse, error)
	// UpdateStatus transitions an invoice status (unpaid → paid or unpaid → void).
	UpdateStatus(ctx context.Context, invoiceID uuid.UUID, newStatus model.InvoiceStatus) (*dto.InvoiceResponse, error)
	// UpdateStatusBySubscriptionID finds the latest invoice for a subscription and updates its status.
	UpdateStatusBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID, newStatus model.InvoiceStatus) (*dto.InvoiceResponse, error)
	// GetInvoice retrieves a single invoice by ID.
	GetInvoice(ctx context.Context, invoiceID uuid.UUID) (*dto.InvoiceResponse, error)
	// ListInvoices returns paginated invoices for a user.
	ListInvoices(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.InvoiceListResponse, error)
}

// CreateInvoiceInput is the internal input for creating an invoice.
// This is not an HTTP DTO — invoices are created programmatically by other services.
type CreateInvoiceInput struct {
	UserID         uuid.UUID
	TransactionID  *uuid.UUID
	SubscriptionID *uuid.UUID
	Amount         int64
	Currency       string
	DueDate        time.Time
}

// invoiceService implements InvoiceService.
type invoiceService struct {
	invoiceRepo repository.InvoiceRepository
	txManager   database.TransactionManager
}

// NewInvoiceService creates a new InvoiceService with all required dependencies.
func NewInvoiceService(
	invoiceRepo repository.InvoiceRepository,
	txManager database.TransactionManager,
) InvoiceService {
	return &invoiceService{
		invoiceRepo: invoiceRepo,
		txManager:   txManager,
	}
}

// CreateInvoice generates a unique invoice number, validates the input,
// and creates a new invoice linked to a Transaction or Subscription.
func (s *invoiceService) CreateInvoice(ctx context.Context, req *CreateInvoiceInput) (*dto.InvoiceResponse, error) {
	// Validate amount.
	if req.Amount <= 0 {
		return nil, apierror.NewBadRequest("invoice amount must be greater than 0")
	}

	// Default currency to IDR if not specified.
	currency := req.Currency
	if currency == "" {
		currency = "IDR"
	}

	// Generate unique invoice number via repository.
	invoiceNumber, err := s.invoiceRepo.GenerateInvoiceNumber(ctx)
	if err != nil {
		return nil, err
	}

	invoice := &model.Invoice{
		UserID:         req.UserID,
		TransactionID:  req.TransactionID,
		SubscriptionID: req.SubscriptionID,
		InvoiceNumber:  invoiceNumber,
		Amount:         req.Amount,
		Currency:       currency,
		Status:         model.InvoiceUnpaid,
		DueDate:        req.DueDate,
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.invoiceRepo.Create(txCtx, invoice)
	})
	if err != nil {
		return nil, err
	}

	return toInvoiceResponse(invoice), nil
}

// UpdateStatus transitions an invoice from unpaid to paid or void.
// Only unpaid invoices can be transitioned. When marking as paid, paid_at is recorded.
func (s *invoiceService) UpdateStatus(ctx context.Context, invoiceID uuid.UUID, newStatus model.InvoiceStatus) (*dto.InvoiceResponse, error) {
	// Fetch the current invoice.
	invoice, err := s.invoiceRepo.FindByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	// Validate status transition: only unpaid → paid or unpaid → void.
	if invoice.Status != model.InvoiceUnpaid {
		return nil, apierror.NewBadRequest("only unpaid invoices can be updated")
	}
	if newStatus != model.InvoicePaid && newStatus != model.InvoiceVoid {
		return nil, apierror.NewBadRequest("invoice status can only be changed to paid or void")
	}

	// Perform the status update (repository handles paid_at for paid status).
	if err := s.invoiceRepo.UpdateStatus(ctx, invoiceID, newStatus); err != nil {
		return nil, err
	}

	// Re-fetch to get the updated record (including paid_at if set).
	updated, err := s.invoiceRepo.FindByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	return toInvoiceResponse(updated), nil
}

// UpdateStatusBySubscriptionID finds the latest invoice for a subscription and updates its status.
func (s *invoiceService) UpdateStatusBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID, newStatus model.InvoiceStatus) (*dto.InvoiceResponse, error) {
	invoice, err := s.invoiceRepo.FindBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	return s.UpdateStatus(ctx, invoice.ID, newStatus)
}

// GetInvoice retrieves a single invoice by its ID.
func (s *invoiceService) GetInvoice(ctx context.Context, invoiceID uuid.UUID) (*dto.InvoiceResponse, error) {
	invoice, err := s.invoiceRepo.FindByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	return toInvoiceResponse(invoice), nil
}

// ListInvoices returns paginated invoices for a user.
func (s *invoiceService) ListInvoices(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.InvoiceListResponse, error) {
	invoices, total, err := s.invoiceRepo.FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	invoiceResponses := make([]dto.InvoiceResponse, len(invoices))
	for i, inv := range invoices {
		invoiceResponses[i] = *toInvoiceResponse(&inv)
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &dto.InvoiceListResponse{
		Invoices: invoiceResponses,
		Pagination: dto.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int(total),
			TotalPages: totalPages,
		},
	}, nil
}

// toInvoiceResponse maps an Invoice model to an InvoiceResponse DTO.
func toInvoiceResponse(inv *model.Invoice) *dto.InvoiceResponse {
	resp := &dto.InvoiceResponse{
		ID:            inv.ID.String(),
		InvoiceNumber: inv.InvoiceNumber,
		Amount:        inv.Amount,
		Currency:      inv.Currency,
		Status:        string(inv.Status),
		DueDate:       inv.DueDate,
		PaidAt:        inv.PaidAt,
		CreatedAt:     inv.CreatedAt,
	}

	if inv.TransactionID != nil {
		txID := inv.TransactionID.String()
		resp.TransactionID = &txID
	}
	if inv.SubscriptionID != nil {
		subID := inv.SubscriptionID.String()
		resp.SubscriptionID = &subID
	}

	return resp
}
