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
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// mockAPIKeyService implements service.APIKeyService for testing.
type mockAPIKeyService struct {
	createKeyFn   func(ctx context.Context, userID uuid.UUID, req *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error)
	listKeysFn    func(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.APIKeyListResponse, error)
	revokeKeyFn   func(ctx context.Context, userID uuid.UUID, keyID uuid.UUID) error
	validateKeyFn func(ctx context.Context, rawKey string) (*model.APIKey, error)
}

func (m *mockAPIKeyService) CreateKey(ctx context.Context, userID uuid.UUID, req *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error) {
	return m.createKeyFn(ctx, userID, req)
}

func (m *mockAPIKeyService) ListKeys(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.APIKeyListResponse, error) {
	return m.listKeysFn(ctx, userID, page, pageSize)
}

func (m *mockAPIKeyService) RevokeKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID) error {
	return m.revokeKeyFn(ctx, userID, keyID)
}

func (m *mockAPIKeyService) ValidateKey(ctx context.Context, rawKey string) (*model.APIKey, error) {
	return m.validateKeyFn(ctx, rawKey)
}

// helper to set up a Fiber app with the API key handler and auth middleware.
func setupAPIKeyTestApp(svc *mockAPIKeyService) *fiber.App {
	app := fiber.New()
	h := NewAPIKeyHandler(svc)
	authMW := middleware.NewAuthMiddleware(testSecret)
	api := app.Group("/api/v1")
	h.RegisterRoutes(api, authMW)
	return app
}

func apiKeyToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, "developer", testSecret, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

func apiKeyToJSON(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func parseAPIKeyResponse(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal response: %v\nbody: %s", err, string(body))
	}
	return r
}

// --- CreateKey tests ---

func TestCreateKey_Success_Returns201(t *testing.T) {
	uid := uuid.New()
	keyID := uuid.New()
	svc := &mockAPIKeyService{
		createKeyFn: func(_ context.Context, userID uuid.UUID, req *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error) {
			return &dto.APIKeyCreatedResponse{
				APIKeyResponse: dto.APIKeyResponse{
					ID:        keyID.String(),
					Name:      req.Name,
					KeyPrefix: "test_key_abcd",
					IsActive:  true,
					RateLimit: 1000,
					CreatedAt: time.Now(),
				},
				FullKey: "test_key_abcdef1234567890abcdef1234567890",
			}, nil
		},
	}

	app := setupAPIKeyTestApp(svc)
	body := apiKeyToJSON(t, dto.CreateAPIKeyRequest{Name: "My Test Key"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	r := parseAPIKeyResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestCreateKey_InvalidBody_Returns400(t *testing.T) {
	svc := &mockAPIKeyService{}
	app := setupAPIKeyTestApp(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys/", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateKey_MissingName_Returns400(t *testing.T) {
	svc := &mockAPIKeyService{}
	app := setupAPIKeyTestApp(svc)

	body := apiKeyToJSON(t, map[string]string{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	r := parseAPIKeyResponse(t, resp)
	if r.Success {
		t.Error("expected success=false")
	}
}

func TestCreateKey_NoToken_Returns401(t *testing.T) {
	svc := &mockAPIKeyService{}
	app := setupAPIKeyTestApp(svc)

	body := apiKeyToJSON(t, dto.CreateAPIKeyRequest{Name: "Key"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys/", body)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateKey_ServiceError_Returns500(t *testing.T) {
	svc := &mockAPIKeyService{
		createKeyFn: func(_ context.Context, _ uuid.UUID, _ *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error) {
			return nil, apierror.NewInternalError("failed to generate api key")
		},
	}

	app := setupAPIKeyTestApp(svc)
	body := apiKeyToJSON(t, dto.CreateAPIKeyRequest{Name: "Key"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- ListKeys tests ---

func TestListKeys_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	svc := &mockAPIKeyService{
		listKeysFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.APIKeyListResponse, error) {
			return &dto.APIKeyListResponse{
				Keys: []dto.APIKeyResponse{
					{
						ID:        uuid.New().String(),
						Name:      "Key 1",
						KeyPrefix: "test_key_abcd",
						IsActive:  true,
						RateLimit: 1000,
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

	app := setupAPIKeyTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys/?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseAPIKeyResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestListKeys_DefaultPagination_Returns200(t *testing.T) {
	uid := uuid.New()
	var capturedPage, capturedPageSize int
	svc := &mockAPIKeyService{
		listKeysFn: func(_ context.Context, _ uuid.UUID, page, pageSize int) (*dto.APIKeyListResponse, error) {
			capturedPage = page
			capturedPageSize = pageSize
			return &dto.APIKeyListResponse{
				Keys: []dto.APIKeyResponse{},
				Pagination: dto.PaginationResponse{
					Page:       page,
					PageSize:   pageSize,
					TotalItems: 0,
					TotalPages: 0,
				},
			}, nil
		},
	}

	app := setupAPIKeyTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys/", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uid))
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

func TestListKeys_NoToken_Returns401(t *testing.T) {
	svc := &mockAPIKeyService{}
	app := setupAPIKeyTestApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys/", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- RevokeKey tests ---

func TestRevokeKey_Success_Returns200(t *testing.T) {
	uid := uuid.New()
	keyID := uuid.New()
	svc := &mockAPIKeyService{
		revokeKeyFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
			return nil
		},
	}

	app := setupAPIKeyTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/"+keyID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uid))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	r := parseAPIKeyResponse(t, resp)
	if !r.Success {
		t.Error("expected success=true")
	}
}

func TestRevokeKey_InvalidUUID_Returns400(t *testing.T) {
	svc := &mockAPIKeyService{}
	app := setupAPIKeyTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/not-a-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRevokeKey_NotFound_Returns404(t *testing.T) {
	svc := &mockAPIKeyService{
		revokeKeyFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
			return apierror.NewNotFound("api key not found")
		},
	}

	app := setupAPIKeyTestApp(svc)
	keyID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/"+keyID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRevokeKey_Forbidden_Returns403(t *testing.T) {
	svc := &mockAPIKeyService{
		revokeKeyFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
			return apierror.NewForbidden("you do not have permission to revoke this key")
		},
	}

	app := setupAPIKeyTestApp(svc)
	keyID := uuid.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/"+keyID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyToken(t, uuid.New()))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRevokeKey_NoToken_Returns401(t *testing.T) {
	svc := &mockAPIKeyService{}
	app := setupAPIKeyTestApp(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/"+uuid.New().String(), nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
