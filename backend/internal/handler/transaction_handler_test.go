package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// mockTransactionService implements service.TransactionService for testing.
type mockTransactionService struct {
	createTransactionFn func(ctx context.Context, userID uuid.UUID, req *dto.CreateTransactionRequest) (*dto.TransactionResponse, error)
	simulatePaymentFn   func(ctx context.Context, txID uuid.UUID) error
	getTransactionFn    func(ctx context.Context, txID uuid.UUID) (*dto.TransactionResponse, error)
	listTransactionsFn  func(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.TransactionListResponse, error)
}

func (m *mockTransactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req *dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	return m.createTransactionFn(ctx, userID, req)
}

func (m *mockTransactionService) SimulatePayment(ctx context.Context, txID uuid.UUID) error {
	if m.simulatePaymentFn != nil {
		return m.simulatePaymentFn(ctx, txID)
	}
	return nil
}

func (m *mockTransactionService) GetTransaction(ctx context.Context, txID uuid.UUID) (*dto.TransactionResponse, error) {
	return m.getTransactionFn(ctx, txID)
}

func (m *mockTransactionService) ListTransactions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.TransactionListResponse, error) {
	return m.listTransactionsFn(ctx, userID, page, pageSize)
}

// mockAPIKeyServiceForTx implements service.APIKeyService to bypass API key middleware in tests.
type mockAPIKeyServiceForTx struct {
	userID uuid.UUID
}

func (m *mockAPIKeyServiceForTx) CreateKey(_ context.Context, _ uuid.UUID, _ *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error) {
	return nil, nil
}

func (m *mockAPIKeyServiceForTx) ListKeys(_ context.Context, _ uuid.UUID, _, _ int) (*dto.APIKeyListResponse, error) {
	return nil, nil
}

