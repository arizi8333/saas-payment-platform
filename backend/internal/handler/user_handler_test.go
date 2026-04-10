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

const testSecret = "test-handler-secret"

// mockUserService implements service.UserService for testing.
type mockUserService struct {
	registerFn   func(ctx context.Context, req *dto.RegisterRequest) (*dto.UserProfileResponse, error)
	loginFn      func(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
	getProfileFn func(ctx context.Context, userID uuid.UUID) (*dto.UserProfileResponse, error)
}

func (m *mockUserService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.UserProfileResponse, error) {
	return m.registerFn(ctx, req)
}

func (m *mockUserService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	return m.loginFn(ctx, req)
}

func (m *mockUserService) GetProfile(ctx context.Context, userID uuid.UUID) (*dto.UserProfileResponse, error) {
	return m.getProfileFn(ctx, userID)
}

// helper to set up a Fiber app with the user handler and auth middleware.
func setupTestApp(svc *mockUserService) *fiber.App {
	app := fiber.New()
	h := NewUserHandler(svc)
	authMW := middleware.NewAuthMiddleware(testSecret)
	api := app.Group("/api/v1")
	h.RegisterRoutes(api, authMW)
	return app
}

func toJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func parseResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

func testToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, "developer", testSecret, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

// --- Register tests ---

func TestRegister_Success_Returns201(t *testing.T) {
	uid := uuid.New()
	svc := &mockUserService{
		registerFn: func(_ context.Context, req *dto.RegisterRequest) (*dto.UserProfileResponse, error) {
			return &dto.UserProfileResponse{
				ID:        uid.String(),
				Email:     req.Email,
				FullName:  req.FullName,
				Role:      "developer",
				IsActive:  true,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	app := setupTestApp(svc)
	body := toJSON(t, dto.RegisterRequest{
		Email:    "test@example.com",
		Password: "securepass123",
		FullName: "Test User",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	r := parseResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestRegister_InvalidBody_Returns400(t *testing.T) {
	svc := &mockUserService{}
	app := setupTestApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRegister_MissingEmail_Returns400(t *testing.T) {
	svc := &mockUserService{}
	app := setupTestApp(svc)

	body := toJSON(t, map[string]string{
		"password":  "securepass123",
		"full_name": "Test User",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	r := parseResponse(t, resp)
	if r.Success {
		t.Error("expected success=false")
	}
}

func TestRegister_DuplicateEmail_Returns409(t *testing.T) {
	svc := &mockUserService{
		registerFn: func(_ context.Context, _ *dto.RegisterRequest) (*dto.UserProfileResponse, error) {
			return nil, apierror.NewConflict("email already registered")
		},
	}

	app := setupTestApp(svc)
	body := toJSON(t, dto.RegisterRequest{
		Email:    "dup@example.com",
		Password: "securepass123",
		FullName: "Dup User",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

// --- Login tests ---

func TestLogin_Success_Returns200(t *testing.T) {
	svc := &mockUserService{
		loginFn: func(_ context.Context, _ *dto.LoginRequest) (*dto.LoginResponse, error) {
			return &dto.LoginResponse{Token: "mock-token-value", ExpiresIn: 86400}, nil
		},
	}

	app := setupTestApp(svc)
	body := toJSON(t, dto.LoginRequest{
		Email:    "test@example.com",
		Password: "securepass123",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestLogin_InvalidCredentials_Returns401(t *testing.T) {
	svc := &mockUserService{
		loginFn: func(_ context.Context, _ *dto.LoginRequest) (*dto.LoginResponse, error) {
			return nil, apierror.NewUnauthorized("invalid email or password")
		},
	}

	app := setupTestApp(svc)
	body := toJSON(t, dto.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpass",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_MissingFields_Returns400(t *testing.T) {
	svc := &mockUserService{}
	app := setupTestApp(svc)

	body := toJSON(t, map[string]string{"email": "test@example.com"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- GetProfile tests ---

func TestGetProfile_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockUserService{
		getProfileFn: func(_ context.Context, userID uuid.UUID) (*dto.UserProfileResponse, error) {
			return &dto.UserProfileResponse{
				ID:        userID.String(),
				Email:     "test@example.com",
				FullName:  "Test User",
				Role:      "developer",
				IsActive:  true,
				CreatedAt: time.Now(),
			}, nil
		},
	}

	app := setupTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+testToken(t, uid))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestGetProfile_NoToken_Returns401(t *testing.T) {
	svc := &mockUserService{}
	app := setupTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetProfile_UserNotFound_Returns404(t *testing.T) {
	uid := uuid.New()
	svc := &mockUserService{
		getProfileFn: func(_ context.Context, _ uuid.UUID) (*dto.UserProfileResponse, error) {
			return nil, apierror.NewNotFound("user not found")
		},
	}

	app := setupTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+testToken(t, uid))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
