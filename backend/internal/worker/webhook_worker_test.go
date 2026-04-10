package worker

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/saas-payment-platform/backend/internal/dto"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/pkg/apierror"
)

// --- Mock HTTP Client ---

type mockHTTPResponse struct {
	statusCode int
	body       string
	err        error
}

type mockHTTPClient struct {
	responses []mockHTTPResponse
	calls     int
	requests  []*http.Request
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	idx := m.calls
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1 // repeat last response
	}
	m.calls++
	r := m.responses[idx]
	if r.err != nil {
		return nil, r.err
	}
	return &http.Response{
		StatusCode: r.statusCode,
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}, nil
}

// --- Mock WebhookService (satisfies service.WebhookService) ---

type mockWebhookService struct{}

func (m *mockWebhookService) RegisterEndpoint(_ context.Context, _ uuid.UUID, _ *dto.CreateWebhookRequest) (*dto.WebhookResponse, error) {
	return nil, nil
}
func (m *mockWebhookService) ListEndpoints(_ context.Context, _ uuid.UUID, _, _ int) (*dto.WebhookListResponse, error) {
	return nil, nil
}
func (m *mockWebhookService) DeleteEndpoint(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
	return nil
}
func (m *mockWebhookService) TriggerWebhook(_ context.Context, _ uuid.UUID, _ string, _ map[string]interface{}) error {
	return nil
}
func (m *mockWebhookService) GetDeliveries(_ context.Context, _ uuid.UUID, _, _ int) (*dto.WebhookDeliveryListResponse, error) {
	return nil, nil
}
func (m *mockWebhookService) GenerateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// --- Mock WebhookEndpointRepository ---

type mockEndpointRepo struct {
	endpoints map[uuid.UUID]*model.WebhookEndpoint
}

func newMockEndpointRepo() *mockEndpointRepo {
	return &mockEndpointRepo{endpoints: make(map[uuid.UUID]*model.WebhookEndpoint)}
}

func (m *mockEndpointRepo) Create(_ context.Context, ep *model.WebhookEndpoint) error {
	if ep.ID == uuid.Nil {
		ep.ID = uuid.New()
	}
	m.endpoints[ep.ID] = ep
	return nil
}

func (m *mockEndpointRepo) FindByUserID(_ context.Context, userID uuid.UUID, _, _ int) ([]model.WebhookEndpoint, int64, error) {
	var result []model.WebhookEndpoint
	for _, ep := range m.endpoints {
		if ep.UserID == userID {
			result = append(result, *ep)
		}
	}
	return result, int64(len(result)), nil
}

func (m *mockEndpointRepo) FindByID(_ context.Context, id uuid.UUID) (*model.WebhookEndpoint, error) {
	if ep, ok := m.endpoints[id]; ok {
		return ep, nil
	}
	return nil, apierror.NewNotFound("webhook endpoint not found")
}

func (m *mockEndpointRepo) Update(_ context.Context, ep *model.WebhookEndpoint) error {
	m.endpoints[ep.ID] = ep
	return nil
}

func (m *mockEndpointRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.endpoints, id)
	return nil
}

func (m *mockEndpointRepo) FindActiveByUserIDAndEvent(_ context.Context, userID uuid.UUID, _ string) ([]model.WebhookEndpoint, error) {
	var result []model.WebhookEndpoint
	for _, ep := range m.endpoints {
		if ep.UserID == userID && ep.IsActive {
			result = append(result, *ep)
		}
	}
	return result, nil
}

// --- Mock WebhookDeliveryRepository ---

type mockDeliveryRepo struct {
	mu         sync.Mutex
	deliveries map[uuid.UUID]*model.WebhookDelivery
}

func newMockDeliveryRepo() *mockDeliveryRepo {
	return &mockDeliveryRepo{deliveries: make(map[uuid.UUID]*model.WebhookDelivery)}
}

func (m *mockDeliveryRepo) Create(_ context.Context, d *model.WebhookDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	m.deliveries[d.ID] = d
	return nil
}

