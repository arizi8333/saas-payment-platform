package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/repository"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// SubscriptionService defines the interface for subscription business logic operations.
type SubscriptionService interface {
	// CreateProduct creates a new product with name, description, and active status.
	CreateProduct(ctx context.Context, userID uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error)
	// ListProducts returns paginated products for a user.
	ListProducts(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.ProductListResponse, error)
	// CreatePlan creates a new plan for a product with amount, currency, and billing interval.
	CreatePlan(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req *dto.CreatePlanRequest) (*dto.PlanResponse, error)
	// CreateSubscription creates a subscription with status "pending_payment",
	// a related transaction via TransactionService, and an invoice via InvoiceService.
	CreateSubscription(ctx context.Context, userID uuid.UUID, req *dto.CreateSubscriptionRequest) (*dto.SubscriptionResponse, error)
	// ActivateSubscription updates subscription to "active" and invoice to "paid".
	ActivateSubscription(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error)
	// CancelSubscription updates subscription to "cancelled" and records cancelled_at.
	CancelSubscription(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error)
	// ListSubscriptions returns paginated subscriptions for a user.
	ListSubscriptions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.SubscriptionListResponse, error)
}

// subscriptionService implements SubscriptionService.
type subscriptionService struct {
	productRepo repository.ProductRepository
	planRepo    repository.PlanRepository
	subRepo     repository.SubscriptionRepository
	txService   TransactionService
	invService  InvoiceService
	txManager   database.TransactionManager
}


// NewSubscriptionService creates a new SubscriptionService with all required dependencies.
func NewSubscriptionService(
	productRepo repository.ProductRepository,
	planRepo repository.PlanRepository,
	subRepo repository.SubscriptionRepository,
	txService TransactionService,
	invService InvoiceService,
	txManager database.TransactionManager,
) SubscriptionService {
	return &subscriptionService{
		productRepo: productRepo,
		planRepo:    planRepo,
		subRepo:     subRepo,
		txService:   txService,
		invService:  invService,
		txManager:   txManager,
	}
}

// CreateProduct creates a new product with name, description, and active status.
func (s *subscriptionService) CreateProduct(ctx context.Context, userID uuid.UUID, req *dto.CreateProductRequest) (*dto.ProductResponse, error) {
	if req.Name == "" {
		return nil, apierror.NewBadRequest("product name is required")
	}

	product := &model.Product{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
	}

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.productRepo.Create(txCtx, product)
	})
	if err != nil {
		return nil, err
	}

	return toProductResponse(product), nil
}

// ListProducts returns paginated products for a user.
func (s *subscriptionService) ListProducts(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.ProductListResponse, error) {
	products, total, err := s.productRepo.FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	productResponses := make([]dto.ProductResponse, len(products))
	for i, p := range products {
		productResponses[i] = *toProductResponse(&p)
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &dto.ProductListResponse{
		Products: productResponses,
		Pagination: dto.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int(total),
			TotalPages: totalPages,
		},
	}, nil
}

// CreatePlan creates a new plan for a product with amount, currency, and billing interval.
func (s *subscriptionService) CreatePlan(ctx context.Context, userID uuid.UUID, productID uuid.UUID, req *dto.CreatePlanRequest) (*dto.PlanResponse, error) {
	// Validate product exists and belongs to user.
	product, err := s.productRepo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	if product.UserID != userID {
		return nil, apierror.NewForbidden("product does not belong to user")
	}
	if !product.IsActive {
		return nil, apierror.NewBadRequest("product is not active")
	}

	// Validate billing interval.
	interval := model.BillingInterval(req.BillingInterval)
	if interval != model.BillingMonthly && interval != model.BillingYearly {
		return nil, apierror.NewBadRequest("billing interval must be monthly or yearly")
	}

	if req.Amount <= 0 {
		return nil, apierror.NewBadRequest("plan amount must be greater than 0")
	}

	plan := &model.Plan{
		ProductID:       productID,
		Name:            req.Name,
		Amount:          req.Amount,
		Currency:        req.Currency,
		BillingInterval: interval,
		IsActive:        true,
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.planRepo.Create(txCtx, plan)
	})
	if err != nil {
		return nil, err
	}

	return toPlanResponse(plan), nil
}


