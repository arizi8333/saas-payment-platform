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
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// mockWebhookService implements service.WebhookService for testing.
type mockWebhookService struct {
	registerEndpointFn func(ctx context.Context, userID uuid.UUID, req *dto.CreateWebhookRequest) (*dto.WebhookResponse, error)
	listEndpointsFn    func(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookListResponse, error)
	deleteEndpointFn   func(ctx context.Context, userID uuid.UUID, endpointID uuid.UUID) error
	triggerWebhookFn   func(ctx context.Context, userID uuid.UUID, eventType string, payload map[string]interface{}) error
	getDeliveriesFn    func(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookDeliveryListResponse, error)
	generateSigFn      func(payload []byte, secret string) string
}

func (m *mockWebhookService) RegisterEndpoint(ctx context.Context, userID uuid.UUID, req *dto.CreateWebhookRequest) (*dto.WebhookResponse, error) {
	return m.registerEndpointFn(ctx, userID, req)
}

func (m *mockWebhookService) ListEndpoints(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookListResponse, error) {
	return m.listEndpointsFn(ctx, userID, page, pageSize)
}

func (m *mockWebhookService) DeleteEndpoint(ctx context.Context, userID uuid.UUID, endpointID uuid.UUID) error {
	return m.deleteEndpointFn(ctx, userID, endpointID)
}

func (m *mockWebhookService) TriggerWebhook(ctx context.Context, userID uuid.UUID, eventType string, payload map[string]interface{}) error {
	if m.triggerWebhookFn != nil {
		return m.triggerWebhookFn(ctx, userID, eventType, payload)
	}
	return nil
}

func (m *mockWebhookService) GetDeliveries(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.WebhookDeliveryListResponse, error) {
	return m.getDeliveriesFn(ctx, userID, page, pageSize)
}

func (m *mockWebhookService) GenerateSignature(payload []byte, secret string) string {
	if m.generateSigFn != nil {
		return m.generateSigFn(payload, secret)
	}
	return ""
}

// setupWebhookTestApp creates a Fiber app with the webhook handler and auth middleware.
func setupWebhookTestApp(svc *mockWebhookService) *fiber.App {
	app := fiber.New()
	h := NewWebhookHandler(svc)
	authMW := middleware.NewAuthMiddleware(testSecret)
	api := app.Group("/api/v1")
	h.RegisterRoutes(api, authMW)
	return app
}

func webhookToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, "developer", testSecret, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

func webhookToJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func parseWebhookResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

// --- RegisterEndpoint tests ---

