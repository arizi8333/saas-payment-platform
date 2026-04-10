package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/auth"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/pkg/hash"
	"github.com/saas-payment-platform/backend/internal/repository"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// UserService defines the interface for user business logic operations.
type UserService interface {
	// Register creates a new user account with the given registration data.
	Register(ctx context.Context, req *dto.RegisterRequest) (*dto.UserProfileResponse, error)
	// Login authenticates a user and returns a JWT token.
	Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error)
	// GetProfile retrieves user profile data by user ID (without password_hash).
	GetProfile(ctx context.Context, userID uuid.UUID) (*dto.UserProfileResponse, error)
}

// userService implements UserService.
type userService struct {
	userRepo           repository.UserRepository
	txManager          database.TransactionManager
	jwtSecret          string
	jwtExpirationHours int
}

// NewUserService creates a new UserService with all required dependencies.
func NewUserService(
	userRepo repository.UserRepository,
	txManager database.TransactionManager,
	jwtSecret string,
	jwtExpirationHours int,
) UserService {
	return &userService{
		userRepo:           userRepo,
		txManager:          txManager,
		jwtSecret:          jwtSecret,
		jwtExpirationHours: jwtExpirationHours,
	}
}

// Register creates a new user with the given registration data.
// It validates password length, checks email uniqueness, hashes the password,
// and assigns the default "developer" role.
func (s *userService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.UserProfileResponse, error) {
	// Validate password minimum length.
	if len(req.Password) < 8 {
		return nil, apierror.NewBadRequest("password must be at least 8 characters")
	}

	// Check if email is already registered.
	existing, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		var apiErr *apierror.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "NOT_FOUND" {
			return nil, err
		}
	}
	if existing != nil {
		return nil, apierror.NewConflict("email already registered")
	}

	// Hash the password using bcrypt.
	passwordHash, err := hash.HashPassword(req.Password)
	if err != nil {
		return nil, apierror.NewInternalError("failed to hash password")
	}

	user := &model.User{
		Email:        req.Email,
		PasswordHash: passwordHash,
		FullName:     req.FullName,
		Role:         model.RoleDeveloper,
		IsActive:     true,
	}

	// Use transaction manager for the write operation.
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.userRepo.Create(txCtx, user)
	})
	if err != nil {
		return nil, err
	}

	return toUserProfileResponse(user), nil
}

// Login authenticates a user by email and password, then returns a JWT token.
func (s *userService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// Find user by email.
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		var apiErr *apierror.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "NOT_FOUND" {
			return nil, apierror.NewUnauthorized("invalid email or password")
		}
		return nil, err
	}

	// Verify password.
	if err := hash.CheckPassword(req.Password, user.PasswordHash); err != nil {
		return nil, apierror.NewUnauthorized("invalid email or password")
	}

	// Generate JWT token with user_id and role.
	token, err := auth.GenerateToken(user.ID, string(user.Role), s.jwtSecret, s.jwtExpirationHours)
	if err != nil {
		return nil, apierror.NewInternalError("failed to generate token")
	}

	return &dto.LoginResponse{
		Token:     token,
		ExpiresIn: s.jwtExpirationHours * 3600,
	}, nil
}

// GetProfile retrieves user profile data by ID without exposing password_hash.
func (s *userService) GetProfile(ctx context.Context, userID uuid.UUID) (*dto.UserProfileResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return toUserProfileResponse(user), nil
}

// toUserProfileResponse maps a User model to a UserProfileResponse DTO.
func toUserProfileResponse(user *model.User) *dto.UserProfileResponse {
	return &dto.UserProfileResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		FullName:  user.FullName,
		Role:      string(user.Role),
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
	}
}
