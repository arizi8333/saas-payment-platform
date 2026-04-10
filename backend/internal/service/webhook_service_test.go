package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock WebhookEndpointRepository ---

type mockWebhookEndpointRepo struct {
	endpoints map[uuid.UUID]*model.WebhookEndpoint
}

func newMockWebhookEndpointRepo() *mockWebhookEndpointRepo {
	return &mockWebhookEndpointRepo{endpoints: make(map[uuid.UUID]*model.WebhookEndpoint)}
}

func (m *mockWebhookEndpointRepo) Create(_ context.Context, ep *model.WebhookEndpoint) error {
	if ep.ID == uuid.Nil {
		ep.ID = uuid.New()
	}
	ep.CreatedAt = time.Now()
	ep.UpdatedAt = time.Now()
	m.endpoints[ep.ID] = ep
	return nil
}

func (m *mockWebhookEndpointRepo) FindByUserID(_ context.Context, userID uuid.UUID, page, pageSize int) ([]model.WebhookEndpoint, int64, error) {
	var result []model.WebhookEndpoint
	for _, ep := range m.endpoints {
		if ep.UserID == userID {
			result = append(result, *ep)
		}
	}
	total := int64(len(result))
	offset := (page - 1) * pageSize
	if offset >= len(result) {
		return []model.WebhookEndpoint{}, total, nil
	}
	end := offset + pageSize
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockWebhookEndpointRepo) FindByID(_ context.Context, id uuid.UUID) (*model.WebhookEndpoint, error) {
	if ep, ok := m.endpoints[id]; ok {
		return ep, nil
	}
	return nil, apierror.NewNotFound("webhook endpoint not found")
}

func (m *mockWebhookEndpointRepo) Update(_ context.Context, ep *model.WebhookEndpoint) error {
	if _, ok := m.endpoints[ep.ID]; !ok {
		return apierror.NewNotFound("webhook endpoint not found")
	}
	m.endpoints[ep.ID] = ep
	return nil
}

func (m *mockWebhookEndpointRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.endpoints[id]; !ok {
		return apierror.NewNotFound("webhook endpoint not found")
	}
	delete(m.endpoints, id)
	return nil
}

func (m *mockWebhookEndpointRepo) FindActiveByUserIDAndEvent(_ context.Context, userID uuid.UUID, _ string) ([]model.WebhookEndpoint, error) {
	var result []model.WebhookEndpoint
	for _, ep := range m.endpoints {
		if ep.UserID == userID && ep.IsActive {
			result = append(result, *ep)
		}
	}
	return result, nil
}

// --- Mock WebhookDeliveryRepository ---

type mockWebhookDeliveryRepo struct {
	deliveries map[uuid.UUID]*model.WebhookDelivery
}

func newMockWebhookDeliveryRepo() *mockWebhookDeliveryRepo {
	return &mockWebhookDeliveryRepo{deliveries: make(map[uuid.UUID]*model.WebhookDelivery)}
}

func (m *mockWebhookDeliveryRepo) Create(_ context.Context, d *model.WebhookDelivery) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()
	m.deliveries[d.ID] = d
	return nil
}

