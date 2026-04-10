package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
)

const testJWTSecret = "test-secret-for-middleware"

func generateTestToken(t *testing.T, userID uuid.UUID, role string, hours int) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, role, testJWTSecret, hours)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

func generateExpiredToken(t *testing.T, userID uuid.UUID, role string) string {
	t.Helper()
	claims := auth.Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "saas-payment-platform",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}
	return s
}

func parseBody(t *testing.T, resp *http.Response) response.APIResponse {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	var r response.APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return r
}

// --- JWTProtected tests ---

func TestJWTProtected_ValidToken_SetsLocalsAndContext(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	userID := uuid.New()
	role := "developer"

	var gotUserID uuid.UUID
	var gotRole string

	app.Use(mw.JWTProtected())
	app.Get("/test", func(c *fiber.Ctx) error {
		gotUserID, _ = c.Locals(LocalsUserID).(uuid.UUID)
		gotRole, _ = c.Locals(LocalsRole).(string)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestToken(t, userID, role, 24))

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
	if gotRole != role {
		t.Errorf("expected role %q, got %q", role, gotRole)
	}
}

func TestJWTProtected_MissingAuthHeader_Returns401(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected())
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

func TestJWTProtected_MalformedHeader_Returns401(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic abc123")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestJWTProtected_InvalidToken_Returns401(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestJWTProtected_ExpiredToken_Returns401(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+generateExpiredToken(t, uuid.New(), "developer"))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestJWTProtected_WrongSecret_Returns401(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	tok, _ := auth.GenerateToken(uuid.New(), "developer", "different-secret", 24)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- AdminOnly tests ---

func TestAdminOnly_AdminRole_Passes(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected(), mw.AdminOnly())
	app.Get("/admin", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestToken(t, uuid.New(), "admin", 24))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAdminOnly_DeveloperRole_Returns403(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected(), mw.AdminOnly())
	app.Get("/admin", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestToken(t, uuid.New(), "developer", 24))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAdminOnly_EndUserRole_Returns403(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	app.Use(mw.JWTProtected(), mw.AdminOnly())
	app.Get("/admin", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestToken(t, uuid.New(), "end_user", 24))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// --- Context helper tests ---

func TestJWTProtected_PropagatesUserContext(t *testing.T) {
	app := fiber.New()
	mw := NewAuthMiddleware(testJWTSecret)
	userID := uuid.New()
	role := "admin"

	var ctxUserID uuid.UUID
	var ctxRole string
	var okID, okRole bool

	app.Use(mw.JWTProtected())
	app.Get("/test", func(c *fiber.Ctx) error {
		ctxUserID, okID = UserIDFromContext(c.UserContext())
		ctxRole, okRole = RoleFromContext(c.UserContext())
		return c.SendStatus(200)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestToken(t, userID, role, 24))
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !okID || ctxUserID != userID {
		t.Errorf("context user_id: expected %s, got %s (ok=%v)", userID, ctxUserID, okID)
	}
	if !okRole || ctxRole != role {
		t.Errorf("context role: expected %q, got %q (ok=%v)", role, ctxRole, okRole)
	}
}
