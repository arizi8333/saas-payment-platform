package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
)

// Context key constants for Fiber locals.
const (
	LocalsUserID = "user_id"
	LocalsRole   = "role"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey string

const (
	// CtxUserIDKey is the context key for the authenticated user's ID.
	CtxUserIDKey contextKey = "user_id"
	// CtxRoleKey is the context key for the authenticated user's role.
	CtxRoleKey contextKey = "role"
)

// AuthMiddleware provides JWT-based authentication and role-based authorization.
type AuthMiddleware struct {
	jwtSecret string
}

// NewAuthMiddleware creates a new AuthMiddleware with the given JWT secret.
// The secret MUST NOT be logged.
func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{jwtSecret: jwtSecret}
}

// JWTProtected returns a Fiber handler that validates the Bearer token from the
// Authorization header. On success it stores user_id (uuid.UUID) and role
// (string) in Fiber locals and the request context for downstream use.
// Returns 401 for missing, malformed, invalid, or expired tokens.
func (m *AuthMiddleware) JWTProtected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.ErrorResponse(c, fiber.StatusUnauthorized, "missing authorization header")
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid authorization header format")
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			return response.ErrorResponse(c, fiber.StatusUnauthorized, "missing token")
		}

		claims, err := auth.ValidateToken(tokenString, m.jwtSecret)
		if err != nil {
			return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid or expired token")
		}

		// Store in Fiber locals for handler-level access.
		c.Locals(LocalsUserID, claims.UserID)
		c.Locals(LocalsRole, claims.Role)

		// Propagate through context for service/repository layers.
		ctx := c.UserContext()
		ctx = context.WithValue(ctx, CtxUserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, CtxRoleKey, claims.Role)
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// AdminOnly returns a Fiber handler that checks the authenticated user's role.
// Returns 403 Forbidden if the role is not "admin".
// Must be used after JWTProtected.
func (m *AuthMiddleware) AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals(LocalsRole).(string)
		if !ok || role != "admin" {
			return response.ErrorResponse(c, fiber.StatusForbidden, "admin access required")
		}

		return c.Next()
	}
}

// UserIDFromContext extracts the authenticated user's ID from the context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxUserIDKey).(uuid.UUID)
	return id, ok
}

// RoleFromContext extracts the authenticated user's role from the context.
func RoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(CtxRoleKey).(string)
	return role, ok
}
