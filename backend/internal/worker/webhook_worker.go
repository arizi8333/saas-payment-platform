package worker

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/repository"
	"github.com/saas-payment-platform/backend/internal/service"
)

// --- Circuit Breaker ---

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                         // Failing, reject requests
	CircuitHalfOpen                     // Testing recovery
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds configuration for the circuit breaker.
type CircuitBreakerConfig struct {
	FailureThreshold int           // Number of failures before opening
	Timeout          time.Duration // How long to stay open before half-open
	HalfOpenMaxReqs  int           // Max requests allowed in half-open state
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		Timeout:          30 * time.Second,
		HalfOpenMaxReqs:  2,
	}
}

// circuitBreaker implements a simple in-memory circuit breaker per endpoint URL.
type circuitBreaker struct {
	mu              sync.Mutex
	state           CircuitState
	failureCount    int
	successCount    int // successes in half-open state
	lastFailureTime time.Time
	config          CircuitBreakerConfig
}

func newCircuitBreaker(cfg CircuitBreakerConfig) *circuitBreaker {
	return &circuitBreaker{
		state:  CircuitClosed,
		config: cfg,
	}
}

// Allow checks if a request is allowed through the circuit breaker.
// Returns true if the request can proceed.
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has elapsed to transition to half-open.
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			return true // Will transition in RecordResult
		}
		return false
	case CircuitHalfOpen:
		return cb.successCount < cb.config.HalfOpenMaxReqs
	}
	return false
}

// RecordSuccess records a successful request.
// Returns the previous and new state if a transition occurred.
func (cb *circuitBreaker) RecordSuccess() (from, to CircuitState, changed bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	prev := cb.state
	switch cb.state {
	case CircuitClosed:
		cb.failureCount = 0
	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.HalfOpenMaxReqs {
			cb.state = CircuitClosed
			cb.failureCount = 0
			cb.successCount = 0
		}
	case CircuitOpen:
		// Came through after timeout — treat as half-open test.
		cb.state = CircuitHalfOpen
		cb.successCount = 1
		cb.failureCount = 0
		if cb.successCount >= cb.config.HalfOpenMaxReqs {
			cb.state = CircuitClosed
			cb.successCount = 0
		}
	}
	return prev, cb.state, prev != cb.state
}

// RecordFailure records a failed request.
// Returns the previous and new state if a transition occurred.
func (cb *circuitBreaker) RecordFailure() (from, to CircuitState, changed bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	prev := cb.state
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failureCount++
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = CircuitOpen
			cb.successCount = 0
		}
	case CircuitHalfOpen:
		// Any failure in half-open goes back to open.
		cb.state = CircuitOpen
		cb.successCount = 0
	case CircuitOpen:
		// Already open, stay open.
		cb.successCount = 0
	}
	return prev, cb.state, prev != cb.state
}

// State returns the current circuit state.
func (cb *circuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// circuitBreakerRegistry manages per-endpoint circuit breakers.
type circuitBreakerRegistry struct {
	mu       sync.Mutex
	breakers map[string]*circuitBreaker
	config   CircuitBreakerConfig
}

func newCircuitBreakerRegistry(cfg CircuitBreakerConfig) *circuitBreakerRegistry {
	return &circuitBreakerRegistry{
		breakers: make(map[string]*circuitBreaker),
		config:   cfg,
	}
}

func (r *circuitBreakerRegistry) Get(url string) *circuitBreaker {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, ok := r.breakers[url]; ok {
		return cb
	}
	cb := newCircuitBreaker(r.config)
	r.breakers[url] = cb
	return cb
}

// --- Webhook Worker ---

// WebhookWorkerConfig holds configuration for the webhook worker.
type WebhookWorkerConfig struct {
	PollInterval   time.Duration
	BatchSize      int
	MaxRetries     int
	HTTPTimeout    time.Duration
	CircuitBreaker CircuitBreakerConfig
}

// DefaultWebhookWorkerConfig returns sensible defaults.
func DefaultWebhookWorkerConfig() WebhookWorkerConfig {
	return WebhookWorkerConfig{
		PollInterval:   5 * time.Second,
		BatchSize:      10,
		MaxRetries:     5,
		HTTPTimeout:    30 * time.Second,
		CircuitBreaker: DefaultCircuitBreakerConfig(),
	}
}

// HTTPDoer abstracts the HTTP client for testability.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// WebhookWorker processes pending webhook deliveries in the background.
type WebhookWorker struct {
	webhookService service.WebhookService
	deliveryRepo   repository.WebhookDeliveryRepository
	endpointRepo   repository.WebhookEndpointRepository
	httpClient     HTTPDoer
	config         WebhookWorkerConfig
	cbRegistry     *circuitBreakerRegistry

	mu      sync.Mutex
	cancel  context.CancelFunc
	started chan struct{} // closed once Start() has initialized cancel
	wg      sync.WaitGroup
}

// NewWebhookWorker creates a new WebhookWorker with the given dependencies and config.
func NewWebhookWorker(
	ws service.WebhookService,
	deliveryRepo repository.WebhookDeliveryRepository,
	endpointRepo repository.WebhookEndpointRepository,
	cfg WebhookWorkerConfig,
) *WebhookWorker {
	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
	}
	return &WebhookWorker{
		webhookService: ws,
		deliveryRepo:   deliveryRepo,
		endpointRepo:   endpointRepo,
		httpClient:     httpClient,
		config:         cfg,
		cbRegistry:     newCircuitBreakerRegistry(cfg.CircuitBreaker),
		started:        make(chan struct{}),
	}
}

