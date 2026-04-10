package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock TransactionRepository ---

type mockTransactionRepo struct {
	transactions map[uuid.UUID]*model.Transaction
}

func newMockTransactionRepo() *mockTransactionRepo {
	return &mockTransactionRepo{transactions: make(map[uuid.UUID]*model.Transaction)}
}

func (m *mockTransactionRepo) Create(_ context.Context, tx *model.Transaction) error {
	for _, existing := range m.transactions {
		if existing.ExternalID == tx.ExternalID {
			return apierror.NewConflict("transaction with this external_id already exists")
		}
	}
	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
	tx.CreatedAt = time.Now()
	tx.UpdatedAt = time.Now()
	m.transactions[tx.ID] = tx
	return nil
}

func (m *mockTransactionRepo) FindByID(_ context.Context, id uuid.UUID) (*model.Transaction, error) {
	if tx, ok := m.transactions[id]; ok {
		return tx, nil
	}
	return nil, apierror.NewNotFound("transaction not found")
}

func (m *mockTransactionRepo) FindByUserID(_ context.Context, userID uuid.UUID, page, pageSize int) ([]model.Transaction, int64, error) {
	var result []model.Transaction
	for _, tx := range m.transactions {
		if tx.UserID == userID {
			result = append(result, *tx)
		}
	}
	total := int64(len(result))

	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return []model.Transaction{}, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockTransactionRepo) FindByExternalID(_ context.Context, externalID string) (*model.Transaction, error) {
	for _, tx := range m.transactions {
		if tx.ExternalID == externalID {
			return tx, nil
		}
	}
	return nil, apierror.NewNotFound("transaction not found")
}

func (m *mockTransactionRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.TransactionStatus) error {
	tx, ok := m.transactions[id]
	if !ok {
		return apierror.NewNotFound("transaction not found")
	}
	tx.Status = status
	if status == model.TxStatusSuccess {
		now := time.Now()
		tx.PaidAt = &now
	}
	return nil
}

// --- Mock WebhookService ---

type mockWebhookService struct {
	triggerCalled    bool
	triggerUserID    uuid.UUID
	triggerEventType string
	triggerPayload   map[string]interface{}
	triggerErr       error
}

func (m *mockWebhookService) RegisterEndpoint(_ context.Context, _ uuid.UUID, _ *dto.CreateWebhookRequest) (*dto.WebhookResponse, error) {
	return nil, nil
}

func (m *mockWebhookService) ListEndpoints(_ context.Context, _ uuid.UUID, _, _ int) (*dto.WebhookListResponse, error) {
	return nil, nil
}

