package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// ProductRepository defines the interface for product database operations.
type ProductRepository interface {
	// Create inserts a new product record into the database.
	Create(ctx context.Context, product *model.Product) error
	// FindByUserID retrieves paginated products for a user. Returns items, total count, and error.
	FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Product, int64, error)
	// FindByID retrieves a product by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	// Update persists changes to an existing product record.
	Update(ctx context.Context, product *model.Product) error
}

// PlanRepository defines the interface for plan database operations.
type PlanRepository interface {
	// Create inserts a new plan record into the database.
	Create(ctx context.Context, plan *model.Plan) error
	// FindByProductID retrieves paginated plans for a product. Returns items, total count, and error.
	FindByProductID(ctx context.Context, productID uuid.UUID, page, pageSize int) ([]model.Plan, int64, error)
	// FindByID retrieves a plan by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.Plan, error)
	// Update persists changes to an existing plan record.
	Update(ctx context.Context, plan *model.Plan) error
}

// SubscriptionRepository defines the interface for subscription database operations.
type SubscriptionRepository interface {
	// Create inserts a new subscription record into the database.
	Create(ctx context.Context, subscription *model.Subscription) error
	// FindByUserID retrieves paginated subscriptions for a user. Returns items, total count, and error.
	FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Subscription, int64, error)
	// FindByID retrieves a subscription by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error)
	// UpdateStatus updates the status of a subscription. Sets cancelled_at if status is "cancelled".
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubscriptionStatus) error
}

// --- Product Repository Implementation ---

// productRepository implements ProductRepository using GORM.
type productRepository struct {
	db *gorm.DB
}

// NewProductRepository creates a new ProductRepository backed by the given GORM DB.
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

// Create inserts a new product.
func (r *productRepository) Create(ctx context.Context, product *model.Product) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(product).Error; err != nil {
		return apierror.NewInternalError("failed to create product")
	}

	return nil
}

// FindByUserID retrieves paginated products belonging to a user, ordered by created_at DESC.
func (r *productRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Product, int64, error) {
	db := database.GetDB(ctx, r.db)

	var total int64
	if err := db.Model(&model.Product{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to count products")
	}

	var products []model.Product
	offset := (page - 1) * pageSize
	if err := db.Where("user_id = ?", userID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&products).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to find products by user id")
	}

	return products, total, nil
}

// FindByID looks up a product by its UUID primary key.
func (r *productRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	db := database.GetDB(ctx, r.db)

	var product model.Product
	if err := db.Where("id = ?", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("product not found")
		}
		return nil, apierror.NewInternalError("failed to find product by id")
	}

	return &product, nil
}

// Update saves all fields of the given product record.
func (r *productRepository) Update(ctx context.Context, product *model.Product) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Save(product).Error; err != nil {
		return apierror.NewInternalError("failed to update product")
	}

	return nil
}

// --- Plan Repository Implementation ---

// planRepository implements PlanRepository using GORM.
type planRepository struct {
	db *gorm.DB
}

// NewPlanRepository creates a new PlanRepository backed by the given GORM DB.
func NewPlanRepository(db *gorm.DB) PlanRepository {
	return &planRepository{db: db}
}

// Create inserts a new plan.
func (r *planRepository) Create(ctx context.Context, plan *model.Plan) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(plan).Error; err != nil {
		return apierror.NewInternalError("failed to create plan")
	}

	return nil
}

// FindByProductID retrieves paginated plans belonging to a product, ordered by created_at DESC.
func (r *planRepository) FindByProductID(ctx context.Context, productID uuid.UUID, page, pageSize int) ([]model.Plan, int64, error) {
	db := database.GetDB(ctx, r.db)

	var total int64
	if err := db.Model(&model.Plan{}).Where("product_id = ?", productID).Count(&total).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to count plans")
	}

	var plans []model.Plan
	offset := (page - 1) * pageSize
	if err := db.Where("product_id = ?", productID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&plans).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to find plans by product id")
	}

	return plans, total, nil
}

// FindByID looks up a plan by its UUID primary key.
func (r *planRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Plan, error) {
	db := database.GetDB(ctx, r.db)

	var plan model.Plan
	if err := db.Where("id = ?", id).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("plan not found")
		}
		return nil, apierror.NewInternalError("failed to find plan by id")
	}

	return &plan, nil
}

// Update saves all fields of the given plan record.
func (r *planRepository) Update(ctx context.Context, plan *model.Plan) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Save(plan).Error; err != nil {
		return apierror.NewInternalError("failed to update plan")
	}

	return nil
}

// --- Subscription Repository Implementation ---

// subscriptionRepository implements SubscriptionRepository using GORM.
type subscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository creates a new SubscriptionRepository backed by the given GORM DB.
func NewSubscriptionRepository(db *gorm.DB) SubscriptionRepository {
	return &subscriptionRepository{db: db}
}

// Create inserts a new subscription.
func (r *subscriptionRepository) Create(ctx context.Context, subscription *model.Subscription) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(subscription).Error; err != nil {
		return apierror.NewInternalError("failed to create subscription")
	}

	return nil
}

// FindByUserID retrieves paginated subscriptions belonging to a user, ordered by created_at DESC.
func (r *subscriptionRepository) FindByUserID(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]model.Subscription, int64, error) {
	db := database.GetDB(ctx, r.db)

	var total int64
	if err := db.Model(&model.Subscription{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to count subscriptions")
	}

	var subscriptions []model.Subscription
	offset := (page - 1) * pageSize
	if err := db.Where("user_id = ?", userID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&subscriptions).Error; err != nil {
		return nil, 0, apierror.NewInternalError("failed to find subscriptions by user id")
	}

	return subscriptions, total, nil
}

// FindByID looks up a subscription by its UUID primary key.
func (r *subscriptionRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Subscription, error) {
	db := database.GetDB(ctx, r.db)

	var subscription model.Subscription
	if err := db.Where("id = ?", id).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("subscription not found")
		}
		return nil, apierror.NewInternalError("failed to find subscription by id")
	}

	return &subscription, nil
}

// UpdateStatus updates only the status field of a subscription.
// If the new status is "cancelled", cancelled_at is also set to the current time.
func (r *subscriptionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.SubscriptionStatus) error {
	db := database.GetDB(ctx, r.db)

	updates := map[string]interface{}{
		"status": status,
	}
	if status == model.SubStatusCancelled {
		now := time.Now()
		updates["cancelled_at"] = &now
	}

	result := db.Model(&model.Subscription{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return apierror.NewInternalError("failed to update subscription status")
	}
	if result.RowsAffected == 0 {
		return apierror.NewNotFound("subscription not found")
	}

	return nil
}