func (m *mockDeliveryRepo) FindPendingDeliveries(_ context.Context, limit int) ([]model.WebhookDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *mockDeliveryRepo) UpdateStatus(_ context.Context, id uuid.UUID, status model.DeliveryStatus, responseCode *int, responseBody *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *mockDeliveryRepo) IncrementRetry(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deliveries[id]
	if !ok {
		return apierror.NewNotFound("webhook delivery not found")
	}
	d.RetryCount++
	return nil
}

// --- Test Helpers ---

func testWorkerConfig() WebhookWorkerConfig {
	return WebhookWorkerConfig{
		PollInterval: 50 * time.Millisecond,
		BatchSize:    10,
		MaxRetries:   5,
		HTTPTimeout:  5 * time.Second,
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: 3,
			Timeout:          100 * time.Millisecond,
			HalfOpenMaxReqs:  2,
		},
	}
}

func createTestDelivery(repo *mockDeliveryRepo, endpointID uuid.UUID, retryCount int) *model.WebhookDelivery {
	payloadJSON, _ := json.Marshal(map[string]interface{}{"event": "test", "id": uuid.New().String()})
	d := &model.WebhookDelivery{
		ID:                uuid.New(),
		WebhookEndpointID: endpointID,
		EventType:         "transaction.success",
		Payload:           datatypes.JSON(payloadJSON),
		Status:            model.DeliveryPending,
		RetryCount:        retryCount,
		MaxRetries:        5,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	repo.deliveries[d.ID] = d
	return d
}

func createTestEndpointInRepo(repo *mockEndpointRepo, url, secret string) *model.WebhookEndpoint {
	eventsJSON, _ := json.Marshal([]string{"transaction.success"})
	ep := &model.WebhookEndpoint{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		URL:       url,
		Secret:    secret,
		Events:    datatypes.JSON(eventsJSON),
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.endpoints[ep.ID] = ep
	return ep
}

func newTestWorker(httpClient *mockHTTPClient) (*WebhookWorker, *mockDeliveryRepo, *mockEndpointRepo) {
	epRepo := newMockEndpointRepo()
	dlRepo := newMockDeliveryRepo()
	ws := &mockWebhookService{}
	cfg := testWorkerConfig()
	w := NewWebhookWorkerWithClient(ws, dlRepo, epRepo, httpClient, cfg)
	return w, dlRepo, epRepo
}

// --- Tests: Successful Delivery ---

func TestProcessDelivery_Success(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{statusCode: 200, body: "ok"}},
	}
	w, dlRepo, epRepo := newTestWorker(httpClient)

	ep := createTestEndpointInRepo(epRepo, "https://example.com/webhook", "test-hmac-secret")
	delivery := createTestDelivery(dlRepo, ep.ID, 0)

	w.processDelivery(context.Background(), delivery)

	// Verify delivery was marked as delivered.
	d := dlRepo.deliveries[delivery.ID]
	if d.Status != model.DeliveryDelivered {
		t.Errorf("expected status delivered, got %s", d.Status)
	}
	if d.DeliveredAt == nil {
		t.Error("expected delivered_at to be set")
	}
	if d.ResponseCode == nil || *d.ResponseCode != 200 {
		t.Errorf("expected response_code 200, got %v", d.ResponseCode)
	}

	// Verify HTTP request was made with correct headers.
	if httpClient.calls != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", httpClient.calls)
	}
	req := httpClient.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("expected POST method, got %s", req.Method)
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("X-Webhook-Signature") == "" {
		t.Error("expected X-Webhook-Signature header to be set")
	}
	if req.Header.Get("X-Webhook-ID") != delivery.ID.String() {
		t.Errorf("expected X-Webhook-ID %s, got %s", delivery.ID, req.Header.Get("X-Webhook-ID"))
	}
	if req.Header.Get("X-Webhook-Event") != "transaction.success" {
		t.Errorf("expected X-Webhook-Event transaction.success, got %s", req.Header.Get("X-Webhook-Event"))
	}
}