func (m *mockWebhookService) DeleteEndpoint(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (m *mockWebhookService) TriggerWebhook(_ context.Context, userID uuid.UUID, eventType string, payload map[string]interface{}) error {
	m.triggerCalled = true
	m.triggerUserID = userID
	m.triggerEventType = eventType
	m.triggerPayload = payload
	return m.triggerErr
}

func (m *mockWebhookService) GetDeliveries(_ context.Context, _ uuid.UUID, _, _ int) (*dto.WebhookDeliveryListResponse, error) {
	return nil, nil
}

func (m *mockWebhookService) GenerateSignature(_ []byte, _ string) string {
	return ""
}

// --- Helper ---

func newTestTransactionService() (TransactionService, *mockTransactionRepo) {
	repo := newMockTransactionRepo()
	txm := &mockTxManager{}
	svc := NewTransactionService(repo, txm)
	return svc, repo
}

func newTestTransactionServiceWithWebhook() (TransactionService, *mockTransactionRepo, *mockWebhookService) {
	repo := newMockTransactionRepo()
	txm := &mockTxManager{}
	whSvc := &mockWebhookService{}
	svc := NewTransactionService(repo, txm, whSvc)
	return svc, repo, whSvc
}

func validCreateTransactionRequest() *dto.CreateTransactionRequest {
	return &dto.CreateTransactionRequest{
		Amount:        50000,
		Currency:      "IDR",
		PaymentMethod: "bank_transfer",
		ExternalID:    "ext-" + uuid.New().String(),
		Description:   "Test payment",
		CustomerEmail: "customer@example.com",
		Metadata:      map[string]interface{}{"order_id": "ORD-001"},
	}
}

// --- Tests ---

func TestCreateTransaction_Success(t *testing.T) {
	svc, repo := newTestTransactionService()
	userID := uuid.New()
	req := validCreateTransactionRequest()

	resp, err := svc.CreateTransaction(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Amount != req.Amount {
		t.Errorf("expected amount %d, got %d", req.Amount, resp.Amount)
	}
	if resp.Currency != req.Currency {
		t.Errorf("expected currency %s, got %s", req.Currency, resp.Currency)
	}
	if resp.Status != "pending" {
		t.Errorf("expected status pending, got %s", resp.Status)
	}
	if resp.PaymentMethod != req.PaymentMethod {
		t.Errorf("expected payment_method %s, got %s", req.PaymentMethod, resp.PaymentMethod)
	}
	if resp.ExternalID != req.ExternalID {
		t.Errorf("expected external_id %s, got %s", req.ExternalID, resp.ExternalID)
	}
	if resp.Description != req.Description {
		t.Errorf("expected description %s, got %s", req.Description, resp.Description)
	}
	if resp.CustomerEmail != req.CustomerEmail {
		t.Errorf("expected customer_email %s, got %s", req.CustomerEmail, resp.CustomerEmail)
	}

	// Verify metadata was stored.
	if resp.Metadata == nil {
		t.Fatal("expected metadata to be present")
	}
	if resp.Metadata["order_id"] != "ORD-001" {
		t.Errorf("expected metadata order_id ORD-001, got %v", resp.Metadata["order_id"])
	}

	// Verify stored in repo.
	if len(repo.transactions) != 1 {
		t.Fatalf("expected 1 transaction in repo, got %d", len(repo.transactions))
	}
}

func TestCreateTransaction_InvalidAmount(t *testing.T) {
	svc, _ := newTestTransactionService()
	userID := uuid.New()

	req := validCreateTransactionRequest()
	req.Amount = 0

	_, err := svc.CreateTransaction(context.Background(), userID, req)
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestCreateTransaction_NegativeAmount(t *testing.T) {
	svc, _ := newTestTransactionService()
	userID := uuid.New()

	req := validCreateTransactionRequest()
	req.Amount = -100

	_, err := svc.CreateTransaction(context.Background(), userID, req)
	if err == nil {
		t.Fatal("expected error for negative amount, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestCreateTransaction_DuplicateExternalID(t *testing.T) {
	svc, _ := newTestTransactionService()
	userID := uuid.New()

	req := validCreateTransactionRequest()

	_, err := svc.CreateTransaction(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("first create should succeed, got %v", err)
	}

	// Second create with same external_id should fail.
	_, err = svc.CreateTransaction(context.Background(), userID, req)
	if err == nil {
		t.Fatal("expected error for duplicate external_id, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT code, got %s", apiErr.Code)
	}
}

func TestCreateTransaction_MetadataMarshaling(t *testing.T) {
	svc, repo := newTestTransactionService()
	userID := uuid.New()

	req := validCreateTransactionRequest()
	req.Metadata = map[string]interface{}{
		"key1": "value1",
		"key2": float64(42),
	}

	resp, err := svc.CreateTransaction(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify metadata in response.
	if resp.Metadata["key1"] != "value1" {
		t.Errorf("expected metadata key1=value1, got %v", resp.Metadata["key1"])
	}

	// Verify metadata stored as JSON in repo.
	for _, stored := range repo.transactions {
		var m map[string]interface{}
		if err := json.Unmarshal(stored.Metadata, &m); err != nil {
			t.Fatalf("failed to unmarshal stored metadata: %v", err)
		}
		if m["key1"] != "value1" {
			t.Errorf("expected stored metadata key1=value1, got %v", m["key1"])
		}
	}
}

func TestSimulatePayment_Success(t *testing.T) {
	svc, repo := newTestTransactionService()
	txID := uuid.New()

	repo.transactions[txID] = &model.Transaction{
		ID:            txID,
		UserID:        uuid.New(),
		ExternalID:    "ext-sim-001",
		Amount:        10000,
		Currency:      "IDR",
		Status:        model.TxStatusPending,
		PaymentMethod: model.PMBankTransfer,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := svc.SimulatePayment(context.Background(), txID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Status should be either success or failed (not pending).
	stored := repo.transactions[txID]
	if stored.Status != model.TxStatusSuccess && stored.Status != model.TxStatusFailed {
		t.Errorf("expected status success or failed, got %s", stored.Status)
	}
}

func TestSimulatePayment_NotFound(t *testing.T) {
	svc, _ := newTestTransactionService()

	err := svc.SimulatePayment(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent transaction, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestGetTransaction_Success(t *testing.T) {
	svc, repo := newTestTransactionService()
	txID := uuid.New()

	repo.transactions[txID] = &model.Transaction{
		ID:            txID,
		UserID:        uuid.New(),
		ExternalID:    "ext-get-001",
		Amount:        25000,
		Currency:      "IDR",
		Status:        model.TxStatusPending,
		PaymentMethod: model.PMCreditCard,
		Description:   "Get test",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	resp, err := svc.GetTransaction(context.Background(), txID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.ID != txID.String() {
		t.Errorf("expected ID %s, got %s", txID.String(), resp.ID)
	}
	if resp.Amount != 25000 {
		t.Errorf("expected amount 25000, got %d", resp.Amount)
	}
	if resp.Status != "pending" {
		t.Errorf("expected status pending, got %s", resp.Status)
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	svc, _ := newTestTransactionService()

	_, err := svc.GetTransaction(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent transaction, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestListTransactions_Success(t *testing.T) {
	svc, repo := newTestTransactionService()
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		txID := uuid.New()
		repo.transactions[txID] = &model.Transaction{
			ID:            txID,
			UserID:        userID,
			ExternalID:    "ext-list-" + uuid.New().String(),
			Amount:        int64((i + 1) * 10000),
			Currency:      "IDR",
			Status:        model.TxStatusPending,
			PaymentMethod: model.PMBankTransfer,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
	}

	resp, err := svc.ListTransactions(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Transactions) != 3 {
		t.Errorf("expected 3 transactions, got %d", len(resp.Transactions))
	}
	if resp.Pagination.TotalItems != 3 {
		t.Errorf("expected total_items 3, got %d", resp.Pagination.TotalItems)
	}
	if resp.Pagination.TotalPages != 1 {
		t.Errorf("expected total_pages 1, got %d", resp.Pagination.TotalPages)
	}
}

func TestListTransactions_Pagination(t *testing.T) {
	svc, repo := newTestTransactionService()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		txID := uuid.New()
		repo.transactions[txID] = &model.Transaction{
			ID:            txID,
			UserID:        userID,
			ExternalID:    "ext-page-" + uuid.New().String(),
			Amount:        int64((i + 1) * 1000),
			Currency:      "IDR",
			Status:        model.TxStatusPending,
			PaymentMethod: model.PMEWallet,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
	}

	resp, err := svc.ListTransactions(context.Background(), userID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Transactions) != 2 {
		t.Errorf("expected 2 transactions on page 1, got %d", len(resp.Transactions))
	}
	if resp.Pagination.TotalItems != 5 {
		t.Errorf("expected total_items 5, got %d", resp.Pagination.TotalItems)
	}
	if resp.Pagination.TotalPages != 3 {
		t.Errorf("expected total_pages 3, got %d", resp.Pagination.TotalPages)
	}
}

func TestListTransactions_Empty(t *testing.T) {
	svc, _ := newTestTransactionService()
	userID := uuid.New()

	resp, err := svc.ListTransactions(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Transactions) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(resp.Transactions))
	}
	if resp.Pagination.TotalItems != 0 {
		t.Errorf("expected total_items 0, got %d", resp.Pagination.TotalItems)
	}
}

// --- Webhook Integration Tests ---

func TestSimulatePayment_TriggersWebhook(t *testing.T) {
	svc, repo, whSvc := newTestTransactionServiceWithWebhook()
	userID := uuid.New()
	txID := uuid.New()

	repo.transactions[txID] = &model.Transaction{
		ID:            txID,
		UserID:        userID,
		ExternalID:    "ext-wh-001",
		Amount:        10000,
		Currency:      "IDR",
		Status:        model.TxStatusPending,
		PaymentMethod: model.PMBankTransfer,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := svc.SimulatePayment(context.Background(), txID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !whSvc.triggerCalled {
		t.Fatal("expected webhook TriggerWebhook to be called")
	}

	if whSvc.triggerUserID != userID {
		t.Errorf("expected webhook userID %s, got %s", userID, whSvc.triggerUserID)
	}

	// Event type should match the resulting status.
	stored := repo.transactions[txID]
	expectedEvent := "transaction.success"
	if stored.Status == model.TxStatusFailed {
		expectedEvent = "transaction.failed"
	}
	if whSvc.triggerEventType != expectedEvent {
		t.Errorf("expected event type %s, got %s", expectedEvent, whSvc.triggerEventType)
	}

	// Verify payload contains required fields.
	if whSvc.triggerPayload["id"] != txID.String() {
		t.Errorf("expected payload id %s, got %v", txID.String(), whSvc.triggerPayload["id"])
	}
	if whSvc.triggerPayload["external_id"] != "ext-wh-001" {
		t.Errorf("expected payload external_id ext-wh-001, got %v", whSvc.triggerPayload["external_id"])
	}
	if whSvc.triggerPayload["amount"] != int64(10000) {
		t.Errorf("expected payload amount 10000, got %v", whSvc.triggerPayload["amount"])
	}
	if whSvc.triggerPayload["currency"] != "IDR" {
		t.Errorf("expected payload currency IDR, got %v", whSvc.triggerPayload["currency"])
	}
}

func TestSimulatePayment_WebhookFailureDoesNotFailTransaction(t *testing.T) {
	svc, repo, whSvc := newTestTransactionServiceWithWebhook()
	txID := uuid.New()

	repo.transactions[txID] = &model.Transaction{
		ID:            txID,
		UserID:        uuid.New(),
		ExternalID:    "ext-wh-fail-001",
		Amount:        20000,
		Currency:      "IDR",
		Status:        model.TxStatusPending,
		PaymentMethod: model.PMCreditCard,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Make webhook trigger return an error.
	whSvc.triggerErr = errors.New("webhook delivery failed")

	err := svc.SimulatePayment(context.Background(), txID)
	if err != nil {
		t.Fatalf("expected no error even when webhook fails, got %v", err)
	}

	// Transaction status should still be updated.
	stored := repo.transactions[txID]
	if stored.Status != model.TxStatusSuccess && stored.Status != model.TxStatusFailed {
		t.Errorf("expected status success or failed, got %s", stored.Status)
	}

	// Webhook should still have been called.
	if !whSvc.triggerCalled {
		t.Fatal("expected webhook TriggerWebhook to be called even if it fails")
	}
}

func TestSimulatePayment_WithoutWebhookService(t *testing.T) {
	// Use the constructor without webhook service (backward compatible).
	svc, repo := newTestTransactionService()
	txID := uuid.New()

	repo.transactions[txID] = &model.Transaction{
		ID:            txID,
		UserID:        uuid.New(),
		ExternalID:    "ext-no-wh-001",
		Amount:        15000,
		Currency:      "IDR",
		Status:        model.TxStatusPending,
		PaymentMethod: model.PMEWallet,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err := svc.SimulatePayment(context.Background(), txID)
	if err != nil {
		t.Fatalf("expected no error without webhook service, got %v", err)
	}

	stored := repo.transactions[txID]
	if stored.Status != model.TxStatusSuccess && stored.Status != model.TxStatusFailed {
		t.Errorf("expected status success or failed, got %s", stored.Status)
	}
}
