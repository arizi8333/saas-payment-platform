package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock ProductRepository ---

type mockProductRepo struct {
	products map[uuid.UUID]*model.Product
}

func newMockProductRepo() *mockProductRepo {
	return &mockProductRepo{products: make(map[uuid.UUID]*model.Product)}
}

func (m *mockProductRepo) Create(_ context.Context, product *model.Product) error {
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()
	m.products[product.ID] = product
	return nil
}

func (m *mockProductRepo) FindByUserID(_ context.Context, userID uuid.UUID, page, pageSize int) ([]model.Product, int64, error) {
	var result []model.Product
	for _, p := range m.products {
		if p.UserID == userID {
			result = append(result, *p)
		}
	}
	total := int64(len(result))
	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return []model.Product{}, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockProductRepo) FindByID(_ context.Context, id uuid.UUID) (*model.Product, error) {
	if p, ok := m.products[id]; ok {
		return p, nil
	}
	return nil, apierror.NewNotFound("product not found")
}

func (m *mockProductRepo) Update(_ context.Context, product *model.Product) error {
	if _, ok := m.products[product.ID]; !ok {
		return apierror.NewNotFound("product not found")
	}
	product.UpdatedAt = time.Now()
	m.products[product.ID] = product
	return nil
}

// --- Mock PlanRepository ---

type mockPlanRepo struct {
	plans map[uuid.UUID]*model.Plan
}

func newMockPlanRepo() *mockPlanRepo {
	return &mockPlanRepo{plans: make(map[uuid.UUID]*model.Plan)}
}

func (m *mockPlanRepo) Create(_ context.Context, plan *model.Plan) error {
	if plan.ID == uuid.Nil {
		plan.ID = uuid.New()
	}
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()
	m.plans[plan.ID] = plan
	return nil
}

func (m *mockPlanRepo) FindByProductID(_ context.Context, productID uuid.UUID, page, pageSize int) ([]model.Plan, int64, error) {
	var result []model.Plan
	for _, p := range m.plans {
		if p.ProductID == productID {
			result = append(result, *p)
		}
	}
	total := int64(len(result))
	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return []model.Plan{}, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockPlanRepo) FindByID(_ context.Context, id uuid.UUID) (*model.Plan, error) {
	if p, ok := m.plans[id]; ok {
		return p, nil
	}
	return nil, apierror.NewNotFound("plan not found")
}

func (m *mockPlanRepo) Update(_ context.Context, plan *model.Plan) error {
	if _, ok := m.plans[plan.ID]; !ok {
		return apierror.NewNotFound("plan not found")
	}
	plan.UpdatedAt = time.Now()
	m.plans[plan.ID] = plan
	return nil
}

// --- Mock SubscriptionRepository ---

type mockSubRepo struct {
	subscriptions map[uuid.UUID]*model.Subscription
}

func newMockSubRepo() *mockSubRepo {
	return &mockSubRepo{subscriptions: make(map[uuid.UUID]*model.Subscription)}
}

func (m *mockSubRepo) Create(_ context.Context, sub *model.Subscription) error {
	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	m.subscriptions[sub.ID] = sub
	return nil
}

func (m *mockSubRepo) FindByUserID(_ context.Context, userID uuid.UUID, page, pageSize int) ([]model.Subscription, int64, error) {
	var result []model.Subscription
	for _, s := range m.subscriptions {
		if s.UserID == userID {
			result = append(result, *s)
		}
	}
	total := int64(len(result))
	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return []model.Subscription{}, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockSubRepo) FindByID(_ context.Context, id uuid.UUID) (*model.Subscription, error) {
	if s, ok := m.subscriptions[id]; ok {
		return s, nil
	}
	return nil, apierror.NewNotFound("subscription not found")
}

func (m *mockSubRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.SubscriptionStatus) error {
	sub, ok := m.subscriptions[id]
	if !ok {
		return apierror.NewNotFound("subscription not found")
	}
	sub.Status = status
	if status == model.SubStatusCancelled {
		now := time.Now()
		sub.CancelledAt = &now
	}
	return nil
}


// --- Mock TransactionService (for subscription) ---

type mockTxService struct {
	createCalled bool
	createErr    error
	lastReq      *dto.CreateTransactionRequest
}

