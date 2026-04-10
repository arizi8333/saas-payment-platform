package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/saas-payment-platform/backend/internal/model"
	"github.com/saas-payment-platform/backend/internal/pkg/response"
)

// RedisClient defines the minimal Redis interface needed by the rate limiter.
// This allows easy mocking in tests without requiring a real Redis connection.
type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) RedisCmd
}

// RedisCmd abstracts the result of a Redis command.
type RedisCmd interface {
	Int64() (int64, error)
}

// RateLimiter implements a sliding window rate limiter backed by Redis.
type RateLimiter struct {
	redis        RedisClient
	defaultLimit int
	window       time.Duration
}

// NewRateLimiter creates a new RateLimiter.
// defaultLimit is used when no per-key limit is available.
// window is the sliding window duration (e.g. 1 hour).
func NewRateLimiter(redis RedisClient, defaultLimit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redis:        redis,
		defaultLimit: defaultLimit,
		window:       window,
	}
}

// slidingWindowScript is a Lua script that atomically implements a sliding
// window counter using a Redis sorted set. It removes expired entries, adds
// the current timestamp, sets the TTL, and returns the current count.
const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local min_score = now - window

redis.call('ZREMRANGEBYSCORE', key, '-inf', min_score)
redis.call('ZADD', key, now, now .. '-' .. math.random(1000000))
local count = redis.call('ZCARD', key)
redis.call('PEXPIRE', key, window)

return count
`

// Limit returns a Fiber middleware handler that enforces rate limiting.
// It identifies the client by API key (from Fiber locals, set by apikey_middleware)
// or falls back to the request IP address.
// Per-key limits are read from the APIKey model stored in locals; otherwise the
// configured default limit is used.
func (rl *RateLimiter) Limit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		identifier, limit := rl.resolveClient(c)
		key := fmt.Sprintf("ratelimit:%s", identifier)

		nowMs := time.Now().UnixMilli()
		windowMs := rl.window.Milliseconds()

		cmd := rl.redis.Eval(c.UserContext(), slidingWindowScript, []string{key}, nowMs, windowMs)
		count, err := cmd.Int64()
		if err != nil {
			// If Redis is unavailable, allow the request (fail-open).
			return c.Next()
		}

		remaining := int64(limit) - count
		if remaining < 0 {
			remaining = 0
		}

		resetAt := time.Now().Add(rl.window).Unix()

		// Always set rate limit headers.
		c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))

		if count > int64(limit) {
			retryAfter := int(rl.window.Seconds())
			c.Set("Retry-After", strconv.Itoa(retryAfter))
			return response.ErrorResponse(c, fiber.StatusTooManyRequests, "rate limit exceeded")
		}

		return c.Next()
	}
}

// resolveClient determines the client identifier and applicable rate limit.
// Priority: API key (from locals set by apikey_middleware) > IP address.
func (rl *RateLimiter) resolveClient(c *fiber.Ctx) (string, int) {
	if apiKey, ok := c.Locals(LocalsAPIKey).(*model.APIKey); ok && apiKey != nil {
		limit := apiKey.RateLimit
		if limit <= 0 {
			limit = rl.defaultLimit
		}
		return apiKey.ID.String(), limit
	}
	return c.IP(), rl.defaultLimit
}
