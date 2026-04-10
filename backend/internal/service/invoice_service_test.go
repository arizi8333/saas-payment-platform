package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock InvoiceRepository ---

type mockInvoiceRepo struct {
	invoices     map[uuid.UUID]*model.Invoice
	invoiceCount int64 // tracks daily count for number generation
}

func newMockInvoiceRepo() *mockInvoiceRepo {
	return &mockInvoiceRepo{invoices: make(map[uuid.UUID]*model.Invoice)}
}

func (m *mockInvoiceRepo) Create(_ context.Context, invoice *model.Invoice) error {
	// Check unique invoice_number.
	for _, existing := range m.invoices {
		if existing.InvoiceNumber == invoice.InvoiceNumber {
			return apierror.NewConflict("invoice with this number already exists")
		}
	}
	if invoice.ID == uuid.Nil {
		invoice.ID = uuid.New()
	}
	invoice.CreatedAt = time.Now()
	invoice.UpdatedAt = time.Now()
	m.invoices[invoice.ID] = invoice
	return nil
}

func (m *mockInvoiceRepo) FindByID(_ context.Context, id uuid.UUID) (*model.Invoice, error) {
	if inv, ok := m.invoices[id]; ok {
		return inv, nil
	}
	return nil, apierror.NewNotFound("invoice not found")
}

func (m *mockInvoiceRepo) FindByUserID(_ context.Context, userID uuid.UUID, page, pageSize int) ([]model.Invoice, int64, error) {
	var result []model.Invoice
	for _, inv := range m.invoices {
		if inv.UserID == userID {
			result = append(result, *inv)
		}
	}
	total := int64(len(result))

	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return []model.Invoice{}, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockInvoiceRepo) FindBySubscriptionID(_ context.Context, subscriptionID uuid.UUID) (*model.Invoice, error) {
	for _, inv := range m.invoices {
		if inv.SubscriptionID != nil && *inv.SubscriptionID == subscriptionID {
			return inv, nil
		}
	}
	return nil, apierror.NewNotFound("invoice not found for subscription")
}

func (m *mockInvoiceRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.InvoiceStatus) error {
	inv, ok := m.invoices[id]
	if !ok {
		return apierror.NewNotFound("invoice not found")
	}
	inv.Status = status
	if status == model.InvoicePaid {
		now := time.Now()
		inv.PaidAt = &now
	}
	return nil
}

func (m *mockInvoiceRepo) GenerateInvoiceNumber(_ context.Context) (string, error) {
	m.invoiceCount++
	dateStr := time.Now().Format("20060102")
	return fmt.Sprintf("INV-%s-%05d", dateStr, m.invoiceCount), nil
}

// --- Helper ---

func newTestInvoiceService() (InvoiceService, *mockInvoiceRepo) {
	repo := newMockInvoiceRepo()
	txm := &mockTxManager{}
	svc := NewInvoiceService(repo, txm)
	return svc, repo
}

func validCreateInvoiceInput() *CreateInvoiceInput {
	txID := uuid.New()
	return &CreateInvoiceInput{
		UserID:        uuid.New(),
		TransactionID: &txID,
		Amount:        50000,
		Currency:      "IDR",
		DueDate:       time.Now().Add(7 * 24 * time.Hour),
	}
}

// --- Tests ---