func (m *mockTxService) CreateTransaction(_ context.Context, _ uuid.UUID, req *dto.CreateTransactionRequest) (*dto.TransactionResponse, error) {
	m.createCalled = true
	m.lastReq = req
	if m.createErr != nil {
		return nil, m.createErr
	}
	return &dto.TransactionResponse{
		ID:         uuid.New().String(),
		ExternalID: req.ExternalID,
		Amount:     req.Amount,
		Currency:   req.Currency,
		Status:     "pending",
	}, nil
}

func (m *mockTxService) SimulatePayment(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockTxService) GetTransaction(_ context.Context, _ uuid.UUID) (*dto.TransactionResponse, error) {
	return nil, nil
}

func (m *mockTxService) ListTransactions(_ context.Context, _ uuid.UUID, _, _ int) (*dto.TransactionListResponse, error) {
	return nil, nil
}

// --- Mock InvoiceService (for subscription) ---

type mockInvService struct {
	invoices               map[uuid.UUID]*model.Invoice
	createCalled           bool
	createErr              error
	updateBySubCalled      bool
	updateBySubErr         error
	lastSubscriptionIDUsed uuid.UUID
}

func newMockInvService() *mockInvService {
	return &mockInvService{invoices: make(map[uuid.UUID]*model.Invoice)}
}

func (m *mockInvService) CreateInvoice(_ context.Context, req *CreateInvoiceInput) (*dto.InvoiceResponse, error) {
	m.createCalled = true
	if m.createErr != nil {
		return nil, m.createErr
	}
	invID := uuid.New()
	inv := &model.Invoice{
		ID:             invID,
		UserID:         req.UserID,
		SubscriptionID: req.SubscriptionID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         model.InvoiceUnpaid,
	}
	m.invoices[invID] = inv
	return &dto.InvoiceResponse{
		ID:       invID.String(),
		Amount:   req.Amount,
		Currency: req.Currency,
		Status:   "unpaid",
	}, nil
}

func (m *mockInvService) UpdateStatus(_ context.Context, invoiceID uuid.UUID, newStatus model.InvoiceStatus) (*dto.InvoiceResponse, error) {
	inv, ok := m.invoices[invoiceID]
	if !ok {
		return nil, apierror.NewNotFound("invoice not found")
	}
	inv.Status = newStatus
	return &dto.InvoiceResponse{
		ID:     inv.ID.String(),
		Status: string(newStatus),
	}, nil
}

func (m *mockInvService) UpdateStatusBySubscriptionID(_ context.Context, subscriptionID uuid.UUID, newStatus model.InvoiceStatus) (*dto.InvoiceResponse, error) {
	m.updateBySubCalled = true
	m.lastSubscriptionIDUsed = subscriptionID
	if m.updateBySubErr != nil {
		return nil, m.updateBySubErr
	}
	for _, inv := range m.invoices {
		if inv.SubscriptionID != nil && *inv.SubscriptionID == subscriptionID {
			inv.Status = newStatus
			return &dto.InvoiceResponse{
				ID:     inv.ID.String(),
				Status: string(newStatus),
			}, nil
		}
	}
	return nil, apierror.NewNotFound("invoice not found for subscription")
}

func (m *mockInvService) GetInvoice(_ context.Context, _ uuid.UUID) (*dto.InvoiceResponse, error) {
	return nil, nil
}

func (m *mockInvService) ListInvoices(_ context.Context, _ uuid.UUID, _, _ int) (*dto.InvoiceListResponse, error) {
	return nil, nil
}

// --- Helpers ---

func newTestSubscriptionService() (SubscriptionService, *mockProductRepo, *mockPlanRepo, *mockSubRepo, *mockTxService, *mockInvService) {
	productRepo := newMockProductRepo()
	planRepo := newMockPlanRepo()
	subRepo := newMockSubRepo()
	txService := &mockTxService{}
	invService := newMockInvService()
	txm := &mockTxManager{}

	svc := NewSubscriptionService(productRepo, planRepo, subRepo, txService, invService, txm)
	return svc, productRepo, planRepo, subRepo, txService, invService
}