// NewWebhookWorkerWithClient creates a WebhookWorker with a custom HTTP client (for testing).
func NewWebhookWorkerWithClient(
	ws service.WebhookService,
	deliveryRepo repository.WebhookDeliveryRepository,
	endpointRepo repository.WebhookEndpointRepository,
	httpClient HTTPDoer,
	cfg WebhookWorkerConfig,
) *WebhookWorker {
	return &WebhookWorker{
		webhookService: ws,
		deliveryRepo:   deliveryRepo,
		endpointRepo:   endpointRepo,
		httpClient:     httpClient,
		config:         cfg,
		cbRegistry:     newCircuitBreakerRegistry(cfg.CircuitBreaker),
		started:        make(chan struct{}),
	}
}

// Start begins the worker loop that polls and processes pending webhook deliveries.
// It blocks until the context is cancelled or Stop is called.
func (w *WebhookWorker) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	w.mu.Lock()
	w.cancel = cancel
	w.mu.Unlock()
	close(w.started) // signal that cancel is ready

	slog.InfoContext(ctx, "webhook worker started",
		"poll_interval", w.config.PollInterval,
		"batch_size", w.config.BatchSize,
		"max_retries", w.config.MaxRetries,
		"http_timeout", w.config.HTTPTimeout,
	)

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "webhook worker shutting down, waiting for in-flight deliveries")
			w.wg.Wait()
			slog.InfoContext(ctx, "webhook worker stopped")
			return nil
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

// Stop gracefully stops the worker, waiting for in-flight deliveries to complete.
func (w *WebhookWorker) Stop() {
	<-w.started // wait until Start() has set cancel
	w.mu.Lock()
	cancel := w.cancel
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// processBatch fetches pending deliveries and processes each one.
func (w *WebhookWorker) processBatch(ctx context.Context) {
	deliveries, err := w.deliveryRepo.FindPendingDeliveries(ctx, w.config.BatchSize)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch pending deliveries", "error", err)
		return
	}

	for _, delivery := range deliveries {
		// Check context before processing each delivery.
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.wg.Add(1)
		d := delivery // capture loop variable
		go func() {
			defer w.wg.Done()
			w.processDelivery(ctx, &d)
		}()
	}
}

