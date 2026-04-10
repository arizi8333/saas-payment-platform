package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock APIKeyRepository ---

type mockAPIKeyRepo struct {
	keys map[uuid.UUID]*model.APIKey // keyed by ID
}

func newMockAPIKeyRepo() *mockAPIKeyRepo {
	return &mockAPIKeyRepo{keys: make(map[uuid.UUID]*model.APIKey)}
}

func (m *mockAPIKeyRepo) Create(_ context.Context, apiKey *model.APIKey) error {
	for _, k := range m.keys {
		if k.KeyHash == apiKey.KeyHash {
			return apierror.NewConflict("api key already exists")
		}
	}
	m.keys[apiKey.ID] = apiKey
	return nil
}

func (m *mockAPIKeyRepo) FindByKeyHash(_ context.Context, keyHash string) (*model.APIKey, error) {
	for _, k := range m.keys {
		if k.KeyHash == keyHash {
			return k, nil
		}
	}
	return nil, apierror.NewNotFound("api key not found")
}

func (m *mockAPIKeyRepo) FindByUserID(_ context.Context, userID uuid.UUID, page, pageSize int) ([]model.APIKey, int64, error) {
	var result []model.APIKey
	for _, k := range m.keys {
		if k.UserID == userID {
			result = append(result, *k)
		}
	}
	total := int64(len(result))

	// Apply pagination.
	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return []model.APIKey{}, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockAPIKeyRepo) FindByID(_ context.Context, id uuid.UUID) (*model.APIKey, error) {
	if k, ok := m.keys[id]; ok {
		return k, nil
	}
	return nil, apierror.NewNotFound("api key not found")
}

func (m *mockAPIKeyRepo) Update(_ context.Context, apiKey *model.APIKey) error {
	if _, ok := m.keys[apiKey.ID]; !ok {
		return apierror.NewNotFound("api key not found")
	}
	m.keys[apiKey.ID] = apiKey
	return nil
}

func (m *mockAPIKeyRepo) UpdateLastUsed(_ context.Context, id uuid.UUID) error {
	k, ok := m.keys[id]
	if !ok {
		return apierror.NewNotFound("api key not found")
	}
	now := time.Now()
	k.LastUsedAt = &now
	return nil
}

// --- Helper ---

func newTestAPIKeyService() (APIKeyService, *mockAPIKeyRepo) {
	repo := newMockAPIKeyRepo()
	txm := &mockTxManager{}
	svc := NewAPIKeyService(repo, txm)
	return svc, repo
}

// --- Tests ---

