package middleware

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// GoRedisAdapter wraps a go-redis *redis.Client to satisfy the RedisClient
// interface used by RateLimiter. This allows the rate limiter to be tested
// with a mock while using the real Redis client in production.
type GoRedisAdapter struct {
	client *redis.Client
}

// NewGoRedisAdapter creates a new adapter around a go-redis client.
func NewGoRedisAdapter(client *redis.Client) *GoRedisAdapter {
	return &GoRedisAdapter{client: client}
}

// goRedisCmd wraps *redis.Cmd to satisfy RedisCmd.
type goRedisCmd struct {
	cmd *redis.Cmd
}

func (c *goRedisCmd) Int64() (int64, error) { return c.cmd.Int64() }

// Eval executes a Lua script via the underlying go-redis client.
func (a *GoRedisAdapter) Eval(ctx context.Context, script string, keys []string, args ...interface{}) RedisCmd {
	return &goRedisCmd{cmd: a.client.Eval(ctx, script, keys, args...)}
}
