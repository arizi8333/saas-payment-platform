package dto

import (
	"time"
)

// CreateTransactionRequest represents the request body for creating a new transaction.
type CreateTransactionRequest struct {
	Amount        int64                  `json:"amount" validate:"required,gt=0"`
	Currency      string                 `json:"currency" validate:"required,len=3"`
	PaymentMethod string                 `json:"payment_method" validate:"required"`
	ExternalID    string                 `json:"external_id" validate:"required"`
	Description   string                 `json:"description"`
	CustomerEmail string                 `json:"customer_email" validate:"omitempty,email"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// TransactionResponse represents a transaction returned to clients.
type TransactionResponse struct {
	ID            string                 `json:"id"`
	ExternalID    string                 `json:"external_id"`
	Amount        int64                  `json:"amount"`
	Currency      string                 `json:"currency"`
	Status        string                 `json:"status"`
	PaymentMethod string                 `json:"payment_method"`
	Description   string                 `json:"description"`
	CustomerEmail string                 `json:"customer_email"`
	Metadata      map[string]interface{} `json:"metadata"`
	PaidAt        *time.Time             `json:"paid_at"`
	CreatedAt     time.Time              `json:"created_at"`
}

// TransactionListResponse represents a paginated list of transactions.
type TransactionListResponse struct {
	Transactions []TransactionResponse `json:"transactions"`
	Pagination   PaginationResponse    `json:"pagination"`
}
