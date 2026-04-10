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

// testTransaction is a SQLite-compatible version of model.Transaction for testing.
// The production model uses PostgreSQL-specific defaults (gen_random_uuid(), now(), jsonb)
// which are not supported by SQLite.
type testTransaction struct {
	ID             uuid.UUID               `gorm:"type:text;primary_key"`
	UserID         uuid.UUID               `gorm:"type:text;not null;index"`
	ExternalID     string                  `gorm:"type:varchar(255);not null;uniqueIndex"`
	Amount         int64                   `gorm:"not null"`
	Currency       string                  `gorm:"type:varchar(3);not null;default:'IDR'"`
	Status         model.TransactionStatus `gorm:"type:varchar(20);not null;default:'pending'"`
	PaymentMethod  model.PaymentMethod     `gorm:"type:varchar(30);not null"`
	Description    string                  `gorm:"type:text"`
	CustomerEmail  string                  `gorm:"type:varchar(255)"`
	IdempotencyKey *string                 `gorm:"type:varchar(255);uniqueIndex"`
	Metadata       string                  `gorm:"type:text"`
	PaidAt         *time.Time
	ExpiredAt      *time.Time
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// TableName maps testTransaction to the same table name GORM uses for model.Transaction.
func (testTransaction) TableName() string {
	return "transactions"
}

func setupTransactionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&testUser{}, &testTransaction{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func createTestUserForTx(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	user := newTestUser()
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func newTestTransaction(userID uuid.UUID) *model.Transaction {
	return &model.Transaction{
		ID:            uuid.New(),
		UserID:        userID,
		ExternalID:    "ext_" + uuid.New().String(),
		Amount:        100000,
		Currency:      "IDR",
		Status:        model.TxStatusPending,
		PaymentMethod: model.PMBankTransfer,
		Description:   "Test transaction",
		CustomerEmail: "customer@example.com",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func TestTransactionCreate_Success(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	tx := newTestTransaction(user.ID)
	err := repo.Create(context.Background(), tx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Transaction
	db.First(&found, "id = ?", tx.ID)
	if found.ExternalID != tx.ExternalID {
		t.Errorf("expected external_id %s, got %s", tx.ExternalID, found.ExternalID)
	}
	if found.Amount != tx.Amount {
		t.Errorf("expected amount %d, got %d", tx.Amount, found.Amount)
	}
}

func TestTransactionFindByID_Success(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	tx := newTestTransaction(user.ID)
	db.Create(tx)

	found, err := repo.FindByID(context.Background(), tx.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.ExternalID != tx.ExternalID {
		t.Errorf("expected external_id %s, got %s", tx.ExternalID, found.ExternalID)
	}
}

func TestTransactionFindByID_NotFound(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)

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

func TestTransactionFindByUserID_Success(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	for i := 0; i < 3; i++ {
		tx := newTestTransaction(user.ID)
		db.Create(tx)
	}

	txs, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(txs) != 3 {
		t.Errorf("expected 3 transactions, got %d", len(txs))
	}
}

func TestTransactionFindByUserID_Pagination(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	for i := 0; i < 5; i++ {
		tx := newTestTransaction(user.ID)
		db.Create(tx)
	}

	txs, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(txs) != 2 {
		t.Errorf("expected 2 transactions on page 1, got %d", len(txs))
	}

	// Page 3 should have 1 item
	txs, total, err = repo.FindByUserID(context.Background(), user.ID, 3, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(txs) != 1 {
		t.Errorf("expected 1 transaction on page 3, got %d", len(txs))
	}
}

func TestTransactionFindByUserID_OrderByCreatedAtDesc(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	// Create transactions with distinct created_at times
	for i := 0; i < 3; i++ {
		tx := newTestTransaction(user.ID)
		tx.CreatedAt = time.Now().Add(time.Duration(i) * time.Second)
		tx.Description = fmt.Sprintf("tx_%d", i)
		db.Create(tx)
	}

	txs, _, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Most recent first
	if txs[0].Description != "tx_2" {
		t.Errorf("expected first transaction to be tx_2, got %s", txs[0].Description)
	}
	if txs[2].Description != "tx_0" {
		t.Errorf("expected last transaction to be tx_0, got %s", txs[2].Description)
	}
}

func TestTransactionFindByExternalID_Success(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	tx := newTestTransaction(user.ID)
	db.Create(tx)

	found, err := repo.FindByExternalID(context.Background(), tx.ExternalID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.ID != tx.ID {
		t.Errorf("expected ID %s, got %s", tx.ID, found.ID)
	}
}

func TestTransactionFindByExternalID_NotFound(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)

	_, err := repo.FindByExternalID(context.Background(), "nonexistent_ext_id")
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

func TestTransactionUpdateStatus_Success(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	tx := newTestTransaction(user.ID)
	db.Create(tx)

	err := repo.UpdateStatus(context.Background(), tx.ID, model.TxStatusFailed)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Transaction
	db.First(&found, "id = ?", tx.ID)
	if found.Status != model.TxStatusFailed {
		t.Errorf("expected status %s, got %s", model.TxStatusFailed, found.Status)
	}
	if found.PaidAt != nil {
		t.Error("expected PaidAt to be nil for failed status")
	}
}

func TestTransactionUpdateStatus_SuccessSetsPaidAt(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	user := createTestUserForTx(t, db)

	tx := newTestTransaction(user.ID)
	db.Create(tx)

	err := repo.UpdateStatus(context.Background(), tx.ID, model.TxStatusSuccess)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Transaction
	db.First(&found, "id = ?", tx.ID)
	if found.Status != model.TxStatusSuccess {
		t.Errorf("expected status %s, got %s", model.TxStatusSuccess, found.Status)
	}
	if found.PaidAt == nil {
		t.Error("expected PaidAt to be set for success status")
	}
}

func TestTransactionUpdateStatus_NotFound(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)

	err := repo.UpdateStatus(context.Background(), uuid.New(), model.TxStatusSuccess)
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

func TestTransactionCreate_WithTransaction(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForTx(t, db)

	tx := newTestTransaction(user.ID)
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, tx)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.Transaction{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 transaction, got %d", count)
	}
}

func TestTransactionCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupTransactionTestDB(t)
	repo := NewTransactionRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForTx(t, db)

	tx := newTestTransaction(user.ID)
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, tx); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.Transaction{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 transactions after rollback, got %d", count)
	}
}
