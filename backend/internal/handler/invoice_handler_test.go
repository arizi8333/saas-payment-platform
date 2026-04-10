package handler

import (
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
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/internal/service"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// mockInvoiceService implements service.InvoiceService for testing.
type mockInvoiceService struct {
	listInvoicesFn func(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.InvoiceListResponse, error)
	getInvoiceFn   func(ctx context.Context, invoiceID uuid.UUID) (*dto.InvoiceResponse, error)
}

func (m *mockInvoiceService) CreateInvoice(_ context.Context, _ *service.CreateInvoiceInput) (*dto.InvoiceResponse, error) {
	return nil, nil
}

func (m *mockInvoiceService) UpdateStatus(_ context.Context, _ uuid.UUID, _ model.InvoiceStatus) (*dto.InvoiceResponse, error) {
	return nil, nil
}

func (m *mockInvoiceService) UpdateStatusBySubscriptionID(_ context.Context, _ uuid.UUID, _ model.InvoiceStatus) (*dto.InvoiceResponse, error) {
	return nil, nil
}

func (m *mockInvoiceService) GetInvoice(ctx context.Context, invoiceID uuid.UUID) (*dto.InvoiceResponse, error) {
	return m.getInvoiceFn(ctx, invoiceID)
}

func (m *mockInvoiceService) ListInvoices(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.InvoiceListResponse, error) {
	return m.listInvoicesFn(ctx, userID, page, pageSize)
}

// setupInvoiceTestApp creates a Fiber app with the invoice handler and auth middleware.
func setupInvoiceTestApp(svc *mockInvoiceService) *fiber.App {
	app := fiber.New()
	h := NewInvoiceHandler(svc)
	authMW := middleware.NewAuthMiddleware(testSecret)
	api := app.Group("/api/v1")
	h.RegisterRoutes(api, authMW)
	return app
}

func invoiceToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, "developer", testSecret, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

func parseInvoiceResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

// --- ListInvoices tests ---

func TestListInvoices_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockInvoiceService{
		listInvoicesFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.InvoiceListResponse, error) {
			return &dto.InvoiceListResponse{
				Invoices: []dto.InvoiceResponse{
					{
						ID:            uuid.New().String(),
						InvoiceNumber: "INV-20250101-00001",
						Amount:        50000,
						Currency:      "IDR",
						Status:        "unpaid",
						DueDate:       time.Now().Add(24 * time.Hour),
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

	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+invoiceToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseInvoiceResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestListInvoices_DefaultPagination_Returns200(t *testing.T) {
	uid := uuid.New()
	var capturedPage, capturedPageSize int
	svc := &mockInvoiceService{
		listInvoicesFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.InvoiceListResponse, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return &dto.InvoiceListResponse{
				Invoices: []dto.InvoiceResponse{},
				Pagination: dto.PaginationResponse{
					Page:       page,
					PageSize:   pageSize,
					TotalItems: 0,
					TotalPages: 0,
				},
			}, nil
		},
	}

	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/", nil)
	req.Header.Set("Authorization", "Bearer "+invoiceToken(t, uid))
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

func TestListInvoices_NoToken_Returns401(t *testing.T) {
	svc := &mockInvoiceService{}
	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListInvoices_ServiceError_Returns500(t *testing.T) {
	svc := &mockInvoiceService{
		listInvoicesFn: func(_ context.Context, _ uuid.UUID, _, _ int) (*dto.InvoiceListResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/", nil)
	req.Header.Set("Authorization", "Bearer "+invoiceToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- GetInvoice tests ---

func TestGetInvoice_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	invID := uuid.New()
	svc := &mockInvoiceService{
		getInvoiceFn: func(_ context.Context, id uuid.UUID) (*dto.InvoiceResponse, error) {
			return &dto.InvoiceResponse{
				ID:            id.String(),
				InvoiceNumber: "INV-20250101-00001",
				Amount:        50000,
				Currency:      "IDR",
				Status:        "paid",
				DueDate:       time.Now().Add(24 * time.Hour),
				CreatedAt:     time.Now(),
			}, nil
		},
	}

	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+invID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+invoiceToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseInvoiceResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestGetInvoice_InvalidUUID_Returns400(t *testing.T) {
	svc := &mockInvoiceService{}
	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+invoiceToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetInvoice_NotFound_Returns404(t *testing.T) {
	svc := &mockInvoiceService{
		getInvoiceFn: func(_ context.Context, _ uuid.UUID) (*dto.InvoiceResponse, error) {
			return nil, apierror.NewNotFound("invoice not found")
		},
	}

	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+invoiceToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetInvoice_NoToken_Returns401(t *testing.T) {
	svc := &mockInvoiceService{}
	app := setupInvoiceTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/"+uuid.New().String(), nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
