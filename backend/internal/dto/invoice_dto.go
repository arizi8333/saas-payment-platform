package dto

import (
	"time"
)

// InvoiceResponse represents an invoice returned to clients.
type InvoiceResponse struct {
	ID             string     `json:"id"`
	InvoiceNumber  string     `json:"invoice_number"`
	Amount         int64      `json:"amount"`
	Currency       string     `json:"currency"`
	Status         string     `json:"status"`
	DueDate        time.Time  `json:"due_date"`
	PaidAt         *time.Time `json:"paid_at"`
	TransactionID  *string    `json:"transaction_id"`
	SubscriptionID *string    `json:"subscription_id"`
	CreatedAt      time.Time  `json:"created_at"`
}

// InvoiceListResponse represents a paginated list of invoices.
type InvoiceListResponse struct {
	Invoices   []InvoiceResponse  `json:"invoices"`
	Pagination PaginationResponse `json:"pagination"`
}