func (m *mockAPIKeyServiceForTx) RevokeKey(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (m *mockAPIKeyServiceForTx) ValidateKey(_ context.Context, _ string) (*model.APIKey, error) {
	return &model.APIKey{
		UserID: m.userID,
		User:   model.User{Role: model.RoleDeveloper},
	}, nil
}

// setupTransactionTestApp creates a Fiber app with the transaction handler and API key middleware.
func setupTransactionTestApp(txSvc *mockTransactionService, userID uuid.UUID) *fiber.App {
	app := fiber.New()
	h := NewTransactionHandler(txSvc)
	apiKeySvc := &mockAPIKeyServiceForTx{userID: userID}
	apiKeyMW := middleware.NewAPIKeyMiddleware(apiKeySvc)
	api := app.Group("/api/v1")
	h.RegisterRoutes(api, apiKeyMW)
	return app
}

func txToJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func parseTxResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

// --- CreateTransaction tests ---

func TestCreateTransaction_Success_Returns201(t *testing.T) {
	uid := uuid.New()
	txID := uuid.New()
	svc := &mockTransactionService{
		createTransactionFn: func(_ context.Context, _ uuid.UUID, req *dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
			return &dto.TransactionResponse{
				ID:            txID.String(),
				ExternalID:    req.ExternalID,
				Amount:        req.Amount,
				Currency:      req.Currency,
				Status:        "pending",
				PaymentMethod: req.PaymentMethod,
				Description:   req.Description,
				CustomerEmail: req.CustomerEmail,
				CreatedAt:     time.Now(),
			}, nil
		},
	}

	app := setupTransactionTestApp(svc, uid)
	body := txToJSON(t, dto.CreateTransactionRequest{
		Amount:        50000,
		Currency:      "IDR",
		PaymentMethod: "bank_transfer",
		ExternalID:    "ext-001",
		Description:   "Test payment",
		CustomerEmail: "customer@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	r := parseTxResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestCreateTransaction_InvalidBody_Returns400(t *testing.T) {
	svc := &mockTransactionService{}
	app := setupTransactionTestApp(svc, uuid.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_MissingRequiredFields_Returns400(t *testing.T) {
	svc := &mockTransactionService{}
	app := setupTransactionTestApp(svc, uuid.New())

	body := txToJSON(t, map[string]interface{}{
		"amount": 50000,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	r := parseTxResponse(t, resp)
	if r.Success {
		t.Error("expected success=false")
	}
}

func TestCreateTransaction_NoAPIKey_Returns401(t *testing.T) {
	svc := &mockTransactionService{}
	app := setupTransactionTestApp(svc, uuid.New())

	body := txToJSON(t, dto.CreateTransactionRequest{
		Amount:        50000,
		Currency:      "IDR",
		PaymentMethod: "bank_transfer",
		ExternalID:    "ext-001",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_DuplicateExternalID_Returns409(t *testing.T) {
	svc := &mockTransactionService{
		createTransactionFn: func(_ context.Context, _ uuid.UUID, _ *dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
			return nil, apierror.NewConflict("transaction with this external_id already exists")
		},
	}

	app := setupTransactionTestApp(svc, uuid.New())
	body := txToJSON(t, dto.CreateTransactionRequest{
		Amount:        50000,
		Currency:      "IDR",
		PaymentMethod: "bank_transfer",
		ExternalID:    "ext-dup",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestCreateTransaction_ServiceError_Returns500(t *testing.T) {
	svc := &mockTransactionService{
		createTransactionFn: func(_ context.Context, _ uuid.UUID, _ *dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupTransactionTestApp(svc, uuid.New())
	body := txToJSON(t, dto.CreateTransactionRequest{
		Amount:        50000,
		Currency:      "IDR",
		PaymentMethod: "bank_transfer",
		ExternalID:    "ext-err",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- ListTransactions tests ---

func TestListTransactions_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockTransactionService{
		listTransactionsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.TransactionListResponse, error) {
			return &dto.TransactionListResponse{
				Transactions: []dto.TransactionResponse{
					{
						ID:            uuid.New().String(),
						ExternalID:    "ext-001",
						Amount:        50000,
						Currency:      "IDR",
						Status:        "pending",
						PaymentMethod: "bank_transfer",
						CreatedAt:     time.Now(),
					},
				},
				Pagination: dto.PaginationResponse{
					Page:       page,
					PageSize:   pageSize,
					TotalItems: 1,
					TotalPages: 1,
				},
			}, nil
		},
	}

	app := setupTransactionTestApp(svc, uid)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/?page=1&page_size=10", nil)
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseTxResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestListTransactions_DefaultPagination_Returns200(t *testing.T) {
	uid := uuid.New()
	var capturedPage, capturedPageSize int
	svc := &mockTransactionService{
		listTransactionsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.TransactionListResponse, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return &dto.TransactionListResponse{
				Transactions: []dto.TransactionResponse{},
				Pagination: dto.PaginationResponse{
					Page:       page,
					PageSize:   pageSize,
					TotalItems: 0,
					TotalPages: 0,
				},
			}, nil
		},
	}

	app := setupTransactionTestApp(svc, uid)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/", nil)
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if capturedPage != 1 {
		t.Errorf("expected default page=1, got %d", capturedPage)
	}
	if capturedPageSize != 10 {
		t.Errorf("expected default page_size=10, got %d", capturedPageSize)
	}
}

func TestListTransactions_NoAPIKey_Returns401(t *testing.T) {
	svc := &mockTransactionService{}
	app := setupTransactionTestApp(svc, uuid.New())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- GetTransaction tests ---

func TestGetTransaction_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	txID := uuid.New()
	svc := &mockTransactionService{
		getTransactionFn: func(_ context.Context, id uuid.UUID) (*dto.TransactionResponse, error) {
			return &dto.TransactionResponse{
				ID:            id.String(),
				ExternalID:    "ext-001",
				Amount:        50000,
				Currency:      "IDR",
				Status:        "success",
				PaymentMethod: "bank_transfer",
				CreatedAt:     time.Now(),
			}, nil
		},
	}

	app := setupTransactionTestApp(svc, uid)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+txID.String(), nil)
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseTxResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestGetTransaction_InvalidUUID_Returns400(t *testing.T) {
	svc := &mockTransactionService{}
	app := setupTransactionTestApp(svc, uuid.New())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/not-a-uuid", nil)
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetTransaction_NotFound_Returns404(t *testing.T) {
	svc := &mockTransactionService{
		getTransactionFn: func(_ context.Context, _ uuid.UUID) (*dto.TransactionResponse, error) {
			return nil, apierror.NewNotFound("transaction not found")
		},
	}

	app := setupTransactionTestApp(svc, uuid.New())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+uuid.New().String(), nil)
	req.Header.Set("X-API-Key", "sk_test_testkey123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetTransaction_NoAPIKey_Returns401(t *testing.T) {
	svc := &mockTransactionService{}
	app := setupTransactionTestApp(svc, uuid.New())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+uuid.New().String(), nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
