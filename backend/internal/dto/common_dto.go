package dto

// PaginationRequest represents pagination query parameters.
type PaginationRequest struct {
	Page     int `query:"page" validate:"min=1"`
	PageSize int `query:"page_size" validate:"min=1,max=100"`
}

// PaginationResponse represents pagination metadata in responses.
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// ErrorResponse represents a standardized error response body.
type ErrorResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// SuccessResponse is a generic wrapper for successful API responses.
type SuccessResponse[T any] struct {
	Success bool `json:"success"`
	Data    T    `json:"data,omitempty"`
}