// --- Tests: Failure and Retry ---

func TestProcessDelivery_FailureIncrementsRetry(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{statusCode: 500, body: "internal error"}},
	}
	w, dlRepo, epRepo := newTestWorker(httpClient)

	ep := createTestEndpointInRepo(epRepo, "https://example.com/webhook", "test-hmac-secret")
	delivery := createTestDelivery(dlRepo, ep.ID, 0)

	w.processDelivery(context.Background(), delivery)

	// Verify retry was incremented.
	d := dlRepo.deliveries[delivery.ID]
	if d.Status != model.DeliveryPending {
		t.Errorf("expected status pending (still retrying), got %s", d.Status)
	}
	if d.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", d.RetryCount)
	}
}

func TestProcessDelivery_HTTPError_IncrementsRetry(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{err: context.DeadlineExceeded}},
	}
	w, dlRepo, epRepo := newTestWorker(httpClient)

	ep := createTestEndpointInRepo(epRepo, "https://example.com/webhook", "test-hmac-secret")
	delivery := createTestDelivery(dlRepo, ep.ID, 0)

	w.processDelivery(context.Background(), delivery)

	d := dlRepo.deliveries[delivery.ID]
	if d.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", d.RetryCount)
	}
}

// --- Tests: Max Retries Reached ---

func TestProcessDelivery_MaxRetriesReached_MarksFailed(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{statusCode: 500, body: "error"}},
	}
	w, dlRepo, epRepo := newTestWorker(httpClient)

	ep := createTestEndpointInRepo(epRepo, "https://example.com/webhook", "test-hmac-secret")
	// Set retry_count to 4 (max_retries is 5, so next failure = 5th attempt = mark failed).
	delivery := createTestDelivery(dlRepo, ep.ID, 4)

	w.processDelivery(context.Background(), delivery)

	d := dlRepo.deliveries[delivery.ID]
	if d.Status != model.DeliveryFailed {
		t.Errorf("expected status failed, got %s", d.Status)
	}
}

func TestProcessDelivery_RetryCountBelowMax_StaysPending(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{statusCode: 503, body: "unavailable"}},
	}
	w, dlRepo, epRepo := newTestWorker(httpClient)

	ep := createTestEndpointInRepo(epRepo, "https://example.com/webhook", "test-hmac-secret")
	delivery := createTestDelivery(dlRepo, ep.ID, 3) // 3 retries done, max is 5

	w.processDelivery(context.Background(), delivery)

	d := dlRepo.deliveries[delivery.ID]
	if d.Status != model.DeliveryPending {
		t.Errorf("expected status pending, got %s", d.Status)
	}
	if d.RetryCount != 4 {
		t.Errorf("expected retry_count 4, got %d", d.RetryCount)
	}
}

// --- Tests: Endpoint Not Found ---

func TestProcessDelivery_EndpointNotFound(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{statusCode: 200, body: "ok"}},
	}
	w, dlRepo, _ := newTestWorker(httpClient)

	// Create delivery with non-existent endpoint.
	delivery := createTestDelivery(dlRepo, uuid.New(), 0)

	w.processDelivery(context.Background(), delivery)

	// Should increment retry since endpoint lookup failed.
	d := dlRepo.deliveries[delivery.ID]
	if d.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", d.RetryCount)
	}
	// No HTTP call should have been made.
	if httpClient.calls != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", httpClient.calls)
	}
}

