package dto

import (
	"time"
)

// CreateWebhookRequest represents the request body for registering a webhook endpoint.
type CreateWebhookRequest struct {
	URL    string   `json:"url" validate:"required,url"`
	Events []string `json:"events" validate:"required,min=1"`
}

// WebhookResponse represents a webhook endpoint returned to clients.
type WebhookResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"` // Only returned once on creation.
	Events    []string  `json:"events"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// WebhookDeliveryResponse represents a webhook delivery record returned to clients.
type WebhookDeliveryResponse struct {
	ID           string     `json:"id"`
	EventType    string     `json:"event_type"`
	Status       string     `json:"status"`
	ResponseCode *int       `json:"response_code"`
	RetryCount   int        `json:"retry_count"`
	DeliveredAt  *time.Time `json:"delivered_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// WebhookListResponse represents a paginated list of webhook endpoints.
type WebhookListResponse struct {
	Endpoints  []WebhookResponse  `json:"endpoints"`
	Pagination PaginationResponse `json:"pagination"`
}

// WebhookDeliveryListResponse represents a paginated list of webhook deliveries.
type WebhookDeliveryListResponse struct {
	Deliveries []WebhookDeliveryResponse `json:"deliveries"`
	Pagination PaginationResponse        `json:"pagination"`
}
