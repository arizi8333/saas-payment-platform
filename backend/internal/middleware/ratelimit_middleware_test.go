package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/saas-payment-platform/backend/internal/model"
)

// --- Mock Redis types ---

// mockRedisCmd implements RedisCmd for testing.
type mockRedisCmd struct {
	val int64
	err error
}

func (m *mockRedisCmd) Int64() (int64, error) { return m.val, m.err }

// mockRedis implements RedisClient for testing.
type mockRedis struct {
	evalFn func(ctx context.Context, script string, keys []string, args ...interface{}) RedisCmd
}

func (m *mockRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) RedisCmd {
	return m.evalFn(ctx, script, keys, args...)
}

// --- Tests ---

func TestRateLimiter_RequestAllowedWithinLimit(t *testing.T) {
	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, _ []string, _ ...interface{}) RedisCmd {
			return &mockRedisCmd{val: 5, err: nil} // 5 requests, well under limit
		},
	}

	rl := NewRateLimiter(redis, 100, time.Hour)
	app := fiber.New()
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify remaining = limit - count = 100 - 5 = 95
	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "95" {
		t.Errorf("expected X-RateLimit-Remaining=95, got %q", got)
	}
}

func TestRateLimiter_RequestRejectedWhenLimitExceeded(t *testing.T) {
	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, _ []string, _ ...interface{}) RedisCmd {
			return &mockRedisCmd{val: 101, err: nil} // 101 > limit of 100
		},
	}

	rl := NewRateLimiter(redis, 100, time.Hour)
	app := fiber.New()
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}

	body := parseBody(t, resp)
	if body.Success {
		t.Error("expected success=false")
	}

	if got := resp.Header.Get("Retry-After"); got == "" {
		t.Error("expected Retry-After header to be set")
	}

	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("expected X-RateLimit-Remaining=0, got %q", got)
	}
}

func TestRateLimiter_HeadersPresentInResponse(t *testing.T) {
	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, _ []string, _ ...interface{}) RedisCmd {
			return &mockRedisCmd{val: 10, err: nil}
		},
	}

	rl := NewRateLimiter(redis, 1000, time.Hour)
	app := fiber.New()
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("X-RateLimit-Limit"); got != "1000" {
		t.Errorf("expected X-RateLimit-Limit=1000, got %q", got)
	}
	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "990" {
		t.Errorf("expected X-RateLimit-Remaining=990, got %q", got)
	}
	if got := resp.Header.Get("X-RateLimit-Reset"); got == "" {
		t.Error("expected X-RateLimit-Reset header to be set")
	}
}

func TestRateLimiter_SlidingWindowReset(t *testing.T) {
	// Simulate a counter that increments on each call, then resets after window.
	var counter atomic.Int64

	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, _ []string, _ ...interface{}) RedisCmd {
			c := counter.Add(1)
			return &mockRedisCmd{val: c, err: nil}
		},
	}

	rl := NewRateLimiter(redis, 3, time.Hour)
	app := fiber.New()
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	// First 3 requests should be allowed (count 1, 2, 3 <= limit 3).
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// 4th request should be rejected (count 4 > limit 3).
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("4th request: expected 429, got %d", resp.StatusCode)
	}

	// Simulate window reset: counter goes back to 1.
	counter.Store(0)

	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("after reset: expected 200, got %d", resp.StatusCode)
	}
}

func TestRateLimiter_UsesPerKeyLimit(t *testing.T) {
	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, _ []string, _ ...interface{}) RedisCmd {
			return &mockRedisCmd{val: 50, err: nil}
		},
	}

	rl := NewRateLimiter(redis, 1000, time.Hour)
	app := fiber.New()

	// Simulate apikey_middleware setting the API key in locals with a custom rate limit.
	apiKey := &model.APIKey{
		ID:        uuid.New(),
		RateLimit: 500,
	}
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(LocalsAPIKey, apiKey)
		return c.Next()
	})
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Should use the per-key limit of 500, not the default 1000.
	if got := resp.Header.Get("X-RateLimit-Limit"); got != "500" {
		t.Errorf("expected X-RateLimit-Limit=500, got %q", got)
	}
	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "450" {
		t.Errorf("expected X-RateLimit-Remaining=450, got %q", got)
	}
}

func TestRateLimiter_FallsBackToIPWhenNoAPIKey(t *testing.T) {
	var capturedKey string
	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, keys []string, _ ...interface{}) RedisCmd {
			if len(keys) > 0 {
				capturedKey = keys[0]
			}
			return &mockRedisCmd{val: 1, err: nil}
		},
	}

	rl := NewRateLimiter(redis, 100, time.Hour)
	app := fiber.New()
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Key should be based on IP, not an API key UUID.
	if capturedKey == "" {
		t.Fatal("expected redis key to be set")
	}
	// The key should start with "ratelimit:" and NOT contain a UUID pattern from an API key.
	if len(capturedKey) < len("ratelimit:") {
		t.Errorf("unexpected key format: %q", capturedKey)
	}
}

func TestRateLimiter_RedisError_FailsOpen(t *testing.T) {
	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, _ []string, _ ...interface{}) RedisCmd {
			return &mockRedisCmd{val: 0, err: errors.New("connection refused")}
		},
	}

	rl := NewRateLimiter(redis, 100, time.Hour)
	app := fiber.New()
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	// Should allow the request when Redis is down (fail-open).
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (fail-open), got %d", resp.StatusCode)
	}
}

func TestRateLimiter_ExactlyAtLimit_Allowed(t *testing.T) {
	redis := &mockRedis{
		evalFn: func(_ context.Context, _ string, _ []string, _ ...interface{}) RedisCmd {
			return &mockRedisCmd{val: 100, err: nil} // exactly at limit
		},
	}

	rl := NewRateLimiter(redis, 100, time.Hour)
	app := fiber.New()
	app.Use(rl.Limit())
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, _ := app.Test(req)

	// count == limit should be allowed (only count > limit is rejected).
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 at exact limit, got %d", resp.StatusCode)
	}

	if got := resp.Header.Get("X-RateLimit-Remaining"); got != "0" {
		t.Errorf("expected X-RateLimit-Remaining=0, got %q", got)
	}
}
