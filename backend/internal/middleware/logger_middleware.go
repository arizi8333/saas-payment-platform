package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/saas-payment-platform/backend/internal/pkg/logger"
)

// Logger returns a Fiber middleware that emits a structured JSON log line for
// every request. Sensitive headers (Authorization, X-API-Key) and request/
// response bodies are intentionally excluded.
func Logger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request.
		err := c.Next()

		latency := time.Since(start)

		// Extract request_id from context (set by RequestID middleware).
		reqID, _ := c.UserContext().Value(logger.RequestIDKey).(string)

		slog.Info("http_request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency_ms", latency.Milliseconds(),
			"request_id", reqID,
			"ip", c.IP(),
		)

		return err
	}
}