func TestCreateKey_Success(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	userID := uuid.New()

	req := &dto.CreateAPIKeyRequest{Name: "My Test Key"}
	resp, err := svc.CreateKey(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Full key should start with the expected prefix.
	if !strings.HasPrefix(resp.FullKey, "sk_test_") {
		t.Errorf("expected key to start with prefix, got %s", resp.FullKey)
	}

	// Full key should be prefix (8 chars) + 32 hex chars = 40 chars total.
	if len(resp.FullKey) != 40 {
		t.Errorf("expected key length 40, got %d", len(resp.FullKey))
	}

	// Key prefix should be first 12 chars of the full key.
	if resp.KeyPrefix != resp.FullKey[:12] {
		t.Errorf("expected key_prefix %s, got %s", resp.FullKey[:12], resp.KeyPrefix)
	}

	if resp.Name != "My Test Key" {
		t.Errorf("expected name 'My Test Key', got %s", resp.Name)
	}
	if !resp.IsActive {
		t.Error("expected is_active true")
	}
	if resp.RateLimit != 1000 {
		t.Errorf("expected rate_limit 1000, got %d", resp.RateLimit)
	}

	// Verify key was stored with SHA-256 hash.
	if len(repo.keys) != 1 {
		t.Fatalf("expected 1 key in repo, got %d", len(repo.keys))
	}
	for _, stored := range repo.keys {
		h := sha256.Sum256([]byte(resp.FullKey))
		expectedHash := hex.EncodeToString(h[:])
		if stored.KeyHash != expectedHash {
			t.Errorf("stored hash doesn't match SHA-256 of full key")
		}
	}
}

func TestCreateKey_DefaultRateLimit(t *testing.T) {
	svc, _ := newTestAPIKeyService()
	userID := uuid.New()

	req := &dto.CreateAPIKeyRequest{Name: "Rate Limit Key"}
	resp, err := svc.CreateKey(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.RateLimit != 1000 {
		t.Errorf("expected default rate_limit 1000, got %d", resp.RateLimit)
	}
}

func TestListKeys_Success(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	userID := uuid.New()

	// Insert 3 keys directly into the mock repo.
	for i := 0; i < 3; i++ {
		key := &model.APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "Key",
			KeyHash:   uuid.New().String(),
			KeyPrefix: "sk_test_test",
			IsActive:  true,
			RateLimit: 1000,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repo.keys[key.ID] = key
	}

	resp, err := svc.ListKeys(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(resp.Keys))
	}
	if resp.Pagination.TotalItems != 3 {
		t.Errorf("expected total_items 3, got %d", resp.Pagination.TotalItems)
	}
	if resp.Pagination.TotalPages != 1 {
		t.Errorf("expected total_pages 1, got %d", resp.Pagination.TotalPages)
	}
}

func TestListKeys_Pagination(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		key := &model.APIKey{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "Key",
			KeyHash:   uuid.New().String(),
			KeyPrefix: "sk_test_test",
			IsActive:  true,
			RateLimit: 1000,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		repo.keys[key.ID] = key
	}

	resp, err := svc.ListKeys(context.Background(), userID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Keys) != 2 {
		t.Errorf("expected 2 keys on page 1, got %d", len(resp.Keys))
	}
	if resp.Pagination.TotalItems != 5 {
		t.Errorf("expected total_items 5, got %d", resp.Pagination.TotalItems)
	}
	if resp.Pagination.TotalPages != 3 {
		t.Errorf("expected total_pages 3, got %d", resp.Pagination.TotalPages)
	}
}

func TestRevokeKey_Success(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	userID := uuid.New()
	keyID := uuid.New()

	repo.keys[keyID] = &model.APIKey{
		ID:        keyID,
		UserID:    userID,
		Name:      "Revoke Me",
		KeyHash:   "somehash",
		KeyPrefix: "sk_test_test",
		IsActive:  true,
		RateLimit: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := svc.RevokeKey(context.Background(), userID, keyID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.keys[keyID].IsActive {
		t.Error("expected is_active to be false after revoke")
	}
}

func TestRevokeKey_Forbidden(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	keyID := uuid.New()

	repo.keys[keyID] = &model.APIKey{
		ID:        keyID,
		UserID:    ownerID,
		Name:      "Not Yours",
		KeyHash:   "somehash",
		KeyPrefix: "sk_test_test",
		IsActive:  true,
		RateLimit: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := svc.RevokeKey(context.Background(), otherUserID, keyID)
	if err == nil {
		t.Fatal("expected error for forbidden revoke, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN code, got %s", apiErr.Code)
	}
}

func TestRevokeKey_NotFound(t *testing.T) {
	svc, _ := newTestAPIKeyService()

	err := svc.RevokeKey(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestValidateKey_Success(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	userID := uuid.New()
	keyID := uuid.New()

	rawKey := "sk_test_abcdef1234567890abcdef1234567890"
	h := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(h[:])

	repo.keys[keyID] = &model.APIKey{
		ID:        keyID,
		UserID:    userID,
		Name:      "Valid Key",
		KeyHash:   keyHash,
		KeyPrefix: rawKey[:12],
		IsActive:  true,
		RateLimit: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result, err := svc.ValidateKey(context.Background(), rawKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ID != keyID {
		t.Errorf("expected key ID %s, got %s", keyID, result.ID)
	}
	if result.LastUsedAt == nil {
		t.Error("expected last_used_at to be updated")
	}
}

func TestValidateKey_InvalidKey(t *testing.T) {
	svc, _ := newTestAPIKeyService()

	_, err := svc.ValidateKey(context.Background(), "sk_test_nonexistent000000000000000000")
	if err == nil {
		t.Fatal("expected error for invalid key, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED code, got %s", apiErr.Code)
	}
}

func TestValidateKey_InactiveKey(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	keyID := uuid.New()

	rawKey := "sk_test_inactive0000000000000000000000"
	h := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(h[:])

	repo.keys[keyID] = &model.APIKey{
		ID:        keyID,
		UserID:    uuid.New(),
		Name:      "Inactive Key",
		KeyHash:   keyHash,
		KeyPrefix: rawKey[:12],
		IsActive:  false,
		RateLimit: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := svc.ValidateKey(context.Background(), rawKey)
	if err == nil {
		t.Fatal("expected error for inactive key, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED code, got %s", apiErr.Code)
	}
}

func TestValidateKey_ExpiredKey(t *testing.T) {
	svc, repo := newTestAPIKeyService()
	keyID := uuid.New()

	rawKey := "sk_test_expired00000000000000000000000"
	h := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(h[:])

	expired := time.Now().Add(-1 * time.Hour)
	repo.keys[keyID] = &model.APIKey{
		ID:        keyID,
		UserID:    uuid.New(),
		Name:      "Expired Key",
		KeyHash:   keyHash,
		KeyPrefix: rawKey[:12],
		IsActive:  true,
		ExpiresAt: &expired,
		RateLimit: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := svc.ValidateKey(context.Background(), rawKey)
	if err == nil {
		t.Fatal("expected error for expired key, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED code, got %s", apiErr.Code)
	}
}
