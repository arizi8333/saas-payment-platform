package dto

import (
	"time"
)

// CreateAPIKeyRequest represents the request body for creating a new API key.
type CreateAPIKeyRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

// APIKeyResponse represents an API key returned to clients.
// NOTE: key_hash and full key are intentionally excluded for security.
type APIKeyResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	IsActive   bool       `json:"is_active"`
	RateLimit  int        `json:"rate_limit"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// APIKeyCreatedResponse is returned only once when a new API key is created.
// It extends APIKeyResponse with the full key that cannot be retrieved again.
type APIKeyCreatedResponse struct {
	APIKeyResponse
	FullKey string `json:"full_key"`
}

// APIKeyListResponse represents a paginated list of API keys.
type APIKeyListResponse struct {
	Keys       []APIKeyResponse   `json:"keys"`
	Pagination PaginationResponse `json:"pagination"`
}
