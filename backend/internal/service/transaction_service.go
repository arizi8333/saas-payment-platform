package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"math/rand"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/repository"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// TransactionService defines the interface for transaction business logic operations.
type TransactionService interface {
	// CreateTransaction creates a new payment transaction with status "pending".
	CreateTransaction(ctx context.Context, userID uuid.UUID, req *dto.CreateTransactionRequest) (*dto.TransactionResponse, error)
	// SimulatePayment randomly sets the transaction status to "success" or "failed".
	SimulatePayment(ctx context.Context, txID uuid.UUID) error
	// GetTransaction retrieves a transaction by ID.
	GetTransaction(ctx context.Context, txID uuid.UUID) (*dto.TransactionResponse, error)
	// ListTransactions returns paginated transactions for a user.
	ListTransactions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.TransactionListResponse, error)
}

// transactionService implements TransactionService.
type transactionService struct {
	txRepo         repository.TransactionRepository
	txManager      database.TransactionManager
	webhookService WebhookService
}

// NewTransactionService creates a new TransactionService with all required dependencies.
func NewTransactionService(
	txRepo repository.TransactionRepository,
	txManager database.TransactionManager,
	webhookService ...WebhookService,
) TransactionService {
	svc := &transactionService{
		txRepo:    txRepo,
		txManager: txManager,
	}
	if len(webhookService) > 0 {
		svc.webhookService = webhookService[0]
	}
	return svc
}

// CreateTransaction validates the request, checks external_id uniqueness,
// and creates a new transaction with status "pending".
func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req *dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	// Validate amount > 0.
	if req.Amount <= 0 {
		return nil, apierror.NewBadRequest("amount must be greater than 0")
	}

	// Check external_id uniqueness.
	existing, err := s.txRepo.FindByExternalID(ctx, req.ExternalID)
	if err != nil {
		var apiErr *apierror.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "NOT_FOUND" {
			return nil, err
		}
	}
	if existing != nil {
		return nil, apierror.NewConflict("transaction with this external_id already exists")
	}

	// Marshal metadata to JSON.
	var metadataJSON []byte
	if req.Metadata != nil {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			return nil, apierror.NewInternalError("failed to marshal metadata")
		}
	}

	transaction := &model.Transaction{
		UserID:        userID,
		ExternalID:    req.ExternalID,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Status:        model.TxStatusPending,
		PaymentMethod: model.PaymentMethod(req.PaymentMethod),
		Description:   req.Description,
		CustomerEmail: req.CustomerEmail,
		Metadata:      metadataJSON,
	}

	// Use transaction manager for the write operation.
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.txRepo.Create(txCtx, transaction)
	})
	if err != nil {
		return nil, err
	}

	return toTransactionResponse(transaction), nil
}

// SimulatePayment finds the transaction and randomly sets status to "success" or "failed".
// After updating the status, triggers a webhook notification with the appropriate event type.
// If webhook trigger fails, the error is logged but does NOT fail the transaction.
func (s *transactionService) SimulatePayment(ctx context.Context, txID uuid.UUID) error {
	// Verify transaction exists and get details for webhook payload.
	tx, err := s.txRepo.FindByID(ctx, txID)
	if err != nil {
		return err
	}

	// Randomly determine outcome.
	status := model.TxStatusSuccess
	if rand.Intn(2) == 0 {
		status = model.TxStatusFailed
	}

	if err := s.txRepo.UpdateStatus(ctx, txID, status); err != nil {
		return err
	}

	// Trigger webhook notification after successful status update.
	if s.webhookService != nil {
		eventType := "transaction.success"
		if status == model.TxStatusFailed {
			eventType = "transaction.failed"
		}

		payload := map[string]interface{}{
			"id":          txID.String(),
			"external_id": tx.ExternalID,
			"status":      string(status),
			"amount":      tx.Amount,
			"currency":    tx.Currency,
		}

		if whErr := s.webhookService.TriggerWebhook(ctx, tx.UserID, eventType, payload); whErr != nil {
			slog.ErrorContext(ctx, "failed to trigger webhook after payment simulation",
				"transaction_id", txID.String(),
				"event_type", eventType,
				"error", whErr,
			)
			// Webhook failure does NOT fail the transaction.
		}
	}

	return nil
}

// GetTransaction retrieves a transaction by ID and returns it as a DTO.
func (s *transactionService) GetTransaction(ctx context.Context, txID uuid.UUID) (*dto.TransactionResponse, error) {
	transaction, err := s.txRepo.FindByID(ctx, txID)
	if err != nil {
		return nil, err
	}

	return toTransactionResponse(transaction), nil
}

// ListTransactions returns paginated transactions for a user.
func (s *transactionService) ListTransactions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.TransactionListResponse, error) {
	transactions, total, err := s.txRepo.FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	txResponses := make([]dto.TransactionResponse, len(transactions))
	for i, tx := range transactions {
		txResponses[i] = *toTransactionResponse(&tx)
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &dto.TransactionListResponse{
		Transactions: txResponses,
		Pagination: dto.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int(total),
			TotalPages: totalPages,
		},
	}, nil
}

// toTransactionResponse maps a Transaction model to a TransactionResponse DTO.
func toTransactionResponse(tx *model.Transaction) *dto.TransactionResponse {
	var metadata map[string]interface{}
	if tx.Metadata != nil {
		_ = json.Unmarshal(tx.Metadata, &metadata)
	}

	return &dto.TransactionResponse{
		ID:            tx.ID.String(),
		ExternalID:    tx.ExternalID,
		Amount:        tx.Amount,
		Currency:      tx.Currency,
		Status:        string(tx.Status),
		PaymentMethod: string(tx.PaymentMethod),
		Description:   tx.Description,
		CustomerEmail: tx.CustomerEmail,
		Metadata:      metadata,
		PaidAt:        tx.PaidAt,
		CreatedAt:     tx.CreatedAt,
	}
}
