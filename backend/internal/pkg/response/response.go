package response

import (
	"github.com/gofiber/fiber/v2"
)

// APIResponse is the standardized API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents the error portion of an API response.
type APIError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// SuccessResponse sends a standardized success JSON response.
func SuccessResponse(c *fiber.Ctx, statusCode int, data interface{}) error {
	return c.Status(statusCode).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// ErrorResponse sends a standardized error JSON response.
func ErrorResponse(c *fiber.Ctx, statusCode int, message string) error {
	return c.Status(statusCode).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Message: message,
			Code:    statusCode,
		},
	})
}
