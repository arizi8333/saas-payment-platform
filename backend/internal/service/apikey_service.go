package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

// APIKeyService defines the interface for API key business logic operations.
type APIKeyService interface {
	// CreateKey generates a new API key for the given user.
	CreateKey(ctx context.Context, userID uuid.UUID, req *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error)
	// ListKeys returns paginated API keys belonging to the user.
	ListKeys(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.APIKeyListResponse, error)
	// RevokeKey deactivates an API key owned by the user.
	RevokeKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID) error
	// ValidateKey checks a raw API key and returns the model if valid.
	ValidateKey(ctx context.Context, rawKey string) (*model.APIKey, error)
}

// defaultRateLimit is the default rate limit for new API keys (requests per hour).
const defaultRateLimit = 1000

// apiKeyService implements APIKeyService.
type apiKeyService struct {
	apiKeyRepo repository.APIKeyRepository
	txManager  database.TransactionManager
}

// NewAPIKeyService creates a new APIKeyService with all required dependencies.
func NewAPIKeyService(
	apiKeyRepo repository.APIKeyRepository,
	txManager database.TransactionManager,
) APIKeyService {
	return &apiKeyService{
		apiKeyRepo: apiKeyRepo,
		txManager:  txManager,
	}
}

// generateRawKey creates a unique API key in the format "sk_test_" + 32-char hex string.
// Uses crypto/rand for cryptographically secure random bytes.
func generateRawKey() (string, error) {
	bytes := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return "sk_test_" + hex.EncodeToString(bytes), nil
}

// hashKey computes the SHA-256 hash of a raw API key and returns it as a hex string.
func hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// CreateKey generates a new API key, hashes it, stores it, and returns the full key once.
func (s *apiKeyService) CreateKey(ctx context.Context, userID uuid.UUID, req *dto.CreateAPIKeyRequest) (*dto.APIKeyCreatedResponse, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, apierror.NewInternalError("failed to generate api key")
	}

	keyHash := hashKey(rawKey)
	keyPrefix := rawKey[:12] // "sk_test_" (8) + first 4 hex chars

	apiKey := &model.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      req.Name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		IsActive:  true,
		RateLimit: defaultRateLimit,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.apiKeyRepo.Create(txCtx, apiKey)
	})
	if err != nil {
		return nil, err
	}

	return &dto.APIKeyCreatedResponse{
		APIKeyResponse: toAPIKeyResponse(apiKey),
		FullKey:        rawKey,
	}, nil
}

// ListKeys returns paginated API keys for the given user (without full key or hash).
func (s *apiKeyService) ListKeys(ctx context.Context, userID uuid.UUID, page, pageSize int) (*dto.APIKeyListResponse, error) {
	keys, total, err := s.apiKeyRepo.FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}

	keyResponses := make([]dto.APIKeyResponse, len(keys))
	for i, k := range keys {
		keyResponses[i] = toAPIKeyResponse(&k)
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &dto.APIKeyListResponse{
		Keys: keyResponses,
		Pagination: dto.PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: int(total),
			TotalPages: totalPages,
		},
	}, nil
}

// RevokeKey deactivates an API key after verifying ownership.
func (s *apiKeyService) RevokeKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID) error {
	apiKey, err := s.apiKeyRepo.FindByID(ctx, keyID)
	if err != nil {
		return err
	}

	// Verify the key belongs to the requesting user.
	if apiKey.UserID != userID {
		return apierror.NewForbidden("you do not have permission to revoke this key")
	}

	apiKey.IsActive = false
	apiKey.UpdatedAt = time.Now()

	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		return s.apiKeyRepo.Update(txCtx, apiKey)
	})
}

// ValidateKey hashes the raw key, looks it up, and checks active/expiry status.
func (s *apiKeyService) ValidateKey(ctx context.Context, rawKey string) (*model.APIKey, error) {
	keyHash := hashKey(rawKey)

	apiKey, err := s.apiKeyRepo.FindByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, apierror.NewUnauthorized("invalid api key")
	}

	if !apiKey.IsActive {
		return nil, apierror.NewUnauthorized("api key is inactive")
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, apierror.NewUnauthorized("api key has expired")
	}

	// Update last_used_at (best-effort, don't fail validation on this).
	_ = s.apiKeyRepo.UpdateLastUsed(ctx, apiKey.ID)

	return apiKey, nil
}

// toAPIKeyResponse maps an APIKey model to an APIKeyResponse DTO.
func toAPIKeyResponse(k *model.APIKey) dto.APIKeyResponse {
	return dto.APIKeyResponse{
		ID:         k.ID.String(),
		Name:       k.Name,
		KeyPrefix:  k.KeyPrefix,
		IsActive:   k.IsActive,
		RateLimit:  k.RateLimit,
		LastUsedAt: k.LastUsedAt,
		ExpiresAt:  k.ExpiresAt,
		CreatedAt:  k.CreatedAt,
	}
}