func TestCreateInvoice_Success(t *testing.T) {
	svc, repo := newTestInvoiceService()
	input := validCreateInvoiceInput()

	resp, err := svc.CreateInvoice(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Amount != input.Amount {
		t.Errorf("expected amount %d, got %d", input.Amount, resp.Amount)
	}
	if resp.Currency != input.Currency {
		t.Errorf("expected currency %s, got %s", input.Currency, resp.Currency)
	}
	if resp.Status != "unpaid" {
		t.Errorf("expected status unpaid, got %s", resp.Status)
	}
	if resp.InvoiceNumber == "" {
		t.Error("expected non-empty invoice number")
	}
	// Verify INV-YYYYMMDD-XXXXX format.
	dateStr := time.Now().Format("20060102")
	expectedPrefix := "INV-" + dateStr + "-"
	if len(resp.InvoiceNumber) < len(expectedPrefix) || resp.InvoiceNumber[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected invoice number to start with %s, got %s", expectedPrefix, resp.InvoiceNumber)
	}
	if resp.TransactionID == nil {
		t.Error("expected transaction_id to be set")
	}
	if resp.PaidAt != nil {
		t.Error("expected paid_at to be nil for unpaid invoice")
	}

	// Verify stored in repo.
	if len(repo.invoices) != 1 {
		t.Fatalf("expected 1 invoice in repo, got %d", len(repo.invoices))
	}
}

func TestCreateInvoice_WithSubscription(t *testing.T) {
	svc, _ := newTestInvoiceService()
	subID := uuid.New()
	input := &CreateInvoiceInput{
		UserID:         uuid.New(),
		SubscriptionID: &subID,
		Amount:         100000,
		Currency:       "IDR",
		DueDate:        time.Now().Add(30 * 24 * time.Hour),
	}

	resp, err := svc.CreateInvoice(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.SubscriptionID == nil {
		t.Error("expected subscription_id to be set")
	}
	if resp.TransactionID != nil {
		t.Error("expected transaction_id to be nil")
	}
}

func TestCreateInvoice_InvalidAmount(t *testing.T) {
	svc, _ := newTestInvoiceService()
	input := validCreateInvoiceInput()
	input.Amount = 0

	_, err := svc.CreateInvoice(context.Background(), input)
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

func TestCreateInvoice_NegativeAmount(t *testing.T) {
	svc, _ := newTestInvoiceService()
	input := validCreateInvoiceInput()
	input.Amount = -500

	_, err := svc.CreateInvoice(context.Background(), input)
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

func TestCreateInvoice_DefaultCurrency(t *testing.T) {
	svc, _ := newTestInvoiceService()
	input := validCreateInvoiceInput()
	input.Currency = ""

	resp, err := svc.CreateInvoice(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Currency != "IDR" {
		t.Errorf("expected default currency IDR, got %s", resp.Currency)
	}
}

func TestCreateInvoice_UniqueNumbers(t *testing.T) {
	svc, _ := newTestInvoiceService()

	resp1, err := svc.CreateInvoice(context.Background(), validCreateInvoiceInput())
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	resp2, err := svc.CreateInvoice(context.Background(), validCreateInvoiceInput())
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}

	if resp1.InvoiceNumber == resp2.InvoiceNumber {
		t.Errorf("expected unique invoice numbers, both got %s", resp1.InvoiceNumber)
	}
}

func TestUpdateStatus_UnpaidToPaid(t *testing.T) {
	svc, repo := newTestInvoiceService()
	invID := uuid.New()

	repo.invoices[invID] = &model.Invoice{
		ID:            invID,
		UserID:        uuid.New(),
		InvoiceNumber: "INV-20250101-00001",
		Amount:        50000,
		Currency:      "IDR",
		Status:        model.InvoiceUnpaid,
		DueDate:       time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	resp, err := svc.UpdateStatus(context.Background(), invID, model.InvoicePaid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Status != "paid" {
		t.Errorf("expected status paid, got %s", resp.Status)
	}
	if resp.PaidAt == nil {
		t.Error("expected paid_at to be set")
	}
}

func TestUpdateStatus_UnpaidToVoid(t *testing.T) {
	svc, repo := newTestInvoiceService()
	invID := uuid.New()

	repo.invoices[invID] = &model.Invoice{
		ID:            invID,
		UserID:        uuid.New(),
		InvoiceNumber: "INV-20250101-00002",
		Amount:        30000,
		Currency:      "IDR",
		Status:        model.InvoiceUnpaid,
		DueDate:       time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	resp, err := svc.UpdateStatus(context.Background(), invID, model.InvoiceVoid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Status != "void" {
		t.Errorf("expected status void, got %s", resp.Status)
	}
	if resp.PaidAt != nil {
		t.Error("expected paid_at to be nil for void invoice")
	}
}

func TestUpdateStatus_AlreadyPaid(t *testing.T) {
	svc, repo := newTestInvoiceService()
	invID := uuid.New()
	now := time.Now()

	repo.invoices[invID] = &model.Invoice{
		ID:            invID,
		UserID:        uuid.New(),
		InvoiceNumber: "INV-20250101-00003",
		Amount:        50000,
		Currency:      "IDR",
		Status:        model.InvoicePaid,
		DueDate:       time.Now().Add(7 * 24 * time.Hour),
		PaidAt:        &now,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err := svc.UpdateStatus(context.Background(), invID, model.InvoiceVoid)
	if err == nil {
		t.Fatal("expected error for already paid invoice, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestUpdateStatus_InvalidTransition(t *testing.T) {
	svc, repo := newTestInvoiceService()
	invID := uuid.New()

	repo.invoices[invID] = &model.Invoice{
		ID:            invID,
		UserID:        uuid.New(),
		InvoiceNumber: "INV-20250101-00004",
		Amount:        50000,
		Currency:      "IDR",
		Status:        model.InvoiceUnpaid,
		DueDate:       time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Try to set to "unpaid" (same status, invalid transition target).
	_, err := svc.UpdateStatus(context.Background(), invID, model.InvoiceUnpaid)
	if err == nil {
		t.Fatal("expected error for invalid status transition, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	svc, _ := newTestInvoiceService()

	_, err := svc.UpdateStatus(context.Background(), uuid.New(), model.InvoicePaid)
	if err == nil {
		t.Fatal("expected error for non-existent invoice, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestGetInvoice_Success(t *testing.T) {
	svc, repo := newTestInvoiceService()
	invID := uuid.New()
	txID := uuid.New()

	repo.invoices[invID] = &model.Invoice{
		ID:            invID,
		UserID:        uuid.New(),
		TransactionID: &txID,
		InvoiceNumber: "INV-20250101-00005",
		Amount:        75000,
		Currency:      "IDR",
		Status:        model.InvoiceUnpaid,
		DueDate:       time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	resp, err := svc.GetInvoice(context.Background(), invID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.ID != invID.String() {
		t.Errorf("expected ID %s, got %s", invID.String(), resp.ID)
	}
	if resp.Amount != 75000 {
		t.Errorf("expected amount 75000, got %d", resp.Amount)
	}
	if resp.TransactionID == nil || *resp.TransactionID != txID.String() {
		t.Errorf("expected transaction_id %s", txID.String())
	}
}

func TestGetInvoice_NotFound(t *testing.T) {
	svc, _ := newTestInvoiceService()

	_, err := svc.GetInvoice(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent invoice, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestListInvoices_Success(t *testing.T) {
	svc, repo := newTestInvoiceService()
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		invID := uuid.New()
		repo.invoices[invID] = &model.Invoice{
			ID:            invID,
			UserID:        userID,
			InvoiceNumber: fmt.Sprintf("INV-20250101-%05d", i+1),
			Amount:        int64((i + 1) * 10000),
			Currency:      "IDR",
			Status:        model.InvoiceUnpaid,
			DueDate:       time.Now().Add(7 * 24 * time.Hour),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
	}

	resp, err := svc.ListInvoices(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Invoices) != 3 {
		t.Errorf("expected 3 invoices, got %d", len(resp.Invoices))
	}
	if resp.Pagination.TotalItems != 3 {
		t.Errorf("expected total_items 3, got %d", resp.Pagination.TotalItems)
	}
	if resp.Pagination.TotalPages != 1 {
		t.Errorf("expected total_pages 1, got %d", resp.Pagination.TotalPages)
	}
}

func TestListInvoices_Pagination(t *testing.T) {
	svc, repo := newTestInvoiceService()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		invID := uuid.New()
		repo.invoices[invID] = &model.Invoice{
			ID:            invID,
			UserID:        userID,
			InvoiceNumber: fmt.Sprintf("INV-20250101-%05d", i+10),
			Amount:        int64((i + 1) * 5000),
			Currency:      "IDR",
			Status:        model.InvoiceUnpaid,
			DueDate:       time.Now().Add(7 * 24 * time.Hour),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
	}

	resp, err := svc.ListInvoices(context.Background(), userID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Invoices) != 2 {
		t.Errorf("expected 2 invoices on page 1, got %d", len(resp.Invoices))
	}
	if resp.Pagination.TotalItems != 5 {
		t.Errorf("expected total_items 5, got %d", resp.Pagination.TotalItems)
	}
	if resp.Pagination.TotalPages != 3 {
		t.Errorf("expected total_pages 3, got %d", resp.Pagination.TotalPages)
	}
}

func TestListInvoices_Empty(t *testing.T) {
	svc, _ := newTestInvoiceService()
	userID := uuid.New()

	resp, err := svc.ListInvoices(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Invoices) != 0 {
		t.Errorf("expected 0 invoices, got %d", len(resp.Invoices))
	}
	if resp.Pagination.TotalItems != 0 {
		t.Errorf("expected total_items 0, got %d", resp.Pagination.TotalItems)
	}
}
