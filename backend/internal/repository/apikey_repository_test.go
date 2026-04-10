package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// testAPIKey is a SQLite-compatible version of model.APIKey for testing.
// The production model uses PostgreSQL-specific defaults (gen_random_uuid(), now())
// which are not supported by SQLite.
type testAPIKey struct {
	ID         uuid.UUID `gorm:"type:text;primary_key"`
	UserID     uuid.UUID `gorm:"type:text;not null;index"`
	Name       string    `gorm:"type:varchar(100);not null"`
	KeyHash    string    `gorm:"type:varchar(255);not null;uniqueIndex"`
	KeyPrefix  string    `gorm:"type:varchar(12);not null"`
	IsActive   bool      `gorm:"not null;default:true"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RateLimit  int            `gorm:"not null;default:1000"`
	CreatedAt  time.Time      `gorm:"not null"`
	UpdatedAt  time.Time      `gorm:"not null"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// TableName maps testAPIKey to the same table name GORM uses for model.APIKey.
func (testAPIKey) TableName() string {
	return "api_keys"
}

func setupAPIKeyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&testUser{}, &testAPIKey{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func createTestUserInDB(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	user := newTestUser()
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func newTestAPIKey(userID uuid.UUID) *model.APIKey {
	return &model.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      "Test Key",
		KeyHash:   "hash_" + uuid.New().String(),
		KeyPrefix: "sk_test_abc",
		IsActive:  true,
		RateLimit: 1000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestAPIKeyCreate_Success(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	user := createTestUserInDB(t, db)

	apiKey := newTestAPIKey(user.ID)
	err := repo.Create(context.Background(), apiKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.APIKey
	db.First(&found, "id = ?", apiKey.ID)
	if found.Name != apiKey.Name {
		t.Errorf("expected name %s, got %s", apiKey.Name, found.Name)
	}
}

func TestAPIKeyFindByKeyHash_Success(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	user := createTestUserInDB(t, db)

	apiKey := newTestAPIKey(user.ID)
	db.Create(apiKey)

	found, err := repo.FindByKeyHash(context.Background(), apiKey.KeyHash)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.ID != apiKey.ID {
		t.Errorf("expected ID %s, got %s", apiKey.ID, found.ID)
	}
}

func TestAPIKeyFindByKeyHash_NotFound(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)

	_, err := repo.FindByKeyHash(context.Background(), "nonexistent_hash")
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

func TestAPIKeyFindByUserID_Success(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	user := createTestUserInDB(t, db)

	for i := 0; i < 3; i++ {
		key := newTestAPIKey(user.ID)
		db.Create(key)
	}

	keys, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestAPIKeyFindByUserID_Pagination(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	user := createTestUserInDB(t, db)

	for i := 0; i < 5; i++ {
		key := newTestAPIKey(user.ID)
		db.Create(key)
	}

	keys, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys on page 1, got %d", len(keys))
	}

	// Page 3 should have 1 item
	keys, total, err = repo.FindByUserID(context.Background(), user.ID, 3, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key on page 3, got %d", len(keys))
	}
}

func TestAPIKeyFindByID_Success(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	user := createTestUserInDB(t, db)

	apiKey := newTestAPIKey(user.ID)
	db.Create(apiKey)

	found, err := repo.FindByID(context.Background(), apiKey.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.KeyHash != apiKey.KeyHash {
		t.Errorf("expected key hash %s, got %s", apiKey.KeyHash, found.KeyHash)
	}
}

func TestAPIKeyFindByID_NotFound(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)

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

func TestAPIKeyUpdate_Success(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	user := createTestUserInDB(t, db)

	apiKey := newTestAPIKey(user.ID)
	db.Create(apiKey)

	apiKey.Name = "Updated Key Name"
	apiKey.IsActive = false
	err := repo.Update(context.Background(), apiKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.APIKey
	db.First(&found, "id = ?", apiKey.ID)
	if found.Name != "Updated Key Name" {
		t.Errorf("expected name 'Updated Key Name', got %s", found.Name)
	}
	if found.IsActive != false {
		t.Errorf("expected IsActive false, got %v", found.IsActive)
	}
}

func TestAPIKeyUpdateLastUsed_Success(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	user := createTestUserInDB(t, db)

	apiKey := newTestAPIKey(user.ID)
	db.Create(apiKey)

	err := repo.UpdateLastUsed(context.Background(), apiKey.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.APIKey
	db.First(&found, "id = ?", apiKey.ID)
	if found.LastUsedAt == nil {
		t.Fatal("expected LastUsedAt to be set, got nil")
	}
}

func TestAPIKeyUpdateLastUsed_NotFound(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)

	err := repo.UpdateLastUsed(context.Background(), uuid.New())
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

func TestAPIKeyCreate_WithTransaction(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserInDB(t, db)

	apiKey := newTestAPIKey(user.ID)
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, apiKey)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.APIKey{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 api key, got %d", count)
	}
}

func TestAPIKeyCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserInDB(t, db)

	apiKey := newTestAPIKey(user.ID)
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, apiKey); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.APIKey{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 api keys after rollback, got %d", count)
	}
}
