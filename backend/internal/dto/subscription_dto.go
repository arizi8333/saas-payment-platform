package dto

import (
	"time"
)

// CreateProductRequest represents the request body for creating a new product.
type CreateProductRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

// ProductResponse represents a product returned to clients.
type ProductResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreatePlanRequest represents the request body for creating a new plan.
type CreatePlanRequest struct {
	Name            string `json:"name" validate:"required"`
	Amount          int64  `json:"amount" validate:"required,gt=0"`
	Currency        string `json:"currency" validate:"required,len=3"`
	BillingInterval string `json:"billing_interval" validate:"required,oneof=monthly yearly"`
}

// PlanResponse represents a plan returned to clients.
type PlanResponse struct {
	ID              string    `json:"id"`
	ProductID       string    `json:"product_id"`
	Name            string    `json:"name"`
	Amount          int64     `json:"amount"`
	Currency        string    `json:"currency"`
	BillingInterval string    `json:"billing_interval"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

// CreateSubscriptionRequest represents the request body for creating a new subscription.
type CreateSubscriptionRequest struct {
	PlanID string `json:"plan_id" validate:"required,uuid"`
}

// SubscriptionResponse represents a subscription returned to clients.
type SubscriptionResponse struct {
	ID                 string     `json:"id"`
	PlanID             string     `json:"plan_id"`
	Status             string     `json:"status"`
	CurrentPeriodStart time.Time  `json:"current_period_start"`
	CurrentPeriodEnd   time.Time  `json:"current_period_end"`
	CancelledAt        *time.Time `json:"cancelled_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ProductListResponse represents a paginated list of products.
type ProductListResponse struct {
	Products   []ProductResponse  `json:"products"`
	Pagination PaginationResponse `json:"pagination"`
}

// PlanListResponse represents a paginated list of plans.
type PlanListResponse struct {
	Plans      []PlanResponse     `json:"plans"`
	Pagination PaginationResponse `json:"pagination"`
}

// SubscriptionListResponse represents a paginated list of subscriptions.
type SubscriptionListResponse struct {
	Subscriptions []SubscriptionResponse `json:"subscriptions"`
	Pagination    PaginationResponse     `json:"pagination"`
}
