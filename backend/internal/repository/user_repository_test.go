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

// testUser is a SQLite-compatible version of model.User for testing.
// The production model uses PostgreSQL-specific defaults (gen_random_uuid(), now())
// which are not supported by SQLite.
type testUser struct {
	ID           uuid.UUID      `gorm:"type:text;primary_key"`
	Email        string         `gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash string         `gorm:"type:varchar(255);not null"`
	FullName     string         `gorm:"type:varchar(255);not null"`
	Role         model.UserRole `gorm:"type:varchar(20);not null;default:'developer'"`
	IsActive     bool           `gorm:"not null;default:true"`
	CreatedAt    time.Time      `gorm:"not null"`
	UpdatedAt    time.Time      `gorm:"not null"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// TableName maps testUser to the same table name GORM uses for model.User.
func (testUser) TableName() string {
	return "users"
}

func setupUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&testUser{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func newTestUser() *model.User {
	return &model.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: "hashed_value_placeholder",
		FullName:     "Test User",
		Role:         model.RoleDeveloper,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func TestCreate_Success(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)

	user := newTestUser()
	err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.User
	db.First(&found, "id = ?", user.ID)
	if found.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, found.Email)
	}
}

func TestFindByEmail_Success(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)

	user := newTestUser()
	db.Create(user)

	found, err := repo.FindByEmail(context.Background(), user.Email)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, found.ID)
	}
}

func TestFindByEmail_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)

	_, err := repo.FindByEmail(context.Background(), "nonexistent@example.com")
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

func TestFindByID_Success(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)

	user := newTestUser()
	db.Create(user)

	found, err := repo.FindByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, found.Email)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)

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

func TestUpdate_Success(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)

	user := newTestUser()
	db.Create(user)

	user.FullName = "Updated Name"
	err := repo.Update(context.Background(), user)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.User
	db.First(&found, "id = ?", user.ID)
	if found.FullName != "Updated Name" {
		t.Errorf("expected FullName 'Updated Name', got %s", found.FullName)
	}
}

func TestCreate_WithTransaction(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)
	tm := database.NewTransactionManager(db)

	user := newTestUser()
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, user)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.User{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
}

func TestCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupUserTestDB(t)
	repo := NewUserRepository(db)
	tm := database.NewTransactionManager(db)

	user := newTestUser()
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, user); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.User{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 users after rollback, got %d", count)
	}
}
