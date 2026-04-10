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

// testWebhookEndpoint is a SQLite-compatible version of model.WebhookEndpoint for testing.
// The production model uses PostgreSQL-specific defaults (gen_random_uuid(), now(), jsonb)
// which are not supported by SQLite.
type testWebhookEndpoint struct {
	ID        uuid.UUID      `gorm:"type:text;primary_key"`
	UserID    uuid.UUID      `gorm:"type:text;not null;index"`
	URL       string         `gorm:"type:varchar(500);not null"`
	Secret    string         `gorm:"type:varchar(255);not null"`
	Events    string         `gorm:"type:text;not null"`
	IsActive  bool           `gorm:"not null;default:true"`
	CreatedAt time.Time      `gorm:"not null"`
	UpdatedAt time.Time      `gorm:"not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName maps testWebhookEndpoint to the same table name GORM uses for model.WebhookEndpoint.
func (testWebhookEndpoint) TableName() string {
	return "webhook_endpoints"
}

// testWebhookDelivery is a SQLite-compatible version of model.WebhookDelivery for testing.
type testWebhookDelivery struct {
	ID                uuid.UUID            `gorm:"type:text;primary_key"`
	WebhookEndpointID uuid.UUID            `gorm:"type:text;not null;index"`
	EventType         string               `gorm:"type:varchar(50);not null"`
	Payload           string               `gorm:"type:text;not null"`
	Status            model.DeliveryStatus `gorm:"type:varchar(20);not null;default:'pending'"`
	ResponseCode      *int
	ResponseBody      *string    `gorm:"type:text"`
	RetryCount        int        `gorm:"not null;default:0"`
	MaxRetries        int        `gorm:"not null;default:5"`
	NextRetryAt       *time.Time `gorm:"index"`
	DeliveredAt       *time.Time
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

// TableName maps testWebhookDelivery to the same table name GORM uses for model.WebhookDelivery.
func (testWebhookDelivery) TableName() string {
	return "webhook_deliveries"
}

func setupWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&testUser{}, &testWebhookEndpoint{}, &testWebhookDelivery{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func createTestUserForWebhook(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	user := newTestUser()
	user.Email = fmt.Sprintf("webhook_test_%s@example.com", uuid.New().String()[:8])
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func newTestWebhookEndpoint(userID uuid.UUID) *model.WebhookEndpoint {
	return &model.WebhookEndpoint{
		ID:        uuid.New(),
		UserID:    userID,
		URL:       "https://example.com/webhook",
		Secret:    "whsec_test_placeholder",
		Events:    []byte(`["transaction.success","transaction.failed"]`),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func newTestWebhookDelivery(endpointID uuid.UUID) *model.WebhookDelivery {
	return &model.WebhookDelivery{
		ID:                uuid.New(),
		WebhookEndpointID: endpointID,
		EventType:         "transaction.success",
		Payload:           []byte(`{"transaction_id":"abc-123","status":"success"}`),
		Status:            model.DeliveryPending,
		RetryCount:        0,
		MaxRetries:        5,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

// --- WebhookEndpoint Repository Tests ---

func TestWebhookEndpointCreate_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	user := createTestUserForWebhook(t, db)

	ep := newTestWebhookEndpoint(user.ID)
	err := repo.Create(context.Background(), ep)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.WebhookEndpoint
	db.First(&found, "id = ?", ep.ID)
	if found.URL != ep.URL {
		t.Errorf("expected URL %s, got %s", ep.URL, found.URL)
	}
	if found.IsActive != true {
		t.Error("expected IsActive to be true")
	}
}

func TestWebhookEndpointFindByID_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	user := createTestUserForWebhook(t, db)

	ep := newTestWebhookEndpoint(user.ID)
	db.Create(ep)

	found, err := repo.FindByID(context.Background(), ep.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.URL != ep.URL {
		t.Errorf("expected URL %s, got %s", ep.URL, found.URL)
	}
}

func TestWebhookEndpointFindByID_NotFound(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)

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

func TestWebhookEndpointFindByUserID_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	user := createTestUserForWebhook(t, db)

	for i := 0; i < 3; i++ {
		ep := newTestWebhookEndpoint(user.ID)
		ep.URL = fmt.Sprintf("https://example.com/webhook/%d", i)
		db.Create(ep)
	}

	endpoints, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(endpoints))
	}
}

func TestWebhookEndpointFindByUserID_Pagination(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	user := createTestUserForWebhook(t, db)

	for i := 0; i < 5; i++ {
		ep := newTestWebhookEndpoint(user.ID)
		ep.URL = fmt.Sprintf("https://example.com/webhook/%d", i)
		db.Create(ep)
	}

	endpoints, total, err := repo.FindByUserID(context.Background(), user.ID, 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(endpoints) != 2 {
		t.Errorf("expected 2 endpoints on page 1, got %d", len(endpoints))
	}

	// Page 3 should have 1 item
	endpoints, total, err = repo.FindByUserID(context.Background(), user.ID, 3, 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(endpoints) != 1 {
		t.Errorf("expected 1 endpoint on page 3, got %d", len(endpoints))
	}
}

func TestWebhookEndpointUpdate_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	user := createTestUserForWebhook(t, db)

	ep := newTestWebhookEndpoint(user.ID)
	db.Create(ep)

	ep.URL = "https://updated.example.com/webhook"
	ep.IsActive = false
	err := repo.Update(context.Background(), ep)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.WebhookEndpoint
	db.First(&found, "id = ?", ep.ID)
	if found.URL != "https://updated.example.com/webhook" {
		t.Errorf("expected updated URL, got %s", found.URL)
	}
	if found.IsActive != false {
		t.Error("expected IsActive to be false")
	}
}

func TestWebhookEndpointDelete_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	user := createTestUserForWebhook(t, db)

	ep := newTestWebhookEndpoint(user.ID)
	db.Create(ep)

	err := repo.Delete(context.Background(), ep.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should not be found via normal query (soft deleted)
	_, err = repo.FindByID(context.Background(), ep.ID)
	if err == nil {
		t.Fatal("expected not found error after delete, got nil")
	}
	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

func TestWebhookEndpointDelete_NotFound(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)

	err := repo.Delete(context.Background(), uuid.New())
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

func TestWebhookEndpointFindActiveByUserIDAndEvent_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	user := createTestUserForWebhook(t, db)

	// Create active endpoint
	ep1 := newTestWebhookEndpoint(user.ID)
	ep1.IsActive = true
	db.Create(ep1)

	// Create inactive endpoint — use db.Model().Update() to set false (zero value)
	ep2 := newTestWebhookEndpoint(user.ID)
	ep2.URL = "https://inactive.example.com/webhook"
	db.Create(ep2)
	db.Model(&model.WebhookEndpoint{}).Where("id = ?", ep2.ID).Update("is_active", false)

	endpoints, err := repo.FindActiveByUserIDAndEvent(context.Background(), user.ID, "transaction.success")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(endpoints) != 1 {
		t.Errorf("expected 1 active endpoint, got %d", len(endpoints))
	}
	if endpoints[0].ID != ep1.ID {
		t.Errorf("expected active endpoint ID %s, got %s", ep1.ID, endpoints[0].ID)
	}
}

func TestWebhookEndpointCreate_WithTransaction(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForWebhook(t, db)

	ep := newTestWebhookEndpoint(user.ID)
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, ep)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.WebhookEndpoint{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 endpoint, got %d", count)
	}
}

func TestWebhookEndpointCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookEndpointRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForWebhook(t, db)

	ep := newTestWebhookEndpoint(user.ID)
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, ep); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.WebhookEndpoint{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 endpoints after rollback, got %d", count)
	}
}

// --- WebhookDelivery Repository Tests ---

func createTestEndpointForDelivery(t *testing.T, db *gorm.DB, userID uuid.UUID) *model.WebhookEndpoint {
	t.Helper()
	ep := newTestWebhookEndpoint(userID)
	if err := db.Create(ep).Error; err != nil {
		t.Fatalf("failed to create test endpoint: %v", err)
	}
	return ep
}

func TestWebhookDeliveryCreate_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	delivery := newTestWebhookDelivery(ep.ID)
	err := repo.Create(context.Background(), delivery)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.WebhookDelivery
	db.First(&found, "id = ?", delivery.ID)
	if found.EventType != delivery.EventType {
		t.Errorf("expected event_type %s, got %s", delivery.EventType, found.EventType)
	}
	if found.Status != model.DeliveryPending {
		t.Errorf("expected status %s, got %s", model.DeliveryPending, found.Status)
	}
}

func TestWebhookDeliveryFindPendingDeliveries_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	// Create pending delivery with no next_retry_at (first attempt)
	d1 := newTestWebhookDelivery(ep.ID)
	db.Create(d1)

	// Create pending delivery with next_retry_at in the past (ready for retry)
	d2 := newTestWebhookDelivery(ep.ID)
	pastTime := time.Now().Add(-1 * time.Minute)
	d2.NextRetryAt = &pastTime
	db.Create(d2)

	// Create pending delivery with next_retry_at in the future (not ready)
	d3 := newTestWebhookDelivery(ep.ID)
	futureTime := time.Now().Add(1 * time.Hour)
	d3.NextRetryAt = &futureTime
	db.Create(d3)

	deliveries, err := repo.FindPendingDeliveries(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(deliveries) != 2 {
		t.Errorf("expected 2 pending deliveries, got %d", len(deliveries))
	}
}

func TestWebhookDeliveryFindPendingDeliveries_ExcludesNonPending(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	// Create pending delivery
	d1 := newTestWebhookDelivery(ep.ID)
	db.Create(d1)

	// Create delivered delivery
	d2 := newTestWebhookDelivery(ep.ID)
	d2.Status = model.DeliveryDelivered
	db.Create(d2)

	// Create failed delivery
	d3 := newTestWebhookDelivery(ep.ID)
	d3.Status = model.DeliveryFailed
	db.Create(d3)

	deliveries, err := repo.FindPendingDeliveries(context.Background(), 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(deliveries) != 1 {
		t.Errorf("expected 1 pending delivery, got %d", len(deliveries))
	}
	if deliveries[0].ID != d1.ID {
		t.Errorf("expected delivery ID %s, got %s", d1.ID, deliveries[0].ID)
	}
}

func TestWebhookDeliveryFindPendingDeliveries_RespectsLimit(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	for i := 0; i < 5; i++ {
		d := newTestWebhookDelivery(ep.ID)
		db.Create(d)
	}

	deliveries, err := repo.FindPendingDeliveries(context.Background(), 3)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(deliveries) != 3 {
		t.Errorf("expected 3 deliveries (limit), got %d", len(deliveries))
	}
}

func TestWebhookDeliveryUpdateStatus_ToDelivered(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	delivery := newTestWebhookDelivery(ep.ID)
	db.Create(delivery)

	code := 200
	body := "OK"
	err := repo.UpdateStatus(context.Background(), delivery.ID, model.DeliveryDelivered, &code, &body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.WebhookDelivery
	db.First(&found, "id = ?", delivery.ID)
	if found.Status != model.DeliveryDelivered {
		t.Errorf("expected status %s, got %s", model.DeliveryDelivered, found.Status)
	}
	if found.ResponseCode == nil || *found.ResponseCode != 200 {
		t.Error("expected response_code 200")
	}
	if found.ResponseBody == nil || *found.ResponseBody != "OK" {
		t.Error("expected response_body 'OK'")
	}
	if found.DeliveredAt == nil {
		t.Error("expected DeliveredAt to be set for delivered status")
	}
}

func TestWebhookDeliveryUpdateStatus_ToFailed(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	delivery := newTestWebhookDelivery(ep.ID)
	db.Create(delivery)

	code := 500
	body := "Internal Server Error"
	err := repo.UpdateStatus(context.Background(), delivery.ID, model.DeliveryFailed, &code, &body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.WebhookDelivery
	db.First(&found, "id = ?", delivery.ID)
	if found.Status != model.DeliveryFailed {
		t.Errorf("expected status %s, got %s", model.DeliveryFailed, found.Status)
	}
	if found.DeliveredAt != nil {
		t.Error("expected DeliveredAt to be nil for failed status")
	}
}

func TestWebhookDeliveryUpdateStatus_NotFound(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)

	err := repo.UpdateStatus(context.Background(), uuid.New(), model.DeliveryDelivered, nil, nil)
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

func TestWebhookDeliveryIncrementRetry_Success(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	delivery := newTestWebhookDelivery(ep.ID)
	delivery.RetryCount = 0
	db.Create(delivery)

	err := repo.IncrementRetry(context.Background(), delivery.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.WebhookDelivery
	db.First(&found, "id = ?", delivery.ID)
	if found.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", found.RetryCount)
	}
	if found.NextRetryAt == nil {
		t.Fatal("expected NextRetryAt to be set")
	}
	// First retry: backoff = 2^0 = 1 second
	if found.NextRetryAt.Before(time.Now().Add(-2 * time.Second)) {
		t.Error("NextRetryAt is too far in the past")
	}
}

func TestWebhookDeliveryIncrementRetry_ExponentialBackoff(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	delivery := newTestWebhookDelivery(ep.ID)
	delivery.RetryCount = 3 // Next backoff should be 2^3 = 8 seconds
	db.Create(delivery)

	beforeRetry := time.Now()
	err := repo.IncrementRetry(context.Background(), delivery.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found model.WebhookDelivery
	db.First(&found, "id = ?", delivery.ID)
	if found.RetryCount != 4 {
		t.Errorf("expected retry_count 4, got %d", found.RetryCount)
	}
	if found.NextRetryAt == nil {
		t.Fatal("expected NextRetryAt to be set")
	}
	// Backoff for retry_count=3 is 2^3 = 8 seconds
	expectedMinRetry := beforeRetry.Add(7 * time.Second)
	if found.NextRetryAt.Before(expectedMinRetry) {
		t.Errorf("NextRetryAt %v is before expected minimum %v", found.NextRetryAt, expectedMinRetry)
	}
}

func TestWebhookDeliveryIncrementRetry_NotFound(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)

	err := repo.IncrementRetry(context.Background(), uuid.New())
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

func TestWebhookDeliveryCreate_WithTransaction(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	delivery := newTestWebhookDelivery(ep.ID)
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		return repo.Create(ctx, delivery)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&model.WebhookDelivery{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 delivery, got %d", count)
	}
}

func TestWebhookDeliveryCreate_TransactionRollbackOnError(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := NewWebhookDeliveryRepository(db)
	tm := database.NewTransactionManager(db)
	user := createTestUserForWebhook(t, db)
	ep := createTestEndpointForDelivery(t, db, user.ID)

	delivery := newTestWebhookDelivery(ep.ID)
	testErr := errors.New("forced error")

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		if err := repo.Create(ctx, delivery); err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}

	var count int64
	db.Model(&model.WebhookDelivery{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 deliveries after rollback, got %d", count)
	}
}
