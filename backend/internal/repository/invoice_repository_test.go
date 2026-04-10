package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// testInvoice is a SQLite-compatible version of model.Invoice for testing.
// The production model uses PostgreSQL-specific defaults (gen_random_uuid(), now())
// which are not supported by SQLite.
type testInvoice struct {
	ID             uuid.UUID           `gorm:"type:text;primary_key"`
	UserID         uuid.UUID           `gorm:"type:text;not null;index"`
	TransactionID  *uuid.UUID          `gorm:"type:text;index"`
	SubscriptionID *uuid.UUID          `gorm:"type:text;index"`
	InvoiceNumber  string              `gorm:"type:varchar(50);not null;uniqueIndex"`
	Amount         int64               `gorm:"not null"`
	Currency       string              `gorm:"type:varchar(3);not null;default:'IDR'"`
	Status         model.InvoiceStatus `gorm:"type:varchar(20);not null;default:'unpaid'"`
	DueDate        time.Time           `gorm:"not null"`
	PaidAt         *time.Time
	CreatedAt      time.Time      `gorm:"not null"`
	UpdatedAt      time.Time      `gorm:"not null"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// TableName maps testInvoice to the same table name GORM uses for model.Invoice.
func (testInvoice) TableName() string {
	return "invoices"
}

func setupInvoiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&testUser{}, &testInvoice{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func createTestUserForInvoice(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	user := newTestUser()
	user.Email = fmt.Sprintf("invoice_test_%s@example.com", uuid.New().String()[:8])
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func newTestInvoice(userID uuid.UUID, invoiceNumber string) *model.Invoice {
	return &model.Invoice{
		ID:            uuid.New(),
		UserID:        userID,
		InvoiceNumber: invoiceNumber,
		Amount:        100000,
		Currency:      "IDR",
		Status:        model.InvoiceUnpaid,
		DueDate:       time.Now().Add(7 * 24 * time.Hour),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func TestInvoiceCreate_Success(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	inv := newTestInvoice(user.ID, "INV-20250101-00001")
	err := repo.Create(context.Background(), inv)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Invoice
	db.First(&found, "id = ?", inv.ID)
	if found.InvoiceNumber != inv.InvoiceNumber {
		t.Errorf("expected invoice_number %s, got %s", inv.InvoiceNumber, found.InvoiceNumber)
	}
	if found.Amount != inv.Amount {
		t.Errorf("expected amount %d, got %d", inv.Amount, found.Amount)
	}
}

func TestInvoiceFindByID_Success(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	inv := newTestInvoice(user.ID, "INV-20250101-00001")
	db.Create(inv)

	found, err := repo.FindByID(context.Background(), inv.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.InvoiceNumber != inv.InvoiceNumber {
		t.Errorf("expected invoice_number %s, got %s", inv.InvoiceNumber, found.InvoiceNumber)
	}
}

func TestInvoiceFindByID_NotFound(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)

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

func TestInvoiceFindByUserID_Success(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	for i := 0; i < 3; i++ {
		inv := newTestInvoice(user.ID, fmt.Sprintf("INV-20250101-%05d", i+1))
		db.Create(inv)
	}

	invoices, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(invoices) != 3 {
		t.Errorf("expected 3 invoices, got %d", len(invoices))
	}
}

func TestInvoiceFindByUserID_Pagination(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	for i := 0; i < 5; i++ {
		inv := newTestInvoice(user.ID, fmt.Sprintf("INV-20250101-%05d", i+1))
		db.Create(inv)
	}

	invoices, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(invoices) != 2 {
		t.Errorf("expected 2 invoices on page 1, got %d", len(invoices))
	}

	// Page 3 should have 1 item
	invoices, total, err = repo.FindByUserID(context.Background(), user.ID, 3, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(invoices) != 1 {
		t.Errorf("expected 1 invoice on page 3, got %d", len(invoices))
	}
}

func TestInvoiceFindByUserID_OrderByCreatedAtDesc(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	for i := 0; i < 3; i++ {
		inv := newTestInvoice(user.ID, fmt.Sprintf("INV-20250101-%05d", i+1))
		inv.CreatedAt = time.Now().Add(time.Duration(i) * time.Second)
		inv.Amount = int64((i + 1) * 10000)
		db.Create(inv)
	}

	invoices, _, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Most recent first (amount 30000)
	if invoices[0].Amount != 30000 {
		t.Errorf("expected first invoice amount 30000, got %d", invoices[0].Amount)
	}
	if invoices[2].Amount != 10000 {
		t.Errorf("expected last invoice amount 10000, got %d", invoices[2].Amount)
	}
}

func TestInvoiceUpdateStatus_ToPaid(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	inv := newTestInvoice(user.ID, "INV-20250101-00001")
	db.Create(inv)

	err := repo.UpdateStatus(context.Background(), inv.ID, model.InvoicePaid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Invoice
	db.First(&found, "id = ?", inv.ID)
	if found.Status != model.InvoicePaid {
		t.Errorf("expected status %s, got %s", model.InvoicePaid, found.Status)
	}
	if found.PaidAt == nil {
		t.Error("expected PaidAt to be set for paid status")
	}
}

func TestInvoiceUpdateStatus_ToVoid(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	inv := newTestInvoice(user.ID, "INV-20250101-00001")
	db.Create(inv)

	err := repo.UpdateStatus(context.Background(), inv.ID, model.InvoiceVoid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.Invoice
	db.First(&found, "id = ?", inv.ID)
	if found.Status != model.InvoiceVoid {
		t.Errorf("expected status %s, got %s", model.InvoiceVoid, found.Status)
	}
	if found.PaidAt != nil {
		t.Error("expected PaidAt to be nil for void status")
	}
}

func TestInvoiceUpdateStatus_NotFound(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)

	err := repo.UpdateStatus(context.Background(), uuid.New(), model.InvoicePaid)
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

func TestInvoiceGenerateInvoiceNumber_Format(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)

	number, err := repo.GenerateInvoiceNumber(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	dateStr := time.Now().Format("20060102")
	expected := fmt.Sprintf("INV-%s-00001", dateStr)
	if number != expected {
		t.Errorf("expected %s, got %s", expected, number)
	}

	if !strings.HasPrefix(number, "INV-") {
		t.Errorf("expected prefix INV-, got %s", number)
	}
}

func TestInvoiceGenerateInvoiceNumber_Increments(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	user := createTestUserForInvoice(t, db)

	dateStr := time.Now().Format("20060102")

	// Create 3 invoices for today
	for i := 1; i <= 3; i++ {
		inv := newTestInvoice(user.ID, fmt.Sprintf("INV-%s-%05d", dateStr, i))
		db.Create(inv)
	}

	number, err := repo.GenerateInvoiceNumber(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := fmt.Sprintf("INV-%s-00004", dateStr)
	if number != expected {
		t.Errorf("expected %s, got %s", expected, number)
	}
}

func TestInvoiceCreate_WithTransaction(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForInvoice(t, db)

	inv := newTestInvoice(user.ID, "INV-20250101-00001")
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, inv)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.Invoice{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 invoice, got %d", count)
	}
}

func TestInvoiceCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupInvoiceTestDB(t)
	repo := NewInvoiceRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForInvoice(t, db)

	inv := newTestInvoice(user.ID, "INV-20250101-00001")
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, inv); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.Invoice{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 invoices after rollback, got %d", count)
	}
}