func (m *mockWebhookDeliveryRepo) FindPendingDeliveries(_ context.Context, limit int) ([]model.WebhookDelivery, error) {
	var result []model.WebhookDelivery
	for _, d := range m.deliveries {
		if d.Status == model.DeliveryPending {
			result = append(result, *d)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockWebhookDeliveryRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.DeliveryStatus, responseCode *int, responseBody *string) error {
	d, ok := m.deliveries[id]
	if !ok {
		return apierror.NewNotFound("webhook delivery not found")
	}
	d.Status = status
	d.ResponseCode = responseCode
	d.ResponseBody = responseBody
	if status == model.DeliveryDelivered {
		now := time.Now()
		d.DeliveredAt = &now
	}
	return nil
}

func (m *mockWebhookDeliveryRepo) IncrementRetry(_ context.Context, id uuid.UUID) error {
	d, ok := m.deliveries[id]
	if !ok {
		return apierror.NewNotFound("webhook delivery not found")
	}
	d.RetryCount++
	return nil
}

// --- Helper ---

func newTestWebhookService() (WebhookService, *mockWebhookEndpointRepo, *mockWebhookDeliveryRepo) {
	epRepo := newMockWebhookEndpointRepo()
	dlRepo := newMockWebhookDeliveryRepo()
	txm := &mockTxManager{}
	svc := NewWebhookService(epRepo, dlRepo, txm)
	return svc, epRepo, dlRepo
}

func createTestEndpoint(repo *mockWebhookEndpointRepo, userID uuid.UUID, url string, events []string, active bool) *model.WebhookEndpoint {
	eventsJSON, _ := json.Marshal(events)
	ep := &model.WebhookEndpoint{
		ID:        uuid.New(),
		UserID:    userID,
		URL:       url,
		Secret:    "whsec_testsecretvalue0000000000000000000000000000000000000000000000",
		Events:    datatypes.JSON(eventsJSON),
		IsActive:  active,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.endpoints[ep.ID] = ep
	return ep
}

// --- Tests: RegisterEndpoint ---

func TestRegisterEndpoint_Success(t *testing.T) {
	svc, repo, _ := newTestWebhookService()
	userID := uuid.New()

	req := &dto.CreateWebhookRequest{
		URL:    "https://example.com/webhook",
		Events: []string{"transaction.success", "transaction.failed"},
	}

	resp, err := svc.RegisterEndpoint(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.URL != req.URL {
		t.Errorf("expected URL %s, got %s", req.URL, resp.URL)
	}
	if !resp.IsActive {
		t.Error("expected is_active true")
	}
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}
	// Secret should be returned on creation and start with whsec_ prefix.
	if !strings.HasPrefix(resp.Secret, "whsec_") {
		t.Errorf("expected secret to start with whsec_, got %s", resp.Secret)
	}
	// Secret should be whsec_ (6) + 64 hex chars = 70 chars total.
	if len(resp.Secret) != 70 {
		t.Errorf("expected secret length 70, got %d", len(resp.Secret))
	}

	// Verify endpoint was stored.
	if len(repo.endpoints) != 1 {
		t.Fatalf("expected 1 endpoint in repo, got %d", len(repo.endpoints))
	}
}

func TestRegisterEndpoint_EmptyURL(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	req := &dto.CreateWebhookRequest{
		URL:    "",
		Events: []string{"transaction.success"},
	}

	_, err := svc.RegisterEndpoint(context.Background(), uuid.New(), req)
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

func TestRegisterEndpoint_NoEvents(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	req := &dto.CreateWebhookRequest{
		URL:    "https://example.com/webhook",
		Events: []string{},
	}

	_, err := svc.RegisterEndpoint(context.Background(), uuid.New(), req)
	if err == nil {
		t.Fatal("expected error for empty events, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST code, got %s", apiErr.Code)
	}
}

// --- Tests: ListEndpoints ---

func TestListEndpoints_Success(t *testing.T) {
	svc, repo, _ := newTestWebhookService()
	userID := uuid.New()

	createTestEndpoint(repo, userID, "https://example.com/hook1", []string{"transaction.success"}, true)
	createTestEndpoint(repo, userID, "https://example.com/hook2", []string{"transaction.failed"}, true)

	resp, err := svc.ListEndpoints(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(resp.Endpoints))
	}
	if resp.Pagination.TotalItems != 2 {
		t.Errorf("expected total_items 2, got %d", resp.Pagination.TotalItems)
	}
	// Secret should NOT be returned in list responses.
	for _, ep := range resp.Endpoints {
		if ep.Secret != "" {
			t.Error("expected secret to be empty in list response")
		}
	}
}

func TestListEndpoints_Empty(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	resp, err := svc.ListEndpoints(context.Background(), uuid.New(), 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Endpoints) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(resp.Endpoints))
	}
}

// --- Tests: DeleteEndpoint ---

func TestDeleteEndpoint_Success(t *testing.T) {
	svc, repo, _ := newTestWebhookService()
	userID := uuid.New()

	ep := createTestEndpoint(repo, userID, "https://example.com/hook", []string{"transaction.success"}, true)

	err := svc.DeleteEndpoint(context.Background(), userID, ep.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(repo.endpoints) != 0 {
		t.Errorf("expected 0 endpoints after delete, got %d", len(repo.endpoints))
	}
}

func TestDeleteEndpoint_Forbidden(t *testing.T) {
	svc, repo, _ := newTestWebhookService()
	ownerID := uuid.New()
	otherUserID := uuid.New()

	ep := createTestEndpoint(repo, ownerID, "https://example.com/hook", []string{"transaction.success"}, true)

	err := svc.DeleteEndpoint(context.Background(), otherUserID, ep.ID)
	if err == nil {
		t.Fatal("expected error for forbidden delete, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN code, got %s", apiErr.Code)
	}
}

func TestDeleteEndpoint_NotFound(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	err := svc.DeleteEndpoint(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for non-existent endpoint, got nil")
	}

	var apiErr *apierror.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND code, got %s", apiErr.Code)
	}
}

// --- Tests: TriggerWebhook ---

func TestTriggerWebhook_Success(t *testing.T) {
	svc, repo, dlRepo := newTestWebhookService()
	userID := uuid.New()

	createTestEndpoint(repo, userID, "https://example.com/hook1", []string{"transaction.success"}, true)
	createTestEndpoint(repo, userID, "https://example.com/hook2", []string{"transaction.success", "transaction.failed"}, true)

	payload := map[string]interface{}{
		"transaction_id": uuid.New().String(),
		"status":         "success",
	}

	err := svc.TriggerWebhook(context.Background(), userID, "transaction.success", payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Both endpoints subscribe to transaction.success, so 2 deliveries should be created.
	if len(dlRepo.deliveries) != 2 {
		t.Errorf("expected 2 deliveries, got %d", len(dlRepo.deliveries))
	}

	for _, d := range dlRepo.deliveries {
		if d.Status != model.DeliveryPending {
			t.Errorf("expected delivery status pending, got %s", d.Status)
		}
		if d.EventType != "transaction.success" {
			t.Errorf("expected event_type transaction.success, got %s", d.EventType)
		}
		if d.MaxRetries != 5 {
			t.Errorf("expected max_retries 5, got %d", d.MaxRetries)
		}
	}
}

func TestTriggerWebhook_FiltersByEventType(t *testing.T) {
	svc, repo, dlRepo := newTestWebhookService()
	userID := uuid.New()

	createTestEndpoint(repo, userID, "https://example.com/hook1", []string{"transaction.success"}, true)
	createTestEndpoint(repo, userID, "https://example.com/hook2", []string{"transaction.failed"}, true)

	payload := map[string]interface{}{"status": "failed"}

	err := svc.TriggerWebhook(context.Background(), userID, "transaction.failed", payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Only hook2 subscribes to transaction.failed.
	if len(dlRepo.deliveries) != 1 {
		t.Errorf("expected 1 delivery, got %d", len(dlRepo.deliveries))
	}
}

func TestTriggerWebhook_WildcardEvent(t *testing.T) {
	svc, repo, dlRepo := newTestWebhookService()
	userID := uuid.New()

	createTestEndpoint(repo, userID, "https://example.com/hook", []string{"*"}, true)

	payload := map[string]interface{}{"status": "success"}

	err := svc.TriggerWebhook(context.Background(), userID, "transaction.success", payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(dlRepo.deliveries) != 1 {
		t.Errorf("expected 1 delivery for wildcard, got %d", len(dlRepo.deliveries))
	}
}

func TestTriggerWebhook_NoMatchingEndpoints(t *testing.T) {
	svc, repo, dlRepo := newTestWebhookService()
	userID := uuid.New()

	createTestEndpoint(repo, userID, "https://example.com/hook", []string{"transaction.success"}, true)

	payload := map[string]interface{}{"status": "expired"}

	err := svc.TriggerWebhook(context.Background(), userID, "transaction.expired", payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(dlRepo.deliveries) != 0 {
		t.Errorf("expected 0 deliveries, got %d", len(dlRepo.deliveries))
	}
}

func TestTriggerWebhook_InactiveEndpointSkipped(t *testing.T) {
	svc, repo, dlRepo := newTestWebhookService()
	userID := uuid.New()

	createTestEndpoint(repo, userID, "https://example.com/hook", []string{"transaction.success"}, false)

	payload := map[string]interface{}{"status": "success"}

	err := svc.TriggerWebhook(context.Background(), userID, "transaction.success", payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(dlRepo.deliveries) != 0 {
		t.Errorf("expected 0 deliveries for inactive endpoint, got %d", len(dlRepo.deliveries))
	}
}

// --- Tests: GetDeliveries ---

func TestGetDeliveries_Success(t *testing.T) {
	svc, repo, dlRepo := newTestWebhookService()
	userID := uuid.New()

	ep := createTestEndpoint(repo, userID, "https://example.com/hook", []string{"transaction.success"}, true)

	// Create deliveries directly in the mock.
	payloadJSON, _ := json.Marshal(map[string]interface{}{"status": "success"})
	for i := 0; i < 3; i++ {
		d := &model.WebhookDelivery{
			ID:                uuid.New(),
			WebhookEndpointID: ep.ID,
			EventType:         "transaction.success",
			Payload:           datatypes.JSON(payloadJSON),
			Status:            model.DeliveryPending,
			MaxRetries:        5,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		dlRepo.deliveries[d.ID] = d
	}

	resp, err := svc.GetDeliveries(context.Background(), userID, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Deliveries) != 3 {
		t.Errorf("expected 3 deliveries, got %d", len(resp.Deliveries))
	}
	if resp.Pagination.TotalItems != 3 {
		t.Errorf("expected total_items 3, got %d", resp.Pagination.TotalItems)
	}
}

func TestGetDeliveries_NoEndpoints(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	resp, err := svc.GetDeliveries(context.Background(), uuid.New(), 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resp.Deliveries) != 0 {
		t.Errorf("expected 0 deliveries, got %d", len(resp.Deliveries))
	}
}

// --- Tests: GenerateSignature ---

func TestGenerateSignature_HMACSHA256(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	payload := []byte(`{"transaction_id":"abc","status":"success"}`)
	testSecret := "whsec_testsecretvalue"

	sig := svc.GenerateSignature(payload, testSecret)

	// Verify independently.
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if sig != expected {
		t.Errorf("expected signature %s, got %s", expected, sig)
	}
}

func TestGenerateSignature_DifferentPayloads(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	testSecret := "whsec_testsecretvalue"
	sig1 := svc.GenerateSignature([]byte(`{"a":"1"}`), testSecret)
	sig2 := svc.GenerateSignature([]byte(`{"a":"2"}`), testSecret)

	if sig1 == sig2 {
		t.Error("expected different signatures for different payloads")
	}
}

func TestGenerateSignature_DifferentSecrets(t *testing.T) {
	svc, _, _ := newTestWebhookService()

	payload := []byte(`{"status":"success"}`)
	sig1 := svc.GenerateSignature(payload, "testsecret1")
	sig2 := svc.GenerateSignature(payload, "testsecret2")

	if sig1 == sig2 {
		t.Error("expected different signatures for different secrets")
	}
}

// --- Tests: endpointSubscribesToEvent ---

func TestEndpointSubscribesToEvent(t *testing.T) {
	tests := []struct {
		name      string
		events    []string
		eventType string
		expected  bool
	}{
		{"exact match", []string{"transaction.success"}, "transaction.success", true},
		{"no match", []string{"transaction.success"}, "transaction.failed", false},
		{"wildcard match", []string{"*"}, "transaction.success", true},
		{"multiple events match", []string{"transaction.success", "transaction.failed"}, "transaction.failed", true},
		{"empty events", []string{}, "transaction.success", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventsJSON, _ := json.Marshal(tt.events)
			ep := &model.WebhookEndpoint{Events: datatypes.JSON(eventsJSON)}
			result := endpointSubscribesToEvent(ep, tt.eventType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
