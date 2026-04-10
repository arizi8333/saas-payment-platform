package middleware

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/saas-payment-platform/backend/internal/pkg/logger"
)

const headerRequestID = "X-Request-ID"

// RequestID returns a Fiber middleware that ensures every request has a unique
// X-Request-ID. If the client sends one it is reused; otherwise a new UUID is
// generated. The ID is propagated via the response header, Fiber locals, and
// the request context using logger.RequestIDKey.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := c.Get(headerRequestID)
		if reqID == "" {
			reqID = uuid.New().String()
		}

		// Set response header.
		c.Set(headerRequestID, reqID)

		// Store in Fiber locals for easy access in handlers.
		c.Locals(string(logger.RequestIDKey), reqID)

		// Propagate through context so downstream services / logger can use it.
		ctx := context.WithValue(c.UserContext(), logger.RequestIDKey, reqID)
		c.SetUserContext(ctx)

		return c.Next()
	}
}
