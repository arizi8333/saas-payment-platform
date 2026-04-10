package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/middleware"
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// mockAdminService implements service.AdminService for testing.
type mockAdminService struct {
	getDashboardStatsFn func(ctx context.Context) (*dto.DashboardStatsResponse, error)
}

func (m *mockAdminService) GetDashboardStats(ctx context.Context) (*dto.DashboardStatsResponse, error) {
	return m.getDashboardStatsFn(ctx)
}

func setupAdminTestApp(svc *mockAdminService) *fiber.App {
	app := fiber.New()
	h := NewAdminHandler(svc)
	authMW := middleware.NewAuthMiddleware(testSecret)
	api := app.Group("/api/v1")
	h.RegisterRoutes(api, authMW)
	return app
}

func adminToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, "admin", testSecret, 24)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}
	return tok
}

func developerToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, "developer", testSecret, 24)
	if err != nil {
		t.Fatalf("generate developer token: %v", err)
	}
	return tok
}

func parseAdminResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

// --- GetDashboardStats tests ---

func TestGetDashboardStats_AdminSuccess_Returns200(t *testing.T) {
	svc := &mockAdminService{
		getDashboardStatsFn: func(_ context.Context) (*dto.DashboardStatsResponse, error) {
			return &dto.DashboardStatsResponse{
				Transactions: dto.TransactionStats{
					Total: 100, Pending: 10, Success: 80, Failed: 10,
				},
				ActiveUsers: 25,
				WebhookStats: dto.WebhookStats{
					TotalDeliveries: 50, Delivered: 45, Failed: 3, Pending: 2, SuccessRate: 90.0,
				},
			}, nil
		},
	}

	app := setupAdminTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t, uuid.New()))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseAdminResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestGetDashboardStats_NoToken_Returns401(t *testing.T) {
	svc := &mockAdminService{}
	app := setupAdminTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetDashboardStats_DeveloperRole_Returns403(t *testing.T) {
	svc := &mockAdminService{}
	app := setupAdminTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+developerToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestGetDashboardStats_ServiceError_Returns500(t *testing.T) {
	svc := &mockAdminService{
		getDashboardStatsFn: func(_ context.Context) (*dto.DashboardStatsResponse, error) {
			return nil, apierror.NewInternalError("database error")
		},
	}

	app := setupAdminTestApp(svc)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	r := parseAdminResponse(t, resp)
	if r.Success {
		t.Error("expected success=false")
	}
}
