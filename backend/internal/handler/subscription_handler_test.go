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

// mockSubscriptionService implements service.SubscriptionService for testing.
type mockSubscriptionService struct {
	createProductFn      func(ctx context.Context, userID uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error)
	listProductsFn       func(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.ProductListResponse, error)
	createPlanFn         func(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req *dto.CreatePlanRequest) (*dto.PlanResponse, error)
	createSubscriptionFn func(ctx context.Context, userID uuid.UUID, req *dto.CreateSubscriptionRequest) (*dto.SubscriptionResponse, error)
	activateSubFn        func(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error)
	cancelSubFn          func(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error)
	listSubsFn           func(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.SubscriptionListResponse, error)
}

func (m *mockSubscriptionService) CreateProduct(ctx context.Context, userID uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error) {
	return m.createProductFn(ctx, userID, req)
}

func (m *mockSubscriptionService) ListProducts(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.ProductListResponse, error) {
	return m.listProductsFn(ctx, userID, page, pageSize)
}

func (m *mockSubscriptionService) CreatePlan(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req *dto.CreatePlanRequest) (*dto.PlanResponse, error) {
	return m.createPlanFn(ctx, userID, productID, req)
}

func (m *mockSubscriptionService) CreateSubscription(ctx context.Context, userID uuid.UUID, req *dto.CreateSubscriptionRequest) (*dto.SubscriptionResponse, error) {
	return m.createSubscriptionFn(ctx, userID, req)
}

func (m *mockSubscriptionService) ActivateSubscription(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error) {
	if m.activateSubFn != nil {
		return m.activateSubFn(ctx, subscriptionID)
	}
	return nil, nil
}

func (m *mockSubscriptionService) CancelSubscription(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error) {
	return m.cancelSubFn(ctx, subscriptionID)
}

func (m *mockSubscriptionService) ListSubscriptions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.SubscriptionListResponse, error) {
	return m.listSubsFn(ctx, userID, page, pageSize)
}

// setupSubscriptionTestApp creates a Fiber app with the subscription handler and auth middleware.
func setupSubscriptionTestApp(svc *mockSubscriptionService) *fiber.App {
	app := fiber.New()
	h := NewSubscriptionHandler(svc)
	authMW := middleware.NewAuthMiddleware(testSecret)
	api := app.Group("/api/v1")
	h.RegisterRoutes(api, authMW)
	return app
}

func subToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, "developer", testSecret, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

func subToJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func parseSubResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

// --- CreateProduct tests ---

