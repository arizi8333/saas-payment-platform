package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// testProduct is a SQLite-compatible version of model.Product for testing.
type testProduct struct {
	ID          uuid.UUID      `gorm:"type:text;primary_key"`
	UserID      uuid.UUID      `gorm:"type:text;not null;index"`
	Name        string         `gorm:"type:varchar(255);not null"`
	Description string         `gorm:"type:text"`
	IsActive    bool           `gorm:"not null;default:true"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (testProduct) TableName() string { return "products" }

// testPlan is a SQLite-compatible version of model.Plan for testing.
type testPlan struct {
	ID              uuid.UUID             `gorm:"type:text;primary_key"`
	ProductID       uuid.UUID             `gorm:"type:text;not null;index"`
	Name            string                `gorm:"type:varchar(255);not null"`
	Amount          int64                 `gorm:"not null"`
	Currency        string                `gorm:"type:varchar(3);not null;default:'IDR'"`
	BillingInterval model.BillingInterval `gorm:"type:varchar(20);not null"`
	IsActive        bool                  `gorm:"not null;default:true"`
	CreatedAt       time.Time             `gorm:"not null"`
	UpdatedAt       time.Time             `gorm:"not null"`
	DeletedAt       gorm.DeletedAt        `gorm:"index"`
}

func (testPlan) TableName() string { return "plans" }

// testSubscription is a SQLite-compatible version of model.Subscription for testing.
type testSubscription struct {
	ID                 uuid.UUID                `gorm:"type:text;primary_key"`
	UserID             uuid.UUID                `gorm:"type:text;not null;index"`
	PlanID             uuid.UUID                `gorm:"type:text;not null;index"`
	Status             model.SubscriptionStatus `gorm:"type:varchar(30);not null;default:'pending_payment'"`
	CurrentPeriodStart time.Time                `gorm:"not null"`
	CurrentPeriodEnd   time.Time                `gorm:"not null"`
	CancelledAt        *time.Time
	CreatedAt          time.Time      `gorm:"not null"`
	UpdatedAt          time.Time      `gorm:"not null"`
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (testSubscription) TableName() string { return "subscriptions" }

func setupSubscriptionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&testUser{}, &testProduct{}, &testPlan{}, &testSubscription{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func createTestUserForSubscription(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	user := newTestUser()
	user.Email = fmt.Sprintf("sub_test_%s@example.com", uuid.New().String()[:8])
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func newTestProduct(userID uuid.UUID) *model.Product {
	return &model.Product{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        "Test Product",
		Description: "A test product",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func newTestPlan(productID uuid.UUID) *model.Plan {
	return &model.Plan{
		ID:              uuid.New(),
		ProductID:       productID,
		Name:            "Monthly Plan",
		Amount:          100000,
		Currency:        "IDR",
		BillingInterval: model.BillingMonthly,
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func newTestSubscription(userID, planID uuid.UUID) *model.Subscription {
	now := time.Now()
	return &model.Subscription{
		ID:                 uuid.New(),
		UserID:             userID,
		PlanID:             planID,
		Status:             model.SubStatusPendingPayment,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// --- Product Repository Tests ---

func TestProductCreate_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)
	user := createTestUserForSubscription(t, db)

	product := newTestProduct(user.ID)
	err := repo.Create(context.Background(), product)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Product
	db.First(&found, "id = ?", product.ID)
	if found.Name != product.Name {
		t.Errorf("expected name %s, got %s", product.Name, found.Name)
	}
	if found.IsActive != true {
		t.Error("expected IsActive to be true")
	}
}

func TestProductFindByID_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)
	user := createTestUserForSubscription(t, db)

	product := newTestProduct(user.ID)
	db.Create(product)

	found, err := repo.FindByID(context.Background(), product.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Name != product.Name {
		t.Errorf("expected name %s, got %s", product.Name, found.Name)
	}
}

func TestProductFindByID_NotFound(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestProductFindByUserID_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)
	user := createTestUserForSubscription(t, db)

	for i := 0; i < 3; i++ {
		p := newTestProduct(user.ID)
		p.Name = fmt.Sprintf("Product %d", i)
		db.Create(p)
	}

	products, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(products) != 3 {
		t.Errorf("expected 3 products, got %d", len(products))
	}
}

func TestProductFindByUserID_Pagination(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)
	user := createTestUserForSubscription(t, db)

	for i := 0; i < 5; i++ {
		p := newTestProduct(user.ID)
		p.Name = fmt.Sprintf("Product %d", i)
		db.Create(p)
	}

	products, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(products) != 2 {
		t.Errorf("expected 2 products on page 1, got %d", len(products))
	}

	products, total, err = repo.FindByUserID(context.Background(), user.ID, 3, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(products) != 1 {
		t.Errorf("expected 1 product on page 3, got %d", len(products))
	}
}

func TestProductUpdate_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)
	user := createTestUserForSubscription(t, db)

	product := newTestProduct(user.ID)
	db.Create(product)

	product.Name = "Updated Product"
	product.IsActive = false
	err := repo.Update(context.Background(), product)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Product
	db.First(&found, "id = ?", product.ID)
	if found.Name != "Updated Product" {
		t.Errorf("expected updated name, got %s", found.Name)
	}
	if found.IsActive != false {
		t.Error("expected IsActive to be false")
	}
}

func TestProductCreate_WithTransaction(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForSubscription(t, db)

	product := newTestProduct(user.ID)
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, product)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.Product{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 product, got %d", count)
	}
}

func TestProductCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewProductRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForSubscription(t, db)

	product := newTestProduct(user.ID)
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, product); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.Product{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 products after rollback, got %d", count)
	}
}

// --- Plan Repository Tests ---

func createTestProductForPlan(t *testing.T, db *gorm.DB, userID uuid.UUID) *model.Product {
	t.Helper()
	product := newTestProduct(userID)
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("failed to create test product: %v", err)
	}
	return product
}

func TestPlanCreate_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)
	user := createTestUserForSubscription(t, db)
	product := createTestProductForPlan(t, db, user.ID)

	plan := newTestPlan(product.ID)
	err := repo.Create(context.Background(), plan)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Plan
	db.First(&found, "id = ?", plan.ID)
	if found.Name != plan.Name {
		t.Errorf("expected name %s, got %s", plan.Name, found.Name)
	}
	if found.Amount != 100000 {
		t.Errorf("expected amount 100000, got %d", found.Amount)
	}
	if found.BillingInterval != model.BillingMonthly {
		t.Errorf("expected billing interval monthly, got %s", found.BillingInterval)
	}
}

func TestPlanFindByID_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)
	user := createTestUserForSubscription(t, db)
	product := createTestProductForPlan(t, db, user.ID)

	plan := newTestPlan(product.ID)
	db.Create(plan)

	found, err := repo.FindByID(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Name != plan.Name {
		t.Errorf("expected name %s, got %s", plan.Name, found.Name)
	}
}

func TestPlanFindByID_NotFound(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestPlanFindByProductID_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)
	user := createTestUserForSubscription(t, db)
	product := createTestProductForPlan(t, db, user.ID)

	for i := 0; i < 3; i++ {
		p := newTestPlan(product.ID)
		p.Name = fmt.Sprintf("Plan %d", i)
		db.Create(p)
	}

	plans, total, err := repo.FindByProductID(context.Background(), product.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(plans) != 3 {
		t.Errorf("expected 3 plans, got %d", len(plans))
	}
}

func TestPlanFindByProductID_Pagination(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)
	user := createTestUserForSubscription(t, db)
	product := createTestProductForPlan(t, db, user.ID)

	for i := 0; i < 5; i++ {
		p := newTestPlan(product.ID)
		p.Name = fmt.Sprintf("Plan %d", i)
		db.Create(p)
	}

	plans, total, err := repo.FindByProductID(context.Background(), product.ID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(plans) != 2 {
		t.Errorf("expected 2 plans on page 1, got %d", len(plans))
	}

	plans, total, err = repo.FindByProductID(context.Background(), product.ID, 3, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(plans) != 1 {
		t.Errorf("expected 1 plan on page 3, got %d", len(plans))
	}
}

func TestPlanUpdate_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)
	user := createTestUserForSubscription(t, db)
	product := createTestProductForPlan(t, db, user.ID)

	plan := newTestPlan(product.ID)
	db.Create(plan)

	plan.Name = "Updated Plan"
	plan.Amount = 200000
	err := repo.Update(context.Background(), plan)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Plan
	db.First(&found, "id = ?", plan.ID)
	if found.Name != "Updated Plan" {
		t.Errorf("expected updated name, got %s", found.Name)
	}
	if found.Amount != 200000 {
		t.Errorf("expected amount 200000, got %d", found.Amount)
	}
}

func TestPlanCreate_WithTransaction(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForSubscription(t, db)
	product := createTestProductForPlan(t, db, user.ID)

	plan := newTestPlan(product.ID)
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, plan)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.Plan{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 plan, got %d", count)
	}
}

func TestPlanCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewPlanRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForSubscription(t, db)
	product := createTestProductForPlan(t, db, user.ID)

	plan := newTestPlan(product.ID)
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, plan); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.Plan{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 plans after rollback, got %d", count)
	}
}

// --- Subscription Repository Tests ---

func createTestPlanForSubscription(t *testing.T, db *gorm.DB, userID uuid.UUID) *model.Plan {
	t.Helper()
	product := createTestProductForPlan(t, db, userID)
	plan := newTestPlan(product.ID)
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to create test plan: %v", err)
	}
	return plan
}

func TestSubscriptionCreate_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	sub := newTestSubscription(user.ID, plan.ID)
	err := repo.Create(context.Background(), sub)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Subscription
	db.First(&found, "id = ?", sub.ID)
	if found.Status != model.SubStatusPendingPayment {
		t.Errorf("expected status %s, got %s", model.SubStatusPendingPayment, found.Status)
	}
	if found.UserID != user.ID {
		t.Errorf("expected user_id %s, got %s", user.ID, found.UserID)
	}
	if found.PlanID != plan.ID {
		t.Errorf("expected plan_id %s, got %s", plan.ID, found.PlanID)
	}
}

func TestSubscriptionFindByID_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	sub := newTestSubscription(user.ID, plan.ID)
	db.Create(sub)

	found, err := repo.FindByID(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Status != model.SubStatusPendingPayment {
		t.Errorf("expected status %s, got %s", model.SubStatusPendingPayment, found.Status)
	}
}

func TestSubscriptionFindByID_NotFound(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestSubscriptionFindByUserID_Success(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	for i := 0; i < 3; i++ {
		sub := newTestSubscription(user.ID, plan.ID)
		db.Create(sub)
	}

	subs, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(subs) != 3 {
		t.Errorf("expected 3 subscriptions, got %d", len(subs))
	}
}

func TestSubscriptionFindByUserID_Pagination(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	for i := 0; i < 5; i++ {
		sub := newTestSubscription(user.ID, plan.ID)
		db.Create(sub)
	}

	subs, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions on page 1, got %d", len(subs))
	}

	subs, total, err = repo.FindByUserID(context.Background(), user.ID, 3, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription on page 3, got %d", len(subs))
	}
}

func TestSubscriptionUpdateStatus_ToActive(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	sub := newTestSubscription(user.ID, plan.ID)
	db.Create(sub)

	err := repo.UpdateStatus(context.Background(), sub.ID, model.SubStatusActive)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Subscription
	db.First(&found, "id = ?", sub.ID)
	if found.Status != model.SubStatusActive {
		t.Errorf("expected status %s, got %s", model.SubStatusActive, found.Status)
	}
	if found.CancelledAt != nil {
		t.Error("expected CancelledAt to be nil for active status")
	}
}

func TestSubscriptionUpdateStatus_ToCancelled(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	sub := newTestSubscription(user.ID, plan.ID)
	db.Create(sub)

	err := repo.UpdateStatus(context.Background(), sub.ID, model.SubStatusCancelled)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Subscription
	db.First(&found, "id = ?", sub.ID)
	if found.Status != model.SubStatusCancelled {
		t.Errorf("expected status %s, got %s", model.SubStatusCancelled, found.Status)
	}
	if found.CancelledAt == nil {
		t.Error("expected CancelledAt to be set for cancelled status")
	}
}

func TestSubscriptionUpdateStatus_NotFound(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)

	err := repo.UpdateStatus(context.Background(), uuid.New(), model.SubStatusActive)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestSubscriptionCreate_WithTransaction(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	sub := newTestSubscription(user.ID, plan.ID)
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, sub)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.Subscription{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 subscription, got %d", count)
	}
}

func TestSubscriptionCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForSubscription(t, db)
	plan := createTestPlanForSubscription(t, db, user.ID)

	sub := newTestSubscription(user.ID, plan.ID)
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, sub); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.Subscription{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 subscriptions after rollback, got %d", count)
	}
}