func seedProduct(repo *mockProductRepo, userID uuid.UUID) *model.Product {
	p := &model.Product{
		ID:       uuid.New(),
		UserID:   userID,
		Name:     "Test Product",
		IsActive: true,
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	repo.products[p.ID] = p
	return p
}

func seedPlan(repo *mockPlanRepo, productID uuid.UUID) *model.Plan {
	p := &model.Plan{
		ID:              uuid.New(),
		ProductID:       productID,
		Name:            "Monthly Plan",
		Amount:          100000,
		Currency:        "IDR",
		BillingInterval: model.BillingMonthly,
		IsActive:        true,
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	repo.plans[p.ID] = p
	return p
}


// --- Tests: CreateProduct ---

func TestCreateProduct_Success(t *testing.T) {
	svc, productRepo, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	resp, err := svc.CreateProduct(context.Background(), userID, &dto.CreateProductRequest{
		Name:        "Premium SaaS",
		Description: "A premium product",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Name != "Premium SaaS" {
		t.Errorf("expected name Premium SaaS, got %s", resp.Name)
	}
	if resp.Description != "A premium product" {
		t.Errorf("expected description, got %s", resp.Description)
	}
	if !resp.IsActive {
		t.Error("expected is_active true")
	}
	if len(productRepo.products) != 1 {
		t.Fatalf("expected 1 product in repo, got %d", len(productRepo.products))
	}
}

func TestCreateProduct_EmptyName(t *testing.T) {
	svc, _, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	_, err := svc.CreateProduct(context.Background(), userID, &dto.CreateProductRequest{
		Name: "",
	})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

// --- Tests: CreatePlan ---

func TestCreatePlan_Success(t *testing.T) {
	svc, productRepo, planRepo, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)

	resp, err := svc.CreatePlan(context.Background(), userID, product.ID, &dto.CreatePlanRequest{
		Name:            "Monthly Basic",
		Amount:          50000,
		Currency:        "IDR",
		BillingInterval: "monthly",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Name != "Monthly Basic" {
		t.Errorf("expected name Monthly Basic, got %s", resp.Name)
	}
	if resp.Amount != 50000 {
		t.Errorf("expected amount 50000, got %d", resp.Amount)
	}
	if resp.BillingInterval != "monthly" {
		t.Errorf("expected billing_interval monthly, got %s", resp.BillingInterval)
	}
	if !resp.IsActive {
		t.Error("expected is_active true")
	}
	if len(planRepo.plans) != 1 {
		t.Fatalf("expected 1 plan in repo, got %d", len(planRepo.plans))
	}
}

func TestCreatePlan_ProductNotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	_, err := svc.CreatePlan(context.Background(), userID, uuid.New(), &dto.CreatePlanRequest{
		Name:            "Plan",
		Amount:          50000,
		Currency:        "IDR",
		BillingInterval: "monthly",
	})
	if err == nil {
		t.Fatal("expected error for non-existent product, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestCreatePlan_ProductNotOwnedByUser(t *testing.T) {
	svc, productRepo, _, _, _, _ := newTestSubscriptionService()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	product := seedProduct(productRepo, ownerID)

	_, err := svc.CreatePlan(context.Background(), otherUserID, product.ID, &dto.CreatePlanRequest{
		Name:            "Plan",
		Amount:          50000,
		Currency:        "IDR",
		BillingInterval: "monthly",
	})
	if err == nil {
		t.Fatal("expected error for product not owned by user, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN code, got %s", apiErr.Code)
	}
}

func TestCreatePlan_InactiveProduct(t *testing.T) {
	svc, productRepo, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)
	product.IsActive = false

	_, err := svc.CreatePlan(context.Background(), userID, product.ID, &dto.CreatePlanRequest{
		Name:            "Plan",
		Amount:          50000,
		Currency:        "IDR",
		BillingInterval: "monthly",
	})
	if err == nil {
		t.Fatal("expected error for inactive product, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestCreatePlan_InvalidBillingInterval(t *testing.T) {
	svc, productRepo, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)

	_, err := svc.CreatePlan(context.Background(), userID, product.ID, &dto.CreatePlanRequest{
		Name:            "Plan",
		Amount:          50000,
		Currency:        "IDR",
		BillingInterval: "weekly",
	})
	if err == nil {
		t.Fatal("expected error for invalid billing interval, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestCreatePlan_InvalidAmount(t *testing.T) {
	svc, productRepo, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)

	_, err := svc.CreatePlan(context.Background(), userID, product.ID, &dto.CreatePlanRequest{
		Name:            "Plan",
		Amount:          0,
		Currency:        "IDR",
		BillingInterval: "monthly",
	})
	if err == nil {
		t.Fatal("expected error for zero amount, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}


// --- Tests: CreateSubscription ---

func TestCreateSubscription_Success(t *testing.T) {
	svc, productRepo, planRepo, subRepo, txService, invService := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)
	plan := seedPlan(planRepo, product.ID)

	resp, err := svc.CreateSubscription(context.Background(), userID, &dto.CreateSubscriptionRequest{
		PlanID: plan.ID.String(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Status != "pending_payment" {
		t.Errorf("expected status pending_payment, got %s", resp.Status)
	}
	if resp.PlanID != plan.ID.String() {
		t.Errorf("expected plan_id %s, got %s", plan.ID.String(), resp.PlanID)
	}

	// Verify period dates are set.
	if resp.CurrentPeriodStart.IsZero() {
		t.Error("expected current_period_start to be set")
	}
	expectedEnd := resp.CurrentPeriodStart.AddDate(0, 1, 0) // monthly
	if !resp.CurrentPeriodEnd.Equal(expectedEnd) {
		t.Errorf("expected current_period_end %v, got %v", expectedEnd, resp.CurrentPeriodEnd)
	}

	// Verify subscription stored.
	if len(subRepo.subscriptions) != 1 {
		t.Fatalf("expected 1 subscription in repo, got %d", len(subRepo.subscriptions))
	}

	// Verify transaction was created.
	if !txService.createCalled {
		t.Error("expected TransactionService.CreateTransaction to be called")
	}
	if txService.lastReq.Amount != plan.Amount {
		t.Errorf("expected transaction amount %d, got %d", plan.Amount, txService.lastReq.Amount)
	}

	// Verify invoice was created.
	if !invService.createCalled {
		t.Error("expected InvoiceService.CreateInvoice to be called")
	}
	if len(invService.invoices) != 1 {
		t.Fatalf("expected 1 invoice, got %d", len(invService.invoices))
	}
}

func TestCreateSubscription_YearlyPlan(t *testing.T) {
	svc, productRepo, planRepo, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)
	plan := seedPlan(planRepo, product.ID)
	plan.BillingInterval = model.BillingYearly

	resp, err := svc.CreateSubscription(context.Background(), userID, &dto.CreateSubscriptionRequest{
		PlanID: plan.ID.String(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedEnd := resp.CurrentPeriodStart.AddDate(1, 0, 0) // yearly
	if !resp.CurrentPeriodEnd.Equal(expectedEnd) {
		t.Errorf("expected current_period_end %v for yearly, got %v", expectedEnd, resp.CurrentPeriodEnd)
	}
}

func TestCreateSubscription_PlanNotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	_, err := svc.CreateSubscription(context.Background(), userID, &dto.CreateSubscriptionRequest{
		PlanID: uuid.New().String(),
	})
	if err == nil {
		t.Fatal("expected error for non-existent plan, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestCreateSubscription_InactivePlan(t *testing.T) {
	svc, productRepo, planRepo, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)
	plan := seedPlan(planRepo, product.ID)
	plan.IsActive = false

	_, err := svc.CreateSubscription(context.Background(), userID, &dto.CreateSubscriptionRequest{
		PlanID: plan.ID.String(),
	})
	if err == nil {
		t.Fatal("expected error for inactive plan, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestCreateSubscription_InvalidPlanID(t *testing.T) {
	svc, _, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	_, err := svc.CreateSubscription(context.Background(), userID, &dto.CreateSubscriptionRequest{
		PlanID: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid plan_id, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

// --- Tests: ActivateSubscription ---

func TestActivateSubscription_Success(t *testing.T) {
	svc, productRepo, planRepo, subRepo, _, invService := newTestSubscriptionService()
	userID := uuid.New()
	product := seedProduct(productRepo, userID)
	plan := seedPlan(planRepo, product.ID)

	// Create subscription first.
	resp, err := svc.CreateSubscription(context.Background(), userID, &dto.CreateSubscriptionRequest{
		PlanID: plan.ID.String(),
	})
	if err != nil {
		t.Fatalf("create subscription failed: %v", err)
	}

	subID, _ := uuid.Parse(resp.ID)

	// Activate.
	activated, err := svc.ActivateSubscription(context.Background(), subID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if activated.Status != "active" {
		t.Errorf("expected status active, got %s", activated.Status)
	}

	// Verify subscription in repo.
	stored := subRepo.subscriptions[subID]
	if stored.Status != model.SubStatusActive {
		t.Errorf("expected stored status active, got %s", stored.Status)
	}

	// Verify invoice was updated.
	if !invService.updateBySubCalled {
		t.Error("expected InvoiceService.UpdateStatusBySubscriptionID to be called")
	}
	if invService.lastSubscriptionIDUsed != subID {
		t.Errorf("expected subscription ID %s, got %s", subID, invService.lastSubscriptionIDUsed)
	}
}

func TestActivateSubscription_NotPendingPayment(t *testing.T) {
	svc, _, _, subRepo, _, _ := newTestSubscriptionService()
	subID := uuid.New()

	subRepo.subscriptions[subID] = &model.Subscription{
		ID:     subID,
		UserID: uuid.New(),
		PlanID: uuid.New(),
		Status: model.SubStatusActive,
	}

	_, err := svc.ActivateSubscription(context.Background(), subID)
	if err == nil {
		t.Fatal("expected error for non-pending subscription, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestActivateSubscription_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestSubscriptionService()

	_, err := svc.ActivateSubscription(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent subscription, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

// --- Tests: CancelSubscription ---

func TestCancelSubscription_Success(t *testing.T) {
	svc, _, _, subRepo, _, _ := newTestSubscriptionService()
	subID := uuid.New()

	subRepo.subscriptions[subID] = &model.Subscription{
		ID:                 subID,
		UserID:             uuid.New(),
		PlanID:             uuid.New(),
		Status:             model.SubStatusActive,
		CurrentPeriodStart: time.Now(),
		CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0),
	}

	resp, err := svc.CancelSubscription(context.Background(), subID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Status != "cancelled" {
		t.Errorf("expected status cancelled, got %s", resp.Status)
	}
	if resp.CancelledAt == nil {
		t.Error("expected cancelled_at to be set")
	}

	stored := subRepo.subscriptions[subID]
	if stored.Status != model.SubStatusCancelled {
		t.Errorf("expected stored status cancelled, got %s", stored.Status)
	}
	if stored.CancelledAt == nil {
		t.Error("expected stored cancelled_at to be set")
	}
}

func TestCancelSubscription_NotActive(t *testing.T) {
	svc, _, _, subRepo, _, _ := newTestSubscriptionService()
	subID := uuid.New()

	subRepo.subscriptions[subID] = &model.Subscription{
		ID:     subID,
		UserID: uuid.New(),
		PlanID: uuid.New(),
		Status: model.SubStatusPendingPayment,
	}

	_, err := svc.CancelSubscription(context.Background(), subID)
	if err == nil {
		t.Fatal("expected error for non-active subscription, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestCancelSubscription_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := newTestSubscriptionService()

	_, err := svc.CancelSubscription(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent subscription, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

// --- Tests: ListSubscriptions ---

func TestListSubscriptions_Success(t *testing.T) {
	svc, _, _, subRepo, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		subID := uuid.New()
		subRepo.subscriptions[subID] = &model.Subscription{
			ID:                 subID,
			UserID:             userID,
			PlanID:             uuid.New(),
			Status:             model.SubStatusActive,
			CurrentPeriodStart: time.Now(),
			CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0),
			CreatedAt:          time.Now(),
		}
	}

	resp, err := svc.ListSubscriptions(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Subscriptions) != 3 {
		t.Errorf("expected 3 subscriptions, got %d", len(resp.Subscriptions))
	}
	if resp.Pagination.TotalItems != 3 {
		t.Errorf("expected total_items 3, got %d", resp.Pagination.TotalItems)
	}
}

func TestListSubscriptions_Empty(t *testing.T) {
	svc, _, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	resp, err := svc.ListSubscriptions(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Subscriptions) != 0 {
		t.Errorf("expected 0 subscriptions, got %d", len(resp.Subscriptions))
	}
}

// --- Tests: ListProducts ---

func TestListProducts_Success(t *testing.T) {
	svc, productRepo, _, _, _, _ := newTestSubscriptionService()
	userID := uuid.New()

	for i := 0; i < 2; i++ {
		seedProduct(productRepo, userID)
	}

	resp, err := svc.ListProducts(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Products) != 2 {
		t.Errorf("expected 2 products, got %d", len(resp.Products))
	}
	if resp.Pagination.TotalItems != 2 {
		t.Errorf("expected total_items 2, got %d", resp.Pagination.TotalItems)
	}
}

// --- Tests: calculatePeriodEnd ---

func TestCalculatePeriodEnd_Monthly(t *testing.T) {
	start := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	end := calculatePeriodEnd(start, model.BillingMonthly)
	expected := time.Date(2025, 2, 15, 10, 0, 0, 0, time.UTC)
	if !end.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, end)
	}
}

func TestCalculatePeriodEnd_Yearly(t *testing.T) {
	start := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	end := calculatePeriodEnd(start, model.BillingYearly)
	expected := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !end.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, end)
	}
}