func TestCreateProduct_Success_Returns201(t *testing.T) {
	uid := uuid.New()
	svc := &mockSubscriptionService{
		createProductFn: func(_ context.Context, _ uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error) {
			return &dto.ProductResponse{
				ID:          uuid.New().String(),
				Name:        req.Name,
				Description: req.Description,
				IsActive:    true,
				CreatedAt:   time.Now(),
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	body := subToJSON(t, dto.CreateProductRequest{Name: "Premium Plan", Description: "Premium features"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	r := parseSubResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestCreateProduct_InvalidBody_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateProduct_MissingName_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	body := subToJSON(t, map[string]interface{}{"description": "no name"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateProduct_NoToken_Returns401(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	body := subToJSON(t, dto.CreateProductRequest{Name: "Test"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateProduct_ServiceError_Returns500(t *testing.T) {
	svc := &mockSubscriptionService{
		createProductFn: func(_ context.Context, _ uuid.UUID, _ *dto.CreateProductRequest) (*dto.ProductResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupSubscriptionTestApp(svc)
	body := subToJSON(t, dto.CreateProductRequest{Name: "Test"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- ListProducts tests ---

func TestListProducts_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockSubscriptionService{
		listProductsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.ProductListResponse, error) {
			return &dto.ProductListResponse{
				Products: []dto.ProductResponse{
					{ID: uuid.New().String(), Name: "Product A", IsActive: true, CreatedAt: time.Now()},
				},
				Pagination: dto.PaginationResponse{Page: page, PageSize: pageSize, TotalItems: 1, TotalPages: 1},
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	r := parseSubResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestListProducts_DefaultPagination_Returns200(t *testing.T) {
	uid := uuid.New()
	var capturedPage, capturedPageSize int
	svc := &mockSubscriptionService{
		listProductsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.ProductListResponse, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return &dto.ProductListResponse{
				Products:   []dto.ProductResponse{},
				Pagination: dto.PaginationResponse{Page: page, PageSize: pageSize, TotalItems: 0, TotalPages: 0},
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
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

func TestListProducts_NoToken_Returns401(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListProducts_ServiceError_Returns500(t *testing.T) {
	svc := &mockSubscriptionService{
		listProductsFn: func(_ context.Context, _ uuid.UUID, _, _ int) (*dto.ProductListResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- CreatePlan tests ---

func TestCreatePlan_Success_Returns201(t *testing.T) {
	uid := uuid.New()
	productID := uuid.New()
	svc := &mockSubscriptionService{
		createPlanFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, req *dto.CreatePlanRequest) (*dto.PlanResponse, error) {
			return &dto.PlanResponse{
				ID:              uuid.New().String(),
				ProductID:       productID.String(),
				Name:            req.Name,
				Amount:          req.Amount,
				Currency:        req.Currency,
				BillingInterval: req.BillingInterval,
				IsActive:        true,
				CreatedAt:       time.Now(),
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	body := subToJSON(t, dto.CreatePlanRequest{Name: "Monthly", Amount: 99000, Currency: "IDR", BillingInterval: "monthly"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+productID.String()+"/plans", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	r := parseSubResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestCreatePlan_InvalidProductID_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	body := subToJSON(t, dto.CreatePlanRequest{Name: "Monthly", Amount: 99000, Currency: "IDR", BillingInterval: "monthly"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/not-a-uuid/plans", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreatePlan_InvalidBody_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+uuid.New().String()+"/plans", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreatePlan_MissingFields_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	body := subToJSON(t, map[string]interface{}{"name": "Monthly"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+uuid.New().String()+"/plans", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreatePlan_NoToken_Returns401(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	body := subToJSON(t, dto.CreatePlanRequest{Name: "Monthly", Amount: 99000, Currency: "IDR", BillingInterval: "monthly"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+uuid.New().String()+"/plans", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreatePlan_Forbidden_Returns403(t *testing.T) {
	svc := &mockSubscriptionService{
		createPlanFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ *dto.CreatePlanRequest) (*dto.PlanResponse, error) {
			return nil, apierror.NewForbidden("product does not belong to user")
		},
	}

	app := setupSubscriptionTestApp(svc)
	body := subToJSON(t, dto.CreatePlanRequest{Name: "Monthly", Amount: 99000, Currency: "IDR", BillingInterval: "monthly"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+uuid.New().String()+"/plans", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// --- CreateSubscription tests ---

func TestCreateSubscription_Success_Returns201(t *testing.T) {
	uid := uuid.New()
	planID := uuid.New()
	svc := &mockSubscriptionService{
		createSubscriptionFn: func(_ context.Context, _ uuid.UUID, req *dto.CreateSubscriptionRequest) (*dto.SubscriptionResponse, error) {
			return &dto.SubscriptionResponse{
				ID:                 uuid.New().String(),
				PlanID:             req.PlanID,
				Status:             "pending_payment",
				CurrentPeriodStart: time.Now(),
				CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0),
				CreatedAt:          time.Now(),
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	body := subToJSON(t, dto.CreateSubscriptionRequest{PlanID: planID.String()})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	r := parseSubResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestCreateSubscription_InvalidBody_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateSubscription_InvalidPlanID_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	body := subToJSON(t, map[string]interface{}{"plan_id": "not-a-uuid"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateSubscription_NoToken_Returns401(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	body := subToJSON(t, dto.CreateSubscriptionRequest{PlanID: uuid.New().String()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateSubscription_ServiceError_Returns500(t *testing.T) {
	svc := &mockSubscriptionService{
		createSubscriptionFn: func(_ context.Context, _ uuid.UUID, _ *dto.CreateSubscriptionRequest) (*dto.SubscriptionResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupSubscriptionTestApp(svc)
	body := subToJSON(t, dto.CreateSubscriptionRequest{PlanID: uuid.New().String()})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- ListSubscriptions tests ---

func TestListSubscriptions_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockSubscriptionService{
		listSubsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.SubscriptionListResponse, error) {
			return &dto.SubscriptionListResponse{
				Subscriptions: []dto.SubscriptionResponse{
					{
						ID:                 uuid.New().String(),
						PlanID:             uuid.New().String(),
						Status:             "active",
						CurrentPeriodStart: time.Now(),
						CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0),
						CreatedAt:          time.Now(),
					},
				},
				Pagination: dto.PaginationResponse{Page: page, PageSize: pageSize, TotalItems: 1, TotalPages: 1},
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	r := parseSubResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestListSubscriptions_DefaultPagination_Returns200(t *testing.T) {
	uid := uuid.New()
	var capturedPage, capturedPageSize int
	svc := &mockSubscriptionService{
		listSubsFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.SubscriptionListResponse, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return &dto.SubscriptionListResponse{
				Subscriptions: []dto.SubscriptionResponse{},
				Pagination:    dto.PaginationResponse{Page: page, PageSize: pageSize, TotalItems: 0, TotalPages: 0},
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
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

func TestListSubscriptions_NoToken_Returns401(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestListSubscriptions_ServiceError_Returns500(t *testing.T) {
	svc := &mockSubscriptionService{
		listSubsFn: func(_ context.Context, _ uuid.UUID, _, _ int) (*dto.SubscriptionListResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- CancelSubscription tests ---

func TestCancelSubscription_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	subID := uuid.New()
	now := time.Now()
	svc := &mockSubscriptionService{
		cancelSubFn: func(_ context.Context, id uuid.UUID) (*dto.SubscriptionResponse, error) {
			return &dto.SubscriptionResponse{
				ID:                 id.String(),
				PlanID:             uuid.New().String(),
				Status:             "cancelled",
				CurrentPeriodStart: now,
				CurrentPeriodEnd:   now.AddDate(0, 1, 0),
				CancelledAt:        &now,
				CreatedAt:          now,
			}, nil
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/"+subID.String()+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	r := parseSubResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestCancelSubscription_InvalidUUID_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/not-a-uuid/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCancelSubscription_NotActive_Returns400(t *testing.T) {
	svc := &mockSubscriptionService{
		cancelSubFn: func(_ context.Context, _ uuid.UUID) (*dto.SubscriptionResponse, error) {
			return nil, apierror.NewBadRequest("only active subscriptions can be cancelled")
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/"+uuid.New().String()+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCancelSubscription_NoToken_Returns401(t *testing.T) {
	svc := &mockSubscriptionService{}
	app := setupSubscriptionTestApp(svc)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/"+uuid.New().String()+"/cancel", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCancelSubscription_NotFound_Returns404(t *testing.T) {
	svc := &mockSubscriptionService{
		cancelSubFn: func(_ context.Context, _ uuid.UUID) (*dto.SubscriptionResponse, error) {
			return nil, apierror.NewNotFound("subscription not found")
		},
	}

	app := setupSubscriptionTestApp(svc)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/"+uuid.New().String()+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+subToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