// --- Tests: Circuit Breaker State Transitions ---

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          100 * time.Millisecond,
		HalfOpenMaxReqs:  2,
	}
	cb := newCircuitBreaker(cfg)

	// Initial state should be closed.
	if cb.State() != CircuitClosed {
		t.Fatalf("expected initial state closed, got %s", cb.State())
	}

	// Record failures up to threshold.
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
		if cb.State() != CircuitClosed {
			t.Errorf("expected state closed after %d failures, got %s", i+1, cb.State())
		}
	}

	// Third failure should trip to open.
	from, to, changed := cb.RecordFailure()
	if !changed {
		t.Error("expected state change on threshold failure")
	}
	if from != CircuitClosed || to != CircuitOpen {
		t.Errorf("expected closed→open, got %s→%s", from, to)
	}
	if cb.State() != CircuitOpen {
		t.Errorf("expected state open, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpenBlocksRequests(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          1 * time.Hour, // long timeout so it stays open
		HalfOpenMaxReqs:  2,
	}
	cb := newCircuitBreaker(cfg)

	cb.RecordFailure() // Trip to open.
	if cb.State() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	if cb.Allow() {
		t.Error("expected Allow() to return false when circuit is open")
	}
}

func TestCircuitBreaker_OpenToHalfOpenAfterTimeout(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          50 * time.Millisecond,
		HalfOpenMaxReqs:  2,
	}
	cb := newCircuitBreaker(cfg)

	cb.RecordFailure() // Trip to open.
	if cb.State() != CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	// Wait for timeout to elapse.
	time.Sleep(60 * time.Millisecond)

	// Should allow request now (timeout elapsed).
	if !cb.Allow() {
		t.Error("expected Allow() to return true after timeout")
	}

	// Record success should transition to half-open.
	from, to, changed := cb.RecordSuccess()
	if !changed {
		t.Error("expected state change")
	}
	if from != CircuitOpen || to != CircuitHalfOpen {
		t.Errorf("expected open→half-open, got %s→%s", from, to)
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          1 * time.Millisecond,
		HalfOpenMaxReqs:  2,
	}
	cb := newCircuitBreaker(cfg)

	// Trip to open, then wait for timeout.
	cb.RecordFailure()
	time.Sleep(5 * time.Millisecond)

	// First success transitions open→half-open.
	cb.Allow()
	cb.RecordSuccess()
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("expected half-open, got %s", cb.State())
	}

	// Second success in half-open should close the circuit.
	from, to, changed := cb.RecordSuccess()
	if !changed {
		t.Error("expected state change")
	}
	if from != CircuitHalfOpen || to != CircuitClosed {
		t.Errorf("expected half-open→closed, got %s→%s", from, to)
	}
}

func TestCircuitBreaker_HalfOpenToOpenOnFailure(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          1 * time.Millisecond,
		HalfOpenMaxReqs:  2,
	}
	cb := newCircuitBreaker(cfg)

	// Trip to open, then wait for timeout.
	cb.RecordFailure()
	time.Sleep(5 * time.Millisecond)

	// Transition to half-open.
	cb.Allow()
	cb.RecordSuccess()
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("expected half-open, got %s", cb.State())
	}

	// Failure in half-open should go back to open.
	from, to, changed := cb.RecordFailure()
	if !changed {
		t.Error("expected state change")
	}
	if from != CircuitHalfOpen || to != CircuitOpen {
		t.Errorf("expected half-open→open, got %s→%s", from, to)
	}
}

func TestCircuitBreaker_SuccessResetFailureCount(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		Timeout:          100 * time.Millisecond,
		HalfOpenMaxReqs:  2,
	}
	cb := newCircuitBreaker(cfg)

	// Record 2 failures (below threshold).
	cb.RecordFailure()
	cb.RecordFailure()

	// Success should reset failure count.
	cb.RecordSuccess()

	// Now 2 more failures should not trip (count was reset).
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after reset, got %s", cb.State())
	}

	// Third failure after reset should trip.
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after 3 failures, got %s", cb.State())
	}
}

// --- Tests: Circuit Breaker Registry ---

