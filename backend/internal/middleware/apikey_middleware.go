package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
	"github.com/saas-payment-platform/backend/internal/service"
)

// LocalsAPIKey is the Fiber locals key for the validated API key model.
const LocalsAPIKey = "api_key"

// APIKeyMiddleware provides API key-based authentication for endpoints.
type APIKeyMiddleware struct {
	apiKeyService service.APIKeyService
}

// NewAPIKeyMiddleware creates a new APIKeyMiddleware with the given APIKeyService.
func NewAPIKeyMiddleware(apiKeyService service.APIKeyService) *APIKeyMiddleware {
	return &APIKeyMiddleware{apiKeyService: apiKeyService}
}

// ValidateAPIKey returns a Fiber handler that validates the X-API-Key header.
// On success it stores user_id (uuid.UUID), role (string), and the API key model
// in Fiber locals and the request context for downstream use.
// Returns 401 for missing, invalid, inactive, or expired keys.
func (m *APIKeyMiddleware) ValidateAPIKey() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawKey := c.Get("X-API-Key")
		if rawKey == "" {
			return response.ErrorResponse(c, fiber.StatusUnauthorized, "missing api key")
		}

		apiKey, err := m.apiKeyService.ValidateKey(c.UserContext(), rawKey)
		if err != nil {
			return response.ErrorResponse(c, fiber.StatusUnauthorized, "invalid or inactive api key")
		}

		// Store in Fiber locals for handler-level access (same keys as auth_middleware).
		c.Locals(LocalsUserID, apiKey.UserID)
		c.Locals(LocalsRole, string(apiKey.User.Role))
		c.Locals(LocalsAPIKey, apiKey)

		// Propagate through context for service/repository layers.
		ctx := c.UserContext()
		ctx = context.WithValue(ctx, CtxUserIDKey, apiKey.UserID)
		ctx = context.WithValue(ctx, CtxRoleKey, string(apiKey.User.Role))
		c.SetUserContext(ctx)

		return c.Next()
	}
}
