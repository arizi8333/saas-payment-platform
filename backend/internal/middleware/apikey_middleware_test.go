package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
)

// mockAPIKeyService implements service.APIKeyService for testing.
type mockAPIKeyService struct {
	validateKeyFn func(ctx context.Context, rawKey string) (*model.APIKey, error)
}

func (m *mockAPIKeyService) CreateKey(_ context.Context, _ uuid.UUID, _ *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error) {
	return nil, nil
}

func (m *mockAPIKeyService) ListKeys(_ context.Context, _ uuid.UUID, _, _ int) (*dto.APIKeyListResponse, error) {
	return nil, nil
}

func (m *mockAPIKeyService) RevokeKey(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}

func (m *mockAPIKeyService) ValidateKey(ctx context.Context, rawKey string) (*model.APIKey, error) {
	return m.validateKeyFn(ctx, rawKey)
}

func newTestAPIKey(userID uuid.UUID, role model.UserRole) *model.APIKey {
	return &model.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      "test-key",
		KeyHash:   "hash",
		KeyPrefix: "test_key_abcd",
		IsActive:  true,
		RateLimit: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		User: model.User{
			ID:   userID,
			Role: role,
		},
	}
}

// --- ValidateAPIKey tests ---

func TestValidateAPIKey_ValidKey_SetsLocalsAndContext(t *testing.T) {
	userID := uuid.New()
	apiKey := newTestAPIKey(userID, model.RoleDeveloper)

	svc := &mockAPIKeyService{
		validateKeyFn: func(_ context.Context, _ string) (*model.APIKey, error) {
			return apiKey, nil
		},
	}

	app := fiber.New()
	mw := NewAPIKeyMiddleware(svc)

	var gotUserID uuid.UUID
	var gotRole string
	var gotAPIKey *model.APIKey

	app.Use(mw.ValidateAPIKey())
	app.Get("/test", func(c *fiber.Ctx) error {
		gotUserID, _ = c.Locals(LocalsUserID).(uuid.UUID)
		gotRole, _ = c.Locals(LocalsRole).(string)
		gotAPIKey, _ = c.Locals(LocalsAPIKey).(*model.APIKey)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test_key_testapikey1234")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if gotUserID != userID {
		t.Errorf("expected user_id %s, got %s", userID, gotUserID)
	}
	if gotRole != string(model.RoleDeveloper) {
		t.Errorf("expected role %q, got %q", model.RoleDeveloper, gotRole)
	}
	if gotAPIKey == nil || gotAPIKey.ID != apiKey.ID {
		t.Errorf("expected api_key in locals")
	}
}

func TestValidateAPIKey_ValidKey_PropagatesContext(t *testing.T) {
	userID := uuid.New()
	apiKey := newTestAPIKey(userID, model.RoleAdmin)

	svc := &mockAPIKeyService{
		validateKeyFn: func(_ context.Context, _ string) (*model.APIKey, error) {
			return apiKey, nil
		},
	}

	app := fiber.New()
	mw := NewAPIKeyMiddleware(svc)

	var ctxUserID uuid.UUID
	var ctxRole string
	var okID, okRole bool

	app.Use(mw.ValidateAPIKey())
	app.Get("/test", func(c *fiber.Ctx) error {
		ctxUserID, okID = UserIDFromContext(c.UserContext())
		ctxRole, okRole = RoleFromContext(c.UserContext())
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test_key_testapikey1234")

	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !okID || ctxUserID != userID {
		t.Errorf("context user_id: expected %s, got %s (ok=%v)", userID, ctxUserID, okID)
	}
	if !okRole || ctxRole != string(model.RoleAdmin) {
		t.Errorf("context role: expected %q, got %q (ok=%v)", model.RoleAdmin, ctxRole, okRole)
	}
}

func TestValidateAPIKey_MissingHeader_Returns401(t *testing.T) {
	svc := &mockAPIKeyService{
		validateKeyFn: func(_ context.Context, _ string) (*model.APIKey, error) {
			return nil, errors.New("should not be called")
		},
	}

	app := fiber.New()
	mw := NewAPIKeyMiddleware(svc)
	app.Use(mw.ValidateAPIKey())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	body := parseBody(t, resp)
	if body.Success {
		t.Error("expected success=false")
	}
}

func TestValidateAPIKey_InvalidKey_Returns401(t *testing.T) {
	svc := &mockAPIKeyService{
		validateKeyFn: func(_ context.Context, _ string) (*model.APIKey, error) {
			return nil, errors.New("invalid api key")
		},
	}

	app := fiber.New()
	mw := NewAPIKeyMiddleware(svc)
	app.Use(mw.ValidateAPIKey())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test_key_invalidkey1234")

	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	body := parseBody(t, resp)
	if body.Success {
		t.Error("expected success=false")
	}
}

func TestValidateAPIKey_InactiveKey_Returns401(t *testing.T) {
	svc := &mockAPIKeyService{
		validateKeyFn: func(_ context.Context, _ string) (*model.APIKey, error) {
			return nil, errors.New("api key is inactive")
		},
	}

	app := fiber.New()
	mw := NewAPIKeyMiddleware(svc)
	app.Use(mw.ValidateAPIKey())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test_key_inactivekey12")

	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestValidateAPIKey_ExpiredKey_Returns401(t *testing.T) {
	svc := &mockAPIKeyService{
		validateKeyFn: func(_ context.Context, _ string) (*model.APIKey, error) {
			return nil, errors.New("api key has expired")
		},
	}

	app := fiber.New()
	mw := NewAPIKeyMiddleware(svc)
	app.Use(mw.ValidateAPIKey())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "test_key_expiredkey123")

	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