func TestCircuitBreakerRegistry_ReturnsSameInstance(t *testing.T) {
	reg := newCircuitBreakerRegistry(DefaultCircuitBreakerConfig())

	cb1 := reg.Get("https://example.com/hook1")
	cb2 := reg.Get("https://example.com/hook1")
	cb3 := reg.Get("https://example.com/hook2")

	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance for same URL")
	}
	if cb1 == cb3 {
		t.Error("expected different circuit breaker instances for different URLs")
	}
}

// --- Tests: Circuit Breaker Integration with Worker ---

func TestProcessDelivery_CircuitBreakerOpens_SkipsDelivery(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{statusCode: 500, body: "error"}},
	}
	w, dlRepo, epRepo := newTestWorker(httpClient)

	ep := createTestEndpointInRepo(epRepo, "https://example.com/webhook", "test-hmac-secret")

	// Trigger enough failures to open the circuit breaker (threshold=3).
	for i := 0; i < 3; i++ {
		d := createTestDelivery(dlRepo, ep.ID, 0)
		w.processDelivery(context.Background(), d)
	}

	// Verify circuit is open.
	if w.GetCircuitBreakerState(ep.URL) != CircuitOpen {
		t.Fatalf("expected circuit open, got %s", w.GetCircuitBreakerState(ep.URL))
	}

	// Next delivery should be skipped (circuit open).
	httpClient.calls = 0
	skipDelivery := createTestDelivery(dlRepo, ep.ID, 0)
	w.processDelivery(context.Background(), skipDelivery)

	// No additional HTTP call should have been made.
	if httpClient.calls != 0 {
		t.Errorf("expected 0 HTTP calls when circuit open, got %d", httpClient.calls)
	}

	// Delivery should still be pending (not failed, not retried).
	d := dlRepo.deliveries[skipDelivery.ID]
	if d.Status != model.DeliveryPending {
		t.Errorf("expected status pending (skipped), got %s", d.Status)
	}
	if d.RetryCount != 0 {
		t.Errorf("expected retry_count 0 (skipped), got %d", d.RetryCount)
	}
}

// --- Tests: Start and Stop (Graceful Shutdown) ---

func TestStartAndStop_GracefulShutdown(t *testing.T) {
	httpClient := &mockHTTPClient{
		responses: []mockHTTPResponse{{statusCode: 200, body: "ok"}},
	}
	w, dlRepo, epRepo := newTestWorker(httpClient)

	ep := createTestEndpointInRepo(epRepo, "https://example.com/webhook", "test-hmac-secret")
	createTestDelivery(dlRepo, ep.ID, 0)

	// Start worker in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Start(context.Background())
	}()

	// Give the worker time to process at least one batch.
	time.Sleep(200 * time.Millisecond)

	// Stop the worker.
	w.Stop()

	// Wait for Start to return.
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected no error from Start, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return within timeout after Stop")
	}

	// Verify the delivery was processed.
	processed := false
	for _, d := range dlRepo.deliveries {
		if d.Status == model.DeliveryDelivered {
			processed = true
			break
		}
	}
	if !processed {
		t.Error("expected at least one delivery to be processed before shutdown")
	}
}

// --- Tests: NewWebhookWorker (default HTTP client) ---

func TestNewWebhookWorker_DefaultHTTPClient(t *testing.T) {
	epRepo := newMockEndpointRepo()
	dlRepo := newMockDeliveryRepo()
	ws := &mockWebhookService{}
	cfg := DefaultWebhookWorkerConfig()

	w := NewWebhookWorker(ws, dlRepo, epRepo, cfg)
	if w == nil {
		t.Fatal("expected non-nil worker")
	}
	if w.httpClient == nil {
		t.Error("expected non-nil HTTP client")
	}
	if w.config.HTTPTimeout != 30*time.Second {
		t.Errorf("expected default HTTP timeout 30s, got %v", w.config.HTTPTimeout)
	}
	if w.config.MaxRetries != 5 {
		t.Errorf("expected default max retries 5, got %d", w.config.MaxRetries)
	}
}

// --- Tests: CircuitState String ---

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}
	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.state.String())
		}
	}
}