// processDelivery handles a single webhook delivery attempt.
func (w *WebhookWorker) processDelivery(ctx context.Context, delivery *model.WebhookDelivery) {
	// Look up the endpoint to get URL and secret.
	endpoint, err := w.endpointRepo.FindByID(ctx, delivery.WebhookEndpointID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find webhook endpoint",
			"delivery_id", delivery.ID,
			"endpoint_id", delivery.WebhookEndpointID,
			"error", err,
		)
		w.handleFailure(ctx, delivery)
		return
	}

	// Check circuit breaker for this endpoint URL.
	cb := w.cbRegistry.Get(endpoint.URL)
	if !cb.Allow() {
		slog.WarnContext(ctx, "circuit breaker open, skipping delivery",
			"delivery_id", delivery.ID,
			"url", endpoint.URL,
			"circuit_state", cb.State().String(),
		)
		// Don't count as a failure attempt — just skip and let it be retried later.
		return
	}

	// Generate HMAC signature.
	payload := []byte(delivery.Payload)
	signature := w.webhookService.GenerateSignature(payload, endpoint.Secret)

	// Build HTTP request.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(payload))
	if err != nil {
		slog.ErrorContext(ctx, "failed to create HTTP request",
			"delivery_id", delivery.ID,
			"url", endpoint.URL,
			"error", err,
		)
		w.handleFailure(ctx, delivery)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-ID", delivery.ID.String())
	req.Header.Set("X-Webhook-Event", delivery.EventType)

	// Execute HTTP POST.
	resp, err := w.httpClient.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "webhook delivery HTTP error",
			"delivery_id", delivery.ID,
			"url", endpoint.URL,
			"error", err,
		)
		w.recordCircuitFailure(cb, endpoint.URL)
		w.handleFailure(ctx, delivery)
		return
	}
	defer resp.Body.Close()

	// Read response body (limited to 1KB).
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	bodyStr := string(bodyBytes)
	statusCode := resp.StatusCode

	if statusCode >= 200 && statusCode < 300 {
		// Success: update status to delivered.
		w.recordCircuitSuccess(cb, endpoint.URL)
		if err := w.deliveryRepo.UpdateStatus(ctx, delivery.ID, model.DeliveryDelivered, &statusCode, &bodyStr); err != nil {
			slog.ErrorContext(ctx, "failed to update delivery status to delivered",
				"delivery_id", delivery.ID,
				"error", err,
			)
		} else {
			slog.InfoContext(ctx, "webhook delivered successfully",
				"delivery_id", delivery.ID,
				"url", endpoint.URL,
				"status_code", statusCode,
			)
		}
	} else {
		// Non-2xx: record failure.
		slog.WarnContext(ctx, "webhook delivery failed with non-2xx",
			"delivery_id", delivery.ID,
			"url", endpoint.URL,
			"status_code", statusCode,
		)
		w.recordCircuitFailure(cb, endpoint.URL)
		w.handleFailureWithResponse(ctx, delivery, &statusCode, &bodyStr)
	}
}

// handleFailure handles a delivery failure without HTTP response info.
func (w *WebhookWorker) handleFailure(ctx context.Context, delivery *model.WebhookDelivery) {
	w.handleFailureWithResponse(ctx, delivery, nil, nil)
}

// handleFailureWithResponse handles a delivery failure, incrementing retry or marking as failed.
func (w *WebhookWorker) handleFailureWithResponse(ctx context.Context, delivery *model.WebhookDelivery, responseCode *int, responseBody *string) {
	nextRetry := delivery.RetryCount + 1

	if nextRetry >= w.config.MaxRetries {
		// Max retries reached: mark as failed.
		if err := w.deliveryRepo.UpdateStatus(ctx, delivery.ID, model.DeliveryFailed, responseCode, responseBody); err != nil {
			slog.ErrorContext(ctx, "failed to update delivery status to failed",
				"delivery_id", delivery.ID,
				"error", err,
			)
		} else {
			slog.WarnContext(ctx, "webhook delivery permanently failed after max retries",
				"delivery_id", delivery.ID,
				"retry_count", nextRetry,
				"max_retries", w.config.MaxRetries,
			)
		}
		return
	}

	// Increment retry count and schedule next retry with exponential backoff.
	if err := w.deliveryRepo.IncrementRetry(ctx, delivery.ID); err != nil {
		slog.ErrorContext(ctx, "failed to increment delivery retry",
			"delivery_id", delivery.ID,
			"error", err,
		)
	} else {
		slog.InfoContext(ctx, "webhook delivery scheduled for retry",
			"delivery_id", delivery.ID,
			"retry_count", nextRetry,
		)
	}
}

// recordCircuitSuccess records a success on the circuit breaker and logs state changes.
func (w *WebhookWorker) recordCircuitSuccess(cb *circuitBreaker, url string) {
	from, to, changed := cb.RecordSuccess()
	if changed {
		slog.Info("circuit breaker state changed",
			"url", url,
			"from", from.String(),
			"to", to.String(),
		)
	}
}

// recordCircuitFailure records a failure on the circuit breaker and logs state changes.
func (w *WebhookWorker) recordCircuitFailure(cb *circuitBreaker, url string) {
	from, to, changed := cb.RecordFailure()
	if changed {
		slog.Warn("circuit breaker state changed",
			"url", url,
			"from", from.String(),
			"to", to.String(),
		)
	}
}

// GetCircuitBreakerState returns the circuit breaker state for a given URL (for testing/monitoring).
func (w *WebhookWorker) GetCircuitBreakerState(url string) CircuitState {
	return w.cbRegistry.Get(url).State()
}