// CreateSubscription creates a subscription with status "pending_payment",
// creates a related transaction via TransactionService, and an invoice via InvoiceService.
// All operations run within a single database transaction.
func (s *subscriptionService) CreateSubscription(ctx context.Context, userID uuid.UUID, req *dto.CreateSubscriptionRequest) (*dto.SubscriptionResponse, error) {
	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		return nil, apierror.NewBadRequest("invalid plan_id format")
	}

	// Validate plan exists and is active.
	plan, err := s.planRepo.FindByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if !plan.IsActive {
		return nil, apierror.NewBadRequest("plan is not active")
	}

	now := time.Now()
	periodEnd := calculatePeriodEnd(now, plan.BillingInterval)

	subscription := &model.Subscription{
		UserID:             userID,
		PlanID:             planID,
		Status:             model.SubStatusPendingPayment,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// 1. Create subscription record.
		if err := s.subRepo.Create(txCtx, subscription); err != nil {
			return err
		}

		// 2. Create transaction via TransactionService.
		txReq := &dto.CreateTransactionRequest{
			Amount:        plan.Amount,
			Currency:      plan.Currency,
			PaymentMethod: "bank_transfer",
			ExternalID:    fmt.Sprintf("sub-%s", subscription.ID.String()),
			Description:   fmt.Sprintf("Subscription payment for plan %s", plan.Name),
		}
		if _, err := s.txService.CreateTransaction(txCtx, userID, txReq); err != nil {
			return err
		}

		// 3. Create invoice via InvoiceService.
		subID := subscription.ID
		invInput := &CreateInvoiceInput{
			UserID:         userID,
			SubscriptionID: &subID,
			Amount:         plan.Amount,
			Currency:       plan.Currency,
			DueDate:        now.Add(7 * 24 * time.Hour),
		}
		if _, err := s.invService.CreateInvoice(txCtx, invInput); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return toSubscriptionResponse(subscription), nil
}

// ActivateSubscription updates subscription to "active" and invoice to "paid".
// Called after payment succeeds.
func (s *subscriptionService) ActivateSubscription(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error) {
	sub, err := s.subRepo.FindByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	if sub.Status != model.SubStatusPendingPayment {
		return nil, apierror.NewBadRequest("only pending_payment subscriptions can be activated")
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Update subscription status to active.
		if err := s.subRepo.UpdateStatus(txCtx, subscriptionID, model.SubStatusActive); err != nil {
			return err
		}

		// Update related invoice to paid.
		if _, err := s.invService.UpdateStatusBySubscriptionID(txCtx, subscriptionID, model.InvoicePaid); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Re-fetch to get updated record.
	updated, err := s.subRepo.FindByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	return toSubscriptionResponse(updated), nil
}

// CancelSubscription updates subscription to "cancelled" and records cancelled_at.
// Only active subscriptions can be cancelled.
func (s *subscriptionService) CancelSubscription(ctx context.Context, subscriptionID uuid.UUID) (*dto.SubscriptionResponse, error) {
	sub, err := s.subRepo.FindByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	if sub.Status != model.SubStatusActive {
		return nil, apierror.NewBadRequest("only active subscriptions can be cancelled")
	}

	if err := s.subRepo.UpdateStatus(ctx, subscriptionID, model.SubStatusCancelled); err != nil {
		return nil, err
	}

	// Re-fetch to get updated record with cancelled_at.
	updated, err := s.subRepo.FindByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	return toSubscriptionResponse(updated), nil
}

// ListSubscriptions returns paginated subscriptions for a user.
func (s *subscriptionService) ListSubscriptions(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.SubscriptionListResponse, error) {
	subscriptions, total, err := s.subRepo.FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	subResponses := make([]dto.SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		subResponses[i] = *toSubscriptionResponse(&sub)
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &dto.SubscriptionListResponse{
		Subscriptions: subResponses,
		Pagination: dto.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int(total),
			TotalPages: totalPages,
		},
	}, nil
}

// calculatePeriodEnd calculates the end of the billing period based on interval.
func calculatePeriodEnd(start time.Time, interval model.BillingInterval) time.Time {
	switch interval {
	case model.BillingYearly:
		return start.AddDate(1, 0, 0)
	default: // monthly
		return start.AddDate(0, 1, 0)
	}
}

// toProductResponse maps a Product model to a ProductResponse DTO.
func toProductResponse(p *model.Product) *dto.ProductResponse {
	return &dto.ProductResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
	}
}

// toPlanResponse maps a Plan model to a PlanResponse DTO.
func toPlanResponse(p *model.Plan) *dto.PlanResponse {
	return &dto.PlanResponse{
		ID:              p.ID.String(),
		ProductID:       p.ProductID.String(),
		Name:            p.Name,
		Amount:          p.Amount,
		Currency:        p.Currency,
		BillingInterval: string(p.BillingInterval),
		IsActive:        p.IsActive,
		CreatedAt:       p.CreatedAt,
	}
}

// toSubscriptionResponse maps a Subscription model to a SubscriptionResponse DTO.
func toSubscriptionResponse(sub *model.Subscription) *dto.SubscriptionResponse {
	return &dto.SubscriptionResponse{
		ID:                 sub.ID.String(),
		PlanID:             sub.PlanID.String(),
		Status:             string(sub.Status),
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   sub.CurrentPeriodEnd,
		CancelledAt:        sub.CancelledAt,
		CreatedAt:          sub.CreatedAt,
	}
}
