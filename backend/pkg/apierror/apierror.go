package apierror

import "net/http"

// APIError represents a structured API error with HTTP status code.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Code       string `json:"code"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Message
}

// NewBadRequest creates a 400 Bad Request error.
func NewBadRequest(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusBadRequest,
		Message:    message,
		Code:       "BAD_REQUEST",
	}
}

// NewUnauthorized creates a 401 Unauthorized error.
func NewUnauthorized(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusUnauthorized,
		Message:    message,
		Code:       "UNAUTHORIZED",
	}
}

// NewForbidden creates a 403 Forbidden error.
func NewForbidden(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusForbidden,
		Message:    message,
		Code:       "FORBIDDEN",
	}
}

// NewNotFound creates a 404 Not Found error.
func NewNotFound(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusNotFound,
		Message:    message,
		Code:       "NOT_FOUND",
	}
}

// NewConflict creates a 409 Conflict error.
func NewConflict(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusConflict,
		Message:    message,
		Code:       "CONFLICT",
	}
}

// NewInternalError creates a 500 Internal Server Error.
func NewInternalError(message string) *APIError {
	return &APIError{
		StatusCode: http.StatusInternalServerError,
		Message:    message,
		Code:       "INTERNAL_ERROR",
	}
}
