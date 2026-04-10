package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// UserRepository defines the interface for user database operations.
type UserRepository interface {
	// Create inserts a new user record into the database.
	Create(ctx context.Context, user *model.User) error
	// FindByEmail retrieves a user by email address. Returns not found error if absent.
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	// FindByID retrieves a user by UUID. Returns not found error if absent.
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	// Update persists changes to an existing user record.
	Update(ctx context.Context, user *model.User) error
}

// userRepository implements UserRepository using GORM.
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository backed by the given GORM DB.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create inserts a new user. It returns a conflict error if the email already exists.
func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apierror.NewConflict("email already registered")
		}
		return apierror.NewInternalError("failed to create user")
	}

	return nil
}

// FindByEmail looks up a user by their email address.
// GORM automatically filters soft-deleted records.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	db := database.GetDB(ctx, r.db)

	var user model.User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("user not found")
		}
		return nil, apierror.NewInternalError("failed to find user by email")
	}

	return &user, nil
}

// FindByID looks up a user by their UUID primary key.
// GORM automatically filters soft-deleted records.
func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	db := database.GetDB(ctx, r.db)

	var user model.User
	if err := db.Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.NewNotFound("user not found")
		}
		return nil, apierror.NewInternalError("failed to find user by id")
	}

	return &user, nil
}

// Update saves all fields of the given user record.
func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	db := database.GetDB(ctx, r.db)

	if err := db.Save(user).Error; err != nil {
		return apierror.NewInternalError("failed to update user")
	}

	return nil
}