func TestRegisterEndpoint_Success_Returns201(t *testing.T) {
	uid := uuid.New()
	svc := &mockWebhookService{
		registerEndpointFn: func(_ context.Context, _ uuid.UUID, req *dto.CreateWebhookRequest) (*dto.WebhookResponse, error) {
			return &dto.WebhookResponse{
				ID:        uuid.New().String(),
				URL:       req.URL,
				Secret:    "whsec_mock",
				Events:    req.Events,
				IsActive:  true,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	app := setupWebhookTestApp(svc)
	body := webhookToJSON(t, dto.CreateWebhookRequest{
		URL:    "https://example.com/webhook",
		Events: []string{"transaction.success"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/endpoints", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	r := parseWebhookResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestRegisterEndpoint_InvalidBody_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/endpoints", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRegisterEndpoint_MissingURL_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	app := setupWebhookTestApp(svc)

	body := webhookToJSON(t, map[string]interface{}{
		"events": []string{"transaction.success"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/endpoints", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	r := parseWebhookResponse(t, resp)
	if r.Success {
		t.Error("expected success=false")
	}
}

func TestRegisterEndpoint_NoToken_Returns401(t *testing.T) {
	svc := &mockWebhookService{}
	app := setupWebhookTestApp(svc)

	body := webhookToJSON(t, dto.CreateWebhookRequest{
		URL:    "https://example.com/webhook",
		Events: []string{"transaction.success"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/endpoints", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRegisterEndpoint_ServiceError_Returns500(t *testing.T) {
	svc := &mockWebhookService{
		registerEndpointFn: func(_ context.Context, _ uuid.UUID, _ *dto.CreateWebhookRequest) (*dto.WebhookResponse, error) {
			return nil, apierror.NewInternalError("failed to register endpoint")
		},
	}

	app := setupWebhookTestApp(svc)
	body := webhookToJSON(t, dto.CreateWebhookRequest{
		URL:    "https://example.com/webhook",
		Events: []string{"transaction.success"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/endpoints", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- ListEndpoints tests ---

func TestListEndpoints_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockWebhookService{
		listEndpointsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.WebhookListResponse, error) {
			return &dto.WebhookListResponse{
				Endpoints: []dto.WebhookResponse{
					{
						ID:        uuid.New().String(),
						URL:       "https://example.com/webhook",
						Events:    []string{"transaction.success"},
						IsActive:  true,
						CreatedAt: time.Now(),
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

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/endpoints?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseWebhookResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestListEndpoints_DefaultPagination_Returns200(t *testing.T) {
	uid := uuid.New()
	var capturedPage, capturedPageSize int
	svc := &mockWebhookService{
		listEndpointsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.WebhookListResponse, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return &dto.WebhookListResponse{
				Endpoints: []dto.WebhookResponse{},
				Pagination: dto.PaginationResponse{
					Page:       page,
					PageSize:   pageSize,
					TotalItems: 0,
					TotalPages: 0,
				},
			}, nil
		},
	}

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/endpoints", nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uid))
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

func TestListEndpoints_NoToken_Returns401(t *testing.T) {
	svc := &mockWebhookService{}
	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/endpoints", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListEndpoints_ServiceError_Returns500(t *testing.T) {
	svc := &mockWebhookService{
		listEndpointsFn: func(_ context.Context, _ uuid.UUID, _, _ int) (*dto.WebhookListResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/endpoints", nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- DeleteEndpoint tests ---

func TestDeleteEndpoint_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	epID := uuid.New()
	svc := &mockWebhookService{
		deleteEndpointFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
			return nil
		},
	}

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/endpoints/"+epID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseWebhookResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestDeleteEndpoint_InvalidUUID_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/endpoints/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestDeleteEndpoint_NotFound_Returns404(t *testing.T) {
	svc := &mockWebhookService{
		deleteEndpointFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
			return apierror.NewNotFound("webhook endpoint not found")
		},
	}

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/endpoints/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteEndpoint_Forbidden_Returns403(t *testing.T) {
	svc := &mockWebhookService{
		deleteEndpointFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
			return apierror.NewForbidden("you do not have permission to delete this endpoint")
		},
	}

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/endpoints/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDeleteEndpoint_NoToken_Returns401(t *testing.T) {
	svc := &mockWebhookService{}
	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/endpoints/"+uuid.New().String(), nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- ListDeliveries tests ---

func TestListDeliveries_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockWebhookService{
		getDeliveriesFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.WebhookDeliveryListResponse, error) {
			return &dto.WebhookDeliveryListResponse{
				Deliveries: []dto.WebhookDeliveryResponse{
					{
						ID:         uuid.New().String(),
						EventType:  "transaction.success",
						Status:     "delivered",
						RetryCount: 0,
						CreatedAt:  time.Now(),
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

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/deliveries?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseWebhookResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestListDeliveries_DefaultPagination_Returns200(t *testing.T) {
	uid := uuid.New()
	var capturedPage, capturedPageSize int
	svc := &mockWebhookService{
		getDeliveriesFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.WebhookDeliveryListResponse, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return &dto.WebhookDeliveryListResponse{
				Deliveries: []dto.WebhookDeliveryResponse{},
				Pagination: dto.PaginationResponse{
					Page:       page,
					PageSize:   pageSize,
					TotalItems: 0,
					TotalPages: 0,
				},
			}, nil
		},
	}

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/deliveries", nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uid))
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

func TestListDeliveries_NoToken_Returns401(t *testing.T) {
	svc := &mockWebhookService{}
	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/deliveries", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListDeliveries_ServiceError_Returns500(t *testing.T) {
	svc := &mockWebhookService{
		getDeliveriesFn: func(_ context.Context, _ uuid.UUID, _, _ int) (*dto.WebhookDeliveryListResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupWebhookTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/deliveries", nil)
	req.Header.Set("Authorization", "Bearer "+webhookToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
