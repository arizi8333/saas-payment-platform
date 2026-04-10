package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS returns a Fiber CORS middleware with sensible defaults for the platform.
// allowOrigins can be used to restrict origins; pass "*" (or empty) to allow all.
func CORS(allowOrigins string) fiber.Handler {
	if allowOrigins == "" {
		allowOrigins = "*"
	}

	return cors.New(cors.Config{
		AllowOrigins:  allowOrigins,
		AllowMethods:  "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:  "Origin,Content-Type,Accept,Authorization,X-API-Key,X-Request-ID",
		ExposeHeaders: "X-Request-ID,X-RateLimit-Limit,X-RateLimit-Remaining,X-RateLimit-Reset",
	})
}
